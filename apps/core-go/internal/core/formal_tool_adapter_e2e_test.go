package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// formalToolAdapterInventory is a product baseline, not a projection of the
// current registry. A removed registration must leave an explicit failing row.
var formalToolAdapterInventory = []string{
	"active_memory_event",
	"affect_event",
	"capability.request",
	"conversation.reply",
	"media.image.generate",
	"memory.recall",
	"memory_event",
	"moment.publish",
	"persona.detail",
	"persona.switch",
	"persona.takeover",
	"schedule.replan",
	"presence_event",
	"relationship.lookup",
	"scene_event",
	"visual_identity.initialize",
	"visual_identity.generate_candidate",
	"visual_identity.commit_review",
	"visual_identity.finalize",
}

type formalToolAdapterCase struct {
	name       string
	surface    CapabilitySurface
	wantStatus string
	request    func(*testing.T, *formalToolAdapterFixture, string) ToolExecutionRequest
	verify     func(*testing.T, *formalToolAdapterFixture, ToolExecutionReceipt)
}

type formalToolAdapterFixture struct {
	independentToolE2EFixture
	provider *controlledFormalToolProvider
	server   *httptest.Server

	visualSessionID        string
	visualCandidateIntent  string
	visualCharacterIntent  string
	visualCandidateAssetID string
	memorySecret           string
}

