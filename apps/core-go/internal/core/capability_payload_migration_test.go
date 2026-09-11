package core

import (
	"testing"
)

func TestMigrateCapabilityPayloadPreservesStableIdentityAndAddsCanonicalInvocation(t *testing.T) {
	payload := map[string]any{
		"decision": map[string]any{
			"tool_calls": []any{map[string]any{
				"id": "call-1", "name": "scene_event", "arguments": map[string]any{"operation": "switch"},
				"source_fact_id": "fact-1", "provider_request_id": "provider-1", "sequence": 3,
			}},
			"context_projection": map[string]any{"fluctlight_id": "fl-1", "inner_state": map[string]any{"revision": 4}, "life_context": map[string]any{"scene": "studio"}, "schedule": map[string]any{"revision": 2}},
		},
	}
	migrated, err := MigrateCapabilityPayload(payload, true)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if migrated["capability_runtime_version"] != CapabilityRuntimePayloadVersion {
		t.Fatalf("version = %#v", migrated["capability_runtime_version"])
	}
	invocations, parseErr := capabilityInvocationsFromValue(migrated["capability_invocations"])
	if parseErr != nil || len(invocations) != 1 || invocations[0].CallID != "call-1" || invocations[0].ProviderRequestID != "provider-1" || invocations[0].Metadata.FluctlightID != "fl-1" {
		t.Fatalf("invocations = %#v err=%v", invocations, parseErr)
	}
	if len(mapValue(migrated["capability_context_snapshot"])) == 0 {
		t.Fatal("context snapshot missing")
	}
	if snapshot := mapValue(migrated["capability_context_snapshot"]); len(mapValue(snapshot["identity"])) == 0 || snapshot["current_user_text"] != nil || snapshot["recent_messages"] != nil {
		t.Fatalf("context projection was not reduced to a bounded slot snapshot: %#v", snapshot)
	}
	if _, exists := mapValue(migrated["decision"])["tool_calls"]; exists {
		t.Fatalf("legacy decision tool_calls retained: %#v", migrated)
	}
	if _, exists := migrated["tool_calls"]; exists {
		t.Fatalf("legacy root tool_calls retained: %#v", migrated)
	}
	canonical, err := capabilityInvocationsFromValue(migrated["capability_invocations"])
	if err != nil || len(canonical) != 1 || canonical[0].CallID != "call-1" || canonical[0].CapabilityName != "scene_event" {
		t.Fatalf("canonical invocations = %#v err=%v", canonical, err)
	}
}

func TestMigrateCapabilityPayloadLeavesCompletedAuditPayloadUntouched(t *testing.T) {
	payload := map[string]any{"tool_calls": []any{map[string]any{"id": "call-1", "name": "scene_event", "arguments": map[string]any{}}}}
	migrated, err := MigrateCapabilityPayload(payload, false)
	if err != nil {
		t.Fatalf("migrate completed payload = %v", err)
	}
	if _, ok := migrated["capability_invocations"]; ok {
		t.Fatalf("completed payload was rewritten: %#v", migrated)
	}
}

func TestMigrateCapabilityPayloadFailsClosedForMalformedActiveCall(t *testing.T) {
	_, err := MigrateCapabilityPayload(map[string]any{"tool_calls": []any{map[string]any{"name": "scene_event", "arguments": []any{}}}}, true)
	if err == nil {
		t.Fatal("expected malformed active payload error")
	}
}

func TestMigrateCapabilityPayloadPreservesCanonicalInvocationProvenance(t *testing.T) {
	payload := map[string]any{
		"capability_invocations": []any{map[string]any{
			"call_id": "call-1", "capability_name": "scene_event", "arguments": map[string]any{"operation": "end", "confidence": 0.8},
			"source_fact_id": "fact-1", "action_id": "action-1", "provider_request_id": "provider-1", "sequence": 4,
			"schema_version":   CapabilityInvocationSchemaVersion,
			"metadata":         map[string]any{"surface": "wake_up", "fluctlight_id": "fl-1", "conversation_id": "conv-1"},
			"context_snapshot": map[string]any{"identity": map[string]any{"fluctlight_id": "fl-1"}, "current_life": map[string]any{"scene": "studio"}, "schedule": map[string]any{"revision": 2}},
		}},
	}
	migrated, err := MigrateCapabilityPayload(payload, true)
	if err != nil {
		t.Fatal(err)
	}
	invocations, err := capabilityInvocationsFromValue(migrated["capability_invocations"])
	if err != nil || len(invocations) != 1 {
		t.Fatalf("invocations=%#v err=%v", invocations, err)
	}
	invocation := invocations[0]
	if invocation.Metadata.Surface != CapabilitySurfaceWakeUp || invocation.ActionID != "action-1" || stringValue(mapValue(invocation.ContextSnapshot["current_life"])["scene"]) != "studio" {
		t.Fatalf("provenance lost: %#v", invocation)
	}
}

