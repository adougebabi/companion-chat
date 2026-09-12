package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type testCapability struct{}

func testCapabilityDefinitions() []CapabilityDefinition {
	return []CapabilityDefinition{
		conversationReplyCapabilityDefinition(),
		momentPublishCapabilityDefinition(),
		imageCapabilityDefinition(),
		affectEventCapabilityDefinition(),
	}
}

func definitionMapForName(definitions map[string]CapabilityDefinition, name string) CapabilityDefinition {
	return definitions[name]
}

func testInvocations(calls []ToolCallV1) []CapabilityInvocation {
	result := make([]CapabilityInvocation, 0, len(calls))
	for _, call := range calls {
		invocation := CapabilityInvocation{CallID: call.ID, CapabilityName: call.Name, Arguments: append(json.RawMessage(nil), call.Arguments...), SourceFactID: call.SourceFactID, ProviderRequestID: call.ProviderRequestID, ActionID: call.ActionID, Sequence: call.Sequence, SchemaVersion: CapabilityInvocationSchemaVersion}
		if invocation.SourceFactID == "" {
			invocation.SourceFactID = "fact-1"
		}
		if invocation.ProviderRequestID == "" {
			invocation.ProviderRequestID = "provider-1"
		}
		result = append(result, invocation)
	}
	return result
}

func (testCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{Name: "search.lookup", Version: "v1", Type: CapabilityTypeQuery, Description: "Look up a bounded fact.", SideEffectClass: "read_only", ConcurrencyClass: "parallel", FailurePolicy: FailurePolicyOptionalInternal, InputSchema: map[string]any{"type": "object"}}
}
func (testCapability) RequiredContext() []ContextSlot { return nil }

func (testCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"value": "ok"}, ProviderRequestID: invocation.ProviderRequestID}, nil
}

func TestNormalizeProviderToolCallsAcceptsNativeAndSidecarShapes(t *testing.T) {
	calls, err := NormalizeProviderToolCalls([]any{
		map[string]any{
			"id": "call_native",
			"function": map[string]any{
				"name":      "media.image.generate",
				"arguments": `{"concept":{"subject":"a cat"}}`,
			},
		},
		map[string]any{
			"id":        "call_sidecar",
			"name":      "media.image.generate",
			"arguments": map[string]any{"concept": map[string]any{"subject": "a dog"}},
		},
	}, "fact-1", "provider-1")
	if err != nil {
		t.Fatalf("NormalizeProviderToolCalls() error = %v", err)
	}
	if len(calls) != 2 || calls[0].Sequence != 0 || calls[1].Sequence != 1 {
		t.Fatalf("normalized calls = %#v", calls)
	}
	if calls[0].SchemaVersion != CapabilityInvocationSchemaVersion || calls[0].SourceFactID != "fact-1" {
		t.Fatalf("normalized metadata = %#v", calls[0])
	}
	var firstArgs map[string]any
	if err := json.Unmarshal(calls[0].Arguments, &firstArgs); err != nil {
		t.Fatalf("first arguments are not JSON = %v", err)
	}
	if firstArgs["concept"] == nil {
		t.Fatalf("first arguments = %#v", firstArgs)
	}
}

func TestNormalizeProviderToolCallsWrapsSingleObject(t *testing.T) {
	calls, err := NormalizeProviderToolCalls(map[string]any{
		"id":   "call_single",
		"name": "media.image.generate",
		"arguments": map[string]any{
			"concept": map[string]any{"subject": "a cat"},
		},
	}, "fact-1", "provider-1")
	if err != nil {
		t.Fatalf("single object normalization error = %v", err)
	}
	if len(calls) != 1 || calls[0].CallID != "call_single" || calls[0].Sequence != 0 {
		t.Fatalf("normalized single call = %#v", calls)
	}
}

func TestNormalizeProviderToolCallsRejectsMalformedOrDuplicateCalls(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{name: "missing id", value: []any{map[string]any{"name": "media.image.generate", "arguments": `{}`}}},
		{name: "invalid name", value: []any{map[string]any{"id": "call", "name": "media image", "arguments": `{}`}}},
		{name: "non object arguments", value: []any{map[string]any{"id": "call", "name": "media.image.generate", "arguments": `[]`}}},
		{name: "duplicate id", value: []any{
			map[string]any{"id": "call", "name": "media.image.generate", "arguments": `{}`},
			map[string]any{"id": "call", "name": "media.image.generate", "arguments": `{}`},
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := NormalizeProviderToolCalls(testCase.value, "fact", "provider"); err == nil {
				t.Fatal("expected normalization error")
			}
		})
	}

	tooLarge := strings.Repeat("x", maxToolArgumentsBytes)
	if _, err := NormalizeProviderToolCalls([]any{map[string]any{"id": "call", "name": "media.image.generate", "arguments": `{"value":"` + tooLarge + `"}`}}, "fact", "provider"); err == nil {
		t.Fatal("expected oversized arguments error")
	}
}

