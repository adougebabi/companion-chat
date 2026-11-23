package core

import (
	"fmt"
	"net/url"
	"testing"
)

func TestChatTaskBusinessOutcomeOverridesCompletedExecution(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := durableReplyFixture(t, "task-business-outcome")
	accepted, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "测试消息", "turn_id": "business-outcome", "idempotency_key": "business-outcome"})
	if err != nil {
		t.Fatal(err)
	}
	intent := "cognition_intent:" + accepted.InboxID
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed',completed_at=now() WHERE intent_id=$1`, intent); err != nil {
		t.Fatal(err)
	}
	app.DiagnosticDebugReadEnabled = true
	for _, state := range []string{"failed", "pending", "processed"} {
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET status=$2::varchar,error_code=CASE WHEN $2::varchar='failed' THEN 'direct_media_delivery_unresolved' ELSE NULL END WHERE id=$1`, accepted.InboxID, state); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"failed": "failed", "pending": "pending", "processed": "completed"}[state]
		task, err := app.OwnerTask(ctx, owner, intent)
		if err != nil || task["status"] != want || task["workflow_status"] != "completed" || task["reply_delivered"] != false || task["correlation_id"] != "turn:business-outcome" {
			t.Fatalf("state=%s task=%#v err=%v", state, task, err)
		}
		page, err := app.OwnerDiagnosticRecords(ctx, owner, "workflow-intents", url.Values{"intentId": {intent}})
		if err != nil || len(arrayValue(page["items"])) != 1 {
			t.Fatal(page, err)
		}
		data := mapValue(mapValue(arrayValue(page["items"])[0])["data"])
		if data["status"] != want || data["workflow_status"] != "completed" {
			t.Fatal("task/list disagreement", task, data)
		}
		group, _, err := app.readPendingSource(ctx, owner, fid, "failed_task", "", 100, false)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, raw := range arrayValue(group["items"]) {
			if mapValue(raw)["source_id"] == intent {
				found = true
			}
		}
		if found != (state == "failed") {
			t.Fatal("pending attention disagrees with business outcome", state, group)
		}
		if state == "failed" {
			result, err := app.ProcessCognitionInbox(ctx, accepted.InboxID)
			if err != nil || result["status"] != "failed" || result["error_code"] != "direct_media_delivery_unresolved" {
				t.Fatal("terminal replay discarded failure", result, err)
			}
		}
	}
}

func TestMediaPublicationFailureCodeSurvivesTypedWrapping(t *testing.T) {
	err := fmt.Errorf("adk_run: %w", newCapabilityError("direct_media_delivery_unresolved", false, fmt.Errorf("private detail")))
	if got := ConversationTurnFailureCode(err); got != "direct_media_delivery_unresolved" {
		t.Fatal(got)
	}
	if got := adkTerminationReason(err); got != "direct_media_delivery_unresolved" {
		t.Fatal(got)
	}
}
