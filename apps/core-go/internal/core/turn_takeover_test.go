package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
)

// ---------------------------------------------------------------------------
// Canonical visible text (R05 / F02)
// ---------------------------------------------------------------------------

func invocationWithText(t *testing.T, name, text string) CapabilityInvocation {
	t.Helper()
	return CapabilityInvocation{
		CallID: "call-" + name, CapabilityName: name,
		Arguments: json.RawMessage(jsonString(map[string]any{"text": text})),
	}
}

func TestResolveCanonicalVisibleReplyPrefersResponsePlanAndReportsConflict(t *testing.T) {
	registry := capabilityRegistryForTest(t)
	canonical, diagnostics := resolveCanonicalVisibleReply(
		map[string]any{"visible_text": "来自 response_plan 的文本"},
		map[string]any{"visible_text": "来自 root 的不同文本"},
		[]CapabilityInvocation{invocationWithText(t, "conversation.reply", "来自 reply 参数的第三种文本")},
		registry,
	)
	if canonical.Text != "来自 response_plan 的文本" {
		t.Fatalf("the historical precedence changed: %#v", canonical)
	}
	if canonical.Source != canonicalVisibleSourceResponsePlan {
		t.Fatalf("the winning source is wrong: %#v", canonical)
	}
	if !canonical.Conflict {
		t.Fatalf("three disagreeing sources must be reported as a conflict: %#v", canonical)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != canonicalVisibleTextConflictCode {
		t.Fatalf("the conflict diagnostic is missing: %#v", diagnostics)
	}
	if diagnostics[0].Digest != canonical.Digest || canonical.Digest == "" {
		t.Fatalf("the diagnostic must bind to the frozen digest: %#v %#v", diagnostics, canonical)
	}
}

func TestResolveCanonicalVisibleReplyFallsBackToReplyCapability(t *testing.T) {
	registry := capabilityRegistryForTest(t)
	canonical, diagnostics := resolveCanonicalVisibleReply(
		map[string]any{},
		map[string]any{},
		[]CapabilityInvocation{invocationWithText(t, "conversation.reply", "只有 reply 参数带文本")},
		registry,
	)
	if canonical.Text != "只有 reply 参数带文本" || canonical.Source != canonicalVisibleSourceReplyCapability {
		t.Fatalf("the reply fallback is wrong: %#v", canonical)
	}
	if canonical.Conflict || len(diagnostics) != 0 {
		t.Fatalf("a single source must not be reported as a conflict: %#v %#v", canonical, diagnostics)
	}
}

func TestResolveCanonicalVisibleReplyUsesCanonicalReplyNameWhenRegistryOmitsIt(t *testing.T) {
	// A provider response already carries the canonical capability name. The
	// visible reply must not disappear merely because an incomplete runtime
	// catalog omitted the output-role metadata for this built-in slot.
	registry := mustCapabilityRegistry(memoryEventCapability{}, imageGenerateCapability{})
	canonical, diagnostics := resolveCanonicalVisibleReply(
		map[string]any{},
		map[string]any{},
		[]CapabilityInvocation{invocationWithText(t, "conversation.reply", "直接发送这段私聊内容")},
		registry,
	)
	if canonical.Text != "直接发送这段私聊内容" || canonical.Source != canonicalVisibleSourceReplyCapability {
		t.Fatalf("the canonical reply name was not used: %#v", canonical)
	}
	if canonical.Conflict || len(diagnostics) != 0 {
		t.Fatalf("a single canonical reply source must not be reported as a conflict: %#v %#v", canonical, diagnostics)
	}
}

func TestResolveCanonicalVisibleReplyTreatsEqualSourcesAsAgreement(t *testing.T) {
	registry := capabilityRegistryForTest(t)
	canonical, diagnostics := resolveCanonicalVisibleReply(
		map[string]any{"visible_text": "同一段话"},
		map[string]any{"visible_text": "同一段话"},
		nil,
		registry,
	)
	if canonical.Conflict || len(diagnostics) != 0 {
		t.Fatalf("identical sources are not a conflict: %#v %#v", canonical, diagnostics)
	}
	if canonical.Text != "同一段话" {
		t.Fatalf("the text was lost: %#v", canonical)
	}
}

func TestResolveCanonicalVisibleReplyIgnoresBlankSources(t *testing.T) {
	registry := capabilityRegistryForTest(t)
	canonical, diagnostics := resolveCanonicalVisibleReply(
		map[string]any{"visible_text": "   "},
		map[string]any{"visible_text": "唯一有内容的来源"},
		nil,
		registry,
	)
	if canonical.Text != "唯一有内容的来源" || canonical.Source != canonicalVisibleSourceDecision {
		t.Fatalf("blank sources must not win: %#v", canonical)
	}
	if canonical.Conflict || len(diagnostics) != 0 {
		t.Fatalf("blank sources are not a conflict: %#v %#v", canonical, diagnostics)
	}
}

// ---------------------------------------------------------------------------
// Candidate preview (R05 / F02 / F05)
// ---------------------------------------------------------------------------

type previewSpyCapability struct {
	executeCalls atomic.Int32
}

func (capability *previewSpyCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "test.preview.spy", Version: "v1", Type: CapabilityTypeAction,
		Description: "Prove the candidate preview never executes a capability.",
		InputSchema: map[string]any{"type": "object"}, OutputSchema: map[string]any{"type": "object"},
		Surfaces:    []CapabilitySurface{CapabilitySurfaceConversation},
		TargetKinds: []string{"conversation_message"}, OutputRole: "conversation_message",
		SideEffectClass: "external_async", ConcurrencyClass: "exclusive",
		SuccessBoundary: "preview_test_never_executes", FailurePolicy: FailurePolicyOptionalInternal,
	}
}

