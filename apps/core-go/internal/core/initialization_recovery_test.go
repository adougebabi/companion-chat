package core

import "testing"

// Source cases for the next user-run backend regression; this change is built,
// but no database/model/backend logic regression is claimed for this session.
func TestInitializationOptionalContainersRecoverWithReviewWarnings(t *testing.T) {
	input := map[string]any{"schema_version": nil, "core_persona": map[string]any{"identity": map[string]any{"name": "回归人物", "timezone": "unknown-zone"}, "personality": "安静、好奇", "personality_system": map[string]any{"profiles": []any{nil, map[string]any{"name": "本人"}}}}, "actor_user": nil}
	prepared, err := prepareInitializationResponse(input)
	if err != nil {
		t.Fatal(err)
	}
	if mapValue(mapValue(prepared["core_persona"])["identity"])["timezone"] != nil {
		t.Fatal("unknown timezone became a fact")
	}
	if len(arrayValue(mapValue(prepared["extensions"])["initialization_warnings"])) == 0 {
		t.Fatal("repairs were hidden")
	}
	persona := mapValue(prepared["core_persona"])
	if mapValue(persona["personality"])["openness"] != nil || mapValue(persona["behavioral_policy"])["response_style"] != nil {
		t.Fatal("unknown trait or expression became a fact")
	}
	if _, err := prepareInitializationResponse(map[string]any{}); err == nil {
		t.Fatal("empty provider output became a pretend analysis")
	}
}

func TestInitializationWarningShapeAndOriginalRuleArePreserved(t *testing.T) {
	rule := map[string]any{"kind": "custom", "version": "draft", "target_profile_id": "unknown"}
	input := map[string]any{"core_persona": map[string]any{"personality_system": map[string]any{"profiles": []any{map[string]any{"id": "one", "name": "本人"}, map[string]any{"id": "two", "name": "另一人格"}}, "takeover_rules": []any{rule}}}, "extensions": map[string]any{"initialization_warnings": []any{nil}}}
	prepared, err := prepareInitializationResponse(input)
	if err != nil {
		t.Fatal(err)
	}
	if mapValue(mapValue(prepared["core_persona"])["identity"])["timezone"] != nil {
		t.Fatal("missing identity supplied a timezone")
	}
	extensions := mapValue(prepared["extensions"])
	for _, entry := range arrayValue(extensions["initialization_warnings"]) {
		if stringValue(mapValue(entry)["message"]) == "" {
			t.Fatal("malformed warning reached the preview")
		}
	}
	original := mapValue(arrayValue(extensions["unmatched_takeover_rules"])[0])
	if original["kind"] != "custom" || original["version"] != "draft" || original["condition"] != nil {
		t.Fatalf("unmatched rule was mutated: %#v", original)
	}
}
