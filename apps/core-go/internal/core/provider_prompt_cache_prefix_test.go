package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGoalEvaluationAssemblyKeepsStablePhysicalPrefix(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	binding, stable, current, err := goalEvaluationWireInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	memory := WorkingMemory{RuntimeFacts: []PromptFragment{
		{Kind: PromptFragmentRuntimeFact, Required: true, Content: map[string]any{"kind": "goals", "value": map[string]any{"summary": "长期目标事实"}}},
		{Kind: PromptFragmentRuntimeFact, Required: true, Content: map[string]any{"kind": "time_view", "value": map[string]any{"reference_timezone": "Asia/Shanghai", "current_time": "2026-10-08T10:00:00+08:00"}}},
		{Kind: PromptFragmentRuntimeFact, Required: true, Content: map[string]any{"kind": "current_state", "value": map[string]any{"mood": "calm"}}},
		{Kind: PromptFragmentRetrievedMemory, Required: true, Content: map[string]any{"content": "一条真实证据"}},
	}, Recent: []PromptFragment{
		{Kind: PromptFragmentRecentMessage, Required: true, Content: map[string]any{"role": "user", "content": "上一轮问题"}, GroupKey: "turn:1"},
		{Kind: PromptFragmentRecentMessage, Required: true, Content: map[string]any{"role": "assistant", "content": "上一轮回答"}, GroupKey: "turn:1"},
	}}
	base := PromptAssemblyInput{
		Role: "cognitive_assessment", OperationRules: []string{goalEvaluationInstruction},
		CorePersona: map[string]any{"identity": map[string]any{"name": "摇光"}}, StableTaskContext: stable,
		WorkingMemory: memory, CurrentInput: jsonString(current), ResponseFormat: providerResponseFormatForSchema("cognitive_assessment", "goal_evaluation_v1", goalEvaluationResponseSchema(binding)),
		Policy: DefaultPromptBudgetPolicy(4096),
	}
	first, err := AssemblePromptContext(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Messages) != 6 {
		t.Fatalf("physical messages=%#v", first.Messages)
	}
	wantRoles := []string{"system", "user", "user", "user", "assistant", "user"}
	for index, want := range wantRoles {
		if got := stringValue(first.Messages[index]["role"]); got != want {
			t.Fatalf("message %d role=%q want=%q", index, got, want)
		}
	}
	if strings.Count(jsonString(first.Messages), "[STABLE TASK CONTEXT]") != 1 || strings.Count(jsonString(first.Messages), "# 运行协议") != 1 {
		t.Fatalf("stable facts or leading system duplicated: %#v", first.Messages)
	}
	runtime := stringValue(first.Messages[2]["content"])
	if !(strings.Index(runtime, "goals:") < strings.Index(runtime, "time_view:") && strings.Index(runtime, "time_view:") < strings.Index(runtime, "current_state:") && strings.Index(runtime, "current_state:") < strings.Index(runtime, "retrieved_memory:")) {
		t.Fatalf("runtime facts are not slow-to-volatile: %s", runtime)
	}
	if first.Trace.SectionTokens["stable_task_context"] <= 0 || first.Trace.SectionBytes["stable_task_context"] <= 0 || first.Trace.EstimatedInputTokens <= 0 || first.Trace.WireBytes <= 0 {
		t.Fatalf("stable facts missing from physical budget diagnostics: %#v", first.Trace)
	}

	changed := richGoalEvaluationWireSnapshot()
	changed.Sources[0].Data["text"] = "波动的新证据"
	changed.Sources[0].OccurredAt = changed.Sources[0].OccurredAt.Add(60_000_000_000)
	_, stableAgain, currentAgain, err := goalEvaluationWireInput(changed)
	if err != nil {
		t.Fatal(err)
	}
	secondInput := base
	secondInput.StableTaskContext = stableAgain
	secondInput.CurrentInput = jsonString(currentAgain)
	secondInput.WorkingMemory = memory
	secondInput.WorkingMemory.RuntimeFacts = append([]PromptFragment(nil), memory.RuntimeFacts...)
	secondInput.WorkingMemory.RuntimeFacts[1].Content = map[string]any{"kind": "time_view", "value": map[string]any{"reference_timezone": "Asia/Shanghai", "current_time": "2026-10-08T10:01:00+08:00"}}
	second, err := AssemblePromptContext(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(first.Messages[:2]) != jsonString(second.Messages[:2]) {
		t.Fatalf("clock/evidence changed stable physical prefix\nfirst=%s\nsecond=%s", jsonString(first.Messages[:2]), jsonString(second.Messages[:2]))
	}
	secondRuntime := stringValue(second.Messages[2]["content"])
	timeIndex := strings.Index(runtime, "time_view:")
	if timeIndex < 0 || len(secondRuntime) < timeIndex || runtime[:timeIndex] != secondRuntime[:timeIndex] {
		t.Fatalf("unchanged slow runtime facts did not remain in the byte-identical prefix\nfirst=%s\nsecond=%s", runtime, secondRuntime)
	}
	if jsonString(first.Messages[2:]) == jsonString(second.Messages[2:]) {
		t.Fatal("volatile physical suffix did not change")
	}
	if stringValue(first.Messages[len(first.Messages)-1]["content"]) != jsonString(current) || strings.Contains(stringValue(first.Messages[len(first.Messages)-1]["content"]), "stable_definitions") {
		t.Fatalf("current input was not final and current-only: %#v", first.Messages[len(first.Messages)-1])
	}

	changed.Goals[0].Goal.DesiredOutcome = "经 Owner 修订的新目标"
	_, changedStable, changedCurrent, err := goalEvaluationWireInput(changed)
	if err != nil {
		t.Fatal(err)
	}
	thirdInput := secondInput
	thirdInput.StableTaskContext = changedStable
	thirdInput.CurrentInput = jsonString(changedCurrent)
	third, err := AssemblePromptContext(thirdInput)
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(first.Messages[:1]) != jsonString(third.Messages[:1]) || jsonString(first.Messages[:2]) == jsonString(third.Messages[:2]) {
		t.Fatalf("slow definition change invalidated the wrong prefix boundary: first=%#v third=%#v", first.Messages[:2], third.Messages[:2])
	}
	t.Logf("goal assembled wire bytes=%d estimated_tokens=%d stable_bytes=%d stable_estimated_tokens=%d", first.Trace.WireBytes, first.Trace.EstimatedInputTokens, first.Trace.SectionBytes["stable_task_context"], first.Trace.SectionTokens["stable_task_context"])
}

func TestStableTaskContextIsRequiredWithoutRuntimeAndCopiedFromContext(t *testing.T) {
	original := map[string]any{"definitions": map[string]any{"goal": "first"}}
	ctx := withProviderStableTaskContext(context.Background(), original)
	mapValue(original["definitions"])["goal"] = "mutated"
	captured := providerStableTaskContext(ctx)
	if got := stringValue(mapValue(captured["definitions"])["goal"]); got != "first" {
		t.Fatalf("stable context was not copied: %q", got)
	}
	captured["definitions"] = "changed again"
	if got := stringValue(mapValue(providerStableTaskContext(ctx)["definitions"])["goal"]); got != "first" {
		t.Fatalf("stable context read leaked mutation: %q", got)
	}

	result, err := AssemblePromptContext(PromptAssemblyInput{
		Role: "cognitive_assessment", StableTaskContext: providerStableTaskContext(ctx), CurrentInput: `{"current":true}`,
		Policy: DefaultPromptBudgetPolicy(4096),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 3 || !strings.Contains(stringValue(result.Messages[1]["content"]), `"goal":"first"`) || stringValue(result.Messages[2]["content"]) != `{"current":true}` {
		t.Fatalf("stable context was dropped or merged into current input: %#v", result.Messages)
	}

	policy := DefaultPromptBudgetPolicy(1000)
	policy.ContextWindowTokens = 10000
	policy.MaxInputTokens = 3000
	_, err = AssemblePromptContext(PromptAssemblyInput{
		Role: "cognitive_assessment", StableTaskContext: map[string]any{"definitions": strings.Repeat("定义", 10000)}, CurrentInput: "current", Policy: policy,
	})
	if !errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("oversize stable facts were pruned or admitted: %v", err)
	}
}
