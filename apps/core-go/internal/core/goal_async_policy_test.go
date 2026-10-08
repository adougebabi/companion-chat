package core

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestAsyncAggregatePolicyFailureOutranksTransientSibling(t *testing.T) {
	f, intentionID, due, activityID, _ := startDueShoppingThroughNative(t)
	var actionID string
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT action_id FROM public.cognition_action_outcomes WHERE external_ref=$1`, activityID).Scan(&actionID); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload->'due_context' FROM public.cognition_inbox WHERE id=$1`, due["inbox_id"]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	frozen := decodeObject(raw)
	results := []CapabilityResult{
		{CallID: "a-transient", CapabilityName: lifeActivityAdvanceCapabilityName, Status: "failed", ErrorCode: "transient_failure"},
		{CallID: "z-policy", CapabilityName: lifeActivityAdvanceCapabilityName, Status: "rejected", ErrorCode: "policy_denied"},
	}
	outcomes, err := buildActionOutcomes(actionID, f.fluctlightID, stringValue(due["inbox_id"]), "no_op", results, map[string]any{"status": "failed", "goal_refs": []string{stringValue(frozen["goal_ref"])}, "intention_refs": []string{stringValue(frozen["intention_ref"])}, "context_references": frozen["context_references"]}, f.app.capabilityRegistry(), f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		// The real started operation still owns the already-persisted pending primary.
		if err := persistActionOutcomesTx(f.ctx, tx, outcomes[1:]); err != nil {
			return err
		}
		_, err := f.app.settleActionOutcomeByExternalRefTx(f.ctx, tx, activityID, ActionOutcomeCompleted, map[string]any{"result": "actual started operation completed"}, "")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var state, reason, outcomeCode, attemptCode string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT i.status,i.retry_reason,o.error_code,a.result->'attempt'->>'error_code' FROM public.fluctlight_intentions i JOIN public.fluctlight_intention_attempts a ON a.attempt_id=i.current_attempt_id JOIN public.cognition_action_outcomes o ON o.id=a.outcome_id WHERE i.id=$1`, intentionID).Scan(&state, &reason, &outcomeCode, &attemptCode); err != nil {
		t.Fatal(err)
	}
	if state != "paused" || reason != "retry_stopped:policy_denied" || outcomeCode != "policy_denied" || attemptCode != "policy_denied" {
		t.Fatalf("async policy failure hidden: %s %s %s %s", state, reason, outcomeCode, attemptCode)
	}
}
