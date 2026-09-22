package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func setupDirectPublicationToolTest(t *testing.T, suffix string) (context.Context, *PostgresRepository, *App, string, string, string) {
	t.Helper()
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "tool-owner-" + suffix
	fluctlightID := "tool-fluctlight-" + suffix
	conversationID := "tool-conversation-" + suffix
	seedTurnConversation(t, ctx, repository, ownerID, fluctlightID, conversationID)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET identity=$2 WHERE id=$1`, fluctlightID, jsonBytes(map[string]any{
		"timezone": "Asia/Shanghai", "appearance": map[string]any{"hair": "black", "outfit": "blue coat"},
	})); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{
		"baseUrl": "http://comfy.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"},
	})); err != nil {
		t.Fatal(err)
	}
	return ctx, repository, newTestApp(t, repository, nil), ownerID, fluctlightID, conversationID
}

func TestExecuteToolConversationReplyPublishesReplaysAndRejectsPayloadConflict(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupDirectPublicationToolTest(t, "reply")
	request := ToolExecutionRequest{
		CapabilityName: "conversation.reply", OperationID: "reply-operation-1",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		Arguments: json.RawMessage(`{"text":"独立工具回复"}`),
	}
	first, err := app.ExecuteTool(ctx, request)
	if err != nil {
		t.Fatalf("execute reply: %v", err)
	}
	firstOutput := mapValue(first.Result.Output)
	if first.Result.Status != "completed" || stringValue(firstOutput["target_kind"]) != "conversation_message" || stringValue(firstOutput["target_ref"]) == "" || toolBoolValue(firstOutput["replayed"]) {
		t.Fatalf("first reply receipt = %#v", first)
	}

	// The native adapter has a different model ToolCall identity but reuses the
	// same stable business operation and therefore the same committed message.
	request.NativeToolCallID = "provider-reply-call-2"
	request.ProviderRequestID = "provider-reply-request-2"
	replayed, err := app.ExecuteTool(ctx, request)
	if err != nil {
		t.Fatalf("replay reply through native adapter: %v", err)
	}
	replayedOutput := mapValue(replayed.Result.Output)
	if !toolBoolValue(replayedOutput["replayed"]) || stringValue(replayedOutput["target_ref"]) != stringValue(firstOutput["target_ref"]) || replayed.ExecutionCallID != request.NativeToolCallID {
		t.Fatalf("replayed reply receipt = %#v, first = %#v", replayed, first)
	}

	conflict := request
	conflict.Arguments = json.RawMessage(`{"text":"同键不同内容"}`)
	conflict.NativeToolCallID = "provider-reply-call-3"
	conflict.ProviderRequestID = "provider-reply-request-3"
	conflicted, err := app.ExecuteTool(ctx, conflict)
	if err == nil || !errors.Is(err, ErrConflict) || conflicted.Result.ErrorCode != "operation_id_conflict" {
		t.Fatalf("reply payload conflict receipt=%#v err=%v", conflicted, err)
	}
	var messageCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='独立工具回复'`, conversationID).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("assistant message count = %d, want 1", messageCount)
	}
	var cognitionCount, actionCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1`, fluctlightID).Scan(&cognitionCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.autonomy_actions WHERE fluctlight_id=$1`, fluctlightID).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if cognitionCount != 0 || actionCount != 0 {
		t.Fatalf("independent reply fabricated cognition/action facts: cognition=%d action=%d", cognitionCount, actionCount)
	}
}

