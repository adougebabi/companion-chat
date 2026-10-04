package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// A scheduled activity has no confirmed outcome until advance commits one.
// If its Intention was revoked in the meantime, close that pending run before
// any Life Event or appearance effect can be written.
func cancelUnsettledScheduledLifeActivityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, activityID, fluctlightID, intentionID string, revision int, resultRaw []byte, now time.Time) (CapabilityResult, error) {
	currentResult := decodeObject(resultRaw)
	currentResult["result"] = map[string]any{"status": "cancelled", "reason": "scheduled_intention_inactive"}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_life_activity_runs SET status='cancelled',resolved_at=$3,result_json=$4,revision=revision+1 WHERE id=$1 AND fluctlight_id=$2 AND revision=$5`, activityID, fluctlightID, now, jsonBytes(currentResult), revision)
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = ErrConflict
		}
		return failedCapabilityResult(invocation, "activity_cancel_failed", true), err
	}
	if err := appendOutboxTx(ctx, tx, "life.activity.resolved", "life_activity", activityID, fluctlightID, invocation.SourceFactID,
		"activity:"+activityID, "activity-resolved:"+activityID, map[string]any{"activity_id": activityID, "status": "cancelled", "revision": revision + 1}); err != nil {
		return failedCapabilityResult(invocation, "activity_outbox_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"activity_id": activityID, "status": "cancelled", "intention_id": intentionID, "reason": "scheduled_intention_inactive"},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "activity:" + activityID}, nil
}

func applyVirtualActivityResultTx(ctx context.Context, tx pgx.Tx, app *App, invocation CapabilityInvocation, activityID, fluctlightID, profileID, intentionID, kind string, revision int, result map[string]any) (CapabilityResult, error) {
	status := stringValue(result["status"])
	now := app.now().UTC()
	if status == "deferred" {
		nextDue := now.Add(30 * time.Minute)
		command, err := tx.Exec(ctx, `UPDATE public.fluctlight_life_activity_runs SET status='deferred',not_before=$3,result_json=jsonb_set(result_json,'{last_result}',$4::jsonb,true),revision=revision+1 WHERE id=$1 AND fluctlight_id=$2 AND revision=$5`, activityID, fluctlightID, nextDue, jsonBytes(result), revision)
		if err != nil || command.RowsAffected() != 1 {
			if err == nil {
				err = ErrConflict
			}
			return failedCapabilityResult(invocation, "activity_defer_failed", true), err
		}
		if err := appendOutboxTx(ctx, tx, "life.activity.deferred", "life_activity", activityID, fluctlightID, invocation.SourceFactID,
			"activity:"+activityID, "activity-deferred:"+activityID+":"+fmt.Sprint(revision+1), map[string]any{"activity_id": activityID, "revision": revision + 1, "not_before": nextDue.Format(instantLayout)}); err != nil {
			return failedCapabilityResult(invocation, "activity_event_failed", true), err
		}
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "accepted",
			Output:            map[string]any{"activity_id": activityID, "status": "deferred", "not_before": nextDue.Format(instantLayout)},
			ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "activity:" + activityID}, nil
	}
	var startedAt time.Time
	var resultRaw []byte
	if err := tx.QueryRow(ctx, `SELECT started_at,result_json FROM public.fluctlight_life_activity_runs WHERE id=$1 AND fluctlight_id=$2`, activityID, fluctlightID).Scan(&startedAt, &resultRaw); err != nil {
		return failedCapabilityResult(invocation, "activity_read_failed", true), err
	}
	if !now.After(startedAt) {
		return failedCapabilityResult(invocation, "activity_elapsed_time_invalid", false), ErrConflict
	}
	request := mapValue(decodeObject(resultRaw)["request"])
	if err := validateVirtualActivityResult(kind, request, result); err != nil {
		return failedCapabilityResult(invocation, "activity_result_invalid", false), err
	}
	businessCompleted := status == "completed" && (kind != "virtual_shopping" || len(acquiredShoppingItems(result)) > 0)
	if _, err := tx.Exec(ctx, `UPDATE public.life_events SET end_at=LEAST(end_at,$3),expires_at=$3,revision=revision+1,result=jsonb_set(result,'{revision}',to_jsonb(revision+1),true),updated_at=$3 WHERE id=(SELECT authority_event_id FROM public.fluctlight_life_activity_runs WHERE id=$1 AND fluctlight_id=$2) AND fluctlight_id=$2 AND status IN ('confirmed','inferred')`, activityID, fluctlightID, now); err != nil {
		return failedCapabilityResult(invocation, "activity_authority_end_failed", true), err
	}
	eventID := "life_event_" + stableDigest(activityID+"\x1fresult")
	_, lifeBefore, err := resolveLifeContextSnapshotWith(ctx, tx, fluctlightID, now)
	if err != nil {
		return failedCapabilityResult(invocation, "activity_life_context_failed", true), err
	}
	lifeRevision := stringValue(lifeBefore["context_revision"])
	eventResult := map[string]any{"id": eventID, "activity_id": activityID, "kind": kind, "status": "confirmed", "revision": 1,
		"expected_context_revision": lifeRevision, "resulting_context_revision": lifeRevision, "replayed": false, "result": result}
	requestDigest := stableDigest(jsonString(eventResult))
	activityLabel := "虚拟购物"
	if kind == "haircut" {
		activityLabel = "剪发"
	} else if kind == "hair_dye" {
		activityLabel = "染发"
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,activity,status,revision,evidence_refs,idempotency_key,request_digest,result) VALUES($1,$2,$3,$4,$5,$6,'confirmed',1,$7,$8,$9,$10)`, eventID, fluctlightID, kind, startedAt, now, activityLabel, jsonBytes([]any{"activity:" + activityID}), "activity-result:"+activityID, requestDigest, jsonBytes(eventResult)); err != nil {
		return failedCapabilityResult(invocation, "activity_event_insert_failed", true), err
	}
	output := map[string]any{"activity_id": activityID, "event_id": eventID, "status": status, "reason": result["reason"]}
	if status == "completed" {
		switch kind {
		case "virtual_shopping":
			acquired := acquiredShoppingItems(result)
			ids := make([]string, 0, len(acquired))
			for index, item := range acquired {
				key := "primary"
				if len(acquired) > 1 {
					key = fmt.Sprintf("member:%d", index)
				}
				itemID, wardrobeRevision, err := grantVirtualPurchaseMemberTx(ctx, tx, fluctlightID, eventID, key, item)
				if err != nil {
					return failedCapabilityResult(invocation, "purchase_item_invalid", true), err
				}
				ids = append(ids, itemID)
				output["wardrobe_revision"] = wardrobeRevision
			}
			if len(ids) > 0 {
				output["item_ids"] = ids
				output["item_id"] = ids[0]
			}

		case "haircut":
			bodyRevision, err := applyHaircutResultTx(ctx, tx, fluctlightID, eventID, result)
			if err != nil {
				return failedCapabilityResult(invocation, "haircut_body_update_failed", true), err
			}
			output["body_revision"] = bodyRevision
		case "hair_dye":
			bodyRevision, err := applyHairDyeResultTx(ctx, tx, fluctlightID, eventID, result)
			if err != nil {
				return failedCapabilityResult(invocation, "hair_dye_body_update_failed", true), err
			}
			output["body_revision"] = bodyRevision
		default:
			return failedCapabilityResult(invocation, "activity_kind_invalid", false), ErrInvalidArguments
		}
	}
	newRevision := revision + 1
	currentResult := decodeObject(resultRaw)
	currentResult["result"] = result
	currentResult["event_id"] = eventID
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_life_activity_runs SET status=$3,source_event_id=$4,resolved_at=$5,result_json=$6,revision=$7 WHERE id=$1 AND fluctlight_id=$2 AND revision=$8`, activityID, fluctlightID, status, eventID, now, jsonBytes(currentResult), newRevision, revision)
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = ErrConflict
		}
		return failedCapabilityResult(invocation, "activity_settlement_conflict", true), err
	}
	settlementStatus := status
	reasonCode := "virtual_activity_" + status
	if status == "completed" && !businessCompleted {
		settlementStatus, reasonCode = "failed", "virtual_activity_no_acquisition"
	}
	settlement := map[string]any{"status": settlementStatus, "resulting_state_ref": eventID, "reason_code": reasonCode}
	outcomes, err := buildActionOutcomes(activityID, fluctlightID, eventID, kind, nil, settlement, nil, now)
	if err != nil {
		return failedCapabilityResult(invocation, "activity_outcome_invalid", true), err
	}
	var scheduledGoalID string
	if intentionID != "" {
		if err := tx.QueryRow(ctx, `SELECT COALESCE(goal_id,'') FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2`, intentionID, fluctlightID).Scan(&scheduledGoalID); err != nil {
			return failedCapabilityResult(invocation, "activity_goal_read_failed", true), err
		}
		if scheduledGoalID != "" {
			outcomes[0].GoalRefs = []string{"goal:ctx_" + stableDigest(scheduledGoalID)}
		}
	}
	if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
		return failedCapabilityResult(invocation, "activity_outcome_persist_failed", true), err
	}
	// A start Tool may be the pending execution of an intention.due attempt.
	// Resolve its frozen ActionOutcome in the same transaction as the actual
	// body/item effect. The aggregate outcome then settles the due attempt.
	// Independent activities have no such outcome and use the ordinary linked
	// intention transition below.
	if app != nil {
		outcomeStatus := ActionOutcomeCompleted
		errorCode := ""
		if !businessCompleted {
			outcomeStatus, errorCode = ActionOutcomeFailed, reasonCode
		}
		if _, err := app.settleActionOutcomeByExternalRefTx(ctx, tx, activityID, outcomeStatus,
			map[string]any{"event_id": eventID, "resulting_state_ref": eventID}, errorCode); err != nil {
			return failedCapabilityResult(invocation, "activity_prior_outcome_failed", true), err
		}
	}
	if intentionID != "" {
		var attemptSettled, frozenOutcome bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_intention_attempts a JOIN public.fluctlight_intentions i ON i.current_attempt_id=a.attempt_id JOIN public.cognition_action_outcomes o ON o.action_id=a.action_id WHERE o.external_ref=$1 AND i.id=$2 AND i.fluctlight_id=$3),EXISTS(SELECT 1 FROM public.cognition_action_outcomes WHERE external_ref=$1 AND fluctlight_id=$3 AND completion_boundary='virtual_activity_resolved')`, activityID, intentionID, fluctlightID).Scan(&attemptSettled, &frozenOutcome); err != nil {
			return failedCapabilityResult(invocation, "activity_attempt_lookup_failed", true), err
		}
		var linkedStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, intentionID, fluctlightID).Scan(&linkedStatus); err != nil {
			return failedCapabilityResult(invocation, "activity_intention_read_failed", true), err
		}
		if attemptSettled || frozenOutcome || linkedStatus == string(IntentionPaused) || linkedStatus == string(IntentionCancelled) || linkedStatus == string(IntentionExpired) {
			output["intention_id"] = intentionID
		} else {
			operation := IntentionRetry
			if businessCompleted {
				operation = IntentionComplete
			} else if stringValue(request["schedule_item_id"]) != "" {
				operation = IntentionPause
			}
			if _, err := transitionLinkedIntentionTx(ctx, tx, fluctlightID, profileID, intentionID, operation,
				"outcome:"+outcomes[0].ID, "virtual activity "+status, "activity-intention:"+activityID+":"+status, now); err != nil {
				return failedCapabilityResult(invocation, "activity_intention_settlement_failed", true), err
			}
			output["intention_id"] = intentionID
		}
	}
	if businessCompleted && scheduledGoalID != "" {
		if err := completeScheduledGoalTx(ctx, tx, fluctlightID, scheduledGoalID, activityID, outcomes[0], now); err != nil {
			return failedCapabilityResult(invocation, "activity_goal_settlement_failed", true), err
		}
	}
	if err := appendOutboxTx(ctx, tx, "life.activity.resolved", "life_activity", activityID, fluctlightID, eventID,
		"activity:"+activityID, "activity-resolved:"+activityID, map[string]any{"activity_id": activityID, "status": status, "event_id": eventID, "revision": newRevision}); err != nil {
		return failedCapabilityResult(invocation, "activity_outbox_failed", true), err
	}
	toolStatus := "completed"
	errorCode := ""
	if status == "failed" {
		toolStatus, errorCode = "rejected", "virtual_activity_failed"
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: toolStatus, ErrorCode: errorCode,
		Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "activity:" + activityID}, nil
}

