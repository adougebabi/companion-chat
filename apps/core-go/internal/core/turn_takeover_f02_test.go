package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// F-02: Judge 前候选校验必须覆盖权限和冻结上下文约束
// ---------------------------------------------------------------------------
//
// request.md:79/163/296 要求：仲裁前验证结构、工具存在性、参数合法性、
// 权限及基本约束；非法候选不能靠 B 接管掩盖。validateCandidateCapability
// Invocations 在 A 路径（mutations.go）与 B 路径（turn_takeover.go）共用，
// 做确定性、无副作用的授权检查：冻结 surface/身份所有权、声明 target
// kinds、RequiredContext 的 snapshot 解码，以及 Capability 自己的纯候选
// hook。任何依赖网络/DB/context-resolver 的 Preflight 留在 winner-only
// Prepare。

// unauthorizedSurfaceSpyCapability declares a surface that does not authorize
// the conversation candidate surface, so a Provider result that names it is
// structurally valid but unauthorized.
type unauthorizedSurfaceSpyCapability struct{}

func (unauthorizedSurfaceSpyCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "test.unauthorized.surface", Version: "v1", Type: CapabilityTypeAction,
		Description:     "Prove an unauthorized surface is rejected before the Judge.",
		InputSchema:     map[string]any{"type": "object"},
		OutputSchema:    map[string]any{"type": "object"},
		Surfaces:        []CapabilitySurface{CapabilitySurfaceWakeUp},
		SideEffectClass: "external_async", ConcurrencyClass: "exclusive",
		FailurePolicy: FailurePolicyOptionalInternal,
	}
}

func (unauthorizedSurfaceSpyCapability) RequiredContext() []ContextSlot { return nil }
func (unauthorizedSurfaceSpyCapability) Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error) {
	return CapabilityResult{}, nil
}

// unauthorizedTargetKindSpyCapability declares a conversation surface but a
// narrow target kind allowlist, so an output binding with a foreign target
// kind is rejected before the Judge.
type unauthorizedTargetKindSpyCapability struct{}

func (unauthorizedTargetKindSpyCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "test.unauthorized.target", Version: "v1", Type: CapabilityTypeAction,
		Description:     "Prove an unauthorized output target kind is rejected before the Judge.",
		InputSchema:     map[string]any{"type": "object"},
		OutputSchema:    map[string]any{"type": "object"},
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation},
		TargetKinds:     []string{"conversation_message"},
		OutputRole:      "conversation_message",
		SideEffectClass: "external_async", ConcurrencyClass: "exclusive",
		FailurePolicy: FailurePolicyOptionalInternal,
	}
}

func (unauthorizedTargetKindSpyCapability) RequiredContext() []ContextSlot { return nil }
func (unauthorizedTargetKindSpyCapability) Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error) {
	return CapabilityResult{}, nil
}

// appWithExtraCapabilities rebuilds the registry/runtime of a test app so the
// extra capabilities participate in validation without changing the builtin
// set the Provider catalog already exposes.
func appWithExtraCapabilities(t *testing.T, app *App, extras ...Capability) *App {
	t.Helper()
	if len(extras) == 0 {
		return app
	}
	entries := append(builtinCapabilities(app), extras...)
	registry, err := NewCapabilityRegistry(entries...)
	if err != nil {
		t.Fatalf("rebuild capability registry: %v", err)
	}
	app.Capabilities = registry
	runtime, err := NewCapabilityRuntime(registry, app.ContextResolver)
	if err != nil {
		t.Fatalf("rebuild capability runtime: %v", err)
	}
	app.Runtime = runtime
	return app
}

