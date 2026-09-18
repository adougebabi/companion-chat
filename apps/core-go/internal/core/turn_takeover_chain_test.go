package core

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// This file is the chain-level verification of phase 5 (implement.md phase 5 /
// task #12). It drives a whole HandleTurn through the single arbitration point
// with a scripted Provider so the claims that matter are asserted against
// durable rows and the real wire payload, never against a unit helper in
// isolation:
//
//   * a declined Judge executes the original candidate, byte for byte;
//   * an approved Judge executes only the takeover reply — the rejected
//     candidate leaves no appraisal, no state revision and no persistent
//     dominant-profile change behind;
//   * the takeover reply is generated inside the reply owner's scope;
//   * a pure-query turn never reaches the Judge;
//   * an invalid candidate fails before the Judge, and a failing takeover
//     generation never falls back to sending the rejected candidate;
//   * only turn_stage == winner_ready may execute.

const (
	takeoverChainRuleID = "takeover:public-doubt"
	// Per-profile markers prove which persona the Provider actually saw. The
	// roster is identifier-only, so a marker can only arrive through the
	// Working Persona of that profile.
	takeoverChainSparkMarker    = "SPARK_SCOPE_MARKER"
	takeoverChainTwilightMarker = "TWILIGHT_SCOPE_MARKER"
)

// takeoverChainSeed prepares a multi-profile fluctlight whose Core Persona
// declares both a persistent switching rule (so the Main turn stays authorized
// to propose a dominant-profile change) and a turn_takeover rule (so the
// arbitration point has exactly one selectable handover).
func takeoverChainSeed(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID string) {
	t.Helper()
	takeoverChainSeedWithTakeoverRules(t, ctx, repository, ownerID, fluctlightID, conversationID, takeoverChainDefaultRules())
}

// takeoverChainDefaultRules is the single declared handover the seeded persona
// carries. It is returned fresh so a test cannot mutate another test's seed.
func takeoverChainDefaultRules() []any {
	return []any{map[string]any{
		"id": "public-doubt", "kind": "turn_takeover", "version": personaTakeoverRuleVersion, "condition": "用户在强烈质疑本轮回复时由暮光接管",
		"target_profile_id": "twilight", "source_profile_id": "spark",
	}}
}

// takeoverChainSeedWithTakeoverRules is the same fluctlight with an explicit
// takeover_rules list, so a scenario that must produce zero extra calls can
// declare no handover rule at all ([1] of design.md 4.7).
func takeoverChainSeedWithTakeoverRules(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID string, takeoverRules []any) {
	t.Helper()
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	personalitySystem := map[string]any{
		"mode": "multiple", "active_profile_id": "spark",
		"profiles": []any{
			map[string]any{
				"id": "spark", "name": "星火",
				"personality":       map[string]any{"openness": 0.8, "signature_phrase": takeoverChainSparkMarker},
				"behavioral_policy": map[string]any{"directness": 0.9},
			},
			map[string]any{
				"id": "twilight", "name": "暮光",
				"personality":       map[string]any{"openness": 0.3, "signature_phrase": takeoverChainTwilightMarker},
				"behavioral_policy": map[string]any{"gentleness": 0.9},
			},
		},
		"switching": map[string]any{"rules": []any{
			map[string]any{"id": "safety", "condition": "收到明确的安全确认后由暮光主导", "target_profile_id": "twilight"},
		}},
	}
	if len(takeoverRules) > 0 {
		personalitySystem["takeover_rules"] = takeoverRules
	}
	corePersona := map[string]any{
		"identity":           map[string]any{"name": "摇光"},
		"life_profile":       map[string]any{"city": "上海"},
		"personality_system": personalitySystem,
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active',$3,'{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID, jsonString(corePersona)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'spark',0)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'takeover chain')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repository, "takeover-chain-endpoint-"+fluctlightID)
}

// takeoverChainTurnPayload is the HandleTurn input every chain test uses.
func takeoverChainTurnPayload(fluctlightID, text, idempotencyKey, turnID string) map[string]any {
	return map[string]any{
		"fluctlight_id": fluctlightID, "text": text, "idempotency_key": idempotencyKey,
		"turn_id": turnID, "attachment_refs": []any{},
	}
}

// takeoverChainMainResult scripts one Main/takeover generation reply.
func takeoverChainMainResult(text string, extra map[string]any) fakeProviderResult {
	structured := map[string]any{
		"response_mode": "final", "action_type": "reply", "response_intent": "reply",
		"visible_text": text, "tool_calls": []any{}, "influences": []any{},
	}
	for key, value := range extra {
		structured[key] = value
	}
	return fakeProviderResult{Structured: structured}
}

// takeoverChainSequence scripts the successive conversation_turn_response
// replies (candidate A). An extra call is answered with a distinctive text so
// "an extra generation happened" is observable instead of being silently
// absorbed.
func takeoverChainSequence(results ...fakeProviderResult) fakeProviderScript {
	position := 0
	return func(map[string]any) fakeProviderResult {
		if position >= len(results) {
			position++
			return takeoverChainMainResult("unexpected extra main generation", nil)
		}
		result := results[position]
		position++
		return result
	}
}

func takeoverChainJudge(takeover bool) fakeProviderScript {
	return func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"takeover": takeover, "decision_code": "other"}}
	}
}

// takeoverChainFailGeneration always fails. It is routed on the takeover-reply
// schema, which is only ever requested for the second generation, so the failure
// is exactly "the takeover reply could not be produced".
func takeoverChainFailGeneration() fakeProviderScript {
	return func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Status: 503}
	}
}

