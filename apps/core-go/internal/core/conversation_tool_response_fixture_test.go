package core

import (
	"encoding/json"
	"testing"
)

// This fixture mirrors a real cognition response that returned affect_event,
// memory_event, and media.image.generate without conversation.reply. The
// response must settle as an optional no-visible-reply action instead of
// entering cognition_visible_text_missing/conversation_turn_failed.
func TestConversationToolResponseFixtureWithoutReplySettlesAsNoOp(t *testing.T) {
	fixture := map[string]any{
		"structured": map[string]any{},
		"text":       "",
		"tool_calls": []any{
			map[string]any{
				"id": "call_affect", "name": "affect_event", "provider_request_id": "provider:fixture", "schema_version": ToolCallSchemaVersion, "sequence": 0,
				"arguments": map[string]any{"event": map[string]any{"confidence": 0.85, "evidence_refs": []any{"current_message.content", "sequence:20"}, "idempotency_key": "affect_xixi_tilde_excited_0908", "type": "excited"}},
			},
			map[string]any{
				"id": "call_memory", "name": "memory_event", "provider_request_id": "provider:fixture", "schema_version": ToolCallSchemaVersion, "sequence": 1,
				"arguments": map[string]any{"confidence": 0.9, "content": "actor_user承诺绝对不笑并催促林夏希快发工作间半成品照片", "emotional_significance": 0.7, "evidence_refs": []any{"current_message.content"}, "idempotency_key": "mem_send_photo_agreed_0908", "importance": 0.6, "operation": "record", "type": "episodic"},
			},
			map[string]any{
				"id": "call_media", "name": "media.image.generate", "provider_request_id": "provider:fixture", "schema_version": ToolCallSchemaVersion, "sequence": 2,
				"arguments": map[string]any{"concept": map[string]any{"context_override": map[string]any{"explicit": false}, "location": "home workspace, Shanghai", "mood": "shy, excited, a little nervous", "style": "soft realistic, warm lighting", "subject": "young woman wearing a half-finished silver-white cosplay wig"}},
			},
		},
	}
	data, err := json.Marshal(fixture["tool_calls"])
	if err != nil {
		t.Fatal(err)
	}
	var rawCalls []any
	if err := json.Unmarshal(data, &rawCalls); err != nil {
		t.Fatal(err)
	}
	calls, err := NormalizeProviderToolCalls(rawCalls, "inbox_fixture", "provider:fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 {
		t.Fatalf("normalized calls = %#v", calls)
	}
	for _, call := range calls {
		if call.SourceFactID != "inbox_fixture" {
			t.Fatalf("source fact was not normalized: %#v", call)
		}
	}
	manifests := toolManifestMap([]CapabilityManifest{
		conversationReplyCapabilityManifest(), imageCapabilityManifest(), affectEventCapabilityManifest(), memoryCapabilityManifest(),
	})
	if action, _, err := resolveToolCallAction(calls, manifests); err != nil || action != "no_op" {
		t.Fatalf("resolve tool action = %q err=%v", action, err)
	}
	action, _, err := resolveToolCallAction(calls, manifests)
	if err != nil || action != "no_op" {
		t.Fatalf("missing reply action = %q err=%v", action, err)
	}
	decision := map[string]any{"action_type": action, "tool_calls": calls}
	composite, err := normalizeCompositeAction(decision, calls, "inbox_fixture", action)
	if err != nil {
		t.Fatal(err)
	}
	if composite.Kind != "none" || composite.ActionType != "no_op" || len(composite.ToolCalls) != 3 {
		t.Fatalf("composite optional-tool action = %#v", composite)
	}
}
