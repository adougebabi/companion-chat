package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestConversationReplyPublicationFiltersNativeAndPreparedDrift(t *testing.T) {
	ctx, repo, app, owner, self, conversation := setupDirectPublicationToolTest(t, "body-drift")
	raw := "# 回复\n\n我想和你聊聊新书。\n\n# 认知\n私密推理\n\n# 事务记录\n- 使用工具：无"
	want := "我想和你聊聊新书。"
	request := ToolExecutionRequest{CapabilityName: "conversation.reply", OperationID: "drift-native", AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, Arguments: json.RawMessage(jsonBytes(map[string]any{"text": raw}))}
	receipt, err := app.ExecuteTool(ctx, request)
	if err != nil || receipt.Result.Status != "completed" || stringValue(mapValue(receipt.Result.Output)["text"]) != want {
		t.Fatalf("native receipt leaked raw protocol: %#v, %v", receipt.Result, err)
	}
	// The shared boundary also protects durable prepared drafts made before this
	// fix, independent of the current Writer or native Tool argument decoder.
	err = withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		resource, err := NewToolPublicationService(app).PublishConversationReplyTx(ctx, tx, ConversationReplyPublication{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, OperationID: "drift-prepared", Text: raw})
		if err == nil && resource.Text != want {
			return fmt.Errorf("publication returned unfiltered text: %q", resource.Text)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	err = withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		resource, err := NewToolPublicationService(app).PublishConversationReplyTx(ctx, tx, ConversationReplyPublication{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, OperationID: "drift-fenced-envelope", Text: "```json\n{\"text\":\"# Reply\\n我想和你聊聊新书。\\n# Ledger\\nprivate\"}\n```"})
		if err == nil && resource.Text != want {
			return fmt.Errorf("publication returned raw JSON envelope: %q", resource.Text)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2`, conversation, want).Scan(&count); err != nil || count != 3 {
		t.Fatalf("clean committed replies = %d, want 3: %v", count, err)
	}
	request.OperationID = "drift-private-only"
	request.Arguments = json.RawMessage(`{"text":"# Cognition\nprivate reasoning\n# Ledger\nprivate transaction"}`)
	rejected, err := app.ExecuteTool(ctx, request)
	if err != nil || rejected.Result.Status != "handled" || goalSubmissionReceiptCode(rejected.Result) != "invalid_arguments" || mapValue(rejected.Result.Output)["effect_applied"] != false {
		t.Fatalf("private-only output was not handled without publication: %#v, %v", rejected.Result, err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count); err != nil || count != 3 {
		t.Fatalf("failed parse inserted an assistant: count=%d, error=%v", count, err)
	}
}

func TestReplyWriterActualTransportExtractsDriftedBody(t *testing.T) {
	for _, raw := range []string{
		"# 回复\n你好，很高兴认识你。\n# 认知\nprivate\n# 事务记录\n- 发送时间：2026-11-02T20:48:16+08:00\n- 使用工具：无",
		"```markdown\n# Reply\n你好，很高兴认识你。\n# Reasoning\nprivate\n# Transaction Log\nTools used: none\n```",
	} {
		router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
			if payload["max_tokens"] != nil || payload["max_completion_tokens"] != nil {
				t.Error("reply parser change introduced an output cap")
			}
			return fakeProviderResult{Text: raw}
		})
		app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
		draft, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "model-agnostic-drift", Timeout: time.Second}, Concurrency: 1})
		if err != nil || draft.Text != "你好，很高兴认识你。" {
			t.Fatalf("transport accepted envelope: text=%q error=%v", draft.Text, err)
		}
	}
}