func (capability *previewSpyCapability) RequiredContext() []ContextSlot { return nil }
func (capability *previewSpyCapability) Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error) {
	capability.executeCalls.Add(1)
	return CapabilityResult{}, nil
}
func (capability *previewSpyCapability) ExecuteTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext) (CapabilityResult, error) {
	capability.executeCalls.Add(1)
	return CapabilityResult{}, nil
}

func TestCandidatePreviewHasNoSideEffect(t *testing.T) {
	spy := &previewSpyCapability{}
	registry := previewRegistry(spy)
	canonical := canonicalVisibleReply{Text: "预览文本", Digest: stableDigest("预览文本"), Source: canonicalVisibleSourceDecision}
	invocations := []CapabilityInvocation{{
		CallID: "call-preview", CapabilityName: "test.preview.spy",
		Arguments: json.RawMessage(`{"text":"预览文本","secret_argument":"不得 dump"}`),
	}}

	preview := buildCandidatePreview(canonical, "final", "reply", invocations, registry)
	if spy.executeCalls.Load() != 0 {
		t.Fatalf("the candidate preview executed a capability %d times", spy.executeCalls.Load())
	}
	if preview.ReplyText != "预览文本" || preview.ReplyTextDigest != canonical.Digest {
		t.Fatalf("the preview must carry the frozen canonical text: %#v", preview)
	}
	if !preview.HasEffect {
		t.Fatalf("a non read-only capability must be reported as an effect: %#v", preview)
	}
	if len(preview.ActionSummary) != 1 {
		t.Fatalf("expected one action summary entry: %#v", preview.ActionSummary)
	}
	entry := preview.ActionSummary[0]
	if stringValue(entry["output_role"]) != "conversation_message" || stringValue(entry["type"]) != string(CapabilityTypeAction) {
		t.Fatalf("the summary must come from declared metadata: %#v", entry)
	}
	if serialized := jsonString(preview.ActionSummary); strings.Contains(serialized, "不得 dump") {
		t.Fatalf("the preview dumped raw capability arguments: %s", serialized)
	}
}

func TestCandidatePreviewUsesMetadataNotCapabilityName(t *testing.T) {
	// A capability whose name looks like a reply but declares a non-reply role
	// must not be treated as the reply source, and the preview must never trust
	// the name over the declared metadata.
	spy := &previewSpyCapability{}
	definition := spy.Definition()
	definition.Name = "conversation.reply"
	definition.OutputRole = "moment"
	definition.TargetKinds = []string{"moment"}
	registry := previewRegistry(&namedPreviewCapability{definition: definition})

	canonical := canonicalVisibleReply{Text: "权威文本", Digest: stableDigest("权威文本")}
	preview := buildCandidatePreview(canonical, "final", "reply", []CapabilityInvocation{{
		CallID: "call-x", CapabilityName: "conversation.reply", Arguments: json.RawMessage(`{"text":"参数里的文本"}`),
	}}, registry)
	if preview.ReplyText != "权威文本" {
		t.Fatalf("the preview re-derived the reply text from arguments: %#v", preview)
	}
	if len(preview.ActionSummary) != 1 || stringValue(preview.ActionSummary[0]["output_role"]) != "moment" {
		t.Fatalf("the preview trusted the capability name instead of its metadata: %#v", preview.ActionSummary)
	}
}