const queryContinuationTestSchemaName = "query_continuation_response"

// takeoverChainFrozenPayload reads the durable payload of one turn by its inbox
// idempotency key.
func takeoverChainFrozenPayload(t *testing.T, ctx context.Context, repository *PostgresRepository, idempotencyKey string) map[string]any {
	t.Helper()
	var payload []byte
	if err := repository.Pool().QueryRow(ctx,
		`SELECT payload FROM public.cognition_frozen_actions WHERE inbox_id=(SELECT id FROM public.cognition_inbox WHERE idempotency_key=$1 AND event_type='conversation.turn' ORDER BY created_at DESC LIMIT 1)`,
		idempotencyKey).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	return decodeObject(payload)
}

// takeoverChainAssistantTexts returns every assistant message of a turn.
func takeoverChainAssistantTexts(t *testing.T, ctx context.Context, repository *PostgresRepository, conversationID, turnID string) []string {
	t.Helper()
	rows, err := repository.Pool().Query(ctx,
		`SELECT text FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND turn_id=$2 ORDER BY sequence`, conversationID, turnID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := make([]string, 0, 1)
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			t.Fatal(err)
		}
		result = append(result, text)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func takeoverChainCount(t *testing.T, ctx context.Context, repository *PostgresRepository, query string, arguments ...any) int {
	t.Helper()
	var count int
	if err := repository.Pool().QueryRow(ctx, query, arguments...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// takeoverChainInboxPredicate scopes a row count to one turn's inbox fact.
const takeoverChainInboxPredicate = `(SELECT id FROM public.cognition_inbox WHERE idempotency_key=$1 AND event_type='conversation.turn' ORDER BY created_at DESC LIMIT 1)`

func takeoverChainSystemContent(t *testing.T, payload map[string]any) string {
	t.Helper()
	for _, raw := range arrayValue(payload["messages"]) {
		message := mapValue(raw)
		if stringValue(message["role"]) == "system" {
			return stringValue(message["content"])
		}
	}
	t.Fatalf("the request carried no system message: %#v", payload["messages"])
	return ""
}

// ---------------------------------------------------------------------------
// Declared rules must survive the projection envelope
// ---------------------------------------------------------------------------

// TestDeclaredRulesReachTheProductionTurn pins the plumbing that the whole
// feature depends on: the context projection carries the Core Persona inside a
// {authority,data} envelope, so every persona-semantics reader must unwrap it.
// Without the unwrap the declared rules are invisible to the turn, the Main
// call silently loses its authorized personality_decision field and no takeover
// rule can ever be selected.
func TestDeclaredRulesReachTheProductionTurn(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-rules-owner", "chain-rules-fluctlight", "chain-rules-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult("星火的候选回复", nil))).
		on(takeoverJudgeSchemaName, takeoverChainJudge(false))
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你到底有没有认真听？", "chain-rules-turn", "chain-rules-turn-1")); err != nil {
		t.Fatal(err)
	}

	mainRequests := router.payloads(workingPersonaMainTurnSchema)
	if len(mainRequests) != 1 {
		t.Fatalf("expected exactly one main generation, got %d", len(mainRequests))
	}
	properties := mapValue(mapValue(mapValue(mapValue(mainRequests[0]["response_format"])["json_schema"])["schema"])["properties"])
	if _, exists := properties["personality_decision"]; !exists {
		t.Fatalf("the Main turn lost the personality_decision field even though the persona declares a switching rule: %#v", properties)
	}

	frozen := takeoverChainFrozenPayload(t, ctx, repository, "chain-rules-turn")
	takeover := mapValue(frozen[turnTakeoverPayloadKey])
	if stringValue(takeover["rule_id"]) != takeoverChainRuleID {
		t.Fatalf("the declared takeover rule never reached selection: %#v", takeover)
	}
	if stringValue(takeover["target_profile_id"]) != "twilight" {
		t.Fatalf("the selected rule lost its target: %#v", takeover)
	}
	if stringValue(takeover["decision"]) != takeoverDecisionJudgeKeptA {
		t.Fatalf("a declined Judge must be recorded as judge_kept_a: %#v", takeover)
	}
	if stage := turnStageOf(frozen); stage != turnStageExecuting {
		t.Fatalf("a settled turn must have entered the execution window, got %q", stage)
	}
}

// takeoverChainAssertExecutionWindow checks that a turn moved past arbitration
// into the execution window. A settled turn is `executing`: winner_ready is
// transient and only observable when a crash happened between arbitration and
// Prepare.
func takeoverChainAssertExecutionWindow(t *testing.T, frozen map[string]any) {
	t.Helper()
	if stage := turnStageOf(frozen); stage != turnStageExecuting {
		t.Fatalf("the turn never entered the execution window, got stage %q", stage)
	}
}

// ---------------------------------------------------------------------------
// Judge declines: the original candidate is the one that executes
// ---------------------------------------------------------------------------

