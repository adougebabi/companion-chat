package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// independentToolProductInventory is copied from the product baseline in
// .trellis/tasks/09-22-agent-tool-plugin-e2e/research/inventory.md.  Keep this
// list explicit: deriving the expectation from the registry would let a
// removed product Tool silently shrink this suite.
var independentToolProductInventory = []string{
	"active_memory_event",
	"affect_event",
	"capability.request",
	"conversation.reply",
	"media.image.generate",
	"memory.recall",
	"memory_event",
	"moment.publish",
	"persona.switch",
	"persona.takeover",
	"presence_event",
	"relationship.lookup",
	"scene_event",
	"schedule.replan",
	"visual_identity.commit_review",
	"visual_identity.finalize",
	"visual_identity.generate_candidate",
	"visual_identity.initialize",
}

type independentToolE2EFixture struct {
	ctx            context.Context
	repository     *PostgresRepository
	app            *App
	ownerID        string
	foreignOwnerID string
	fluctlightID   string
	conversationID string
	suffix         string
}

type independentMutationToolMatrix struct {
	request              ToolExecutionRequest
	wantStatus           string
	product              func(*testing.T, ToolExecutionReceipt)
	replayIdentity       func(ToolExecutionReceipt) string
	businessFailure      ToolExecutionRequest
	checkBusinessError   func(*testing.T, ToolExecutionReceipt, error)
	conflictingArgs      json.RawMessage
	mutateConflict       func(*ToolExecutionRequest)
	dependencyApp        func(*testing.T) *App
	checkDependencyError func(*testing.T, ToolExecutionReceipt, error)
}

func TestIndependentToolE2EFixedProductInventory(t *testing.T) {
	registry, err := NewCapabilityRegistry(builtinCapabilities(&App{})...)
	if err != nil {
		t.Fatal(err)
	}
	got := capabilityDefinitionNames(registry.Definitions())
	want := append([]string(nil), independentToolProductInventory...)
	sort.Strings(want)
	if !phase8EqualStrings(got, want) {
		t.Fatalf("fixed product Tool inventory drifted: got=%v want=%v", got, want)
	}
	for _, name := range []string{"scene_event", "presence_event", "schedule.replan", "affect_event", "relationship.lookup", "capability.request", "visual_identity.initialize", "visual_identity.generate_candidate", "visual_identity.commit_review", "visual_identity.finalize"} {
		if _, ok := registry.Definition(name); !ok {
			t.Errorf("remaining independent Tool %q is absent from the fixed 18-Tool product registry", name)
		}
	}
}