func TestToolCallValidateRequiresRegisteredCapability(t *testing.T) {
	definitions := capabilityDefinitionMap(testCapabilityDefinitions())
	calls, err := NormalizeProviderToolCalls([]any{map[string]any{
		"id":   "call",
		"name": "media.image.generate",
		"arguments": map[string]any{
			"intent": "a cat",
		},
	}}, "fact", "provider")
	if err != nil {
		t.Fatalf("normalize = %v", err)
	}
	if err := calls[0].Validate(definitionMapForName(definitions, "media.image.generate")); err != nil {
		t.Fatalf("registered capability rejected = %v", err)
	}
	calls[0].CapabilityName = "media.video.generate"
	if err := calls[0].Validate(definitionMapForName(definitions, "media.image.generate")); err == nil {
		t.Fatal("expected unavailable capability error")
	}
}

func TestCapabilityRendererKeepsProviderSchemaAtBoundary(t *testing.T) {
	payload := RenderCapabilityTools(testCapabilityDefinitions())
	if len(payload) != 4 {
		t.Fatalf("tool payload = %#v", payload)
	}
	names := make(map[string]struct{}, len(payload))
	for _, item := range payload {
		function, ok := item["function"].(map[string]any)
		if !ok {
			t.Fatalf("function payload = %#v", item)
		}
		parameters, ok := function["parameters"].(map[string]any)
		if !ok || parameters["type"] != "object" {
			t.Fatalf("parameters payload = %#v", function)
		}
		names[stringValue(function["name"])] = struct{}{}
	}
	if _, ok := names["conversation.reply"]; !ok {
		t.Fatalf("conversation.reply manifest missing = %#v", names)
	}
	if _, ok := names["moment.publish"]; !ok {
		t.Fatalf("moment.publish manifest missing = %#v", names)
	}
	if _, ok := names["affect_event"]; !ok {
		t.Fatalf("affect_event manifest missing = %#v", names)
	}
	if _, ok := names["media.image.generate"]; !ok {
		t.Fatalf("media manifest missing = %#v", names)
	}
	definition := imageCapabilityDefinition()
	if len(definition.TargetKinds) != 3 || definition.TargetKinds[0] != "conversation_message" || definition.TargetKinds[1] != "moment" || definition.TargetKinds[2] != "wake_up" {
		t.Fatalf("media target kinds = %#v", definition.TargetKinds)
	}
	if !definition.IsDeferredOutput() || definition.OutputSchema == nil {
		t.Fatalf("media definition must be a typed deferred output slot: %#v", definition)
	}
	if err := definition.ValidateOutput(map[string]any{"media_intent_id": "intent"}); err == nil {
		t.Fatal("missing typed output target fields must be rejected")
	}
	replyDefinition := conversationReplyCapabilityDefinition()
	if !replyDefinition.IsDeferredOutput() || replyDefinition.TargetKinds[0] != "conversation_message" {
		t.Fatalf("conversation reply must be a deferred conversation output: %#v", replyDefinition)
	}
	if len(replyDefinition.RequiredContext) != 1 || replyDefinition.RequiredContext[0] != SlotCurrentLife {
		t.Fatalf("conversation reply must freeze the current Life Context: %#v", replyDefinition.RequiredContext)
	}
}

func TestVisualIdentityInitializationManifestIsWakeUpCapability(t *testing.T) {
	definition := visualIdentityInitializeCapabilityDefinition()
	if definition.Name != "visual_identity.initialize" || definition.IsDeferredOutput() || definition.SideEffectClass != "native_projection" {
		t.Fatalf("visual identity definition = %#v", definition)
	}
	if err := definition.ValidateOutput(map[string]any{"session_id": "session-1", "status": "queued"}); err != nil {
		t.Fatalf("definition output rejected: %v", err)
	}
}

func TestCapabilityRegistryIsAnExtensibleSlot(t *testing.T) {
	registry, err := NewCapabilityRegistry(testCapability{})
	if err != nil {
		t.Fatal(err)
	}
	definitions := registry.Definitions()
	if len(definitions) != 1 || definitions[0].Name != "search.lookup" {
		t.Fatalf("definitions = %#v", definitions)
	}
	calls, err := NormalizeProviderToolCalls([]any{map[string]any{"id": "call", "name": "search.lookup", "arguments": `{ "query": "fluctlight" }`}}, "fact", "provider")
	if err != nil {
		t.Fatalf("normalize = %v", err)
	}
	if err := calls[0].Validate(testCapability{}.Definition()); err != nil {
		t.Fatalf("call validation = %v", err)
	}
	result, ok := registry.Lookup("search.lookup")
	if !ok || result == nil {
		t.Fatalf("lookup = %#v, ok=%v", result, ok)
	}
	output, err := result.Execute(context.Background(), calls[0], CapabilityContext{})
	if err != nil {
		t.Fatalf("execute = %v", err)
	}
	if err := output.Validate(calls[0]); err != nil {
		t.Fatalf("result validation = %v", err)
	}
	if err := registry.Register(testCapability{}); err == nil {
		t.Fatal("expected duplicate capability registration error")
	}
}