func TestExecuteToolConversationReplyRejectsUnownedTargetWithoutProduct(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, _ := setupDirectPublicationToolTest(t, "reply-target")
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: "conversation.reply", OperationID: "reply-operation-unowned",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: "missing-conversation",
		Arguments: json.RawMessage(`{"text":"不能落库"}`),
	})
	if err == nil || !errors.Is(err, ErrUnauthorized) || receipt.Result.ErrorCode != "tool_target_unauthorized" {
		t.Fatalf("unowned reply receipt=%#v err=%v", receipt, err)
	}
	var messageCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE text='不能落库'`).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 0 {
		t.Fatalf("unowned reply created %d messages", messageCount)
	}
}

func TestExecuteToolMomentPublishCommitsOutboxAndReplays(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, _ := setupDirectPublicationToolTest(t, "moment")
	request := ToolExecutionRequest{
		CapabilityName: "moment.publish", OperationID: "moment-operation-1",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		Arguments: json.RawMessage(`{"text":"今天的独立动态"}`),
	}
	first, err := app.ExecuteTool(ctx, request)
	if err != nil {
		t.Fatalf("execute Moment: %v", err)
	}
	momentID := stringValue(mapValue(first.Result.Output)["target_ref"])
	if first.Result.Status != "completed" || momentID == "" {
		t.Fatalf("Moment receipt = %#v", first)
	}
	replayed, err := app.ExecuteTool(ctx, request)
	if err != nil || !toolBoolValue(mapValue(replayed.Result.Output)["replayed"]) || stringValue(mapValue(replayed.Result.Output)["target_ref"]) != momentID {
		t.Fatalf("Moment replay receipt=%#v err=%v", replayed, err)
	}
	var momentCount, outboxCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE id=$1 AND owner_fluctlight_id=$2 AND text='今天的独立动态'`, momentID, fluctlightID).Scan(&momentCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE kind='moment.published' AND aggregate_id=$1`, momentID).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if momentCount != 1 || outboxCount != 1 {
		t.Fatalf("Moment facts moment=%d outbox=%d", momentCount, outboxCount)
	}

	request.Arguments = json.RawMessage(`{"text":"同键不同动态"}`)
	conflict, err := app.ExecuteTool(ctx, request)
	if err == nil || !errors.Is(err, ErrConflict) || conflict.Result.ErrorCode != "operation_id_conflict" {
		t.Fatalf("Moment conflict receipt=%#v err=%v", conflict, err)
	}
}

func TestExecuteToolImageGenerateAcceptsDurableTaskAndRejectsConflict(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupDirectPublicationToolTest(t, "image")
	request := ToolExecutionRequest{
		CapabilityName: "media.image.generate", OperationID: "image-operation-1",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		TargetKind: "conversation", TargetRef: conversationID,
		Arguments: json.RawMessage(`{"intent":"窗边阅读的写实照片"}`),
	}
	accepted, err := app.ExecuteTool(ctx, request)
	if err != nil {
		t.Fatalf("accept image task: %v", err)
	}
	output := mapValue(accepted.Result.Output)
	intentID, taskID := stringValue(output["media_intent_id"]), stringValue(output["task_id"])
	if accepted.Result.Status != "accepted" || stringValue(output["status"]) != "pending" || intentID == "" || taskID == "" || toolBoolValue(output["replayed"]) {
		t.Fatalf("accepted image receipt = %#v", accepted)
	}
	var intentStatus, workflowStatus string
	var intentCount, workflowCount, outboxCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*),min(status) FROM public.media_intents WHERE id=$1 AND owner_fluctlight_id=$2 AND conversation_id=$3`, intentID, fluctlightID, conversationID).Scan(&intentCount, &intentStatus); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*),min(status) FROM public.platform_workflow_intents WHERE workflow_id=$1 AND intent_type='media.generation'`, taskID).Scan(&workflowCount, &workflowStatus); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE kind='media.intent.created' AND aggregate_id=$1`, intentID).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if intentCount != 1 || intentStatus != "pending" || workflowCount != 1 || workflowStatus != "pending" || outboxCount != 1 {
		t.Fatalf("media facts intent=%d/%q workflow=%d/%q outbox=%d", intentCount, intentStatus, workflowCount, workflowStatus, outboxCount)
	}

	request.NativeToolCallID = "provider-image-call-2"
	request.ProviderRequestID = "provider-image-request-2"
	replayed, err := app.ExecuteTool(ctx, request)
	if err != nil || replayed.Result.Status != "accepted" || !toolBoolValue(mapValue(replayed.Result.Output)["replayed"]) || stringValue(mapValue(replayed.Result.Output)["media_intent_id"]) != intentID {
		t.Fatalf("image replay receipt=%#v err=%v", replayed, err)
	}

	request.Arguments = json.RawMessage(`{"intent":"同键不同图片"}`)
	request.NativeToolCallID = "provider-image-call-3"
	request.ProviderRequestID = "provider-image-request-3"
	conflict, err := app.ExecuteTool(ctx, request)
	if err == nil || !errors.Is(err, ErrConflict) || conflict.Result.ErrorCode != "operation_id_conflict" {
		t.Fatalf("image conflict receipt=%#v err=%v", conflict, err)
	}
}

func TestExecuteToolImageGenerateRejectsInvalidTargetWithoutIntent(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupDirectPublicationToolTest(t, "image-target")
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: "media.image.generate", OperationID: "image-operation-invalid-target",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		TargetKind: "conversation_message", TargetRef: "missing-message",
		Arguments: json.RawMessage(`{"intent":"不应受理的图片"}`),
	})
	if err == nil || !errors.Is(err, ErrNotFound) || receipt.Result.ErrorCode != "tool_target_not_found" {
		t.Fatalf("invalid media target receipt=%#v err=%v", receipt, err)
	}
	var count int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid media target created %d intents", count)
	}
}

func TestExecuteToolImageGenerateDependencyFailureCreatesNoIntent(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupDirectPublicationToolTest(t, "image-dependency")
	if _, err := repository.Pool().Exec(ctx, `DELETE FROM public.runtime_settings WHERE key='media.comfyui'`); err != nil {
		t.Fatal(err)
	}
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: "media.image.generate", OperationID: "image-operation-no-renderer",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		TargetKind: "conversation", TargetRef: conversationID,
		Arguments: json.RawMessage(`{"intent":"依赖失败不能受理"}`),
	})
	if err == nil || receipt.Result.Status != "failed" || receipt.Result.ErrorCode != "capability_prepare_failed" {
		t.Fatalf("media dependency failure receipt=%#v err=%v", receipt, err)
	}
	var count int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("media dependency failure created %d intents", count)
	}
}

func toolBoolValue(value any) bool {
	result, _ := value.(bool)
	return result
}
