package core

import (
	"strings"
	"testing"
)

func TestReflectionProviderSchemaClosesActiveMemoryCandidates(t *testing.T) {
	schema := reflectionProposalV2ProviderSchema()
	if !containsSchemaRequired(schema, "active_memory_candidates") {
		t.Fatalf("Reflection schema does not require active_memory_candidates: %#v", schema)
	}
	candidate := mapValue(mapValue(mapValue(schema["properties"])["active_memory_candidates"])["items"])
	if candidate["additionalProperties"] != false {
		t.Fatalf("Active Memory candidate schema is open: %#v", candidate)
	}
	properties := mapValue(candidate["properties"])
	for _, field := range []string{"operation", "target_ref", "kind", "content", "confidence", "importance", "original_time_expression", "valid_from", "valid_until", "time_precision", "evidence_refs", "semantic_reason"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("Active Memory candidate schema missing %q: %#v", field, candidate)
		}
	}
	for _, forbidden := range []string{"active_memory_id", "expected_revision", "owner_fluctlight_id", "conversation_id", "source_fact_id", "timezone", "canonical_key", "request_digest", "idempotency_key"} {
		if _, ok := properties[forbidden]; ok {
			t.Fatalf("Active Memory provider schema exposes %q: %#v", forbidden, candidate)
		}
	}
}

func TestDecodeReflectionProposalV2AcceptsActiveMemoryAndRejectsUnknownFields(t *testing.T) {
	fixture := reflectionProposalV2Fixture(nil)
	fixture["active_memory_candidates"] = []any{map[string]any{
		"operation": "create", "kind": "future_event", "content": "明早七点赶飞机", "confidence": 0.95, "importance": 1.0,
		"original_time_expression": "明早七点", "valid_until": "2026-09-12T07:00:00+08:00", "time_precision": "exact",
		"evidence_refs": []any{"sequence:7"}, "semantic_reason": "明确的未来事件",
	}}
	proposal, err := DecodeReflectionProposalV2(jsonBytes(fixture))
	if err != nil {
		t.Fatalf("DecodeReflectionProposalV2() error = %v", err)
	}
	if len(proposal.ActiveMemoryCandidates) != 1 || proposal.ActiveMemoryCandidates[0].ValidUntil == nil || proposal.ActiveMemoryCandidates[0].TimePrecision != "exact" {
		t.Fatalf("decoded Active Memory candidates = %#v", proposal.ActiveMemoryCandidates)
	}
	fixture["active_memory_candidates"].([]any)[0].(map[string]any)["revision"] = 1
	if _, err := DecodeReflectionProposalV2(jsonBytes(fixture)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown runtime field error = %v", err)
	}
}

func TestConversationSummaryProviderSchemaIsClosedAndSemanticOnly(t *testing.T) {
	schema := conversationSummaryProviderSchema()
	if schema["additionalProperties"] != false || !containsSchemaRequired(schema, "schema_version") || !containsSchemaRequired(schema, "summary") {
		t.Fatalf("Conversation Summary schema is not strict: %#v", schema)
	}
	properties := mapValue(schema["properties"])
	version := mapValue(properties["schema_version"])
	if values := arrayValue(version["enum"]); len(values) != 1 || stringValue(values[0]) != conversationSummarySchemaVersion {
		t.Fatalf("summary schema version = %#v", version)
	}
	summary := mapValue(properties["summary"])
	if intValue(summary["minLength"]) != 1 || intValue(summary["maxLength"]) != conversationSummaryMaxRunes {
		t.Fatalf("summary bounds = %#v", summary)
	}
	for _, forbidden := range []string{"conversation_id", "fluctlight_id", "source_message_id", "source_sequence", "from_sequence", "to_sequence", "projection_revision", "request_digest", "idempotency_key"} {
		if _, ok := properties[forbidden]; ok {
			t.Fatalf("summary Provider schema exposes runtime field %q: %#v", forbidden, properties)
		}
	}
}
