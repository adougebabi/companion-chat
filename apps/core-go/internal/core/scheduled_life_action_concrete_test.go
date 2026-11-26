package core

import "testing"

func TestConcreteLifeActionIsScheduledWithoutExistingGoal(t *testing.T) {
	for _, kind := range []string{"virtual_shopping", "haircut", "hair_dye"} {
		if !scheduledLifeActionConcrete(map[string]any{"action_plan": map[string]any{"kind": kind}}) {
			t.Fatalf("%s should be scheduled directly", kind)
		}
	}
	for _, args := range []map[string]any{{}, {"action_plan": map[string]any{}}, {"action_plan": map[string]any{"kind": "travel"}}} {
		if scheduledLifeActionConcrete(args) {
			t.Fatalf("vague wish must still go to goal planning: %#v", args)
		}
	}
	if !scheduledLifeActionConcrete(map[string]any{"action_plan": map[string]any{"kind": "virtual_shopping"}}) || len(formalAgentMandatoryTools(FormalAgentConversationCognition)) == 0 || !formalAgentMandatoryTools(FormalAgentConversationCognition)[scheduleActivityCapabilityName] {
		t.Fatal("chat cannot see intention.schedule from the first round")
	}
}
