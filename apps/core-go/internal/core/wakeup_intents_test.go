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
	for _, file := range []string{"cognition.go", "mutations.go"} {
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
	source, err := os.ReadFile("mutations.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) handleTurn", "func (a *App) authorizeActorTurn")
	if !strings.Contains(body, "CancelLifecycleForCognition") {
		t.Fatal("synchronous cognition path does not preempt WakeUp/Reflection")
	}
	preemptIndex := strings.Index(body, "CancelLifecycleForCognition")
	providerIndex := strings.Index(body, "StructuredAssembledWithToolsSchema")
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
	source, err := os.ReadFile("wakeup.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) persistWakeUp", "func updateWakeUpNextDueTx")
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
	wakeSource, err := os.ReadFile("wakeup.go")
	if err != nil {
		t.Fatal(err)
	}
	wakeBody := sourceBetween(t, string(wakeSource), "func (a *App) ProcessWakeUp", "func capabilityInvocationText")
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
	source, err := os.ReadFile("wakeup.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) ProcessWakeUp", "func capabilityInvocationText")
	ensureAt := strings.Index(body, "EnsureDirectConversation(ctx, ownerID, fluctlightID)")
	projectionAt := strings.Index(body, "BuildContextProjectionFor(ctx, ContextProjectionRequest{")
	if ensureAt < 0 || projectionAt < 0 || ensureAt > projectionAt {
		t.Fatal("WakeUp must ensure the direct conversation before building the reply context")
	}
}

func TestWakeUpConversationReplyCreatesAndDeliversPrivateMessage(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "wakeup-reply-owner", "wakeup-reply-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	baseApp := &App{DB: repository}
	initialLife := currentLifeForTest(t, ctx, baseApp, fluctlightID, time.Now().UTC())
	if _, err := baseApp.AcceptSchedule(ctx, ownerID, fluctlightID, fullDaySchedulePayloadForTest(time.Now().UTC(), "wakeup-reply-schedule", stringValue(initialLife["context_revision"]))); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repository, "wakeup-reply-endpoint")
	text := "我刚刚想起你了，等你忙完再聊。"
	providerCalls := 0
	router := newFakeProviderRouter().on("wake_up_response", func(_ map[string]any) fakeProviderResult {
		providerCalls++
		if providerCalls > 1 {
			// The first ADK generation chooses the deferred output capability;
			// the second generation receives its ToolResult and terminates with
			// the structured wake-up decision. Keep the production tool call in
			// the final ADK trace so Core can freeze it exactly once.
			return fakeProviderResult{Structured: map[string]any{
				"action_type": "no_op", "response_intent": "主动联系 Owner", "influences": []any{},
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
	actionID := stringValue(wakeResult["action_id"])
	if actionID == "" {
		t.Fatalf("WakeUp did not freeze an action: %#v", wakeResult)
	}
	var conversationID string
	if err := repository.Pool().QueryRow(ctx, `SELECT conversation_id FROM public.fluctlight_direct_conversations WHERE owner_actor_id=$1 AND fluctlight_actor_id=$2`, ownerID, fluctlightID).Scan(&conversationID); err != nil {
		t.Fatal(err)
	}
	if conversationID == "" {
		t.Fatal("WakeUp did not ensure a direct conversation")
	}
	if _, err := app.ProcessAutonomyAction(ctx, actionID); err != nil {
		t.Fatalf("ProcessAutonomyAction failed: %v", err)
	}
	var messageCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2`, conversationID, text).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("WakeUp private message count = %d, want 1", messageCount)
	}
}

func TestWakeUpDerivedIntentsKeepCycleCorrelation(t *testing.T) {
	wakeSource, err := os.ReadFile("wakeup.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(wakeSource), "func (a *App) persistWakeUp", "func updateWakeUpNextDueTx")
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

func TestWakeUpReturnPreservesQueuedBlockedAndNoopStatus(t *testing.T) {
	source, err := os.ReadFile("wakeup.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) ProcessWakeUp", "func capabilityInvocationText")
	if !strings.Contains(body, `"status": firstString(result["status"], "no_op")`) {
		t.Fatal("WakeUp completion still collapses queued/blocked/no-op into completed")
	}
	for _, reason := range []string{"wake_up_disabled", "fluctlight_paused", "fluctlight_not_active", "no_action_selected"} {
		if !strings.Contains(body, reason) {
			t.Fatalf("WakeUp suppression/no-op reason %q is missing", reason)
		}
	}
}

func TestWakeUpMissingCapabilityActionFallsBackBeforeInfluenceValidation(t *testing.T) {
	for _, proposedActionType := range []string{"reply", "proactive_message", "moment", "schedule"} {
		assessment := map[string]any{"action_type": proposedActionType}
		if proposed := normalizeWakeUpActionWithoutCapability(assessment, nil); proposed != proposedActionType {
			t.Fatalf("proposed action = %q, want %q", proposed, proposedActionType)
		}
		if actionType := stringValue(assessment["action_type"]); actionType != "no_op" {
			t.Fatalf("normalized action = %q, want no_op", actionType)
		}
		if wakeUpDecisionRequiresInfluences(stringValue(assessment["action_type"]), nil, nil) {
			t.Fatalf("missing optional %q capability still requires influences", proposedActionType)
		}
		actual, result := fallbackWakeUpActionWithoutCapability(proposedActionType)
		if actual != "no_op" || result["status"] != "no_op" || result["reason"] != "action_requires_capability_call" || result["proposed_action_type"] != proposedActionType {
			t.Fatalf("fallback for %q = %q %#v", proposedActionType, actual, result)
		}
	}

	assessment := map[string]any{"action_type": "no_op"}
	if proposed := normalizeWakeUpActionWithoutCapability(assessment, nil); proposed != "" || assessment["action_type"] != "no_op" {
		t.Fatalf("no-op assessment changed: proposed=%q assessment=%#v", proposed, assessment)
	}
	assessment = map[string]any{"action_type": "proactive_message"}
	call := CapabilityInvocation{CapabilityName: "conversation.reply"}
	if proposed := normalizeWakeUpActionWithoutCapability(assessment, []CapabilityInvocation{call}); proposed != "" || assessment["action_type"] != "proactive_message" {
		t.Fatalf("capability-backed assessment changed: proposed=%q assessment=%#v", proposed, assessment)
	}
}

func TestWakeUpMissingInfluencesNeverDropsProviderCalls(t *testing.T) {
	registry := mustCapabilityRegistry(conversationReplyCapability{}, affectEventCapability{})
	calls := []CapabilityInvocation{
		{CapabilityName: "affect_event"},
		{CapabilityName: "conversation.reply"},
	}
	deferred, immediate := splitDeferredOutputCapabilities(calls, registry)
	if len(deferred) != 1 || deferred[0].CapabilityName != "conversation.reply" {
		t.Fatalf("deferred calls = %#v", deferred)
	}
	if len(immediate) != 1 || immediate[0].CapabilityName != "affect_event" {
		t.Fatalf("state-changing calls = %#v", immediate)
	}
	if wakeUpDecisionRequiresInfluences("proactive_message", deferred, registry) {
		t.Fatal("retained deferred output still requires an influence sidecar")
	}
	if !wakeUpDecisionRequiresInfluences("no_op", immediate, registry) {
		t.Fatal("state-changing capability unexpectedly became ungrounded")
	}
	source, err := os.ReadFile("wakeup.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) ProcessWakeUp", "func capabilityInvocationText")
	if strings.Contains(body, "toolCalls = deferredCalls") || strings.Contains(body, "toolCalls = nil") {
		t.Fatal("WakeUp must not silently discard a Provider tool call")
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