func TestTakeoverJudgeDeclinesExecutesTheOriginalCandidate(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-decline-owner", "chain-decline-fluctlight", "chain-decline-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult("星火的候选回复", nil))).
		on(takeoverJudgeSchemaName, takeoverChainJudge(false))
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你在敷衍我吗？", "chain-decline-turn", "chain-decline-turn-1")); err != nil {
		t.Fatal(err)
	}

	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-decline-turn-1"); len(texts) != 1 || texts[0] != "星火的候选回复" {
		t.Fatalf("a declined takeover must send the validated candidate verbatim, got %#v", texts)
	}
	if count := router.requestCount(takeoverJudgeSchemaName); count != 1 {
		t.Fatalf("the Judge must run exactly once, got %d", count)
	}
	if count := router.requestCount(workingPersonaMainTurnSchema); count != 1 {
		t.Fatalf("a declined takeover must not spend a second main generation, got %d", count)
	}
	if count := router.requestCount(takeoverReplySchemaName); count != 0 {
		t.Fatalf("a declined takeover must not generate a takeover reply, got %d", count)
	}

	frozen := takeoverChainFrozenPayload(t, ctx, repository, "chain-decline-turn")
	if winner := mapValue(frozen[turnWinnerPayloadKey]); stringValue(winner["source"]) != "a" {
		t.Fatalf("the winner must be the original candidate: %#v", winner)
	}
	if owner := stringValue(mapValue(frozen[turnPersonaScopePayloadKey])["reply_owner_profile_id"]); owner != "spark" {
		t.Fatalf("a declined takeover must keep the reply owner, got %q", owner)
	}
}

// ---------------------------------------------------------------------------
// Judge approves: only the takeover reply executes, in the owner's scope
// ---------------------------------------------------------------------------

func TestTakeoverJudgeApprovesExecutesOnlyTheTakeoverReply(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-approve-owner", "chain-approve-fluctlight", "chain-approve-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	// The rejected candidate proposes BOTH a state transition (appraisal) and a
	// persistent dominant-profile switch. Neither may survive the takeover.
	appraisal := map[string]any{}
	for _, field := range appraisalFields {
		appraisal[field] = 0.5
	}
	rejected := takeoverChainMainResult("星火的候选回复", map[string]any{
		"appraisal": appraisal,
		"personality_decision": map[string]any{
			"decision": "switch", "from_profile_id": "spark", "target_profile_id": "twilight",
			"trigger_id": "safety", "reason": "候选认为自己应让位",
		},
	})

	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(rejected)).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult("暮光的接管回复", nil))).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你根本没在听我说话。", "chain-approve-turn", "chain-approve-turn-1")); err != nil {
		t.Fatal(err)
	}

	texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-approve-turn-1")
	if len(texts) != 1 || texts[0] != "暮光的接管回复" {
		t.Fatalf("only the takeover reply may be sent, got %#v", texts)
	}
	for _, text := range texts {
		if strings.Contains(text, "星火的候选回复") {
			t.Fatalf("the rejected candidate reached the conversation: %#v", texts)
		}
	}
	if count := router.requestCount(workingPersonaMainTurnSchema); count != 1 {
		t.Fatalf("a takeover spends exactly one candidate generation, got %d", count)
	}
	if count := router.requestCount(takeoverReplySchemaName); count != 1 {
		t.Fatalf("a takeover spends exactly one takeover generation, got %d", count)
	}
	if count := router.requestCount(takeoverJudgeSchemaName); count != 1 {
		t.Fatalf("a takeover must not re-judge, got %d judge calls", count)
	}

	frozen := takeoverChainFrozenPayload(t, ctx, repository, "chain-approve-turn")
	takeover := mapValue(frozen[turnTakeoverPayloadKey])
	if stringValue(takeover["decision"]) != takeoverDecisionTakeoverB {
		t.Fatalf("the arbitration record lost the takeover decision: %#v", takeover)
	}
	if stringValue(takeover["reply_owner_profile_id"]) != "twilight" {
		t.Fatalf("the takeover record lost its reply owner: %#v", takeover)
	}
	rejectedRecord := mapValue(takeover["rejected_candidate"])
	if stringValue(rejectedRecord["visible_text"]) != "星火的候选回复" {
		t.Fatalf("the rejected candidate must be kept for diagnostics only: %#v", rejectedRecord)
	}
	if winner := mapValue(frozen[turnWinnerPayloadKey]); stringValue(winner["source"]) != "b" || stringValue(winner["reply_owner_profile_id"]) != "twilight" {
		t.Fatalf("the frozen winner must be the takeover reply: %#v", winner)
	}

	// takeover_once: the persistent dominant profile is untouched even though the
	// rejected candidate asked to switch it and was authorized to ask.
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("a takeover reply changed the persistent dominant profile to %q", active)
	}
	// The frozen scope distinguishes the persistent dominant profile from the
	// reply owner of this turn only.
	scope := mapValue(frozen[turnPersonaScopePayloadKey])
	if stringValue(scope["active_profile_id"]) != "spark" || stringValue(scope["reply_owner_profile_id"]) != "twilight" {
		t.Fatalf("the frozen scope must separate active from reply owner: %#v", scope)
	}

	// The rejected candidate's cognitive-state proposal left no trace: no
	// appraisal and no state revision was committed for this turn.
	if count := takeoverChainCount(t, ctx, repository,
		`SELECT count(*) FROM public.cognition_appraisals WHERE source_fact_id=`+takeoverChainInboxPredicate, "chain-approve-turn"); count != 0 {
		t.Fatalf("the rejected candidate's appraisal was committed (%d rows)", count)
	}
	if count := takeoverChainCount(t, ctx, repository,
		`SELECT count(*) FROM public.fluctlight_state_revisions WHERE source_event_id=`+takeoverChainInboxPredicate, "chain-approve-turn"); count != 0 {
		t.Fatalf("the rejected candidate's state transition was committed (%d rows)", count)
	}
}