func TestIndependentToolE2ESceneEvent(t *testing.T) {
	fixture := newIndependentToolE2EFixture(t, "scene")
	request := fixture.request("scene_event", "scene-start", map[string]any{
		"operation": "start", "scene": "阳台", "activity": "阅读", "location": "家", "confidence": 0.95,
	})
	runIndependentMutationToolMatrix(t, fixture, independentMutationToolMatrix{
		request: request, wantStatus: "completed",
		replayIdentity: func(receipt ToolExecutionReceipt) string {
			return stringValue(mapValue(receipt.Result.Output)["event_id"])
		},
		product: func(t *testing.T, receipt ToolExecutionReceipt) {
			t.Helper()
			eventID := stringValue(mapValue(receipt.Result.Output)["event_id"])
			var scene, activity, status string
			var revision, outbox, fact int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT scene,activity,status,revision FROM public.life_events WHERE id=$1 AND fluctlight_id=$2`, eventID, fixture.fluctlightID).Scan(&scene, &activity, &status, &revision); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind='life.scene.updated' AND payload->>'event_id'=$2`, fixture.fluctlightID, eventID).Scan(&outbox); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1 AND event_type='life.scene.updated'`, fixture.fluctlightID).Scan(&fact); err != nil {
				t.Fatal(err)
			}
			if scene != "阳台" || activity != "阅读" || status != "inferred" || revision != 1 || outbox != 1 || fact != 1 {
				t.Fatalf("scene product scene=%q activity=%q status=%q revision=%d outbox=%d native_facts=%d", scene, activity, status, revision, outbox, fact)
			}
			fixture.requireNoModelRuns(t)
		},
		businessFailure: fixture.request("scene_event", "scene-start-while-active", map[string]any{
			"operation": "start", "scene": "厨房", "activity": "做饭", "confidence": 1.0,
		}),
		checkBusinessError:   expectIndependentToolFailure("scene_start_requires_no_active_event"),
		conflictingArgs:      jsonBytes(map[string]any{"operation": "start", "scene": "厨房", "activity": "做饭", "confidence": 0.95}),
		dependencyApp:        func(t *testing.T) *App { return fixture.faultApp(t, sceneEventCapability{}) },
		checkDependencyError: expectIndependentToolFailure("capability_prepare_failed"),
	})
}

func TestIndependentToolE2EPresenceEvent(t *testing.T) {
	fixture := newIndependentToolE2EFixture(t, "presence")
	request := fixture.request("presence_event", "presence-set", map[string]any{
		"operation": "set", "user_presence": "online", "current_task": "核对发布清单", "confidence": 0.9,
	})
	runIndependentMutationToolMatrix(t, fixture, independentMutationToolMatrix{
		request: request, wantStatus: "completed",
		replayIdentity: func(receipt ToolExecutionReceipt) string {
			return stringValue(mapValue(receipt.Result.Output)["overlay_id"])
		},
		product: func(t *testing.T, receipt ToolExecutionReceipt) {
			t.Helper()
			overlayID := stringValue(mapValue(receipt.Result.Output)["overlay_id"])
			var actorID, task, presence, status string
			var revision, outbox, fact int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT actor_id,current_task,user_presence,status,revision FROM public.life_presence_overlays WHERE id=$1 AND fluctlight_id=$2`, overlayID, fixture.fluctlightID).Scan(&actorID, &task, &presence, &status, &revision); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind='life.presence.updated' AND payload->>'overlay_id'=$2`, fixture.fluctlightID, overlayID).Scan(&outbox); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1 AND event_type='life.presence.updated'`, fixture.fluctlightID).Scan(&fact); err != nil {
				t.Fatal(err)
			}
			if actorID != fixture.ownerID || task != "核对发布清单" || presence != "online" || status != "active" || revision != 1 || outbox != 1 || fact != 1 {
				t.Fatalf("presence product actor=%q task=%q presence=%q status=%q revision=%d outbox=%d native_facts=%d", actorID, task, presence, status, revision, outbox, fact)
			}
			fixture.requireNoModelRuns(t)
		},
		businessFailure: fixture.request("presence_event", "presence-expired", map[string]any{
			"operation": "set", "user_presence": "online", "expires_at": time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), "confidence": 0.8,
		}),
		checkBusinessError:   expectIndependentToolFailure("presence_expiration_invalid"),
		conflictingArgs:      jsonBytes(map[string]any{"operation": "set", "user_presence": "away", "current_task": "不同任务", "confidence": 0.9}),
		dependencyApp:        func(t *testing.T) *App { return fixture.faultApp(t, presenceEventCapability{}) },
		checkDependencyError: expectIndependentToolFailure("capability_prepare_failed"),
	})
}

