package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestReplyMediaActionReferencesRejectForeignAndKeepPending(t *testing.T) {
	ref := replyActionRef("draft-a", 0)
	if ref == replyActionRef("draft-b", 0) || ref == replyActionRef("draft-a", 1) {
		t.Fatal("component identity collides")
	}
	states := []replyActionState{{Ref: ref, Status: "pending"}, {Ref: "unavailable", Status: "unavailable"}}
	if err := validateReplyActionRefs([]string{ref}, states); err != nil {
		t.Fatal(err)
	}
	if err := validateReplyActionRefs([]string{replyActionRef("draft-b", 0)}, states); err == nil {
		t.Fatal("foreign result accepted")
	}
	if !replyActionsPending(states) {
		t.Fatal("pending action became sendable")
	}
	states[0].Status = "ready"
	if replyActionsPending(states) {
		t.Fatal("terminal components still waiting")
	}
}

func TestReplyMediaPendingIsAuthoritativeWaitingButNeverCommitted(t *testing.T) {
	for _, status := range []string{"needs_action", "waiting"} {
		trace := &ADKCapabilityTrace{}
		trace.Append(CapabilityInvocation{CallID: "reply", CapabilityName: conversationReplyCapabilityName}, CapabilityResult{CallID: "reply", CapabilityName: conversationReplyCapabilityName, Status: "accepted", Output: map[string]any{"reply_status": status, "draft_ref": "draft"}})
		if _, committed := committedConversationReplyResult(func() []CapabilityResult { _, r := trace.Snapshot(); return r }()); committed {
			t.Fatal("accepted reply counted as publication")
		}
		if got := conversationHasAuthoritativeOutput(trace); got != (status == "waiting") {
			t.Fatalf("status %s allowed final: %v", status, got)
		}
	}
}

func TestReplyMediaContinuationCanonicalScope(t *testing.T) {
	run := &privateReplyRun{Scope: ToolExecutionRequest{AuthorizationActorID: "owner", FluctlightID: "self", ConversationID: "conversation"}}
	saved := replyExecutionScope(run, CapabilityInvocation{SourceFactID: "original-inbox", Metadata: InvocationMetadata{CorrelationID: "conversation-cognition-agent:turn:original", WorkingProfileID: "persona"}})
	scope := saved["scope"].(ToolExecutionRequest)
	if scope.CorrelationID != "turn:original" || scope.EvidenceID != "original-inbox" || scope.WorkingProfileID != "persona" {
		t.Fatalf("lost original authority: %#v", scope)
	}
	if err := authorizeReplyContinuationTx(context.Background(), nil, "source", "self", "conversation"); err != ErrUnauthorized {
		t.Fatalf("unissued continuation authority: %v", err)
	}
}

func TestReplyMediaPublicationPreservesOriginalSourceAndRejectsCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "waiting", true: "cancelled"}[cancelled], func(t *testing.T) {
			ctx, repo, app, owner, self, conversation := setupDirectPublicationToolTest(t, "media-resume")
			source := "source-media"
			draft := "draft-media"
			status := "waiting_reply"
			if cancelled {
				status = "failed"
			}
			_, err := repo.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,1,'conversation.turn',$3,'source','turn:original','source-media',now(),$4)`, source, self, jsonBytes(map[string]any{"turn_id": "original", "conversation_id": conversation}), status)
			if err != nil {
				t.Fatal(err)
			}
			_, err = repo.Pool().Exec(ctx, `INSERT INTO public.conversation_reply_drafts(id,logical_key,fluctlight_id,conversation_id,authorization_actor_id,subject_actor_id,source_fact_id,status,config_snapshot) VALUES($1,$1,$2,$3,$4,$4,$5,'ready','{}')`, draft, self, conversation, owner, source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = repo.Pool().Exec(ctx, `INSERT INTO public.conversation_reply_continuations(draft_id,workflow_id,status) VALUES($1,'workflow','started')`, draft)
			if err != nil {
				t.Fatal(err)
			}
			ctx = context.WithValue(ctx, replyContinuationKey{}, draft)
			err = withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
				resource, err := NewToolPublicationService(app).PublishConversationReplyTx(ctx, tx, ConversationReplyPublication{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, OperationID: "original-reply", CorrelationID: "turn:original", Text: "实际媒体结果后的正文🙂"})
				if err != nil {
					return err
				}
				var actualSource, actualTurn string
				if err := tx.QueryRow(ctx, `SELECT source_fact_id,turn_id FROM public.conversation_messages WHERE id=$1`, resource.ID).Scan(&actualSource, &actualTurn); err != nil {
					return err
				}
				if actualSource != source || actualTurn != "original" {
					t.Fatalf("lost source: %s / %s", actualSource, actualTurn)
				}
				return nil
			})
			if cancelled && err == nil {
				t.Fatal("cancelled source published")
			}
			if !cancelled && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReplyMediaExecutionFailureRemainsConsumableByNativeLoop(t *testing.T) {
	result, err := replyMediaExecutionFailure(CapabilityResult{CallID: "component", CapabilityName: "media.image.generate", Status: "failed", ErrorCode: "media_capability_unavailable", Retryable: true}, errors.New("unavailable"))
	if result.Status != "failed" || result.Retryable || result.ErrorCode != "media_capability_unavailable" || result.CallID != "component" {
		t.Fatalf("failed media would abort Agent: %#v", result)
	}
	if _, retryable := capabilityErrorInfo(err, "", true); retryable {
		t.Fatal("wrapped error remains retryable")
	}
}
