package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
)

func TestReplyDraftHintRecovery(t *testing.T) {
	for _, hint := range []string{"reply_draft_old_source", "reply_draft_foreign", "cr_ffffffffffff", "unparseable hint"} {
		t.Run(hint, func(t *testing.T) {
			report := reconcileReplyDraftHint("reply_draft_current", hint)
			if stringValue(report["warning_code"]) != "reply_draft_hint_ignored" {
				t.Fatalf("missing safe warning: %v", report)
			}
			if strings.Contains(jsonString(report), hint) {
				t.Fatal("raw hint in report")
			}
		})
	}
}

func TestReplyDraftCodecRecovery(t *testing.T) {
	codec, err := newProviderContextRefCodec(ContextReferenceIndex{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = codec.decodePrivateReplyArguments(json.RawMessage(`{"draft_ref":"cr_ffffffffffff"}`))
	if err != nil {
		t.Fatalf("optional draft alias blocked reconciliation: %v", err)
	}
}

func TestReplyDraftHintCanonicalAndSafeWarnings(t *testing.T) {
	for _, hint := range []string{"", "reply_draft_current"} {
		if warning := reconcileReplyDraftHint("reply_draft_current", hint); warning != nil {
			t.Fatalf("matching/absent hint warned: %v", warning)
		}
	}
	hint := "reply_draft_foreign_sensitive_body"
	report := reconcileReplyDraftHint("reply_draft_current", hint)
	if strings.Contains(jsonString(report), hint) || strings.Contains(jsonString(report), "reply_draft_current") {
		t.Fatal("diagnostic leaks pointer")
	}
}

func TestReplyDraftCodecKeepsRequiredRefsAndOtherToolsStrict(t *testing.T) {
	ref := "memory:ctx_0123456789abcdef0123456789abcdef"
	codec, err := newProviderContextRefCodec(ContextReferenceIndex{ByRef: map[string]ContextReference{ref: {Kind: ContextReferenceMemory}}})
	if err != nil {
		t.Fatal(err)
	}
	raw := jsonBytes(map[string]any{"draft_ref": "cr_ffffffffffff", "evidence_refs": []any{"cr_eeeeeeeeeeee"}, "action_result_refs": []any{providerShortRef(ref)}})
	decoded, err := codec.decodePrivateReplyArguments(raw)
	if err != nil {
		t.Fatal(err)
	}
	args := decodeObject(decoded)
	if stringValue(args["draft_ref"]) != "cr_ffffffffffff" || stringValue(arrayValue(args["action_result_refs"])[0]) != ref {
		t.Fatalf("wrong hint/resource mapping: %s", decoded)
	}
	for _, decode := range []func(json.RawMessage) (json.RawMessage, error){codec.decodeArguments, codec.decodeReplyArguments} {
		if _, err := decode(jsonBytes(map[string]any{"draft_ref": "cr_ffffffffffff"})); err == nil {
			t.Fatal("legacy/Moments draft decoding was relaxed")
		}
	}
	if _, err := codec.decodePrivateReplyArguments(jsonBytes(map[string]any{"draft_ref": "cr_ffffffffffff", "action_result_refs": []any{"cr_eeeeeeeeeeee"}})); err == nil {
		t.Fatal("required action alias became optional")
	}
	if _, err := codec.decodePrivateReplyArguments(jsonBytes(map[string]any{"target_ref": "cr_eeeeeeeeeeee"})); err == nil {
		t.Fatal("target alias became optional")
	}
}

func assertDraftRecoveryWarning(t *testing.T, capability factOnlyReplyCapability, result CapabilityResult) {
	t.Helper()
	if err := capability.Definition().ValidateOutput(result.Output); err != nil {
		t.Fatalf("warning result violates reply schema: %v", err)
	}
	warnings := arrayValue(mapValue(result.Output)["warnings"])
	if len(warnings) != 1 || stringValue(mapValue(warnings[0])["warning_code"]) != "reply_draft_hint_ignored" || stringValue(mapValue(warnings[0])["disposition"]) != "continue_canonical_draft" {
		t.Fatalf("safe warning did not survive result: %+v", result)
	}
}

func draftRecoveryInvocation(t *testing.T, ctx context.Context, app *App, owner, self, conversation, source string) (context.Context, *privateReplyRun, CapabilityInvocation) {
	t.Helper()
	turn, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": "当前轮的问题", "turn_id": source, "idempotency_key": source, "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	settings := defaultConversationReplySettings()
	settings.Mode = "fact_only"
	settings.RetryLimit = 0
	run := &privateReplyRun{Settings: settings, ConfigVersion: 1, CurrentInput: "当前轮的问题", Scope: ToolExecutionRequest{RunID: source, EvidenceID: turn.InboxID, AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, Surface: CapabilitySurfaceConversation}, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-writer", Timeout: time.Second, TokenBudget: 1024}}
	ctx = context.WithValue(ctx, privateReplyRunKey{}, run)
	inv := CapabilityInvocation{CallID: source, CapabilityName: conversationReplyCapabilityName, SourceFactID: turn.InboxID, Arguments: json.RawMessage(`{}`), Metadata: InvocationMetadata{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, CorrelationID: "turn:" + turn.TurnID, Surface: CapabilitySurfaceConversation}}
	return ctx, run, inv
}

// Requires an explicitly task-owned disposable PG; controlled transport only.
func TestReplyDraftRecoveryPreparePublishReplay(t *testing.T) {
	writerCalls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		writerCalls++
		if strings.Contains(jsonString(payload), "foreign-secret") {
			t.Error("foreign draft leaked into Writer")
		}
		return fakeProviderResult{Text: "当前草稿的独立回复。"}
	})
	ctx, repo, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "draft-recovery", router)
	ctx, run, inv := draftRecoveryInvocation(t, ctx, app, owner, self, conversation, "draft-source")
	service := newFactOnlyReplyService(app)
	logicalKey := privateReplyLogicalKey(ctx, inv)
	canonical := "reply_draft_" + stableDigest(logicalKey)
	// A different scoped row exists; the invalid pointer must never select it.
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_reply_drafts(id,logical_key,fluctlight_id,conversation_id,authorization_actor_id,subject_actor_id,profile_id,status,config_snapshot,config_version,binding_snapshot,payload) VALUES('reply_draft_foreign','foreign-key',$1,$2,$3,$3,'','prepared','{}',999,'{}',$4)`, "foreign-self", "foreign-conversation", "foreign-owner", jsonBytes(ReplyDraft{Text: "foreign-secret", AttachmentRefs: []string{"foreign-asset"}, ConfigVersion: 999})); err != nil {
		t.Fatal(err)
	}
	inv.Arguments = jsonBytes(map[string]any{"draft_ref": "reply_draft_foreign"})
	capability := factOnlyReplyCapability{service: service, run: run, publication: NewToolPublicationService(app)}
	prepared, err := capability.Prepare(ctx, inv, CapabilityContext{Life: &CurrentLifeContext{Data: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := capabilityPreparedData(prepared, "fact_only_reply")
	if err != nil || stringValue(mapValue(data)["draft_ref"]) != canonical || writerCalls != 1 {
		t.Fatalf("canonical generation failed: %v %v calls=%d", data, err, writerCalls)
	}
	if len(arrayValue(mapValue(data)["warnings"])) != 1 {
		t.Fatal("safe warning missing from prepared envelope")
	}
	inv.CallID = "cached-replay"
	inv.Arguments = jsonBytes(map[string]any{"draft_ref": "reply_draft_old_source"})
	cached, ref, err := service.prepare(ctx, inv, run)
	if err != nil || ref != canonical || cached.Text != "当前草稿的独立回复。" || len(cached.AttachmentRefs) != 0 || writerCalls != 1 {
		t.Fatalf("cache recovered foreign draft: %+v ref=%s err=%v calls=%d", cached, ref, err, writerCalls)
	}
	target := DirectToolTarget{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation}
	var published CapabilityResult
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		var err error
		published, err = capability.ExecuteDirectTx(ctx, tx, prepared, CapabilityContext{}, target)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if published.Status != "completed" || len(arrayValue(mapValue(published.Output)["warnings"])) != 1 {
		t.Fatalf("publication missing truthful result/warning: %+v", published)
	}
	assertDraftRecoveryWarning(t, capability, published)
	run.AssignmentError = errors.New("writer unavailable on replay")
	replay, err := capability.Prepare(ctx, inv, CapabilityContext{Life: &CurrentLifeContext{Data: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		result, err := capability.ExecuteDirectTx(ctx, tx, replay, CapabilityContext{}, target)
		if err == nil {
			assertDraftRecoveryWarning(t, capability, result)
		}
		if err == nil && (mapValue(result.Output)["replayed"] != true || stringValue(mapValue(result.Output)["text"]) != "当前草稿的独立回复。") {
			t.Fatalf("wrong published replay: %+v", result)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var diagnostic []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT payload FROM public.diagnostic_events WHERE event_type='reply.draft.reconciled' AND correlation_id=$1 ORDER BY created_at DESC LIMIT 1`, inv.Metadata.CorrelationID).Scan(&diagnostic); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"reply_draft_foreign", "reply_draft_old_source", "foreign-secret", "foreign-asset", canonical} {
		if strings.Contains(string(diagnostic), secret) {
			t.Fatalf("persisted reconciliation diagnostic leaked %s", secret)
		}
	}
	var count int
	var foreignPayload []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT payload FROM public.conversation_reply_drafts WHERE id='reply_draft_foreign'`).Scan(&foreignPayload); err != nil {
		t.Fatal(err)
	}
	if count != 1 || writerCalls != 1 || stringValue(decodeObject(foreignPayload)["text"]) != "foreign-secret" {
		t.Fatalf("replay duplicated or changed foreign state: messages=%d calls=%d", count, writerCalls)
	}
}

func TestReplyDraftRecoveryProviderFailureAndCancellation(t *testing.T) {
	writerCalls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		writerCalls++
		return fakeProviderResult{Status: 500, Text: "controlled failure"}
	})
	ctx, repo, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "draft-failure", router)
	ctx, run, inv := draftRecoveryInvocation(t, ctx, app, owner, self, conversation, "draft-failure-source")
	inv.Arguments = jsonBytes(map[string]any{"draft_ref": "reply_draft_foreign"})
	service := newFactOnlyReplyService(app)
	if _, _, err := service.prepare(ctx, inv, run); err == nil {
		t.Fatal("Provider failure masked")
	}
	if _, _, err := service.prepare(ctx, inv, run); err == nil {
		t.Fatal("failed same-run draft retried")
	}
	if writerCalls != 1 {
		t.Fatalf("terminal Writer retried: %d", writerCalls)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := service.prepare(cancelled, inv, run); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed Writer published: %d %v", count, err)
	}
}

func TestReplyDraftRecoveryCancelledBeforeDB(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	inv := CapabilityInvocation{Arguments: json.RawMessage(`{"draft_ref":"reply_draft_foreign"}`)}
	if _, _, err := newFactOnlyReplyService(nil).prepare(ctx, inv, &privateReplyRun{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled recovery reached DB: %v", err)
	}
}

func TestReplyDraftRecoveryCanonicalMediaOnly(t *testing.T) {
	writerCalls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		writerCalls++
		switch writerCalls {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{{"id": "draft-image", "type": "function", "function": map[string]any{"name": replyImageRequestName, "arguments": `{"intent":"当前草稿夜景"}`}}}}
		case 2:
			return fakeProviderResult{Text: "我想配一张夜景图。"}
		default:
			wire := jsonString(payload)
			if len(arrayValue(payload["tools"])) != 0 || !strings.Contains(wire, "canonical_media_failed") || strings.Contains(wire, "foreign_media_secret") {
				t.Errorf("adjustment consumed wrong media: %s", wire)
			}
			return fakeProviderResult{Text: "这次图片失败了，我们继续聊。"}
		}
	})
	ctx, repo, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "draft-media", router)
	ctx, run, inv := draftRecoveryInvocation(t, ctx, app, owner, self, conversation, "draft-media-source")
	service := newFactOnlyReplyService(app)
	draft, ref, err := service.prepare(ctx, inv, run)
	if err != nil || len(draft.MediaRequests) != 1 {
		t.Fatalf("media preparation: %+v %v", draft, err)
	}
	capability := factOnlyReplyCapability{service: service, run: run, publication: NewToolPublicationService(app)}
	target := DirectToolTarget{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation}
	inv.Arguments = jsonBytes(map[string]any{"draft_ref": "reply_draft_foreign"})
	prepared, err := capability.Prepare(ctx, inv, CapabilityContext{Life: &CurrentLifeContext{Data: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		result, err := capability.ExecuteDirectTx(ctx, tx, prepared, CapabilityContext{}, target)
		if err == nil {
			assertDraftRecoveryWarning(t, capability, result)
			if result.Status != "accepted" || stringValue(mapValue(result.Output)["draft_ref"]) != ref || stringValue(mapValue(result.Output)["reply_status"]) != "needs_action" {
				t.Fatalf("wrong canonical media receipt: %+v", result)
			}
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	inv.Arguments = jsonBytes(map[string]any{"draft_ref": "reply_draft_foreign", "action_result_refs": []any{"foreign_media_secret"}})
	if _, _, err := service.prepare(ctx, inv, run); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("foreign action resource accepted: %v", err)
	}
	if writerCalls != 2 {
		t.Fatal("foreign action caused generation")
	}
	inv.Arguments = jsonBytes(map[string]any{"draft_ref": "reply_draft_foreign", "action_result_refs": []any{replyActionRef(ref, 0)}})
	pending, got, err := service.prepare(ctx, inv, run)
	if err != nil || got != ref || len(pending.MediaRequests) != 1 || writerCalls != 2 {
		t.Fatalf("pending canonical draft failed: %+v %v", pending, err)
	}
	// Controlled durable waiting state; no image generation service is invoked.
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_reply_actions SET status='bound' WHERE draft_id=$1`, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_reply_drafts SET status='waiting_media' WHERE id=$1`, ref); err != nil {
		t.Fatal(err)
	}
	waiting, err := capability.Prepare(ctx, inv, CapabilityContext{Life: &CurrentLifeContext{Data: map[string]any{}}})
	if err != nil || writerCalls != 2 {
		t.Fatalf("canonical waiting recovery regenerated: %v calls=%d", err, writerCalls)
	}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		result, err := capability.ExecuteDirectTx(ctx, tx, waiting, CapabilityContext{}, target)
		if err == nil {
			assertDraftRecoveryWarning(t, capability, result)
			if result.Status != "accepted" || stringValue(mapValue(result.Output)["reply_status"]) != "waiting" || stringValue(mapValue(result.Output)["draft_ref"]) != ref {
				t.Fatalf("waiting receipt lost canonical identity: %+v", result)
			}
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_reply_actions SET status='failed',error_code='canonical_media_failed' WHERE draft_id=$1`, ref); err != nil {
		t.Fatal(err)
	}
	adjusted, got, err := service.prepare(ctx, inv, run)
	if err != nil || got != ref || adjusted.Text != "这次图片失败了，我们继续聊。" || len(adjusted.AttachmentRefs) != 0 || writerCalls != 3 {
		t.Fatalf("canonical adjustment: %+v %v calls=%d", adjusted, err, writerCalls)
	}
	if _, _, err := service.prepare(ctx, inv, run); err != nil || writerCalls != 3 {
		t.Fatalf("ready replay regenerated: %v calls=%d", err, writerCalls)
	}
}