func TestIndependentToolE2EScheduleReplan(t *testing.T) {
	if strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST")) != "1" {
		t.Skip("FLUCTLIGHT_LIVE_PROVIDER_TEST=1 is required for the final serial schedule.replan live E2E")
	}
	fixture := newIndependentToolE2EFixture(t, "schedule")
	liveCtx, liveCancel := context.WithTimeout(context.Background(), liveProviderRequestTimeout()+2*time.Minute)
	t.Cleanup(liveCancel)
	fixture.ctx = liveCtx
	fixture.seedAcceptedSchedule(t)
	fixture.configureLiveScheduleProvider(t)
	request := fixture.request("schedule.replan", "schedule-replan", map[string]any{
		"intent": "保持现有计划的语义字段完全不变。必须原样保留 completed_before 之前的历史；若当前项跨越 completed_before，只在该边界截断它，并用完全相同的 activity、scene、location、item_type、status、priority、flexibility、interruption_cost 重建后半段。返回覆盖完整本地日的计划。",
	})
	runIndependentMutationToolMatrix(t, fixture, independentMutationToolMatrix{
		request: request, wantStatus: "completed",
		replayIdentity: func(receipt ToolExecutionReceipt) string {
			return stringValue(mapValue(receipt.Result.Output)["schedule_id"])
		},
		product: func(t *testing.T, receipt ToolExecutionReceipt) {
			t.Helper()
			output := mapValue(receipt.Result.Output)
			scheduleID := stringValue(output["schedule_id"])
			var status string
			var revision, items, accepted, plannerRuns int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status,revision FROM public.life_schedules WHERE id=$1 AND fluctlight_id=$2`, scheduleID, fixture.fluctlightID).Scan(&status, &revision); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.life_schedule_items WHERE schedule_id=$1`, scheduleID).Scan(&items); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.life_schedules WHERE fluctlight_id=$1 AND status='accepted'`, fixture.fluctlightID).Scan(&accepted); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.diagnostic_model_runs WHERE scenario='schedule_replan_planner' AND status='completed'`).Scan(&plannerRuns); err != nil {
				t.Fatal(err)
			}
			if status != "accepted" || revision < 2 || items == 0 || accepted != 1 || plannerRuns < 1 {
				t.Fatalf("schedule product id=%q status=%q revision=%d items=%d accepted=%d planner_runs=%d", scheduleID, status, revision, items, accepted, plannerRuns)
			}
			var unrelatedRuns int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.diagnostic_model_runs WHERE scenario<>'schedule_replan_planner'`).Scan(&unrelatedRuns); err != nil {
				t.Fatal(err)
			}
			if unrelatedRuns != 0 {
				t.Fatalf("independent schedule Tool started %d unrelated/Main model runs", unrelatedRuns)
			}
		},
		businessFailure:      fixture.request("schedule.replan", "schedule-empty-intent", map[string]any{"intent": ""}),
		checkBusinessError:   expectIndependentToolFailure("capability_prepare_failed"),
		conflictingArgs:      jsonBytes(map[string]any{"intent": "同一业务 operation 不得接受不同的剩余日程计划。"}),
		dependencyApp:        func(t *testing.T) *App { return fixture.faultApp(t, scheduleReplanCapability{}) },
		checkDependencyError: expectIndependentToolFailure("schedule_replan_planner_failed"),
	})
}

func TestIndependentToolE2EAffectEvent(t *testing.T) {
	fixture := newIndependentToolE2EFixture(t, "affect")
	request := fixture.request("affect_event", "affect-happy", map[string]any{
		"event": map[string]any{"type": "happy", "confidence": 0.8},
	})
	runIndependentMutationToolMatrix(t, fixture, independentMutationToolMatrix{
		request: request, wantStatus: "completed",
		replayIdentity: func(receipt ToolExecutionReceipt) string {
			return stringValue(mapValue(receipt.Result.Output)["event_id"])
		},
		product: func(t *testing.T, receipt ToolExecutionReceipt) {
			t.Helper()
			eventID := stringValue(mapValue(receipt.Result.Output)["event_id"])
			var eventType string
			var revision, stateRevision, outbox int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT event_type,revision FROM public.fluctlight_inner_state_events WHERE id=$1 AND fluctlight_id=$2`, eventID, fixture.fluctlightID).Scan(&eventType, &revision); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&stateRevision); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind='affect.updated' AND payload->>'event_id'=$2`, fixture.fluctlightID, eventID).Scan(&outbox); err != nil {
				t.Fatal(err)
			}
			if eventType != "affect.event" || revision != 1 || stateRevision != 1 || outbox != 1 {
				t.Fatalf("affect product type=%q event_revision=%d state_revision=%d outbox=%d", eventType, revision, stateRevision, outbox)
			}
			fixture.requireNoModelRuns(t)
		},
		businessFailure: fixture.request("affect_event", "affect-invalid-type", map[string]any{
			"event": map[string]any{"type": "not-a-real-affect", "confidence": 0.8},
		}),
		checkBusinessError:   expectIndependentToolFailure("capability_prepare_failed"),
		conflictingArgs:      jsonBytes(map[string]any{"event": map[string]any{"type": "sad", "confidence": 0.8}}),
		dependencyApp:        func(t *testing.T) *App { return fixture.faultApp(t, affectEventCapability{}) },
		checkDependencyError: expectIndependentToolFailure("affect_capability_unavailable"),
	})
}

func TestIndependentToolE2ERelationshipLookup(t *testing.T) {
	fixture := newIndependentToolE2EFixture(t, "relationship")
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,profile_id,target_actor_id,role,metrics,interaction_frequency,trend,summary,emotional_association,provenance,revision) VALUES($1,$2,NULL,$3,'{"label":"owner"}','{"trust":0.82}',3,'stable','真实关系摘要','{"tone":"warm"}','{"source":"e2e"}',4)`, "relationship-e2e-"+fixture.suffix, fixture.fluctlightID, fixture.ownerID); err != nil {
		t.Fatal(err)
	}
	request := fixture.request("relationship.lookup", "relationship-lookup", map[string]any{"target_actor_id": fixture.ownerID})
	var first ToolExecutionReceipt
	t.Run("direct_success_without_native_call", func(t *testing.T) {
		var err error
		first, err = fixture.app.ExecuteTool(fixture.ctx, request)
		if err != nil || first.Result.Status != "completed" || first.NativeToolCallID != "" || !strings.HasPrefix(first.ExecutionCallID, "direct_call_") {
			t.Fatalf("relationship direct receipt=%#v err=%v", first, err)
		}
	})
	t.Run("durable_product", func(t *testing.T) {
		output := mapValue(first.Result.Output)
		if stringValue(output["target_actor_id"]) != fixture.ownerID || stringValue(output["summary"]) != "真实关系摘要" || intValue(output["revision"]) != 4 || numberOrZero(mapValue(output["metrics"])["trust"]) != 0.82 {
			t.Fatalf("relationship output does not match PostgreSQL authority: %#v", output)
		}
		var summary string
		if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT summary FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2`, fixture.fluctlightID, fixture.ownerID).Scan(&summary); err != nil || summary != "真实关系摘要" {
			t.Fatalf("relationship database summary=%q err=%v", summary, err)
		}
		fixture.requireNoModelRuns(t)
	})
	t.Run("business_failure", func(t *testing.T) {
		bad := fixture.request("relationship.lookup", "relationship-forbidden", map[string]any{"target_actor_id": fixture.foreignOwnerID})
		receipt, err := fixture.app.ExecuteTool(fixture.ctx, bad)
		if err == nil || receipt.Result.Status != "failed" || receipt.Result.ErrorCode != "relationship_lookup_target_forbidden" || !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("forbidden relationship receipt=%#v err=%v", receipt, err)
		}
	})
	t.Run("repeat_query_is_read_only", func(t *testing.T) {
		repeated, err := fixture.app.ExecuteTool(fixture.ctx, request)
		if err != nil || repeated.Result.Status != "completed" || repeated.Replayed || stringValue(mapValue(repeated.Result.Output)["summary"]) != "真实关系摘要" {
			t.Fatalf("repeated relationship query receipt=%#v err=%v", repeated, err)
		}
		var ledgerRows int
		if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='relationship.lookup'`, fixture.fluctlightID).Scan(&ledgerRows); err != nil || ledgerRows != 0 {
			t.Fatalf("pure relationship query ledger rows=%d err=%v", ledgerRows, err)
		}
	})
	t.Run("native_identity_is_only_correlation", func(t *testing.T) {
		native := request
		native.NativeToolCallID = "native-relationship-" + fixture.suffix
		native.ProviderRequestID = "provider-relationship-" + fixture.suffix
		receipt, err := fixture.app.ExecuteTool(fixture.ctx, native)
		if err != nil || receipt.ExecutionCallID != native.NativeToolCallID || receipt.NativeToolCallID != native.NativeToolCallID || receipt.Replayed {
			t.Fatalf("relationship native correlation receipt=%#v err=%v", receipt, err)
		}
	})
	t.Run("dependency_failure", func(t *testing.T) {
		fault := fixture.faultApp(t, relationshipLookupCapability{})
		receipt, err := fault.ExecuteTool(fixture.ctx, fixture.request("relationship.lookup", "relationship-dependency", map[string]any{"target_actor_id": fixture.ownerID}))
		if err == nil || receipt.Result.Status != "failed" || receipt.Result.ErrorCode != "relationship_capability_unavailable" {
			t.Fatalf("relationship dependency receipt=%#v err=%v", receipt, err)
		}
	})
}

