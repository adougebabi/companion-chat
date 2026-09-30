package core

import (
	"encoding/json"
	"testing"
)

func TestParseStandardPersonaJSON(t *testing.T) {
	// Not JSON
	if _, ok := parseStandardPersonaJSON("This is a regular description"); ok {
		t.Fatal("expected non-json text to return false")
	}

	// JSON array
	if _, ok := parseStandardPersonaJSON("[1, 2, 3]"); ok {
		t.Fatal("expected array json to return false")
	}

	// Unrelated JSON object
	if _, ok := parseStandardPersonaJSON(`{"foo": "bar"}`); ok {
		t.Fatal("expected unrelated object json to return false")
	}

	// Standard envelope with core_persona
	standardEnvelope := map[string]any{
		"schema_version": 2,
		"core_persona": map[string]any{
			"schema_version": 1,
			"identity": map[string]any{
				"name": "星见",
			},
			"personality_system": map[string]any{
				"mode": "single",
			},
		},
		"developing_self": map[string]any{
			"claims": []any{},
		},
		"analysis_id":    "old_analysis_id",
		"correlation_id": "old_correlation_id",
	}
	bytes, err := json.Marshal(standardEnvelope)
	if err != nil {
		t.Fatal(err)
	}

	parsed, ok := parseStandardPersonaJSON(string(bytes))
	if !ok {
		t.Fatal("expected standard envelope to be recognized as standard persona JSON")
	}
	if parsed["analysis_id"] != nil || parsed["correlation_id"] != nil {
		t.Fatal("expected old analysis_id and correlation_id to be stripped")
	}
	if parsed["core_persona"] == nil {
		t.Fatal("expected core_persona to be preserved")
	}

	// Root is core_persona itself
	rootCore := map[string]any{
		"identity": map[string]any{
			"name": "月华",
		},
		"personality_system": map[string]any{
			"mode": "single",
		},
	}
	bytesCore, err := json.Marshal(rootCore)
	if err != nil {
		t.Fatal(err)
	}

	parsedCore, ok := parseStandardPersonaJSON(string(bytesCore))
	if !ok {
		t.Fatal("expected root core_persona to be recognized as standard persona JSON")
	}
	if parsedCore["core_persona"] == nil {
		t.Fatal("expected core_persona wrapper to be generated")
	}
}

func TestPrepareInitializationResponseOnStandardJSON(t *testing.T) {
	standard := map[string]any{
		"schema_version": 2,
		"core_persona": map[string]any{
			"identity": map[string]any{
				"name": "云舒",
			},
			"personality_system": map[string]any{
				"mode": "single",
			},
		},
	}
	prepared, err := prepareInitializationResponse(standard)
	if err != nil {
		t.Fatalf("prepareInitializationResponse failed: %v", err)
	}
	if prepared == nil {
		t.Fatal("expected non-nil prepared response")
	}
	persona := mapValue(prepared["core_persona"])
	identity := mapValue(persona["identity"])
	if stringValue(identity["name"]) != "云舒" {
		t.Fatalf("expected name to be 云舒, got %v", identity["name"])
	}
	if prepared["developing_self"] == nil {
		t.Fatal("expected developing_self to be defaulted")
	}
}
