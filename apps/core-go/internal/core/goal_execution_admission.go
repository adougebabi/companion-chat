package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// All linked execution and governance transactions share the Life lock. It
// serializes local admission with lifecycle changes, not external systems.
func requireIntentionGoalActiveTx(ctx context.Context, tx pgx.Tx, intention IntentionAuthority, at time.Time) error {
	if err := lockLifeContextTx(ctx, tx, intention.FluctlightID); err != nil {
		return err
	}
	if intention.GoalEntityID == "" {
		return errors.New("intention_goal_required")
	}
	var status, profile, policy string
	var deadline *time.Time
	if err := tx.QueryRow(ctx, `SELECT status,COALESCE(profile_id,''),deadline,deadline_policy FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, intention.GoalEntityID, intention.FluctlightID).Scan(&status, &profile, &deadline, &policy); err != nil {
		return err
	}
	if status != string(GoalActive) {
		return newCapabilityError("goal_not_active", false, ErrConflict)
	}
	if profile != "" && profile != intention.ProfileID {
		return newCapabilityError("intention_goal_scope_invalid", false, ErrUnauthorized)
	}
	if policy == "hard" && deadline != nil && !at.Before(*deadline) {
		return newCapabilityError("goal_hard_deadline_elapsed", false, ErrConflict)
	}
	if !at.Before(intention.Expiration) {
		return newCapabilityError("intention_expired", false, ErrConflict)
	}
	var next *time.Time
	if err := tx.QueryRow(ctx, `SELECT next_attempt_at FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2`, intention.EntityID, intention.FluctlightID).Scan(&next); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if next != nil && at.Before(*next) {
		return newCapabilityError("intention_retry_not_due", true, ErrConflict)
	}
	return nil
}

func (a *App) admitGoalLinkedToolTx(ctx context.Context, tx pgx.Tx, request ToolExecutionRequest) error {
	// Result queries/settlement accept real late facts even while a Goal is held.
	// Lifecycle commands must remain available to pause/cancel a held intention.
	if request.CapabilityName == lifeActivityAdvanceCapabilityName || request.CapabilityName == intentionDecideCapabilityName {
		return nil
	}
	definition, ok := a.capabilityRegistry().Definition(request.CapabilityName)
	if !ok || definition.Type == CapabilityTypeQuery {
		return nil
	}
	intentionID := stringValue(decodeObject(request.Arguments)["intention_id"])
	var raw []byte
	var eventType string
	err := tx.QueryRow(ctx, `SELECT event_type,payload FROM public.cognition_inbox WHERE id=$1 AND fluctlight_id=$2`, request.EvidenceID, request.FluctlightID).Scan(&eventType, &raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var due map[string]any
	if err == nil && eventType == intentionDueFactType {
		fact := decodeObject(raw)
		due = mapValue(fact["due_context"])
		if len(due) == 0 {
			return newCapabilityError("intention_due_admission_required", false, ErrConflict)
		}
		if intentionID != "" && intentionID != stringValue(due["intention_id"]) {
			return newCapabilityError("intention_due_identity_invalid", false, ErrConflict)
		}
		intentionID = stringValue(due["intention_id"])
	}
	if intentionID == "" {
		return nil
	}
	current, err := loadIntentionAuthorityByIDTx(ctx, tx, intentionID)
	if err != nil {
		return err
	}
	if current.FluctlightID != request.FluctlightID || (current.ProfileID != "" && request.WorkingProfileID != "" && current.ProfileID != request.WorkingProfileID) {
		return ErrUnauthorized
	}
	if err := requireIntentionGoalActiveTx(ctx, tx, current, a.now().UTC()); err != nil {
		return err
	}
	if current.Trigger.Type == IntentionTriggerTime && current.Trigger.DueAt != nil && a.now().Before(*current.Trigger.DueAt) {
		return newCapabilityError("intention_trigger_not_due", false, ErrConflict)
	}
	if current.Status != IntentionQualified && current.Status != IntentionDue {
		return newCapabilityError("intention_not_executable", false, ErrConflict)
	}
	if len(current.CapabilityConstraints) > 0 && !containsString(current.CapabilityConstraints, request.CapabilityName) {
		return newCapabilityError("intention_capability_not_allowed", false, ErrUnauthorized)
	}
	if len(due) > 0 {
		refs, err := actionOutcomeContextReferences(due["context_references"])
		if err != nil {
			return err
		}
		goalEntry, ok := refs[stringValue(due["goal_ref"])]
		if !ok {
			return ErrConflict
		}
		var revision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2`, current.GoalEntityID, current.FluctlightID).Scan(&revision); err != nil {
			return err
		}
		if revision != goalEntry.Revision {
			return newCapabilityError("goal_execution_revision_stale", false, ErrConflict)
		}
	}
	if len(due) > 0 && (current.Revision != intValue(due["intention_revision"]) || current.LastAttemptID != stringValue(due["attempt_id"])) {
		return newCapabilityError("intention_due_stale", false, ErrConflict)
	}
	if request.NativeToolCallID == "" && request.CapabilityName == lifeActivityStartCapabilityName {
		return a.beginDirectLinkedAttemptTx(ctx, tx, request, current)
	}
	return nil
}