func TestCapabilityRequestIsAdvertisedAsOptionalToolAction(t *testing.T) {
	definition := capabilityRequestDefinition()
	if definition.Name != "capability.request" || definition.InputSchema == nil {
		t.Fatalf("capability request definition = %#v", definition)
	}
	call := ToolCallV1{ID: "need-1", Name: "capability.request", Arguments: json.RawMessage(`{"capability_key":"calendar.read","title":"读取日历","description":"需要知道日程安排","rationale":"帮助安排后续行动","desired_contract":{},"evidence_refs":["fact-1"]}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1", SchemaVersion: ToolCallSchemaVersion}
	action, err := resolveCapabilityAction(testInvocations([]ToolCallV1{call}), capabilityDefinitionMap([]CapabilityDefinition{definition}))
	if err != nil {
		t.Fatal(err)
	}
	if action != "no_op" {
		t.Fatalf("capability request action = %q, want no_op", action)
	}
}

func TestDefaultNativeCapabilitySlotsAreVersioned(t *testing.T) {
	registry, err := NewCapabilityRegistry(testCapability{})
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range []CapabilityDefinition{sceneCapabilityDefinition(), presenceCapabilityDefinition(), memoryCapabilityDefinition()} {
		if err := registry.Register(testCapabilityWithDefinition{definition: definition}); err != nil {
			t.Fatalf("register %s = %v", definition.Name, err)
		}
	}
	if got := len(registry.Definitions()); got != 4 {
		t.Fatalf("definition count = %d", got)
	}
}

func TestSceneCapabilityAdvertisesExplicitTransitionOperation(t *testing.T) {
	definition := sceneCapabilityDefinition()
	if definition.Name != "scene_event" {
		t.Fatalf("scene definition name = %q", definition.Name)
	}
	if !containsSchemaRequired(definition.InputSchema, "operation") {
		t.Fatalf("scene operation must be required: %#v", definition.InputSchema)
	}
	operation := mapValue(mapValue(definition.InputSchema["properties"])["operation"])
	values := arrayValue(operation["enum"])
	for _, want := range []string{"start", "switch", "end"} {
		found := false
		for _, raw := range values {
			if stringValue(raw) == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("scene operation enum missing %q: %#v", want, operation)
		}
	}
}

func TestRelationshipLookupCapabilityIsReadOnly(t *testing.T) {
	definition := relationshipLookupCapabilityDefinition()
	if definition.Name != "relationship.lookup" || definition.SideEffectClass != "read_only" || definition.InputSchema == nil {
		t.Fatalf("relationship lookup definition = %#v", definition)
	}
	if definition.IsDeferredOutput() {
		t.Fatal("relationship lookup must not be a deferred output")
	}
}

func TestMomentPublishCapabilityIsRegisteredDeferredOutput(t *testing.T) {
	definition := momentPublishCapabilityDefinition()
	if definition.Name != "moment.publish" || !definition.IsDeferredOutput() {
		t.Fatalf("moment publish definition = %#v", definition)
	}
	if !containsStringValue(stringSliceAny(definition.TargetKinds), "moment") {
		t.Fatalf("moment publish target kinds = %#v", definition.TargetKinds)
	}
	if !containsSchemaRequired(definition.InputSchema, "text") {
		t.Fatalf("moment publish parameters = %#v", definition.InputSchema)
	}
}

func TestAffectEventCapabilityOwnsSemanticEmotionInput(t *testing.T) {
	definition := affectEventCapabilityDefinition()
	if definition.Name != "affect_event" || definition.IsDeferredOutput() {
		t.Fatalf("affect definition = %#v", definition)
	}
	if !containsSchemaRequired(definition.InputSchema, "event") {
		t.Fatalf("affect parameters = %#v", definition.InputSchema)
	}
	if containsSchemaRequired(mapValue(mapValue(definition.InputSchema["properties"])["event"]), "pad") {
		t.Fatal("affect event must not expose raw PAD input")
	}
}

func TestConversationCapabilityCatalogOmitsMomentOutput(t *testing.T) {
	registry, err := NewCapabilityRegistry(conversationReplyCapability{}, momentPublishCapability{}, imageGenerateCapability{})
	if err != nil {
		t.Fatal(err)
	}
	definitions := registry.Catalog(CapabilitySurfaceConversation)
	foundImage := false
	for _, definition := range definitions {
		if definition.Name == "moment.publish" {
			t.Fatalf("moment.publish leaked into conversation catalog: %#v", definitions)
		}
		if definition.Name == "media.image.generate" {
			foundImage = true
		}
	}
	if !foundImage {
		t.Fatal("media.image.generate missing from conversation catalog")
	}
}

func TestImageDeferredFailureDoesNotClassifyConversationReplyAsFatal(t *testing.T) {
	registry, err := NewCapabilityRegistry(imageGenerateCapability{}, conversationReplyCapability{})
	if err != nil {
		t.Fatal(err)
	}
	if definition, ok := registry.Definition("media.image.generate"); !ok || definition.FailurePolicy != FailurePolicyRequiredForVisibleClaim {
		t.Fatalf("image capability policy = %#v", definition)
	}
	if definition, ok := registry.Definition("conversation.reply"); !ok || definition.FailurePolicy != FailurePolicyRequiredForVisibleClaim {
		t.Fatalf("reply capability policy = %#v", definition)
	}
}

func TestToolOnlyActionDoesNotRequireConversationReply(t *testing.T) {
	action, err := resolveCapabilityAction([]CapabilityInvocation{{CapabilityName: "affect_event"}}, capabilityDefinitionMap([]CapabilityDefinition{affectEventCapabilityDefinition()}))
	if err != nil || action != "no_op" {
		t.Fatalf("tool-only action = %q err=%v", action, err)
	}
	action, err = resolveCapabilityAction([]CapabilityInvocation{{CapabilityName: "conversation.reply", Arguments: json.RawMessage(`{"text":"你好"}`)}}, capabilityDefinitionMap([]CapabilityDefinition{conversationReplyCapabilityDefinition()}))
	if err != nil || action != "reply" {
		t.Fatalf("reply action = %q err=%v", action, err)
	}
}

func TestOptionalToolFailuresDoNotAbortConversation(t *testing.T) {
	registry, err := NewCapabilityRegistry(affectEventCapability{}, memoryEventCapability{}, activeMemoryEventCapability{}, memoryRecallCapability{}, relationshipLookupCapability{}, capabilityRequestCapability{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"affect_event", "active_memory_event", "memory.recall", "relationship.lookup", "capability.request"} {
		if definition, ok := registry.Definition(name); !ok || definition.FailurePolicy != FailurePolicyOptionalInternal {
			t.Fatalf("tool %s should be optional", name)
		}
	}
	if definition, ok := registry.Definition("memory_event"); !ok || definition.FailurePolicy != FailurePolicyRequiredForVisibleClaim {
		t.Fatalf("explicit Memory write must fail the turn when its claimed revision cannot commit: %#v", definition)
	}
}

type testCapabilityWithDefinition struct{ definition CapabilityDefinition }

func (executor testCapabilityWithDefinition) Definition() CapabilityDefinition {
	return executor.definition
}
func (executor testCapabilityWithDefinition) RequiredContext() []ContextSlot {
	return executor.definition.RequiredContext
}
func (executor testCapabilityWithDefinition) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", ProviderRequestID: invocation.ProviderRequestID}, nil
}

func TestProviderChatPayloadUsesToolsInsteadOfProseControl(t *testing.T) {
	messages := []map[string]any{{"role": "user", "content": "draw a cat"}}
	payload := providerChatPayload("model", messages, 512, false, testCapabilityDefinitions())
	if _, ok := payload["response_format"]; ok {
		t.Fatal("tool-call payload must not force JSON response_format")
	}
	if payload["tool_choice"] != "auto" {
		t.Fatalf("tool_choice = %#v", payload["tool_choice"])
	}
	tools, ok := payload["tools"].([]map[string]any)
	if !ok || len(tools) != 4 {
		t.Fatalf("tools = %#v", payload["tools"])
	}
	if payload["max_tokens"] != 512 {
		t.Fatalf("max_tokens = %#v", payload["max_tokens"])
	}

	sidecar := providerChatPayload("model", messages, 0, true, nil)
	if _, ok := sidecar["tools"]; ok {
		t.Fatal("sidecar payload must not advertise absent tools")
	}
	if _, ok := sidecar["response_format"]; !ok {
		t.Fatal("structured sidecar payload must request JSON mode")
	}
}

func TestStructuredProviderPayloadUsesJSONFormatAndCognitiveThinking(t *testing.T) {
	assessment := providerChatPayloadForRole("model", []map[string]any{{"role": "user", "content": "hello"}}, 512, true, testCapabilityDefinitions(), "cognitive_assessment")
	format, ok := assessment["response_format"].(map[string]any)
	if !ok || format["type"] != "json_schema" {
		t.Fatalf("cognitive assessment must request JSON output: %#v", assessment)
	}
	schemaEnvelope, ok := format["json_schema"].(map[string]any)
	if !ok || schemaEnvelope["strict"] != true || schemaEnvelope["schema"] == nil {
		t.Fatalf("cognitive assessment schema is not strict: %#v", format)
	}
	if assessment["enable_thinking"] != true {
		t.Fatalf("cognitive assessment must enable thinking: %#v", assessment)
	}
	reflection := providerChatPayloadForRole("model", nil, 512, true, nil, "reflection")
	reflectionFormat, ok := reflection["response_format"].(map[string]any)
	if !ok || reflectionFormat["type"] != "json_schema" {
		t.Fatalf("structured provider must request JSON output: %#v", reflection)
	}
	if _, ok := reflection["enable_thinking"]; ok {
		t.Fatalf("non-cognitive provider must not enable thinking: %#v", reflection)
	}
	text := providerChatPayloadForRole("model", nil, 512, false, nil, "action_realization")
	if _, ok := text["response_format"]; ok {
		t.Fatalf("text realization must not force JSON output: %#v", text)
	}
}

func TestDailyReviewProviderPayloadUsesItsCompositeActionSchema(t *testing.T) {
	payload := providerChatPayloadWithSchema("model", nil, 512, true, testCapabilityDefinitions(), "cognitive_assessment", "daily_review_response", dailyReviewResponseSchema(), true)
	format, ok := payload["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("daily review response_format = %#v", payload["response_format"])
	}
	envelope, ok := format["json_schema"].(map[string]any)
	if !ok || envelope["name"] != "daily_review_response" || envelope["strict"] != true {
		t.Fatalf("daily review schema envelope = %#v", format["json_schema"])
	}
	if payload["enable_thinking"] != true {
		t.Fatalf("daily review must enable thinking: %#v", payload)
	}
	schema, ok := envelope["schema"].(map[string]any)
	if !ok || !containsSchemaRequired(schema, "action_type") || !containsSchemaRequired(schema, "response_intent") {
		t.Fatalf("daily review schema = %#v", envelope["schema"])
	}
}

func TestProviderStructuredContentAcceptsMlxReasoningContent(t *testing.T) {
	message := map[string]any{
		"content":           "",
		"reasoning_content": `{"names":["李雷","韩梅梅"]}`,
	}
	if got := providerStructuredContent(message); got != `{"names":["李雷","韩梅梅"]}` {
		t.Fatalf("structured content = %q", got)
	}
	content := map[string]any{"content": `{"action_type":"reply"}`, "reasoning_content": `{"action_type":"wrong"}`}
	if got := providerStructuredContent(content); got != `{"action_type":"reply"}` {
		t.Fatalf("content should take precedence over reasoning_content: %q", got)
	}
	if parsed, ok := parseStructuredCandidates(providerStructuredCandidates(map[string]any{
		"content":           `{"action_type":"reply"}尾部文本`,
		"reasoning_content": `{"action_type":"reply","visible_text":"你好"}`,
	})); !ok || parsed["visible_text"] != "你好" {
		t.Fatalf("invalid content should fall back to valid reasoning JSON: %#v, ok=%v", parsed, ok)
	}
}

func TestProviderStructuredContentAcceptsWrappedAndEncodedJSON(t *testing.T) {
	cases := []struct {
		name      string
		message   map[string]any
		wantValue string
	}{
		{
			name: "thinking wrapper",
			message: map[string]any{
				"content":           "",
				"reasoning_content": "<think>先判断动作</think>\n```json\n{\"action_type\":\"no_op\",\"response_intent\":\"\",\"tool_calls\":[]}\n```",
			},
			wantValue: "no_op",
		},
		{
			name: "json inside thinking block",
			message: map[string]any{
				"content":           "",
				"reasoning_content": "<think>\n```json\n{\"action_type\":\"no_op\",\"response_intent\":\"\",\"tool_calls\":[]}\n```\n</think>",
			},
			wantValue: "no_op",
		},
		{
			name: "thinking prelude before terminal object",
			message: map[string]any{
				"content":           "",
				"reasoning_content": "先完成内部判断，然后给出结构化结果：\n{\"action_type\":\"no_op\",\"response_intent\":\"\",\"tool_calls\":[]}",
			},
			wantValue: "no_op",
		},
		{
			name: "double encoded reasoning",
			message: map[string]any{
				"content":           "",
				"reasoning_content": `"{\"action_type\":\"no_op\",\"response_intent\":\"\",\"tool_calls\":[]}"`,
			},
			wantValue: "no_op",
		},
		{
			name: "object reasoning channel",
			message: map[string]any{
				"content":           nil,
				"reasoning_content": map[string]any{"action_type": "no_op", "response_intent": "", "tool_calls": []any{}},
			},
			wantValue: "no_op",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			parsed, ok := parseStructuredCandidates(providerStructuredCandidates(testCase.message))
			if !ok || stringValue(parsed["action_type"]) != testCase.wantValue {
				t.Fatalf("parsed=%#v ok=%v", parsed, ok)
			}
		})
	}
	if parsed, ok := parseStructuredCandidates(providerStructuredCandidates(map[string]any{
		"content":           `{"action_type":"reply"}尾部文本`,
		"reasoning_content": `{"action_type":"no_op","response_intent":"","tool_calls":[]}`,
	})); !ok || stringValue(parsed["action_type"]) != "no_op" {
		t.Fatalf("malformed content must fall back to reasoning JSON: parsed=%#v ok=%v", parsed, ok)
	}
}

