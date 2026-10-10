package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func recordGoalSourceEventTx(ctx context.Context, tx pgx.Tx, fluctlightID, profileID, kind, id, version, conversationID, subject, status string, at time.Time) (int64, error) {
	var eventID int64
	err := tx.QueryRow(ctx, `INSERT INTO public.goal_source_events(fluctlight_id,profile_id,source_kind,source_id,source_version,conversation_id,subject_actor_id,source_status,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(fluctlight_id,source_kind,source_id,source_version,source_status) DO UPDATE SET source_id=EXCLUDED.source_id RETURNING id`, fluctlightID, nullableString(profileID), kind, id, version, nullableString(conversationID), nullableString(subject), status, at.UTC()).Scan(&eventID)
	return eventID, err
}

func queueGoalEvaluationTx(ctx context.Context, tx pgx.Tx, owner, profile, reason, identity string, goalIDs []string) (string, error) {
	var eligible bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND (profile_id IS NULL OR profile_id=$2) AND status IN ('active','paused'))`, owner, profile).Scan(&eligible); err != nil {
		return "", err
	}
	if !eligible {
		return "", nil
	}
	requestID := "goal_evaluation_" + stableDigest(owner+"\x1f"+profile+"\x1f"+identity)
	forcedGoalIDs := []string{}
	if reason == "owner_reassess" {
		forcedGoalIDs = append(forcedGoalIDs, goalIDs...)
	}
	var actualID string
	err := tx.QueryRow(ctx, `INSERT INTO public.goal_evaluation_requests(id,fluctlight_id,profile_id,goal_ids,reason,status,available_at,snapshot) VALUES($1,$2,$3,$4,$5,'pending',now()+interval '2 seconds',jsonb_build_object('forced_goal_ids',$6::jsonb)) ON CONFLICT(fluctlight_id,profile_id) WHERE status='pending' DO UPDATE SET goal_ids=(SELECT COALESCE(jsonb_agg(v ORDER BY v),'[]'::jsonb) FROM (SELECT DISTINCT value v FROM jsonb_array_elements(goal_evaluation_requests.goal_ids || EXCLUDED.goal_ids)) ids),reason=CASE WHEN EXCLUDED.reason='owner_reassess' THEN EXCLUDED.reason ELSE goal_evaluation_requests.reason END,snapshot=jsonb_set(goal_evaluation_requests.snapshot,'{forced_goal_ids}',(SELECT COALESCE(jsonb_agg(v ORDER BY v),'[]'::jsonb) FROM (SELECT DISTINCT value v FROM jsonb_array_elements(COALESCE(goal_evaluation_requests.snapshot->'forced_goal_ids','[]'::jsonb) || (EXCLUDED.snapshot->'forced_goal_ids'))) ids),true),updated_at=now() RETURNING id`, requestID, owner, profile, jsonBytes(goalIDs), reason, jsonBytes(forcedGoalIDs)).Scan(&actualID)
	if err != nil {
		return "", err
	}
	payload := map[string]any{"fluctlight_id": owner, "evaluation_id": actualID, "profile_id": profile}
	_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at) VALUES($1,$2,'lifecycle','goal.evaluate',$3,now()+interval '2 seconds') ON CONFLICT(intent_id) DO NOTHING`, actualID, "goal-evaluation:"+actualID, jsonBytes(payload))
	return actualID, err
}

func (a *App) recordGoalMessageTx(ctx context.Context, tx pgx.Tx, owner, messageID, profile string) error {
	var conversation, author, fingerprint string
	var at time.Time
	if err := tx.QueryRow(ctx, `SELECT conversation_id,author_actor_id,md5(text || attachment_refs::text),created_at FROM public.conversation_messages WHERE id=$1`, messageID).Scan(&conversation, &author, &fingerprint, &at); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.conversation_messages SET acting_profile_id=COALESCE(acting_profile_id,$2) WHERE id=$1`, messageID, nullableString(profile)); err != nil {
		return err
	}
	_, err := recordGoalSourceEventTx(ctx, tx, owner, profile, "message", messageID, fingerprint, conversation, author, "committed", at)
	return err
}

func recordGoalOutcomeSourceTx(ctx context.Context, tx pgx.Tx, outcome ActionOutcome) error {
	if !goalOutcomeCarriesEvidence(outcome) {
		return nil
	}
	profile := ""
	goalIDs := []string{}
	for _, ref := range outcome.GoalRefs {
		if entry, ok := outcome.ContextReferences[ref]; ok && entry.Kind == ContextReferenceGoal {
			goalIDs = append(goalIDs, entry.EntityID)
			profile = firstString(decodeObject(entry.Snapshot)["profile_id"], profile)
		}
	}
	eventID, err := recordGoalSourceEventTx(ctx, tx, outcome.FluctlightID, profile, "outcome", outcome.ID, strconv.Itoa(outcome.Revision), "", outcome.FluctlightID, "committed", outcome.OccurredAt)
	if err != nil {
		return err
	}
	if len(goalIDs) > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO public.goal_evidence_links(id,fluctlight_id,goal_id,source_event_id,status,reason) SELECT 'goal_outcome_'||md5(g.id||':'||$3::text),$1,g.id,$3,'candidate','actual outcome explicitly bound to Goal' FROM public.fluctlight_goals g WHERE g.fluctlight_id=$1 AND g.id=ANY($2::text[]) ON CONFLICT(goal_id,source_event_id) DO NOTHING`, outcome.FluctlightID, goalIDs, eventID); err != nil {
			return err
		}
		_, err = queueGoalEvaluationTx(ctx, tx, outcome.FluctlightID, profile, "actual_outcome", fmt.Sprint(eventID), goalIDs)
	}
	return err
}