func completeScheduledGoalTx(ctx context.Context, tx pgx.Tx, fluctlightID, goalID, activityID string, outcome ActionOutcome, at time.Time) error {
	var revision, openIntentions int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, goalID, fluctlightID).Scan(&revision); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_intentions WHERE goal_id=$1 AND fluctlight_id=$2 AND status NOT IN ('completed','cancelled','expired')`, goalID, fluctlightID).Scan(&openIntentions); err != nil {
		return err
	}
	if openIntentions != 0 {
		return nil
	}
	goalRef := "goal:ctx_" + stableDigest(goalID)
	goal, err := loadGoalAuthorityTx(ctx, tx, fluctlightID, goalRef, ContextReference{EntityID: goalID, Revision: revision})
	if err != nil {
		return err
	}
	if goalStatusTerminal(goal.Status) || goal.Scope == "relationship" || len(goal.SuccessCriteria) != 1 {
		return nil
	}
	next, record, err := ApplyGoalProgress(goal, GoalProgressProposal{
		GoalRef: goal.Ref, OutcomeRefs: []string{outcome.ID}, CriterionIndexes: []int{0},
		Strength: 1, Confidence: 1, Complete: true, EvidenceRefs: []string{"activity:" + activityID}, OccurredAt: at,
	}, map[string]ActionOutcome{outcome.ID: outcome})
	if err != nil {
		return err
	}
	_, err = persistGoalAuthorityTx(ctx, tx, &goal, next, record, "activity-goal:"+activityID)
	return err
}

func verifyCompletedActivityEventTx(ctx context.Context, tx pgx.Tx, fluctlightID, eventID, kind string) error {
	var storedKind, eventStatus, outcomeStatus string
	err := tx.QueryRow(ctx, `SELECT kind,status,result #>> '{result,status}' FROM public.life_events WHERE id=$1 AND fluctlight_id=$2`, eventID, fluctlightID).Scan(&storedKind, &eventStatus, &outcomeStatus)
	if err != nil {
		return err
	}
	if storedKind != kind || eventStatus != "confirmed" || outcomeStatus != "completed" {
		return errors.New("activity_event_result_not_completed")
	}
	return nil
}