func TestIndependentToolE2ECapabilityRequest(t *testing.T) {
	fixture := newIndependentToolE2EFixture(t, "capability-request")
	request := fixture.request("capability.request", "capability-propose", map[string]any{
		"capability_key": "calendar.freebusy", "title": "读取空闲时间", "description": "读取授权日历的空闲时间段", "rationale": "用于避免安排冲突", "priority": "normal",
		"desired_contract": map[string]any{"input": "date_range", "output": "busy_intervals"},
	})
	runIndependentMutationToolMatrix(t, fixture, independentMutationToolMatrix{
		request: request, wantStatus: "completed",
		replayIdentity: func(receipt ToolExecutionReceipt) string {
			return stringValue(mapValue(receipt.Result.Output)["request_id"])
		},
		product: func(t *testing.T, receipt ToolExecutionReceipt) {
			t.Helper()
			requestID := stringValue(mapValue(receipt.Result.Output)["request_id"])
			var key, title, status, sourceFact string
			var outbox int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT capability_key,title,status,source_fact_id FROM public.capability_requests WHERE id=$1 AND fluctlight_id=$2`, requestID, fixture.fluctlightID).Scan(&key, &title, &status, &sourceFact); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE kind='capability.requested' AND aggregate_id=$1`, requestID).Scan(&outbox); err != nil {
				t.Fatal(err)
			}
			if key != "calendar.freebusy" || title != "读取空闲时间" || status != "proposed" || sourceFact != request.EvidenceID || outbox != 1 {
				t.Fatalf("capability request product key=%q title=%q status=%q source=%q outbox=%d", key, title, status, sourceFact, outbox)
			}
			fixture.requireNoModelRuns(t)
		},
		businessFailure: fixture.request("capability.request", "capability-sensitive-contract", map[string]any{
			"capability_key": "calendar.write", "title": "写入日历", "description": "创建事件", "rationale": "安排计划", "desired_contract": map[string]any{"api_key": "must-not-persist"},
		}),
		checkBusinessError: expectIndependentToolFailure("capability_request_contract_invalid"),
		conflictingArgs: jsonBytes(map[string]any{
			"capability_key": "calendar.freebusy", "title": "不同标题", "description": "读取授权日历的空闲时间段", "rationale": "用于避免安排冲突", "priority": "normal",
			"desired_contract": map[string]any{"input": "date_range", "output": "busy_intervals"},
		}),
		dependencyApp:        func(t *testing.T) *App { return fixture.faultApp(t, capabilityRequestCapability{}) },
		checkDependencyError: expectIndependentToolFailure("capability_request_unavailable"),
	})
}

