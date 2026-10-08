package core

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func backgroundTriggerRedis(t *testing.T, f independentToolE2EFixture) {
	t.Helper()
	url := os.Getenv("GO_CORE_TEST_REDIS_URL")
	if url == "" {
		t.Skip("GO_CORE_TEST_REDIS_URL is not set")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(f.ctx).Err(); err != nil {
		t.Fatal(err)
	}
	f.app.SetRedisClient(client, "background-trigger-"+f.suffix)
	t.Cleanup(func() {
		client.Del(context.Background(), wakeUpTriggerPrefix+f.fluctlightID, reflectionTriggerPrefix+f.fluctlightID)
		client.Close()
	})
}

func TestBackgroundStartupExistingKeyAndMissingKeyReleaseOnce(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	backgroundTriggerRedis(t, f)
	if _, err := f.app.EnsureWakeUpIntents(f.ctx); err != nil {
		t.Fatal(err)
	}
	key := wakeUpTriggerPrefix + f.fluctlightID
	if err := f.app.Redis.Set(f.ctx, key, f.fluctlightID, 4*time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	before := f.app.Redis.PTTL(f.ctx, key).Val()
	if n, err := f.app.ScheduleWakeUpTriggers(f.ctx); err != nil || n != 0 {
		t.Fatalf("existing clock released: %d %v", n, err)
	}
	after := f.app.Redis.PTTL(f.ctx, key).Val()
	if after > before || after < before-time.Second {
		t.Fatalf("startup rewrote TTL: %s -> %s", before, after)
	}
	if err := f.app.Redis.Del(f.ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	if n, err := f.app.ScheduleWakeUpTriggers(f.ctx); err != nil || n != 1 {
		t.Fatalf("missing key not released once: %d %v", n, err)
	}
	if n, err := f.app.ScheduleWakeUpTriggers(f.ctx); err != nil || n != 0 {
		t.Fatalf("duplicate startup release: %d %v", n, err)
	}
	var cycle int
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,(payload->>'cycle')::int FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+f.fluctlightID).Scan(&status, &cycle); err != nil {
		t.Fatal(err)
	}
	if cycle != 1 || status != "retry" {
		t.Fatalf("wrong startup cycle: %d %s", cycle, status)
	}
}

func TestBackgroundReflectionLastChatThirtyMinutesAndReplayDoesNotReopen(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	backgroundTriggerRedis(t, f)
	last := time.Now().UTC().Add(-5 * time.Minute)
	messageID := "quiet-message-" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,created_at) VALUES($1,$2,80,$3,'assistant','actual chat','[]',$1,$4)`, messageID, f.conversationID, f.fluctlightID, last); err != nil {
		t.Fatal(err)
	}
	queue := func(source string) {
		t.Helper()
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			return enqueueQuietPeriodReflectionIntentTx(f.ctx, tx, f.fluctlightID, source, time.Now().Add(time.Hour), "test")
		}); err != nil {
			t.Fatal(err)
		}
	}
	queue("completion-source")
	var due time.Time
	intentID := "reflection_intent:" + stableDigest(f.fluctlightID+"\x1f"+messageID)
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT next_attempt_at FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&due); err != nil {
		t.Fatal(err)
	}
	if due.Sub(last.Add(30*time.Minute)).Abs() > time.Millisecond {
		t.Fatalf("not anchored to last chat: %s", due)
	}
	if err := f.app.schedulePendingReflectionTrigger(f.ctx, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	ttl := f.app.Redis.TTL(f.ctx, reflectionTriggerPrefix+f.fluctlightID).Val()
	if ttl < 24*time.Minute || ttl > 25*time.Minute {
		t.Fatalf("Redis mirror reset from completion: %s", ttl)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='completed' WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	queue("same-chat-completion-replay")
	var state string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&state); err != nil || state != "completed" {
		t.Fatalf("replay reopened Reflection: %s %v", state, err)
	}
	newID := "quiet-new-message-" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,81,$3,'user','new chat','[]',$1)`, newID, f.conversationID, f.ownerID); err != nil {
		t.Fatal(err)
	}
	queue("new-completion")
	var pending int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='reflection.run' AND payload->>'fluctlight_id'=$1 AND status='pending'`, f.fluctlightID).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("not one fresh epoch: %d %v", pending, err)
	}
}

func TestBackgroundStartupCannotResurrectWakeWhileCognitionIsPending(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	backgroundTriggerRedis(t, f)
	if _, err := f.app.EnsureWakeUpIntents(f.ctx); err != nil {
		t.Fatal(err)
	}
	var inbox string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		inbox, err = f.app.enqueueNativeFactTx(f.ctx, tx, f.fluctlightID, f.conversationID, "new-source", "life.presence.updated", "startup-cognition", map[string]any{})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if n, err := f.app.ScheduleWakeUpTriggers(f.ctx); err != nil || n != 0 {
		t.Fatalf("startup revived old WakeUp: %d %v", n, err)
	}
	var cycle int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (payload->>'cycle')::int FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+f.fluctlightID).Scan(&cycle); err != nil || cycle != 0 {
		t.Fatalf("busy cognition clock advanced: %d %v", cycle, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='completed' WHERE intent_id=$1`, "cognition_intent:"+inbox); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Redis.Del(f.ctx, wakeUpTriggerPrefix+f.fluctlightID).Err(); err != nil {
		t.Fatal(err)
	}
	if n, err := f.app.ScheduleWakeUpTriggers(f.ctx); err != nil || n != 1 {
		t.Fatalf("idle startup did not release one new cycle: %d %v", n, err)
	}
}