func grantVirtualPurchaseItemTx(ctx context.Context, tx pgx.Tx, fluctlightID, eventID string, item map[string]any) (string, int, error) {
	return grantVirtualPurchaseMemberTx(ctx, tx, fluctlightID, eventID, "primary", item)
}
func grantVirtualPurchaseMemberTx(ctx context.Context, tx pgx.Tx, fluctlightID, eventID, key string, item map[string]any) (string, int, error) {
	if err := verifyCompletedActivityEventTx(ctx, tx, fluctlightID, eventID, "virtual_shopping"); err != nil {
		return "", 0, err
	}
	normalized, err := normalizeShoppingItem(item)
	if err != nil {
		return "", 0, err
	}
	itemID := "wardrobe_" + stableDigest(fluctlightID+"\x1f"+eventID+"\x1f"+key)
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,item_kind,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key,acquired_at) SELECT $1::text,$2::text,$3::text,$4::text,$5::text,$6::text,'owned','available','purchase_result',$7::text,$8::text,end_at FROM public.life_events WHERE id=$7::text`, itemID, fluctlightID, normalized["item_kind"], normalized["category"], normalized["slot"], normalized["description"], eventID, key); err != nil {
		return "", 0, err
	}
	var revision int
	if err := tx.QueryRow(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 RETURNING revision`, fluctlightID).Scan(&revision); err != nil {
		return "", 0, err
	}
	return itemID, revision, nil
}