func TestIndependentToolE2EVisualIdentityInitialize(t *testing.T) {
	fixture := newIndependentToolE2EFixture(t, "visual-identity")
	persona := map[string]any{
		"identity":           map[string]any{"name": "岚音", "age": 24, "gender": "female", "nationality": "Chinese"},
		"life_profile":       map[string]any{"appearance": map[string]any{"hair": "black shoulder-length", "body_type": "slender", "chest_cup": "B"}},
		"personality_system": map[string]any{"active_profile_id": "default"},
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, fixture.fluctlightID, jsonBytes(persona)); err != nil {
		t.Fatal(err)
	}
	request := fixture.request("visual_identity.initialize", "visual-initialize", map[string]any{})
	runIndependentMutationToolMatrix(t, fixture, independentMutationToolMatrix{
		request: request, wantStatus: "accepted",
		replayIdentity: func(receipt ToolExecutionReceipt) string {
			return stringValue(mapValue(receipt.Result.Output)["session_id"])
		},
		product: func(t *testing.T, receipt ToolExecutionReceipt) {
			t.Helper()
			sessionID := stringValue(mapValue(receipt.Result.Output)["session_id"])
			var sessionStatus, trigger, workflowID, attemptStatus, intentStatus string
			var attempts, timeline, intents int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status,trigger_type,workflow_id FROM public.fluctlight_visual_identity_sessions WHERE id=$1 AND fluctlight_id=$2`, sessionID, fixture.fluctlightID).Scan(&sessionStatus, &trigger, &workflowID); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*),min(status) FROM public.fluctlight_visual_identity_attempts WHERE session_id=$1`, sessionID).Scan(&attempts, &attemptStatus); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_visual_identity_timeline WHERE session_id=$1 AND stage='session_created'`, sessionID).Scan(&timeline); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*),min(status) FROM public.platform_workflow_intents WHERE workflow_id=$1 AND intent_type='visual_identity.initialize'`, workflowID).Scan(&intents, &intentStatus); err != nil {
				t.Fatal(err)
			}
			if sessionStatus != "queued" || trigger != "initialization" || attempts != 1 || attemptStatus != "queued" || timeline != 1 || intents != 1 || intentStatus != "pending" {
				t.Fatalf("visual accepted product session=%q/%q attempts=%d/%q timeline=%d intent=%d/%q", sessionStatus, trigger, attempts, attemptStatus, timeline, intents, intentStatus)
			}
			fixture.requireNoModelRuns(t)
		},
		businessFailure: ToolExecutionRequest{
			CapabilityName: "visual_identity.initialize", OperationID: "visual-unowned-" + fixture.suffix,
			AuthorizationActorID: fixture.foreignOwnerID, FluctlightID: fixture.fluctlightID, Arguments: json.RawMessage(`{}`),
		},
		checkBusinessError: func(t *testing.T, receipt ToolExecutionReceipt, err error) {
			t.Helper()
			if err == nil || !errors.Is(err, ErrNotFound) {
				t.Fatalf("unowned visual initialization receipt=%#v err=%v", receipt, err)
			}
		},
		conflictingArgs: json.RawMessage(`{}`),
		mutateConflict: func(request *ToolExecutionRequest) {
			request.TargetKind = "visual_identity"
			request.TargetRef = "different-target"
		},
		dependencyApp:        func(t *testing.T) *App { return fixture.faultApp(t, visualIdentityInitializeCapability{}) },
		checkDependencyError: expectIndependentToolFailure("visual_identity_unavailable"),
	})
}

