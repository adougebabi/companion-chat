package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
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

func TestPhysicalEinoContinuationBudgetCountsToolResultAndMultimodalImage(t *testing.T) {
	model := &queuedToolCallingChatModel{assignment: providerAssignment{
		TokenBudget: 1000, ContextWindowTokens: 12000, MaxInputTokens: 5000, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1,
	}}
	input := []*schema.Message{
		{Role: schema.System, Content: "系统约束"},
		{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "call-1", Type: "function", Function: schema.FunctionCall{Name: "wardrobe.inspect", Arguments: `{}`}}}},
		{Role: schema.Tool, ToolCallID: "call-1", Content: strings.Repeat("真实衣柜查询结果", 1500)},
	}
	if err := model.enforcePhysicalInputBudget(context.Background(), input); !errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("oversize real Tool result was sent to Provider: %v", err)
	}
	parts, err := providerMessagesToEino([]map[string]any{{"role": "user", "content": []any{
		map[string]any{"type": "text", "text": "看这张图"},
		map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64," + strings.Repeat("a", 200000)}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	imageMessage := einoBudgetMessage(parts[0])
	if EstimatePromptTokens(imageMessage) < defaultPromptImageTokens {
		t.Fatalf("physical Eino budget omitted multimodal image: %#v", imageMessage)
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
	if result.Trace.EstimatedInputTokens <= 0 || result.Trace.WireBytes <= result.Trace.WireChars || result.Trace.TokenEstimateMethod == "" || result.Trace.SectionBytes["working_persona"] == 0 || result.Trace.SectionTokens["tools"] == 0 || len(result.Tools) != 1 || len(result.ResponseFormat) == 0 {
		t.Fatalf("assembly budget/result = %#v", result)
	}
}

func TestPromptAssemblerFailsWhenRequiredWireSectionsExceedCaps(t *testing.T) {
	policy := DefaultPromptBudgetPolicy(4096)
	policy.CurrentInputTokensCap = 8
	result, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", CurrentInput: strings.Repeat("x", 100), Policy: policy})
	if !errors.Is(err, ErrPromptRequiredBudgetExceeded) || len(result.Messages) != 0 {
		t.Fatalf("oversize required content was sent or silently cut: result=%#v err=%v", result, err)
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
	if len(result.Messages) != 2 || len(result.Trace.Dropped) != 2 || strings.Contains(jsonString(result.Messages), strings.Repeat("u", 100)) || strings.Contains(jsonString(result.Messages), strings.Repeat("a", 100)) {
		t.Fatalf("oversize recent turn was split or silently retained: messages=%#v trace=%#v", result.Messages, result.Trace)
	}
	for _, dropped := range result.Trace.Dropped {
		if dropped.Reason != "budget_excluded" {
			t.Fatalf("recent turn exclusion lacks a budget reason: %#v", result.Trace.Dropped)
		}
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

func TestPromptEstimatorHandlesMultimodalImages(t *testing.T) {
	fakeBase64Image := "data:image/png;base64," + strings.Repeat("a", 2000000)

	// String containing raw Base64 data URL
	rawStringTokens := EstimatePromptTokens(fakeBase64Image)
	if rawStringTokens < defaultPromptImageTokens || rawStringTokens > defaultPromptImageTokens+50 {
		t.Fatalf("EstimatePromptTokens(raw base64 string) = %d, expected ~%d", rawStringTokens, defaultPromptImageTokens)
	}

	// Multimodal image part map
	imagePart := map[string]any{
		"type": "image_url",
		"image_url": map[string]any{
			"url": fakeBase64Image,
		},
	}
	partTokens := EstimatePromptTokens(imagePart)
	if partTokens < defaultPromptImageTokens || partTokens > defaultPromptImageTokens+50 {
		t.Fatalf("EstimatePromptTokens(imagePart) = %d, expected ~%d", partTokens, defaultPromptImageTokens)
	}

	// Multimodal image part with low detail
	lowDetailPart := map[string]any{
		"type": "image_url",
		"image_url": map[string]any{
			"url":    fakeBase64Image,
			"detail": "low",
		},
	}
	lowTokens := EstimatePromptTokens(lowDetailPart)
	if lowTokens < defaultPromptLowDetailImage || lowTokens > defaultPromptLowDetailImage+50 {
		t.Fatalf("EstimatePromptTokens(lowDetailPart) = %d, expected ~%d", lowTokens, defaultPromptLowDetailImage)
	}

	// Two messages structure matching visual_identity_vision
	messages := []map[string]any{
		{
			"role":    "system",
			"content": "Inspect the candidate image.",
		},
		{
			"role": "user",
			"content": []any{
				map[string]any{"type": "text", "text": "Please analyze this image."},
				imagePart,
			},
		},
	}
	messagesTokens := EstimatePromptTokens(messages)
	// Should be around ~1,600 tokens, far below 98304 and definitely not 800,000+
	if messagesTokens < 1500 || messagesTokens > 2000 {
		t.Fatalf("EstimatePromptTokens(multimodal messages) = %d, expected between 1500 and 2000", messagesTokens)
	}

	wireEstimate := estimatePromptWireInput(messages, nil, nil)
	if wireEstimate > defaultMaxInputTokens {
		t.Fatalf("wireEstimate = %d exceeded max input tokens %d", wireEstimate, defaultMaxInputTokens)
	}
}

func TestOrdinaryStructuredAndStreamRejectOversizeInputBeforeHTTP(t *testing.T) {
	router := newFakeProviderRouter()
	provider := &ProviderClient{HTTP: &http.Client{Transport: router}}
	assignment := providerAssignment{TokenBudget: 1000, ContextWindowTokens: 12000, MaxInputTokens: 5000, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
	messages := []map[string]any{{"role": "system", "content": "约束"}, {"role": "user", "content": strings.Repeat("真实对话", 6000)}}
	_, err := provider.generateWithEino(context.Background(), EinoModelCall{
		Assignment: assignment, Role: "generic", Messages: messages, JSONMode: true,
		SchemaName: "bounded_result", ResponseSchema: objectSchema(map[string]any{"result": map[string]any{"type": "string"}}, []string{"result"}, false),
	})
	if !errors.Is(err, ErrPromptRequiredBudgetExceeded) || router.totalRequests() != 0 {
		t.Fatalf("structured over-budget request reached HTTP: err=%v requests=%d", err, router.totalRequests())
	}
	_, err = provider.streamWithEino(context.Background(), assignment, messages, "oversize-stream", func(string) error { return nil })
	if !errors.Is(err, ErrPromptRequiredBudgetExceeded) || router.totalRequests() != 0 {
		t.Fatalf("stream over-budget request reached HTTP: err=%v requests=%d", err, router.totalRequests())
	}
}
