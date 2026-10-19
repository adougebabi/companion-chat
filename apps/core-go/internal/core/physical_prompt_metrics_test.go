package core

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestPhysicalPromptMetricsRetainLogicalTraceButUseActualNativeAndRepairWire(t *testing.T) {
	input := []*schema.Message{{Role: schema.System, Content: "权威协议"}, {Role: schema.User, Content: "当前问题"}}
	parent := WithPromptDiagnostics(context.Background(), map[string]any{"fluctlight_id": "scope-A", "prompt_budget": map[string]any{"estimated_input_tokens": 99999, "selection_marker": "keep"}})
	for _, repair := range []bool{false, true} {
		m := queuedToolCallingChatModel{assignment: providerAssignment{TokenBudget: 1024, ContextWindowTokens: 32768, MaxInputTokens: 24576, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}}
		if repair {
			m.responseFormat = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "final", "schema": map[string]any{"type": "object"}}}
		} else {
			m.definitions = []CapabilityDefinition{conversationReplyCapabilityDefinition()}
		}
		ctx := m.withPhysicalPromptBudgetDiagnostics(parent, input)
		d := providerPromptDiagnostics(ctx)
		budget := mapValue(d["prompt_budget"])
		messages := []map[string]any{einoBudgetMessage(input[0]), einoBudgetMessage(input[1])}
		expected := estimatePromptWireInput(messages, RenderCapabilityTools(m.definitions), m.responseFormat)
		if intValue(budget["estimated_input_tokens"]) != expected || budget["request_stage"] != "physical" || d["fluctlight_id"] != "scope-A" {
			t.Fatalf("physical metrics %#v", d)
		}
		logical := mapValue(d["logical_prompt_budget"])
		if logical["selection_marker"] != "keep" || intValue(logical["estimated_input_tokens"]) != 99999 {
			t.Fatal("logical trace lost")
		}
		counts := mapValue(budget["section_counts"])
		if repair {
			if intValue(counts["response_schema"]) != 1 || intValue(counts["tools"]) != 0 {
				t.Fatal("repair metrics use native tools")
			}
		} else if intValue(counts["response_schema"]) != 0 || intValue(counts["tools"]) != 1 {
			t.Fatal("native metrics count omitted schema")
		}
	}
	if intValue(mapValue(providerPromptDiagnostics(parent)["prompt_budget"])["estimated_input_tokens"]) != 99999 {
		t.Fatal("parent trace mutated")
	}
}

func TestFormalPromptDiagnosticScopeComesFromAuthorizedCapability(t *testing.T) {
	ctx, err := projectFormalPromptBoundary(context.Background(), ADKStructuredTaskInput{Role: "cognitive_assessment", SchemaName: "test", Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "system", "content": "规则"}, {"role": "user", "content": "问题"}}}, Capability: &ADKCapabilityRequest{FluctlightID: "authorized-role", ConversationID: "authorized-conversation"}})
	if err != nil {
		t.Fatal(err)
	}
	d := providerPromptDiagnostics(ctx)
	if d["fluctlight_id"] != "authorized-role" || d["conversation_id"] != "authorized-conversation" {
		t.Fatalf("scope missing %#v", d)
	}
}