type namedPreviewCapability struct {
	definition CapabilityDefinition
}

func (capability *namedPreviewCapability) Definition() CapabilityDefinition {
	return capability.definition
}
func (capability *namedPreviewCapability) RequiredContext() []ContextSlot { return nil }
func (capability *namedPreviewCapability) Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error) {
	return CapabilityResult{}, nil
}
func (capability *namedPreviewCapability) ExecuteTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext) (CapabilityResult, error) {
	return CapabilityResult{}, nil
}

// ---------------------------------------------------------------------------
// Takeover control view (R08)
// ---------------------------------------------------------------------------

func TestTakeoverControlViewIsBoundedAndHidesOtherProfileMaterial(t *testing.T) {
	projection := multiProfileWorkingPersonaProjection()
	rule := personaSwitchRule{
		RuleID: "takeover:spark", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion,
		Condition: "遇到强烈的公开质疑时星火可强制接管", SourceProfileID: "twilight", TargetProfileID: "spark",
	}
	view := buildTakeoverControlView(rule, projection)

	if stringValue(view["rule_id"]) != "takeover:spark" || stringValue(view["target_profile_id"]) != "spark" {
		t.Fatalf("the rule identity is missing from the control view: %#v", view)
	}
	if stringValue(view["judgement_boundary"]) == "" {
		t.Fatalf("the judgement boundary must be explicit: %#v", view)
	}
	candidate := mapValue(view["handover_candidate"])
	if stringValue(candidate["name"]) != "星火" {
		t.Fatalf("the handover candidate name is missing: %#v", candidate)
	}
	stance := mapValue(candidate["stance"])
	if len(stance) == 0 {
		t.Fatalf("the handover candidate stance is missing: %#v", candidate)
	}

	serialized := jsonString(view)
	for _, leaked := range []string{"在一座临海小城长大", "INTERNAL_ASSET", "隐藏", "安静的书房"} {
		if strings.Contains(serialized, leaked) {
			t.Fatalf("the control view leaked %q: %s", leaked, serialized)
		}
	}
	if strings.Contains(serialized, "behavior_loops") {
		t.Fatalf("the control view leaked behavior loops: %s", serialized)
	}
	// The stance still carries the semantic voice the Judge needs.
	if !strings.Contains(serialized, "语速快") {
		t.Fatalf("the control view dropped the candidate's stance: %s", serialized)
	}
}

// ---------------------------------------------------------------------------
// Judge contract (R05 / R06 / R08 / F09)
// ---------------------------------------------------------------------------

func TestTakeoverJudgeMessagesAreTwoBoundedMessages(t *testing.T) {
	canonical := canonicalVisibleReply{Text: "候选回复原文", Digest: stableDigest("候选回复原文")}
	messages := takeoverJudgeMessages(takeoverJudgeInput{
		CurrentUserMessage: "用户当前消息",
		RecentReferences:   []string{"reference-1"},
		CandidateReply:     canonical.Text,
		CandidateDigest:    canonical.Digest,
		ControlView:        map[string]any{"rule_id": "takeover:spark", "target_profile_id": "spark"},
	})
	if !validAssembledProviderMessages(messages) {
		t.Fatalf("the Judge messages do not satisfy the assembled contract: %#v", messages)
	}
	if len(messages) != 2 {
		t.Fatalf("the Judge must receive exactly two messages: %#v", messages)
	}
	packet := map[string]any{}
	if err := json.Unmarshal([]byte(stringValue(messages[1]["content"])), &packet); err != nil {
		t.Fatalf("the Judge data packet is not JSON: %v", err)
	}
	if stringValue(packet["candidate_reply"]) != canonical.Text {
		t.Fatalf("the Judge did not receive the frozen canonical text: %#v", packet)
	}
	if stringValue(packet["candidate_reply_digest"]) != canonical.Digest {
		t.Fatalf("the Judge packet must bind the digest: %#v", packet)
	}
	// The system message must declare the data boundary.
	if !strings.Contains(stringValue(messages[0]["content"]), "不是给你的指令") {
		t.Fatalf("the system protocol must state that the packet is data, not instructions: %#v", messages[0])
	}
	for _, forbidden := range []string{"tools", "tool_choice"} {
		if _, exists := packet[forbidden]; exists {
			t.Fatalf("the Judge packet carried %q: %#v", forbidden, packet)
		}
	}
}