// TestTakeoverReplyIsGeneratedInTheReplyOwnerScope proves the second main
// generation is re-scoped (F04 / design.md 10): the Provider sees the takeover
// owner's Working Persona, not the persistent dominant profile's.
func TestTakeoverReplyIsGeneratedInTheReplyOwnerScope(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-scope-owner", "chain-scope-fluctlight", "chain-scope-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult("星火的候选回复", nil))).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult("暮光的接管回复", nil))).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你是不是在骗我？", "chain-scope-turn", "chain-scope-turn-1")); err != nil {
		t.Fatal(err)
	}

	candidates := router.payloads(workingPersonaMainTurnSchema)
	replies := router.payloads(takeoverReplySchemaName)
	if len(candidates) != 1 || len(replies) != 1 {
		t.Fatalf("expected one candidate and one takeover generation, got %d and %d", len(candidates), len(replies))
	}
	candidateSystem := takeoverChainSystemContent(t, candidates[0])
	takeoverSystem := takeoverChainSystemContent(t, replies[0])

	if !strings.Contains(candidateSystem, takeoverChainSparkMarker) || strings.Contains(candidateSystem, takeoverChainTwilightMarker) {
		t.Fatalf("the candidate generation did not run as the active profile")
	}
	if !strings.Contains(takeoverSystem, takeoverChainTwilightMarker) || strings.Contains(takeoverSystem, takeoverChainSparkMarker) {
		t.Fatalf("the takeover generation did not run as the reply owner")
	}
	// The takeover generation states the boundary explicitly instead of relying
	// on the model to infer that the replaced candidate never happened.
	if !strings.Contains(takeoverSystem, "未发送") {
		t.Fatalf("the takeover generation is missing the replaced-candidate boundary rule")
	}
	// F06: a replaced candidate's query capabilities are never called, so no
	// query result exists for this turn. The boundary is a system rule rather
	// than something the model is asked to infer from "the plan was dropped".
	if !strings.Contains(takeoverSystem, "没有返回任何查询结果") {
		t.Fatalf("the takeover generation may treat an unreported query result as a known fact")
	}
	// Only the interactive Main turn may carry the persistent-switch section.
	if strings.Contains(takeoverSystem, "persistent_switch") {
		t.Fatalf("the takeover generation was offered a persistent dominant-profile switch")
	}
}

// ---------------------------------------------------------------------------
// Mutual exclusion, invalid candidates, failing takeover generation
// ---------------------------------------------------------------------------

func TestPureQueryTurnNeverInvokesTheJudge(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-query-owner", "chain-query-fluctlight", "chain-query-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	candidate := fakeProviderResult{
		Structured: map[string]any{
			"response_mode": "query_continuation", "action_type": "reply", "response_intent": "recall",
			"tool_calls": []any{}, "influences": []any{},
		},
		ToolCalls: []map[string]any{{
			"id": "query-recall", "type": "function",
			"function": map[string]any{"name": "memory.recall", "arguments": jsonString(map[string]any{"intent": "用户上次提到的计划"})},
		}},
	}
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(candidate)).
		on(queryContinuationTestSchemaName, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"visible_text": "我记得那件事。"}}
		}).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你还记得我上次说的计划吗？", "chain-query-turn", "chain-query-turn-1")); err != nil {
		t.Fatal(err)
	}

	if count := router.requestCount(takeoverJudgeSchemaName); count != 0 {
		t.Fatalf("a pure-query turn must never invoke the Judge, got %d calls", count)
	}
	if count := router.requestCount(workingPersonaMainTurnSchema); count != 1 {
		t.Fatalf("a pure-query turn spends exactly one main generation, got %d", count)
	}
	if count := router.requestCount(takeoverReplySchemaName); count != 0 {
		t.Fatalf("a pure-query turn must not generate a takeover reply, got %d", count)
	}
	frozen := takeoverChainFrozenPayload(t, ctx, repository, "chain-query-turn")
	takeover := mapValue(frozen[turnTakeoverPayloadKey])
	if stringValue(takeover["decision"]) != takeoverDecisionNotApplicable || stringValue(takeover["skip_reason"]) != "query_continuation" {
		t.Fatalf("the mutual exclusion must be recorded on the payload: %#v", takeover)
	}
	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-query-turn-1"); len(texts) != 1 || texts[0] != "我记得那件事。" {
		t.Fatalf("the continuation answer must be the visible text, got %#v", texts)
	}
}

func TestInvalidCandidateFailsBeforeTheJudge(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-invalid-owner", "chain-invalid-fluctlight", "chain-invalid-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	// conversation.reply requires `text`; an empty argument object is a candidate
	// that cannot be persisted, so it must fail before the Judge is consulted.
	candidate := fakeProviderResult{
		Structured: map[string]any{
			"response_mode": "final", "action_type": "reply", "response_intent": "reply",
			"visible_text": "星火的候选回复", "tool_calls": []any{}, "influences": []any{},
		},
		ToolCalls: []map[string]any{{
			"id": "invalid-reply", "type": "function",
			"function": map[string]any{"name": "conversation.reply", "arguments": "{}"},
		}},
	}
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(candidate)).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "确认一下。", "chain-invalid-turn", "chain-invalid-turn-1")); err == nil {
		t.Fatal("an invalid candidate must fail the turn")
	}

	if count := router.requestCount(takeoverJudgeSchemaName); count != 0 {
		t.Fatalf("an invalid candidate must fail before the Judge, got %d judge calls", count)
	}
	if count := router.requestCount(takeoverReplySchemaName); count != 0 {
		t.Fatalf("an invalid candidate must not reach the takeover generation, got %d", count)
	}
	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-invalid-turn-1"); len(texts) != 0 {
		t.Fatalf("an invalid candidate must not be delivered: %#v", texts)
	}
	frozen := takeoverChainFrozenPayload(t, ctx, repository, "chain-invalid-turn")
	if stage := turnStageOf(frozen); stage == turnStageWinnerReady {
		t.Fatalf("an invalid candidate must never become executable: %#v", frozen[turnStagePayloadKey])
	}
}