func TestProviderStructuredFailureDiagnosticRetainsChannelShape(t *testing.T) {
	message := map[string]any{
		"content":           "",
		"reasoning_content": "先思考，再输出一个不完整的结果",
	}
	diagnostic := providerResponseDiagnostic(message, providerStructuredCandidates(message), 0)
	if diagnostic["reasoning_content_present"] != true || diagnostic["reasoning_content_length"] == 0 {
		t.Fatalf("reasoning channel shape was not retained: %#v", diagnostic)
	}
	if diagnostic["content_present"] != false || diagnostic["parse_error"] != "structured_response_invalid" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestNormalizeStructuredShapeOnlyRepairsAbnormalFields(t *testing.T) {
	schema := objectSchema(map[string]any{
		"valid_items":    arraySchema(stringSchema()),
		"array_expected": arraySchema(openObjectSchema()),
		"object_expected": objectSchema(map[string]any{
			"name": stringSchema(),
		}, []string{"name"}, false),
		"missing_text": stringSchema(),
	}, []string{"valid_items", "array_expected", "object_expected", "missing_text"}, false)
	value := map[string]any{
		"valid_items":    []any{"keep", "as-is"},
		"array_expected": map[string]any{"id": "call-1"},
		"object_expected": []any{
			map[string]any{"name": "first"},
			map[string]any{"name": "second"},
		},
	}
	normalized, fields := normalizeStructuredShape(value, schema)
	if got := arrayValue(normalized["valid_items"]); len(got) != 2 || stringValue(got[0]) != "keep" {
		t.Fatalf("valid array was changed: %#v", normalized["valid_items"])
	}
	if got := arrayValue(normalized["array_expected"]); len(got) != 1 || stringValue(mapValue(got[0])["id"]) != "call-1" {
		t.Fatalf("object was not wrapped as array: %#v", normalized["array_expected"])
	}
	if got := mapValue(normalized["object_expected"]); got["name"] != "first" {
		t.Fatalf("array was not reduced to first object: %#v", normalized["object_expected"])
	}
	if normalized["missing_text"] != "" {
		t.Fatalf("missing required field was not emptied: %#v", normalized["missing_text"])
	}
	for _, field := range []string{"array_expected", "object_expected", "missing_text"} {
		found := false
		for _, changed := range fields {
			if changed == field {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("field %q was not reported as normalized: %#v", field, fields)
		}
	}
}

func TestEmptyProviderStructuredUsesOperationTypedEmptyValues(t *testing.T) {
	value, fields := emptyProviderStructured("wake_up_response", wakeUpResponseSchema())
	if value["action_type"] != "no_op" || value["response_intent"] != "" {
		t.Fatalf("wake-up empty values = %#v", value)
	}
	if refs := arrayValue(value["evidence_refs"]); len(refs) != 0 {
		t.Fatalf("wake-up evidence refs = %#v", refs)
	}
	if len(arrayValue(value["tool_calls"])) != 0 {
		t.Fatalf("wake-up empty tool calls = %#v", value["tool_calls"])
	}
	if len(fields) == 0 {
		t.Fatal("empty fallback should report normalized fields")
	}
}

func TestOperationSpecificResponseSchemasRequireTheirDomainShape(t *testing.T) {
	schedule := scheduleResponseSchema()
	if !containsSchemaRequired(schedule, "items") || !containsSchemaRequired(schedule, "reschedule_policy") {
		t.Fatalf("schedule schema required fields = %#v", schedule)
	}
	itemsProperty := mapValue(mapValue(schedule["properties"])["items"])
	itemSchema := mapValue(itemsProperty["items"])
	for _, key := range []string{"start_at", "end_at", "activity", "scene", "item_type", "status", "priority", "flexibility", "interruption_cost"} {
		if !containsSchemaRequired(itemSchema, key) {
			t.Fatalf("schedule item schema missing required field %q: %#v", key, itemSchema)
		}
	}
	for _, key := range []string{"priority", "flexibility", "interruption_cost"} {
		value := mapValue(mapValue(itemSchema["properties"])[key])
		if value["minimum"] != float64(0) && value["minimum"] != 0 {
			t.Fatalf("schedule item %s must have minimum 0: %#v", key, value)
		}
		if value["maximum"] != float64(1) && value["maximum"] != 1 {
			t.Fatalf("schedule item %s must have maximum 1: %#v", key, value)
		}
	}
	reflection := reflectionProposalV2ProviderSchema()
	for _, key := range []string{"active_memory_candidates", "memory_candidates", "relationship_observations", "emotional_summary", "developing_self_candidates", "drive_candidates", "trigger_candidates", "personality_evolution_candidates", "behavior_policy_evolution_candidates"} {
		if !containsSchemaRequired(reflection, key) {
			t.Fatalf("reflection schema missing required field %q: %#v", key, reflection)
		}
	}
	memoryCandidate := mapValue(mapValue(mapValue(reflection["properties"])["memory_candidates"])["items"])
	if memoryCandidate["additionalProperties"] != false {
		t.Fatalf("reflection Memory candidate must be closed: %#v", memoryCandidate)
	}
	memoryCandidateProperties := mapValue(memoryCandidate["properties"])
	for _, required := range []string{"operation", "target_ref", "merge_refs", "evidence_refs", "semantic_reason"} {
		if _, ok := memoryCandidateProperties[required]; !ok {
			t.Fatalf("reflection Memory candidate missing %q: %#v", required, memoryCandidate)
		}
	}
	for _, runtimeOwned := range []string{"memory_id", "expected_revision", "profile_id", "visibility", "personality_perspectives", "actor_refs", "event_refs", "conversation_id", "idempotency_key", "provenance"} {
		if _, ok := memoryCandidateProperties[runtimeOwned]; ok {
			t.Fatalf("reflection Memory candidate exposes runtime field %q: %#v", runtimeOwned, memoryCandidate)
		}
	}
	activeMemoryCandidate := mapValue(mapValue(mapValue(reflection["properties"])["active_memory_candidates"])["items"])
	if activeMemoryCandidate["additionalProperties"] != false {
		t.Fatalf("reflection Active Memory candidate must be closed: %#v", activeMemoryCandidate)
	}
	activeMemoryProperties := mapValue(activeMemoryCandidate["properties"])
	for _, required := range []string{"operation", "target_ref", "kind", "content", "confidence", "importance", "original_time_expression", "valid_from", "valid_until", "time_precision", "evidence_refs", "semantic_reason"} {
		if _, ok := activeMemoryProperties[required]; !ok {
			t.Fatalf("reflection Active Memory candidate missing %q: %#v", required, activeMemoryCandidate)
		}
	}
	for _, runtimeOwned := range []string{"active_memory_id", "expected_revision", "owner_fluctlight_id", "owner_actor_id", "actor_id", "conversation_id", "source_fact_id", "timezone", "canonical_key", "request_digest", "idempotency_key", "policy_version"} {
		if _, ok := activeMemoryProperties[runtimeOwned]; ok {
			t.Fatalf("reflection Active Memory candidate exposes runtime field %q: %#v", runtimeOwned, activeMemoryCandidate)
		}
	}
	cognitive := cognitiveTurnResponseSchema()
	for _, key := range []string{"action_type", "appraisal", "attention", "thought", "desire", "agency", "personality_decision", "output_preference_decision", "tool_calls"} {
		if _, ok := mapValue(cognitive["properties"])[key]; !ok {
			t.Fatalf("cognitive schema missing property %q: %#v", key, cognitive)
		}
	}
	cognitiveActionSchema := mapValue(mapValue(cognitive["properties"])["action_type"])
	if values := arrayValue(cognitiveActionSchema["enum"]); len(values) != 1 || stringValue(values[0]) != "reply" {
		t.Fatalf("direct cognitive action_type must require reply: %#v", cognitiveActionSchema)
	}
	outputDecision := mapValue(mapValue(cognitive["properties"])["output_preference_decision"])
	if !containsSchemaRequired(outputDecision, "profile_id") || !containsSchemaRequired(outputDecision, "trigger_id") {
		t.Fatalf("output preference decision must identify its profile and trigger: %#v", outputDecision)
	}
	personalityDecision := mapValue(mapValue(cognitive["properties"])["personality_decision"])
	for _, key := range []string{"from_profile_id", "target_profile_id", "trigger_id"} {
		if !containsSchemaRequired(personalityDecision, key) {
			t.Fatalf("personality decision must identify %q: %#v", key, personalityDecision)
		}
	}
	claimSchemaValue := mapValue(mapValue(mapValue(cognitive["properties"])["claims"])["items"])
	for _, key := range []string{"kind", "content", "confidence", "evidence_refs"} {
		if !containsSchemaRequired(claimSchemaValue, key) {
			t.Fatalf("cognitive claim schema missing required field %q: %#v", key, claimSchemaValue)
		}
	}
	if claimSchemaValue["additionalProperties"] != false {
		t.Fatalf("cognitive claim schema must be closed: %#v", claimSchemaValue)
	}
	responsePlan := mapValue(mapValue(cognitive["properties"])["response_plan"])
	if _, ok := mapValue(responsePlan["properties"])["profile_id"]; !ok {
		t.Fatalf("response plan must expose the active profile: %#v", responsePlan)
	}
	selfEvaluation := mapValue(mapValue(cognitive["properties"])["self_evaluation"])
	for _, key := range []string{"mode", "reason_codes", "confidence"} {
		if _, ok := mapValue(selfEvaluation["properties"])[key]; !ok {
			t.Fatalf("self evaluation schema missing property %q: %#v", key, selfEvaluation)
		}
	}
	memorySchema := mapValue(memoryCapabilityDefinition().InputSchema)
	memoryProperties := mapValue(memorySchema["properties"])
	for _, runtimeOwned := range []string{"operation", "target_ref", "merge_refs", "memory_id", "expected_revision", "personality_perspectives", "profile_id", "evidence_refs", "provenance", "idempotency_key", "visibility", "actor_refs", "event_refs", "conversation_id", "source_fact_id"} {
		if _, found := memoryProperties[runtimeOwned]; found {
			t.Fatalf("memory provider schema exposes runtime-owned field %q: %#v", runtimeOwned, memoryProperties)
		}
	}
	typeSchema := mapValue(memoryProperties["type"])
	if containsStringValue(arrayValue(typeSchema["enum"]), "working") {
		t.Fatalf("durable working Memory leaked into memory_event schema: %#v", typeSchema)
	}
	daily := dailyReviewResponseSchema()
	actionSchema := mapValue(mapValue(daily["properties"])["action_type"])
	if actionSchema["type"] != "string" || len(arrayValue(actionSchema["enum"])) != 3 {
		t.Fatalf("daily review action_type must be the three-value enum: %#v", actionSchema)
	}
}

func containsSchemaRequired(schema map[string]any, key string) bool {
	for _, raw := range arrayValue(schema["required"]) {
		if stringValue(raw) == key {
			return true
		}
	}
	return false
}

func TestResolveToolCallActionSupportsNativeObservationSlots(t *testing.T) {
	manifests := capabilityDefinitionMap([]CapabilityDefinition{sceneCapabilityDefinition(), presenceCapabilityDefinition(), memoryCapabilityDefinition()})
	calls, err := NormalizeProviderToolCalls([]any{
		map[string]any{"id": "scene", "name": "scene_event", "arguments": map[string]any{"scene": "cafe", "activity": "read", "source_fact_id": "fact", "evidence_refs": []any{"fact"}, "confidence": 0.8}},
		map[string]any{"id": "presence", "name": "presence_event", "arguments": map[string]any{"current_task": "chat", "source_fact_id": "fact", "evidence_refs": []any{"fact"}, "confidence": 0.9}},
	}, "fact", "provider")
	if err != nil {
		t.Fatalf("normalize = %v", err)
	}
	action, err := resolveCapabilityAction(calls, manifests)
	concept := map[string]any{}
	if err != nil {
		t.Fatalf("resolve = %v", err)
	}
	if action != "no_op" || len(concept) != 0 {
		t.Fatalf("action=%q concept=%#v", action, concept)
	}
}

func TestResolveToolCallActionKeepsMediaAsReplyComposite(t *testing.T) {
	call := ToolCallV1{
		ID: "media-1", Name: "media.image.generate",
		Arguments:    json.RawMessage(`{"concept":{"scene":"window"}}`),
		SourceFactID: "fact", ProviderRequestID: "provider", SchemaVersion: ToolCallSchemaVersion,
	}
	action, err := resolveCapabilityAction(testInvocations([]ToolCallV1{call}), capabilityDefinitionMap(testCapabilityDefinitions()))
	concept := map[string]any{}
	if err != nil {
		t.Fatal(err)
	}
	if action != "no_op" || len(concept) != 0 {
		t.Fatalf("action=%q concept=%#v; media-only calls are no-op outputs", action, concept)
	}
}