// TestValidateCandidateCapabilityInvocationsAuthorizesDeterministically
// exercises the F-02 authorization gate directly: a structurally valid
// invocation that is unauthorized (surface, fluctlight ownership, conversation
// ownership, or declared target kind) must fail with ErrUnauthorized, while the
// authorized baseline passes.
func TestValidateCandidateCapabilityInvocationsAuthorizesDeterministically(t *testing.T) {
	app := &App{}
	entries := append(builtinCapabilities(app), unauthorizedSurfaceSpyCapability{}, unauthorizedTargetKindSpyCapability{})
	registry, err := NewCapabilityRegistry(entries...)
	if err != nil {
		t.Fatalf("build capability registry: %v", err)
	}
	app.Capabilities = registry

	base := CapabilityInvocation{
		CallID: "call-1", CapabilityName: "conversation.reply",
		SchemaVersion: CapabilityInvocationSchemaVersion,
		SourceFactID:  "fact-1", ProviderRequestID: "req-1",
		Arguments: json.RawMessage(`{"text":"hi"}`),
		Metadata: InvocationMetadata{
			Surface: CapabilitySurfaceConversation, FluctlightID: "fl-1", ConversationID: "conv-1",
		},
	}
	candidate := candidateValidationContext{
		FluctlightID: "fl-1", ConversationID: "conv-1", SourceFactID: "fact-1", Surface: CapabilitySurfaceConversation,
		ContextSnapshot: map[string]any{
			"identity":     map[string]any{"fluctlight_id": "fl-1", "conversation_id": "conv-1", "source_fact_id": "fact-1"},
			"current_life": map[string]any{"scene": "studio"},
		},
	}

	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{base}, candidate); err != nil {
		t.Fatalf("a valid candidate must pass the authorization gate: %v", err)
	}

	surfaceBad := base
	surfaceBad.CallID = "call-surface"
	surfaceBad.CapabilityName = "test.unauthorized.surface"
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{surfaceBad}, candidate); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("an unauthorized surface must fail before the Judge, got %v", err)
	}

	fluctlightBad := base
	fluctlightBad.CallID = "call-fluctlight"
	fluctlightBad.Metadata.FluctlightID = "rogue-fluctlight"
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{fluctlightBad}, candidate); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("a foreign fluctlight must fail before the Judge, got %v", err)
	}

	conversationBad := base
	conversationBad.CallID = "call-conversation"
	conversationBad.Metadata.ConversationID = "rogue-conversation"
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{conversationBad}, candidate); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("a foreign conversation must fail before the Judge, got %v", err)
	}

	targetBad := base
	targetBad.CallID = "call-target"
	targetBad.CapabilityName = "test.unauthorized.target"
	targetBad.Metadata.OutputBinding = &OutputBindingV1{TargetKind: "moment", TargetRef: "mom-1"}
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{targetBad}, candidate); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("an undeclared target kind must fail before the Judge, got %v", err)
	}

	targetOk := targetBad
	targetOk.CallID = "call-target-ok"
	targetOk.Metadata.OutputBinding = &OutputBindingV1{TargetKind: "conversation_message", TargetRef: "msg-1"}
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{targetOk}, candidate); err != nil {
		t.Fatalf("a declared target kind must pass the authorization gate: %v", err)
	}
}

// TestUnauthorizedCandidateAFailsBeforeTheJudge proves the F-02 requirement
// (request.md:79/296): a structurally valid A that proposes an unauthorized
// surface must fail before the Judge is consulted, so the illegal invocation is
// never hidden behind a takeover.
func TestUnauthorizedCandidateAFailsBeforeTheJudge(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-unauth-a-owner", "chain-unauth-a-fluctlight", "chain-unauth-a-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	// The candidate pairs a legal conversation.reply (the visible text carrier)
	// with a tool call whose declared surface does not authorize the
	// conversation candidate surface. Schema is valid; authorization is not.
	candidate := fakeProviderResult{
		Structured: map[string]any{
			"response_mode": "final", "action_type": "reply", "response_intent": "reply",
			"visible_text": "星火的越权候选", "tool_calls": []any{}, "influences": []any{},
		},
		ToolCalls: []map[string]any{
			{"id": "a-reply", "type": "function", "function": map[string]any{"name": "conversation.reply", "arguments": `{"text":"星火的越权候选"}`}},
			{"id": "a-spy", "type": "function", "function": map[string]any{"name": "test.unauthorized.surface", "arguments": "{}"}},
		},
	}
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(candidate)).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)
	app = appWithExtraCapabilities(t, app, unauthorizedSurfaceSpyCapability{})

	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "确认一下。", "chain-unauth-a-turn", "chain-unauth-a-turn-1")); err == nil {
		t.Fatal("an unauthorized candidate must fail the turn")
	}

	if count := router.requestCount(takeoverJudgeSchemaName); count != 0 {
		t.Fatalf("an unauthorized candidate must fail before the Judge, got %d judge calls", count)
	}
	if count := router.requestCount(takeoverReplySchemaName); count != 0 {
		t.Fatalf("an unauthorized candidate must not reach the takeover generation, got %d", count)
	}
	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-unauth-a-turn-1"); len(texts) != 0 {
		t.Fatalf("an unauthorized candidate must not be delivered: %#v", texts)
	}
	frozen := takeoverChainFrozenPayload(t, ctx, repository, "chain-unauth-a-turn")
	if stage := turnStageOf(frozen); stage == turnStageWinnerReady {
		t.Fatalf("an unauthorized candidate must never become executable: %#v", frozen[turnStagePayloadKey])
	}
}