func TestFailingTakeoverGenerationNeverSendsTheRejectedCandidate(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-bfail-owner", "chain-bfail-fluctlight", "chain-bfail-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult("星火的候选回复", nil))).
		on(takeoverReplySchemaName, takeoverChainFailGeneration()).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你根本不在乎。", "chain-bfail-turn", "chain-bfail-turn-1")); err == nil {
		t.Fatal("a failing takeover generation must fail the turn")
	}

	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-bfail-turn-1"); len(texts) != 0 {
		t.Fatalf("a failing takeover must never fall back to the rejected candidate: %#v", texts)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("a failing takeover changed the persistent dominant profile to %q", active)
	}
}

// ---------------------------------------------------------------------------
// QUERY mutual exclusion and the main-generation budget (R06 / F06)
// ---------------------------------------------------------------------------

// takeoverChainFrozenFailure reads the terminal status and failure code of the
// frozen turn for one inbox fact.
func takeoverChainFrozenFailure(t *testing.T, ctx context.Context, repository *PostgresRepository, idempotencyKey string) (string, string) {
	t.Helper()
	var status, code string
	if err := repository.Pool().QueryRow(ctx,
		`SELECT status, COALESCE(error_code,'') FROM public.cognition_frozen_actions WHERE inbox_id=`+takeoverChainInboxPredicate,
		idempotencyKey).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	return status, code
}

// takeoverChainMixedCandidate proposes BOTH a pure query and a direct reply, and
// deliberately declares no response_mode: the resolved mode must be derived, not
// taken on faith. The batch is not a pure query, so it is a mixed final turn.
func takeoverChainMixedCandidate(text string) fakeProviderResult {
	return fakeProviderResult{
		Structured: map[string]any{"action_type": "reply", "response_intent": "reply", "influences": []any{}},
		ToolCalls: []map[string]any{
			{
				"id": "mixed-recall", "type": "function",
				"function": map[string]any{"name": "memory.recall", "arguments": jsonString(map[string]any{"intent": "用户提到的计划"})},
			},
			{
				"id": "mixed-reply", "type": "function",
				"function": map[string]any{"name": "conversation.reply", "arguments": jsonString(map[string]any{"text": text})},
			},
		},
	}
}

// takeoverChainPureQueryCandidate proposes exactly one pure query and no reply,
// which is the only shape that may enter the result-dependent continuation path.
func takeoverChainPureQueryCandidate() fakeProviderResult {
	return fakeProviderResult{
		Structured: map[string]any{
			"response_mode": "query_continuation", "action_type": "reply", "response_intent": "recall",
			"tool_calls": []any{}, "influences": []any{},
		},
		ToolCalls: []map[string]any{{
			"id": "pure-recall", "type": "function",
			"function": map[string]any{"name": "memory.recall", "arguments": jsonString(map[string]any{"intent": "用户上次提到的计划"})},
		}},
	}
}

// TestMixedQueryActionBatchMayBeArbitratedButNotContinued pins the F06
// correction: the mutual exclusion is between the RESULT-DEPENDENT continuation
// path and the takeover path, not between "any QUERY capability" and takeover.
// A mixed QUERY + ACTION batch resolves to final, so it is arbitrated — and
// because it is final, it never enters the continuation path.
func TestMixedQueryActionBatchMayBeArbitratedButNotContinued(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-mixed-owner", "chain-mixed-fluctlight", "chain-mixed-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMixedCandidate("星火先答一句，再回想一下。"))).
		on(takeoverJudgeSchemaName, takeoverChainJudge(false)).
		// A mixed batch must never reach the continuation synthesis.
		on(queryContinuationTestSchemaName, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"visible_text": "不该发生的续调用"}}
		})
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "顺便帮我回忆一下，还有我上次说的计划。", "chain-mixed-turn", "chain-mixed-turn-1")); err != nil {
		t.Fatal(err)
	}

	// Arbitration IS allowed for a mixed final batch.
	if count := router.requestCount(takeoverJudgeSchemaName); count != 1 {
		t.Fatalf("a mixed QUERY+ACTION final batch must be arbitrable, got %d judge calls", count)
	}
	// ...but the result-dependent continuation must not run.
	if count := router.requestCount(queryContinuationTestSchemaName); count != 0 {
		t.Fatalf("a mixed final batch must not enter the continuation path, got %d synthesis calls", count)
	}
	if count := router.requestCount(takeoverReplySchemaName); count != 0 {
		t.Fatalf("a declined Judge must not generate a takeover reply, got %d", count)
	}
	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-mixed-turn-1"); len(texts) != 1 || texts[0] != "星火先答一句，再回想一下。" {
		t.Fatalf("the mixed candidate's visible text must be delivered verbatim, got %#v", texts)
	}

	frozen := takeoverChainFrozenPayload(t, ctx, repository, "chain-mixed-turn")
	decision := mapValue(frozen["decision"])
	if mode := stringValue(mapValue(decision["response_plan"])["response_mode"]); mode != "final" {
		t.Fatalf("a mixed batch must resolve to final, got %q", mode)
	}
	if responseMode := stringValue(frozen["response_mode"]); responseMode == "query_continuation" {
		t.Fatalf("a mixed batch must never be recorded as a continuation: %#v", frozen["response_mode"])
	}
	// The batch really was mixed, and it really is not a pure query — which is
	// exactly why the continuation path was correctly not taken.
	invocations, err := capabilityInvocationsFromValue(frozen["capability_invocations"])
	if err != nil {
		t.Fatal(err)
	}
	if len(invocations) != 2 {
		t.Fatalf("the mixed candidate must persist both invocations, got %d: %#v", len(invocations), invocations)
	}
	if err := validatePureQueryContinuation(invocations, app.capabilityRegistry()); err == nil {
		t.Fatal("a QUERY + ACTION batch must not satisfy the pure-query continuation contract")
	}
}

