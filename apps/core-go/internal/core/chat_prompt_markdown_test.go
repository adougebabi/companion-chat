package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// Forbidden in chat prompts: former JSON-view metadata and the deprecated name.
var chatPromptForbidden = []string{
	"_semantics", "provenance_status", "lineage", "source_kind", "epistemic_kind",
	"candidate_not_event_proof", "past_message_verbatim", "摇光",
}

func chatPromptRuntime(t *testing.T, messages []map[string]any) string {
	t.Helper()
	for _, message := range messages {
		content := stringValue(message["content"])
		if strings.HasPrefix(content, "[RUNTIME CONTEXT]\n") {
			return strings.TrimSuffix(strings.TrimPrefix(content, "[RUNTIME CONTEXT]\n"), "\n[/RUNTIME CONTEXT]")
		}
	}
	t.Fatal("runtime context missing")
	return ""
}

func assertChatPromptShape(t *testing.T, name string, messages []map[string]any, systemLimit, runtimeLimit int) {
	t.Helper()
	system := stringValue(messages[0]["content"])
	runtime := chatPromptRuntime(t, messages)
	if !strings.HasPrefix(system, "# 运行协议") || !strings.Contains(system, "# 人格设定") {
		t.Errorf("%s: system lost protocol/persona parts:\n%s", name, system)
	}
	if _, persona, _ := strings.Cut(system, "# 人格设定"); strings.Contains(persona, `{"`) {
		t.Errorf("%s: persona is still JSON:\n%s", name, system)
	}
	if !strings.HasPrefix(runtime, "# 现在") || strings.HasPrefix(strings.TrimSpace(runtime), "{") {
		t.Errorf("%s: runtime context is not Markdown:\n%s", name, runtime)
	}
	for _, forbidden := range chatPromptForbidden {
		if strings.Contains(system, forbidden) || strings.Contains(runtime, forbidden) {
			t.Errorf("%s: forbidden prompt text %q", name, forbidden)
		}
	}
	// Fixed-fixture ceilings guard against prompt bloat creeping back. They
	// are test assertions only; no runtime token budget or truncation exists.
	if got := utf8.RuneCountInString(system); got > systemLimit {
		t.Errorf("%s: system grew to %d runes (limit %d)", name, got, systemLimit)
	}
	if got := utf8.RuneCountInString(runtime); got > runtimeLimit {
		t.Errorf("%s: runtime context grew to %d runes (limit %d)", name, got, runtimeLimit)
	}
}