func TestBackgroundSilentHistoryDoesNotCallModelOrAdvanceGoalWatermark(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"real recommendation"})
	var before, after int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_source_events WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 125; i++ {
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			_, err := appendProcessedCognitionFactTx(f.ctx, tx, f.fluctlightID, "internal.wake_up", map[string]any{"action_type": "no_op"}, "silent-"+randomID(""))
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	results := []CapabilityResult{{CallID: "inspect", CapabilityName: "goal.inspect", Status: "completed", Output: map[string]any{"goal_id": goalID}}}
	outcomes, err := buildActionOutcomes("silent-inspection", f.fluctlightID, "audit", "capability", results, map[string]any{"status": "completed"}, f.app.capabilityRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return persistActionOutcomesTx(f.ctx, tx, outcomes) }); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_source_events WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&after); err != nil || after != before {
		t.Fatalf("inspection moved Goal source watermark: %d -> %d %v", before, after, err)
	}
	f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("silent facts called Provider")
		return nil, errors.New("unexpected Provider")
	})}
	for i := 0; i < 2; i++ {
		r, err := f.app.ProcessReflection(f.ctx, f.fluctlightID, "silent-test")
		if err != nil || stringValue(r["status"]) != "no_op" || intValue(r["watermark"]) < 125 {
			t.Fatalf("silent history did not settle without model: %v %v", r, err)
		}
	}
	// A real observation beyond the old twenty-row window must still be
	// offered; the preceding audit noise must not starve it.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.cognition_reflection_windows SET watermark=0 WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := appendProcessedCognitionFactTx(f.ctx, tx, f.fluctlightID, "conversation.turn", map[string]any{"text": "real new observation", "conversation_id": f.conversationID}, "real-observation")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "background-reflection-"+f.suffix)
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("reflection_proposal_v2", func(input map[string]any) fakeProviderResult {
		calls++
		wire := jsonString(input)
		if !strings.Contains(wire, "real new observation") || strings.Contains(wire, "internal.wake_up") {
			t.Error("real evidence was starved or periodic noise offered")
		}
		return fakeProviderResult{Structured: reflectionProposalV2Fixture([]any{})}
	})}
	result, err := f.app.ProcessReflection(f.ctx, f.fluctlightID, "real-after-silent")
	if err != nil || calls != 1 || stringValue(result["status"]) != "no_change" {
		t.Fatalf("real observation not reflected once: %v calls=%d err=%v", result, calls, err)
	}
}

type backgroundCancellationRuntime struct {
	WorkflowRuntime
	cancelled []string
}

func (r *backgroundCancellationRuntime) Cancel(_ context.Context, id, _, _ string) error {
	r.cancelled = append(r.cancelled, id)
	return nil
}

func TestBackgroundNewCognitionCancelsRunningProviderAndRejectsLateWatermark(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	backgroundTriggerRedis(t, f)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "background-cancel-"+f.suffix)
	intentID := "reflection_intent:running-" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,started_at) VALUES($1,$1,'lifecycle','reflection.run',$2,'started',now())`, intentID, jsonBytes(map[string]any{"fluctlight_id": f.fluctlightID})); err != nil {
		t.Fatal(err)
	}
	leaseCtx, err := f.app.claimReflectionWindow(WithLifecycleIntentID(f.ctx, intentID), f.fluctlightID, 0, 0, "late-reflection")
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	done := make(chan error, 1)
	f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	marker := ReflectionProviderCancellationMarker(intentID)
	workflows := &backgroundCancellationRuntime{}
	f.app.Workflows = workflows
	go func() {
		_, err := f.app.Provider.Structured(WithProviderCancellationKey(f.ctx, marker), "reflection", []map[string]any{{"role": "user", "content": "test"}})
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Provider did not start")
	}
	var inbox string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		inbox, err = f.app.enqueueNativeFactTx(f.ctx, tx, f.fluctlightID, f.conversationID, "test", "life.presence.updated", "fresh-cognition", map[string]any{})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&state); err != nil || state != "superseded" {
		t.Fatalf("enqueue did not preempt: %s %v", state, err)
	}
	if err := f.app.CancelLifecycleForCognition(f.ctx, f.fluctlightID, "cognition:"+inbox); err != nil {
		t.Fatal(err)
	}
	if len(workflows.cancelled) != 1 || workflows.cancelled[0] != "go:"+intentID {
		t.Fatalf("Temporal cancel not forwarded exactly once: %v", workflows.cancelled)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled Provider returned success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("running Provider not cancelled")
	}
	if err := f.app.advanceReflectionWatermarkWithoutModel(leaseCtx, f.fluctlightID, 99); !errors.Is(err, errLifecycleSupersededByCognition) {
		t.Fatalf("late Reflection committed: %v", err)
	}
	if err := f.app.setReflectionWindowIdle(leaseCtx, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
}