// TestTakeoverReplyQueryContinuationFailsClosed pins the controlled failure: a
// takeover reply that asks for yet another result-dependent continuation is not
// answered by a third generation and not answered by fabricating text.
func TestTakeoverReplyQueryContinuationFailsClosed(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-budget-owner", "chain-budget-fluctlight", "chain-budget-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	// B answers in the only shape the continuation path accepts. Because a
	// takeover speaker may not spend a third call, this must fail closed.
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult("星火的候选回复", nil))).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainPureQueryCandidate())).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true)).
		on(queryContinuationTestSchemaName, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"visible_text": "第三次调用不该发生"}}
		})
	app := newTestApp(t, repository, router)
	_, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你又在搪塞我吧。", "chain-budget-turn", "chain-budget-turn-1"))
	if err == nil {
		t.Fatal("a takeover reply that asks for another continuation must fail the turn")
	}
	if !errors.Is(err, errTakeoverReplyBudgetExhausted) {
		t.Fatalf("the failure must be the controlled budget exhaustion, got %v", err)
	}

	if count := router.requestCount(workingPersonaMainTurnSchema); count != 1 {
		t.Fatalf("the candidate generation must run exactly once, got %d", count)
	}
	if count := router.requestCount(takeoverReplySchemaName); count != 1 {
		t.Fatalf("the takeover generation must run exactly once, got %d", count)
	}
	if count := router.requestCount(queryContinuationTestSchemaName); count != 0 {
		t.Fatalf("a failing takeover must never spend a third generation, got %d", count)
	}
	if count := router.requestCount(takeoverJudgeSchemaName); count != 1 {
		t.Fatalf("a takeover must not re-judge, got %d", count)
	}
	// Neither candidate may reach the conversation.
	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-budget-turn-1"); len(texts) != 0 {
		t.Fatalf("a failing takeover must deliver nothing: %#v", texts)
	}
	// The turn fails closed with its own code instead of staying resumable.
	status, code := takeoverChainFrozenFailure(t, ctx, repository, "chain-budget-turn")
	if status != "failed" || code != takeoverReplyBudgetExhaustedCode {
		t.Fatalf("the frozen turn must be quarantined as %q / %q, got %q / %q", "failed", takeoverReplyBudgetExhaustedCode, status, code)
	}
}

// TestMainGenerationBudgetNeverExceedsTwo is the runtime counterpart of the
// budget table in design.md 4.7: A plus B is at most two main generations, the
// Judge is metered outside that budget, and the continuation path and the
// takeover path never co-occur in one turn.
func TestMainGenerationBudgetNeverExceedsTwo(t *testing.T) {
	cases := []struct {
		name           string
		rules          []any
		candidate      fakeProviderResult
		judge          bool
		wantCandidate  int
		wantTakeover   int
		wantJudge      int
		wantContinuity int
	}{
		{
			name:  "no takeover rule pays for nothing extra",
			rules: nil, candidate: takeoverChainMainResult("星火的候选回复", nil),
			wantCandidate: 1,
		},
		{
			name:  "declined Judge keeps the candidate",
			rules: takeoverChainDefaultRules(), candidate: takeoverChainMainResult("星火的候选回复", nil), judge: false,
			wantCandidate: 1, wantJudge: 1,
		},
		{
			name:  "approved Judge replaces the candidate",
			rules: takeoverChainDefaultRules(), candidate: takeoverChainMainResult("星火的候选回复", nil), judge: true,
			wantCandidate: 1, wantTakeover: 1, wantJudge: 1,
		},
		{
			name:  "pure query skips arbitration entirely",
			rules: takeoverChainDefaultRules(), candidate: takeoverChainPureQueryCandidate(),
			wantCandidate: 1, wantContinuity: 1,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			slug := strings.ReplaceAll(testCase.name, " ", "-")
			ownerID, fluctlightID, conversationID := "budget-owner-"+slug, "budget-fluctlight-"+slug, "budget-conversation-"+slug
			takeoverChainSeedWithTakeoverRules(t, ctx, repository, ownerID, fluctlightID, conversationID, testCase.rules)

			router := newFakeProviderRouter().
				on(workingPersonaMainTurnSchema, takeoverChainSequence(testCase.candidate)).
				on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult("暮光的接管回复", nil))).
				on(takeoverJudgeSchemaName, takeoverChainJudge(testCase.judge)).
				on(queryContinuationTestSchemaName, func(map[string]any) fakeProviderResult {
					return fakeProviderResult{Structured: map[string]any{"visible_text": "我记得那件事。"}}
				})
			app := newTestApp(t, repository, router)
			if _, err := app.HandleTurn(ctx, ownerID, conversationID,
				takeoverChainTurnPayload(fluctlightID, "你在敷衍我吗？", "budget-turn-"+slug, "budget-turn-"+slug+"-1")); err != nil {
				t.Fatal(err)
			}

			candidates := router.requestCount(workingPersonaMainTurnSchema)
			takeovers := router.requestCount(takeoverReplySchemaName)
			if candidates != testCase.wantCandidate || takeovers != testCase.wantTakeover {
				t.Fatalf("expected %d candidate + %d takeover generations, got %d + %d",
					testCase.wantCandidate, testCase.wantTakeover, candidates, takeovers)
			}
			if total := candidates + takeovers; total > 2 {
				t.Fatalf("the main-generation budget is two, spent %d", total)
			}
			if judge := router.requestCount(takeoverJudgeSchemaName); judge != testCase.wantJudge {
				t.Fatalf("expected %d judge calls, got %d", testCase.wantJudge, judge)
			}
			if continuity := router.requestCount(queryContinuationTestSchemaName); continuity != testCase.wantContinuity {
				t.Fatalf("expected %d continuation calls, got %d", testCase.wantContinuity, continuity)
			}
			// The two paths are mutually exclusive per turn, whatever the budget
			// allows: a continuation turn never consults the Judge, and a
			// takeover turn never continues.
			if router.requestCount(queryContinuationTestSchemaName) > 0 && router.requestCount(takeoverJudgeSchemaName) > 0 {
				t.Fatal("the continuation path and the takeover path co-occurred in one turn")
			}

			frozen := takeoverChainFrozenPayload(t, ctx, repository, "budget-turn-"+slug)
			takeoverChainAssertExecutionWindow(t, frozen)
		})
	}
}

