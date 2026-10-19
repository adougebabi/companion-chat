package core

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestScheduledResolutionRecoveryRequiresExactCommittedNoEffectShape(t *testing.T) {
	base := handledCapabilityResult(CapabilityInvocation{CallID: "test", CapabilityName: lifeActivityAdvanceCapabilityName}, "activity_result_invalid", "awaiting_evidence", nil)
	if !knownInvalidScheduledResolution(base) {
		t.Fatal("known no-effect preparation cannot recover")
	}
	for _, change := range []func(*CapabilityResult){
		func(r *CapabilityResult) { r.Status = "unknown" },
		func(r *CapabilityResult) { mapValue(r.Output)["mutation_committed"] = true },
		func(r *CapabilityResult) { delete(mapValue(r.Output), "mutation_committed") },
		func(r *CapabilityResult) { mapValue(r.Output)["effect_applied"] = true },
		func(r *CapabilityResult) { mapValue(r.Output)["warning_code"] = "activity_not_found" },
		func(r *CapabilityResult) { mapValue(r.Output)["disposition"] = "awaiting_state" },
	} {
		candidate := base
		candidate.Output = cloneMap(mapValue(base.Output))
		change(&candidate)
		if knownInvalidScheduledResolution(candidate) {
			t.Fatal("ambiguous or unrelated receipt allowed a new operation", candidate)
		}
	}
}

func TestScheduledResolutionRecoveryIsDurableConcurrentBoundedAndNeverPurchasesFromInvalidOutput(t *testing.T) {
	f, intention, _, activity, _ := startDueShoppingThroughNative(t)
	forceVirtualActivityDue(t, f, activity)
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("virtual_activity_result", func(map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "mismatched item cannot become an actual purchase", "acquired_items": []any{map[string]any{"item_kind": "wearable", "category": "coat", "slot": "outerwear", "description": "unrequested coat"}}}}
	})}
	first, err := f.app.ResolveScheduledLifeActivity(f.ctx, activity)
	if err != nil || first["status"] != "pending" || calls != 1 {
		t.Fatal(first, calls, err)
	}
	var revision int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_life_activity_runs WHERE id=$1`, activity).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	original := fmt.Sprintf("scheduled-resolve:%s:%d", activity, revision-1)
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := f.app.recoverScheduledResolution(f.ctx, activity, f.fluctlightID, revision-1, original)
			errors <- e
		}()
	}
	wg.Wait()
	close(errors)
	for e := range errors {
		if e != nil {
			t.Fatal(e)
		}
	}
	var current, count int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT revision,(result_json#>>'{resolution_recovery,attempt_count}')::int FROM public.fluctlight_life_activity_runs WHERE id=$1`, activity).Scan(&current, &count); err != nil || current != revision || count != 1 {
		t.Fatal("same no-effect receipt rotated twice", current, revision, count, err)
	}
	waiting, err := f.app.ResolveScheduledLifeActivity(f.ctx, activity)
	if err != nil || waiting["not_before"] != first["not_before"] || calls != 1 {
		t.Fatal("retry boundary ran a model early", waiting, calls, err)
	}
	next := first
	for attempt := 2; attempt <= scheduledResolutionMaximumAttempts; attempt++ {
		due, e := time.Parse(time.RFC3339Nano, stringValue(next["not_before"]))
		if e != nil {
			t.Fatal(e)
		}
		f.app.Clock = fixedClock(due.Add(time.Millisecond))
		next, err = f.app.ResolveScheduledLifeActivity(f.ctx, activity)
		if err != nil {
			t.Fatal(attempt, next, err)
		}
	}
	if next["status"] != "cancelled" || next["reason"] != "result_validation_retry_exhausted" || calls != 3 {
		t.Fatal("invalid result retry did not terminate", next, calls)
	}
	var status, intentionStatus string
	var purchased int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT r.status,i.status,(SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result') FROM public.fluctlight_life_activity_runs r JOIN public.fluctlight_intentions i ON i.id=r.intention_id WHERE r.id=$2 AND i.id=$3`, f.fluctlightID, activity, intention).Scan(&status, &intentionStatus, &purchased); err != nil || status != "cancelled" || intentionStatus != "paused" || purchased != 0 {
		t.Fatal("retry exhaustion fabricated result or left Intention running", status, intentionStatus, purchased, err)
	}
	final, err := f.app.ResolveScheduledLifeActivity(f.ctx, activity)
	if err != nil || final["status"] != "cancelled" || calls != 3 {
		t.Fatal("terminal activity restarted model", final, calls, err)
	}
}

