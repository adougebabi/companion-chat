package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func applyVirtualActivityResultTx(ctx context.Context, tx pgx.Tx, app *App, invocation CapabilityInvocation, activityID, fluctlightID, profileID, intentionID, kind string, revision int, result map[string]any) (CapabilityResult, error) {
	status := stringValue(result["status"])
	now := time.Now().UTC()
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
			"activity:"+activityID, "activity-deferred:"+activityID+":"+fmt.Sprint(revision+1), map[string]any{"activity_id": activityID, "revision": revision + 1, "not_before": nextDue.Format(time.RFC3339Nano)}); err != nil {
			return failedCapabilityResult(invocation, "activity_event_failed", true), err
		}
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "accepted",
			Output:            map[string]any{"activity_id": activityID, "status": "deferred", "not_before": nextDue.Format(time.RFC3339Nano)},
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
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,activity,status,revision,evidence_refs,idempotency_key,request_digest,result) VALUES($1,$2,$3,$4,$5,$6,'confirmed',1,$7,$8,$9,$10)`, eventID, fluctlightID, kind, startedAt, now, activityLabel, jsonBytes([]any{"activity:" + activityID}), "activity-result:"+activityID, requestDigest, jsonBytes(eventResult)); err != nil {
		return failedCapabilityResult(invocation, "activity_event_insert_failed", true), err
	}
	output := map[string]any{"activity_id": activityID, "event_id": eventID, "status": status, "reason": result["reason"]}
	if status == "completed" {
		switch kind {
		case "virtual_shopping":
			itemID, wardrobeRevision, err := grantVirtualPurchaseItemTx(ctx, tx, fluctlightID, eventID, mapValue(result["acquired_item"]))
			if err != nil {
				return failedCapabilityResult(invocation, "purchase_item_invalid", true), err
			}
			output["item_id"], output["wardrobe_revision"] = itemID, wardrobeRevision
		case "haircut":
			bodyRevision, err := applyHaircutResultTx(ctx, tx, fluctlightID, eventID, result)
			if err != nil {
				return failedCapabilityResult(invocation, "haircut_body_update_failed", true), err
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
	settlement := map[string]any{"status": status, "resulting_state_ref": eventID, "reason_code": "virtual_activity_" + status}
	outcomes, err := buildActionOutcomes(activityID, fluctlightID, eventID, kind, nil, settlement, nil)
	if err != nil {
		return failedCapabilityResult(invocation, "activity_outcome_invalid", true), err
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
		if status == "failed" {
			outcomeStatus, errorCode = ActionOutcomeFailed, "virtual_activity_failed"
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
			if status == "completed" {
				operation = IntentionComplete
			}
			if _, err := transitionLinkedIntentionTx(ctx, tx, fluctlightID, profileID, intentionID, operation,
				"outcome:"+outcomes[0].ID, "virtual activity "+status, "activity-intention:"+activityID+":"+status, now); err != nil {
				return failedCapabilityResult(invocation, "activity_intention_settlement_failed", true), err
			}
			output["intention_id"] = intentionID
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
	if err := verifyCompletedActivityEventTx(ctx, tx, fluctlightID, eventID, "virtual_shopping"); err != nil {
		return "", 0, err
	}
	category, slot, description := strings.TrimSpace(stringValue(item["category"])), strings.TrimSpace(stringValue(item["slot"])), strings.TrimSpace(stringValue(item["description"]))
	if category == "" || slot == "" || description == "" {
		return "", 0, ErrInvalidArguments
	}
	itemID := "wardrobe_" + stableDigest(fluctlightID+"\x1f"+eventID+"\x1fprimary")
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key) VALUES($1,$2,$3,$4,$5,'owned','available','purchase_result',$6,'primary')`, itemID, fluctlightID, category, slot, description, eventID); err != nil {
		return "", 0, err
	}
	var wardrobeRevision int
	if err := tx.QueryRow(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 RETURNING revision`, fluctlightID).Scan(&wardrobeRevision); err != nil {
		return "", 0, err
	}
	return itemID, wardrobeRevision, nil
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
