package core

import "testing"

func TestNormalizeRelationshipRole(t *testing.T) {
	role, err := normalizeRelationshipRole(map[string]any{"label": "准恋人/暧昧对象", "addressing": map[string]any{"preferred": "你", "self_reference": "夏希/希希"}})
	if err != nil {
		t.Fatalf("normalize role: %v", err)
	}
	addressing := mapValue(role["addressing"])
	if role["label"] != "准恋人/暧昧对象" || stringValue(addressing["preferred"]) != "你" || stringValue(addressing["self_reference"]) != "夏希/希希" {
		t.Fatalf("normalized role = %#v", role)
	}
}

func TestNormalizeRelationshipRoleAcceptsUnclassifiedLabel(t *testing.T) {
	role, err := normalizeRelationshipRole(map[string]any{"primary": "made_up"})
	if err != nil || role["label"] != "made_up" {
		t.Fatalf("unclassified relationship role = %#v, err=%v", role, err)
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