// TestFormalToolEinoAdapterE2E executes every fixed product Tool once through
// the direct production boundary, then replays the same stable operation
// through an actual OpenAI-compatible HTTP response, Eino ChatModelAgent,
// NewADKCapabilityTools, appADKCapabilityInvoker, and App.ExecuteTool. Every
// row observes two physical controlled model calls and the exact serialized
// Tool receipt in the second request.
func TestFormalToolEinoAdapterE2E(t *testing.T) {
	if strings.TrimSpace(testEnvironment("GO_CORE_TEST_DATABASE_URL")) == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is required for the isolated PostgreSQL adapter E2E")
	}
	fixture := newFormalToolAdapterFixture(t)
	cases := formalToolAdapterCases()
	if len(cases) != len(formalToolAdapterInventory) {
		t.Fatalf("formal adapter matrix has %d rows, want fixed %d", len(cases), len(formalToolAdapterInventory))
	}
	for index, expected := range formalToolAdapterInventory {
		if cases[index].name != expected {
			t.Fatalf("formal adapter matrix row %d=%q, want fixed product Tool %q", index, cases[index].name, expected)
		}
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			operationRoot := "formal-adapter-run-" + strings.ReplaceAll(testCase.name, ".", "-") + "-" + fixture.suffix
			request := testCase.request(t, fixture, operationRoot)
			request.OperationID = formalAdapterOperationID(operationRoot, testCase.name)
			if request.TargetKind == "" && request.TargetRef == "" && request.ConversationID != "" {
				request.TargetKind, request.TargetRef = "conversation", request.ConversationID
			}
			if request.CapabilityName != testCase.name {
				t.Fatalf("case %q built request for %q", testCase.name, request.CapabilityName)
			}
			t.Run("dependency_failure", func(t *testing.T) {
				missing := &App{DB: fixture.repository, ContextResolver: fixture.app.ContextResolver, Capabilities: mustCapabilityRegistry(builtinCapabilities(nil)...)}
				failedRequest := request
				failedRequest.OperationID += "-dependency"
				receipt, err := missing.ExecuteTool(fixture.ctx, failedRequest)
				if err == nil || receipt.Result.Status == "completed" || receipt.Result.Status == "accepted" {
					t.Fatalf("missing business dependency returned false success: %#v %v", receipt, err)
				}
				requireFormalAdapterSQLCountArgs(t, fixture, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND operation_id=$2`, 0, request.FluctlightID, failedRequest.OperationID)
			})
			t.Run("foreign_owner_rejected", func(t *testing.T) {
				foreignOwner := "foreign-adapter-owner-" + fixture.suffix
				if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active') ON CONFLICT DO NOTHING`, foreignOwner); err != nil {
					t.Fatal(err)
				}
				foreignRequest := request
				foreignRequest.OperationID += "-foreign"
				foreignRequest.AuthorizationActorID = foreignOwner
				if receipt, err := fixture.app.ExecuteTool(fixture.ctx, foreignRequest); err == nil || receipt.Result.Status == "completed" || receipt.Result.Status == "accepted" {
					t.Fatalf("foreign owner accepted: %#v %v", receipt, err)
				}
				requireFormalAdapterSQLCountArgs(t, fixture, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND operation_id=$2`, 0, request.FluctlightID, foreignRequest.OperationID)
			})
			direct, err := fixture.app.ExecuteTool(fixture.ctx, request)
			if err != nil || direct.Result.Status != testCase.wantStatus {
				t.Fatalf("direct production execution status=%q receipt=%#v err=%v", direct.Result.Status, direct, err)
			}
			if direct.NativeToolCallID != "" || !strings.HasPrefix(direct.ExecutionCallID, "direct_call_") {
				t.Fatalf("direct execution fabricated native identity: %#v", direct)
			}
			if testCase.verify == nil {
				t.Fatal("independent product verifier is missing")
			}
			testCase.verify(t, fixture, direct)

			callID := "controlled-native-" + strings.ReplaceAll(testCase.name, ".", "-")
			fixture.provider.begin(testCase.name, callID, request.Arguments)
			definition, ok := fixture.app.capabilityRegistry().Definition(testCase.name)
			if !ok {
				t.Fatalf("fixed product Tool %q is absent from the production registry", testCase.name)
			}
			responseSchema := objectSchema(map[string]any{"answer": stringSchema()}, []string{"answer"}, false)
			run, runErr := fixture.app.RunFormalAgent(
				WithProviderCorrelation(fixture.ctx, "controlled-formal-tool-adapter:"+testCase.name+":"+fixture.suffix),
				FormalAgentConversationCognition,
				FormalAgentRunInput{
					Prompt: PromptAssemblyResult{
						Messages: []map[string]any{
							{"role": "system", "content": "Controlled adapter evidence: call the single supplied Tool once, then return the final DTO."},
							{"role": "user", "content": "Execute the authorized operation."},
						},
						ResponseFormat: responseSchema,
					},
					Definitions: []CapabilityDefinition{definition}, SchemaName: "conversation_turn_response",
					Capability: &ADKCapabilityRequest{
						AuthorizationActorID: fixture.ownerID, FluctlightID: fixture.fluctlightID,
						ConversationID: request.ConversationID, SourceFactID: request.EvidenceID,
						ActionID:    "formal-adapter-action-" + strings.ReplaceAll(testCase.name, ".", "-"),
						OperationID: operationRoot, CorrelationID: "controlled-formal-tool-adapter:" + testCase.name,
						TargetKind: request.TargetKind, TargetRef: request.TargetRef, Surface: testCase.surface,
					},
				},
			)
			if runErr != nil {
				t.Fatalf("formal Eino adapter run failed: %v", runErr)
			}
			if stringValue(run.Completion.Structured["answer"]) != "adapter-ok:"+testCase.name {
				t.Fatalf("controlled final DTO=%#v", run.Completion.Structured)
			}
			if run.Trace == nil {
				t.Fatal("formal adapter omitted request-scoped Tool trace")
			}
			invocations, results := run.Trace.Snapshot()
			if len(invocations) != 1 || len(results) != 1 {
				t.Fatalf("formal adapter trace invocations=%d results=%d", len(invocations), len(results))
			}
			if invocations[0].CallID != callID || invocations[0].CapabilityName != testCase.name || invocations[0].Metadata.OperationID != request.OperationID {
				t.Fatalf("formal invocation identity mismatch: %#v request=%#v", invocations[0], request)
			}
			if invocations[0].Metadata.AuthorizationActorID != fixture.ownerID || invocations[0].Metadata.SubjectActorID != fixture.ownerID {
				t.Fatalf("formal invocation lost authorized reader identity: %#v", invocations[0].Metadata)
			}
			if string(invocations[0].Arguments) != string(request.Arguments) {
				t.Fatalf("adapter changed arguments: got=%s want=%s", invocations[0].Arguments, request.Arguments)
			}
			if results[0].CallID != callID || results[0].CapabilityName != testCase.name || results[0].Status != direct.Result.Status {
				t.Fatalf("formal result identity/status mismatch: %#v direct=%#v", results[0], direct.Result)
			}

			requests, requestIDs := fixture.provider.snapshot()
			if len(requests) != 2 || len(requestIDs) != 2 || requestIDs[0] == "" || requestIDs[0] == requestIDs[1] {
				t.Fatalf("physical controlled model calls=%d request_ids=%v", len(requests), requestIDs)
			}
			if invocations[0].ProviderRequestID != requestIDs[0] || results[0].ProviderRequestID != requestIDs[0] {
				t.Fatalf("provider identity mismatch invocation=%q result=%q physical=%q", invocations[0].ProviderRequestID, results[0].ProviderRequestID, requestIDs[0])
			}
			adapterReceipt, found := formalAdapterReceiptFromPayload(requests[1], callID)
			if !found {
				t.Fatalf("second physical request omitted matching role=tool receipt: %#v", requests[1]["messages"])
			}
			if adapterReceipt.OperationID != request.OperationID || adapterReceipt.NativeToolCallID != callID || adapterReceipt.ExecutionCallID != callID {
				t.Fatalf("serialized adapter receipt identity mismatch: %#v", adapterReceipt)
			}
			if adapterReceipt.Result.Status != direct.Result.Status || !formalAdapterSameBusinessOutput(direct.Result.Output, adapterReceipt.Result.Output) {
				t.Fatalf("serialized adapter result drifted: direct=%#v adapter=%#v", direct.Result, adapterReceipt.Result)
			}
			if testCase.name != "memory.recall" && testCase.name != personaDetailCapabilityName && testCase.name != "relationship.lookup" && !adapterReceipt.Replayed {
				t.Fatalf("stable mutation operation was executed twice instead of replayed: %#v", adapterReceipt)
			}
			testCase.verify(t, fixture, adapterReceipt)
		})
	}
}

func formalToolAdapterCases() []formalToolAdapterCase {
	return []formalToolAdapterCase{
		{
			name: "active_memory_event", surface: CapabilitySurfaceNativeCognition, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				return f.request("active_memory_event", "adapter-active", map[string]any{
					"operation": "create", "kind": "commitment", "content": "完成正式 adapter 核验",
					"confidence": 0.95, "importance": 0.9, "original_time_expression": "", "time_precision": "unknown",
				})
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, _ ToolExecutionReceipt) {
				requireFormalAdapterSQLCount(t, f, `SELECT count(*) FROM public.active_memories WHERE owner_fluctlight_id=$1 AND source_fact_id LIKE 'owner-command-adapter-active-%'`, 1)
			},
		},
		{
			name: "affect_event", surface: CapabilitySurfaceNativeCognition, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				return f.request("affect_event", "adapter-affect", map[string]any{"event": map[string]any{"type": "happy", "confidence": 0.8}})
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				eventID := stringValue(mapValue(receipt.Result.Output)["event_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.fluctlight_inner_state_events WHERE id=$1 AND fluctlight_id=$2`, 1, eventID, f.fluctlightID)
			},
		},
		{
			name: "capability.request", surface: CapabilitySurfaceNativeCognition, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				return f.request("capability.request", "adapter-capability-request", map[string]any{
					"capability_key": "calendar.freebusy", "title": "读取空闲时间", "description": "读取授权日历空闲时间段",
					"rationale": "避免安排冲突", "priority": "normal", "desired_contract": map[string]any{"input": "date_range", "output": "busy_intervals"},
				})
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				id := stringValue(mapValue(receipt.Result.Output)["request_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.capability_requests WHERE id=$1 AND fluctlight_id=$2`, 1, id, f.fluctlightID)
			},
		},
		{
			name: "conversation.reply", surface: CapabilitySurfaceConversation, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				request := f.request("conversation.reply", "adapter-reply", map[string]any{"text": "正式 adapter 独立回复"})
				request.TargetKind, request.TargetRef = "conversation", f.conversationID
				return request
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				id := stringValue(mapValue(receipt.Result.Output)["target_ref"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.conversation_messages WHERE id=$1 AND conversation_id=$2 AND text='正式 adapter 独立回复'`, 1, id, f.conversationID)
			},
		},
		{
			name: "media.image.generate", surface: CapabilitySurfaceConversation, wantStatus: "accepted",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				request := f.request("media.image.generate", "adapter-image", map[string]any{"intent": "窗边阅读的纪实照片"})
				request.TargetKind, request.TargetRef = "conversation", f.conversationID
				return request
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				id := stringValue(mapValue(receipt.Result.Output)["media_intent_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.media_intents WHERE id=$1 AND owner_fluctlight_id=$2`, 1, id, f.fluctlightID)
			},
		},
		{
			name: "memory.recall", surface: CapabilitySurfaceConversation, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				return ToolExecutionRequest{CapabilityName: "memory.recall", AuthorizationActorID: f.ownerID, FluctlightID: f.fluctlightID, ConversationID: f.conversationID, Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{"intent": f.memorySecret})}
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				if !strings.Contains(jsonString(receipt.Result.Output), f.memorySecret) {
					t.Fatalf("memory recall did not return PostgreSQL fact: %#v", receipt.Result.Output)
				}
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.memories WHERE owner_fluctlight_id=$1 AND content=$2`, 1, f.fluctlightID, f.memorySecret)
			},
		},
		{
			name: "memory_event", surface: CapabilitySurfaceNativeCognition, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				return f.request("memory_event", "adapter-memory", map[string]any{"content": "正式 adapter 记忆", "type": "semantic", "confidence": 1.0, "importance": 0.8})
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				id := stringValue(mapValue(receipt.Result.Output)["memory_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.memories WHERE id=$1 AND content='正式 adapter 记忆'`, 1, id)
			},
		},
		{
			name: "moment.publish", surface: CapabilitySurfaceAutonomy, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				request := f.request("moment.publish", "adapter-moment", map[string]any{"text": "正式 adapter 动态"})
				request.ConversationID = ""
				return request
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				id := stringValue(mapValue(receipt.Result.Output)["target_ref"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.moments WHERE id=$1 AND owner_fluctlight_id=$2 AND text='正式 adapter 动态'`, 1, id, f.fluctlightID)
			},
		},
		{
			name: "persona.detail", surface: CapabilitySurfaceConversation, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				return f.request(personaDetailCapabilityName, "adapter-persona-detail", map[string]any{"operation": "read", "section_id": "identity"})
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				output := mapValue(receipt.Result.Output)
				if stringValue(output["section_id"]) != "identity" || stringValue(output["content"]) == "" {
					t.Fatalf("persona detail did not read canonical identity: %#v", output)
				}
				var sourceRevision int
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1`, f.fluctlightID).Scan(&sourceRevision); err != nil || intValue(output["source_revision"]) != sourceRevision {
					t.Fatalf("persona detail source version=%v expected=%d err=%v", output["source_revision"], sourceRevision, err)
				}
			},
		},
		{
			name: "persona.switch", surface: CapabilitySurfaceConversation, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				request := f.request("persona.switch", "adapter-persona-switch", map[string]any{"decision": "switch", "source_profile_id": "spark", "target_profile_id": "twilight", "trigger_id": "switch:safety", "reason": "adapter evidence"})
				request.ConversationID = ""
				return request
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, _ ToolExecutionReceipt) {
				var active string
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&active); err != nil || active != "twilight" {
					t.Fatalf("persona switch product active=%q err=%v", active, err)
				}
			},
		},
		{
			name: "persona.takeover", surface: CapabilitySurfaceConversation, wantStatus: "completed",
			request: func(t *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_personality_runtime SET active_profile_id='spark',previous_profile_id=NULL,revision=revision+1,cooldown_until=NULL WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
					t.Fatal(err)
				}
				request := f.request("persona.takeover", "adapter-persona-takeover", map[string]any{"decision": takeoverDecisionTakeoverB, "rule_id": "takeover-night", "source_profile_id": "spark", "target_profile_id": "twilight"})
				request.ConversationID = ""
				return request
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				auditID := stringValue(mapValue(receipt.Result.Output)["audit_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.platform_outbox_events WHERE aggregate_type='persona_action' AND aggregate_id=$1`, 1, auditID)
			},
		},
		{
			name: "schedule.replan", surface: CapabilitySurfaceNativeCognition, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				return f.request("schedule.replan", "adapter-schedule", map[string]any{"intent": "按受控计划完整重排今天剩余日程"})
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				id := stringValue(mapValue(receipt.Result.Output)["schedule_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.life_schedules WHERE id=$1 AND fluctlight_id=$2 AND status='accepted'`, 1, id, f.fluctlightID)
			},
		},
		{
			name: "presence_event", surface: CapabilitySurfaceNativeCognition, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				return f.request("presence_event", "adapter-presence", map[string]any{"operation": "set", "user_presence": "online", "current_task": "adapter 核验", "confidence": 0.9})
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				id := stringValue(mapValue(receipt.Result.Output)["overlay_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.life_presence_overlays WHERE id=$1 AND fluctlight_id=$2`, 1, id, f.fluctlightID)
			},
		},
		{
			name: "relationship.lookup", surface: CapabilitySurfaceConversation, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				request := f.request("relationship.lookup", "adapter-relationship", map[string]any{"target_actor_id": f.ownerID})
				request.ConversationID = ""
				return request
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				if stringValue(mapValue(receipt.Result.Output)["summary"]) != "adapter relationship authority" {
					t.Fatalf("relationship adapter result is not the PostgreSQL authority: %#v", receipt.Result.Output)
				}
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2`, 1, f.fluctlightID, f.ownerID)
			},
		},
		{
			name: "scene_event", surface: CapabilitySurfaceNativeCognition, wantStatus: "completed",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				return f.request("scene_event", "adapter-scene", map[string]any{"operation": "start", "scene": "书房", "activity": "核验", "location": "家", "confidence": 0.95})
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				id := stringValue(mapValue(receipt.Result.Output)["event_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.life_events WHERE id=$1 AND fluctlight_id=$2`, 1, id, f.fluctlightID)
			},
		},
		{
			name: "visual_identity.initialize", surface: CapabilitySurfaceVisualIdentity, wantStatus: "accepted",
			request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				request := f.request("visual_identity.initialize", "adapter-visual-initialize", map[string]any{})
				request.ConversationID = ""
				return request
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				f.visualSessionID = stringValue(mapValue(receipt.Result.Output)["session_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.fluctlight_visual_identity_sessions WHERE id=$1 AND fluctlight_id=$2`, 1, f.visualSessionID, f.fluctlightID)
			},
		},
		{
			name: "visual_identity.generate_candidate", surface: CapabilitySurfaceVisualIdentity, wantStatus: "accepted",
			request: func(t *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				requireFormalAdapterVisualSession(t, f)
				return f.visualRequest("visual_identity.generate_candidate", "adapter-visual-generate", map[string]any{"reason": "initial"})
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				f.visualCandidateIntent = stringValue(mapValue(receipt.Result.Output)["media_intent_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.media_intents WHERE id=$1 AND owner_fluctlight_id=$2`, 1, f.visualCandidateIntent, f.fluctlightID)
			},
		},
		{
			name: "visual_identity.commit_review", surface: CapabilitySurfaceVisualIdentity, wantStatus: "accepted",
			request: func(t *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				f.makeVisualCandidateReady(t)
				return f.visualRequest("visual_identity.commit_review", "adapter-visual-review", visualIdentityAcceptedReview())
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, receipt ToolExecutionReceipt) {
				f.visualCharacterIntent = stringValue(mapValue(receipt.Result.Output)["character_sheet_media_intent_id"])
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.media_intents WHERE id=$1 AND owner_fluctlight_id=$2`, 1, f.visualCharacterIntent, f.fluctlightID)
			},
		},
		{
			name: "visual_identity.finalize", surface: CapabilitySurfaceVisualIdentity, wantStatus: "completed",
			request: func(t *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
				f.makeVisualCharacterSheetReady(t)
				return f.visualRequest("visual_identity.finalize", "adapter-visual-finalize", map[string]any{})
			},
			verify: func(t *testing.T, f *formalToolAdapterFixture, _ ToolExecutionReceipt) {
				requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.fluctlight_visual_identity_sessions WHERE id=$1 AND status='completed'`, 1, f.visualSessionID)
			},
		},
	}
}

func newFormalToolAdapterFixture(t *testing.T) *formalToolAdapterFixture {
	t.Helper()
	base := newIndependentToolE2EFixture(t, "formal-adapter")
	fixture := &formalToolAdapterFixture{independentToolE2EFixture: base}
	fixture.memorySecret = "formal-adapter-memory-" + stableDigest(base.suffix)[:20]
	persona := map[string]any{
		"identity":     map[string]any{"name": "澄光", "gender": "female", "age": 24, "appearance": map[string]any{"hair": "black shoulder-length hair", "face_shape": "oval"}},
		"life_profile": map[string]any{"city": "上海", "timezone": "Asia/Shanghai", "appearance": map[string]any{"body_type": "slim", "chest_cup": "B"}},
		"personality_system": map[string]any{
			"mode": "multiple", "active_profile_id": "spark",
			"profiles":       []any{map[string]any{"id": "spark", "name": "Spark"}, map[string]any{"id": "twilight", "name": "Twilight"}},
			"switching":      map[string]any{"cooldown_seconds": 0, "rules": []any{map[string]any{"id": "safety"}}},
			"takeover_rules": []any{map[string]any{"id": "takeover-night", "kind": "turn_takeover", "version": "turn-takeover.v1", "source_profile_id": "spark", "target_profile_id": "twilight", "condition": "night"}},
		},
	}
	if _, err := base.repository.Pool().Exec(base.ctx, `UPDATE public.fluctlights SET core_persona=$2,identity=$3,life_profile=$4 WHERE id=$1`, base.fluctlightID, jsonBytes(persona), jsonBytes(mapValue(persona["identity"])), jsonBytes(mapValue(persona["life_profile"]))); err != nil {
		t.Fatal(err)
	}
	if _, err := base.repository.Pool().Exec(base.ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'spark',0) ON CONFLICT(fluctlight_id) DO UPDATE SET active_profile_id='spark',revision=0`, base.fluctlightID); err != nil {
		t.Fatal(err)
	}
	seedLegacyTestWorkingPersonas(t, base.app)
	if _, err := base.repository.Pool().Exec(base.ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json`, jsonString(map[string]any{"baseUrl": "http://controlled-comfy.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
		t.Fatal(err)
	}
	if _, err := base.repository.Pool().Exec(base.ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,profile_id,target_actor_id,role,metrics,interaction_frequency,trend,summary,emotional_association,provenance,revision) VALUES($1,$2,NULL,$3,'{"label":"owner"}','{"trust":0.82}',3,'stable','adapter relationship authority','{"tone":"warm"}','{"source":"adapter-e2e"}',4)`, "formal-adapter-relationship-"+base.suffix, base.fluctlightID, base.ownerID); err != nil {
		t.Fatal(err)
	}
	seed := base.request("memory_event", "adapter-memory-seed", map[string]any{"content": fixture.memorySecret, "type": "semantic", "confidence": 1.0, "importance": 1.0})
	if receipt, err := base.app.ExecuteTool(base.ctx, seed); err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("seed adapter memory receipt=%#v err=%v", receipt, err)
	}
	base.seedAcceptedSchedule(t)

	now := time.Now().UTC()
	location, _ := time.LoadLocation("Asia/Shanghai")
	local := now.In(location)
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	fixture.provider = &controlledFormalToolProvider{schedulePlan: map[string]any{
		"local_date": dayStart.Format("2006-01-02"), "timezone": "Asia/Shanghai", "expected_revision": 1,
		"completed_before": dayStart.Format(time.RFC3339), "reschedule_policy": map[string]any{},
		"items": []any{map[string]any{
			"start_at": dayStart.Format(time.RFC3339), "end_at": dayStart.AddDate(0, 0, 1).Format(time.RFC3339),
			"activity": "受控 adapter 核验", "scene": "书房", "location": "家", "item_type": "planned", "status": "planned",
			"priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5,
		}},
	}}
	fixture.server = httptest.NewServer(fixture.provider)
	t.Cleanup(fixture.server.Close)
	fixture.app.Provider.HTTP = fixture.server.Client()
	endpointID := "formal-adapter-endpoint-" + base.suffix
	if _, err := base.repository.Pool().Exec(base.ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible',$2,$3,'ready',now())`, endpointID, fixture.server.URL, "formal-adapter-secret-"+base.suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := base.repository.Pool().Exec(base.ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'controlled-formal-adapter-model','structured_output,tool_calling',4096,30,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (fixture *formalToolAdapterFixture) visualRequest(name, operation string, arguments map[string]any) ToolExecutionRequest {
	return ToolExecutionRequest{
		CapabilityName: name, AuthorizationActorID: fixture.ownerID, FluctlightID: fixture.fluctlightID,
		TargetKind: "visual_identity_session", TargetRef: fixture.visualSessionID,
		EvidenceID: "owner-command-" + operation + "-" + fixture.suffix, Surface: CapabilitySurfaceVisualIdentity,
		Arguments: jsonBytes(arguments),
	}
}

func (fixture *formalToolAdapterFixture) makeVisualCandidateReady(t *testing.T) {
	t.Helper()
	if fixture.visualCandidateIntent == "" {
		t.Fatal("visual candidate intent is missing")
	}
	fixture.visualCandidateAssetID = "asset_" + fixture.visualCandidateIntent
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.media_intents SET status='completed',revision=revision+1 WHERE id=$1`, fixture.visualCandidateIntent); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) SELECT $2,owner_fluctlight_id,'v1','image','image/png',68,$3,'adapter-bucket',$4,provider_request_id,workflow_id,'ready',now() FROM public.media_intents WHERE id=$1`, fixture.visualCandidateIntent, fixture.visualCandidateAssetID, stableDigest(fixture.visualCandidateAssetID), "adapter/"+fixture.visualCandidateAssetID+".png"); err != nil {
		t.Fatal(err)
	}
}

func (fixture *formalToolAdapterFixture) makeVisualCharacterSheetReady(t *testing.T) {
	t.Helper()
	if fixture.visualCharacterIntent == "" {
		t.Fatal("visual character-sheet intent is missing")
	}
	assetID := "asset_" + fixture.visualCharacterIntent
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.media_intents SET status='completed',revision=revision+1 WHERE id=$1`, fixture.visualCharacterIntent); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) SELECT $2,owner_fluctlight_id,'v1','image','image/png',68,$3,'adapter-bucket',$4,provider_request_id,workflow_id,'ready',now() FROM public.media_intents WHERE id=$1`, fixture.visualCharacterIntent, assetID, stableDigest(assetID), "adapter/"+assetID+".png"); err != nil {
		t.Fatal(err)
	}
}

func requireFormalAdapterVisualSession(t *testing.T, fixture *formalToolAdapterFixture) {
	t.Helper()
	if fixture.visualSessionID == "" {
		t.Fatal("visual initialize adapter row did not establish a session")
	}
}

type controlledFormalToolProvider struct {
	mu sync.Mutex

	toolName  string
	callID    string
	arguments json.RawMessage
	requests  []map[string]any
	requestID []string

	schedulePlan map[string]any
}

func (provider *controlledFormalToolProvider) begin(toolName, callID string, arguments json.RawMessage) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.toolName, provider.callID = toolName, callID
	provider.arguments = append(json.RawMessage(nil), arguments...)
	provider.requests = nil
	provider.requestID = nil
}

func (provider *controlledFormalToolProvider) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	provider.mu.Lock()
	toolName, callID := provider.toolName, provider.callID
	arguments := append(json.RawMessage(nil), provider.arguments...)
	if providerWireSchemaName(payload) != "schedule_replan_plan" {
		provider.requests = append(provider.requests, cloneMap(payload))
		provider.requestID = append(provider.requestID, request.Header.Get("X-Fluctlight-Provider-Request-Id"))
	}
	schedulePlan := cloneMap(provider.schedulePlan)
	provider.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if providerWireSchemaName(payload) == "schedule_replan_plan" {
		location, _ := time.LoadLocation("Asia/Shanghai")
		now := time.Now().In(location).Truncate(time.Second)
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
		dayEnd := dayStart.AddDate(0, 0, 1)
		schedulePlan["local_date"] = dayStart.Format("2006-01-02")
		schedulePlan["completed_before"] = now.Format(time.RFC3339)
		schedulePlan["items"] = []any{
			map[string]any{
				"start_at": dayStart.Format(time.RFC3339), "end_at": now.Format(time.RFC3339),
				"activity": "阅读", "scene": "书房", "item_type": "planned", "status": "planned",
				"priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5,
			},
			map[string]any{
				"start_at": now.Format(time.RFC3339), "end_at": dayEnd.Format(time.RFC3339),
				"activity": "阅读", "scene": "书房", "item_type": "planned", "status": "planned",
				"priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5,
			},
		}
		response := map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(schedulePlan), "tool_calls": []any{}}}}}
		_, _ = w.Write(jsonBytes(response))
		return
	}
	if !payloadHasToolResult(payload) {
		response := map[string]any{"choices": []any{map[string]any{
			"finish_reason": "tool_calls", "message": map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{
				"id": callID, "type": "function", "function": map[string]any{"name": toolName, "arguments": string(arguments)},
			}}},
		}}}
		_, _ = w.Write(jsonBytes(response))
		return
	}
	final := map[string]any{"answer": "adapter-ok:" + toolName}
	response := map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(final), "tool_calls": []any{}}}}}
	_, _ = w.Write(jsonBytes(response))
}

