package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func ownerPromisePlanningPending(kind string, results []CapabilityResult) bool {
	if kind != "promise" {
		return false
	}
	for _, result := range results {
		if result.CapabilityName == intentionDecideCapabilityName && result.Status == "completed" && mapValue(result.Output)["status"] == "planning_requested" {
			return true
		}
	}
	return false
}

func (a *App) validatePromiseRescheduleTx(ctx context.Context, tx pgx.Tx, owner, fid string, body map[string]any) error {
	p, err := readPromiseLinkTx(ctx, tx, owner, fid, stringValue(body["promise_id"]), true)
	if err != nil {
		return err
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok || intValue(p["revision"]) != expected {
		return conflict("promise", expected, intValue(p["revision"]))
	}
	if p["cancelled_at"] != nil || stringValue(p["intention_id"]) == "" {
		return newCapabilityError("promise_not_reschedulable", false, ErrConflict)
	}
	end, e := time.Parse(time.RFC3339Nano, stringValue(body["due_at"]))
	start, e2 := time.Parse(time.RFC3339Nano, stringValue(body["starts_at"]))
	if e != nil || e2 != nil || !end.After(start) || !end.After(a.now()) {
		return invalid("a future promise window with explicit offsets is required")
	}
	if body["intention_id"] != p["intention_id"] || body["commitment_id"] != p["commitment_id"] {
		return ErrUnauthorized
	}
	var cRevision, iRevision int
	var state, iState string
	if err := tx.QueryRow(ctx, `SELECT c.revision,c.status,i.revision,i.status FROM public.goal_commitments c JOIN public.fluctlight_intentions i ON i.commitment_id=c.id AND i.fluctlight_id=c.fluctlight_id WHERE c.id=$1 AND c.fluctlight_id=$2 AND i.id=$3`, p["commitment_id"], fid, p["intention_id"]).Scan(&cRevision, &state, &iRevision, &iState); err != nil {
		return err
	}
	if intValue(body["expected_commitment_revision"]) != cRevision || intValue(body["expected_intention_revision"]) != iRevision || state == "completed" || state == "cancelled" || iState == "completed" || iState == "cancelled" || iState == "expired" || iState == "in_progress" {
		return newCapabilityError("promise_execution_changed", false, ErrConflict)
	}
	active, err := (&intentionService{}).resolveProfile(ctx, tx, fid, "")
	if err != nil {
		return err
	}
	if active != p["profile_id"] {
		return newCapabilityError("promise_profile_changed", false, ErrConflict)
	}
	return nil
}

func (a *App) applyPromiseRescheduleTx(ctx context.Context, tx pgx.Tx, owner, fid string, body map[string]any, inv CapabilityInvocation) (CapabilityResult, error) {
	p, err := readPromiseLinkTx(ctx, tx, owner, fid, stringValue(body["promise_id"]), true)
	if err != nil {
		return failedCapabilityResult(inv, "promise_not_found", false), err
	}
	var revision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2`, p["goal_id"], fid).Scan(&revision); err != nil {
		return failedCapabilityResult(inv, "promise_goal_unavailable", false), err
	}
	parent, err := loadGoalAuthorityTx(ctx, tx, fid, "goal:ctx_"+stableDigest(stringValue(p["goal_id"])), ContextReference{EntityID: stringValue(p["goal_id"]), Revision: revision})
	if err != nil {
		return failedCapabilityResult(inv, "promise_goal_unavailable", false), err
	}
	if parent.Status != GoalActive {
		return failedCapabilityResult(inv, "promise_goal_not_active", false), ErrConflict
	}
	_, commitments, err := readGoalObjectsWith(ctx, tx, fid, parent.EntityID)
	if err != nil {
		return failedCapabilityResult(inv, "promise_commitment_unavailable", true), err
	}
	var current *GoalCommitment
	for i := range commitments {
		if commitments[i].ID == p["commitment_id"] {
			current = &commitments[i]
		}
	}
	if current == nil {
		return failedCapabilityResult(inv, "promise_commitment_unavailable", false), ErrNotFound
	}
	criteria := []string{}
	for _, c := range current.Criteria {
		criteria = append(criteria, c.Text)
	}
	start, _ := time.Parse(time.RFC3339Nano, stringValue(body["starts_at"]))
	end, _ := time.Parse(time.RFC3339Nano, stringValue(body["due_at"]))
	at := a.now().UTC()
	if err := applyGoalCommitmentPlanTx(ctx, tx, parent, GoalCommitmentPlan{OwnerPromiseID: stringValue(p["request_id"]), Operation: "adjust", ID: current.ID, ExpectedRevision: current.Revision, Reason: stringValue(body["reason"]), ExpectedResult: current.ExpectedResult, Criteria: criteria, WindowStart: &start, WindowEnd: &end, OpportunityCondition: current.OpportunityCondition, Blocker: current.Blocker}, "promise-reschedule:"+stringValue(body["_request_id"]), at); err != nil {
		return failedCapabilityResult(inv, "promise_window_rejected", false), err
	}
	id := stringValue(p["intention_id"])
	old, err := loadIntentionAuthorityTx(ctx, tx, fid, "intention:ctx_"+stableDigest(id), parent.Ref, ContextReference{EntityID: id, Revision: intValue(body["expected_intention_revision"])})
	if err != nil {
		return failedCapabilityResult(inv, "promise_intention_stale", false), err
	}
	// This changes the deadline. It does not invent a new schedule interval, move
	// the character, or pretend to undo an activity that has already occurred.
	next, record, err := ApplyIntentionCommand(old, IntentionCommand{Operation: IntentionUpdate, ExpectedRevision: old.Revision, Patch: promiseWindowPatch(old, start, end), Reason: stringValue(body["reason"]), EvidenceRefs: []string{inv.SourceFactID}, OccurredAt: at})
	if err != nil {
		return failedCapabilityResult(inv, "promise_intention_rejected", false), err
	}
	if _, err := persistIntentionAuthorityTx(ctx, tx, &old, next, record, "promise-reschedule:"+stringValue(body["_request_id"])); err != nil {
		return failedCapabilityResult(inv, "promise_intention_write_failed", true), err
	}
	if err := syncIntentionTriggerWorkflowTx(ctx, tx, next); err != nil {
		return failedCapabilityResult(inv, "promise_trigger_failed", true), err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.owner_promise_links SET window_start=$2,window_end=$3,revision=revision+1,updated_at=now() WHERE request_id=$1`, p["request_id"], start, end); err != nil {
		return failedCapabilityResult(inv, "promise_window_write_failed", true), err
	}
	if err := appendPromiseHistoryTx(ctx, tx, stringValue(p["request_id"]), "rescheduled", inv.SourceFactID, stringValue(body["reason"]), p, at); err != nil {
		return failedCapabilityResult(inv, "promise_history_write_failed", true), err
	}
	if _, err := a.enqueueNativeFactTx(ctx, tx, fid, "", stringValue(p["request_id"]), "life.promise.rescheduled", "promise-rescheduled:"+stringValue(body["_request_id"]), map[string]any{"promise_request_id": p["request_id"], "intention_id": id, "window_start": start, "window_end": end}); err != nil {
		return failedCapabilityResult(inv, "promise_schedule_review_failed", true), err
	}
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "completed", Output: map[string]any{"promise_request_id": p["request_id"], "intention_id": id, "commitment_id": current.ID, "status": "rescheduled", "window_start": start, "window_end": end}, ProviderRequestID: inv.ProviderRequestID}, nil
}

