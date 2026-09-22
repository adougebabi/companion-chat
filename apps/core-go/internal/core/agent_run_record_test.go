package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestAgentRunRecordRetainsCommittedToolAfterCancellationAndPreventsReplay(t *testing.T) {
	ctx, repo, app, owner, fluctlight, conversation := setupDirectPublicationToolTest(t, "run-record")
	definition, _ := FormalAgentDefinitionByID(FormalAgentConversationCognition)
	input := ADKStructuredTaskInput{AgentID: definition.ID, Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "one immutable business request"}}}, Capability: &ADKCapabilityRequest{AuthorizationActorID: owner, FluctlightID: fluctlight, OperationID: "run-with-cancelled-result"}}
	record, previous, err := app.admitFormalRun(ctx, definition, input)
	if err != nil || record == nil || previous != nil {
		t.Fatalf("admit: record=%v previous=%v err=%v", record, previous, err)
	}
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{AgentID: definition.ID, RunID: record.RunID, CapabilityName: "conversation.reply", OperationID: "committed-reply", AuthorizationActorID: owner, FluctlightID: fluctlight, ConversationID: conversation, Arguments: json.RawMessage(`{"text":"已真实提交"}`)})
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("commit: %#v %v", receipt, err)
	}
	_, partial, err := app.admitFormalRun(ctx, definition, input)
	if err == nil || !strings.Contains(err.Error(), "agent_run_incomplete") || partial == nil || len(partial.Trace.Results) != 1 {
		t.Fatalf("interrupted admission lost committed fact: %#v %v", partial, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := app.finishFormalRun(cancelled, record, *partial, context.Canceled); err != nil {
		t.Fatalf("cancelled run fact recording: %v", err)
	}
	_, failed, replayErr := app.admitFormalRun(ctx, definition, input)
	if replayErr == nil || !strings.Contains(replayErr.Error(), "context canceled") || failed == nil || len(failed.Trace.Results) != 1 {
		t.Fatalf("failed run admission: %#v %v", failed, replayErr)
	}
	var status string
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.agent_runs WHERE run_id=$1`, record.RunID).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("run status %q: %v", status, err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count); err != nil || count != 1 {
		t.Fatalf("committed message count %d: %v", count, err)
	}
	changed := input
	changed.Prompt.Messages = []map[string]any{{"role": "user", "content": "different business input"}}
	if _, _, err := app.admitFormalRun(ctx, definition, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("reused run identity must conflict: %v", err)
	}
}

func TestStructuredAgentOutputDoesNotPublishProtocolAsVisibleText(t *testing.T) {
	for _, structured := range []map[string]any{{"visible_text": ""}, {"action_type": "no_op"}} {
		if text := finalAgentVisibleText(ProviderCompletion{Structured: structured, Text: jsonString(structured)}); text != "" {
			t.Fatalf("protocol leaked as text: %q", text)
		}
	}
	if text := finalAgentVisibleText(ProviderCompletion{Text: "ordinary text"}); text != "ordinary text" {
		t.Fatalf("plain text contract lost: %q", text)
	}
}