func (a *App) beginDueAttemptTx(ctx context.Context, tx pgx.Tx, fluctlightID, inboxID string, frozen map[string]any) error {
	if err := lockLifeContextTx(ctx, tx, fluctlightID); err != nil {
		return err
	}
	current, err := loadIntentionAuthorityByIDTx(ctx, tx, stringValue(frozen["intention_id"]))
	if err != nil {
		return err
	}
	if current.FluctlightID != fluctlightID || current.Revision != intValue(frozen["intention_revision"]) || current.LastAttemptID != stringValue(frozen["attempt_id"]) || current.Status != IntentionDue {
		return newCapabilityError("intention_due_stale", false, ErrConflict)
	}
	if err := requireIntentionGoalActiveTx(ctx, tx, current, a.now().UTC()); err != nil {
		return err
	}
	actionID := "agent_native_" + stableDigest(inboxID)
	at := a.now().UTC()
	frozen["inbox_id"] = inboxID
	leaseUntil := at.Add(5 * time.Minute)
	if current.Expiration.Before(leaseUntil) {
		leaseUntil = current.Expiration
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_intention_attempts(attempt_id,fluctlight_id,intention_id,intention_ref,goal_ref,action_id,outcome_id,outcome_digest,status,result,occurred_at,started_at,deadline) VALUES($1,$2,$3,$4,$5,$6,$7,'','running',$8,$9,$9,$10) ON CONFLICT(attempt_id) DO NOTHING`, current.LastAttemptID, fluctlightID, current.EntityID, frozen["intention_ref"], frozen["goal_ref"], actionID, "outcome_"+stableDigest(actionID+"\x1f"+actionPrimaryCallID), jsonBytes(frozen), at, leaseUntil)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE public.fluctlight_intention_attempts SET intention_ref=$2,goal_ref=$3,result=$4 WHERE attempt_id=$1 AND status='running'`, current.LastAttemptID, frozen["intention_ref"], frozen["goal_ref"], jsonBytes(frozen))
	}
	return err
}