func goalOutcomeCarriesEvidence(outcome ActionOutcome) bool {
	if outcome.Status == ActionOutcomePending {
		return false
	}
	switch strings.TrimSpace(outcome.CapabilityName) {
	case goalEvaluationSubmit, goalObjectSubmit, goalPlanSubmit, "goal.inspect", "goal.decide", "goal.evaluate", "goal.review",
		"actor.inspect", "habit.inspect", "intention.inspect", "schedule.inspect",
		"persona.detail", "relationship.lookup", "memory.recall", "capability.discover":
		return false
	}
	if strings.TrimSpace(outcome.CapabilityName) != "" {
		return true
	}
	if names := arrayValue(outcome.Expected["capability_names"]); len(names) > 0 {
		for _, raw := range names {
			child := outcome
			child.CapabilityName = stringValue(raw)
			if child.CapabilityName != "" && goalOutcomeCarriesEvidence(child) {
				return true
			}
		}
		return false
	}
	switch strings.TrimSpace(stringValue(outcome.Expected["action_type"])) {
	case "", "no_op", "noop", "no-op", "inspect", "control":
		return false
	default:
		return true
	}
}

// Source reads return current authority plus its exact version. Text is input
// to semantic assessment, never copied into a second fact authority.
func readGoalSourceWith(ctx context.Context, q lifeContextQuerier, eventID int64, lock bool) (GoalSource, error) {
	var source GoalSource
	var profile, conversation, subject *string
	var status string
	if err := q.QueryRow(ctx, `SELECT id,fluctlight_id,source_kind,source_id,source_version,profile_id,conversation_id,subject_actor_id,occurred_at,recorded_at,source_status FROM public.goal_source_events WHERE id=$1`, eventID).Scan(&source.EventID, &source.FluctlightID, &source.Kind, &source.ID, &source.Version, &profile, &conversation, &subject, &source.OccurredAt, &source.RecordedAt, &status); err != nil {
		return source, err
	}
	source.Ref = "source:" + strconv.FormatInt(eventID, 10)
	if profile != nil {
		source.ProfileID = *profile
	}
	if conversation != nil {
		source.ConversationID = *conversation
	}
	if subject != nil {
		source.SubjectActorID = *subject
	}
	source.Data = map[string]any{}
	source.Valid = status == "committed"
	share := ""
	if lock {
		share = " FOR SHARE"
	}
	switch source.Kind {
	case "message":
		var text, kind, author, version, conversationID string
		err := q.QueryRow(ctx, `SELECT text,kind,author_actor_id,md5(text || attachment_refs::text),conversation_id FROM public.conversation_messages WHERE id=$1`+share, source.ID).Scan(&text, &kind, &author, &version, &conversationID)
		if errors.Is(err, pgx.ErrNoRows) {
			source.Valid = false
			return source, nil
		}
		if err != nil {
			return source, err
		}
		source.Valid = source.Valid && version == source.Version && author == source.SubjectActorID && conversationID == source.ConversationID
		var member bool
		if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id=$2)`, conversationID, source.FluctlightID).Scan(&member); err != nil {
			return source, err
		}
		source.Valid = source.Valid && member
		source.CanSupportSuccess = source.Valid && (kind == "user" || kind == "assistant")
		if source.Valid {
			participants := []string{}
			rows, err := q.Query(ctx, `SELECT actor_id FROM public.conversation_participants WHERE conversation_id=$1 ORDER BY actor_id LIMIT 32`, conversationID)
			if err != nil {
				return source, err
			}
			for rows.Next() {
				var actor string
				if err := rows.Scan(&actor); err != nil {
					rows.Close()
					return source, err
				}
				participants = append(participants, actor)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return source, err
			}
			source.Data = map[string]any{"participants": participants, "text": text, "message_kind": kind, "author_actor_id": author, "conversation_id": conversationID}
		}
	case "outcome":
		var outcome ActionOutcome
		var rawGoal, rawIntention, rawRefs, rawObserved, rawExpected []byte
		err := q.QueryRow(ctx, `SELECT fluctlight_id,status,revision,observed,expected,goal_refs,intention_refs,context_references,capability_name,success_boundary,COALESCE(completion_boundary,''),COALESCE(external_ref,'') FROM public.cognition_action_outcomes WHERE id=$1`+share, source.ID).Scan(&outcome.FluctlightID, &outcome.Status, &outcome.Revision, &rawObserved, &rawExpected, &rawGoal, &rawIntention, &rawRefs, &outcome.CapabilityName, &outcome.SuccessBoundary, &outcome.CompletionBoundary, &outcome.ExternalRef)
		if errors.Is(err, pgx.ErrNoRows) {
			source.Valid = false
			return source, nil
		}
		if err != nil {
			return source, err
		}
		source.Valid = source.Valid && outcome.FluctlightID == source.FluctlightID && strconv.Itoa(outcome.Revision) == source.Version
		refs, err := actionOutcomeContextReferences(decodeObject(rawRefs))
		if err != nil {
			return source, err
		}
		for _, ref := range decisionServiceRefValues(decodeArray(rawGoal)) {
			if entry, ok := refs[ref]; ok && entry.Kind == ContextReferenceGoal {
				source.GoalIDs = append(source.GoalIDs, entry.EntityID)
			}
		}
		source.CanSupportSuccess = source.Valid && outcome.Status == ActionOutcomeCompleted
		switch outcome.CapabilityName {
		case goalEvaluationSubmit, goalObjectSubmit, goalPlanSubmit, "goal.decide", "goal.evaluate", "goal.review", intentionDecideCapabilityName, scheduleActivityCapabilityName, scheduleEditCapabilityName, "schedule.replan":
			source.CanSupportSuccess = false // plan/governance acknowledgement is not a business result
		}

		source.Data = map[string]any{"status": outcome.Status, "capability": outcome.CapabilityName, "success_boundary": outcome.SuccessBoundary, "completion_boundary": outcome.CompletionBoundary, "external_ref": outcome.ExternalRef, "observed": decodeObject(rawObserved), "expected": decodeObject(rawExpected)}
		verified := []string{}
		for _, rawID := range arrayValue(decodeObject(rawObserved)["item_ids"]) {
			id := stringValue(rawID)
			var owned bool
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_wardrobe_items i WHERE id=$1 AND fluctlight_id=$2 AND availability='available' AND ownership='owned' AND source_kind='purchase_result' AND `+inventorySourceVerifiedSQL+`)`, id, source.FluctlightID).Scan(&owned); err != nil {
				return source, err
			}
			if owned {
				verified = append(verified, id)
			}
		}
		source.Data["verified_item_ids"] = verified

	case "item":
		var owner, status, fingerprint string
		var raw []byte
		err := q.QueryRow(ctx, `SELECT fluctlight_id,availability,to_jsonb(w),md5(to_jsonb(w)::text) FROM public.fluctlight_wardrobe_items w WHERE id=$1`+share, source.ID).Scan(&owner, &status, &raw, &fingerprint)
		if errors.Is(err, pgx.ErrNoRows) {
			source.Valid = false
			return source, nil
		}
		if err != nil {
			return source, err
		}
		source.Valid = source.Valid && owner == source.FluctlightID && status == "available" && fingerprint == source.Version
		var verified bool
		if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_wardrobe_items i WHERE id=$1 AND fluctlight_id=$2 AND `+inventorySourceVerifiedSQL+`)`, source.ID, source.FluctlightID).Scan(&verified); err != nil {
			return source, err
		}
		source.CanSupportSuccess = source.Valid && verified && stringValue(decodeObject(raw)["ownership"]) == "owned"
		source.Data = map[string]any{"item": decodeObject(raw)}
	case "activity":
		var owner, state string
		var revision int
		var raw []byte
		err := q.QueryRow(ctx, `SELECT fluctlight_id,status,revision,result_json FROM public.fluctlight_life_activity_runs WHERE id=$1`+share, source.ID).Scan(&owner, &state, &revision, &raw)
		if errors.Is(err, pgx.ErrNoRows) {
			source.Valid = false
			return source, nil
		}
		if err != nil {
			return source, err
		}
		source.Valid = source.Valid && owner == source.FluctlightID && strconv.Itoa(revision) == source.Version
		source.CanSupportSuccess = source.Valid && state == "completed"
		source.Data = map[string]any{"status": state, "result": decodeObject(raw)}
	case "actor_fact":
		var owner, state string
		var revision int
		var raw []byte
		err := q.QueryRow(ctx, `SELECT owner_fluctlight_id,status,revision,to_jsonb(f) FROM public.actor_facts f WHERE id=$1`+share, source.ID).Scan(&owner, &state, &revision, &raw)
		if errors.Is(err, pgx.ErrNoRows) {
			source.Valid = false
			return source, nil
		}
		if err != nil {
			return source, err
		}
		source.Valid = source.Valid && owner == source.FluctlightID && state == "active" && strconv.Itoa(revision) == source.Version
		source.CanSupportSuccess = source.Valid
		source.Data = map[string]any{"fact": decodeObject(raw)}
		if stringValue(decodeObject(raw)["epistemic_kind"]) == "inference" {
			source.CanSupportSuccess = false
		}
	case "goal_revision":
		source.CanSupportSuccess = false
		source.Data = map[string]any{"reason": "Goal activated or standards revised; planning/reevaluation required"}
	default:
		return source, errors.New("goal_source_kind_invalid")
	}
	return source, nil
}

func (a *App) enqueueTurnGoalCandidatesTx(ctx context.Context, tx pgx.Tx, owner, inboxID string, projection ContextProjection, structured map[string]any) error {
	candidates := arrayValue(structured["goal_event_candidates"])
	if len(candidates) > 8 {
		return errors.New("goal_event_candidates_limit")
	}
	goalIDs := []string{}
	for _, raw := range candidates {
		candidate := mapValue(raw)
		ref := strings.TrimSpace(stringValue(candidate["goal_ref"]))
		entry, ok := projection.ReferenceIndex.ByRef[ref]
		if !ok || entry.Kind != ContextReferenceGoal {
			return errors.New("goal_event_candidate_scope_invalid")
		}
		goalIDs = mergeStableRefs(goalIDs, []string{entry.EntityID})
	}
	if len(goalIDs) == 0 {
		return nil
	}
	profile := projection.ReferenceIndex.ActiveProfileID
	if profile == "" {
		var err error
		profile, err = (&intentionService{}).resolveProfile(ctx, tx, owner, "")
		if err != nil {
			return err
		}
	}
	linked := []string{}
	for _, goalID := range goalIDs {
		command, err := tx.Exec(ctx, `INSERT INTO public.goal_evidence_links(id,fluctlight_id,goal_id,source_event_id,status,reason) SELECT 'goal_candidate_'||md5($3::text||':'||e.id::text),$1::text,$3::text,e.id,'candidate','current cognition proposed related actual conversation' FROM public.goal_source_events e JOIN public.conversation_messages m ON m.id=e.source_id WHERE e.fluctlight_id=$1 AND e.source_kind='message' AND m.source_fact_id=$2 AND e.source_status='committed' AND (e.profile_id IS NULL OR e.profile_id=$4) ON CONFLICT(goal_id,source_event_id) DO NOTHING`, owner, inboxID, goalID, profile)
		if err != nil {
			return err
		}
		if command.RowsAffected() > 0 {
			linked = append(linked, goalID)
		}
	}
	if len(linked) == 0 {
		return nil
	}
	_, err := queueGoalEvaluationTx(ctx, tx, owner, profile, "conversation_candidate", inboxID, linked)
	return err
}

const pendingLinkedGoalSourceSQL = `SELECT EXISTS(
 SELECT 1 FROM public.goal_source_events e
 WHERE e.fluctlight_id=$1 AND ($2='' OR e.profile_id IS NULL OR e.profile_id=$2) AND e.processed_at IS NULL
 AND EXISTS(SELECT 1 FROM public.fluctlight_goals g
  WHERE g.fluctlight_id=e.fluctlight_id AND (g.profile_id IS NULL OR g.profile_id=$2) AND g.status IN ('active','paused')
  AND ((e.source_kind='goal_revision' AND e.source_id=g.id)
   OR EXISTS(SELECT 1 FROM public.goal_evidence_links l WHERE l.goal_id=g.id AND l.source_event_id=e.id AND l.status IN ('candidate','confirmed','withdrawn'))))
)`