func TestScheduledResolutionRecoveryAcceptsPersistedOldReceiptWithoutReissuingModel(t *testing.T) {
	f, _, _, activity, _ := startDueShoppingThroughNative(t)
	forceVirtualActivityDue(t, f, activity)
	var revision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_life_activity_runs WHERE id=$1`, activity).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("virtual_activity_result", func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "invalid frozen target", "acquired_items": []any{map[string]any{"item_kind": "wearable", "category": "coat", "slot": "outerwear", "description": "unrequested coat"}}}}
	})}
	receipt, err := f.app.ExecuteTool(f.ctx, ToolExecutionRequest{CapabilityName: lifeActivityAdvanceCapabilityName, OperationID: fmt.Sprintf("scheduled-resolve:%s:%d", activity, revision), AuthorizationActorID: f.ownerID, FluctlightID: f.fluctlightID, EvidenceID: "activity:" + activity, Surface: CapabilitySurfaceNativeCognition, CorrelationID: "scheduled-activity:" + activity, Arguments: jsonBytes(map[string]any{"activity_id": activity})})
	if err != nil || !knownInvalidScheduledResolution(receipt.Result) {
		t.Fatal("old stalled receipt fixture", receipt, err)
	}
	// Simulate a restarted resolver: the original committed invalid receipt is
	// the only evidence available; the Provider must not be touched to rotate it.
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Err: errors.New("Provider must not run during receipt recovery")}
	})}
	recovered, err := f.app.ResolveScheduledLifeActivity(f.ctx, activity)
	if err != nil || recovered["status"] != "pending" || calls != 0 {
		t.Fatal("stalled receipt could not recover without reissuing", recovered, calls, err)
	}
}

func startScheduledDyeRecoveryFixture(t *testing.T) (independentToolE2EFixture, string, string) {
	t.Helper()
	f, _, intention, _, item := seedFutureScheduledDye(t)
	start := time.Now().UTC().Add(-time.Minute)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.life_schedule_items SET end_at=$2 WHERE schedule_id=(SELECT schedule_id FROM public.life_schedule_items WHERE id=$1) AND end_at=(SELECT start_at FROM public.life_schedule_items WHERE id=$1) AND intention_id IS NULL`, item, start); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.life_schedule_items SET start_at=$2,end_at=now()+interval '20 minutes' WHERE id=$1`, item, start); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_intentions SET trigger=jsonb_set(trigger,'{due_at}',to_jsonb($2::text),true),preferred_time=$3 WHERE id=$1`, intention, start.Format(time.RFC3339Nano), start); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.ProcessIntentionTrigger(f.ctx, intention)
	if err != nil || result["status"] != "activity_started" {
		t.Fatal(result, err)
	}
	activity := stringValue(result["activity_id"])
	forceVirtualActivityDue(t, f, activity)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "recovery-dye-provider-"+f.suffix)
	return f, intention, activity
}

func TestScheduledResolutionRecoveryCancellationDuringWaitDoesNotResolveAgain(t *testing.T) {
	f, intention, activity := startScheduledDyeRecoveryFixture(t)
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("virtual_activity_result", func(map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "wrong requested color", "hair_color": "蓝色"}}
	})}
	pending, err := f.app.ResolveScheduledLifeActivity(f.ctx, activity)
	if err != nil || pending["status"] != "pending" {
		t.Fatal(pending, err)
	}
	paused, err := f.app.ExecuteTool(f.ctx, f.request(intentionDecideCapabilityName, "pause-result-retry", map[string]any{"operation": "pause", "intention_id": intention, "reason": "Owner pauses the actual appointment before retry"}))
	if err != nil || paused.Result.Status != "completed" {
		t.Fatal(paused, err)
	}
	closed, err := f.app.ResolveScheduledLifeActivity(f.ctx, activity)
	if err != nil || closed["status"] != "cancelled" || calls != 1 {
		t.Fatal("withdrawn appointment invoked result model again", closed, calls, err)
	}
}

func TestScheduledResolutionRecoveryPreservesLateNativeResultWithoutResumingIntention(t *testing.T) {
	f, intention, _, activity, validRouter := startDueShoppingThroughNative(t)
	forceVirtualActivityDue(t, f, activity)
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("virtual_activity_result", func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "invalid target", "acquired_items": []any{map[string]any{"item_kind": "wearable", "category": "coat", "slot": "outerwear", "description": "unrequested coat"}}}}
	})}
	pending, err := f.app.ResolveScheduledLifeActivity(f.ctx, activity)
	if err != nil || pending["status"] != "pending" {
		t.Fatal(pending, err)
	}
	if _, err = f.app.ExecuteTool(f.ctx, f.request(intentionDecideCapabilityName, "pause-native-result-retry", map[string]any{"operation": "pause", "intention_id": intention, "reason": "Pause further actions; preserve late actual results"})); err != nil {
		t.Fatal(err)
	}
	due, err := time.Parse(time.RFC3339Nano, stringValue(pending["not_before"]))
	if err != nil {
		t.Fatal(err)
	}
	validRouter.on("virtual_activity_result", func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "实际虚拟购物结果", "acquired_item": map[string]any{"item_kind": "wearable", "category": "boots", "slot": "shoes", "description": "黑色短靴"}}}
	})
	f.app.Clock = fixedClock(due.Add(time.Millisecond))
	f.app.Provider.HTTP = &http.Client{Transport: validRouter}
	result, err := f.app.ResolveScheduledLifeActivity(f.ctx, activity)
	if err != nil || result["status"] != "completed" {
		t.Fatal("native late result was discarded", result, err)
	}
	var state string
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intention).Scan(&state); err != nil || state != "paused" {
		t.Fatal("late result revived Intention", state, err)
	}
}
