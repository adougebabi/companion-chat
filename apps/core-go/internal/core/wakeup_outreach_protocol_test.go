package core

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWakeUpPhysicalRequestSeparatesNewReasonFromUnansweredTopic(t *testing.T) {
	for _, trigger := range []string{"no_new_reason", "new_confirmed_event"} {
		t.Run(trigger, func(t *testing.T) {
			var wire map[string]any
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				wire = cloneMap(payload)
				// Scripted silence checks transport only; it is not evidence of a
				// real model's autonomous decision in either scenario.
				return fakeProviderResult{Structured: map[string]any{"action_type": "no_op", "response_intent": "fixture", "evidence_refs": []any{}, "influences": []any{}}}
			})
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
			system := renderProviderSystemForSurface(ProviderContextSurfaceWakeUp, []string{capabilityWakeUpPolicyInstruction}, nil, nil, "cognitive_assessment")
			input := jsonString(map[string]any{"trigger": trigger, "has_new_inbound_message": false})
			assignment := providerAssignment{Role: "generic_llm", BaseURL: "http://controlled.invalid", ModelID: "controlled-wakeup", Timeout: time.Second}
			ctx := withFrozenProviderAssignment(context.Background(), "cognitive_assessment", assignment)
			_, err := app.RunFormalAgent(ctx, FormalAgentWakeUp, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "system", "content": system}, {"role": "user", "content": input}}, ResponseFormat: wakeUpResponseSchema()}, SchemaName: "wake_up_response"})
			if err != nil {
				t.Fatal(err)
			}
			messages := jsonString(wire["messages"])
			for _, required := range []string{"没有可执行目标或其他独立理由时选择 no_op", "未回应且无新进展的同一话题不追问", "真实的新事件或不同话题可成为自主分享机会", "不永久禁止新话题", "安静时段", "主题冷却", trigger} {
				if !strings.Contains(messages, required) {
					t.Fatalf("physical WakeUp request missing %q: %s", required, messages)
				}
			}
			if router.totalRequests() != 1 || wire["max_tokens"] != nil || wire["max_completion_tokens"] != nil {
				t.Fatalf("invalid controlled WakeUp request: %#v", wire)
			}
		})
	}
}

func TestWakeUpPhysicalRequestPreservesGoalCriteria(t *testing.T) {
	memoryRef := "memory:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	criteria := []any{"原始条件\n第二行", map[string]any{"id": "business-id", "source_ref": memoryRef, "valid": false, "amount": 0.0, "ratio": 0.123456, "changed_at": "2026-10-10T01:02:03Z", "schema": map[string]any{"properties": map[string]any{"customer_id": map[string]any{"type": "string"}}}}}
	projection := ContextProjection{
		ReferenceTimezone: "Asia/Shanghai", AsOf: "2026-10-10T01:00:00Z",
		ReferenceIndex: ContextReferenceIndex{ByRef: map[string]ContextReference{memoryRef: {Ref: memoryRef, Kind: ContextReferenceMemory}}},
		Goals: []map[string]any{{"id": "private-goal", "description": "完成目标", "status": "active", "success_criteria": criteria,
			"execution": map[string]any{"stage": "waiting", "last_evaluation": map[string]any{"impact": "old assessment"}}}},
	}
	before := jsonString(projection)
	input := workingMemoryInputFromProjectionForSurface(projection, ProviderContextSurfaceWakeUp, nil, nil)
	memory, err := ResolveWorkingMemory(input, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	assembly, err := AssemblePromptContext(PromptAssemblyInput{
		Surface: ProviderContextSurfaceWakeUp, Role: "cognitive_assessment", OperationRules: []string{capabilityWakeUpPolicyInstruction},
		WorkingMemory: memory, CurrentInput: `{"trigger":"new_event"}`, ResponseFormat: wakeUpResponseSchema(), Policy: DefaultPromptBudgetPolicy(4096),
	})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		wire = payload
		return fakeProviderResult{Structured: map[string]any{"action_type": "no_op", "response_intent": "fixture", "evidence_refs": []any{}, "influences": []any{}}}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	assignment := providerAssignment{Role: "generic_llm", BaseURL: "http://controlled.invalid", ModelID: "controlled-wakeup", Timeout: time.Second}
	ctx := withFrozenProviderAssignment(context.Background(), "cognitive_assessment", assignment)
	if _, err := app.RunFormalAgent(ctx, FormalAgentWakeUp, FormalAgentRunInput{Prompt: assembly, SchemaName: "wake_up_response"}); err != nil {
		t.Fatal(err)
	}
	var runtime map[string]any
	for _, raw := range arrayValue(wire["messages"]) {
		content := stringValue(mapValue(raw)["content"])
		if strings.HasPrefix(content, "[RUNTIME CONTEXT]\n") {
			text := strings.TrimSuffix(strings.TrimPrefix(content, "[RUNTIME CONTEXT]\n"), "\n[/RUNTIME CONTEXT]")
			if err := json.Unmarshal([]byte(text), &runtime); err != nil {
				t.Fatal(err)
			}
		}
	}
	goals := arrayValue(runtime["goals"])
	if len(goals) != 1 || !reflect.DeepEqual(mapValue(goals[0])["success_criteria"], criteria) {
		t.Fatalf("physical WakeUp request changed Goal criteria: %#v", goals)
	}
	if strings.Contains(jsonString(goals), "private-goal") || strings.Contains(jsonString(goals), "old assessment") || jsonString(projection) != before {
		t.Fatal("physical WakeUp request leaked Core data or mutated source")
	}
}
