package core

import (
	"strings"
	"testing"
)

// A projection contract fixture, not exported workflow execution evidence.
func TestOwnerTaskMediaLifecycleRespectsWorkflowIdentity(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "media-task-read")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "media-read-projection", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, Arguments: jsonBytes(map[string]any{"intent": "隔离投影测试的窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})})
	if err != nil {
		t.Fatal(err)
	}
	output := mapValue(receipt.Result.Output)
	mediaID, key := stringValue(output["media_intent_id"]), stringValue(output["task_id"])
	before, err := app.OwnerTask(ctx, owner, key)
	if err != nil {
		t.Fatal(err)
	}
	wf := strings.TrimPrefix(stringValue(before["workflow_id"]), "go:")
	diagnostic := LifecycleDiagnostic{Surface: "media", Transition: LifecycleTransitionQueued, FluctlightID: fid, IntentID: mediaID, WorkflowID: wf, CorrelationID: "media:" + mediaID, Stage: "projection_fixture", Status: "queued", ReasonCode: "projection_current"}
	right, err := app.RecordLifecycleDiagnostic(ctx, diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic.WorkflowID = "go:" + wf + "-other"
	diagnostic.ReasonCode = "projection_other"
	wrong, err := app.RecordLifecycleDiagnostic(ctx, diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	after, err := app.OwnerTask(ctx, owner, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(arrayValue(after["steps"])) != len(arrayValue(before["steps"]))+1 {
		t.Fatal("media lifecycle was lost or another workflow mixed in")
	}
	found := false
	for _, raw := range arrayValue(after["steps"]) {
		id := stringValue(mapValue(raw)["id"])
		if id == wrong {
			t.Fatal("foreign workflow event leaked")
		}
		found = found || id == right
	}
	if !found {
		t.Fatal("matching domain media event missing")
	}
}