// TestUnauthorizedTakeoverCandidateBFailsControlled proves the B-side of F-02
// (request.md:163): after the Judge hands the turn to B, a B candidate that
// proposes an unauthorized surface is rejected before it can overwrite A, and
// neither A nor the illegal B is delivered.
func TestUnauthorizedTakeoverCandidateBFailsControlled(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-unauth-b-owner", "chain-unauth-b-fluctlight", "chain-unauth-b-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	bCandidate := fakeProviderResult{
		Structured: map[string]any{
			"response_mode": "final", "action_type": "reply", "response_intent": "reply",
			"visible_text": "暮光的越权接管", "tool_calls": []any{}, "influences": []any{},
		},
		ToolCalls: []map[string]any{
			{"id": "b-reply", "type": "function", "function": map[string]any{"name": "conversation.reply", "arguments": `{"text":"暮光的越权接管"}`}},
			{"id": "b-spy", "type": "function", "function": map[string]any{"name": "test.unauthorized.surface", "arguments": "{}"}},
		},
	}
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult("星火的候选回复", nil))).
		on(takeoverReplySchemaName, takeoverChainSequence(bCandidate)).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)
	app = appWithExtraCapabilities(t, app, unauthorizedSurfaceSpyCapability{})

	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你根本不在乎。", "chain-unauth-b-turn", "chain-unauth-b-turn-1")); err == nil {
		t.Fatal("an unauthorized takeover candidate must fail the turn")
	}

	if count := router.requestCount(takeoverReplySchemaName); count != 1 {
		t.Fatalf("the takeover generation must run once before B is rejected, got %d", count)
	}
	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-unauth-b-turn-1"); len(texts) != 0 {
		t.Fatalf("neither A nor the unauthorized B must be delivered: %#v", texts)
	}
	frozen := takeoverChainFrozenPayload(t, ctx, repository, "chain-unauth-b-turn")
	if stage := turnStageOf(frozen); stage == turnStageWinnerReady {
		t.Fatalf("an unauthorized B must never become executable: %#v", frozen[turnStagePayloadKey])
	}
}

func relationshipCandidateSnapshot(fluctlightID, conversationID, sourceFactID string, authorized []any, aliases map[string]any) map[string]any {
	return map[string]any{
		"identity": map[string]any{
			"fluctlight_id": fluctlightID, "conversation_id": conversationID, "source_fact_id": sourceFactID,
		},
		"relationship_scope": map[string]any{
			"authorized_actor_ids": authorized, "actor_aliases": aliases,
		},
	}
}

func relationshipCandidateInvocation(target string) CapabilityInvocation {
	return CapabilityInvocation{
		CallID: "relationship-call", CapabilityName: "relationship.lookup", SchemaVersion: CapabilityInvocationSchemaVersion,
		SourceFactID: "fact-1", ProviderRequestID: "provider-1", Arguments: json.RawMessage(jsonString(map[string]any{"target_actor_id": target})),
		Metadata: InvocationMetadata{Surface: CapabilitySurfaceConversation, FluctlightID: "fl-1", ConversationID: "conv-1"},
	}
}

func TestRelationshipLookupCandidateUsesOnlyFrozenScope(t *testing.T) {
	app := &App{Capabilities: mustCapabilityRegistry(relationshipLookupCapability{})}
	candidate := candidateValidationContext{
		FluctlightID: "fl-1", ConversationID: "conv-1", SourceFactID: "fact-1", Surface: CapabilitySurfaceConversation,
		ContextSnapshot: relationshipCandidateSnapshot("fl-1", "conv-1", "fact-1", []any{"human-1"}, map[string]any{"actor_user": "human-1"}),
	}

	canonical := relationshipCandidateInvocation("human-1")
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{canonical}, candidate); err != nil {
		t.Fatalf("an authorized canonical relationship target must pass: %v", err)
	}

	alias := relationshipCandidateInvocation("actor_user")
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{alias}, candidate); err != nil {
		t.Fatalf("an explicitly frozen actor alias must pass: %v", err)
	}

	foreign := relationshipCandidateInvocation("foreign-actor")
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{foreign}, candidate); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("a foreign canonical actor must fail closed, got %v", err)
	}

	emptyScope := candidate
	emptyScope.ContextSnapshot = relationshipCandidateSnapshot("fl-1", "conv-1", "fact-1", []any{}, map[string]any{"actor_user": "human-1"})
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{canonical}, emptyScope); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("an empty authorized actor set must reject a relationship row target, got %v", err)
	}

	unknownAlias := relationshipCandidateInvocation("actor_b")
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{unknownAlias}, candidate); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("an alias absent from the frozen map must fail closed, got %v", err)
	}
}

