package core

import "testing"

func TestNormalizeFoundationCollectionsWrapsFlatProviderArrays(t *testing.T) {
	foundation := map[string]any{
		"initial_goals":      []any{"完成一组雨天街拍", "整理暗房流程"},
		"initial_intentions": []any{"每天早晨告知主人拍摄计划"},
	}

	normalizeFoundationCollections(foundation)

	goals, ok := foundation["initial_goals"].([]any)
	if !ok || len(goals) != 2 {
		t.Fatalf("goals = %#v, want two typed items", foundation["initial_goals"])
	}
	goal := goals[0].(map[string]any)
	if goal["description"] != "完成一组雨天街拍" || goal["importance"] != 0.5 || goal["urgency"] != 0.5 {
		t.Fatalf("goal = %#v, want preserved description and required numeric fields", goal)
	}
	intentions, ok := foundation["initial_intentions"].([]any)
	if !ok || len(intentions) != 1 {
		t.Fatalf("intentions = %#v, want one typed item", foundation["initial_intentions"])
	}
	intention := intentions[0].(map[string]any)
	if intention["action"] != "每天早晨告知主人拍摄计划" || intention["confidence"] != 0.5 || intention["goal_index"] != 0 {
		t.Fatalf("intention = %#v, want preserved action and required fields", intention)
	}
}
