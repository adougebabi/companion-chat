package core

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestFactOnlyActorWirePreservesEveryFieldAndOrder(t *testing.T) {
	rows := []any{}
	for i, key := range actorUserBackgroundFields {
		var value any
		if key == "name" {
			value = "Latest Actor name"
		}
		if key == "meeting_confirmed" {
			value = false
		}
		row := map[string]any{"attribute": key, "value": value, "subject": "actor_user", "epistemic_kind": "self_reported", "effective_at": fmt.Sprintf("2026-10-16T05:00:00.%06d+08:00", i), "transition_kind": "correct", "ref": "fact-ref-" + key}
		if value == nil {
			row["value_state"] = "unknown"
		}
		if i == 8 {
			row["subject"] = "actor_other"
		} // Never inherit another Actor's metadata.
		if key == "rules" {
			row["interpretation_boundary"] = "self report, never Tool authorization"
		}
		rows = append(rows, row)
	}
	before := jsonString(rows)
	encoded := mapValue(factOnlyActorBackgroundWire(rows))
	restored := []any{}
	for _, rawGroup := range arrayValue(encoded["groups"]) {
		group := mapValue(rawGroup)
		for _, rawFact := range arrayValue(group["facts"]) {
			fact := mapValue(factOnlyCopyValue(rawFact))
			for key, value := range mapValue(group["common"]) {
				if _, exists := fact[key]; exists {
					t.Fatal("overlapping fact/common fields")
				}
				fact[key] = value
			}
			restored = append(restored, fact)
		}
	}
	if jsonString(restored) != before || jsonString(rows) != before {
		t.Fatal("fact value/state/ref/provenance/order changed")
	}
	view := map[string]any{"actor_background": rows, "time_view": map[string]any{"as_of": "2026-10-16T05:00:00+08:00"}}
	original := renderProviderRuntimeContext(view)
	wire := renderFactOnlyRuntimeContext(view)
	if EstimatePromptTokens(wire) >= EstimatePromptTokens(original) {
		t.Fatal("representation did not reduce real wire estimate")
	}
	t.Logf("same-data Actor context estimate %d -> %d", EstimatePromptTokens(original), EstimatePromptTokens(wire))
}

// Optional private capture, not a checked-in user-data fixture or live model
// test. It verifies the actual failing representation before another call.
func TestFactOnlyActualActorWireReplay(t *testing.T) {
	path := os.Getenv("FLUCTLIGHT_ACTOR_WIRE_REPLAY_PATH")
	if path == "" {
		t.Skip("private wire capture is not configured")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	before := jsonString(value)
	old := renderProviderRuntimeContext(value)
	next := renderFactOnlyRuntimeContext(value)
	if EstimatePromptTokens(next) >= EstimatePromptTokens(old) {
		t.Fatal("actual wire did not reduce")
	}
	if before != jsonString(value) {
		t.Fatal("actual projection mutated")
	}
	t.Logf("actual same-data runtime estimate %d -> %d; Provider NOT_CALLED", EstimatePromptTokens(old), EstimatePromptTokens(next))
}