func newIndependentToolE2EFixture(t *testing.T, label string) independentToolE2EFixture {
	t.Helper()
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := strings.ReplaceAll(label, "-", "_") + "_" + fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID := "independent_owner_" + suffix
	fluctlightID := "independent_fluctlight_" + suffix
	conversationID := "independent_conversation_" + suffix
	seedTurnConversation(t, ctx, repository, ownerID, fluctlightID, conversationID)
	foreignOwnerID := "independent_foreign_" + suffix
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, foreignOwnerID); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repository, nil)
	return independentToolE2EFixture{ctx: ctx, repository: repository, app: app, ownerID: ownerID, foreignOwnerID: foreignOwnerID, fluctlightID: fluctlightID, conversationID: conversationID, suffix: suffix}
}

func (fixture independentToolE2EFixture) request(capabilityName, operation string, arguments map[string]any) ToolExecutionRequest {
	return ToolExecutionRequest{
		CapabilityName: capabilityName, OperationID: operation + "-" + fixture.suffix,
		AuthorizationActorID: fixture.ownerID, FluctlightID: fixture.fluctlightID, ConversationID: fixture.conversationID,
		EvidenceID: "owner-command-" + operation + "-" + fixture.suffix,
		Surface:    CapabilitySurfaceNativeCognition, Arguments: jsonBytes(arguments),
	}
}