func TestReplyDraftRecoveryNativeBoundary(t *testing.T) {
	writerCalls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		writerCalls++
		return fakeProviderResult{Text: "原生边界的当前回复。"}
	})
	ctx, repo, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "draft-native", router)
	ctx, run, inv := draftRecoveryInvocation(t, ctx, app, owner, self, conversation, "draft-native-source")
	registry, err := app.privateReplyRegistry(run)
	if err != nil {
		t.Fatal(err)
	}
	ctx = withCapabilityRunRegistry(ctx, registry)
	projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, SourceFactID: inv.SourceFactID, FluctlightID: self, ConversationID: conversation, CurrentUserText: run.CurrentInput, MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
	if err != nil {
		t.Fatal(err)
	}
	codec, err := newProviderContextRefCodec(projection.ReferenceIndex)
	if err != nil {
		t.Fatal(err)
	}
	trace := &ADKCapabilityTrace{}
	invoker := newAppADKCapabilityInvokerForContext(ctx, app, ADKCapabilityRequest{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, SourceFactID: inv.SourceFactID, OperationID: "draft-native-root", CorrelationID: inv.Metadata.CorrelationID, Surface: CapabilitySurfaceConversation, Projection: projection}, trace)
	ctx = withADKCapabilityContextRefs(ctx, invoker, trace, nil, codec)
	trace.RecordModelToolCalls("draft-native-physical", 1, []schema.ToolCall{{ID: "native-draft-call"}})
	output, err := invoker.(interface {
		ExecuteWithID(context.Context, string, string, string) (string, error)
	}).ExecuteWithID(ctx, "native-draft-call", conversationReplyCapabilityName, `{"draft_ref":"cr_ffffffffffff"}`)
	if err != nil || !strings.Contains(output, "committed") || !strings.Contains(output, "reply_draft_hint_ignored") || writerCalls != 1 {
		t.Fatalf("native optional hint did not reach reconciliation: %s err=%v calls=%d", output, err, writerCalls)
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count); err != nil || count != 1 {
		t.Fatalf("native did not publish exactly once: %d %v", count, err)
	}
}