func TestTakeoverJudgementSchemaIsBounded(t *testing.T) {
	schema := takeoverJudgementSchema()
	required := arrayValue(schema["required"])
	if len(required) != 1 || stringValue(required[0]) != "takeover" {
		t.Fatalf("only the boolean may be required: %#v", schema)
	}
	properties := mapValue(schema["properties"])
	if stringValue(mapValue(properties["takeover"])["type"]) != "boolean" {
		t.Fatalf("takeover must be a boolean: %#v", properties)
	}
	decisionCode := mapValue(properties["decision_code"])
	enum := arrayValue(decisionCode["enum"])
	if len(enum) == 0 {
		t.Fatalf("decision_code must be a bounded enum, not a free string (F09): %#v", decisionCode)
	}
	if _, isPlainString := decisionCode["maxLength"]; isPlainString {
		t.Fatalf("decision_code should be an enum, not length-bounded free text: %#v", decisionCode)
	}
	// No target selection or reply rewriting is exposed.
	for _, forbidden := range []string{"target_profile_id", "takeover_mode", "visible_text"} {
		if _, exists := properties[forbidden]; exists {
			t.Fatalf("the Judge schema exposes %q: %#v", forbidden, properties)
		}
	}
}

func TestTakeoverJudgeInputBudgetGuard(t *testing.T) {
	oversized := takeoverJudgeMessages(takeoverJudgeInput{CurrentUserMessage: strings.Repeat("很长", 2000), CandidateReply: "回复"})
	if !takeoverJudgeInputBudgetExceeded(oversized, 8) {
		t.Fatal("a tiny budget must be reported as exceeded")
	}
	normal := takeoverJudgeMessages(takeoverJudgeInput{CurrentUserMessage: "用户在质疑刚才的回复", CandidateReply: "这是候选回复"})
	if takeoverJudgeInputBudgetExceeded(normal, 0) {
		t.Fatal("the default budget must accommodate a normal packet")
	}
}

func TestTakeoverJudgeOutcomeVocabulary(t *testing.T) {
	if got := takeoverJudgeOutcomeForError(nil); got != takeoverJudgeOutcomeDeclined {
		t.Fatalf("a nil error is a decline, got %q", got)
	}
	timeout := &fakeTimeoutError{}
	if got := takeoverJudgeOutcomeForError(timeout); got != takeoverJudgeOutcomeTimeout {
		t.Fatalf("a timeout must be reported as such, got %q", got)
	}
	if got := takeoverJudgeOutcomeForError(errors.New("provider request returned HTTP 503")); got != takeoverJudgeOutcomeUnavailable {
		t.Fatalf("a generic failure must be unavailable, got %q", got)
	}
}

type fakeTimeoutError struct{}

func (fakeTimeoutError) Error() string { return "context deadline exceeded while judging" }

// ---------------------------------------------------------------------------
// Provider role wiring
// ---------------------------------------------------------------------------

func TestTakeoverJudgeRoleMappingAndPriority(t *testing.T) {
	if !validProviderRole(takeoverJudgeRole) {
		t.Fatalf("%s must be a valid provider role", takeoverJudgeRole)
	}
	if got := providerBindingRole(takeoverJudgeRole); got != "generic_llm" {
		t.Fatalf("the Judge must reuse the current model binding, got %q", got)
	}
	if got := providerScenario(context.Background(), takeoverJudgeRole, ""); got != "takeover_judge" {
		t.Fatalf("the Judge scenario must be explicit, got %q", got)
	}
	if providerPriority("takeover_judge") != providerPriority("cognitive_assessment") {
		t.Fatalf("the Judge sits on the critical reply path and must not be starved: %d vs %d", providerPriority("takeover_judge"), providerPriority("cognitive_assessment"))
	}
	if providerSchemaName(takeoverJudgeRole) != takeoverJudgeSchemaName {
		t.Fatalf("the Judge schema name is wrong: %q", providerSchemaName(takeoverJudgeRole))
	}
}