// ---------------------------------------------------------------------------
// Stage machine and static guards
// ---------------------------------------------------------------------------

func TestTurnStageMachineSemantics(t *testing.T) {
	// A payload persisted before the stage machine existed is a frozen but
	// unarbitrated candidate: exactly a_frozen, and never executable.
	if got := turnStageOf(map[string]any{}); got != turnStageAFrozen {
		t.Fatalf("a legacy payload must read as %q, got %q", turnStageAFrozen, got)
	}
	if turnStageExecutable(map[string]any{}) {
		t.Fatal("a legacy payload must not be executable")
	}
	if err := validateFrozenTurnStage(map[string]any{turnStagePayloadKey: "bogus"}); err == nil {
		t.Fatal("an unknown stage must be rejected")
	}
	if err := validateFrozenTurnStage(map[string]any{turnStagePayloadKey: ""}); err != nil {
		t.Fatalf("a payload without a stage must stay readable: %v", err)
	}
	if !turnStageExecutable(map[string]any{turnStagePayloadKey: turnStageWinnerReady}) {
		t.Fatal("winner_ready is the first executable stage")
	}
	if !turnStageExecutable(map[string]any{turnStagePayloadKey: turnStageExecuting}) {
		t.Fatal("executing must stay executable so a crashed-in-flight turn resumes")
	}
	for _, stage := range []string{turnStageAFrozen, turnStageArbitrationDecided, turnStageBFrozen, turnStageSettled} {
		if turnStageExecutable(map[string]any{turnStagePayloadKey: stage}) {
			t.Fatalf("stage %q must not be executable", stage)
		}
	}
}

