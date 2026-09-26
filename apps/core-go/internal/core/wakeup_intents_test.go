package core

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEnsureWakeUpIntentsDoesNotRequireAcceptedSchedule(t *testing.T) {
	source, err := os.ReadFile("wakeup.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) EnsureWakeUpIntents", "func numberOrDefault")
	if strings.Contains(body, "life_schedules") {
		t.Fatal("WakeUp liveness still depends on an accepted Schedule")
	}
}

func TestWakeUpClockRepairOnlyRepairsMissingOrUninitializedRows(t *testing.T) {
	source, err := os.ReadFile("wakeup.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) RepairWakeUpClocks", "// TriggerWakeUp")
	for _, required := range []string{"ON CONFLICT (intent_id) DO NOTHING", "i.next_attempt_at IS NULL"} {
		if !strings.Contains(body, required) {
			t.Fatalf("WakeUp clock repair missing %q", required)
		}
	}
	if strings.Contains(body, "status='failed'") || strings.Contains(body, "status='retry'") {
		t.Fatal("WakeUp clock repair must not requeue terminal workflow failures")
	}
}

func TestConversationRearmsWakeUpThroughCognitionFollowups(t *testing.T) {
	for _, file := range []string{"cognition.go", "agent_result_adapter.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "scheduleWakeUpTrigger(") {
			t.Fatalf("%s bypasses the shared cognition WakeUp follow-up boundary", file)
		}
		if !strings.Contains(string(source), "scheduleCognitionFollowups(") {
			t.Fatalf("%s does not rearm WakeUp/Reflection after cognition", file)
		}
	}
}

func TestSynchronousCognitionPreemptsLifecycleBeforeProvider(t *testing.T) {
	source, err := os.ReadFile("agent_result_adapter.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) handleTurn", "func (a *App) replayCommittedAgentTurn")
	if !strings.Contains(body, "CancelLifecycleForCognition") {
		t.Fatal("synchronous cognition path does not preempt WakeUp/Reflection")
	}
	preemptIndex := strings.Index(body, "CancelLifecycleForCognition")
	providerIndex := strings.Index(body, "RunMain")
	if providerIndex >= 0 && preemptIndex > providerIndex {
		t.Fatal("synchronous cognition starts the Provider before lifecycle preemption")
	}
}

func TestCognitionFollowupsAttemptBothIndependentDebounceClocks(t *testing.T) {
	source, err := os.ReadFile("redis_triggers.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) scheduleCognitionFollowups", "// ScheduleWakeUpTriggers")
	if !strings.Contains(body, "scheduleReflectionTrigger") || !strings.Contains(body, "scheduleWakeUpAfterCognition") {
		t.Fatal("cognition follow-ups do not arm both lifecycle clocks")
	}
	if !strings.Contains(body, "both maintenance attempts ran") {
		t.Fatal("one debounce failure can still suppress the other clock")
	}
}

func TestWakeUpHasPostgresDueSweepAndOverdueDiagnostic(t *testing.T) {
	source, err := os.ReadFile("redis_triggers.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "next_attempt_at <= now()") ||
		!strings.Contains(text, "status='completed'") ||
		!strings.Contains(text, "lifecycle.wake_up.overdue") {
		t.Fatal("completed WakeUp has no PostgreSQL due sweep and overdue diagnostic fallback")
	}
}

func TestWakeUpDueReleaseUsesLockedStatusAndDueCAS(t *testing.T) {
	source, err := os.ReadFile("redis_triggers.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) ReleaseDueWakeUpIntents", "func (a *App) releaseWakeUpIntent")
	for _, required := range []string{
		"FOR UPDATE OF i SKIP LOCKED",
		"i.status='completed'",
		"i.next_attempt_at <= now()",
		"i.next_attempt_at=c.next_attempt_at",
		"jsonb_set(i.payload,'{cycle}'",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("WakeUp due release missing concurrency contract %q", required)
		}
	}
}