// The domain chooses retry eligibility. Temporal receives this exact due time;
// it does not independently replay a failed Agent or unknown operation.
func configureIntentionRetryTx(ctx context.Context, tx pgx.Tx, current IntentionAuthority, at time.Time, reason string) error {
	var count int
	if err := tx.QueryRow(ctx, `SELECT retry_count FROM public.fluctlight_intentions WHERE id=$1`, current.EntityID).Scan(&count); err != nil {
		return err
	}
	policy := map[string]any{}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.goal_retry'`).Scan(&raw); err == nil {
		policy = decodeObject(raw)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	maximum := intValue(policy["max_attempts"])
	if maximum < 1 {
		maximum = 5
	}
	if maximum > 20 {
		maximum = 20
	}
	base := intValue(policy["base_seconds"])
	if base < 30 {
		base = 60
	}
	capSeconds := intValue(policy["max_seconds"])
	if capSeconds < base {
		capSeconds = 3600
	}
	delay := time.Duration(base) * time.Second
	for i := 0; i < count && delay < time.Duration(capSeconds)*time.Second; i++ {
		delay *= 2
	}
	if delay > time.Duration(capSeconds)*time.Second {
		delay = time.Duration(capSeconds) * time.Second
	}
	// Stable jitter remains replayable and spreads different attempts by up to25%.
	jitter := int(stableDigest(current.LastAttemptID)[0]) % 26
	next := at.Add(delay + delay*time.Duration(jitter)/100)
	status := "qualified"
	increment := 1
	if reason == "suppressed" {
		increment = 0
	}
	stoppedByPolicy := strings.HasPrefix(reason, "policy_") || strings.HasPrefix(reason, "goal_") || strings.HasPrefix(reason, "intention_permission") || reason == "permission_denied" || reason == "user_refused"
	if (increment > 0 && count+1 >= maximum) || !next.Before(current.Expiration) || stoppedByPolicy {
		status = "paused"
		reason = "retry_stopped:" + reason
	}
	_, err := tx.Exec(ctx, `UPDATE public.fluctlight_intentions SET retry_count=retry_count+$5,next_attempt_at=$2,retry_reason=$3,status=$4 WHERE id=$1`, current.EntityID, next, reason, status, increment)
	return err
}

