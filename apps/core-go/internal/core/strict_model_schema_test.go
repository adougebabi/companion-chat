package core

import (
	"strings"
	"testing"

	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func TestStrictWireSchemasCoverRegisteredModelContracts(t *testing.T) {
	schemas := map[string]map[string]any{
		"schedule_response":                 scheduleResponseSchema(),
		"schedule_replan_plan":              schedulePlannerOutputSchema(),
		"conversation_turn_response":        cognitiveTurnResponseSchema(),
		"daily_review_response":             dailyReviewResponseSchema(),
		"wake_up_response":                  wakeUpResponseSchema(),
		"native_cognition_response":         nativeCognitionResponseSchema(),
		"persona_compilation_response":      personaCompilationResponseSchema(),
		"initialization_response":           initializationResponseSchema(),
		"reflection_proposal_v2":            reflectionProposalV2ProviderSchema(),
		"visual_identity_vision_response":   visualIdentityVisionResponseSchema(),
		"visual_identity_patch_response":    visualIdentityPatchResponseSchema(),
		"visual_identity_seed_response":     visualIdentitySeedResponseSchema(),
		"visual_identity_agent_response":    visualIdentityAgentResponseSchema(),
		"media_quality_acceptance_response": mediaQualityAcceptanceResponseSchema(),
		"takeover_judgement":                takeoverJudgementSchema(),
		"persistent_switch_assessment":      persistentSwitchAssessmentSchema(),
		"conversation_summary":              conversationSummaryProviderSchema(),
		"conversation_segment":              conversationSegmentProviderSchema(),
		"conversation_daily_episode":        conversationDailyEpisodeProviderSchema(),
		"virtual_activity_result":           virtualActivityResultSchema(),
		"current_capture_plan":              currentCapturePlanSchema(),
		"media_capture_plan":                mediaCapturePlanSchema(),
		"media_normalization":               mediaNormalizationSchema(),
		"appraisal_response":                appraisalResponseSchema(),
	}
	for name, schema := range schemas {
		wire := capabilitycontract.StrictModelSchema(schema)
		assertStrictWireSchema(t, name, wire)
		if schema["minLength"] != nil || nestedKeyword(schema, "minLength") {
			if nestedKeyword(wire, "minLength") || nestedKeyword(wire, "maxLength") || nestedKeyword(wire, "maxItems") {
				t.Fatalf("%s wire schema still carries a constraint keyword", name)
			}
		}
	}
	format := providerResponseFormatForSchema("cognitive_assessment", "schedule_response", scheduleResponseSchema())
	item := mapValue(mapValue(mapValue(mapValue(mapValue(format["json_schema"])["schema"])["properties"])["items"])["items"])
	if !containsSchemaRequired(item, "location") {
		t.Fatalf("schedule_response wire item does not require location: %#v", item["required"])
	}
	sourceItem := mapValue(mapValue(mapValue(scheduleResponseSchema()["properties"])["items"])["items"])
	if containsSchemaRequired(sourceItem, "location") {
		t.Fatal("source schedule schema was mutated; server validation must keep the original contract")
	}
	stage := capabilitycontract.StrictModelSchema(cognitiveStageSchema())
	if stage["additionalProperties"] == true || strings.Contains(jsonString(stage), `"additionalProperties":true`) {
		t.Fatalf("cognitive stage wire schema is still open: %#v", stage)
	}
	registry, err := NewCapabilityRegistry(builtinCapabilities(&App{})...)
	if err != nil {
		t.Fatal(err)
	}
	rendered := RenderCapabilityTools(registry.Definitions())
	if len(rendered) == 0 {
		t.Fatal("builtin capabilities did not render")
	}
	for _, tool := range rendered {
		function := mapValue(tool["function"])
		name := stringValue(function["name"])
		if strings.Contains(name, ".") {
			t.Fatalf("wire tool name %q still contains a dot", name)
		}
		assertStrictWireSchema(t, "tool:"+name, mapValue(function["parameters"]))
	}
	calls, err := NormalizeProviderToolCalls([]any{map[string]any{
		"id": "call-wire", "name": "conversation_reply", "arguments": map[string]any{"text": "hello", "topic_key": nil},
	}}, "fact", "provider")
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].CapabilityName != conversationReplyCapabilityName {
		t.Fatalf("wire tool name did not reverse-map: %#v", calls)
	}
	if _, present := decodeObject(calls[0].Arguments)["topic_key"]; present {
		t.Fatalf("null tool argument was kept: %s", calls[0].Arguments)
	}
	dotted, err := NormalizeProviderToolCalls([]any{map[string]any{
		"id": "call-dotted", "name": "media.image.generate", "arguments": map[string]any{"intent": "a cat"},
	}}, "fact", "provider")
	if err != nil || len(dotted) != 1 || dotted[0].CapabilityName != "media.image.generate" {
		t.Fatalf("canonical dotted tool name changed: %v %#v", err, dotted)
	}
	prepared := prepareModelValue(map[string]any{
		"action_type": "reply", "response_intent": nil, "influences": []any{},
	}, cognitiveTurnResponseSchema())
	if _, present := prepared.(map[string]any)["response_intent"]; present {
		t.Fatalf("null structured field was kept: %#v", prepared)
	}
}

func assertStrictWireSchema(t *testing.T, path string, schema map[string]any) {
	t.Helper()
	for _, key := range []string{"minLength", "maxLength", "minimum", "maximum", "maxItems", "minItems", "pattern", "format", "default"} {
		if _, ok := schema[key]; ok {
			t.Fatalf("%s still has %s", path, key)
		}
	}
	if schema["additionalProperties"] == true {
		t.Fatalf("%s allows additional properties", path)
	}
	properties := mapValue(schema["properties"])
	if len(properties) > 0 {
		if schema["additionalProperties"] != false {
			t.Fatalf("%s additionalProperties = %#v", path, schema["additionalProperties"])
		}
		required := map[string]struct{}{}
		for _, raw := range arrayValue(schema["required"]) {
			required[stringValue(raw)] = struct{}{}
		}
		if len(required) != len(properties) {
			t.Fatalf("%s required %d of %d properties: %#v", path, len(required), len(properties), schema["required"])
		}
		for key, raw := range properties {
			if _, ok := required[key]; !ok {
				t.Fatalf("%s omits required key %s", path, key)
			}
			assertStrictWireSchema(t, path+"."+key, mapValue(raw))
		}
	}
	if items := mapValue(schema["items"]); len(items) > 0 {
		assertStrictWireSchema(t, path+"[]", items)
	}
	for _, keyword := range []string{"anyOf", "oneOf", "allOf"} {
		for index, raw := range arrayValue(schema[keyword]) {
			assertStrictWireSchema(t, path+"."+keyword, mapValue(raw))
			if index > 8 {
				break
			}
		}
	}
}

func nestedKeyword(schema map[string]any, keyword string) bool {
	if _, ok := schema[keyword]; ok {
		return true
	}
	for _, raw := range mapValue(schema["properties"]) {
		if nestedKeyword(mapValue(raw), keyword) {
			return true
		}
	}
	if items := mapValue(schema["items"]); len(items) > 0 && nestedKeyword(items, keyword) {
		return true
	}
	for _, keywordName := range []string{"anyOf", "oneOf", "allOf"} {
		for _, raw := range arrayValue(schema[keywordName]) {
			if nestedKeyword(mapValue(raw), keyword) {
				return true
			}
		}
	}
	return false
}
