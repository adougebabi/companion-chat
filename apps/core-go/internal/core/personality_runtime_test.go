package core

import "testing"

func TestInitialPersonalityProfileIDUsesDeclaredActiveProfile(t *testing.T) {
	persona := map[string]any{
		"personality_system": map[string]any{
			"active_profile_id": "base",
			"profiles": []any{
				map[string]any{"id": "base"},
				map[string]any{"id": "guarded"},
			},
		},
	}
	if got := initialPersonalityProfileID(persona); got != "base" {
		t.Fatalf("initial profile = %q, want base", got)
	}
}

func TestInitialPersonalityProfileIDFallsBackToFirstProfile(t *testing.T) {
	persona := map[string]any{
		"personality_system": map[string]any{
			"profiles": []any{
				map[string]any{"id": "warm"},
				map[string]any{"id": "guarded"},
			},
		},
	}
	if got := initialPersonalityProfileID(persona); got != "warm" {
		t.Fatalf("initial profile = %q, want warm", got)
	}
}

func TestInitializationDefaultProfileRemainsValidForMultiProfileSeeds(t *testing.T) {
	persona := map[string]any{
		"personality_system": map[string]any{
			"mode":              "multiple",
			"active_profile_id": "default",
			"profiles": []any{
				map[string]any{"id": "profile_jinghai"},
				map[string]any{"id": "profile_liuhuo"},
			},
		},
	}

	profileIDs := personalityProfileIDs(persona)
	seedProfileID, _ := normalizeProfileID("", initialPersonalityProfileID(persona))
	if _, ok := profileIDs[seedProfileID]; !ok {
		t.Fatalf("normalized seed profile %q is not valid for multi-profile initialization: %#v", seedProfileID, profileIDs)
	}
}

func TestPersistentSwitchRuleIDAcceptsRawAndCanonicalForms(t *testing.T) {
	cases := []struct {
		trigger  string
		declared string
		want     bool
	}{
		{trigger: "safety", declared: "safety", want: true},
		{trigger: "switch:safety", declared: "safety", want: true},
		{trigger: " switch:safety ", declared: " safety ", want: true},
		{trigger: "takeover:safety", declared: "safety", want: false},
		{trigger: "switch:other", declared: "safety", want: false},
	}
	for _, testCase := range cases {
		if got := persistentSwitchRuleIDMatches(testCase.trigger, testCase.declared); got != testCase.want {
			t.Fatalf("persistentSwitchRuleIDMatches(%q, %q) = %t, want %t", testCase.trigger, testCase.declared, got, testCase.want)
		}
	}
}