func (fixture independentToolE2EFixture) faultApp(t *testing.T, capability Capability) *App {
	t.Helper()
	app := &App{DB: fixture.repository}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = mustCapabilityRegistry(capability)
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	return app
}

func (fixture independentToolE2EFixture) requireNoModelRuns(t *testing.T) {
	t.Helper()
	var count int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.diagnostic_model_runs`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("independent Tool unexpectedly started %d model/Main runs", count)
	}
}

func (fixture independentToolE2EFixture) seedAcceptedSchedule(t *testing.T) {
	t.Helper()
	initialLife := currentLifeForTest(t, fixture.ctx, fixture.app, fixture.fluctlightID, time.Now().UTC())
	payload := fullDaySchedulePayloadForTest(time.Now().UTC(), "independent-schedule-base-"+fixture.suffix, stringValue(initialLife["context_revision"]))
	if _, err := fixture.app.AcceptSchedule(fixture.ctx, fixture.ownerID, fixture.fluctlightID, payload); err != nil {
		t.Fatal(err)
	}
}

func (fixture independentToolE2EFixture) configureLiveScheduleProvider(t *testing.T) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST")) != "1" {
		t.Fatal("FLUCTLIGHT_LIVE_PROVIDER_TEST=1 is required for the real schedule.replan Tool E2E")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_URL")), "/")
	model := strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_MODEL"))
	if baseURL == "" || model == "" {
		t.Fatal("FLUCTLIGHT_LIVE_PROVIDER_URL and FLUCTLIGHT_LIVE_PROVIDER_MODEL are required for schedule.replan E2E")
	}
	endpointID, purpose := "independent-schedule-endpoint-"+fixture.suffix, "independent-schedule-secret-"+fixture.suffix
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible',$2,$3,'ready',now())`, endpointID, baseURL, purpose); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,$2,'structured_output',4096,600,'{}')`, endpointID, model); err != nil {
		t.Fatal(err)
	}
	fixture.app.SettingsKey = bytes.Repeat([]byte{0x51}, 32)
	fixture.app.Provider.SettingsKey = fixture.app.SettingsKey
	fixture.app.Provider.HTTP = &http.Client{Timeout: liveProviderRequestTimeout()}
	if secret := strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_API_KEY")); secret != "" {
		encrypted, err := encryptSecret(fixture.app.SettingsKey, purpose, secret)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES($1,$2,$3)`, purpose, encrypted.ciphertext, encrypted.nonce); err != nil {
			t.Fatal(err)
		}
	}
}

