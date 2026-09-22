package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// Each Tool owns its local transaction. Failure to persist the receipt must
// roll back that Tool's domain writes and outbox, even though prior Tools may
// already have committed independently.
func TestIndependentToolReceiptFailureRollsBackMemoryAndAffect(t *testing.T) {
	ctx, repo, app, owner, fluctlight, _ := setupDirectPublicationToolTest(t, "receipt-rollback")
	if _, err := repo.Pool().Exec(ctx, `CREATE FUNCTION public.test_reject_tool_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test_receipt_write_failure'; END $$; CREATE TRIGGER test_reject_tool_receipt BEFORE INSERT ON public.tool_executions FOR EACH ROW EXECUTE FUNCTION public.test_reject_tool_receipt()`); err != nil {
		t.Fatal(err)
	}
	requests := []ToolExecutionRequest{
		{CapabilityName: "memory_event", OperationID: "rollback-memory", Arguments: json.RawMessage(`{"content":"transactional memory","type":"episodic","confidence":0.9,"importance":0.7}`)},
		{CapabilityName: "affect_event", OperationID: "rollback-affect", Arguments: json.RawMessage(`{"event":{"type":"sad","confidence":1}}`)},
	}
	counts := func() [4]int {
		t.Helper()
		var result [4]int
		if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.memories WHERE owner_fluctlight_id=$1),(SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1),(SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1),(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1)`, fluctlight).Scan(&result[0], &result[1], &result[2], &result[3]); err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := counts()
	for index := range requests {
		requests[index].AuthorizationActorID, requests[index].FluctlightID, requests[index].EvidenceID = owner, fluctlight, "owner-command-"+requests[index].OperationID
		if _, err := app.ExecuteTool(ctx, requests[index]); err == nil || !strings.Contains(err.Error(), "test_receipt_write_failure") {
			t.Fatalf("%s did not reach injected receipt failure: %v", requests[index].CapabilityName, err)
		}
		if after := counts(); after != before {
			t.Fatalf("%s escaped its local rollback: before=%v after=%v", requests[index].CapabilityName, before, after)
		}
	}
	if _, err := repo.Pool().Exec(ctx, `DROP TRIGGER test_reject_tool_receipt ON public.tool_executions; DROP FUNCTION public.test_reject_tool_receipt()`); err != nil {
		t.Fatal(err)
	}
	for _, request := range requests {
		if receipt, err := app.ExecuteTool(ctx, request); err != nil || receipt.Result.Status != "completed" {
			t.Fatalf("restored %s: %#v %v", request.CapabilityName, receipt, err)
		}
	}
	after := counts()
	if after[0] != before[0]+1 || after[1] != before[1]+1 || after[3] != before[3]+2 {
		t.Fatalf("restored writes missing: before=%v after=%v", before, after)
	}
}