func TestMigrateCapabilityPayloadRejectsDualActiveAuthorities(t *testing.T) {
	_, err := MigrateCapabilityPayload(map[string]any{
		"capability_invocations": []any{},
		"tool_calls":             []any{},
	}, true)
	if err == nil {
		t.Fatal("active payload with two invocation authorities must fail closed")
	}
}

func TestMigrateCapabilityPayloadSeparatesProviderArgumentsAndPreparedData(t *testing.T) {
	payload := map[string]any{
		"fluctlight_id": "fl-1", "source_fact_id": "fact-1", "provider_request_id": "provider-1",
		"capability_context_snapshot": map[string]any{
			"identity":        map[string]any{"fluctlight_id": "fl-1", "source_fact_id": "fact-1"},
			"visual_identity": map[string]any{"status": "active"}, "current_life": map[string]any{"scene": "studio"},
			"appearance": map[string]any{"outfit": "coat"}, "current_state": map[string]any{"mood": map[string]any{"label": "calm"}},
		},
		"tool_calls": []any{map[string]any{
			"id": "image-1", "name": "media.image.generate", "source_fact_id": "fact-1", "provider_request_id": "provider-1",
			"arguments": map[string]any{"intent": "portrait", "prepared_concept": map[string]any{"intent": "portrait", "context_binding": map[string]any{"visual_identity": map[string]any{"status": "active"}, "current_life": map[string]any{"scene": "studio"}, "appearance": map[string]any{"outfit": "coat"}, "current_state": map[string]any{"mood": map[string]any{"label": "calm"}}}}},
		}},
	}
	migrated, err := MigrateCapabilityPayload(payload, true)
	if err != nil {
		t.Fatal(err)
	}
	invocations, err := capabilityInvocationsFromValue(migrated["capability_invocations"])
	if err != nil || len(invocations) != 1 {
		t.Fatalf("invocations=%#v err=%v", invocations, err)
	}
	invocation := invocations[0]
	if string(invocation.Arguments) != `{"intent":"portrait"}` {
		t.Fatalf("provider arguments were not reduced to thin input: %s", invocation.Arguments)
	}
	if raw, found, err := capabilityPreparedData(invocation, "media_concept"); err != nil || !found || stringValue(mapValue(raw)["intent"]) != "portrait" {
		t.Fatalf("prepared media concept missing: raw=%#v found=%v err=%v", raw, found, err)
	}
}

func TestMigrateCapabilityPayloadConvertsLegacyResults(t *testing.T) {
	payload := map[string]any{
		"source_fact_id": "fact-1", "provider_request_id": "provider-1",
		"tool_calls":   []any{map[string]any{"id": "reply-1", "name": "conversation.reply", "arguments": map[string]any{"text": "hello"}, "source_fact_id": "fact-1", "provider_request_id": "provider-1"}},
		"tool_results": []any{map[string]any{"tool_call_id": "reply-1", "name": "conversation.reply", "status": "completed", "output": map[string]any{"text": "hello"}, "retryable": false}},
	}
	migrated, err := MigrateCapabilityPayload(payload, true)
	if err != nil {
		t.Fatal(err)
	}
	results, err := capabilityResultsFromValue(migrated["capability_results"])
	if err != nil || len(results) != 1 || results[0].CallID != "reply-1" || results[0].Status != "completed" {
		t.Fatalf("results=%#v err=%v", results, err)
	}
	if _, retained := migrated["tool_results"]; retained {
		t.Fatalf("legacy results retained: %#v", migrated)
	}
}

