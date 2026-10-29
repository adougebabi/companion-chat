package core

import "testing"

func TestR18WardrobeRecoveryRemainsDomainOwned(t *testing.T) {
	value := workflowRecovery("wardrobe.initialize", map[string]any{"fluctlight_id": "r18-role", "task_intent_id": "original-task"})
	if value["href"] != "/c/r18-role/life/wardrobe" || value["mode"] != "domain_recovery" {
		t.Fatal(value)
	}
	if actions := workflowControlActions("wardrobe.initialize", WorkflowExecution{Status: "failed"}); len(actions) != 0 {
		t.Fatal("a domain recovery must not create generic restart/reset actions", actions)
	}
	missing := workflowRecovery("wardrobe.initialize", map[string]any{"task_intent_id": "original-task"})
	if missing["href"] != "/diag/task/original-task" || missing["mode"] != "original_source" {
		t.Fatal("missing scope must retain the original task", missing)
	}
}
