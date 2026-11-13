package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// A separate process is killed after a native Tool commits but before the
// direct conversation can publish its reply. The Provider is controlled; the
// database, Tool ledger and direct streaming entry are the real paths.
func TestM2DirectExpiredClaimDoesNotRepeatCommittedTool(t *testing.T) {
	for _, route := range []string{"stream", "stream-pending", "worker", "worker-pending"} {
		t.Run(route, func(t *testing.T) {
			m218ExerciseExpiredClaim(t, route)
		})
	}
}

func m218ExerciseExpiredClaim(t *testing.T, route string) {
	t.Helper()
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-claim-recovery", newFakeProviderRouter())
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://renderer.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	databaseURL, err := url.Parse(os.Getenv("GO_CORE_TEST_DATABASE_URL"))
	if err != nil || databaseURL.Hostname() != "127.0.0.1" || !strings.HasPrefix(repo.Pool().Config().ConnConfig.Database, "lac_core_") {
		t.Fatal("refusing to pass a non-isolated database to the child")
	}
	databaseURL.Path = "/" + repo.Pool().Config().ConnConfig.Database
	childURL := databaseURL.String()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "committed")
	child := exec.Command(executable, "-test.run=^TestM2DirectExpiredClaimChild$", "-test.v")
	child.Env = append(os.Environ(), "M218_CHILD_DB_URL="+childURL, "M218_CHILD_OWNER="+owner, "M218_CHILD_FLUCTLIGHT="+fid, "M218_CHILD_CONVERSATION="+conversation, "M218_CHILD_MARKER="+marker)
	var childOutput bytes.Buffer
	child.Stdout, child.Stderr = &childOutput, &childOutput
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	childExited := false
	defer func() {
		if !childExited {
			_ = child.Process.Kill()
			<-done
		}
	}()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case err := <-done:
			childExited = true
			redacted := strings.ReplaceAll(childOutput.String(), childURL, "[isolated database URL]")
			t.Fatalf("child exited before committed action: %v; %s", err, redacted)
		case <-deadline.C:
			t.Fatal("child did not reach the post-Tool interruption point")
		case <-ticker.C:
		}
	}
	var intents, receipts, messages int
	var status, originalClaim string
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1), (SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='media.image.generate'), (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$2 AND kind='assistant'), (SELECT status FROM public.cognition_inbox WHERE idempotency_key='m2-claim-recovery'), (SELECT COALESCE(claimed_by,'') FROM public.cognition_inbox WHERE idempotency_key='m2-claim-recovery')`, fid, conversation).Scan(&intents, &receipts, &messages, &status, &originalClaim); err != nil || intents != 1 || receipts != 1 || messages != 0 || status != "claimed" || originalClaim == "" {
		var result []byte
		if readErr := repo.Pool().QueryRow(ctx, `SELECT result FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='media.image.generate' LIMIT 1`, fid).Scan(&result); readErr == nil {
			t.Logf("controlled Image receipt: %s", result)
		}
		t.Fatalf("pre-crash state intents=%d receipts=%d messages=%d status=%s err=%v", intents, receipts, messages, status, err)
	}
	var originalIntent, originalIntentStatus string
	if err := repo.Pool().QueryRow(ctx, `SELECT id,status FROM public.media_intents WHERE owner_fluctlight_id=$1`, fid).Scan(&originalIntent, &originalIntentStatus); err != nil || originalIntent == "" || originalIntentStatus != "pending" {
		t.Fatalf("original Image intent status=%s err=%v", originalIntentStatus, err)
	}
	payload := map[string]any{"fluctlight_id": fid, "text": "请给我一张窗外的风景。", "idempotency_key": "m2-claim-recovery", "turn_id": "m2-claim-recovery", "attachment_refs": []any{}}
	// A stale claim alone does not prove its process is dead: a still-live
	// logical lease must prevent a retry from stealing or failing that turn.
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET claimed_at=now()-interval '4 minutes' WHERE idempotency_key='m2-claim-recovery'`); err != nil {
		t.Fatal(err)
	}
	liveCtx, stopLive := context.WithTimeout(ctx, 2*time.Second)
	_, liveErr := app.handleTurn(liveCtx, owner, conversation, payload, turnCallbacks{}, true)
	stopLive()
	var retainedClaim string
	if err := repo.Pool().QueryRow(ctx, `SELECT status,COALESCE(claimed_by,'') FROM public.cognition_inbox WHERE idempotency_key='m2-claim-recovery'`).Scan(&status, &retainedClaim); err != nil || !errors.Is(liveErr, ErrConflict) || status != "claimed" || retainedClaim != originalClaim {
		t.Fatalf("live process lost its claim: status=%s same_claim=%t retry_err=%v query_err=%v", status, retainedClaim == originalClaim, liveErr, err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatal("child unexpectedly exited without being killed")
	}
	childExited = true
	var rawBudget []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT payload->'reply_budget' FROM public.cognition_inbox WHERE idempotency_key='m2-claim-recovery'`).Scan(&rawBudget); err != nil {
		t.Fatal(err)
	}
	budget := decodeObject(rawBudget)
	age := 4 * time.Minute
	if strings.HasPrefix(route, "worker") {
		age = 11 * time.Minute
		budget["timeout_seconds"] = 720
	}
	budget["admitted_at"] = time.Now().Add(-age).UTC().Format(time.RFC3339Nano)
	budget["deadline_at"] = time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET claimed_at=now()-make_interval(secs=>$2),payload=jsonb_set(payload,'{reply_budget}',$1::jsonb) WHERE idempotency_key='m2-claim-recovery' AND status='claimed'`, jsonBytes(budget), int(age.Seconds())); err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(route, "-pending") {
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET status='pending',claimed_by=NULL,claimed_at=NULL WHERE idempotency_key='m2-claim-recovery'`); err != nil {
			t.Fatal(err)
		}
	}
	// The killed process also left a three-minute logical Agent lease. Simulate
	// its expiration along with the shorter streaming claim, within the same
	// five-minute reply budget; the original process is already confirmed dead.
	updated, err := repo.Pool().Exec(ctx, `UPDATE public.logical_agent_leases SET expires_at=now()-interval '1 second' WHERE fluctlight_id=$1`, fid)
	if err != nil || updated.RowsAffected() != 1 {
		t.Fatalf("expire killed process's logical lease: rows=%d err=%v", updated.RowsAffected(), err)
	}
	calls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		calls++
		name := capabilityCatalogName
		args := map[string]any{"names": []any{"media.image.generate"}}
		switch calls {
		case 2:
			name = "media.image.generate"
			args = map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")}
		case 3:
			name = conversationReplyCapabilityName
			args = map[string]any{"text": "我准备处理这张图片。", "decision": map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}}
		default:
			if calls > 3 {
				return fakeProviderResult{Err: errors.New("unexpected extra recovery model call")}
			}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("m2-resume-%d", calls), name, args)}}
	})
	app.Provider.HTTP.Transport = m218StreamTransport(router)
	recoveryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var runErr error
	if strings.HasPrefix(route, "worker") {
		var inboxID string
		if err := repo.Pool().QueryRow(ctx, `SELECT id FROM public.cognition_inbox WHERE idempotency_key='m2-claim-recovery'`).Scan(&inboxID); err != nil {
			t.Fatal(err)
		}
		_, runErr = app.ProcessCognitionInbox(recoveryCtx, inboxID)
	} else {
		_, runErr = app.handleTurn(recoveryCtx, owner, conversation, payload, turnCallbacks{}, true)
	}
	var code string
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1), (SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='media.image.generate'), (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$2 AND kind='assistant'), (SELECT status FROM public.cognition_inbox WHERE idempotency_key='m2-claim-recovery'), (SELECT COALESCE(error_code,'') FROM public.cognition_inbox WHERE idempotency_key='m2-claim-recovery')`, fid, conversation).Scan(&intents, &receipts, &messages, &status, &code); err != nil {
		t.Fatal(err)
	}
	t.Logf("recovery intents=%d receipts=%d messages=%d status=%s code=%s model_calls=%d err=%v", intents, receipts, messages, status, code, calls, runErr)
	if intents != 1 || receipts != 1 {
		t.Fatalf("same turn repeated an accepted Image operation: intents=%d receipts=%d", intents, receipts)
	}
	var retainedIntent, retainedIntentStatus string
	if err := repo.Pool().QueryRow(ctx, `SELECT id,status FROM public.media_intents WHERE owner_fluctlight_id=$1`, fid).Scan(&retainedIntent, &retainedIntentStatus); err != nil || retainedIntent != originalIntent || retainedIntentStatus != "pending" {
		t.Fatalf("accepted Image intent was not preserved: same=%t status=%s err=%v", retainedIntent == originalIntent, retainedIntentStatus, err)
	}
	if status == "failed" && (code != "agent_partial_committed" || messages != 0) {
		t.Fatalf("partial action was not safely recorded: status=%s code=%s messages=%d err=%v", status, code, messages, runErr)
	}
	if status != "failed" && (runErr != nil || messages != 1) {
		t.Fatalf("successful recovery did not publish exactly one reply: status=%s messages=%d err=%v", status, messages, runErr)
	}
}