func TestReplyDraftRecoveryClosedSourceCannotPublish(t *testing.T) {
	writerCalls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		writerCalls++
		return fakeProviderResult{Text: "当前草稿回复。"}
	})
	ctx, repo, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "draft-closed", router)
	ctx, run, inv := draftRecoveryInvocation(t, ctx, app, owner, self, conversation, "draft-closed-source")
	inv.Arguments = jsonBytes(map[string]any{"draft_ref": "reply_draft_foreign"})
	capability := factOnlyReplyCapability{service: newFactOnlyReplyService(app), run: run, publication: NewToolPublicationService(app)}
	prepared, err := capability.Prepare(ctx, inv, CapabilityContext{Life: &CurrentLifeContext{Data: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET status='failed' WHERE id=$1`, inv.SourceFactID); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		_, err := capability.ExecuteDirectTx(ctx, tx, prepared, CapabilityContext{}, DirectToolTarget{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation})
		return err
	}); err == nil {
		t.Fatal("closed source published")
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count); err != nil || count != 0 || writerCalls != 1 {
		t.Fatalf("closed source produced effect: %d calls=%d err=%v", count, writerCalls, err)
	}
}

func TestReplyDraftRecoveryDatabaseFailureRemainsFailure(t *testing.T) {
	writerCalls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		writerCalls++
		return fakeProviderResult{Text: "不应生成"}
	})
	ctx, repo, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "draft-db-failure", router)
	ctx, run, inv := draftRecoveryInvocation(t, ctx, app, owner, self, conversation, "draft-db-source")
	inv.Arguments = jsonBytes(map[string]any{"draft_ref": "reply_draft_foreign"})
	repo.Pool().Close()
	_, _, err := newFactOnlyReplyService(app).prepare(ctx, inv, run)
	if err == nil || errors.Is(err, ErrUnauthorized) || writerCalls != 0 {
		t.Fatalf("DB failure hidden or relabelled: %v calls=%d", err, writerCalls)
	}
}
