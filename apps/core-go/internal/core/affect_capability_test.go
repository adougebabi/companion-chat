package core

import (
	"encoding/json"
	"testing"
)

func TestNormalizeAffectEventCallRequiresEvidenceAndKnownType(t *testing.T) {
	call := ToolCallV1{ID: "affect-1", Arguments: json.RawMessage(`{"event":{"type":"embarrassed","confidence":0.8,"evidence_refs":["current_message.content"],"idempotency_key":"affect-1"}}`)}
	event, err := normalizeAffectEventCall(call, "fact-1")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "embarrassed" || event.Confidence != 0.8 || !containsStringValue(event.EvidenceRefs, "fact-1") {
		t.Fatalf("normalized affect event = %#v", event)
	}

	call.Arguments = json.RawMessage(`{"event":{"type":"unknown","confidence":0.8,"evidence_refs":["fact"],"idempotency_key":"affect-2"}}`)
	if _, err := normalizeAffectEventCall(call, "fact-1"); err == nil {
		t.Fatal("unknown affect type should be rejected")
	}
}

func TestApplyAffectDeltasSetsLabelAndKeepsRawPADServerOwned(t *testing.T) {
	current := map[string]any{
		"pad":      map[string]any{"pleasure": 0.5, "arousal": 0.5, "dominance": 0.5},
		"mood":     map[string]any{"label": "平静", "intensity": 0.0},
		"revision": 3,
	}
	result, requested, applied, label, err := applyAffectDeltas(current, normalizedAffectEvent{Type: "embarrassed", Confidence: 1})
	if err != nil {
		t.Fatal(err)
	}
	if label != "害羞" || stringValue(mapValue(result["mood"])["label"]) != "害羞" {
		t.Fatalf("affect label = %#v", result["mood"])
	}
	if requested["pad.arousal"] == nil || applied["pad.arousal"] == nil {
		t.Fatalf("affect deltas missing: requested=%#v applied=%#v", requested, applied)
	}
	if result["revision"] != 4 {
		t.Fatalf("affect revision = %#v", result["revision"])
	}
}

func TestDefaultInnerStateStartsWithNeutralMoodLabel(t *testing.T) {
	_, mood, _, _, _, _ := defaultInnerState()
	if stringValue(mood["label"]) != "平静" || stringValue(mood["source"]) != "server_default" {
		t.Fatalf("default mood = %#v", mood)
	}
}