func (a *App) settleDueAgentFailureTx(ctx context.Context, tx pgx.Tx, inboxID, fluctlightID string, payload []byte, outcome agentCommittedOutcome, code string) error {
	due := mapValue(decodeObject(payload)["due_context"])
	if len(due) == 0 {
		return nil
	}
	actionID := "agent_native_" + stableDigest(inboxID)
	var pending bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_action_outcomes WHERE action_id=$1 AND status='pending' AND external_ref IS NOT NULL)`, actionID).Scan(&pending); err != nil {
		return err
	}
	if pending {
		_, err := tx.Exec(ctx, `UPDATE public.fluctlight_intention_attempts SET status='waiting',wait_ref=(SELECT external_ref FROM public.cognition_action_outcomes WHERE action_id=$2 AND external_ref IS NOT NULL LIMIT 1) WHERE attempt_id=$1 AND status IN ('running','waiting')`, due["attempt_id"], actionID)
		return err
	}
	status, reason := dueActionSettlement(outcome.Results, a.capabilityRegistry())
	if status == "suppressed" {
		status = "failed"
		reason = code
	}
	settlement := map[string]any{"status": status, "reason_code": reason, "error_code": code, "goal_refs": []string{stringValue(due["goal_ref"])}, "intention_refs": []string{stringValue(due["intention_ref"])}, "context_references": due["context_references"]}
	outcomes, err := buildActionOutcomes(actionID, fluctlightID, inboxID, "no_op", outcome.Results, settlement, a.capabilityRegistry(), a.now().UTC())
	if err != nil {
		return err
	}
	return persistActionOutcomesTx(ctx, tx, outcomes)
}

func dueActionSettlement(results []CapabilityResult, registry *CapabilityRegistry) (string, string) {
	for _, r := range results {
		def, ok := registry.Definition(r.CapabilityName)
		if !ok {
			continue
		}
		if r.Status == "accepted" && def.CompletionBoundary != "" && def.OutcomeReferenceField != "" && stringValue(mapValue(r.Output)[def.OutcomeReferenceField]) != "" {
			return "pending", "intention_awaits_verified_result"
		}
	}
	for _, r := range results {
		def, ok := registry.Definition(r.CapabilityName)
		if ok && def.Type == CapabilityTypeAction && r.Status == "completed" && def.SuccessBoundary != "schedule_version_committed" && def.SuccessBoundary != "scheduled_intention_committed" && def.SuccessBoundary != "intention_revision_committed" {
			return "completed", "synchronous_action_committed"
		}
	}
	for _, r := range results {
		if r.Status == "failed" || r.Status == "rejected" {
			return "failed", r.ErrorCode
		}
	}
	return "suppressed", "intention_no_action"
}

// Goal governance invalidates queued work in the same transaction; real running
// operations retain their result owner and are never erased by this cascade.
func cascadeGoalLifecycleTx(ctx context.Context, tx pgx.Tx, before *GoalAuthority, after GoalAuthority, key string, at time.Time) error {
	if before == nil {
		return nil
	}
	standardsChanged := effectiveGoalCriteriaVersion(*before) != effectiveGoalCriteriaVersion(after)
	if before.Status == after.Status && !standardsChanged {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT id,status,COALESCE(goal_hold_status,'') FROM public.fluctlight_intentions WHERE fluctlight_id=$1 AND goal_id=$2 AND status NOT IN ('completed','cancelled','expired') ORDER BY id`, after.FluctlightID, after.EntityID)
	if err != nil {
		return err
	}
	type held struct{ id, status, previous string }
	items := []held{}
	for rows.Next() {
		var h held
		if err := rows.Scan(&h.id, &h.status, &h.previous); err != nil {
			rows.Close()
			return err
		}
		items = append(items, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, h := range items {
		operation := IntentionLifecycleOperation("")
		if (after.Status == GoalPaused || standardsChanged) && (h.status == "qualified" || h.status == "due") {
			operation = IntentionPause
		}
		if goalStatusTerminal(after.Status) {
			operation = IntentionCancel
		}
		if after.Status == GoalActive && h.status == "paused" && h.previous != "" {
			operation = IntentionResume
		}
		if operation == "" {
			continue
		}
		current, err := loadIntentionAuthorityByIDTx(ctx, tx, h.id)
		if err != nil {
			return err
		}
		if operation == IntentionResume && !at.Before(current.Expiration) {
			operation = IntentionExpire
		}
		next, record, err := ApplyIntentionCommand(current, IntentionCommand{Operation: operation, ExpectedRevision: current.Revision, EvidenceRefs: after.EvidenceRefs, Reason: "Goal lifecycle: " + string(after.Status), OccurredAt: at})
		if err != nil {
			return err
		}
		if _, err := persistIntentionAuthorityTx(ctx, tx, &current, next, record, fmt.Sprintf("%s:intention:%s", key, h.id)); err != nil {
			return err
		}
		hold := ""
		if operation == IntentionPause && !standardsChanged {
			hold = h.status
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_intentions SET goal_hold_status=$2 WHERE id=$1`, h.id, nullableString(hold)); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) beginDirectLinkedAttemptTx(ctx context.Context, tx pgx.Tx, request ToolExecutionRequest, current IntentionAuthority) error {
	actionID := "linked_tool_" + stableDigest(request.FluctlightID+"\x1f"+request.CapabilityName+"\x1f"+request.OperationID)
	if current.Status == IntentionQualified {
		next, record, err := ApplyIntentionCommand(current, IntentionCommand{Operation: IntentionMarkDue, ExpectedRevision: current.Revision, EvidenceRefs: []string{request.EvidenceID}, Reason: "direct linked execution admitted", OccurredAt: a.now().UTC()})
		if err != nil {
			return err
		}
		next.LastAttemptID = "intention_attempt_" + stableDigest(current.EntityID+"\x1f"+fmt.Sprint(current.Revision)+"\x1f"+request.OperationID)
		if _, err := persistIntentionAuthorityTx(ctx, tx, &current, next, record, "linked-tool-due:"+actionID); err != nil {
			return err
		}
		current = next
	}
	var existingAction string
	err := tx.QueryRow(ctx, `SELECT action_id FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, current.LastAttemptID).Scan(&existingAction)
	if err == nil {
		if existingAction != actionID {
			return newCapabilityError("intention_attempt_already_owned", false, ErrConflict)
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var goalRevision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2`, current.GoalEntityID, current.FluctlightID).Scan(&goalRevision); err != nil {
		return err
	}
	goalEntry := ContextReference{Ref: current.GoalRef, Kind: ContextReferenceGoal, EntityID: current.GoalEntityID, Revision: goalRevision, Snapshot: jsonBytes(map[string]any{"id": current.GoalEntityID, "revision": goalRevision})}
	intentionEntry := ContextReference{Ref: current.Ref, Kind: ContextReferenceIntention, EntityID: current.EntityID, Revision: current.Revision, Snapshot: jsonBytes(map[string]any{"id": current.EntityID, "goal_ref": current.GoalRef, "current_attempt_id": current.LastAttemptID, "revision": current.Revision})}
	frozen := map[string]any{"intention_id": current.EntityID, "intention_ref": current.Ref, "goal_ref": current.GoalRef, "attempt_id": current.LastAttemptID, "intention_revision": current.Revision, "context_references": map[string]ContextReference{current.Ref: intentionEntry, current.GoalRef: goalEntry}}
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_intention_attempts(attempt_id,fluctlight_id,intention_id,intention_ref,goal_ref,action_id,outcome_id,outcome_digest,status,result,occurred_at,started_at,deadline) VALUES($1,$2,$3,$4,$5,$6,$7,'','running',$8,$9,$9,$10)`, current.LastAttemptID, current.FluctlightID, current.EntityID, current.Ref, current.GoalRef, actionID, "outcome_"+stableDigest(actionID+"\x1f"+actionPrimaryCallID), jsonBytes(frozen), a.now().UTC(), current.Expiration)
	return err
}

func (a *App) persistDirectLinkedOutcomeTx(ctx context.Context, tx pgx.Tx, request ToolExecutionRequest, result CapabilityResult) error {
	if request.NativeToolCallID != "" || stringValue(decodeObject(request.Arguments)["intention_id"]) == "" || request.CapabilityName != lifeActivityStartCapabilityName {
		return nil
	}
	actionID := "linked_tool_" + stableDigest(request.FluctlightID+"\x1f"+request.CapabilityName+"\x1f"+request.OperationID)
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT result FROM public.fluctlight_intention_attempts WHERE action_id=$1 AND fluctlight_id=$2`, actionID, request.FluctlightID).Scan(&raw); err != nil {
		return err
	}
	frozen := decodeObject(raw)
	status, reason := dueActionSettlement([]CapabilityResult{result}, a.capabilityRegistry())
	settlement := map[string]any{"status": status, "reason_code": reason, "goal_refs": []string{stringValue(frozen["goal_ref"])}, "intention_refs": []string{stringValue(frozen["intention_ref"])}, "context_references": frozen["context_references"]}
	outcomes, err := buildActionOutcomes(actionID, request.FluctlightID, request.EvidenceID, "capability", []CapabilityResult{result}, settlement, a.capabilityRegistry(), a.now().UTC())
	if err != nil {
		return err
	}
	return persistActionOutcomesTx(ctx, tx, outcomes)
}

// Claim before projection/model preparation so infrastructure failures before
// the first Tool also have a durable attempt. Later projection binding refines
// the exact served refs without changing the attempt's physical identity.
func (a *App) prepareNativeDueAttempt(ctx context.Context, inboxID, fluctlightID string) (bool, error) {
	stopped := false
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, fluctlightID); err != nil {
			return err
		}
		var payload []byte
		if err := tx.QueryRow(ctx, `SELECT payload FROM public.cognition_inbox WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, inboxID, fluctlightID).Scan(&payload); err != nil {
			return err
		}
		fact := decodeObject(payload)
		candidate := mapValue(fact["candidate"])
		current, err := loadIntentionAuthorityByIDTx(ctx, tx, stringValue(fact["source_fact_id"]))
		if err != nil {
			return err
		}
		if current.FluctlightID != fluctlightID {
			return ErrUnauthorized
		}
		gateErr := requireIntentionGoalActiveTx(ctx, tx, current, a.now().UTC())
		if current.Status != IntentionDue || current.LastAttemptID != stringValue(candidate["attempt_id"]) || current.Revision != intValue(candidate["intention_revision"]) {
			gateErr = errors.New("intention_due_stale")
		}
		if gateErr != nil {
			stopped = true
			code, _ := capabilityErrorInfo(gateErr, "", false)
			if (code == "goal_hard_deadline_elapsed" || code == "intention_expired") && !intentionStatusTerminal(current.Status) {
				next, record, err := ApplyIntentionCommand(current, IntentionCommand{Operation: IntentionExpire, ExpectedRevision: current.Revision, EvidenceRefs: current.EvidenceRefs, Reason: code, OccurredAt: a.now().UTC()})
				if err != nil {
					return err
				}
				if _, err := persistIntentionAuthorityTx(ctx, tx, &current, next, record, "intention-deadline:"+current.EntityID+":"+fmt.Sprint(current.Revision)); err != nil {
					return err
				}
			}
			actionID := "agent_native_" + stableDigest(inboxID)
			reason, _ := capabilityErrorInfo(gateErr, "intention_execution_suppressed", false)
			_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_intention_attempts(attempt_id,fluctlight_id,intention_id,intention_ref,goal_ref,action_id,outcome_id,outcome_digest,status,result,occurred_at,settled_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'suppressed',$9,$10,$10) ON CONFLICT(attempt_id) DO NOTHING`, candidate["attempt_id"], fluctlightID, current.EntityID, candidate["intention_ref"], candidate["goal_ref"], actionID, "admission_"+stableDigest(inboxID), stableDigest(reason), jsonBytes(map[string]any{"settlement_kind": "execution_admission", "reason": reason}), a.now().UTC())
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=now(),claimed_at=NULL,claimed_by=NULL,payload=jsonb_set(payload,'{agent_result}',$2::jsonb,true) WHERE id=$1`, inboxID, jsonBytes(map[string]any{"status": "suppressed", "reason_code": reason})); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE public.cognition_inbox_heads h SET last_processed_sequence=GREATEST(h.last_processed_sequence,i.sequence) FROM public.cognition_inbox i WHERE h.fluctlight_id=i.fluctlight_id AND i.id=$1`, inboxID)
			return err
		}
		var goalRevision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1`, current.GoalEntityID).Scan(&goalRevision); err != nil {
			return err
		}
		goalRef, intentionRef := stringValue(candidate["goal_ref"]), stringValue(candidate["intention_ref"])
		refs := map[string]ContextReference{
			goalRef:      {Ref: goalRef, Kind: ContextReferenceGoal, EntityID: current.GoalEntityID, Revision: goalRevision, Snapshot: jsonBytes(map[string]any{"id": current.GoalEntityID, "revision": goalRevision})},
			intentionRef: {Ref: intentionRef, Kind: ContextReferenceIntention, EntityID: current.EntityID, Revision: current.Revision, Snapshot: jsonBytes(map[string]any{"id": current.EntityID, "revision": current.Revision, "goal_ref": goalRef, "current_attempt_id": current.LastAttemptID})},
		}
		frozen := map[string]any{"intention_id": current.EntityID, "attempt_id": current.LastAttemptID, "intention_revision": current.Revision, "goal_ref": goalRef, "intention_ref": intentionRef, "context_references": refs}
		if err := a.beginDueAttemptTx(ctx, tx, fluctlightID, inboxID, frozen); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public.cognition_inbox SET payload=jsonb_set(payload,'{due_context}',$2::jsonb,true) WHERE id=$1`, inboxID, jsonBytes(frozen))
		return err
	})
	return stopped, err
}