func TestCandidateValidationRequiresMatchingFrozenIdentityAndSlots(t *testing.T) {
	app := &App{Capabilities: mustCapabilityRegistry(relationshipLookupCapability{})}
	base := relationshipCandidateInvocation("human-1")
	candidate := candidateValidationContext{FluctlightID: "fl-1", ConversationID: "conv-1", SourceFactID: "fact-1", Surface: CapabilitySurfaceConversation}

	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{base}, candidate); !errors.Is(err, ErrContextResolve) {
		t.Fatalf("missing frozen snapshot must fail with ErrContextResolve, got %v", err)
	}

	candidate.ContextSnapshot = map[string]any{
		"identity":           map[string]any{"fluctlight_id": "foreign-fl", "conversation_id": "conv-1", "source_fact_id": "fact-1"},
		"relationship_scope": map[string]any{"authorized_actor_ids": []any{"human-1"}},
	}
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{base}, candidate); !errors.Is(err, ErrContextResolve) {
		t.Fatalf("mismatched frozen identity must fail with ErrContextResolve, got %v", err)
	}

	candidate.ContextSnapshot = map[string]any{
		"identity": map[string]any{"fluctlight_id": "fl-1", "conversation_id": "conv-1", "source_fact_id": "fact-1"},
	}
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{base}, candidate); !errors.Is(err, ErrContextResolve) {
		t.Fatalf("missing required relationship slot must fail with ErrContextResolve, got %v", err)
	}
}

func TestCandidateValidationUsesFrozenSurfaceAsAuthority(t *testing.T) {
	app := &App{Capabilities: mustCapabilityRegistry(relationshipLookupCapability{})}
	candidate := candidateValidationContext{
		FluctlightID: "fl-1", ConversationID: "conv-1", SourceFactID: "fact-1", Surface: CapabilitySurfaceConversation,
		ContextSnapshot: relationshipCandidateSnapshot("fl-1", "conv-1", "fact-1", []any{"human-1"}, map[string]any{"actor_user": "human-1"}),
	}
	invocation := relationshipCandidateInvocation("human-1")
	invocation.Metadata.Surface = CapabilitySurfaceWakeUp
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{invocation}, candidate); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("a provider-declared wake_up surface must not widen a conversation candidate, got %v", err)
	}
}

type candidateHookSpyCapability struct {
	candidateCalls int
	preflightCalls int
	prepareCalls   int
	executeCalls   int
	seen           CapabilityContext
}

func (spy *candidateHookSpyCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "test.candidate.hook", Version: "v1", Type: CapabilityTypeQuery,
		Description: "Observe the frozen context handed to candidate validation.",
		InputSchema: map[string]any{"type": "object", "additionalProperties": false}, OutputSchema: map[string]any{"type": "object"},
		Surfaces: []CapabilitySurface{CapabilitySurfaceConversation}, SideEffectClass: "read_only", SuccessBoundary: "query_result_available", ConcurrencyClass: "parallel",
		FailurePolicy: FailurePolicyOptionalInternal, RequiredContext: []ContextSlot{SlotCurrentLife},
	}
}

func (spy *candidateHookSpyCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotCurrentLife}
}
func (spy *candidateHookSpyCapability) ValidateCandidate(_ context.Context, _ CapabilityInvocation, resolved CapabilityContext) error {
	spy.candidateCalls++
	spy.seen = resolved
	return nil
}
func (spy *candidateHookSpyCapability) Preflight(context.Context, CapabilityContext) error {
	spy.preflightCalls++
	return nil
}
func (spy *candidateHookSpyCapability) Prepare(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityInvocation, error) {
	spy.prepareCalls++
	return CapabilityInvocation{}, nil
}
func (spy *candidateHookSpyCapability) Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error) {
	spy.executeCalls++
	return CapabilityResult{}, nil
}