func applyHaircutResultTx(ctx context.Context, tx pgx.Tx, fluctlightID, eventID string, result map[string]any) (int, error) {
	if err := verifyCompletedActivityEventTx(ctx, tx, fluctlightID, eventID, "haircut"); err != nil {
		return 0, err
	}
	var revision int
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT revision,state_json FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision, &raw); err != nil {
		return 0, err
	}
	fields := decodeObject(raw)
	fields["hair_length"] = map[string]any{"status": "known", "value": strings.TrimSpace(stringValue(result["hair_length"]))}
	if color := strings.TrimSpace(stringValue(result["hair_color"])); color != "" {
		fields["hair_color"] = map[string]any{"status": "known", "value": color}
	}
	if style := strings.TrimSpace(stringValue(result["hair_style"])); style != "" {
		fields["hair_style"] = map[string]any{"status": "known", "value": style}
	} else {
		fields["hair_style"] = map[string]any{"status": "cleared"}
	}
	newRevision := revision + 1
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_appearance_states SET revision=$2,state_json=$3,source_kind='activity_result',source_ref=$4,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, newRevision, jsonBytes(fields), eventID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_appearance_revisions(fluctlight_id,revision,state_json,source_kind,source_ref) VALUES($1,$2,$3,'activity_result',$4)`, fluctlightID, newRevision, jsonBytes(fields), eventID); err != nil {
		return 0, err
	}
	return newRevision, nil
}

func applyHairDyeResultTx(ctx context.Context, tx pgx.Tx, fluctlightID, eventID string, result map[string]any) (int, error) {
	if err := verifyCompletedActivityEventTx(ctx, tx, fluctlightID, eventID, "hair_dye"); err != nil {
		return 0, err
	}
	var revision int
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT revision,state_json FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision, &raw); err != nil {
		return 0, err
	}
	fields := decodeObject(raw)
	fields["hair_color"] = map[string]any{"status": "known", "value": strings.TrimSpace(stringValue(result["hair_color"]))}
	next := revision + 1
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_appearance_states SET revision=$2,state_json=$3,source_kind='activity_result',source_ref=$4,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, next, jsonBytes(fields), eventID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_appearance_revisions(fluctlight_id,revision,state_json,source_kind,source_ref) VALUES($1,$2,$3,'activity_result',$4)`, fluctlightID, next, jsonBytes(fields), eventID); err != nil {
		return 0, err
	}
	return next, nil
}
