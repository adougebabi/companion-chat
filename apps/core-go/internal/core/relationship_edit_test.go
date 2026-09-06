package core

import "testing"

func TestNormalizeRelationshipRole(t *testing.T) {
	role, err := normalizeRelationshipRole(map[string]any{"primary": "romantic_partner", "secondary": []any{"trusted_companion", "romantic_partner"}, "label": "伴侣"})
	if err != nil {
		t.Fatalf("normalize role: %v", err)
	}
	if role["primary"] != "romantic_partner" || len(arrayValue(role["secondary"])) != 1 || role["label"] != "伴侣" {
		t.Fatalf("normalized role = %#v", role)
	}
}

func TestNormalizeRelationshipRoleRejectsUnknownCode(t *testing.T) {
	if _, err := normalizeRelationshipRole(map[string]any{"primary": "made_up"}); err == nil {
		t.Fatal("expected unknown relationship role to be rejected")
	}
}

func TestValidateRelationshipMetricsBoundsValues(t *testing.T) {
	if _, err := validateRelationshipMetrics(map[string]any{"trust": 0.8, "intimacy": 0.0}); err != nil {
		t.Fatalf("valid metrics rejected: %v", err)
	}
	if _, err := validateRelationshipMetrics(map[string]any{"trust": 1.1}); err == nil {
		t.Fatal("expected out-of-range metric to be rejected")
	}
}

func TestResolveInitializationActorRefKeepsCanonicalPromptAliases(t *testing.T) {
	if got := resolveInitializationActorRef("actor_user", "human-1", "fl-1"); got != "human-1" {
		t.Fatalf("actor_user resolved to %q", got)
	}
	if got := resolveInitializationActorRef("actor_self", "human-1", "fl-1"); got != "fl-1" {
		t.Fatalf("actor_self resolved to %q", got)
	}
	if got := resolveInitializationActorRef("actor-other", "human-1", "fl-1"); got != "actor-other" {
		t.Fatalf("other actor ref changed to %q", got)
	}
}