func TestCandidateValidationCallsOnlyPureHookWithFrozenContext(t *testing.T) {
	spy := &candidateHookSpyCapability{}
	app := &App{Capabilities: mustCapabilityRegistry(spy)}
	invocation := CapabilityInvocation{
		CallID: "hook-call", CapabilityName: "test.candidate.hook", SchemaVersion: CapabilityInvocationSchemaVersion,
		SourceFactID: "fact-1", ProviderRequestID: "provider-1", Arguments: json.RawMessage(`{}`),
		Metadata: InvocationMetadata{Surface: CapabilitySurfaceConversation, FluctlightID: "fl-1", ConversationID: "conv-1"},
	}
	candidate := candidateValidationContext{
		FluctlightID: "fl-1", ConversationID: "conv-1", SourceFactID: "fact-1", Surface: CapabilitySurfaceConversation,
		ContextSnapshot: map[string]any{
			"identity":     map[string]any{"fluctlight_id": "fl-1", "conversation_id": "conv-1", "source_fact_id": "fact-1"},
			"current_life": map[string]any{"scene": "frozen-studio"},
		},
	}
	if err := app.validateCandidateCapabilityInvocations([]CapabilityInvocation{invocation}, candidate); err != nil {
		t.Fatalf("candidate hook validation failed: %v", err)
	}
	if spy.candidateCalls != 1 || spy.preflightCalls != 0 || spy.prepareCalls != 0 || spy.executeCalls != 0 {
		t.Fatalf("candidate validation called the wrong seams: candidate=%d preflight=%d prepare=%d execute=%d", spy.candidateCalls, spy.preflightCalls, spy.prepareCalls, spy.executeCalls)
	}
	if spy.seen.Life == nil || stringValue(spy.seen.Life.Data["scene"]) != "frozen-studio" {
		t.Fatalf("candidate hook did not receive the frozen life slot: %#v", spy.seen)
	}
	if spy.seen.Identity.FluctlightID != "fl-1" || spy.seen.Identity.ConversationID != "conv-1" || spy.seen.Identity.SourceFactID != "fact-1" {
		t.Fatalf("candidate hook received the wrong frozen identity: %#v", spy.seen.Identity)
	}
}

// TestUnauthorizedRelationshipCandidateAFailsBeforeTheJudge proves the real
// registered relationship.lookup permission hook runs before arbitration. Its
// arguments satisfy the schema, but the target actor is outside the frozen
// conversation scope, so a Judge approval cannot mask the illegal A.
func TestUnauthorizedRelationshipCandidateAFailsBeforeTheJudge(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-rel-unauth-owner", "chain-rel-unauth-fluctlight", "chain-rel-unauth-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	candidate := fakeProviderResult{
		Structured: map[string]any{
			"response_mode": "final", "action_type": "reply", "response_intent": "reply",
			"visible_text": "星火的关系越权候选", "tool_calls": []any{}, "influences": []any{},
		},
		ToolCalls: []map[string]any{
			{"id": "a-reply", "type": "function", "function": map[string]any{"name": "conversation.reply", "arguments": `{"text":"星火的关系越权候选"}`}},
			{"id": "a-relationship", "type": "function", "function": map[string]any{"name": "relationship.lookup", "arguments": `{"target_actor_id":"foreign-actor"}`}},
		},
	}
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(candidate)).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)

	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "确认一下。", "chain-rel-unauth-turn", "chain-rel-unauth-turn-1")); err == nil {
		t.Fatal("a relationship target outside the frozen scope must fail the turn")
	}
	if count := router.requestCount(takeoverJudgeSchemaName); count != 0 {
		t.Fatalf("an unauthorized relationship candidate must fail before the Judge, got %d judge calls", count)
	}
	if count := router.requestCount(takeoverReplySchemaName); count != 0 {
		t.Fatalf("an unauthorized relationship candidate must not reach takeover generation, got %d", count)
	}
	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-rel-unauth-turn-1"); len(texts) != 0 {
		t.Fatalf("an unauthorized relationship candidate must not be delivered: %#v", texts)
	}
	frozen := takeoverChainFrozenPayload(t, ctx, repository, "chain-rel-unauth-turn")
	if stage := turnStageOf(frozen); stage == turnStageWinnerReady {
		t.Fatalf("an unauthorized relationship candidate must never become executable: %#v", frozen[turnStagePayloadKey])
	}
}
