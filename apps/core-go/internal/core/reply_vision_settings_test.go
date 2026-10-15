package core

import (
	"context"
	"testing"
)

func TestReplyVisionSettingsAreIndependentAndLegacyByDefault(t *testing.T) {
	reply, err := decodeConversationReplySettings([]byte(`{}`))
	if err != nil || reply.Mode != "legacy" || reply.Concurrency != 1 || reply.FailurePolicy != "error" {
		t.Fatalf("unsafe defaults: %#v %v", reply, err)
	}
	vision, err := decodeVisionUnderstandingSettings([]byte(`{}`))
	if err != nil || vision.Enabled {
		t.Fatalf("vision enabled by default: %#v %v", vision, err)
	}
	reply, err = decodeConversationReplySettings([]byte(`{"mode":"fact_only","temperature":0,"retry_limit":0}`))
	if err != nil || reply.Temperature != 0 || reply.RetryLimit != 0 || vision.Enabled {
		t.Fatalf("explicit zero/default separation lost: %#v %v", reply, err)
	}
	for _, raw := range []string{`null`, `[]`, `{"direction":"wait"}`, `{"mode":"guided"}`, `{"top_p":0}`, `{"retry_limit":99}`, `{} {}`} {
		if _, err := decodeConversationReplySettings([]byte(raw)); err == nil {
			t.Errorf("accepted invalid reply config %s", raw)
		}
	}
	for _, raw := range []string{`null`, `{"enabled":true,"image_generation":false}`, `{"model_ref":"generic_llm"}`, `{"enabled":"false"}`} {
		if _, err := decodeVisionUnderstandingSettings([]byte(raw)); err == nil {
			t.Errorf("accepted invalid vision config %s", raw)
		}
	}
}

func TestIndependentBindingsKeepSharedGeneratedQueue(t *testing.T) {
	for _, role := range []string{conversationReplyModelRole, visionUnderstandingModelRole} {
		if providerBindingRole(role) != role || providerQueueBindingRole(role) != "generic_llm" || !validProviderRole(role) {
			t.Errorf("binding/capacity conflated for %s", role)
		}
	}
	if providerBindingRole("cognitive_assessment") != "generic_llm" || providerQueueBindingRole("embedding") != "embedding" {
		t.Fatal("existing roles changed")
	}
}

func TestFrozenProviderAssignmentDoesNotResolveNewSettings(t *testing.T) {
	original := providerAssignment{Role: conversationReplyModelRole, ModelID: "cydonia-snapshot", EndpointID: "reply-local"}
	ctx := withFrozenProviderAssignment(context.Background(), conversationReplyModelRole, original)
	original.ModelID = "changed"
	// A nil database proves the frozen path performs no role lookup.
	got, err := (&ProviderClient{}).assignment(ctx, conversationReplyModelRole)
	if err != nil || got.ModelID != "cydonia-snapshot" {
		t.Fatalf("snapshot drifted: %#v %v", got, err)
	}
	if _, ok := providerAssignmentSnapshot(ctx, "cognitive_assessment"); ok {
		t.Fatal("reply snapshot overrode parent Qwen")
	}
}
