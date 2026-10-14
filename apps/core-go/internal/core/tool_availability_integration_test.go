package core

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type rollbackAvailabilityCapability struct {
	key             string
	databaseFailure bool
}

func TestToolAvailabilityItemUseNoopAndStopUnavailable(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	added, err := f.app.AddWardrobeItems(f.ctx, f.ownerID, f.fluctlightID, map[string]any{"item_kind": "object", "category": "art_supply", "slot": "", "description": "受控画笔", "ownership": "owned", "availability": "available"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(mapValue(arrayValue(added["items"])[0])["id"])
	for n := 0; n < 2; n++ {
		r, err := f.app.ExecuteTool(f.ctx, f.request(itemUseCapabilityName, fmt.Sprintf("start-%d", n), map[string]any{"operation": "start", "item_id": " " + id + " ", "activity": "画画"}))
		if err != nil || (n == 0 && r.Result.Status != "completed") || (n == 1 && r.Result.Status != "handled") {
			t.Fatal(r, err)
		}
	}
	var started time.Time
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT started_at FROM public.fluctlight_item_uses WHERE fluctlight_id=$1 AND item_id=$2`, f.fluctlightID, id).Scan(&started); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_item_use_events WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_items SET availability='lost' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		r, err := f.app.ExecuteTool(f.ctx, f.request(itemUseCapabilityName, fmt.Sprintf("stop-%d", n), map[string]any{"operation": "stop", "item_id": id}))
		if err != nil || (n == 0 && r.Result.Status != "completed") || (n == 1 && r.Result.Status != "handled") {
			t.Fatal(r, err)
		}
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_item_use_events WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_item_uses WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestToolAvailabilityPersonaNoEffectKeepsProfile(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{personaSwitchCapabilityName, map[string]any{"decision": "keep"}},
		{personaTakeoverCapabilityName, map[string]any{"decision": takeoverDecisionTakeoverB, "rule_id": "not-declared", "target_profile_id": "not-declared"}},
	} {
		r, err := f.app.ExecuteTool(f.ctx, f.request(tc.name, "no-effect-"+tc.name, tc.args))
		if err != nil || r.Result.Status != "handled" || mapValue(r.Result.Output)["effect_applied"] != false || mapValue(r.Result.Output)["audit_id"] != nil {
			t.Fatal(r, err)
		}
	}
	var profile string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&profile); err != nil || profile != "default" {
		t.Fatal(profile, err)
	}
}

func TestToolAvailabilityHabitDuplicateNoop(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	args := map[string]any{"operation": "append", "text": "受控的新习惯", "reason": "明确选择"}
	first, err := f.app.ExecuteTool(f.ctx, f.request(habitDecideCapabilityName, "first-habit", args))
	if err != nil || first.Result.Status != "completed" {
		t.Fatal(first, err)
	}
	second, err := f.app.ExecuteTool(f.ctx, f.request(habitDecideCapabilityName, "same-habit", args))
	if err != nil || second.Result.Status != "handled" {
		t.Fatal(second, err)
	}
	current, err := f.app.ExecuteTool(f.ctx, f.request(habitInspectCapabilityName, "habit-read", map[string]any{}))
	if err != nil || mapValue(current.Result.Output)["revision"] != mapValue(first.Result.Output)["revision"] {
		t.Fatal(current, first, err)
	}
}

func TestToolAvailabilityInvalidProposalKeyIsHandled(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	r, err := f.app.ExecuteTool(f.ctx, f.request("capability.request", "invalid-key", map[string]any{"capability_key": "invalid key with spaces", "title": "受控提案", "description": "受控提案", "rationale": "受控提案"}))
	if err != nil || r.Result.Status != "handled" || goalSubmissionReceiptCode(r.Result) != "invalid_arguments" {
		t.Fatal(r, err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.capability_requests WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestToolAvailabilityHandledClosesPreclaimedDueAttempt(t *testing.T) {
	f, _, due := seedNativeDueForGoalExecution(t)
	if stopped, err := f.app.prepareNativeDueAttempt(f.ctx, stringValue(due["inbox_id"]), f.fluctlightID); err != nil || stopped {
		t.Fatal(stopped, err)
	}
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT result FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, due["attempt_id"]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	frozen := decodeObject(raw)
	result := handledCapabilityResult(CapabilityInvocation{CallID: "no-effect", CapabilityName: wardrobeWearCapabilityName}, "wardrobe_item_unavailable", "awaiting_state", nil)
	actionID := "agent_native_" + stableDigest(stringValue(due["inbox_id"]))
	status, reason := dueActionSettlement([]CapabilityResult{result}, f.app.capabilityRegistry())
	outcomes, err := buildActionOutcomes(actionID, f.fluctlightID, stringValue(due["inbox_id"]), "no_op", []CapabilityResult{result}, map[string]any{"status": status, "reason_code": reason, "goal_refs": []string{stringValue(frozen["goal_ref"])}, "intention_refs": []string{stringValue(frozen["intention_ref"])}, "context_references": frozen["context_references"]}, f.app.capabilityRegistry(), f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return persistActionOutcomesTx(f.ctx, tx, outcomes) }); err != nil {
			t.Fatal(err)
		}
	}
	var attempt, intention string
	var retries int
	var progress float64
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT a.status,i.status,i.retry_count,g.progress FROM public.fluctlight_intention_attempts a JOIN public.fluctlight_intentions i ON i.id=a.intention_id JOIN public.fluctlight_goals g ON g.id=i.goal_id WHERE a.attempt_id=$1`, due["attempt_id"]).Scan(&attempt, &intention, &retries, &progress); err != nil {
		t.Fatal(err)
	}
	if attempt != "suppressed" || intention != "qualified" || retries != 0 || progress != 0 {
		t.Fatal(attempt, intention, retries, progress)
	}
}

func (c rollbackAvailabilityCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{Name: "test.availability.rollback", Version: "v1", Description: "Controlled rollback test", Type: CapabilityTypeAction, InputSchema: objectSchema(nil, nil, false), OutputSchema: openObjectSchema(), Surfaces: []CapabilitySurface{CapabilitySurfaceNativeCognition}, SideEffectClass: "native_projection", SuccessBoundary: "test_write", ConcurrencyClass: "exclusive", FailurePolicy: FailurePolicyOptionalInternal}
}
func (c rollbackAvailabilityCapability) RequiredContext() []ContextSlot { return nil }
func (c rollbackAvailabilityCapability) Execute(_ context.Context, i CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(i)
}
func (c rollbackAvailabilityCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, i CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES($1,'{"provisional":true}')`, c.key); err != nil {
		return CapabilityResult{}, err
	}
	if c.databaseFailure {
		_, err := tx.Exec(ctx, `SELECT nonexistent_availability_column FROM public.runtime_settings`)
		return failedCapabilityResult(i, "actual_database_failure", false), err
	}
	return failedCapabilityResult(i, "test_state_changed", false), ErrConflict
}

func TestToolAvailabilitySavepointRollbackAndReplay(t *testing.T) {
	for _, dependencyFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(dependencyFailure), func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			key := "availability-test-" + f.suffix
			f.app.Capabilities = mustCapabilityRegistry(rollbackAvailabilityCapability{key, dependencyFailure})
			request := f.request("test.availability.rollback", "rollback", map[string]any{})
			receipt, err := f.app.ExecuteTool(f.ctx, request)
			if dependencyFailure {
				if err == nil || receipt.Result.Status == "handled" {
					t.Fatalf("database failure hidden: %#v %v", receipt, err)
				}
			} else {
				if err != nil || receipt.Result.Status != "handled" || mapValue(receipt.Result.Output)["effect_applied"] != false {
					t.Fatalf("no-effect receipt: %#v %v", receipt, err)
				}
				replay, replayErr := f.app.ExecuteTool(f.ctx, request)
				if replayErr != nil || !replay.Replayed || replay.Result.Status != "handled" {
					t.Fatal(replay, replayErr)
				}
			}
			var count int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.runtime_settings WHERE key=$1`, key).Scan(&count); err != nil || count != 0 {
				t.Fatalf("provisional write survived: %d %v", count, err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name=$2`, f.fluctlightID, request.CapabilityName).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if (!dependencyFailure && count != 1) || (dependencyFailure && count != 0) {
				t.Fatalf("incorrect receipt persistence: %d", count)
			}
		})
	}
}

func TestToolAvailabilityMissingReviewDoesNotUndoValidRoot(t *testing.T) {
	for _, completed := range []bool{false, true} {
		t.Run(fmt.Sprint(completed), func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			goalID := createDialogueGoalForClosure(t, f, []string{"实际明确发送表达"})
			message := insertGoalBoundaryMessage(t, f)
			seedCognitiveProviderRole(t, f.ctx, f.repository, "missing-review-"+f.suffix)
			if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				_, err := queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "default", "missing-review", f.app.now())
				return err
			}); err != nil {
				t.Fatal(err)
			}
			calls := 0
			f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
				calls++
				if calls == 1 {
					snapshot := readProcessingGoalSnapshot(t, f)
					args := nativeGoalRootFixture(t, snapshot, goalID, message)
					if !completed {
						binding, _ := newGoalEvaluationWireBinding(snapshot)
						args = map[string]any{"goal_ref": binding.goalRefsByID[goalID], "judgments": []any{}, "impact": "needs_evidence"}
					}
					delete(args, "review")
					return fakeProviderResult{ToolCalls: []map[string]any{{"call_id": "missing-review-root", "capability_name": goalEvaluationSubmit, "arguments": args}}}
				}
				return fakeProviderResult{Structured: map[string]any{"summary": "已消费真实回执"}}
			})}
			if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, latestPendingGoalRequest(t, f)); err != nil {
				t.Fatal(err)
			}
			var status, reviewStatus, reason, decision string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT g.status,r.status,COALESCE(r.reason_category,''),COALESCE(r.decision,'') FROM public.fluctlight_goals g JOIN public.goal_reviews r ON r.goal_id=g.id WHERE g.id=$1`, goalID).Scan(&status, &reviewStatus, &reason, &decision); err != nil {
				t.Fatal(err)
			}
			if completed {
				if status != "completed" || reviewStatus != "superseded" {
					t.Fatal(status, reviewStatus)
				}
			} else {
				if status != "active" || reviewStatus != "succeeded" || reason != "evaluation_incomplete" || decision != "wait" {
					t.Fatal(status, reviewStatus, reason, decision)
				}
			}
			if calls != 2 {
				t.Fatal("root loop did not close", calls)
			}
		})
	}
}