func TestDirectChatPromptIsFourPartMarkdown(t *testing.T) {
	input := morningReplyFixture(t)
	projection := privateTextChatProjection(input.Projection)
	projection.Wardrobe = []map[string]any{
		{"category": "上衣", "description": "米白针织衫", "availability": "available"},
		{"category": "鞋", "description": "黑色短靴", "availability": "unavailable"},
	}
	working := workingMemoryInputForSurface(projection, ProviderContextSurfaceConversationMain, nil, nil, true)
	wm, err := ResolveWorkingMemory(working, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	result, err := AssemblePromptContext(PromptAssemblyInput{Surface: ProviderContextSurfaceConversationMain, Role: "cognitive_assessment",
		OperationRules: []string{providerContextAuthorityRule, directMainConversationProtocol, factOnlyExecutionBoundary},
		CorePersona:    input.WorkingPersona, CurrentInput: input.CurrentInput, WorkingMemory: wm,
		ResponseFormat: providerResponseFormatForSchema("cognitive_assessment", "conversation_turn_response", cognitiveTurnResponseSchema()), Policy: DefaultPromptBudgetPolicy(1024)})
	if err != nil {
		t.Fatal(err)
	}
	assertChatPromptShape(t, "direct", result.Messages, 1300, 1400)
	system := stringValue(result.Messages[0]["content"])
	if strings.Contains(system, directMainConversationProtocol) || strings.Contains(system, providerContextAuthorityRule) {
		t.Error("direct system still carries the replaced jargon rules")
	}
	runtime := chatPromptRuntime(t, result.Messages)
	for _, required := range []string{"## 衣柜", "米白针织衫", "黑色短靴（暂时不能穿）", "# 关于对方", "（推测）", "书桌"} {
		if !strings.Contains(runtime, required) {
			t.Errorf("direct runtime missing %q:\n%s", required, runtime)
		}
	}
	if last := result.Messages[len(result.Messages)-1]; stringValue(last["content"]) != input.CurrentInput {
		t.Error("current input is not the final unsummarized message")
	}
}

func TestFactOnlyChatPromptsAreFourPartMarkdown(t *testing.T) {
	input := morningReplyFixture(t)
	writer, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil {
		t.Fatal(err)
	}
	assertChatPromptShape(t, "writer", writer.Messages, 1400, 2000)
	working := factOnlyCognitionMemoryInput(input.Projection, nil, nil)
	wm, err := ResolveWorkingMemory(working, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	cognition, err := AssemblePromptContext(PromptAssemblyInput{Surface: ProviderContextSurfaceFactOnlyCognition, Role: "cognitive_assessment", OperationRules: []string{factOnlyExecutionBoundary}, CorePersona: input.WorkingPersona, CurrentInput: input.CurrentInput, WorkingMemory: wm, Policy: DefaultPromptBudgetPolicy(1024)})
	if err != nil {
		t.Fatal(err)
	}
	assertChatPromptShape(t, "fact-only cognition", cognition.Messages, 1500, 1600)
}

func TestChatMarkdownKeepsUnknownEmptyAndOpaqueValues(t *testing.T) {
	rendered := renderChatRuntimeMarkdown(map[string]any{
		"goals":            []any{map[string]any{"description": "目标", "success_criteria": []any{}, "deadline": nil}},
		"actor_background": []any{map[string]any{"attribute": "preferences", "value": map[string]any{"large": 9007199254740993, "kind": "fruit"}}},
	})
	for _, required := range []string{"完成条件：无", "deadline：未知", `{"kind":"fruit","large":9007199254740993}`} {
		if !strings.Contains(rendered, required) {
			t.Errorf("rendered context lost %q:\n%s", required, rendered)
		}
	}
}

// Regression: a self-reported "我在国外" never reached 关于对方 because the chat
// tool hinted assertion_type=inference (kept uncertain, filtered out), and an
// agreed schedule change was only promised because the schedule tools were not
// visible without a catalog lookup.
func TestChatSelfReportedFactsAndScheduleChangesAreActionable(t *testing.T) {
	native := actorFactCapability{}.NativeDefinition()
	assertion := mapValue(mapValue(native.InputSchema["properties"])["assertion_type"])
	if _, hinted := assertion["default"]; hinted {
		t.Fatalf("assertion_type must not hint a default: %#v", assertion)
	}
	for _, want := range []string{"explicit_statement", "inference", "关于对方", "location"} {
		if !strings.Contains(native.Description, want) {
			t.Fatalf("native description lacks %q: %s", want, native.Description)
		}
	}
	mandatory := formalAgentMandatoryTools(FormalAgentConversationCognition)
	for _, name := range []string{scheduleInspectCapabilityName, scheduleEditCapabilityName, actorFactCapabilityName} {
		if !mandatory[name] {
			t.Fatalf("%s is not visible from the first round", name)
		}
	}
	for _, want := range []string{"explicit_statement", "schedule.inspect", "schedule.edit", "schedule.replan"} {
		if !strings.Contains(directChatProtocol, want) {
			t.Fatalf("direct chat rules lack %q", want)
		}
	}
}

// Chat tools come in tiers: a small first-round set, a one-line index for the
// rest (loaded by name), and no pre-turn "does this need the image tool" call.
// The first-round set may not grow past what it cost before this layout.
func TestChatToolTiersStayWithinTheFirstRoundBudget(t *testing.T) {
	registry, err := NewCapabilityRegistry(builtinCapabilities(&App{})...)
	if err != nil {
		t.Fatal(err)
	}
	mandatory := formalAgentMandatoryTools(FormalAgentConversationCognition)
	for _, name := range []string{"memory_event", "media.image.generate", conversationReplyCapabilityName, capabilityCatalogName} {
		if !mandatory[name] {
			t.Fatalf("%s must be offered from the first chat round", name)
		}
	}
	if mandatory["schedule.replan"] {
		t.Fatal("schedule.replan belongs to the loadable index, not the first round")
	}
	invoker := &appADKCapabilityInvoker{request: ADKCapabilityRequest{Surface: CapabilitySurfaceConversation}, scopeRegistry: registry}
	catalog := registry.Catalog(CapabilitySurfaceConversation)
	shown := []CapabilityDefinition{}
	for _, definition := range catalog {
		if mandatory[definition.Name] {
			shown = append(shown, definition)
		}
	}
	bytes := 0
	for _, definition := range invoker.NativeDefinitions(shown) {
		raw, _ := json.Marshal(map[string]any{"name": definition.Name, "description": definition.Description, "parameters": definition.InputSchema})
		bytes += len(raw)
		if definition.Name == "media.image.generate" {
			if len(raw) > 1500 || !strings.Contains(jsonString(definition.InputSchema), `"intent"`) || strings.Contains(jsonString(definition.InputSchema), "media_plan") {
				t.Fatalf("chat image tool is not the slim chat definition (%d bytes)", len(raw))
			}
		}
	}
	t.Logf("first-round chat tools: %d, %d bytes", len(shown), bytes)
	if len(shown) != len(mandatory) || bytes > 6782 {
		t.Fatalf("first-round chat tools: %d of %d installed, %d bytes (budget 6782, the first round before tiers)", len(shown), len(mandatory), bytes)
	}
	index := directCapabilityIndex(context.Background(), catalog)
	t.Logf("index: %d bytes\n%s", len(index), index)
	if !strings.Contains(index, "- schedule.replan：") || !strings.Contains(index, "capability_catalog") {
		t.Fatalf("index lacks the loadable tools: %s", index)
	}
	for name := range mandatory {
		if strings.Contains(index, "- "+name+"：") {
			t.Fatalf("index repeats first-round tool %s", name)
		}
	}
	if strings.Contains(index, "- capability.discover：") || len(index) > 4000 {
		t.Fatalf("index is not a compact directory (%d bytes)", len(index))
	}
	full, _ := registry.Definition("media.image.generate")
	if other := (&appADKCapabilityInvoker{request: ADKCapabilityRequest{Surface: CapabilitySurfaceWakeUp}, scopeRegistry: registry}).NativeDefinitions([]CapabilityDefinition{full}); !strings.Contains(jsonString(other[0].InputSchema), "media_plan") {
		t.Fatal("non-chat surfaces must keep the full image definition")
	}
}
