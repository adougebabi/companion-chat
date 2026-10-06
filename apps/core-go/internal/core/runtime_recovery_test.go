package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
)

func TestRequiredWorkingMemoryBorrowsCapacityWithoutRaisingWireLimit(t *testing.T) {
	input := WorkingMemoryInput{RuntimeFacts: []PromptFragment{
		{Kind: PromptFragmentRuntimeFact, Priority: 999, EstimatedTokens: 1000, Content: "optional", SourceRefs: []string{"optional"}},
		{Kind: PromptFragmentRuntimeFact, Priority: 1, Required: true, EstimatedTokens: 7000, Content: "actual current state", SourceRefs: []string{"required"}},
	}}
	policy, err := reserveRequiredWorkingMemory(input, DefaultWorkingMemoryPolicy(), 8000)
	if err != nil || policy.RuntimeFactTokens != 7000 {
		t.Fatalf("required state cannot use available wire capacity: %#v err=%v", policy, err)
	}
	selected, err := ResolveWorkingMemory(input, policy)
	if err != nil || len(selected.RuntimeFacts) != 1 || !selected.RuntimeFacts[0].Required {
		t.Fatalf("optional priority crowded out mandatory current state: %#v err=%v", selected, err)
	}
	if _, err := reserveRequiredWorkingMemory(input, DefaultWorkingMemoryPolicy(), 6000); !errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("required state bypassed total limit: %v", err)
	}
}

func TestADKRepairsUnknownInfluenceWithoutReplayingTools(t *testing.T) {
	unknown := "appearance:ctx_abcdef0123456789abcdef0123456789"
	codec, err := newProviderContextRefCodec(ContextReferenceIndex{ByRef: map[string]ContextReference{}})
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		result := map[string]any{"influences": []any{}}
		if requests.Add(1) == 1 {
			result["influences"] = []any{map[string]any{"ref": unknown, "role": "grounds", "confidence": 0.8, "note": "fabricated reference"}}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jsonBytes(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(result)}}}}))
	}))
	defer server.Close()
	invoker := &adkResultInvoker{results: map[string]string{}}
	ctx := withADKCapabilityContextRefs(context.Background(), invoker, &ADKCapabilityTrace{}, nil, codec)
	response, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, EinoModelCall{
		Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fixture", Timeout: 10 * time.Second, TokenBudget: 1000},
		Role:       "cognitive_assessment", Scenario: "conversation", Messages: []map[string]any{{"role": "user", "content": "only current scoped refs"}},
		JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: objectSchema(map[string]any{"influences": decisionInfluencesSchema()}, []string{"influences"}, false),
		ProviderRequestID: "unknown-ref-repair", CorrelationID: "unknown-ref-repair",
	})
	if err != nil || requests.Load() != 2 || len(invoker.calls) != 0 || response.Message == nil || !strings.Contains(response.Message.Content, `"influences":[]`) {
		t.Fatalf("unknown ref was not corrected in a tool-free final repair: requests=%d calls=%v response=%#v err=%v", requests.Load(), invoker.calls, response, err)
	}
}