func TestM2DirectExpiredClaimWithoutCommittedToolCanRecover(t *testing.T) {
	calls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		calls++
		if calls != 1 {
			return fakeProviderResult{Err: errors.New("unexpected model call")}
		}
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("m2-uncommitted-reply", conversationReplyCapabilityName, map[string]any{"text": "我在这里。", "decision": decision})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-claim-no-write", m218StreamTransport(router))
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	payload := map[string]any{"fluctlight_id": fid, "text": "你好。", "idempotency_key": "m2-claim-no-write", "turn_id": "m2-claim-no-write", "attachment_refs": []any{}}
	if _, err := app.AcceptTurn(ctx, owner, conversation, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET status='claimed',claimed_by='expired-test-process',claimed_at=now()-interval '4 minutes' WHERE idempotency_key='m2-claim-no-write'`); err != nil {
		t.Fatal(err)
	}
	_, releaseLive, err := app.enterLogicalRun(ctx, fid, "conversation")
	if err != nil {
		t.Fatal(err)
	}
	_, _, liveErr := app.enqueueTurnFactClaimed(ctx, owner, fid, conversation, "m2-claim-no-write", "m2-claim-no-write", "你好。", []any{})
	releaseLive()
	var retainedClaim string
	if err := repo.Pool().QueryRow(ctx, `SELECT COALESCE(claimed_by,'') FROM public.cognition_inbox WHERE idempotency_key='m2-claim-no-write'`).Scan(&retainedClaim); err != nil || !errors.Is(liveErr, ErrConflict) || retainedClaim != "expired-test-process" {
		t.Fatalf("live no-write claim was stolen: owner=%s retry_err=%v query_err=%v", retainedClaim, liveErr, err)
	}
	turn, err := app.handleTurn(ctx, owner, conversation, payload, turnCallbacks{}, true)
	if err != nil || stringValue(turn.Assistant["text"]) != "我在这里。" || calls != 1 {
		t.Fatalf("uncommitted turn did not recover: calls=%d turn=%#v err=%v", calls, turn.Assistant, err)
	}
	turn, err = app.handleTurn(ctx, owner, conversation, payload, turnCallbacks{}, true)
	if err != nil || stringValue(turn.Assistant["text"]) != "我在这里。" || calls != 1 {
		t.Fatalf("completed reply replay called the model again: calls=%d turn=%#v err=%v", calls, turn.Assistant, err)
	}
}

func TestM2LostClaimCannotCommitToolOrFailNewOwner(t *testing.T) {
	for _, mode := range []string{"fact_only", "legacy_direct"} {
		t.Run(mode, func(t *testing.T) {
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-lost-claim", newFakeProviderRouter())
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://renderer.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
				t.Fatal(err)
			}
			payload := map[string]any{"fluctlight_id": fid, "text": "看一看窗外。", "idempotency_key": "m2-lost-claim", "turn_id": "m2-lost-claim", "attachment_refs": []any{}}
			turn, err := app.AcceptTurn(ctx, owner, conversation, payload)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET status='claimed',claimed_by='old-stream',claimed_at=now()-interval '4 minutes' WHERE id=$1`, turn.InboxID); err != nil {
				t.Fatal(err)
			}
			if mode == "legacy_direct" {
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=payload-$2::text WHERE id=$1`, turn.InboxID, directTurnModelSnapshotContractKey); err != nil {
					t.Fatal(err)
				}
			}
			oldCtx, release, err := app.enterLogicalRun(ctx, fid, "conversation")
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			oldCtx = context.WithValue(oldCtx, directTurnClaimKey{}, directTurnClaim{InboxID: turn.InboxID, Owner: "old-stream"})
			input := ConversationCognitionAgentInput{AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, SourceFactID: turn.InboxID, CurrentInput: "看一看窗外。"}
			if _, err := app.withDirectTurnCognitionSnapshot(oldCtx, input, mode == "legacy_direct"); err != nil {
				t.Fatalf("old run did not pass its earlier claim check: %v", err)
			}
			// Reproduce the interleaving where the old model has already read
			// its claim, then the stream is taken over before its Tool commits.
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET claimed_by='new-stream',claimed_at=now() WHERE id=$1`, turn.InboxID); err != nil {
				t.Fatal(err)
			}
			image := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "m2-lost-claim-image", NativeToolCallID: "m2-lost-claim-image", ProviderRequestID: "m2-controlled-provider", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, EvidenceID: turn.InboxID, Arguments: jsonBytes(map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")}), Surface: CapabilitySurfaceConversation}
			_, toolErr := app.ExecuteTool(oldCtx, image)
			failureErr := app.failAgentTurnAfterRun(oldCtx, turn.InboxID, agentCommittedOutcome{}, "agent_run_failed")
			var intents, receipts int
			var status, claimedBy string
			if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1),(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='media.image.generate'),(SELECT status FROM public.cognition_inbox WHERE id=$2),(SELECT COALESCE(claimed_by,'') FROM public.cognition_inbox WHERE id=$2)`, fid, turn.InboxID).Scan(&intents, &receipts, &status, &claimedBy); err != nil {
				t.Fatal(err)
			}
			if !errors.Is(toolErr, errDirectTurnSnapshotStale) || !errors.Is(failureErr, errDirectTurnSnapshotStale) || intents != 0 || receipts != 0 || status != "claimed" || claimedBy != "new-stream" {
				t.Fatalf("stale run crossed claim fence: tool_err=%v failure_err=%v intents=%d receipts=%d status=%s owner=%s", toolErr, failureErr, intents, receipts, status, claimedBy)
			}
		})
	}
}

func TestM2ExpiredClaimPreservesFactOnlyMediaDraft(t *testing.T) {
	for _, scenario := range []string{"draft-only", "independent-image", "bound-image"} {
		t.Run(scenario, func(t *testing.T) {
			m218ExerciseFactOnlyDraft(t, scenario)
		})
	}
}

func m218ExerciseFactOnlyDraft(t *testing.T, scenario string) {
	t.Helper()
	writerCalls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		writerCalls++
		if writerCalls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("m2-draft-image", replyImageRequestName, map[string]any{"intent": "当前草稿夜景"})}}
		}
		return fakeProviderResult{Text: "我想配一张夜景图。"}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-claim-fact-draft", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	ctx, run, inv := draftRecoveryInvocation(t, ctx, app, owner, fid, conversation, "m2-claim-fact-draft")
	service := newFactOnlyReplyService(app)
	draft, ref, err := service.prepare(ctx, inv, run)
	if err != nil || len(draft.MediaRequests) != 1 || writerCalls != 2 {
		t.Fatalf("prepare fact-only draft: requests=%d calls=%d err=%v", len(draft.MediaRequests), writerCalls, err)
	}
	capability := factOnlyReplyCapability{service: service, run: run, publication: NewToolPublicationService(app)}
	prepared, err := capability.Prepare(ctx, inv, CapabilityContext{Life: &CurrentLifeContext{Data: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{CapabilityName: conversationReplyCapabilityName, OperationID: "m2-fact-draft-operation", AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, EvidenceID: inv.SourceFactID, Arguments: inv.Arguments, Surface: CapabilitySurfaceConversation}
	target := DirectToolTarget{AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Kind: "conversation", Ref: conversation}
	result, _, _, err := app.executeToolMutation(ctx, request, func(tx pgx.Tx) (CapabilityResult, error) {
		return capability.ExecuteDirectTx(ctx, tx, prepared, CapabilityContext{}, target)
	})
	if err != nil || result.Status != "accepted" || stringValue(mapValue(result.Output)["draft_ref"]) != ref {
		t.Fatalf("commit fact-only draft and Tool receipt: result=%#v err=%v", result, err)
	}
	if scenario != "draft-only" {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://renderer.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "independent-image" {
		image := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "m2-independent-image", NativeToolCallID: "m2-independent-image", ProviderRequestID: "m2-controlled-provider", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, EvidenceID: inv.SourceFactID, Arguments: jsonBytes(map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")}), Surface: CapabilitySurfaceConversation}
		receipt, err := app.ExecuteTool(ctx, image)
		if err != nil || receipt.Result.Status != "accepted" {
			t.Fatalf("commit independent Image operation: result=%#v err=%v", receipt.Result, err)
		}
	}
	if scenario == "bound-image" {
		entry, ok := app.capabilityRegistry().LookupCapability("media.image.generate")
		image, valid := entry.(imageGenerateCapability)
		if !ok || !valid {
			t.Fatal("Image capability not installed")
		}
		component := replyActionRef(ref, 0)
		imageInv := CapabilityInvocation{CallID: "m2-bound-image", CapabilityName: "media.image.generate", SourceFactID: inv.SourceFactID, ProviderRequestID: "m2-controlled-provider", Arguments: jsonBytes(map[string]any{"intent": "当前草稿夜景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown"), "draft_ref": ref, "media_request_ref": component}), Metadata: InvocationMetadata{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: fid, ConversationID: conversation, Surface: CapabilitySurfaceConversation, Source: "model_tool", CorrelationID: "turn:m2-claim-fact-draft"}}
		bound := replyImageCapability{imageGenerateCapability: image, app: app, run: run}
		imageInv, err = bound.Prepare(ctx, imageInv, CapabilityContext{})
		if err != nil {
			t.Fatal(err)
		}
		imageRequest := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: component, NativeToolCallID: "m2-bound-image", ProviderRequestID: "m2-controlled-provider", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, EvidenceID: inv.SourceFactID, Arguments: imageInv.Arguments, Surface: CapabilitySurfaceConversation}
		imageResult, _, _, err := app.executeToolMutation(ctx, imageRequest, func(tx pgx.Tx) (CapabilityResult, error) {
			return bound.ExecuteDirectTx(ctx, tx, imageInv, CapabilityContext{}, target)
		})
		if err != nil || imageResult.Status != "accepted" {
			t.Fatalf("commit bound draft Image: result=%#v err=%v", imageResult, err)
		}
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET status='claimed',claimed_by='dead-fact-process',claimed_at=now()-interval '4 minutes',attempt_count=1 WHERE id=$1`, inv.SourceFactID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.enqueueTurnFactClaimed(ctx, owner, fid, conversation, "m2-claim-fact-draft", "m2-claim-fact-draft", "当前轮的问题", []any{}); err != nil {
		t.Fatal(err)
	}
	var status, draftStatus, code string
	if err := repo.Pool().QueryRow(ctx, `SELECT i.status,d.status,COALESCE(i.error_code,'') FROM public.cognition_inbox i JOIN public.conversation_reply_drafts d ON d.source_fact_id=i.id WHERE i.id=$1 AND d.id=$2`, inv.SourceFactID, ref).Scan(&status, &draftStatus, &code); err != nil {
		t.Fatal(err)
	}
	if scenario == "independent-image" {
		if status != "failed" || code != "agent_partial_committed" || draftStatus != "needs_action" {
			t.Fatalf("draft hid an independent committed image: inbox=%s code=%s draft=%s", status, code, draftStatus)
		}
		return
	}
	wantDraftStatus := "needs_action"
	if scenario == "bound-image" {
		wantDraftStatus = "waiting_media"
	}
	if status != "claimed" || draftStatus != wantDraftStatus {
		t.Fatalf("fact-only draft was blocked on recovery: inbox=%s draft=%s err=%v", status, draftStatus, err)
	}
	resumeCtx := context.WithValue(context.Background(), directMainConversationKey{}, true)
	resumeCtx, err = app.withPrivateConversationReplyRun(resumeCtx, run.Scope, run.CurrentInput)
	if err != nil || replyRunFromContext(resumeCtx) == nil || replyRunFromContext(resumeCtx).DraftRef != ref {
		t.Fatalf("resumed run did not recover the canonical draft: %v", err)
	}
	entry, ok := app.capabilityRegistryForContext(resumeCtx).LookupCapability("media.image.generate")
	restoredImage, valid := entry.(replyImageCapability)
	if !ok || !valid {
		t.Fatal("resumed fact-only Image capability missing")
	}
	_, unboundErr := restoredImage.Prepare(resumeCtx, CapabilityInvocation{Arguments: jsonBytes(map[string]any{"intent": "窗外风景"})}, CapabilityContext{})
	if code, _ := capabilityErrorInfo(unboundErr, "", false); code != "reply_media_binding_required" {
		t.Fatalf("resumed draft allowed an unbound Image request: %v", unboundErr)
	}
	recovered, recoveredRef, err := service.prepare(ctx, inv, run)
	if err != nil || recoveredRef != ref || len(recovered.MediaRequests) != 1 || writerCalls != 2 {
		t.Fatalf("fact-only draft was not recovered: same=%t requests=%d calls=%d err=%v", recoveredRef == ref, len(recovered.MediaRequests), writerCalls, err)
	}
}

