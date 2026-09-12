package core

import (
	"encoding/json"
	"testing"
)

func TestNormalizeWakeUpAssessmentPreservesOptionalLegacyFields(t *testing.T) {
	value, err := normalizeWakeUpAssessment(map[string]any{
		"appraisal": map[string]any{
			"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.0,
			"social_threat": 0.0, "controllability": 0.5, "responsibility": 0.5,
			"relationship_significance": 0.5, "expected_effect": 0.5,
		},
		"attention":   "我注意到今天的节奏发生了变化",
		"thought":     map[string]any{"summary": "需要重新整理优先级"},
		"desire":      "保持清醒并完成重要的事",
		"agency":      map[string]any{"decision": "先观察"},
		"action_type": "no_op",
	})
	if err != nil {
		t.Fatal(err)
	}
	if value["action_type"] != "no_op" || value["attention"] == nil || value["thought"] == nil {
		t.Fatalf("normalized wake-up = %#v", value)
	}
	if refs, ok := value["evidence_refs"].([]any); !ok || len(refs) != 0 {
		t.Fatalf("evidence refs = %#v", value["evidence_refs"])
	}
}

func TestNormalizeWakeUpAssessmentAcceptsActionOnlyDecision(t *testing.T) {
	value, err := normalizeWakeUpAssessment(map[string]any{
		"action_type":     "moment",
		"response_intent": "发布一条简短动态",
		"evidence_refs":   []any{"life_context.scene"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if value["action_type"] != "moment" || stringValue(value["response_intent"]) == "" {
		t.Fatalf("normalized action-only wake-up = %#v", value)
	}
	if _, ok := value["appraisal"]; ok {
		t.Fatalf("wake-up action decision must not synthesize appraisal: %#v", value)
	}
}

func TestTextFromOutputBindingReadsFinalText(t *testing.T) {
	call := CapabilityInvocation{CapabilityName: "moment.publish", Arguments: json.RawMessage(`{"text":"  今天有点风。  "}`)}
	registry := mustCapabilityRegistry(momentPublishCapability{})
	if got := textFromOutputBinding([]CapabilityInvocation{call}, "moment", registry); got != "今天有点风。" {
		t.Fatalf("output capability text = %q", got)
	}
	if got := textFromOutputBinding([]CapabilityInvocation{call}, "conversation_message", registry); got != "" {
		t.Fatalf("wrong output capability should be empty: %q", got)
	}
}

func TestNormalizeWakeUpAssessmentRejectsInvalidAction(t *testing.T) {
	_, err := normalizeWakeUpAssessment(map[string]any{
		"appraisal": map[string]any{
			"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.0,
			"social_threat": 0.0, "controllability": 0.5, "responsibility": 0.5,
			"relationship_significance": 0.5, "expected_effect": 0.5,
		},
		"attention":   "attention",
		"thought":     "thought",
		"desire":      "desire",
		"agency":      "agency",
		"action_type": "send everything",
	})
	if err == nil {
		t.Fatal("invalid wake-up action should be rejected")
	}
}

func TestNormalizeWakeUpSettingsClampsInterval(t *testing.T) {
	settings := normalizeWakeUpSettings(map[string]any{"enabled": false, "interval_seconds": 1})
	if settings.Enabled || settings.IntervalSeconds != minWakeUpIntervalSeconds {
		t.Fatalf("settings = %#v", settings)
	}
	settings = normalizeWakeUpSettings(map[string]any{"interval_seconds": maxWakeUpIntervalSeconds + 1})
	if settings.IntervalSeconds != maxWakeUpIntervalSeconds {
		t.Fatalf("maximum settings = %#v", settings)
	}
}

func TestWakeUpChatOnlyActionFallsBackToNoOp(t *testing.T) {
	actual, result := fallbackWakeUpActionWithoutCapability("reply")
	if actual != "no_op" {
		t.Fatalf("actual action = %q, want no_op", actual)
	}
	if result["status"] != "no_op" || result["reason"] != "action_requires_capability_call" || result["proposed_action_type"] != "reply" {
		t.Fatalf("fallback result = %#v", result)
	}
}

func TestWakeUpToolOnlyReplyBecomesProactiveMessage(t *testing.T) {
	assessment := wakeUpAssessmentFromToolCalls(testInvocations([]ToolCallV1{
		{ID: "media", Name: "media.image.generate", Arguments: json.RawMessage(`{"intent":"午后的窗边"}`), SourceFactID: "fact", ProviderRequestID: "provider", SchemaVersion: ToolCallSchemaVersion},
		{ID: "reply", Name: "conversation.reply", Arguments: json.RawMessage(`{"text":"在呢。"}`), SourceFactID: "fact", ProviderRequestID: "provider", SchemaVersion: ToolCallSchemaVersion},
	}), mustCapabilityRegistry(imageGenerateCapability{}, conversationReplyCapability{}))
	if assessment == nil || assessment["action_type"] != "proactive_message" {
		t.Fatalf("tool-only wake-up assessment = %#v", assessment)
	}
}

func TestWakeUpToolCallAssessmentMergePreservesInfluences(t *testing.T) {
	registry := mustCapabilityRegistry(conversationReplyCapability{}, affectEventCapability{})
	assessment := map[string]any{"action_type": "no_op", "influences": []any{"keep-me"}}
	merged := mergeWakeUpToolCallAssessment(assessment, testInvocations([]ToolCallV1{
		{ID: "reply", Name: "conversation.reply", Arguments: json.RawMessage(`{"text":"在呢。"}`), SourceFactID: "fact", ProviderRequestID: "provider", SchemaVersion: ToolCallSchemaVersion},
	}), registry)
	if merged["action_type"] != "proactive_message" || stringValue(merged["response_intent"]) == "" {
		t.Fatalf("merged output action = %#v", merged)
	}
	if len(arrayValue(merged["influences"])) != 1 {
		t.Fatalf("merge discarded structured influences = %#v", merged["influences"])
	}
	stateful := mergeWakeUpToolCallAssessment(map[string]any{"action_type": "no_op", "influences": []any{"keep-me"}}, testInvocations([]ToolCallV1{
		{ID: "affect", Name: "affect_event", Arguments: json.RawMessage(`{"event":{"type":"happy","confidence":0.8}}`), SourceFactID: "fact", ProviderRequestID: "provider", SchemaVersion: ToolCallSchemaVersion},
	}), registry)
	if stateful["action_type"] != "no_op" || len(arrayValue(stateful["influences"])) != 1 {
		t.Fatalf("stateful tool assessment was rewritten = %#v", stateful)
	}
}

func TestWakeUpToolOnlyNativeCapabilityRemainsNoOpAction(t *testing.T) {
	assessment := wakeUpAssessmentFromToolCalls(testInvocations([]ToolCallV1{
		{ID: "affect", Name: "affect_event", Arguments: json.RawMessage(`{"event":{"type":"excited","confidence":0.8}}`), SourceFactID: "fact", ProviderRequestID: "provider", SchemaVersion: ToolCallSchemaVersion},
		{ID: "media", Name: "media.image.generate", Arguments: json.RawMessage(`{"intent":"a character"}`), SourceFactID: "fact", ProviderRequestID: "provider", SchemaVersion: ToolCallSchemaVersion},
	}), mustCapabilityRegistry(affectEventCapability{}, imageGenerateCapability{}))
	if assessment == nil || assessment["action_type"] != "no_op" {
		t.Fatalf("tool-only capability assessment = %#v", assessment)
	}
}

func TestWakeUpOutputOnlyCapabilitiesDoNotRequireInfluences(t *testing.T) {
	registry := mustCapabilityRegistry(conversationReplyCapability{}, imageGenerateCapability{})
	outputOnly := []CapabilityInvocation{{CapabilityName: "conversation.reply"}, {CapabilityName: "media.image.generate"}}
	if wakeUpDecisionRequiresInfluences("proactive_message", outputOnly, registry) {
		t.Fatal("output-only wake-up decision should not require an influence sidecar")
	}
	if wakeUpDecisionRequiresInfluences("no_op", outputOnly, registry) {
		t.Fatal("tool-only output wake-up should not require an influence sidecar")
	}
	stateful := []CapabilityInvocation{{CapabilityName: "affect_event"}}
	statefulRegistry := mustCapabilityRegistry(affectEventCapability{}, conversationReplyCapability{})
	if !wakeUpDecisionRequiresInfluences("no_op", stateful, statefulRegistry) {
		t.Fatal("stateful wake-up capability must require an influence sidecar")
	}
}

func TestWakeUpPersistenceValidationDoesNotRunCapabilityPreflight(t *testing.T) {
	app := &App{Capabilities: mustCapabilityRegistry(imageGenerateCapability{})}
	invocation := CapabilityInvocation{
		CallID:            "wake-image",
		CapabilityName:    "media.image.generate",
		SchemaVersion:     CapabilityInvocationSchemaVersion,
		Arguments:         json.RawMessage(`{"intent":"午后的窗边"}`),
		SourceFactID:      "wake_up_fact",
		ProviderRequestID: "provider-wake-image",
		Sequence:          0,
	}
	// imageGenerateCapability has no service in this fixture, so a real
	// runtime.Prepare would fail its media preflight. The WakeUp persistence
	// boundary must still accept the deterministic invocation and let the
	// durable action worker record/retry the eventual preflight result.
	if err := app.validateCapabilityInvocationsForPersistence([]CapabilityInvocation{invocation}); err != nil {
		t.Fatalf("persistence validation unexpectedly ran preflight: %v", err)
	}
}
