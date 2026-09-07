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