// TestJudgeWirePayloadHasNoUnsupportedFields asserts against the real wire
// payload (F09): the Judge reuses the main binding but must not send tools, must
// not send enable_thinking, and must declare its own response schema.
func TestJudgeWirePayloadHasNoUnsupportedFields(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedJudgeProviderRole(t, ctx, repository, "judge-owner", "judge-fluctlight")

	router := newFakeProviderRouter().on(takeoverJudgeSchemaName, func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"takeover": false, "decision_code": "other"}}
	})
	capture := captureProviderWirePayload(router)
	app := newTestApp(t, repository, capture)

	completion, err := app.Provider.StructuredAssembledJudgement(
		ctx, takeoverJudgeRole,
		takeoverJudgeMessages(takeoverJudgeInput{CurrentUserMessage: "用户在质疑", CandidateReply: "平淡的回复"}),
		takeoverJudgeSchemaName, takeoverJudgementSchema(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if completion.Structured == nil || completion.Structured["takeover"] != false {
		t.Fatalf("the Judge completion was not parsed: %#v", completion.Structured)
	}

	payloads := capture.payloadsForSchema(takeoverJudgeSchemaName)
	if len(payloads) != 1 {
		t.Fatalf("expected exactly one judge wire payload, got %d", len(payloads))
	}
	payload := payloads[0]
	if _, exists := payload["tools"]; exists {
		t.Fatalf("the Judge request must not carry tools: %#v", payload)
	}
	if _, exists := payload["tool_choice"]; exists {
		t.Fatalf("the Judge request must not carry tool_choice: %#v", payload)
	}
	if _, exists := payload["enable_thinking"]; exists {
		t.Fatalf("enable_thinking must be omitted (the provider has no false branch): %#v", payload)
	}
	if _, exists := payload["response_format"]; !exists {
		t.Fatalf("the Judge request must declare a response_format: %#v", payload)
	}
	if stringValue(mapValue(payload["response_format"])["type"]) != "json_schema" {
		t.Fatalf("the Judge must use a strict json_schema response format: %#v", payload["response_format"])
	}
	if messages, ok := payload["messages"].([]any); !ok || len(messages) != 2 {
		t.Fatalf("the Judge wire payload must carry exactly two messages: %#v", payload["messages"])
	}
}

// TestTakeoverJudgeHasIndependentModelRun proves the Judge is metered on its
// own role, so its token/latency cost is attributable.
func TestTakeoverJudgeHasIndependentModelRun(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedJudgeProviderRole(t, ctx, repository, "judge-owner-2", "judge-fluctlight-2")

	router := newFakeProviderRouter().on(takeoverJudgeSchemaName, func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"takeover": true}}
	})
	app := newTestApp(t, repository, router)
	if _, err := app.Provider.StructuredAssembledJudgement(ctx, takeoverJudgeRole,
		takeoverJudgeMessages(takeoverJudgeInput{CurrentUserMessage: "质疑", CandidateReply: "回复"}),
		takeoverJudgeSchemaName, takeoverJudgementSchema()); err != nil {
		t.Fatal(err)
	}

	var roles, bindings []string
	rows, err := repository.Pool().Query(ctx, `SELECT DISTINCT role, binding_role FROM public.diagnostic_model_runs WHERE role=$1`, takeoverJudgeRole)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var role, binding string
		if err := rows.Scan(&role, &binding); err != nil {
			t.Fatal(err)
		}
		roles = append(roles, role)
		bindings = append(bindings, binding)
	}
	if len(roles) != 1 || roles[0] != takeoverJudgeRole {
		t.Fatalf("expected exactly one model_run role %q, got %#v", takeoverJudgeRole, roles)
	}
	if len(bindings) != 1 || bindings[0] != "generic_llm" {
		t.Fatalf("the Judge must reuse the generic_llm binding for metering, got %#v", bindings)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// capabilityRegistryForTest builds the real registry so the canonical/preview
// tests exercise the production reply capability metadata.
func capabilityRegistryForTest(t *testing.T) *CapabilityRegistry {
	t.Helper()
	app := &App{}
	registry := app.capabilityRegistry()
	if registry == nil {
		t.Fatal("capability registry is nil")
	}
	return registry
}

// previewRegistry builds a registry whose definitions are exactly the ones a
// test declares, so preview dispatch is asserted against declared metadata and
// not against the built-in catalog.
func previewRegistry(capabilities ...Capability) *CapabilityRegistry {
	registry := &CapabilityRegistry{capabilities: map[string]Capability{}, definitions: map[string]CapabilityDefinition{}}
	for _, capability := range capabilities {
		definition := capability.Definition()
		registry.capabilities[definition.Name] = capability
		registry.definitions[definition.Name] = definition
	}
	return registry
}

func seedJudgeProviderRole(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID string) {
	t.Helper()
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	endpointID := "judge-endpoint-" + fluctlightID
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://judge.invalid','judge-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'judge-model','structured_output,tool_calling',4096,10,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// F02 end-to-end: settlement writes the frozen canonical text
// ---------------------------------------------------------------------------

// TestConflictingVisibleTextSourcesFailClosed drives a full turn whose three
// visible-text sources disagree (root, response_plan, reply argument). Under
// the F-01 path b contract the model cannot produce two texts, so the
// disagreement must fail closed: the turn errors before settlement, no message
// is delivered, and the conflict is never silently resolved by precedence.
func TestConflictingVisibleTextSourcesFailClosed(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "canonical-owner", "canonical-fluctlight", "canonical-conversation"
	seedTurnConversation(t, ctx, repository, ownerID, fluctlightID, conversationID)
	seedCognitiveProviderRole(t, ctx, repository, "canonical-endpoint")

	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
		return fakeProviderResult{
			Structured: map[string]any{
				"response_mode": "final", "action_type": "reply", "response_intent": "conflict probe",
				"visible_text":  "root 级文本",
				"response_plan": map[string]any{"visible_text": "response_plan 级文本"},
				"tool_calls":    []any{}, "influences": []any{},
			},
			ToolCalls: []map[string]any{{
				"id": "canonical-reply-call", "type": "function",
				"function": map[string]any{"name": "conversation.reply", "arguments": jsonString(map[string]any{"text": "reply 参数文本"})},
			}},
		}
	})
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "你能确认一下吗？", "idempotency_key": "canonical-turn", "turn_id": "canonical-turn-1", "attachment_refs": []any{},
	}); err == nil {
		t.Fatal("a candidate with disagreeing visible_text sources must fail closed under the F-01 path b contract")
	}

	var assistantCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND turn_id='canonical-turn-1'`, conversationID).Scan(&assistantCount); err != nil {
		t.Fatal(err)
	}
	if assistantCount != 0 {
		t.Fatalf("no assistant message must be delivered for a conflicting candidate, got %d", assistantCount)
	}
}

// ---------------------------------------------------------------------------
// Static guard: one visible-text authority, side-effect-free preview
// ---------------------------------------------------------------------------

func TestVisibleTextStaticGuardHasSingleAuthority(t *testing.T) {
	source, err := os.ReadFile("mutations.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if count := strings.Count(text, "resolveCanonicalVisibleReply("); count != 0 {
		t.Fatalf("mutations.go must not resolve the visible text; the single authority is the shared normalizer, found %d", count)
	}
	if strings.Contains(text, "replyTextFromCapabilityInvocations(") {
		t.Fatalf("mutations.go must not re-derive the visible text from reply arguments (F02)")
	}
	normalizer, err := os.ReadFile("turn_decision.go")
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(normalizer), "resolveCanonicalVisibleReply("); count != 1 {
		t.Fatalf("the visible text must be resolved at exactly one generation-time point, found %d", count)
	}

	preview, err := os.ReadFile("turn_takeover.go")
	if err != nil {
		t.Fatal(err)
	}
	previewText := string(preview)
	if strings.Contains(previewText, "CapabilityName ==") || strings.Contains(previewText, "CapabilityName==") {
		t.Fatalf("the candidate preview must dispatch by declared metadata, never by capability name")
	}
	if strings.Contains(previewText, ".Execute(") {
		t.Fatalf("the candidate preview must never execute a capability")
	}
	if !strings.Contains(previewText, "canonical.Text") {
		t.Fatalf("the preview must read the frozen canonical text")
	}
}
