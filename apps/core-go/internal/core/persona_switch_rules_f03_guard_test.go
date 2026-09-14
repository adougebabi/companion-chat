package core

import (
	"os"
	"strings"
	"testing"
)

// TestPersonaSwitchRulesProductionPathHasNoNaturalLanguageSemanticInference is
// the architecture guard for F-03. The persona switch normalizer is the only
// place that can turn a declared activation into an executable takeover, so it
// must never classify natural-language semantics by keyword lists, regex
// pattern matching against prose, or substring heuristics. Only typed, declared
// `kind` fields may resolve a rule's kind; prose is reported as unclassified.
//
// This guard fails the moment one of the removed heuristic primitives is
// reintroduced, so a future change cannot quietly bring keyword/regex semantic
// inference back onto the production path.
func TestPersonaSwitchRulesProductionPathHasNoNaturalLanguageSemanticInference(t *testing.T) {
	source, err := os.ReadFile("persona_switch_rules.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)

	// Removed heuristic primitives. Each one used to assign a switch semantic
	// to prose; none of them may return to the production path.
	for _, forbidden := range []string{
		"personaTakeoverMarkers",
		"personaPersistentMarkers",
		"personaTimeWindowPattern",
		"personaTimeWindowKeys",
		"personaContainsMarker",
		"classifyForcedActivationKind",
		"classifyPersistentRuleKind",
		"personaRuleCarriesTimeWindow",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("persona_switch_rules.go must not reintroduce the heuristic semantic primitive %q (F-03)", forbidden)
		}
	}

	// No keyword-marker slice may be re-added under any name. A marker slice is
	// the defining shape of keyword classification, regardless of what it is
	// named, so the guard matches the var ... Markers = []string{ pattern.
	if strings.Contains(body, "Markers = []string{") || strings.Contains(body, "Markers  = []string{") {
		t.Fatal("persona_switch_rules.go must not declare a keyword-marker slice (F-03): semantic kind is resolved only from a declared `kind` field")
	}

	// The only regexp patterns the production path may keep are the clause
	// splitter and the bullet trim, which perform structural parsing for
	// diagnostics. A semantic-inference regex (matching takeover/persistent
	// markers or time-window prose to assign a kind) is forbidden.
	for _, allowed := range []string{"personaForcedActivationClauseSplitPattern", "personaForcedActivationBulletTrimPattern"} {
		if !strings.Contains(body, allowed) {
			t.Fatalf("persona_switch_rules.go lost the structural %q helper", allowed)
		}
	}

	// The single sanctioned kind resolver is declaredSwitchRuleKind: it reads
	// an explicit, declared `kind` and validates it against the enum. The
	// normalizer must never branch on prose text to derive a kind.
	if !strings.Contains(body, "func declaredSwitchRuleKind(") {
		t.Fatal("persona_switch_rules.go must resolve rule kind only through declaredSwitchRuleKind (F-03)")
	}
}