func TestMigrateCapabilityPayloadRejectsWrongVersionAndMissingRequiredSnapshot(t *testing.T) {
	if _, err := MigrateCapabilityPayload(map[string]any{"capability_runtime_version": "broken", "capability_invocations": []any{}, "capability_results": []any{}}, true); err == nil {
		t.Fatal("wrong runtime version must fail closed")
	}
	_, err := MigrateCapabilityPayload(map[string]any{
		"source_fact_id": "fact-1", "provider_request_id": "provider-1",
		"tool_calls": []any{map[string]any{"id": "scene-1", "name": "scene_event", "arguments": map[string]any{"operation": "end", "confidence": 0.8}, "source_fact_id": "fact-1", "provider_request_id": "provider-1"}},
	}, true)
	if err == nil {
		t.Fatal("contextful active capability without a bounded snapshot must fail closed")
	}
	_, err = MigrateCapabilityPayload(map[string]any{
		"fluctlight_id": "fl-1", "source_fact_id": "fact-1", "provider_request_id": "provider-1",
		"capability_context_snapshot": map[string]any{
			"identity": map[string]any{"fluctlight_id": "fl-1"}, "visual_identity": map[string]any{"status": "active"},
			"current_life": map[string]any{"scene": "studio"}, "appearance": map[string]any{"outfit": "coat"}, "current_state": map[string]any{"mood": map[string]any{"label": "calm"}},
		},
		"tool_calls": []any{map[string]any{"id": "image-1", "name": "media.image.generate", "arguments": map[string]any{"intent": "portrait", "prepared_concept": map[string]any{"intent": "forged", "context_binding": map[string]any{}}}, "source_fact_id": "fact-1", "provider_request_id": "provider-1"}},
	}, true)
	if err == nil {
		t.Fatal("corrupt prepared capability data must abort active migration")
	}
}

func TestMigrationProducesValidThinArgumentsForEveryBuiltIn(t *testing.T) {
	registry := mustCapabilityRegistry(
		conversationReplyCapability{}, momentPublishCapability{}, imageGenerateCapability{}, visualIdentityInitializeCapability{},
		sceneEventCapability{}, presenceEventCapability{}, scheduleReplanCapability{}, memoryEventCapability{}, affectEventCapability{},
		relationshipLookupCapability{}, capabilityRequestCapability{},
	)
	legacy := map[string]map[string]any{
		"conversation.reply":         {"text": "hello"},
		"moment.publish":             {"text": "moment"},
		"media.image.generate":       {"intent": "portrait", "prepared_concept": map[string]any{"intent": "portrait", "context_binding": map[string]any{"visual_identity": map[string]any{"status": "active"}, "current_life": map[string]any{"scene": "studio"}, "appearance": map[string]any{"outfit": "coat"}, "current_state": map[string]any{"mood": map[string]any{"label": "calm"}}}}},
		"visual_identity.initialize": {"reason": "legacy"},
		"scene_event":                {"operation": "end", "confidence": 0.8, "evidence_refs": []any{"fact-1"}, "idempotency_key": "scene-1"},
		"presence_event":             {"user_presence": "available", "confidence": 0.8, "evidence_refs": []any{"fact-1"}, "idempotency_key": "presence-1"},
		"schedule.replan":            {"intent": "move reading"},
		"memory_event":               {"operation": "record", "content": "memory", "type": "episodic", "confidence": 0.8, "importance": 0.6, "visibility": "private", "personality_perspectives": []any{}, "evidence_refs": []any{"fact-1"}, "idempotency_key": "memory-1"},
		"affect_event":               {"event": map[string]any{"type": "calm", "confidence": 0.8, "evidence_refs": []any{"fact-1"}, "idempotency_key": "affect-1"}},
		"relationship.lookup":        {"target_actor_id": "actor_user"},
		"capability.request":         {"capability_key": "calendar.read", "title": "Calendar", "description": "Read calendar", "rationale": "Need current schedule", "evidence_refs": []any{"fact-1"}, "idempotency_key": "request-1", "side_effect_class": "read_only"},
	}
	for _, definition := range registry.Definitions() {
		arguments, prepared, err := migrateCapabilityCallPayload(definition.Name, legacy[definition.Name], nil, "fact-1", "call-1")
		if err != nil {
			t.Fatalf("%s migration failed: %v", definition.Name, err)
		}
		invocation := CapabilityInvocation{CallID: "call-1", CapabilityName: definition.Name, Arguments: arguments, PreparedPayload: prepared, SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
		if err := invocation.Validate(definition); err != nil {
			t.Fatalf("%s migrated invocation is invalid: %v arguments=%s prepared=%s", definition.Name, err, arguments, prepared)
		}
	}
}