// TestTakeoverChainStaticGuards keeps the structural properties that the chain
// tests rely on from silently regressing.
func TestTakeoverChainStaticGuards(t *testing.T) {
	mutations, err := os.ReadFile("mutations.go")
	if err != nil {
		t.Fatal(err)
	}
	mutationSource := string(mutations)
	if count := strings.Count(mutationSource, "a.applyTurnTakeover("); count != 1 {
		t.Fatalf("there must be exactly one arbitration insertion point, found %d", count)
	}
	if !strings.Contains(mutationSource, "!turnStageExecutable(frozen.Payload)") {
		t.Fatal("the execution-eligibility gate on turn_stage is missing (F03)")
	}
	if !strings.Contains(mutationSource, "if err := a.BeginTurnExecution(ctx, frozen.ID); err != nil {") {
		t.Fatal("entering the side-effect window must be recorded as turn_stage=executing (F03)")
	}
	if !strings.Contains(mutationSource, "a.validateCandidateCapabilityInvocations(capabilityInvocations, candidateValidationContext{") {
		t.Fatal("the side-effect-free candidate validation must precede the arbitration point (F02/F05)")
	}

	takeover, err := os.ReadFile("turn_takeover.go")
	if err != nil {
		t.Fatal(err)
	}
	takeoverSource := string(takeover)
	if strings.Contains(takeoverSource, "a.applyTurnTakeover(") {
		t.Fatal("the takeover reply must never navigate back into arbitration")
	}
	if count := strings.Count(takeoverSource, ".RunTakeoverJudgeTask("); count != 1 {
		t.Fatalf("the Judge must be invoked from exactly one place, found %d", count)
	}
	if count := strings.Count(takeoverSource, ".RunTakeoverReplyTask("); count != 1 {
		t.Fatalf("the takeover reply is the only second main generation, found %d", count)
	}
	// The Main schema name is what authorizes the persistent dominant-profile
	// switch. A takeover speaker must never be handed that section, so it must
	// declare its own schema name.
	if strings.Contains(takeoverSource, `definitions, "conversation_turn_response", schema`) {
		t.Fatal("the takeover generation must not reuse the Main schema name, which authorizes the persistent switch")
	}
	if !strings.Contains(takeoverSource, "definitions, takeoverReplySchemaName, schema") {
		t.Fatal("the takeover generation must declare its own schema name")
	}
	if takeoverReplySchemaName == workingPersonaMainTurnSchema {
		t.Fatal("the takeover schema name must differ from the Main turn schema name")
	}
	if !strings.Contains(takeoverSource, "persistentSwitchGrantScenarioTakeover") {
		t.Fatal("the takeover reply must resolve its authority from the frozen scenario, never from the candidate")
	}
	if !strings.Contains(takeoverSource, "validateCandidateCapabilityInvocations(normalized.Invocations, candidateValidationContext{") {
		t.Fatal("the takeover candidate must pass the same cheap validation as the candidate it replaces")
	}

	cognition, err := os.ReadFile("cognition.go")
	if err != nil {
		t.Fatal(err)
	}
	cognitionSource := string(cognition)
	if !strings.Contains(cognitionSource, "resetPersistentSwitchAuthorization(") {
		t.Fatal("an overwrite must reset the inherited persistent-switch authorization (E5)")
	}
	// The overwrite must hold the row and compare the stage inside one
	// transaction; RowsAffected on its own is not a compare-and-swap (M3).
	if !strings.Contains(cognitionSource, "WHERE id=$1 AND status='frozen' FOR UPDATE") {
		t.Fatal("the overwrite must take the frozen row FOR UPDATE before comparing the stage")
	}
	if !strings.Contains(cognitionSource, "if stage != overwrite.ExpectedStage {") {
		t.Fatal("the overwrite must compare the persisted stage against its expectation")
	}
	// M4: every derived material of the replaced candidate is replaced too.
	for _, key := range []string{"capability_context_snapshot", "context_reference_index", "influences", "goal_refs", "intention_refs"} {
		if !strings.Contains(cognitionSource, `"`+key+`"`) {
			t.Fatalf("the overwrite does not rebuild the derived field %q (M4)", key)
		}
	}
	if !strings.Contains(cognitionSource, "func stripFrozenDecisionSidecars(") {
		t.Fatal("the frozen decision sidecar strip must be shared, not duplicated per writer")
	}

	// [R06/F06] The QUERY mutual exclusion must be decided before any rule is
	// selected and before the Judge is consulted. Otherwise a result-dependent
	// continuation could pay for a Judge call whose verdict it can never use.
	mutexIndex := strings.Index(takeoverSource, `strings.TrimSpace(input.ResponseMode) == "query_continuation"`)
	if mutexIndex < 0 {
		t.Fatal("the QUERY continuation mutual exclusion is missing from the arbitration point (R06/F06)")
	}
	if !strings.Contains(takeoverSource, `SkipReason: "query_continuation"`) {
		t.Fatal("the mutual exclusion must be recorded on the payload, not silently skipped")
	}
	if ruleIndex := strings.Index(takeoverSource, "selectTurnTakeoverRule("); ruleIndex < 0 || ruleIndex < mutexIndex {
		t.Fatal("the QUERY mutual exclusion must be decided before the takeover rule is selected")
	}
	if judgeIndex := strings.Index(takeoverSource, ".RunTakeoverJudgeTask("); judgeIndex < 0 || judgeIndex < mutexIndex {
		t.Fatal("the QUERY mutual exclusion must be decided before the Judge is invoked")
	}
	if takeoverIndex := strings.Index(takeoverSource, ".RunTakeoverReplyTask("); takeoverIndex < 0 || takeoverIndex < mutexIndex {
		t.Fatal("the QUERY mutual exclusion must be decided before the takeover generation")
	}
	// The exclusion is keyed on the RESOLVED response mode, never on "the
	// candidate mentions a QUERY capability": a mixed QUERY + ACTION batch
	// resolves to final and may be arbitrated (design.md 4.7 item [5]).
	for _, forbidden := range []string{"CapabilityExecutionPureQuery", "validatePureQueryContinuation("} {
		if strings.Contains(takeoverSource, forbidden) {
			t.Fatalf("the arbitration point must not gate on %s; the exclusion is on the resolved response mode", forbidden)
		}
	}

	// The budget-exhausted failure must keep its own stable code so it is
	// separable from a transport failure, and the caller must record it on the
	// frozen turn instead of leaving a structurally-unwinnable turn resumable.
	decision, err := os.ReadFile("turn_decision.go")
	if err != nil {
		t.Fatal(err)
	}
	decisionSource := string(decision)
	if !strings.Contains(mutationSource, "takeoverFailureCode(takeoverErr)") {
		t.Fatal("an arbitration failure must be recorded with an exact code (F06)")
	}
	if !strings.Contains(takeoverSource, "takeoverReplyBudgetExhaustedCode") {
		t.Fatal("the budget-exhausted failure must keep its own stable code")
	}
	if !strings.Contains(decisionSource, "errors.New(takeoverReplyBudgetExhaustedCode)") {
		t.Fatal("the budget-exhausted error must carry the same string as the recorded code")
	}
	if !strings.Contains(decisionSource, "input.ForbidQueryContinuation") {
		t.Fatal("the takeover generation must be structurally forbidden from a result-dependent continuation")
	}

	// The projection stores the Core Persona inside its {authority,data}
	// envelope. A persona-semantics reader that forgets to unwrap silently
	// disables the whole persona-switch surface, so pin the accessor.
	rules, err := os.ReadFile("persona_switch_rules.go")
	if err != nil {
		t.Fatal(err)
	}
	rulesSource := string(rules)
	if !strings.Contains(rulesSource, "func corePersonaData(") {
		t.Fatal("the shared Core Persona envelope unwrap is missing")
	}
	if strings.Contains(rulesSource, `mapValue(corePersona["personality_system"])`) {
		t.Fatal("persona-switch semantics must be read from the unwrapped Core Persona")
	}
	persona, err := os.ReadFile("working_persona.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persona), `projection.CorePersona["personality_system"]`) {
		t.Fatal("the Working Persona must read the unwrapped Core Persona semantics")
	}
}
