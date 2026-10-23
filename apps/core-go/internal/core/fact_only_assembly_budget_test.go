package core

import (
	"strings"
	"testing"
)

func TestFactOnlyAssemblyBudgetsNativeWireButRetainsFinalContract(t *testing.T) {
	policy := DefaultPromptBudgetPolicy(4096)
	policy.ContextWindowTokens = 24576
	policy.MaxInputTokens = 16384
	facts := strings.Repeat("CURRENT_FACT_MUST_SURVIVE;", 340)
	input := PromptAssemblyInput{Surface: ProviderContextSurfaceFactOnlyCognition, Role: "cognitive_assessment", CorePersona: map[string]any{"identity": map[string]any{"name": "PERSONA_MUST_SURVIVE"}}, CurrentInput: "CURRENT_USER_MUST_SURVIVE", WorkingMemory: WorkingMemory{RuntimeFacts: []PromptFragment{{Kind: PromptFragmentRuntimeFact, Content: map[string]any{"facts": facts}, Required: true}}}, Tools: RenderCapabilityTools([]CapabilityDefinition{(factOnlyReplyCapability{}).Definition()}), Policy: policy}
	baseline, err := AssemblePromptContext(input)
	if err != nil {
		t.Fatal(err)
	}
	logical := providerResponseFormatForSchema("cognitive_assessment", "conversation_turn_response", cognitiveTurnResponseSchema())
	input.ResponseFormat = logical
	got, err := AssemblePromptContext(input)
	if err != nil {
		t.Fatal("native request rejected using a non-transmitted schema", err)
	}
	if got.Trace.EstimatedInputTokens != baseline.Trace.EstimatedInputTokens {
		t.Fatal(got.Trace, baseline.Trace)
	}
	if jsonString(got.ResponseFormat) != jsonString(logical) || jsonString(got.Tools) != jsonString(input.Tools) {
		t.Fatal("final contract or admitted Tools changed")
	}
	if !strings.Contains(jsonString(got.Messages), facts) || !strings.Contains(jsonString(got.Messages), "CURRENT_USER_MUST_SURVIVE") || !strings.Contains(jsonString(got.Messages), "PERSONA_MUST_SURVIVE") {
		t.Fatal("required context was pruned")
	}
	input.Tools = nil
	if _, err := AssemblePromptContext(input); err != nil {
		t.Fatal("tool-free final schema escaped physical budget", err)
	}
}