func TestWakeUpOutcomePersistsNextDueInsideOwningTransaction(t *testing.T) {
	source, err := os.ReadFile("agent_result_adapter.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) persistCommittedWakeUp", "func (a *App) ProcessNativeCognitionFact")
	if !strings.Contains(body, "updateWakeUpNextDueTx") ||
		!strings.Contains(body, "\"next_due_at\"") ||
		strings.Index(body, "updateWakeUpNextDueTx") > strings.LastIndex(body, "appendOutboxTx") {
		t.Fatal("WakeUp outcome does not persist next due inside its owning transaction")
	}
}

func TestWakeUpSuccessResetsConsecutiveRetryBudget(t *testing.T) {
	source, err := os.ReadFile("wakeup.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	start := strings.Index(body, "func updateWakeUpNextDueTx")
	if start < 0 {
		t.Fatal("updateWakeUpNextDueTx source not found")
	}
	body = body[start:]
	if !strings.Contains(body, "attempt_count=0") {
		t.Fatal("successful WakeUp completion must reset the consecutive workflow retry budget")
	}
}

func TestCreationOwnsIndependentScheduleAndWakeUpIntents(t *testing.T) {
	source, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	create := sourceBetween(t, text, "func (a *App) CreateFluctlight", "func initialPersonalityProfileID")
	if strings.Count(create, "ensureCreationLifecycleIntentsTx") < 2 {
		t.Fatal("new activation and idempotent replay do not share lifecycle-intent repair")
	}
	helper := sourceBetween(t, text, "func ensureCreationLifecycleIntentsTx", "func initialPersonalityProfileID")
	for _, required := range []string{"schedule.current_day", "wake_up.current", "next_attempt_at", "ON CONFLICT(intent_id)"} {
		if !strings.Contains(helper, required) {
			t.Fatalf("creation lifecycle intent helper missing %q", required)
		}
	}
	if strings.Contains(helper, "life_schedules") {
		t.Fatal("creation still gates WakeUp intent on an accepted Schedule")
	}
}

func TestWakeUpScheduleStatusIsExplicitWhenContextIsMissing(t *testing.T) {
	if got := wakeUpScheduleStatus(nil); got != "missing" {
		t.Fatalf("missing schedule status = %q", got)
	}
	if got := wakeUpScheduleStatus(map[string]any{"status": "pending"}); got != "pending" {
		t.Fatalf("pending schedule status = %q", got)
	}
	if got := wakeUpScheduleStatus(map[string]any{"local_date": "2026-09-13"}); got != "ready" {
		t.Fatalf("accepted schedule fallback status = %q", got)
	}
}

func TestWakeUpAndReflectionProviderCallsUseLifecycleCorrelation(t *testing.T) {
	wakeSource, err := os.ReadFile("agent_result_adapter.go")
	if err != nil {
		t.Fatal(err)
	}
	wakeBody := sourceBetween(t, string(wakeSource), "func (a *App) ProcessWakeUp", "func (a *App) ProcessNativeCognitionFact")
	if !strings.Contains(wakeBody, "WithProviderCorrelation") || !strings.Contains(wakeBody, "wakeUpCycleCorrelation(fluctlightID, cycle)") {
		t.Fatal("WakeUp Provider call does not inherit the stable cycle correlation")
	}
	reflectionSource, err := os.ReadFile("workflow_ops.go")
	if err != nil {
		t.Fatal(err)
	}
	reflectionBody := sourceBetween(t, string(reflectionSource), "func (a *App) ProcessReflection", "func boundedNumber")
	if !strings.Contains(reflectionBody, "WithProviderCorrelation(ctx, correlationID)") {
		t.Fatal("Reflection Provider call does not inherit its durable intent correlation")
	}
}

func TestWakeUpCreatesDirectConversationBeforeBuildingReplyContext(t *testing.T) {
	source, err := os.ReadFile("agent_result_adapter.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) ProcessWakeUp", "func (a *App) ProcessNativeCognitionFact")
	ensureAt := strings.Index(body, "EnsureDirectConversation(ctx, ownerID, fluctlightID)")
	projectionAt := strings.Index(body, "BuildContextProjectionFor(ctx, projectionRequest)")
	if ensureAt < 0 || projectionAt < 0 || ensureAt > projectionAt {
		t.Fatal("WakeUp must ensure the direct conversation before building the reply context")
	}
}

func TestWakeUpConversationReplyCreatesAndDeliversPrivateMessage(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "wakeup-reply-owner", "wakeup-reply-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	resource, err := repository.GetFluctlight(ctx, fluctlightID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	corePersona := cloneMap(resource.CorePersona)
	corePersona["life_profile"] = map[string]any{"preferences": map[string]any{"drink": "喜欢咖啡，但不喜欢甜咖啡"}}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET core_persona=$2,life_profile=$3 WHERE id=$1`, fluctlightID, jsonBytes(corePersona), jsonBytes(corePersona["life_profile"])); err != nil {
		t.Fatal(err)
	}
	baseApp := &App{DB: repository}
	initialLife := currentLifeForTest(t, ctx, baseApp, fluctlightID, time.Now().UTC())
	if _, err := baseApp.AcceptSchedule(ctx, ownerID, fluctlightID, fullDaySchedulePayloadForTest(time.Now().UTC(), "wakeup-reply-schedule", stringValue(initialLife["context_revision"]))); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repository, "wakeup-reply-endpoint")
	text := "我刚刚想起你了，等你忙完再聊。"
	providerCalls := 0
	router := newFakeProviderRouter().on("wake_up_response", func(payload map[string]any) fakeProviderResult {
		providerCalls++
		if providerCalls == 1 && !strings.Contains(jsonString(payload), "喜欢咖啡") {
			t.Fatal("formal WakeUp request lost the saved stable preference")
		}
		if providerCalls > 1 {
			// The first ADK generation chooses the deferred output capability;
			// the second generation receives its ToolResult and terminates with
			// the structured wake-up decision. Keep the production tool call in
			// the final ADK trace so Core can freeze it exactly once.
			return fakeProviderResult{Structured: map[string]any{
				"action_type": "no_op", "response_intent": "主动联系 Owner", "evidence_refs": []any{}, "influences": []any{},
			}}
		}
		return fakeProviderResult{
			Structured: map[string]any{"action_type": "reply", "response_intent": "主动联系 Owner", "influences": []any{}},
			ToolCalls: []map[string]any{{
				"id": "wakeup-reply-call", "type": "function",
				"function": map[string]any{"name": conversationReplyCapabilityName, "arguments": jsonString(map[string]any{"text": text})},
			}},
		}
	})
	app := newTestApp(t, repository, router)
	wakeResult, err := app.ProcessWakeUp(ctx, fluctlightID, 1)
	if err != nil {
		t.Fatalf("ProcessWakeUp failed: %v", err)
	}
	if stringValue(wakeResult["action_type"]) != "proactive_message" {
		t.Fatalf("WakeUp action type = %#v", wakeResult)
	}
	var conversationID string
	if err := repository.Pool().QueryRow(ctx, `SELECT conversation_id FROM public.fluctlight_direct_conversations WHERE owner_actor_id=$1 AND fluctlight_actor_id=$2`, ownerID, fluctlightID).Scan(&conversationID); err != nil {
		t.Fatal(err)
	}
	if conversationID == "" {
		t.Fatal("WakeUp did not ensure a direct conversation")
	}
	var messageCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2`, conversationID, text).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("WakeUp private message count = %d, want 1", messageCount)
	}
}