func TestM2DirectExpiredClaimChild(t *testing.T) {
	databaseURL := os.Getenv("M218_CHILD_DB_URL")
	if databaseURL == "" {
		t.Skip("only run by the isolated M2 claim recovery parent")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Hostname() != "127.0.0.1" || !strings.HasPrefix(strings.TrimPrefix(parsed.Path, "/"), "lac_core_") {
		t.Fatal("child refused a non-isolated database")
	}
	ctx := context.Background()
	repo, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatal("connect to isolated test database")
	}
	defer repo.Close()
	calls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		calls++
		name := capabilityCatalogName
		args := map[string]any{"names": []any{"media.image.generate"}}
		if calls == 2 {
			name = "media.image.generate"
			args = map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")}
		} else if calls == 3 {
			if err := os.WriteFile(os.Getenv("M218_CHILD_MARKER"), []byte("committed"), 0600); err != nil {
				t.Fatal(err)
			}
			select {}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("m2-first-%d", calls), name, args)}}
	})
	app := newTestApp(t, repo, m218StreamTransport(router))
	payload := map[string]any{"fluctlight_id": os.Getenv("M218_CHILD_FLUCTLIGHT"), "text": "请给我一张窗外的风景。", "idempotency_key": "m2-claim-recovery", "turn_id": "m2-claim-recovery", "attachment_refs": []any{}}
	_, err = app.handleTurn(ctx, os.Getenv("M218_CHILD_OWNER"), os.Getenv("M218_CHILD_CONVERSATION"), payload, turnCallbacks{}, true)
	t.Fatalf("child exited before the controlled process interruption: %v", err)
}

// The formal streaming entry consumes SSE. Keep this conversion local to the
// test so its controlled Provider follows the same native Tool wire protocol.
func m218StreamTransport(router *fakeProviderRouter) http.RoundTripper {
	return projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		request.Body = io.NopCloser(bytes.NewReader(raw))
		response, err := router.RoundTrip(request)
		if err != nil || decodeObject(raw)["stream"] != true || response.StatusCode != http.StatusOK {
			return response, err
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			return nil, err
		}
		value := decodeObject(body)
		choice := mapValue(arrayValue(value["choices"])[0])
		message := mapValue(choice["message"])
		for index, tool := range arrayValue(message["tool_calls"]) {
			mapValue(tool)["index"] = index
		}
		choice["delta"] = message
		delete(choice, "message")
		response = embeddingHTTPResponse(request, http.StatusOK, "data: "+jsonString(value)+"\n\ndata: [DONE]\n\n")
		response.Header.Set("Content-Type", "text/event-stream")
		return response, nil
	})
}
