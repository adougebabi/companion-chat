package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAcceptScheduleUsesDatabaseIdempotencyBoundary(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	var head string
	if err := repository.Pool().QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != "0030_life_context_revision" {
		t.Skipf("test database is not at capability runtime head: head=%q err=%v", head, err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID := "actor_schedule_idempotency_" + suffix
	fluctlightID := "fluctlight_schedule_idempotency_" + suffix
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active',$3,'{}','{}','{}','{}')`, fluctlightID, ownerID, jsonBytes(map[string]any{"name": "schedule-idempotency", "timezone": "Asia/Shanghai"})); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.life_schedule_items WHERE schedule_id IN (SELECT id FROM public.life_schedules WHERE fluctlight_id=$1)`, fluctlightID)
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.platform_workflow_intents WHERE payload->>'fluctlight_id'=$1`, fluctlightID)
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.platform_outbox_events WHERE fluctlight_id=$1`, fluctlightID)
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.life_context_commands WHERE fluctlight_id=$1`, fluctlightID)
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.life_schedules WHERE fluctlight_id=$1`, fluctlightID)
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.fluctlights WHERE id=$1`, fluctlightID)
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.actors WHERE id IN ($1,$2)`, ownerID, fluctlightID)
	})
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(location)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	app := &App{DB: repository}
	_, life, err := app.readLifeContextSnapshotAt(ctx, fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"local_date": dayStart.Format("2006-01-02"), "timezone": "Asia/Shanghai", "generated_from": "model_replan",
		"source_fact_id": "fact-" + suffix, "idempotency_key": "tool:call-" + suffix, "reschedule_policy": map[string]any{},
		"expected_revision": 0, "expected_life_context_revision": life["context_revision"],
		"items": []any{map[string]any{
			"start_at": dayStart.Format(time.RFC3339), "end_at": dayStart.AddDate(0, 0, 1).Format(time.RFC3339),
			"activity": "reading", "scene": "home", "item_type": "planned", "status": "planned",
			"priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5,
		}},
	}
	first, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, cloneMap(payload))
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, cloneMap(payload))
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(first["id"]) == "" || stringValue(first["id"]) != stringValue(second["id"]) || intValue(first["revision"]) != intValue(second["revision"]) {
		t.Fatalf("schedule replay changed identity: first=%#v second=%#v", first, second)
	}
	changed := cloneMap(payload)
	mapValue(arrayValue(changed["items"])[0])["activity"] = "walking"
	if _, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, changed); err == nil || err.Error() != "schedule_idempotency_conflict" {
		t.Fatalf("different payload reused idempotency key: %v", err)
	}
	var versions int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_schedules WHERE fluctlight_id=$1`, fluctlightID).Scan(&versions); err != nil || versions != 1 {
		t.Fatalf("idempotent schedule created %d versions: %v", versions, err)
	}
}

func TestSchedulePlannerProviderRequestUsesOpaqueAllowlist(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	endpointID := "schedule-provider-egress"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://schedule-egress.invalid','schedule-egress-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'schedule-egress-model','structured_output',4096,10,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	var requestBody string
	client := &ProviderClient{DB: repository, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		requestBody = string(body)
		plan := map[string]any{
			"local_date": "2026-09-11", "timezone": "Asia/Shanghai", "expected_revision": 4,
			"completed_before": "2026-09-11T08:00:00+08:00", "reschedule_policy": map[string]any{},
			"items": []any{map[string]any{"start_at": "2026-09-11T08:00:00+08:00", "end_at": "2026-09-12T00:00:00+08:00", "activity": "阅读", "scene": "书房", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}},
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(plan)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}}
	planner := providerSchedulePlanner{provider: client}
	_, err := planner.Plan(ctx, SchedulePlanInput{
		Intent: "调整计划", SourceFactID: "fact-internal-raw", Timezone: "Asia/Shanghai",
		Schedule: map[string]any{
			"id": "schedule-internal-raw", "ref": "schedule:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "revision": 4,
			"local_date": "2026-09-11", "timezone": "Asia/Shanghai", "completed_before": "2026-09-11T08:00:00+08:00",
			"items": []any{map[string]any{"id": "item-internal-raw", "ref": "schedule_item:ctx_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "start_at": "2026-09-11T08:00:00+08:00", "end_at": "2026-09-12T00:00:00+08:00", "activity": "阅读", "scene": "书房"}},
		},
		CurrentLife: map[string]any{
			"ref": "life_context:ctx_cccccccccccccccccccccccccccccccc", "source": "event", "context_revision": "life_ctx_cccccccccccccccccccccccccccccccc",
			"event_id": "event-internal-raw", "event_ref": "scene:ctx_dddddddddddddddddddddddddddddddd", "scene": "书房", "activity": "阅读",
			"presence": map[string]any{"id": "presence-internal-raw", "actor_id": "actor-internal-raw", "ref": "presence:ctx_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "revision": 2, "current_task": "阅读"},
		},
		Agency: map[string]any{
			"goals":      []any{map[string]any{"id": "goal-internal-raw", "ref": "goal:ctx_ffffffffffffffffffffffffffffffff", "description": "完成阅读", "status": "active", "revision": 3}},
			"intentions": []any{map[string]any{"id": "intention-internal-raw", "ref": "intention:ctx_11111111111111111111111111111111", "goal_id": "goal-internal-raw", "goal_ref": "goal:ctx_ffffffffffffffffffffffffffffffff", "action": "继续阅读", "status": "active", "revision": 2}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"schedule-internal-raw", "item-internal-raw", "event-internal-raw", "presence-internal-raw", "actor-internal-raw", "goal-internal-raw", "intention-internal-raw", "fact-internal-raw"} {
		if strings.Contains(requestBody, forbidden) {
			t.Fatalf("Schedule planner Provider request leaked %q: %s", forbidden, requestBody)
		}
	}
	for _, allowed := range []string{"schedule:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "schedule_item:ctx_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "life_context:ctx_cccccccccccccccccccccccccccccccc", "scene:ctx_dddddddddddddddddddddddddddddddd", "presence:ctx_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "goal:ctx_ffffffffffffffffffffffffffffffff", "intention:ctx_11111111111111111111111111111111"} {
		if !strings.Contains(requestBody, allowed) {
			t.Fatalf("Schedule planner Provider request lost %q: %s", allowed, requestBody)
		}
	}
}

func TestScheduleAcceptanceRequestDigestIsStableAndPayloadSensitive(t *testing.T) {
	payload := map[string]any{
		"local_date": "2026-09-10", "timezone": "Asia/Shanghai", "expected_revision": 3,
		"completed_before": "2026-09-10T12:00:00+08:00", "generated_from": "model_replan",
		"source_fact_id": "fact-1", "idempotency_key": "capability:call-1", "reschedule_policy": map[string]any{},
		"items": []any{map[string]any{"start_at": "2026-09-10T12:00:00+08:00", "end_at": "2026-09-11T00:00:00+08:00", "activity": "reading", "scene": "home"}},
	}
	evidence := []any{"fact-1"}
	first := scheduleAcceptanceRequestDigest(payload, evidence)
	second := scheduleAcceptanceRequestDigest(cloneMap(payload), evidence)
	if first == "" || first != second {
		t.Fatalf("digest is not stable: %q %q", first, second)
	}
	changed := cloneMap(payload)
	mapValue(arrayValue(changed["items"])[0])["activity"] = "walking"
	if scheduleAcceptanceRequestDigest(changed, evidence) == first {
		t.Fatal("different schedule payload reused the same request digest")
	}
}

func TestSchedulePlannerUsesStructuredCognitiveRole(t *testing.T) {
	data, err := os.ReadFile("schedule_capability.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, `"schedule_replan_planner"`) || !strings.Contains(source, `"cognitive_assessment"`) {
		t.Fatalf("schedule planner is not bound to the structured cognition role: %s", source)
	}
	if strings.Contains(source, `StructuredWithSchema(ctx, "action_realization"`) {
		t.Fatal("schedule planner must not use the visible action realization role")
	}
}

func TestScheduleReplanDefinitionKeepsPlannerInputThin(t *testing.T) {
	definition := scheduleReplanCapabilityDefinition()
	if !containsSchemaRequired(definition.InputSchema, "intent") {
		t.Fatalf("schedule.replan must require intent: %#v", definition.InputSchema)
	}
	properties := mapValue(definition.InputSchema["properties"])
	for _, field := range []string{"local_date", "expected_revision", "completed_before", "items", "evidence_refs", "timezone"} {
		if _, found := properties[field]; found {
			t.Fatalf("schedule.replan exposes planner field %q: %#v", field, properties)
		}
	}
	if _, ok := (&App{}).capabilityRegistry().Lookup("schedule.replan"); !ok {
		t.Fatal("schedule.replan is not registered in the runtime catalog")
	}
}

func TestValidateScheduleReplanItemsRejectsMissingSemanticFields(t *testing.T) {
	item := map[string]any{
		"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00",
		"activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned",
		"priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.2,
	}
	if err := validateScheduleReplanItems([]any{item}); err != nil {
		t.Fatalf("valid item rejected: %v", err)
	}
	delete(item, "priority")
	if err := validateScheduleReplanItems([]any{item}); err == nil || !strings.Contains(err.Error(), `field "priority" is required`) {
		t.Fatalf("missing priority error = %v", err)
	}
}

func TestValidateCompletedScheduleHistoryPreservesCompletedItems(t *testing.T) {
	current := scheduleSnapshotForTest()
	boundary, _ := time.Parse(time.RFC3339, "2026-09-08T10:00:00+08:00")

	unchanged := scheduleItemsForTest(
		map[string]any{"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.2},
		map[string]any{"start_at": "2026-09-08T08:00:00+08:00", "end_at": "2026-09-08T10:00:00+08:00", "activity": "早餐", "scene": "厨房", "item_type": "planned", "status": "planned", "priority": 0.4, "flexibility": 0.6, "interruption_cost": 0.2},
		map[string]any{"start_at": "2026-09-08T10:00:00+08:00", "end_at": "2026-09-08T14:00:00+08:00", "activity": "突发事件", "scene": "工作室", "item_type": "planned", "status": "planned", "priority": 0.9, "flexibility": 0.1, "interruption_cost": 0.8},
	)
	if err := validateCompletedScheduleHistory(current, map[string]any{"items": unchanged}, boundary); err != nil {
		t.Fatalf("unchanged completed history rejected: %v", err)
	}

	changed := scheduleItemsForTest(
		map[string]any{"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.2},
		map[string]any{"start_at": "2026-09-08T08:00:00+08:00", "end_at": "2026-09-08T10:00:00+08:00", "activity": "被打断的早餐", "scene": "厨房", "item_type": "planned", "status": "planned", "priority": 0.4, "flexibility": 0.6, "interruption_cost": 0.2},
		map[string]any{"start_at": "2026-09-08T10:00:00+08:00", "end_at": "2026-09-08T14:00:00+08:00", "activity": "突发事件", "scene": "工作室", "item_type": "planned", "status": "planned", "priority": 0.9, "flexibility": 0.1, "interruption_cost": 0.8},
	)
	if err := validateCompletedScheduleHistory(current, map[string]any{"items": changed}, boundary); err == nil || !strings.Contains(err.Error(), "completed_history_changed") {
		t.Fatalf("changed completed history error = %v", err)
	}
}

func TestValidateCompletedScheduleHistoryAllowsCurrentItemToBeTruncated(t *testing.T) {
	current := map[string]any{
		"items": scheduleItemsForTest(
			map[string]any{"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.2},
			map[string]any{"start_at": "2026-09-08T08:00:00+08:00", "end_at": "2026-09-08T12:00:00+08:00", "activity": "工作", "scene": "工作室", "item_type": "planned", "status": "planned", "priority": 0.7, "flexibility": 0.4, "interruption_cost": 0.6},
		),
	}
	boundary, _ := time.Parse(time.RFC3339, "2026-09-08T10:00:00+08:00")
	proposal := map[string]any{"items": scheduleItemsForTest(
		map[string]any{"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.2},
		map[string]any{"start_at": "2026-09-08T08:00:00+08:00", "end_at": "2026-09-08T10:00:00+08:00", "activity": "工作", "scene": "工作室", "item_type": "planned", "status": "planned", "priority": 0.7, "flexibility": 0.4, "interruption_cost": 0.6},
		map[string]any{"start_at": "2026-09-08T10:00:00+08:00", "end_at": "2026-09-08T12:00:00+08:00", "activity": "突发事件", "scene": "医院", "item_type": "planned", "status": "planned", "priority": 1.0, "flexibility": 0.0, "interruption_cost": 1.0},
	)}
	if err := validateCompletedScheduleHistory(current, proposal, boundary); err != nil {
		t.Fatalf("truncated current item rejected: %v", err)
	}
}

func TestValidateScheduleReplanBoundaryRejectsStaleOrFutureBoundary(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if err := validateScheduleReplanBoundary(now.Add(-5*time.Minute), now); err != nil {
		t.Fatalf("boundary at the staleness tolerance should be accepted: %v", err)
	}
	if err := validateScheduleReplanBoundary(now.Add(-5*time.Minute-time.Second), now); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale boundary error = %v", err)
	}
	if err := validateScheduleReplanBoundary(now.Add(30*time.Second), now); err != nil {
		t.Fatalf("boundary at the future tolerance should be accepted: %v", err)
	}
	if err := validateScheduleReplanBoundary(now.Add(30*time.Second+time.Second), now); err == nil || !strings.Contains(err.Error(), "future") {
		t.Fatalf("future boundary error = %v", err)
	}
}

func scheduleSnapshotForTest() map[string]any {
	return map[string]any{
		"local_date": "2026-09-08", "timezone": "Asia/Shanghai", "revision": 4,
		"items": scheduleItemsForTest(
			map[string]any{"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": "0.5", "flexibility": "0.5", "interruption_cost": "0.2"},
			map[string]any{"start_at": "2026-09-08T08:00:00+08:00", "end_at": "2026-09-08T10:00:00+08:00", "activity": "早餐", "scene": "厨房", "item_type": "planned", "status": "planned", "priority": "0.4", "flexibility": "0.6", "interruption_cost": "0.2"},
		),
	}
}

func scheduleItemsForTest(items ...map[string]any) []any {
	result := make([]any, len(items))
	for index, item := range items {
		result[index] = item
	}
	return result
}
