package core

import (
	"context"
	"net/http"
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