func TestWakeUpNoOpSidecarWithAffectAndReplyStillDeliversPrivateMessage(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "wakeup-noop-sidecar-owner", "wakeup-noop-sidecar-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	baseApp := &App{DB: repository}
	initialLife := currentLifeForTest(t, ctx, baseApp, fluctlightID, time.Now().UTC())
	if _, err := baseApp.AcceptSchedule(ctx, ownerID, fluctlightID, fullDaySchedulePayloadForTest(time.Now().UTC(), "wakeup-noop-sidecar-schedule", stringValue(initialLife["context_revision"]))); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repository, "wakeup-noop-sidecar-endpoint")
	text := "……嗯。"
	providerCalls := 0
	router := newFakeProviderRouter().on("wake_up_response", func(_ map[string]any) fakeProviderResult {
		providerCalls++
		if providerCalls > 1 {
			return fakeProviderResult{Structured: map[string]any{"action_type": "no_op", "response_intent": "", "evidence_refs": []any{}, "influences": []any{}}}
		}
		return fakeProviderResult{
			// This is the exact shape observed in diagnostics: the structured
			// sidecar says no_op, while native calls contain an affect update and
			// the actual proactive conversation reply.
			Structured: map[string]any{"action_type": "no_op", "evidence_refs": []any{}, "influences": []any{}, "response_intent": ""},
			ToolCalls: []map[string]any{
				{"id": "noop-affect-call", "type": "function", "function": map[string]any{"name": "affect_event", "arguments": jsonString(map[string]any{"event": map[string]any{"type": "excited", "confidence": 0.35}})}},
				{"id": "noop-reply-call", "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": jsonString(map[string]any{"text": text})}},
			},
		}
	})
	app := newTestApp(t, repository, router)
	wakeResult, err := app.ProcessWakeUp(ctx, fluctlightID, 1)
	if err != nil {
		t.Fatalf("ProcessWakeUp failed: %v", err)
	}
	if stringValue(wakeResult["action_type"]) != "proactive_message" {
		t.Fatalf("WakeUp action type = %#v", wakeResult)
	}
	var conversationID string
	if err := repository.Pool().QueryRow(ctx, `SELECT conversation_id FROM public.fluctlight_direct_conversations WHERE owner_actor_id=$1 AND fluctlight_actor_id=$2`, ownerID, fluctlightID).Scan(&conversationID); err != nil {
		t.Fatal(err)
	}
	var messageCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2`, conversationID, text).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("WakeUp no-op sidecar private message count = %d, want 1; result=%#v", messageCount, wakeResult)
	}
}

func TestWakeUpFinalAgentFailurePersistsCycleAndReplaySkipsProvider(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "wakeup-failed-agent-owner", "wakeup-failed-agent-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := (&App{DB: repository}).EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repository, "wakeup-failed-agent-endpoint")

	text := "这条消息已经由工具提交，最终输出失败也不能再次发送。"
	providerCalls := 0
	router := newFakeProviderRouter().on("wake_up_response", func(_ map[string]any) fakeProviderResult {
		providerCalls++
		if providerCalls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{{
				"id": "wakeup-committed-before-final-failure", "type": "function",
				"function": map[string]any{"name": conversationReplyCapabilityName, "arguments": jsonString(map[string]any{"text": text})},
			}}}
		}
		// The observed failure shape is a successful Tool round followed by an
		// empty final assistant message. The Agent rejects that as
		// adk_final_message_empty; Wake-up must still persist the cycle marker so
		// Temporal's Activity retry cannot replay the committed Tool.
		return fakeProviderResult{}
	})
	app := newTestApp(t, repository, router)

	if _, err := app.ProcessWakeUp(ctx, fluctlightID, 1); err == nil || !strings.Contains(err.Error(), "adk_final_message_empty") {
		t.Fatalf("first Wake-up error = %v, want adk_final_message_empty", err)
	}
	if providerCalls != 2 {
		t.Fatalf("first Wake-up provider calls = %d, want tool round plus failed final round", providerCalls)
	}

	var persistedStatus string
	var persistedResult []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT status,result FROM public.cognition_wakeups WHERE fluctlight_id=$1 AND cycle=1`, fluctlightID).Scan(&persistedStatus, &persistedResult); err != nil {
		t.Fatalf("failed Wake-up cycle was not persisted: %v", err)
	}
	if persistedStatus != "failed" || !strings.Contains(string(persistedResult), conversationReplyCapabilityName) {
		t.Fatalf("failed Wake-up status=%q result=%s", persistedStatus, persistedResult)
	}

	replay, err := app.ProcessWakeUp(ctx, fluctlightID, 1)
	if err != nil {
		t.Fatalf("failed Wake-up replay returned an error: %v", err)
	}
	if stringValue(replay["status"]) != "failed" || stringValue(replay["reason"]) != "wake_up_replayed" {
		t.Fatalf("failed Wake-up replay = %#v", replay)
	}
	if providerCalls != 2 {
		t.Fatalf("failed Wake-up replay called Provider again: calls=%d", providerCalls)
	}

	var messageCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE author_actor_id=$1 AND kind='assistant' AND text=$2`, fluctlightID, text).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("committed reply count after failed Wake-up replay = %d, want 1", messageCount)
	}
}

func TestWakeUpDerivedIntentsKeepCycleCorrelation(t *testing.T) {
	wakeSource, err := os.ReadFile("agent_result_adapter.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(wakeSource), "func (a *App) persistCommittedWakeUp", "func (a *App) ProcessNativeCognitionFact")
	for _, required := range []string{
		"correlationID := wakeUpCycleCorrelation(fluctlightID, cycle)",
		`"correlation_id": correlationID`,
		`"causation_id": factID`,
		"wakeID, correlationID",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("WakeUp derivative lost root correlation contract %q", required)
		}
	}
	workflowSource, err := os.ReadFile("workflow_ops.go")
	if err != nil {
		t.Fatal(err)
	}
	settlement := sourceBetween(t, string(workflowSource), "func (a *App) failAutonomyAction", "func (a *App) ProcessReflection")
	if strings.Count(settlement, `"correlation_id": rootCorrelationID`) < 2 || !strings.Contains(settlement, `"causation_id": factID`) {
		t.Fatal("WakeUp action settlement does not preserve the root cycle correlation into Reflection")
	}
}

func TestWakeUpReturnPreservesCommittedBlockedAndNoopStatus(t *testing.T) {
	source, err := os.ReadFile("agent_result_adapter.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) ProcessWakeUp", "func (a *App) ProcessNativeCognitionFact")
	if strings.Contains(body, `"status": "queued"`) || strings.Contains(body, "capability.action") || strings.Contains(body, "autonomy.action") {
		t.Fatal("WakeUp still queues a second execution after Agent Tool commit")
	}
	for _, reason := range []string{"wake_up_disabled", "fluctlight_paused", "fluctlight_not_active", "no_action_selected"} {
		if !strings.Contains(body, reason) {
			t.Fatalf("WakeUp suppression/no-op reason %q is missing", reason)
		}
	}
}

func TestFluctlightActivationRearmsWakeUpClock(t *testing.T) {
	source, err := os.ReadFile("operations.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) setFluctlightLifecycle", "func (a *App) ProposeFoundation")
	if !strings.Contains(body, `status == "active"`) ||
		!strings.Contains(body, "EnsureWakeUpIntents") ||
		!strings.Contains(body, "releaseWakeUpIntent") ||
		!strings.Contains(body, "wake_up_rearm_failed") {
		t.Fatal("active lifecycle transition does not rearm and diagnose the WakeUp clock")
	}
}

func sourceBetween(t *testing.T, source, start, end string) string {
	t.Helper()
	startIndex := strings.Index(source, start)
	if startIndex < 0 {
		t.Fatalf("source start %q not found", start)
	}
	endIndex := strings.Index(source[startIndex:], end)
	if endIndex < 0 {
		t.Fatalf("source end %q not found", end)
	}
	return source[startIndex : startIndex+endIndex]
}

func TestEnsureWakeUpIntentsRepairsExistingLiveFluctlight(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	fixtureKey := stableDigest(t.Name() + time.Now().UTC().Format(time.RFC3339Nano))
	fluctlightID := "test-wakeup-intent-" + fixtureKey
	intentID := "wake_up_intent:" + fluctlightID
	_, err = pool.Exec(ctx, `
		INSERT INTO public.fluctlights(
			id,created_by_actor_id,initialization_mode,status,identity,personality,
			behavioral_policy,life_profile,provenance
		) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}')
		ON CONFLICT (id) DO UPDATE SET status='active'`, fluctlightID, "test-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.fluctlights WHERE id=$1`, fluctlightID)
	})

	app := &App{DB: &PostgresRepository{pool: pool}}
	count, err := app.EnsureWakeUpIntents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count < 1 {
		t.Fatalf("EnsureWakeUpIntents() count = %d, want at least one", count)
	}

	var intentType, status string
	var payload []byte
	if err := pool.QueryRow(ctx, `SELECT intent_type,status,payload FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&intentType, &status, &payload); err != nil {
		t.Fatal(err)
	}
	if intentType != "wake_up.current" || status != "pending" || string(payload) == "" {
		t.Fatalf("wake-up intent = type %q, status %q, payload %s", intentType, status, payload)
	}

	if _, err := pool.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='failed',last_error='test-failure',completed_at=now() WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "retry" {
		t.Fatalf("repaired wake-up intent status = %q, want retry", status)
	}

	// Test dead_letter recovery with attempt_count reset
	if _, err := pool.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='dead_letter',attempt_count=5,last_error='exhausted',completed_at=now() WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	var attemptCount int
	if err := pool.QueryRow(ctx, `SELECT status,attempt_count FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status, &attemptCount); err != nil {
		t.Fatal(err)
	}
	if status != "retry" || attemptCount != 0 {
		t.Fatalf("dead_letter wake-up intent repaired status = %q, attempt_count = %d; want retry, 0", status, attemptCount)
	}

	// Test stale started recovery (service restart during execution)
	if _, err := pool.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='started',started_at=now()-interval '10 minutes',attempt_count=3 WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ReconcileWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status,attempt_count FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status, &attemptCount); err != nil {
		t.Fatal(err)
	}
	if status != "retry" || attemptCount != 0 {
		t.Fatalf("stale started wake-up intent repaired status = %q, attempt_count = %d; want retry, 0", status, attemptCount)
	}
}

