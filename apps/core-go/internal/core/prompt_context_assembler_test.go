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
	if err := model.enforcePhysicalInputBudget(context.Background(), input); !errors.Is(err, ErrPromptToolResultBudgetExceeded) || !errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("oversize real Tool result was sent to Provider or misclassified: %v", err)
	}
	if got := providerRunErrorCode(ErrPromptRequiredBudgetExceeded); got != "prompt_required_budget_exceeded" {
		t.Fatalf("physical budget classified as %q", got)
	}
	if got := wrapPhysicalProviderRequestError(ErrPromptRequiredBudgetExceeded); !errors.Is(got, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("physical budget error was hidden: %v", got)
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

func TestFormatRuntimeContextTimesUsesLifeTimezoneWithoutChangingSource(t *testing.T) {
	context := map[string]any{
		"current_state": map[string]any{"data": map[string]any{"life_context": map[string]any{
			"timezone": "Asia/Shanghai", "effective_at": "2026-09-29T09:13:44.667612Z",
		}}},
		"retrieved_memory": []any{map[string]any{
			"content":     "记忆中的 2026-09-29T09:13:44Z 保持原样",
			"created_at":  "2026-09-29T09:13:44.667612Z",
			"occurred_at": "2026-09-28T23:00:00+08:00",
		}},
	}
	formatted := formatRuntimeContextTimes(context)
	life := mapValue(mapValue(mapValue(formatted["current_state"])["data"])["life_context"])
	memory := mapValue(arrayValue(formatted["retrieved_memory"])[0])
	if life["effective_at"] != "2026-09-29T17:13:44.667+08:00" || memory["created_at"] != "2026-09-29T17:13:44.667+08:00" || memory["occurred_at"] != "2026-09-28T23:00:00.000+08:00" {
		t.Fatalf("formatted runtime times = %#v %#v", life, memory)
	}
	if memory["content"] != "记忆中的 2026-09-29T09:13:44Z 保持原样" || mapValue(arrayValue(context["retrieved_memory"])[0])["created_at"] != "2026-09-29T09:13:44.667612Z" {
		t.Fatalf("formatting changed content or source: %#v %#v", context, formatted)
	}
}

func TestAssemblePromptMessagesFormatsRuntimeTimesOnWire(t *testing.T) {
	selected := []promptOptionalCandidate{{fragment: PromptFragment{
		Kind: PromptFragmentRuntimeFact,
		Content: map[string]any{"kind": "current_state", "value": map[string]any{"data": map[string]any{"life_context": map[string]any{
			"timezone": "Asia/Shanghai", "effective_at": "2026-09-29T09:13:44.667612Z",
		}}}},
	}}}
	messages := assemblePromptMessages(map[string]any{"role": "system", "content": "test"}, map[string]any{"role": "user", "content": "hi"}, selected)
	wire := stringValue(messages[1]["content"])
	if !strings.Contains(wire, "2026-09-29T17:13:44.667+08:00") || strings.Contains(wire, "2026-09-29T09:13:44.667612Z") {
		t.Fatalf("runtime wire time was not formatted: %s", wire)
	}
}

func TestPromptAssemblerFailsWhenRequiredWireSectionsExceedCaps(t *testing.T) {
	policy := DefaultPromptBudgetPolicy(4096)
	policy.CurrentInputTokensCap = 8
	result, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", CurrentInput: strings.Repeat("x", 100), Policy: policy})
	if !errors.Is(err, ErrPromptRequiredBudgetExceeded) || !errors.Is(err, ErrPromptCurrentInputBudgetExceeded) || len(result.Messages) != 0 {
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

func TestPromptAssemblerRestoresRawWhenSummaryMissesTotalBudget(t *testing.T) {
	input := WorkingMemoryInput{
		Summaries: []PromptFragment{{Kind: PromptFragmentSummary, Priority: 40,
			Content:    map[string]any{"summary": strings.Repeat("历史", 500), "time_semantics": "historical_conversation"},
			SourceRefs: []string{"summary:one", "message:1", "message:2"}}},
		RecentMessages: []PromptFragment{
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "user", "content": "old question"}, SourceRefs: []string{"message:1"}, GroupKey: "turn:1"},
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "assistant", "content": "old answer"}, SourceRefs: []string{"message:2"}, GroupKey: "turn:1"},
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "user", "content": "new question"}, SourceRefs: []string{"message:3"}, GroupKey: "turn:2"},
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "assistant", "content": "new answer"}, SourceRefs: []string{"message:4"}, GroupKey: "turn:2"},
		},
	}
	memory, err := ResolveWorkingMemory(input, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if len(memory.Summaries) != 1 || len(memory.Recent) != 4 {
		t.Fatalf("working memory lost raw fallback before total budget: %#v", memory)
	}
	base, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", CurrentInput: "current", Policy: DefaultPromptBudgetPolicy(1000)})
	if err != nil {
		t.Fatal(err)
	}
	policy := DefaultPromptBudgetPolicy(1000)
	policy.MaxInputTokens = base.Trace.EstimatedInputTokens + 500
	bounded, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", WorkingMemory: memory, CurrentInput: "current", Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"old question", "old answer", "new question", "new answer"} {
		if !strings.Contains(jsonString(bounded.Messages), text) {
			t.Fatalf("raw fallback %q disappeared after Summary budget exclusion: %#v", text, bounded.Messages)
		}
	}
	if !strings.Contains(jsonString(bounded.Trace.Dropped), "budget_excluded") || strings.Contains(jsonString(bounded.Messages), strings.Repeat("历史", 50)) {
		t.Fatalf("oversized Summary was not excluded with budget reason: %#v", bounded.Trace)
	}
	roomy, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", WorkingMemory: memory, CurrentInput: "current", Policy: DefaultPromptBudgetPolicy(1000)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonString(roomy.Messages), strings.Repeat("历史", 50)) || strings.Contains(jsonString(roomy.Messages), "old question") || !strings.Contains(jsonString(roomy.Trace.Dropped), "summarized_source") {
		t.Fatalf("admitted Summary did not replace covered raw: messages=%#v trace=%#v", roomy.Messages, roomy.Trace)
	}
}