func (provider *controlledFormalToolProvider) snapshot() ([]map[string]any, []string) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	requests := make([]map[string]any, len(provider.requests))
	for index, request := range provider.requests {
		requests[index] = cloneMap(request)
	}
	return requests, append([]string(nil), provider.requestID...)
}

func formalAdapterOperationID(operationRoot, capabilityName string) string {
	return "agent_tool_" + stableDigest(fmt.Sprintf("%s\x1f%d\x1f%d\x1f%s", operationRoot, 1, 0, capabilityName))
}

func formalAdapterReceiptFromPayload(payload map[string]any, callID string) (ToolExecutionReceipt, bool) {
	for _, raw := range arrayValue(payload["messages"]) {
		message := mapValue(raw)
		if stringValue(message["role"]) != "tool" || stringValue(message["tool_call_id"]) != callID {
			continue
		}
		var receipt ToolExecutionReceipt
		if json.Unmarshal([]byte(stringValue(message["content"])), &receipt) == nil {
			return receipt, true
		}
	}
	return ToolExecutionReceipt{}, false
}

func formalAdapterSameBusinessOutput(direct, adapter any) bool {
	normalize := func(value any) any {
		encoded := jsonBytes(value)
		var decoded any
		_ = json.Unmarshal(encoded, &decoded)
		if object, ok := decoded.(map[string]any); ok {
			delete(object, "replayed")
		}
		return decoded
	}
	return reflect.DeepEqual(normalize(direct), normalize(adapter))
}

func requireFormalAdapterSQLCount(t *testing.T, fixture *formalToolAdapterFixture, query string, want int) {
	t.Helper()
	requireFormalAdapterSQLCountArgs(t, fixture, query, want, fixture.fluctlightID)
}

func requireFormalAdapterSQLCountArgs(t *testing.T, fixture *formalToolAdapterFixture, query string, want int, arguments ...any) {
	t.Helper()
	var got int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, query, arguments...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("independent durable product count=%d want=%d query=%s", got, want, query)
	}
}

func testEnvironment(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}
