package core

import (
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

func TestNormalizeWakeUpAssessmentCanonicalizesMomentPublishAlias(t *testing.T) {
	value, err := normalizeWakeUpAssessment(map[string]any{
		"action_type":     "moment_publish",
		"response_intent": "发布一条简短动态",
	})
	if err != nil {
		t.Fatal(err)
	}
	if value["action_type"] != "moment" {
		t.Fatalf("action type = %#v, want moment", value["action_type"])
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

func TestNormalizeWakeUpAssessmentRejectsMessageAliases(t *testing.T) {
	for _, actionType := range []string{"message", "reply", "respond", "send_message", "publish_moment"} {
		if _, err := normalizeWakeUpAssessment(map[string]any{"action_type": actionType}); err == nil {
			t.Fatalf("WakeUp accepted non-canonical action_type %q", actionType)
		}
	}
}

func TestWakeUpCommittedActionTypeUsesAuthoritativeReceipts(t *testing.T) {
	tests := []struct {
		name    string
		results []CapabilityResult
		want    string
	}{
		{name: "declared proactive without receipt", want: "no_op"},
		{name: "failed reply", results: []CapabilityResult{{CapabilityName: conversationReplyCapabilityName, Status: "failed"}}, want: "no_op"},
		{name: "completed reply missing target", results: []CapabilityResult{{CapabilityName: conversationReplyCapabilityName, Status: "completed", Output: map[string]any{"target_kind": "conversation_message"}}}, want: "no_op"},
		{name: "completed reply", results: []CapabilityResult{{CapabilityName: conversationReplyCapabilityName, Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "message-1"}}}, want: "proactive_message"},
		{name: "suppressed duplicate is not a publication", results: []CapabilityResult{{CapabilityName: conversationReplyCapabilityName, Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "old-message", "delivery_status": "duplicate_suppressed"}}}, want: "no_op"},
		{name: "accepted moment is not published", results: []CapabilityResult{{CapabilityName: "moment.publish", Status: "accepted", Output: map[string]any{"target_kind": "moment", "target_ref": "moment-1"}}}, want: "no_op"},
		{name: "completed moment", results: []CapabilityResult{{CapabilityName: "moment.publish", Status: "completed", Output: map[string]any{"target_kind": "moment", "target_ref": "moment-1"}}}, want: "moment"},
		{name: "query only is capability not message", results: []CapabilityResult{{CapabilityName: "wardrobe.inspect", Status: "completed", Output: map[string]any{"items": []any{}}}}, want: "capability"},
		{name: "accepted durable capability", results: []CapabilityResult{{CapabilityName: "media.image.generate", Status: "accepted", Output: map[string]any{"media_intent_id": "media-1"}}}, want: "capability"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := wakeUpCommittedActionType(agentCommittedOutcome{Results: test.results}); got != test.want {
				t.Fatalf("wakeUpCommittedActionType() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWakeUpSuppressedReplyCannotResetIdleEpoch(t *testing.T) {
	suppressed := CapabilityResult{CapabilityName: conversationReplyCapabilityName, Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "old-message", "delivery_status": "duplicate_suppressed"}}
	if replies := wakeUpPublishedReplyResults([]CapabilityResult{suppressed}); len(replies) != 0 {
		t.Fatal("old suppressed message admitted to idle-clock update", replies)
	}
	actual := CapabilityResult{CapabilityName: conversationReplyCapabilityName, Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "new-message"}}
	replies := wakeUpPublishedReplyResults([]CapabilityResult{suppressed, actual})
	if len(replies) != 1 || stringValue(mapValue(replies[0].Output)["target_ref"]) != "new-message" {
		t.Fatal("real publication lost or suppressed publication retained", replies)
	}
}

func TestNormalizeWakeUpSettingsUsesFixedTenMinuteInterval(t *testing.T) {
	settings := normalizeWakeUpSettings(map[string]any{"enabled": false, "interval_seconds": 1})
	if settings.Enabled || settings.IntervalSeconds != minWakeUpIntervalSeconds {
		t.Fatalf("settings = %#v", settings)
	}
	settings = normalizeWakeUpSettings(map[string]any{"interval_seconds": 24 * 60 * 60})
	if settings.IntervalSeconds != defaultWakeUpIntervalSeconds {
		t.Fatalf("maximum settings = %#v", settings)
	}
}