func TestPromptAssemblerOmitsDailyCoveredRawOnlyWhenMemorySelected(t *testing.T) {
	input := WorkingMemoryInput{
		RetrievedMemories: []PromptFragment{{Kind: PromptFragmentRetrievedMemory, Priority: 80,
			Content: map[string]any{"ref": "memory:ctx_daily", "type": "episodic", "content": "昨日对话经历"}, SourceRefs: []string{"memory:ctx_daily"}}},
		RecentMessages: []PromptFragment{
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "user", "content": "昨日问题"}, SourceRefs: []string{"message:1"}, GroupKey: "turn:1"},
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "assistant", "content": "昨日回答"}, SourceRefs: []string{"message:2"}, GroupKey: "turn:1"},
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "user", "content": "今天问题"}, SourceRefs: []string{"message:3"}, GroupKey: "turn:2"},
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "assistant", "content": "今天回答"}, SourceRefs: []string{"message:4"}, GroupKey: "turn:2"},
		},
	}
	memory, err := ResolveWorkingMemory(input, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	memory.Retrieved[0].SourceRefs = append(memory.Retrieved[0].SourceRefs, "message:1", "message:2")
	result, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", WorkingMemory: memory, CurrentInput: "现在", Policy: DefaultPromptBudgetPolicy(4096)})
	if err != nil {
		t.Fatal(err)
	}
	wire := jsonString(result.Messages)
	if !strings.Contains(wire, "昨日对话经历") || strings.Contains(wire, "昨日问题") || strings.Contains(wire, "昨日回答") || !strings.Contains(wire, "今天问题") {
		t.Fatalf("daily memory/raw coverage = %s", wire)
	}
}

func TestPromptAssemblerKeepsOlderRepeatedCurrentText(t *testing.T) {
	memory := WorkingMemory{Recent: []PromptFragment{
		{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "user", "content": "repeat"}, SourceRefs: []string{"message:old-user"}, GroupKey: "turn:old"},
		{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "assistant", "content": "earlier reply"}, SourceRefs: []string{"message:old-assistant"}, GroupKey: "turn:old"},
	}}
	result, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", WorkingMemory: memory, CurrentInput: "repeat", Policy: DefaultPromptBudgetPolicy(1000)})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(jsonString(result.Messages), "repeat") != 2 {
		t.Fatalf("older same-text user message was removed: %#v", result.Messages)
	}
}

func TestPromptAssemblerKeepsWholeTurnWhenSummaryCoversOnlyPart(t *testing.T) {
	memory := WorkingMemory{
		Summaries: []PromptFragment{{Kind: PromptFragmentSummary, Content: map[string]any{"summary": "old user summarized"}, SourceRefs: []string{"summary:partial", "message:1"}}},
		Recent: []PromptFragment{
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "user", "content": "old user"}, SourceRefs: []string{"message:1"}, GroupKey: "turn:1"},
			{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "assistant", "content": "still needed assistant"}, SourceRefs: []string{"message:2"}, GroupKey: "turn:1"},
		},
	}
	result, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", WorkingMemory: memory, CurrentInput: "current", Policy: DefaultPromptBudgetPolicy(1000)})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"old user summarized", "old user", "still needed assistant"} {
		if !strings.Contains(jsonString(result.Messages), text) {
			t.Fatalf("partially covered turn lost %q: %#v", text, result.Messages)
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
	compactPolicy := DefaultPromptBudgetPolicy(4096)
	compactPolicy.Version = promptBudgetPolicyVersionV2
	compactPolicy.ContextWindowTokens = 16384
	compactPolicy.MaxInputTokens = 11776
	compactPolicy.SafetyMarginTokens = 512
	compactPolicy.OptionalInputTargetTokens = 8000
	compact, err := AssemblePromptContext(PromptAssemblyInput{
		Role: "cognitive_assessment", OperationRules: []string{"bounded"}, CorePersona: map[string]any{"identity": map[string]any{"name": "摇光"}},
		WorkingMemory: memory, CurrentInput: "当前问题", Policy: compactPolicy,
	})
	if err != nil || compact.Trace.EstimatedInputTokens > 8000 || len(input.RecentMessages) != 1000 {
		t.Fatalf("16K long-history admission: tokens=%d source=%d err=%v", compact.Trace.EstimatedInputTokens, len(input.RecentMessages), err)
	}
}

