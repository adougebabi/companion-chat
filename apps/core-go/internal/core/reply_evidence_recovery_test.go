package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Controlled provider + disposable PostgreSQL, never a real-model request.
func TestReplyEvidenceRecoveryBeforeWriterLease(t *testing.T) {
	for _, physicalFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(physicalFailure), func(t *testing.T) {
			writerCalls := 0
			router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
				writerCalls++
				if physicalFailure {
					return fakeProviderResult{Status: 500, Text: "controlled terminal failure"}
				}
				return fakeProviderResult{Text: "这是受控生成的一次回复。"}
			})
			ctx, repo, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "evidence-recovery", router)
			turn, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": "本轮的当前问题", "turn_id": "evidence-turn", "idempotency_key": "evidence-source", "attachment_refs": []any{}})
			if err != nil {
				t.Fatal(err)
			}
			settings := defaultConversationReplySettings()
			settings.Mode = "fact_only"
			settings.RetryLimit = 0
			run := &privateReplyRun{Settings: settings, ConfigVersion: 1, CurrentInput: "本轮的当前问题", Scope: ToolExecutionRequest{RunID: "same-evidence-run", EvidenceID: turn.InboxID, AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, Surface: CapabilitySurfaceConversation},
				Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-writer", Timeout: time.Second, TokenBudget: 1024}}
			ctx = context.WithValue(ctx, privateReplyRunKey{}, run)
			inv := CapabilityInvocation{CallID: "bad-evidence-call", CapabilityName: conversationReplyCapabilityName, SourceFactID: turn.InboxID, ProviderRequestID: "bad-physical", Arguments: json.RawMessage(`{"evidence_refs":["message:unoffered-history"]}`), Metadata: InvocationMetadata{Source: "model_tool", AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, CorrelationID: "turn:" + turn.TurnID, Surface: CapabilitySurfaceConversation}}
			service := newFactOnlyReplyService(app)
			_, ref, err := service.prepare(ctx, inv, run)
			if code, _ := capabilityErrorInfo(err, "", false); code != "reply_evidence_not_authorized" || writerCalls != 0 {
				t.Fatalf("bad evidence reached Writer: calls=%d err=%v", writerCalls, err)
			}
			var count int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_reply_drafts WHERE id=$1`, ref).Scan(&count); err != nil || count != 0 {
				t.Fatalf("invalid input poisoned durable draft: count=%d err=%v", count, err)
			}
			inv.CallID = "corrected-call"
			inv.Metadata.OperationID = "corrected-operation"
			inv.ProviderRequestID = "corrected-physical"
			inv.Arguments = json.RawMessage(`{}`)
			draft, _, err := service.prepare(ctx, inv, run)
			if physicalFailure {
				if err == nil || writerCalls != 1 {
					t.Fatalf("physical failure not terminal: %v count=%d", err, writerCalls)
				}
			} else if err != nil || draft.Text != "这是受控生成的一次回复。" || writerCalls != 1 {
				t.Fatalf("corrected evidence did not recover: %+v err=%v count=%d", draft, err, writerCalls)
			}
			inv.CallID = "replayed-call"
			inv.Metadata.OperationID = "replayed-operation"
			_, _, err = service.prepare(ctx, inv, run)
			if physicalFailure {
				if code, _ := capabilityErrorInfo(err, "", false); code != "reply_generation_already_failed" {
					t.Fatalf("physical failure retried: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if writerCalls != 1 {
				t.Fatal("replay called Writer again")
			}
		})
	}
}

func TestReplyMediaCatalogDenialCompletesSameDraft(t *testing.T) {
	writerCalls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		writerCalls++
		if writerCalls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{{"id": "private-image-request", "type": "function", "function": map[string]any{"name": replyImageRequestName, "arguments": `{"intent":"受控测试的夜景素描"}`}}}}
		}
		if writerCalls == 2 {
			return fakeProviderResult{Text: "我想配一张夜景图。"}
		}
		if len(arrayValue(payload["tools"])) != 0 || !strings.Contains(jsonString(payload["messages"]), "tool_not_selected") {
			t.Error("adjustment did not consume actual catalog denial without tools")
		}
		return fakeProviderResult{Text: "这次图片能力不可用，我们先继续聊。"}
	})
	ctx, repo, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "catalog-denial", router)
	turn, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": "本轮想要一幅配图", "turn_id": "catalog-turn", "idempotency_key": "catalog-source", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	settings := defaultConversationReplySettings()
	settings.Mode = "fact_only"
	settings.RetryLimit = 0
	run := &privateReplyRun{Settings: settings, ConfigVersion: 1, CurrentInput: "本轮想要一幅配图", Scope: ToolExecutionRequest{RunID: "catalog-run", EvidenceID: turn.InboxID, AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, Surface: CapabilitySurfaceConversation}, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-writer", Timeout: time.Second, TokenBudget: 1024}}
	ctx = context.WithValue(ctx, privateReplyRunKey{}, run)
	inv := CapabilityInvocation{CallID: "first-reply", CapabilityName: conversationReplyCapabilityName, SourceFactID: turn.InboxID, Arguments: json.RawMessage(`{}`), Metadata: InvocationMetadata{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, CorrelationID: "turn:" + turn.TurnID, Surface: CapabilitySurfaceConversation}}
	service := newFactOnlyReplyService(app)
	draft, ref, err := service.prepare(ctx, inv, run)
	if err != nil || len(draft.MediaRequests) != 1 {
		t.Fatalf("private native request not handed off: %+v %v", draft, err)
	}
	capability := factOnlyReplyCapability{service: service, run: run, publication: NewToolPublicationService(app)}
	target := DirectToolTarget{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		_, err := capability.acceptMediaTx(ctx, tx, inv, target, draft, ref)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	app.Kev = controlledKevToolService(t, "no")
	ctx, _, err = app.prepareKevTools(ctx, ADKStructuredTaskInput{AgentID: FormalAgentConversationCognition, Definitions: capabilityCatalog(app.capabilityRegistry(), CapabilitySurfaceConversation), Capability: &ADKCapabilityRequest{Surface: CapabilitySurfaceConversation, Projection: ContextProjection{FluctlightID: self}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := kevSelection(ctx).load(ctx, []string{"media.image.generate"}); err != nil {
		t.Fatal(err)
	}
	inv.CallID = "after-denial"
	adjusted, _, err := service.prepare(ctx, inv, run)
	if err != nil || adjusted.Text != "这次图片能力不可用，我们先继续聊。" || len(adjusted.MediaRequests) != 0 || writerCalls != 3 {
		t.Fatalf("denial stranded draft: %+v %v calls=%d", adjusted, err, writerCalls)
	}
	prepared, err := withCapabilityPreparedData(inv, "fact_only_reply", map[string]any{"draft": adjusted, "draft_ref": ref, "life_revision": adjusted.LifeRevision})
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		_, err := capability.ExecuteDirectTx(ctx, tx, prepared, CapabilityContext{}, target)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var messages, assets int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE fluctlight_id=$1`, self).Scan(&assets); err != nil {
		t.Fatal(err)
	}
	if messages != 1 || assets != 0 {
		t.Fatalf("denial publication fabricated media: messages=%d intents=%d", messages, assets)
	}
}