func runIndependentMutationToolMatrix(t *testing.T, fixture independentToolE2EFixture, matrix independentMutationToolMatrix) {
	t.Helper()
	var first ToolExecutionReceipt
	t.Run("direct_success_without_native_call", func(t *testing.T) {
		var err error
		first, err = fixture.app.ExecuteTool(fixture.ctx, matrix.request)
		if err != nil || first.Result.Status != matrix.wantStatus || first.NativeToolCallID != "" || !strings.HasPrefix(first.ExecutionCallID, "direct_call_") {
			t.Fatalf("direct %s receipt=%#v err=%v", matrix.request.CapabilityName, first, err)
		}
	})
	t.Run("durable_product", func(t *testing.T) {
		if matrix.product == nil {
			t.Fatal("durable product verifier is required")
		}
		matrix.product(t, first)
	})
	t.Run("business_failure", func(t *testing.T) {
		receipt, err := fixture.app.ExecuteTool(fixture.ctx, matrix.businessFailure)
		if matrix.checkBusinessError == nil {
			t.Fatal("business failure verifier is required")
		}
		matrix.checkBusinessError(t, receipt, err)
	})
	t.Run("operation_replay_with_new_native_correlation", func(t *testing.T) {
		replay := matrix.request
		replay.NativeToolCallID = "native-replay-" + fixture.suffix + "-" + strings.ReplaceAll(matrix.request.CapabilityName, ".", "-")
		replay.ProviderRequestID = "provider-replay-" + fixture.suffix + "-" + strings.ReplaceAll(matrix.request.CapabilityName, ".", "-")
		receipt, err := fixture.app.ExecuteTool(fixture.ctx, replay)
		if err != nil || receipt.Result.Status != matrix.wantStatus || !receipt.Replayed || receipt.ExecutionCallID != replay.NativeToolCallID || receipt.NativeToolCallID != replay.NativeToolCallID {
			t.Fatalf("operation replay %s receipt=%#v err=%v", matrix.request.CapabilityName, receipt, err)
		}
		if matrix.replayIdentity == nil || matrix.replayIdentity(first) == "" || matrix.replayIdentity(receipt) != matrix.replayIdentity(first) {
			t.Fatalf("operation replay %s changed durable identity: first=%#v replay=%#v", matrix.request.CapabilityName, first.Result.Output, receipt.Result.Output)
		}
	})
	t.Run("same_operation_payload_conflict", func(t *testing.T) {
		conflict := matrix.request
		conflict.Arguments = append(json.RawMessage(nil), matrix.conflictingArgs...)
		if matrix.mutateConflict != nil {
			matrix.mutateConflict(&conflict)
		}
		conflict.NativeToolCallID = "native-conflict-" + fixture.suffix + "-" + strings.ReplaceAll(matrix.request.CapabilityName, ".", "-")
		conflict.ProviderRequestID = "provider-conflict-" + fixture.suffix + "-" + strings.ReplaceAll(matrix.request.CapabilityName, ".", "-")
		receipt, err := fixture.app.ExecuteTool(fixture.ctx, conflict)
		if err == nil || !errors.Is(err, ErrConflict) || receipt.Result.Status != "failed" || receipt.Result.ErrorCode != "operation_id_conflict" {
			t.Fatalf("operation conflict %s receipt=%#v err=%v", matrix.request.CapabilityName, receipt, err)
		}
	})
	t.Run("dependency_failure", func(t *testing.T) {
		if matrix.dependencyApp == nil || matrix.checkDependencyError == nil {
			t.Fatal("dependency fault and verifier are required")
		}
		dependencyRequest := matrix.request
		dependencyRequest.OperationID += "-dependency"
		receipt, err := matrix.dependencyApp(t).ExecuteTool(fixture.ctx, dependencyRequest)
		matrix.checkDependencyError(t, receipt, err)
	})
}

func expectIndependentToolFailure(code string) func(*testing.T, ToolExecutionReceipt, error) {
	return func(t *testing.T, receipt ToolExecutionReceipt, err error) {
		t.Helper()
		if err == nil || receipt.Result.Status != "failed" || receipt.Result.ErrorCode != code {
			t.Fatalf("Tool failure code=%q receipt=%#v err=%v", code, receipt, err)
		}
	}
}