func TestPromptBudgetConfigurationUsesOutputReserveAndSafetyMargin(t *testing.T) {
	if err := validatePromptBudgetConfiguration(65536, 49152, 4096, promptBudgetPolicyVersionV1); err != nil {
		t.Fatal(err)
	}
	if err := validatePromptBudgetConfiguration(55000, 49152, 4096, promptBudgetPolicyVersionV1); !errors.Is(err, ErrPromptOutputReserveConflict) {
		t.Fatalf("capacity formula accepted insufficient context window or lost category: %v", err)
	}
	if err := validatePromptBudgetConfiguration(65536, 49152, 4096, "unknown"); err == nil || err.Error() != "prompt_budget_policy_unknown" {
		t.Fatalf("unknown policy error = %v", err)
	}
	if err := validatePromptBudgetConfiguration(16384, 11776, 4096, promptBudgetPolicyVersionV2); err != nil {
		t.Fatalf("16K policy rejected its exact boundary: %v", err)
	}
	if err := validatePromptBudgetConfiguration(16384, 11777, 4096, promptBudgetPolicyVersionV2); err == nil {
		t.Fatal("16K policy accepted one token beyond the input boundary")
	}
	if category, code, retryable := classifyProviderPreflightError("assignment", promptBudgetConfigError{cause: ErrPromptOutputReserveConflict}); category != "budget" || code != "prompt_output_reserve_conflict" || retryable {
		t.Fatalf("output conflict diagnostic = %s/%s retryable=%t", category, code, retryable)
	}
	policy, err := promptBudgetPolicyForAssignment(providerAssignment{TokenBudget: 4096, ContextWindowTokens: 16384, MaxInputTokens: 11776, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV2})
	if err != nil || policy.SafetyMarginTokens != 512 || policy.OutputReserveTokens != 4096 {
		t.Fatalf("16K policy=%#v err=%v", policy, err)
	}
}

func TestConversationSoftInputTargetLeavesRoomWithoutDroppingRequiredInput(t *testing.T) {
	policy := DefaultPromptBudgetPolicy(4096)
	policy.Version = promptBudgetPolicyVersionV2
	policy.ContextWindowTokens = 16384
	policy.MaxInputTokens = 11776
	policy.SafetyMarginTokens = 512
	policy.OptionalInputTargetTokens = 8000
	largeMemory := WorkingMemory{RuntimeFacts: []PromptFragment{{Kind: PromptFragmentRuntimeFact,
		Content: map[string]any{"kind": "current_state", "value": map[string]any{"appearance": "当前短发", "activity": "在家阅读"}}, SourceRefs: []string{"runtime:current_state"},
	}}}
	for index := 0; index < 4; index++ {
		largeMemory.Summaries = append(largeMemory.Summaries, PromptFragment{Kind: PromptFragmentSummary,
			Content: map[string]any{"summary": strings.Repeat("较早的对话事实。", 250), "episode": index}, SourceRefs: []string{fmt.Sprintf("summary:%d", index)},
		})
	}
	tools := []map[string]any{{"type": "function", "function": map[string]any{"name": "conversation.reply", "description": "发送回复", "parameters": map[string]any{"type": "object"}}}}
	format := map[string]any{"type": "json_object"}
	result, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", WorkingMemory: largeMemory, CurrentInput: "现在的问题", Tools: tools, ResponseFormat: format, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	if result.Trace.EstimatedInputTokens > 8000 || len(result.Trace.Dropped) == 0 || result.Trace.OptionalInputTargetTokens != 8000 || !strings.Contains(jsonString(result.Messages), "当前短发") {
		t.Fatalf("optional target did not bound the first turn: %#v", result.Trace)
	}
	policy.OptionalInputTargetTokens = 0
	roomy, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", WorkingMemory: largeMemory, CurrentInput: "现在的问题", Tools: tools, ResponseFormat: format, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	if roomy.Trace.EstimatedInputTokens <= result.Trace.EstimatedInputTokens || len(roomy.Trace.Selected) == 0 {
		t.Fatalf("soft target did not distinguish optional admission: target=%#v roomy=%#v", result.Trace, roomy.Trace)
	}
	t.Logf("controlled full-request estimate without/with 8K soft target: %d / %d tokens; system=%d tools=%d schema=%d current=%d summary_before=%d summary_after=%d",
		roomy.Trace.EstimatedInputTokens, result.Trace.EstimatedInputTokens,
		result.Trace.SectionTokens["system"], result.Trace.SectionTokens["tools"], result.Trace.SectionTokens["response_schema"],
		result.Trace.SectionTokens["current_input"], roomy.Trace.SectionTokens["conversation_summary"], result.Trace.SectionTokens["conversation_summary"])
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