func TestPostgresWakeUpClockRecoversLostRedisAndDeduplicatesRelease(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	fixtureKey := stableDigest(t.Name() + time.Now().UTC().Format(time.RFC3339Nano))
	fluctlightID := "test-wakeup-clock-" + fixtureKey
	intentID := "wake_up_intent:" + fluctlightID
	workflowID := "wake_up:" + fluctlightID
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.fluctlights(
			id,created_by_actor_id,initialization_mode,status,identity,personality,
			behavioral_policy,life_profile,provenance
		) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}')`, fluctlightID, "test-owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.platform_workflow_intents(
			intent_id,workflow_id,task_queue,intent_type,payload,status,next_attempt_at,completed_at
		) VALUES($1,$2,'lifecycle','wake_up.current',$3,'completed',now()-interval '5 minutes',now()-interval '5 minutes')`,
		intentID, workflowID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "cycle": 0}),
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.diagnostic_workflow_links WHERE correlation_id LIKE $1`, "wake_up:"+fluctlightID+":%")
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.diagnostic_events WHERE fluctlight_id=$1`, fluctlightID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.fluctlights WHERE id=$1`, fluctlightID)
	})

	app := &App{DB: &PostgresRepository{pool: pool}}
	released, err := app.ReleaseDueWakeUpIntents(ctx, 10)
	if err != nil || released != 1 {
		t.Fatalf("first due release = %d, %v; want one", released, err)
	}
	var status string
	var payload []byte
	if err := pool.QueryRow(ctx, `SELECT status,payload FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status, &payload); err != nil {
		t.Fatal(err)
	}
	if status != "retry" || intValue(decodeObject(payload)["cycle"]) != 1 {
		t.Fatalf("first release status=%q payload=%s", status, payload)
	}
	if released, err := app.ReleaseDueWakeUpIntents(ctx, 10); err != nil || released != 0 {
		t.Fatalf("duplicate due release = %d, %v; want zero", released, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed',next_attempt_at=now()-interval '5 minutes',completed_at=now()-interval '5 minutes' WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if released, err := app.ReleaseDueWakeUpIntents(ctx, 10); err != nil || released != 1 {
		t.Fatalf("second cycle release = %d, %v; want one", released, err)
	}
	if err := pool.QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if intValue(decodeObject(payload)["cycle"]) != 2 {
		t.Fatalf("second cycle payload=%s", payload)
	}
	var overdueCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.diagnostic_events WHERE fluctlight_id=$1 AND event_type='lifecycle.wake_up.overdue'`, fluctlightID).Scan(&overdueCount); err != nil {
		t.Fatal(err)
	}
	if overdueCount < 1 {
		t.Fatal("lost Redis expiry recovery did not record an overdue lifecycle event")
	}
}
