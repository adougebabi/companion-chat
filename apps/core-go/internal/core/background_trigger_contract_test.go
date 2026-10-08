package core

import (
	"testing"
)

func TestSilentLifecycleFactsDoNotBecomeReflectionOrGoalEvidence(t *testing.T) {
	noOp := ActionOutcome{Status: ActionOutcomeCompleted, Expected: map[string]any{"action_type": "no_op"}}
	if goalOutcomeCarriesEvidence(noOp) {
		t.Fatal("silent no-op became Goal evidence")
	}
	inspection := ActionOutcome{Status: ActionOutcomeCompleted, CapabilityName: "goal.inspect", Expected: map[string]any{"action_type": "capability"}}
	if goalOutcomeCarriesEvidence(inspection) {
		t.Fatal("Goal inspection became Goal evidence")
	}
	actorInspection := ActionOutcome{Status: ActionOutcomeCompleted, CapabilityName: "actor.inspect", Expected: map[string]any{"action_type": "capability"}}
	if goalOutcomeCarriesEvidence(actorInspection) {
		t.Fatal("Actor inspection became Goal evidence")
	}
	domainQuery := ActionOutcome{Status: ActionOutcomeCompleted, CapabilityName: "wardrobe.inspect", Expected: map[string]any{"action_type": "capability"}}
	if !goalOutcomeCarriesEvidence(domainQuery) {
		t.Fatal("genuine domain query was discarded")
	}
	domainAction := ActionOutcome{Status: ActionOutcomeCompleted, CapabilityName: "wardrobe.wear", Expected: map[string]any{"action_type": "capability"}}
	if !goalOutcomeCarriesEvidence(domainAction) {
		t.Fatal("genuine domain action was discarded")
	}
	if reflectionFactCarriesRealEvidence("internal.wake_up", map[string]any{}) {
		t.Fatal("periodic WakeUp became Reflection evidence")
	}
	if reflectionFactCarriesRealEvidence("autonomy.result", map[string]any{"outcomes": []any{noOp}}) {
		t.Fatal("silent autonomy result became Reflection evidence")
	}
	if !reflectionFactCarriesRealEvidence("autonomy.result", map[string]any{"outcomes": []any{domainAction}}) {
		t.Fatal("real action outcome was discarded from Reflection")
	}
}