func (a *App) OwnerPromiseCommand(ctx context.Context, owner, fid, id, op string, body map[string]any) (map[string]any, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("promise_transaction_required")
	}
	if err := lockLifeContextTx(ctx, tx, fid); err != nil {
		return nil, err
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	p, err := readPromiseLinkTx(ctx, tx, owner, fid, id, true)
	if err != nil {
		return nil, err
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok || expected != intValue(p["revision"]) {
		return nil, conflict("promise", expected, intValue(p["revision"]))
	}
	reason := strings.TrimSpace(stringValue(body["reason"]))
	if reason == "" || len([]rune(reason)) > 500 {
		return nil, invalid("promise reason required")
	}
	if p["cancelled_at"] != nil {
		return nil, newCapabilityError("promise_already_cancelled", false, ErrConflict)
	}
	audit, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, fid, "promise_"+op, reason, id)
	if err != nil {
		return nil, err
	}
	switch op {
	case "reschedule":
		body = cloneMap(body)
		body["promise_id"], body["intention_id"], body["commitment_id"] = id, p["intention_id"], p["commitment_id"]
		if body["starts_at"] == nil || body["starts_at"] == "" {
			body["starts_at"] = p["window_start"]
		}
		var cRevision, iRevision int
		if err := tx.QueryRow(ctx, `SELECT c.revision,i.revision FROM public.goal_commitments c JOIN public.fluctlight_intentions i ON i.commitment_id=c.id WHERE c.id=$1 AND c.fluctlight_id=$2 AND i.id=$3`, p["commitment_id"], fid, p["intention_id"]).Scan(&cRevision, &iRevision); err != nil {
			return nil, err
		}
		body["expected_commitment_revision"], body["expected_intention_revision"] = cRevision, iRevision
		request, err := NewOwnerSurface(nil, a.surfaceHooks()).insertRequest(ctx, postgresSurfaceTx{tx}, owner, fid, "promise_reschedule", body)
		if err != nil {
			return nil, err
		}
		return withStatus(202, map[string]any{"request": request.Payload, "audit_id": request.Audit}), nil
	case "cancel":
		if stringValue(p["commitment_id"]) != "" {
			var cRevision, gRevision int
			var cState, iState string
			if err := tx.QueryRow(ctx, `SELECT c.revision,c.status,g.revision,i.status FROM public.goal_commitments c JOIN public.fluctlight_goals g ON g.id=c.goal_id JOIN public.fluctlight_intentions i ON i.id=$3 AND i.fluctlight_id=c.fluctlight_id WHERE c.id=$1 AND c.fluctlight_id=$2`, p["commitment_id"], fid, p["intention_id"]).Scan(&cRevision, &cState, &gRevision, &iState); err != nil {
				return nil, err
			}
			if cState == "completed" {
				return nil, newCapabilityError("promise_already_fulfilled", false, ErrConflict)
			}
			if !intentionStatusTerminal(IntentionLifecycleStatus(iState)) {
				if _, err := transitionLinkedIntentionTx(ctx, tx, fid, stringValue(p["profile_id"]), stringValue(p["intention_id"]), IntentionCancel, audit, reason, "promise-cancel:"+audit, a.now()); err != nil {
					return nil, err
				}
			}
			if cState != "cancelled" {
				parent, err := loadGoalAuthorityTx(ctx, tx, fid, "goal:ctx_"+stableDigest(stringValue(p["goal_id"])), ContextReference{EntityID: stringValue(p["goal_id"]), Revision: gRevision})
				if err != nil {
					return nil, err
				}
				if err := applyGoalCommitmentPlanTx(ctx, tx, parent, GoalCommitmentPlan{OwnerPromiseID: id, Operation: "abandon", ID: stringValue(p["commitment_id"]), ExpectedRevision: cRevision, Reason: reason}, audit, a.now()); err != nil {
					return nil, err
				}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.owner_promise_links SET cancelled_at=$2,revision=revision+1,updated_at=now() WHERE request_id=$1`, id, a.now().UTC()); err != nil {
			return nil, err
		}
		if err := appendPromiseHistoryTx(ctx, tx, id, "cancelled", audit, reason, p, a.now()); err != nil {
			return nil, err
		}
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 FOR UPDATE`, id).Scan(&raw); err != nil {
			return nil, err
		}
		root := decodeObject(raw)
		result := map[string]any{"request_id": id, "status": "cancelled", "reason": reason, "source_ref": audit}
		root["status"], root["result"], root["revision"], root["updated_at"], root["decision_reason"], root["decided_by"] = "cancelled", result, intValue(root["revision"])+1, a.now().UTC().Format(time.RFC3339Nano), reason, "owner"
		if _, err := tx.Exec(ctx, `UPDATE public.owner_surface_records SET status='cancelled',payload=$2,revision=revision+1,updated_at=now() WHERE id=$1`, id, jsonBytes(root)); err != nil {
			return nil, err
		}
		if err := appendOwnerRequestNotificationTx(ctx, tx, fid, id, "promise", "cancelled", audit); err != nil {
			return nil, err
		}
		// Cancelling participation never cancels a shared Goal or deletes history.
		if _, err := a.enqueueNativeFactTx(ctx, tx, fid, "", audit, "life.promise.cancelled", "promise-cancel:"+audit, map[string]any{"promise_request_id": id, "intention_id": p["intention_id"], "reason": reason}); err != nil {
			return nil, err
		}
	case "retry":
		if stringValue(p["intention_id"]) != "" {
			return nil, newCapabilityError("promise_already_accepted", false, ErrConflict)
		}
		end, _ := time.Parse(time.RFC3339Nano, stringValue(p["window_end"]))
		if !end.After(a.now()) {
			return nil, newCapabilityError("promise_window_ended", false, ErrConflict)
		}
		if stringValue(p["planning_source_key"]) != "" {
			key := "owner:promise-retry:" + audit
			if err := requestGoalPlanningTx(ctx, tx, fid, key, "owner_request", map[string]any{"promise_request_id": id, "profile_id": p["profile_id"], "desired_outcome": p["proposed_goal"], "action": p["proposed_action"], "success_criteria": []string{stringValue(p["expected_result"])}, "window_start": p["window_start"], "window_end": p["window_end"]}); err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.owner_promise_links SET planning_source_key=$2,revision=revision+1,updated_at=now() WHERE request_id=$1`, id, key); err != nil {
				return nil, err
			}
		} else {
			var raw []byte
			if err := tx.QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 FOR UPDATE`, id).Scan(&raw); err != nil {
				return nil, err
			}
			root := decodeObject(raw)
			if root["status"] != "failed" && root["status"] != "deferred" {
				return nil, newCapabilityError("promise_not_retryable", false, ErrConflict)
			}
			fact, err := a.enqueueNativeFactTx(ctx, tx, fid, "", id, ownerLifeRequestFactType, "promise-retry:"+audit, map[string]any{"request_id": id, "owner_actor_id": owner, "kind": "promise", "request": root["payload"]})
			if err != nil {
				return nil, err
			}
			root["status"], root["source_fact_id"], root["result"], root["revision"], root["updated_at"] = "pending", fact, nil, intValue(root["revision"])+1, a.now().UTC().Format(time.RFC3339Nano)
			if _, err := tx.Exec(ctx, `UPDATE public.owner_surface_records SET status='pending',payload=$2,revision=revision+1,updated_at=now() WHERE id=$1`, id, jsonBytes(root)); err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.owner_promise_links SET revision=revision+1,updated_at=now() WHERE request_id=$1`, id); err != nil {
				return nil, err
			}
		}
		if err := appendPromiseHistoryTx(ctx, tx, id, "retry_requested", audit, reason, p, a.now()); err != nil {
			return nil, err
		}
	default:
		return nil, invalid("unknown promise command")
	}
	updated, err := readPromiseLinkTx(ctx, tx, owner, fid, id, false)
	if err != nil {
		return nil, err
	}
	return map[string]any{"promise": updated, "audit_id": audit, "operation": op}, nil
}

func promiseWindowPatch(old IntentionAuthority, start, end time.Time) IntentionPatch {
	patch := IntentionPatch{PreferredTime: &start, Expiration: &end}
	if old.Trigger.Type == IntentionTriggerTime && old.Trigger.DueAt != nil && (old.Trigger.DueAt.Before(start) || !old.Trigger.DueAt.Before(end)) {
		trigger := TypedIntentionTrigger{Type: IntentionTriggerTime, DueAt: &start}
		patch.Trigger = &trigger
	}
	return patch
}