func TestADKStaleReplyRefreshesContextWithoutPublishingOldProse(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	projectionRequest := ContextProjectionRequest{AuthorizationActorID: f.ownerID, SpeakerActorID: f.ownerID, FluctlightID: f.fluctlightID, ConversationID: f.conversationID, SourceFactID: "stale-reply-source", CurrentUserText: "在哪里", MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact}
	projection, err := f.app.BuildContextProjectionFor(f.ctx, projectionRequest)
	if err != nil {
		t.Fatal(err)
	}
	trace := &ADKCapabilityTrace{}
	refresh := &runtimeContextRefresh{base: projection, refresh: func(ctx context.Context) (modelContextRefreshContent, error) {
		updated, err := f.app.BuildContextProjectionFor(ctx, projectionRequest)
		return modelContextRefreshContent{System: "system", Runtime: "[RUNTIME CONTEXT]\n" + stringValue(updated.LifeContext["scene"]) + "\n[/RUNTIME CONTEXT]", Projection: updated}, err
	}}
	initial := []*schema.Message{{Role: schema.System, Content: "system"}, {Role: schema.User, Content: "[RUNTIME CONTEXT]\nold scene\n[/RUNTIME CONTEXT]"}}
	if _, err := refresh.prepare(f.ctx, initial); err != nil {
		t.Fatal(err)
	}
	invoker := newAppADKCapabilityInvoker(f.app, ADKCapabilityRequest{AuthorizationActorID: f.ownerID, FluctlightID: f.fluctlightID, ConversationID: f.conversationID, SourceFactID: "stale-reply-source", OperationID: "stale-reply", Surface: CapabilitySurfaceConversation, Projection: projection}, trace)
	ctx := withADKCapabilityContext(f.ctx, invoker, trace, refresh)
	var bootsID string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND category='boots'`, f.fluctlightID).Scan(&bootsID); err != nil {
		t.Fatal(err)
	}
	trace.RecordModelToolCalls("model-prior-effect", 1, []schema.ToolCall{{ID: "prior-wear", Function: schema.FunctionCall{Name: wardrobeWearCapabilityName}}})
	if result, err := invoker.(ADKCapabilityInvokerWithID).ExecuteWithID(ctx, "prior-wear", wardrobeWearCapabilityName, jsonString(map[string]any{"mode": "partial", "item_ids": []string{bootsID}})); err != nil || !strings.Contains(result, `"status":"completed"`) {
		t.Fatalf("prior effect failed: %s %v", result, err)
	}
	at := f.app.now().UTC()
	if _, err := f.app.CreateLifeEvent(f.ctx, f.ownerID, f.fluctlightID, map[string]any{
		"kind": "owner_event", "start_at": at.Add(-time.Second).Format(time.RFC3339Nano), "end_at": at.Add(time.Minute).Format(time.RFC3339Nano), "scene": "商店", "activity": "试衣",
		"evidence_refs": []any{"owner:test-change"}, "idempotency_key": "external-scene-" + f.suffix, "expected_life_context_revision": projection.LifeContextRevision,
	}); err != nil {
		t.Fatal(err)
	}
	trace.RecordModelToolCalls("model-stale", 2, []schema.ToolCall{{ID: "old-reply", Function: schema.FunctionCall{Name: conversationReplyCapabilityName}}})
	result, err := invoker.(ADKCapabilityInvokerWithID).ExecuteWithID(ctx, "old-reply", conversationReplyCapabilityName, `{"text":"我还在家"}`)
	if err != nil || !strings.Contains(result, "life_context_stale") {
		t.Fatalf("stale prose did not return recoverable feedback: %s err=%v", result, err)
	}
	updated, err := refresh.prepare(ctx, initial)
	if err != nil || !strings.Contains(updated[1].Content, "商店") {
		t.Fatalf("stale failure did not refresh current scene: %#v err=%v", updated, err)
	}
	trace.RecordModelToolCalls("model-refreshed", 3, []schema.ToolCall{{ID: "new-reply", Function: schema.FunctionCall{Name: conversationReplyCapabilityName}}})
	result, err = invoker.(ADKCapabilityInvokerWithID).ExecuteWithID(ctx, "new-reply", conversationReplyCapabilityName, `{"text":"我现在在商店"}`)
	if err != nil || !strings.Contains(result, `"status":"completed"`) {
		t.Fatalf("refreshed reply did not commit: %s err=%v", result, err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND text='我还在家'`, f.conversationID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("stale reply was published: count=%d err=%v", count, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND text='我现在在商店'`, f.conversationID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("refreshed reply not published once: count=%d err=%v", count, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, f.fluctlightID, bootsID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("recovery lost prior committed wearing: %d %v", count, err)
	}
	outcome, err := committedAgentOutcome(trace)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, _, _, _, err := f.app.agentSettlementAuthorityRevisionsTx(ctx, tx, f.fluctlightID, *refresh.latestProjection(), outcome)
		return err
	}); err != nil {
		t.Fatalf("refreshed decision still settles against obsolete authority: %v", err)
	}
	unrecovered := *refresh.latestProjection()
	unrecovered.AuthorityAtRunStart = &CognitionAuthorityAtRunStart{Foundation: projection.ContextRevision, CurrentState: projection.CurrentStateRevision, CurrentFacts: projection.CurrentFactsRevision, LifeContext: projection.LifeContextRevision}
	if err := withTransaction(ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, _, _, _, err := f.app.agentSettlementAuthorityRevisionsTx(ctx, tx, f.fluctlightID, unrecovered, outcome)
		return err
	}); err == nil {
		t.Fatal("ordinary settlement bypassed intervening external state change")
	}
	forged := *refresh.latestProjection()
	anchor := *forged.AuthorityAtRunStart
	anchor.StaleReplyCallID = "not-a-rejected-reply"
	forged.AuthorityAtRunStart = &anchor
	if err := withTransaction(ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, _, _, _, err := f.app.agentSettlementAuthorityRevisionsTx(ctx, tx, f.fluctlightID, forged, outcome)
		return err
	}); err == nil {
		t.Fatal("invalid recovery boundary bypassed settlement guard")
	}
}

func TestPhysicalBudgetCompactionPreservesCurrentStateAndToolProtocol(t *testing.T) {
	model := &queuedToolCallingChatModel{assignment: providerAssignment{TokenBudget: 1000, ContextWindowTokens: 12000, MaxInputTokens: 5000, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}}
	call := &schema.Message{Role: schema.Assistant, ReasoningContent: strings.Repeat("旧推理", 3000), ToolCalls: []schema.ToolCall{{ID: "wear", Type: "function", Function: schema.FunctionCall{Name: "wardrobe.wear", Arguments: `{}`}}}}
	result := &schema.Message{Role: schema.Tool, ToolCallID: "wear", Content: `{"status":"completed","items":["borrowed-shirt"]}`}
	input := []*schema.Message{{Role: schema.System, Content: "trusted"}, {Role: schema.User, Content: "[RUNTIME CONTEXT]\nactual wearing\n[/RUNTIME CONTEXT]"}, {Role: schema.User, Content: "拍借来的衣服"}, call, result}
	compact, err := model.preparePhysicalInput(context.Background(), input)
	if err != nil || model.enforcePhysicalInputBudget(context.Background(), compact) != nil {
		t.Fatalf("safe reasoning compaction failed: %v", err)
	}
	if len(compact) != len(input) || compact[1].Content != input[1].Content || compact[2].Content != input[2].Content || compact[3].ToolCalls[0].ID != compact[4].ToolCallID || compact[4].Content != result.Content || call.ReasoningContent == "" {
		t.Fatalf("compaction mutated Eino history/current input/Tool result: %#v", compact)
	}
}

func TestPhysicalBudgetDropsOnlyCompleteHistoricalTurnsAndKeepsOversizeLatestResultFailure(t *testing.T) {
	model := &queuedToolCallingChatModel{assignment: providerAssignment{TokenBudget: 1000, ContextWindowTokens: 12000, MaxInputTokens: 5000, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}}
	input := []*schema.Message{
		{Role: schema.System, Content: "trusted"},
		{Role: schema.User, Content: "[RUNTIME CONTEXT]\ncurrent state\n[/RUNTIME CONTEXT]"},
		{Role: schema.User, Content: strings.Repeat("旧聊天", 3000)}, {Role: schema.Assistant, Content: "old answer"},
		{Role: schema.User, Content: "current correction"},
	}
	compact, err := model.preparePhysicalInput(context.Background(), input)
	if err != nil || len(compact) != 3 || compact[2].Content != "current correction" || len(input) != 5 {
		t.Fatalf("history not dropped as complete optional turn: %#v err=%v", compact, err)
	}
	compact = append(compact, &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "latest", Function: schema.FunctionCall{Name: "wardrobe.inspect", Arguments: `{}`}}}}, &schema.Message{Role: schema.Tool, ToolCallID: "latest", Content: strings.Repeat("必需结果", 2000)})
	protected, err := model.preparePhysicalInput(context.Background(), compact)
	if err != nil || len(protected) != len(compact) || !errors.Is(model.enforcePhysicalInputBudget(context.Background(), protected), ErrPromptToolResultBudgetExceeded) {
		t.Fatalf("oversize required result silently discarded: %#v err=%v", protected, err)
	}
}

func TestADKUnknownInfluenceEntersFinalRepairBeforeSettlement(t *testing.T) {
	known := "appearance:ctx_0123456789abcdef0123456789abcdef"
	unknown := "appearance:ctx_abcdef0123456789abcdef0123456789"
	codec, err := newProviderContextRefCodec(ContextReferenceIndex{ByRef: map[string]ContextReference{known: {Ref: known}}})
	if err != nil {
		t.Fatal(err)
	}
	outputSchema := objectSchema(map[string]any{"influences": decisionInfluencesSchema()}, []string{"influences"}, false)
	makeMessage := func(ref string) *schema.Message {
		return schema.AssistantMessage(jsonString(map[string]any{"influences": []any{map[string]any{"ref": ref, "role": "grounds", "confidence": 0.8, "note": "current appearance"}}}), nil)
	}
	if err := validateADKFinalContract(makeMessage(unknown), "cognitive_assessment", outputSchema, codec); err == nil || !strings.Contains(err.Error(), "decision_influence_0_ref_unknown") {
		t.Fatalf("unknown well-shaped ref reached settlement: %v", err)
	}
	for _, ref := range []string{known, providerShortRef(known)} {
		if err := validateADKFinalContract(makeMessage(ref), "cognitive_assessment", outputSchema, codec); err != nil {
			t.Fatalf("known scoped ref rejected: %s: %v", ref, err)
		}
	}
}

func TestStaleReplyIsRecoverableFeedbackAndCancellationRemainsCancellation(t *testing.T) {
	code, retryable := publicationCapabilityError(ErrLifeContextStale, "reply_publication_failed")
	if code != "life_context_stale" || retryable {
		t.Fatalf("stale reply terminates loop as dependency failure: %s/%v", code, retryable)
	}
	stage, code := classifyAgentRunFailure(&agentRunFailure{stage: "tool", code: "dependency_failed", cause: context.Canceled})
	if stage != "cancellation" || code != "request_cancelled" {
		t.Fatalf("cancellation became tool failure: %s/%s", stage, code)
	}
}
