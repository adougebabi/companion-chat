package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPromptEstimatorUsesConservativeUTF8Formula(t *testing.T) {
	for _, value := range []string{"hello", "明早七点赶飞机", strings.Repeat("界", 1000)} {
		units := (len([]byte(value)) + 2) / 3
		if runes := len([]rune(value)); runes > units {
			units = runes
		}
		minimum := (units*5 + 3) / 4
		if got := EstimatePromptTokens(value); got < minimum {
			t.Fatalf("EstimatePromptTokens(%q)=%d want >=%d", value, got, minimum)
		}
	}
}

func TestPromptAssemblerBuildsBLayoutAndCurrentInputExactlyOnce(t *testing.T) {
	current := "这是唯一的当前输入"
	memory, err := ResolveWorkingMemory(WorkingMemoryInput{
		RuntimeFacts:     []PromptFragment{{Kind: PromptFragmentRuntimeFact, Priority: 100, Content: map[string]any{"current_time": "2026-09-12T10:00:00+08:00", "scene": "书房"}, SourceRefs: []string{"life:current"}}},
		ActiveCandidates: []PromptFragment{{Kind: PromptFragmentActiveMemory, Priority: 100, Content: map[string]any{"ref": "active_memory:ctx_0123456789abcdef0123456789abcdef", "content": "明早七点赶飞机"}, SourceRefs: []string{"active:flight"}}},
		RecentMessages: []PromptFragment{
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "user", "content": "上一轮问题"}, SourceRefs: []string{"message:1"}, GroupKey: "turn:1"},
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "assistant", "content": "上一轮回答"}, SourceRefs: []string{"message:2"}, GroupKey: "turn:1"},
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "user", "content": current}, SourceRefs: []string{"message:3"}, GroupKey: "turn:2"},
		},
	}, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	tools := []map[string]any{{"type": "function", "function": map[string]any{"name": "conversation.reply", "parameters": map[string]any{"type": "object"}}}}
	responseFormat := providerResponseFormatForSchema("reflection", "test_schema", objectSchema(map[string]any{"result": stringSchema()}, []string{"result"}, false))
	result, err := AssemblePromptContext(PromptAssemblyInput{
		Role: "cognitive_assessment", OperationRules: []string{"只执行当前任务"},
		CorePersona:   map[string]any{"identity": map[string]any{"name": "摇光"}, "personality": map[string]any{"curiosity": 0.8}},
		WorkingMemory: memory, CurrentInput: current, Tools: tools, ResponseFormat: responseFormat,
		Policy: DefaultPromptBudgetPolicy(4096),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 5 || stringValue(result.Messages[0]["role"]) != "system" || stringValue(result.Messages[1]["role"]) != "user" || !strings.Contains(stringValue(result.Messages[1]["content"]), "[RUNTIME CONTEXT]") || stringValue(result.Messages[2]["role"]) != "user" || stringValue(result.Messages[3]["role"]) != "assistant" || stringValue(result.Messages[4]["role"]) != "user" {
		t.Fatalf("B-layout messages = %#v", result.Messages)
	}
	if strings.Count(jsonString(result.Messages), current) != 1 || stringValue(result.Messages[len(result.Messages)-1]["content"]) != current {
		t.Fatalf("current input duplication: %#v", result.Messages)
	}
	if result.Trace.EstimatedInputTokens > result.Trace.MaxInputTokens || len(result.Tools) != 1 || len(result.ResponseFormat) == 0 {
		t.Fatalf("assembly budget/result = %#v", result)
	}
}

func TestPromptAssemblerFailsWhenRequiredWireSectionsExceedCaps(t *testing.T) {
	policy := DefaultPromptBudgetPolicy(4096)
	policy.CurrentInputTokensCap = 8
	_, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", CurrentInput: strings.Repeat("x", 100), Policy: policy})
	if !errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("required overflow error = %v", err)
	}
}

