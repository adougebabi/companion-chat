package browser

import "testing"

func TestModelRunProjectionKeepsStoredParentAndIndependentBinding(t *testing.T) {
	row := map[string]any{"id": "reply-call", "role": "conversation_reply", "binding_role": "conversation_reply", "logical_run_id": "reply:child", "parent_run_id": "turn:original", "correlation_id": "reply:child"}
	view := browserDiagnosticModelRun(row)
	if view["parentRunId"] != "turn:original" || view["logicalRunId"] != "reply:child" || view["bindingRole"] != "conversation_reply" {
		t.Fatal("Lost independent reply provenance", view)
	}
	if browserDiagnosticModelRun(map[string]any{"correlation_id": "turn:one"})["parentRunId"] != "" {
		t.Fatal("Missing parent was invented from correlation")
	}
}
