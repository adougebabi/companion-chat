package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type testCapabilityExecutor struct{}

func (testCapabilityExecutor) Manifest() CapabilityManifest {
	return CapabilityManifest{Name: "search.lookup", Version: "v1", Description: "Look up a bounded fact.", SideEffectClass: "read_only", ConcurrencyClass: "parallel"}
}

func (testCapabilityExecutor) Execute(_ context.Context, _, _, _ string, call ToolCallV1) (ToolResultV1, error) {
	return ToolResultV1{ToolCallID: call.ID, Name: call.Name, Status: "completed", Output: map[string]any{"value": "ok"}, SchemaVersion: ToolResultSchemaVersion}, nil
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
	if calls[0].SchemaVersion != ToolCallSchemaVersion || calls[0].SourceFactID != "fact-1" {
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
	if len(calls) != 1 || calls[0].ID != "call_single" || calls[0].Sequence != 0 {
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
	manifests := toolManifestMap(ExternalCapabilityManifests())
	calls, err := NormalizeProviderToolCalls([]any{map[string]any{
		"id":   "call",
		"name": "media.image.generate",
		"arguments": map[string]any{
			"concept": map[string]any{"subject": "a cat"},
		},
	}}, "fact", "provider")
	if err != nil {
		t.Fatalf("normalize = %v", err)
	}
	if err := calls[0].Validate(manifests); err != nil {
		t.Fatalf("registered capability rejected = %v", err)
	}
	calls[0].Name = "media.video.generate"
	if err := calls[0].Validate(manifests); err == nil {
		t.Fatal("expected unavailable capability error")
	}
}

func TestToolCallPayloadKeepsProviderSchemaAtBoundary(t *testing.T) {
	payload := ToolCallPayload(ExternalCapabilityManifests())
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
	manifest := imageCapabilityManifest()
	if len(manifest.TargetKinds) != 2 || manifest.TargetKinds[0] != "conversation_message" || manifest.TargetKinds[1] != "moment" {
		t.Fatalf("media target kinds = %#v", manifest.TargetKinds)
	}
	if !manifest.IsDeferredOutput() || manifest.OutputSchema == nil {
		t.Fatalf("media manifest must be a typed deferred output slot: %#v", manifest)
	}
	if err := manifest.ValidateOutput(map[string]any{"media_intent_id": "intent"}); err == nil {
		t.Fatal("missing typed output target fields must be rejected")
	}
	replyManifest := conversationReplyCapabilityManifest()
	if !replyManifest.IsDeferredOutput() || replyManifest.TargetKinds[0] != "conversation_message" {
		t.Fatalf("conversation reply must be a deferred conversation output: %#v", replyManifest)
	}
}

func TestVisualIdentityInitializationManifestIsWakeUpCapability(t *testing.T) {
	manifest := visualIdentityInitializeCapabilityManifest()
	if manifest.Name != "visual_identity.initialize" || manifest.IsDeferredOutput() || manifest.SideEffectClass != "native_projection" {
		t.Fatalf("visual identity manifest = %#v", manifest)
	}
	if err := manifest.ValidateOutput(map[string]any{"session_id": "session-1", "status": "queued"}); err != nil {
		t.Fatalf("manifest output rejected: %v", err)
	}
}

func TestCapabilityRegistryIsAnExtensibleSlot(t *testing.T) {
	registry := NewCapabilityRegistry(testCapabilityExecutor{})
	manifests := registry.Manifests()
	if len(manifests) != 1 || manifests[0].Name != "search.lookup" {
		t.Fatalf("manifests = %#v", manifests)
	}
	calls, err := NormalizeProviderToolCalls([]any{map[string]any{"id": "call", "name": "search.lookup", "arguments": `{ "query": "fluctlight" }`}}, "fact", "provider")
	if err != nil {
		t.Fatalf("normalize = %v", err)
	}
	if err := calls[0].Validate(toolManifestMap(manifests)); err != nil {
		t.Fatalf("call validation = %v", err)
	}
	result, ok := registry.Lookup("search.lookup")
	if !ok || result == nil {
		t.Fatalf("lookup = %#v, ok=%v", result, ok)
	}
	output, err := result.Execute(context.Background(), "fl", "conv", "fact", calls[0])
	if err != nil {
		t.Fatalf("execute = %v", err)
	}
	if err := output.Validate(calls[0]); err != nil {
		t.Fatalf("result validation = %v", err)
	}
	if err := registry.Register(testCapabilityExecutor{}); err == nil {
		t.Fatal("expected duplicate capability registration error")
	}
}

func TestCapabilityRequestIsAdvertisedAsOptionalToolAction(t *testing.T) {
	manifest := capabilityRequestManifest()
	if manifest.Name != "capability.request" || manifest.Parameters == nil {
		t.Fatalf("capability request manifest = %#v", manifest)
	}
	call := ToolCallV1{ID: "need-1", Name: "capability.request", Arguments: json.RawMessage(`{"capability_key":"calendar.read","title":"读取日历","description":"需要知道日程安排","rationale":"帮助安排后续行动","desired_contract":{},"evidence_refs":["fact-1"]}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1", SchemaVersion: ToolCallSchemaVersion}
	action, _, err := resolveToolCallAction([]ToolCallV1{call}, toolManifestMap([]CapabilityManifest{manifest}))
	if err != nil {
		t.Fatal(err)
	}
	if action != "no_op" {
		t.Fatalf("capability request action = %q, want no_op", action)
	}
}

func TestDefaultNativeCapabilitySlotsAreVersioned(t *testing.T) {
	registry := NewCapabilityRegistry(testCapabilityExecutor{})
	for _, manifest := range []CapabilityManifest{sceneCapabilityManifest(), presenceCapabilityManifest(), memoryCapabilityManifest()} {
		if err := registry.Register(testManifestExecutor{manifest: manifest}); err != nil {
			t.Fatalf("register %s = %v", manifest.Name, err)
		}
	}
	if got := len(registry.Manifests()); got != 4 {
		t.Fatalf("manifest count = %d", got)
	}
}

func TestRelationshipLookupCapabilityIsReadOnly(t *testing.T) {
	manifest := relationshipLookupCapabilityManifest()
	if manifest.Name != "relationship.lookup" || manifest.SideEffectClass != "read_only" || manifest.Parameters == nil {
		t.Fatalf("relationship lookup manifest = %#v", manifest)
	}
	if manifest.IsDeferredOutput() {
		t.Fatal("relationship lookup must not be a deferred output")
	}
}

func TestMomentPublishCapabilityIsRegisteredDeferredOutput(t *testing.T) {
	manifest := momentPublishCapabilityManifest()
	if manifest.Name != "moment.publish" || !manifest.IsDeferredOutput() {
		t.Fatalf("moment publish manifest = %#v", manifest)
	}
	if !containsStringValue(stringSliceAny(manifest.TargetKinds), "moment") {
		t.Fatalf("moment publish target kinds = %#v", manifest.TargetKinds)
	}
	if !containsSchemaRequired(manifest.Parameters, "text") {
		t.Fatalf("moment publish parameters = %#v", manifest.Parameters)
	}
}

func TestAffectEventCapabilityOwnsSemanticEmotionInput(t *testing.T) {
	manifest := affectEventCapabilityManifest()
	if manifest.Name != "affect_event" || manifest.IsDeferredOutput() {
		t.Fatalf("affect manifest = %#v", manifest)
	}
	if !containsSchemaRequired(manifest.Parameters, "event") {
		t.Fatalf("affect parameters = %#v", manifest.Parameters)
	}
	if containsSchemaRequired(mapValue(mapValue(manifest.Parameters["properties"])["event"]), "pad") {
		t.Fatal("affect event must not expose raw PAD input")
	}
}

func TestConversationCapabilityCatalogOmitsMomentOutput(t *testing.T) {
	registry := NewCapabilityRegistry(&conversationReplyCapabilityExecutor{}, &momentPublishCapabilityExecutor{}, &imageCapabilityExecutor{})
	manifests := capabilityManifestsExcept(registry, "moment.publish")
	foundImage := false
	for _, manifest := range manifests {
		if manifest.Name == "moment.publish" {
			t.Fatalf("moment.publish leaked into conversation catalog: %#v", manifests)
		}
		if manifest.Name == "media.image.generate" {
			foundImage = true
		}
	}
	if !foundImage {
		t.Fatal("media.image.generate missing from conversation catalog")
	}
}

func TestImageDeferredFailureDoesNotClassifyConversationReplyAsFatal(t *testing.T) {
	for _, name := range []string{"media.image.generate", "conversation.reply"} {
		if !optionalToolFailureNonFatal(ToolCallV1{Name: name}) {
			t.Fatalf("tool %s should be optional", name)
		}
	}
}

func TestToolOnlyActionDoesNotRequireConversationReply(t *testing.T) {
	action, _, err := resolveToolCallAction([]ToolCallV1{{Name: "affect_event"}}, toolManifestMap([]CapabilityManifest{affectEventCapabilityManifest()}))
	if err != nil || action != "no_op" {
		t.Fatalf("tool-only action = %q err=%v", action, err)
	}
	action, _, err = resolveToolCallAction([]ToolCallV1{{Name: "conversation.reply", Arguments: json.RawMessage(`{"text":"你好"}`)}}, toolManifestMap([]CapabilityManifest{conversationReplyCapabilityManifest()}))
	if err != nil || action != "reply" {
		t.Fatalf("reply action = %q err=%v", action, err)
	}
}

func TestOptionalToolFailuresDoNotAbortConversation(t *testing.T) {
	for _, name := range []string{"affect_event", "scene_event", "presence_event", "memory_event", "relationship.lookup", "capability.request"} {
		if !optionalToolFailureNonFatal(ToolCallV1{Name: name}) {
			t.Fatalf("tool %s should be optional", name)
		}
	}
}

type testManifestExecutor struct{ manifest CapabilityManifest }

func (executor testManifestExecutor) Manifest() CapabilityManifest { return executor.manifest }
func (executor testManifestExecutor) Execute(_ context.Context, _, _, _ string, call ToolCallV1) (ToolResultV1, error) {
	return ToolResultV1{ToolCallID: call.ID, Name: call.Name, Status: "completed", SchemaVersion: ToolResultSchemaVersion}, nil
}

func TestProviderChatPayloadUsesToolsInsteadOfProseControl(t *testing.T) {
	messages := []map[string]any{{"role": "user", "content": "draw a cat"}}
	payload := providerChatPayload("model", messages, 512, false, ExternalCapabilityManifests())
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
	assessment := providerChatPayloadForRole("model", []map[string]any{{"role": "user", "content": "hello"}}, 512, true, ExternalCapabilityManifests(), "cognitive_assessment")
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
	payload := providerChatPayloadWithSchema("model", nil, 512, true, ExternalCapabilityManifests(), "cognitive_assessment", "daily_review_response", dailyReviewResponseSchema(), true)
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
	reflection := reflectionResponseSchema()
	for _, key := range []string{"memory_candidates", "developing_self_candidates", "drive_candidates", "trigger_candidates"} {
		if !containsSchemaRequired(reflection, key) {
			t.Fatalf("reflection schema missing required field %q: %#v", key, reflection)
		}
	}
	cognitive := cognitiveTurnResponseSchema()
	for _, key := range []string{"action_type", "appraisal", "attention", "thought", "desire", "agency", "personality_decision", "output_preference_decision", "tool_calls"} {
		if _, ok := mapValue(cognitive["properties"])[key]; !ok {
			t.Fatalf("cognitive schema missing property %q: %#v", key, cognitive)
		}
	}
	cognitiveActionSchema := mapValue(mapValue(cognitive["properties"])["action_type"])
	if len(arrayValue(cognitiveActionSchema["enum"])) != 3 {
		t.Fatalf("cognitive action_type must have reply/media_request/no_op enum: %#v", cognitiveActionSchema)
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
	memoryParameters := mapValue(memoryCapabilityManifest().Parameters)
	perspectives := mapValue(mapValue(memoryParameters["properties"])["personality_perspectives"])
	if perspectives["type"] != "array" {
		t.Fatalf("memory perspectives must be an array: %#v", perspectives)
	}
	perspectiveItem := mapValue(perspectives["items"])
	if !containsSchemaRequired(perspectiveItem, "profile_id") || !containsSchemaRequired(perspectiveItem, "interpretation") {
		t.Fatalf("memory perspective schema is incomplete: %#v", perspectiveItem)
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
	manifests := toolManifestMap([]CapabilityManifest{sceneCapabilityManifest(), presenceCapabilityManifest(), memoryCapabilityManifest()})
	calls, err := NormalizeProviderToolCalls([]any{
		map[string]any{"id": "scene", "name": "scene_event", "arguments": map[string]any{"scene": "cafe", "activity": "read", "source_fact_id": "fact", "evidence_refs": []any{"fact"}, "confidence": 0.8}},
		map[string]any{"id": "presence", "name": "presence_event", "arguments": map[string]any{"current_task": "chat", "source_fact_id": "fact", "evidence_refs": []any{"fact"}, "confidence": 0.9}},
	}, "fact", "provider")
	if err != nil {
		t.Fatalf("normalize = %v", err)
	}
	action, concept, err := resolveToolCallAction(calls, manifests)
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
	action, concept, err := resolveToolCallAction([]ToolCallV1{call}, toolManifestMap(ExternalCapabilityManifests()))
	if err != nil {
		t.Fatal(err)
	}
	if action != "no_op" || len(concept) != 0 {
		t.Fatalf("action=%q concept=%#v; media-only calls are no-op outputs", action, concept)
	}
}