func TestPromptAssemblerTotalCapNeverSplitsRecentTurn(t *testing.T) {
	memory, err := ResolveWorkingMemory(WorkingMemoryInput{RecentMessages: []PromptFragment{
		{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "user", "content": strings.Repeat("u", 5000)}, EstimatedTokens: 700, SourceRefs: []string{"message:1"}, GroupKey: "turn:1"},
		{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "assistant", "content": strings.Repeat("a", 5000)}, EstimatedTokens: 700, SourceRefs: []string{"message:2"}, GroupKey: "turn:1"},
	}}, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	policy := DefaultPromptBudgetPolicy(1000)
	policy.ContextWindowTokens = 10000
	policy.MaxInputTokens = 3000
	result, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", WorkingMemory: memory, CurrentInput: "current", Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 2 || len(result.Trace.Dropped) != 2 || result.Trace.Dropped[0].Reason != "total_cap" || result.Trace.Dropped[1].Reason != "total_cap" {
		t.Fatalf("recent turn was partially selected: messages=%#v trace=%#v", result.Messages, result.Trace)
	}
}

func TestPromptAssemblerPressureDoesNotScaleWithStores(t *testing.T) {
	input := WorkingMemoryInput{}
	for index := 0; index < 1000; index++ {
		role := "user"
		if index%2 == 1 {
			role = "assistant"
		}
		input.RecentMessages = append(input.RecentMessages, PromptFragment{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": role, "content": strings.Repeat(fmt.Sprintf("raw-%d ", index), 8)}, SourceRefs: []string{fmt.Sprintf("message:%d", index)}, GroupKey: fmt.Sprintf("turn:%d", index/2)})
	}
	for index := 0; index < 100; index++ {
		input.RetrievedMemories = append(input.RetrievedMemories, PromptFragment{Kind: PromptFragmentRetrievedMemory, Priority: 100 - index, Content: map[string]any{"ref": fmt.Sprintf("memory:ctx_%032x", index), "content": strings.Repeat("durable memory ", 20)}, SourceRefs: []string{fmt.Sprintf("memory:%d", index)}})
	}
	for index := 0; index < 30; index++ {
		input.ActiveCandidates = append(input.ActiveCandidates, PromptFragment{Kind: PromptFragmentActiveMemory, Priority: 100 - index, Content: map[string]any{"ref": fmt.Sprintf("active_memory:ctx_%032x", index), "content": strings.Repeat("active memory ", 16)}, SourceRefs: []string{fmt.Sprintf("active:%d", index)}})
	}
	memory, err := ResolveWorkingMemory(input, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	result, err := AssemblePromptContext(PromptAssemblyInput{
		Role: "cognitive_assessment", OperationRules: []string{"bounded"}, CorePersona: map[string]any{"identity": map[string]any{"name": "摇光"}},
		WorkingMemory: memory, CurrentInput: "当前问题", Tools: []map[string]any{}, ResponseFormat: map[string]any{}, Policy: DefaultPromptBudgetPolicy(4096),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Trace.EstimatedInputTokens > defaultMaxInputTokens || len(input.RecentMessages) != 1000 || len(input.RetrievedMemories) != 100 || len(input.ActiveCandidates) != 30 {
		t.Fatalf("pressure assembly/store mutation: tokens=%d recent=%d durable=%d active=%d", result.Trace.EstimatedInputTokens, len(input.RecentMessages), len(input.RetrievedMemories), len(input.ActiveCandidates))
	}
}

func TestPromptBudgetConfigurationUsesOutputReserveAndSafetyMargin(t *testing.T) {
	if err := validatePromptBudgetConfiguration(65536, 49152, 4096, promptBudgetPolicyVersionV1); err != nil {
		t.Fatal(err)
	}
	if err := validatePromptBudgetConfiguration(55000, 49152, 4096, promptBudgetPolicyVersionV1); err == nil {
		t.Fatal("capacity formula accepted insufficient context window")
	}
	if err := validatePromptBudgetConfiguration(65536, 49152, 4096, "unknown"); err == nil || err.Error() != "prompt_budget_policy_unknown" {
		t.Fatalf("unknown policy error = %v", err)
	}
}
