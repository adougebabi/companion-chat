package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is the phase-7 verification: crash recovery, concurrency and
// idempotency (implement.md phase 7 / design.md 4.8). Every row of the recovery
// enumeration is driven through a whole HandleTurn against a real database, and
// the assertions are made on durable rows and on the number of model round
// trips, never on an in-memory helper.
//
// The crash-injection primitive is `takeoverChainRewind`: it recreates exactly
// the durable state a crash at a chosen stage would leave behind (nothing
// delivered, the inbox fact claimable again, the frozen row frozen once more).
// A recovery that reaches a verdict and a delivery with ZERO model calls is
// then proof that the verdict was not re-decided.

// ---------------------------------------------------------------------------
// Crash-injection and counting primitives
// ---------------------------------------------------------------------------

// takeoverChainForbiddenCalls scripts every model role an interactive turn can
// reach to fail loudly, so a recovery that wrongly re-consults the model is
// observable as a non-zero count instead of being silently absorbed.
func takeoverChainForbiddenCalls() *fakeProviderRouter {
	return newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainFailGeneration()).
		on(takeoverReplySchemaName, takeoverChainFailGeneration()).
		on(takeoverJudgeSchemaName, takeoverChainFailGeneration()).
		on(queryContinuationTestSchemaName, takeoverChainFailGeneration())
}

// takeoverChainModelRoles is every schema an interactive turn can request.
func takeoverChainModelRoles() []string {
	return []string{workingPersonaMainTurnSchema, takeoverReplySchemaName, takeoverJudgeSchemaName, queryContinuationTestSchemaName}
}

// takeoverChainAssertNoModelCalls proves a recovery reached a verdict and a
// delivery without a single model round trip.
func takeoverChainAssertNoModelCalls(t *testing.T, router *fakeProviderRouter) {
	t.Helper()
	for _, schemaName := range takeoverChainModelRoles() {
		if count := router.requestCount(schemaName); count != 0 {
			t.Fatalf("a recovery must not re-consult %s, got %d calls", schemaName, count)
		}
	}
	if total := router.totalRequests(); total != 0 {
		t.Fatalf("a decided recovery must issue no HTTP attempt at all, saw %d", total)
	}
}

// takeoverChainRewind rewinds a finished turn into exactly the durable state a
// crash at `stage` would have left behind. It is the crash-injection primitive
// for every boundary of design.md 4.8.
func takeoverChainRewind(t *testing.T, ctx context.Context, repository *PostgresRepository, conversationID, turnID, idempotencyKey, stage string, rewrite func(map[string]any)) {
	t.Helper()
	// Nothing was delivered before the crash.
	if _, err := repository.Pool().Exec(ctx,
		`DELETE FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND turn_id=$2`,
		conversationID, turnID); err != nil {
		t.Fatal(err)
	}
	// The inbox fact is claimable again: the crash released the lease.
	if _, err := repository.Pool().Exec(ctx,
		`UPDATE public.cognition_inbox SET status='pending',claimed_by=NULL,claimed_at=NULL,processed_at=NULL,error_code=NULL WHERE idempotency_key=$1 AND event_type='conversation.turn'`,
		idempotencyKey); err != nil {
		t.Fatal(err)
	}
	payload := takeoverChainFrozenPayload(t, ctx, repository, idempotencyKey)
	// A crash before settlement leaves no action outcome behind. Without this the
	// replayed settlement would (correctly) refuse to overwrite an already
	// recorded outcome with a different request digest, which is a *different*
	// guard than the one under test.
	if _, err := repository.Pool().Exec(ctx,
		`DELETE FROM public.cognition_action_outcomes WHERE action_id=(SELECT id FROM public.cognition_frozen_actions WHERE inbox_id=(SELECT id FROM public.cognition_inbox WHERE idempotency_key=$1 AND event_type='conversation.turn' ORDER BY created_at DESC LIMIT 1))`,
		idempotencyKey); err != nil {
		t.Fatal(err)
	}
	payload[turnStagePayloadKey] = stage
	if rewrite != nil {
		rewrite(payload)
	}
	if _, err := repository.Pool().Exec(ctx,
		`UPDATE public.cognition_frozen_actions SET status='frozen',error_code=NULL,payload=$2::jsonb,realization_payload=NULL,completed_at=NULL WHERE inbox_id=(SELECT id FROM public.cognition_inbox WHERE idempotency_key=$1 AND event_type='conversation.turn' ORDER BY created_at DESC LIMIT 1)`,
		idempotencyKey, jsonBytes(payload)); err != nil {
		t.Fatal(err)
	}
}

// takeoverChainRunTurn drives one HandleTurn for a seeded chain fluctlight.
func takeoverChainRunTurn(t *testing.T, app *App, ctx context.Context, ownerID, conversationID, fluctlightID, text, idempotencyKey, turnID string) error {
	t.Helper()
	_, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, text, idempotencyKey, turnID))
	return err
}

// takeoverChainFrozenStatus reads the terminal status and failure code of a
// turn's frozen row.
func takeoverChainFrozenStatus(t *testing.T, ctx context.Context, repository *PostgresRepository, idempotencyKey string) (string, string) {
	t.Helper()
	var status, code string
	if err := repository.Pool().QueryRow(ctx,
		`SELECT status,COALESCE(error_code,'') FROM public.cognition_frozen_actions WHERE inbox_id=(SELECT id FROM public.cognition_inbox WHERE idempotency_key=$1 AND event_type='conversation.turn' ORDER BY created_at DESC LIMIT 1)`,
		idempotencyKey).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	return status, code
}

// takeoverChainAssertDeliveredOnce is the exactly-once delivery assertion shared
// by every recovery scenario.
func takeoverChainAssertDeliveredOnce(t *testing.T, ctx context.Context, repository *PostgresRepository, conversationID, turnID, want string) {
	t.Helper()
	texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, turnID)
	if len(texts) != 1 || texts[0] != want {
		t.Fatalf("the recovery must deliver %q exactly once, got %#v", want, texts)
	}
}

// takeoverChainApproveTurn is the shared setup: a turn whose Judge approves a
// handover and whose takeover reply is produced successfully. A rewind must
// replay it with takeoverChainApproveUserText.
func takeoverChainApproveTurn(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID, prefix string) {
	t.Helper()
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(takeoverChainCandidateText, nil))).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult(takeoverChainTakeoverText, nil))).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, takeoverChainApproveUserText, prefix+"-turn", prefix+"-turn-1"); err != nil {
		t.Fatal(err)
	}
}

// takeoverChainDeclineTurn is the shared setup for a turn whose Judge declines
// the handover, so the original candidate is the winner. It returns the user
// text the rewind must reuse.
func takeoverChainDeclineTurn(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID, prefix string) string {
	t.Helper()
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(takeoverChainCandidateText, nil))).
		on(takeoverJudgeSchemaName, takeoverChainJudge(false))
	app := newTestApp(t, repository, router)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, takeoverChainDeclineUserText, prefix+"-turn", prefix+"-turn-1"); err != nil {
		t.Fatal(err)
	}
	return takeoverChainDeclineUserText
}

const (
	takeoverChainCandidateText = "星火的候选回复"
	takeoverChainTakeoverText  = "暮光的接管回复"
	// A rewound turn MUST be replayed with the original user text: the inbox
	// fact is keyed by idempotency and its stored text is part of the identity
	// check, so a different text is rejected as ErrConflict before recovery can
	// even start. Keeping the texts in one place is what stops that trap from
	// being re-introduced by a copy-pasted literal.
	takeoverChainApproveUserText = "你根本没在听我说话。"
	takeoverChainDeclineUserText = "你在敷衍我吗？"
	// The takeover-pending scenario is driven twice (the failing attempt and the
	// restart), so its text is a constant for the same reason.
	takeoverChainPendingUserText = "你根本不在乎。"
)

// ---------------------------------------------------------------------------
// design.md 4.8 recovery enumeration, row by row
// ---------------------------------------------------------------------------

// Row 1: turn_stage = winner_ready → execute the decided winner, never re-judge.
func TestRecoveryFromWinnerReadyExecutesTheDecidedWinner(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "recover-wr-owner", "recover-wr-fluctlight", "recover-wr-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "recover-wr-turn", "recover-wr-turn-1"
	takeoverChainApproveTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, "recover-wr")

	// The crash happened between arbitration and Prepare: a winner exists and
	// nothing was executed or delivered.
	takeoverChainRewind(t, ctx, repository, conversationID, turnID, inboxKey, turnStageWinnerReady, nil)

	recovery := takeoverChainForbiddenCalls()
	app := newTestApp(t, repository, recovery)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "你根本没在听我说话。", inboxKey, turnID); err != nil {
		t.Fatal(err)
	}
	takeoverChainAssertNoModelCalls(t, recovery)
	takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainTakeoverText)

	if status, _ := takeoverChainFrozenStatus(t, ctx, repository, inboxKey); status != "completed" {
		t.Fatalf("the recovered turn must settle, got status %q", status)
	}
}

// Row 2: turn_stage = b_frozen → advance to the winner and execute it.
func TestRecoveryFromBFrozenExecutesTheTakeoverReply(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "recover-bf-owner", "recover-bf-fluctlight", "recover-bf-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "recover-bf-turn", "recover-bf-turn-1"
	takeoverChainApproveTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, "recover-bf")

	// The crash happened after the takeover reply was frozen but before the
	// turn became executable. Regenerating it would be a second verdict.
	takeoverChainRewind(t, ctx, repository, conversationID, turnID, inboxKey, turnStageBFrozen, nil)

	recovery := takeoverChainForbiddenCalls()
	app := newTestApp(t, repository, recovery)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "你根本没在听我说话。", inboxKey, turnID); err != nil {
		t.Fatal(err)
	}
	takeoverChainAssertNoModelCalls(t, recovery)
	takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainTakeoverText)
}

// Row 3: arbitration_decided + judge_kept_a → promote A, never re-judge.
func TestRecoveryFromArbitrationKeptAPromotesTheCandidate(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "recover-ka-owner", "recover-ka-fluctlight", "recover-ka-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "recover-ka-turn", "recover-ka-turn-1"
	takeoverChainDeclineTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, "recover-ka")

	// The Judge concluded but the conclusion was not yet promoted to a winner.
	takeoverChainRewind(t, ctx, repository, conversationID, turnID, inboxKey, turnStageArbitrationDecided, func(payload map[string]any) {
		record := mapValue(payload[turnTakeoverPayloadKey])
		record["decision"] = takeoverDecisionJudgeKeptA
		payload[turnTakeoverPayloadKey] = record
		// The promotion already wrote a winner; the crash predates it.
		delete(payload, turnWinnerPayloadKey)
		delete(payload, turnExpectedPayloadKey)
	})

	recovery := takeoverChainForbiddenCalls()
	app := newTestApp(t, repository, recovery)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, takeoverChainDeclineUserText, inboxKey, turnID); err != nil {
		t.Fatal(err)
	}
	takeoverChainAssertNoModelCalls(t, recovery)
	takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainCandidateText)

	frozen := takeoverChainFrozenPayload(t, ctx, repository, inboxKey)
	if winner := mapValue(frozen[turnWinnerPayloadKey]); stringValue(winner["source"]) != "a" {
		t.Fatalf("a persisted judge_kept_a must promote the original candidate: %#v", winner)
	}
}

// Row 4: arbitration_decided + judge_a_takeover_b_pending → continue generating
// the takeover reply from the persisted verdict, never re-judge.
func TestRecoveryFromPendingTakeoverContinuesWithoutReJudging(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "recover-pd-owner", "recover-pd-fluctlight", "recover-pd-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "recover-pd-turn", "recover-pd-turn-1"

	// The first attempt persists the Judge verdict and then dies inside the
	// takeover generation. This is exactly the recoverable window the stage
	// machine exists for.
	first := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(takeoverChainCandidateText, nil))).
		on(takeoverReplySchemaName, takeoverChainFailGeneration()).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, first)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, takeoverChainPendingUserText, inboxKey, turnID); err == nil {
		t.Fatal("a failing takeover generation must fail the turn")
	}
	if count := first.requestCount(takeoverJudgeSchemaName); count != 1 {
		t.Fatalf("the first attempt must consult the Judge once, got %d", count)
	}
	// The verdict survived the failure; only the process died.
	frozen := takeoverChainFrozenPayload(t, ctx, repository, inboxKey)
	if stage := turnStageOf(frozen); stage != turnStageArbitrationDecided {
		t.Fatalf("the persisted verdict must leave the turn at %q, got %q", turnStageArbitrationDecided, stage)
	}
	if decision := stringValue(mapValue(frozen[turnTakeoverPayloadKey])["decision"]); decision != takeoverDecisionJudgeTakeoverBPending {
		t.Fatalf("the persisted verdict must be %q, got %q", takeoverDecisionJudgeTakeoverBPending, decision)
	}

	// Restart: the frozen row is frozen again and the inbox fact is claimable.
	takeoverChainRewind(t, ctx, repository, conversationID, turnID, inboxKey, turnStageArbitrationDecided, nil)

	recovery := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainFailGeneration()).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult(takeoverChainTakeoverText, nil))).
		on(takeoverJudgeSchemaName, takeoverChainFailGeneration())
	restarted := newTestApp(t, repository, recovery)
	if err := takeoverChainRunTurn(t, restarted, ctx, ownerID, conversationID, fluctlightID, takeoverChainPendingUserText, inboxKey, turnID); err != nil {
		t.Fatal(err)
	}
	if count := recovery.requestCount(takeoverJudgeSchemaName); count != 0 {
		t.Fatalf("the resumed verdict must never be re-judged, got %d judge calls", count)
	}
	if count := recovery.requestCount(workingPersonaMainTurnSchema); count != 0 {
		t.Fatalf("the resumed verdict must never re-generate the candidate, got %d", count)
	}
	if count := recovery.requestCount(takeoverReplySchemaName); count != 1 {
		t.Fatalf("exactly the takeover reply must be generated, got %d", count)
	}
	takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainTakeoverText)
}

// Row 5: turn_stage = executing → resume the idempotent execution path without a
// second verdict and without re-arbitrating.
func TestRecoveryFromExecutingResumesWithoutANewVerdict(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "recover-ex-owner", "recover-ex-fluctlight", "recover-ex-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "recover-ex-turn", "recover-ex-turn-1"
	takeoverChainApproveTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, "recover-ex")

	// The crash happened inside the side-effect window: the winner was admitted
	// and Prepare had begun, but nothing was delivered.
	takeoverChainRewind(t, ctx, repository, conversationID, turnID, inboxKey, turnStageExecuting, nil)

	recovery := takeoverChainForbiddenCalls()
	app := newTestApp(t, repository, recovery)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "你根本没在听我说话。", inboxKey, turnID); err != nil {
		t.Fatal(err)
	}
	takeoverChainAssertNoModelCalls(t, recovery)
	takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainTakeoverText)

	if status, _ := takeoverChainFrozenStatus(t, ctx, repository, inboxKey); status != "completed" {
		t.Fatalf("a crashed-in-flight turn must be resumable to completion, got status %q", status)
	}
}

// Row 6: turn_stage = a_frozen with no arbitration record → arbitrate normally,
// because no verdict was ever persisted.
func TestRecoveryFromAFrozenReArbitratesExactlyOnce(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "recover-af-owner", "recover-af-fluctlight", "recover-af-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "recover-af-turn", "recover-af-turn-1"
	// The base must be a declined turn: its frozen decision IS the original
	// candidate, which is what an a_frozen payload means. Rewinding an approved
	// turn would leave the takeover reply in the payload and pretend it was the
	// candidate.
	takeoverChainDeclineTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, "recover-af")

	// The crash happened right after the candidate was frozen: there is no
	// arbitration record at all, so the turn legitimately arbitrates again.
	takeoverChainRewind(t, ctx, repository, conversationID, turnID, inboxKey, turnStageAFrozen, func(payload map[string]any) {
		delete(payload, turnTakeoverPayloadKey)
		delete(payload, turnWinnerPayloadKey)
		delete(payload, turnExpectedPayloadKey)
	})

	recovery := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainFailGeneration()).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult(takeoverChainTakeoverText, nil))).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, recovery)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, takeoverChainDeclineUserText, inboxKey, turnID); err != nil {
		t.Fatal(err)
	}
	if count := recovery.requestCount(takeoverJudgeSchemaName); count != 1 {
		t.Fatalf("an unarbitrated candidate must be judged exactly once, got %d", count)
	}
	if count := recovery.requestCount(workingPersonaMainTurnSchema); count != 0 {
		t.Fatalf("a frozen candidate must never be regenerated, got %d", count)
	}
	takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainTakeoverText)
}

// Row 7: an existing assistant message → replay the delivered turn, never the
// model.
func TestRecoveryWithExistingAssistantIsAReplay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "recover-rp-owner", "recover-rp-fluctlight", "recover-rp-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "recover-rp-turn", "recover-rp-turn-1"
	takeoverChainApproveTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, "recover-rp")

	// The same turn is re-submitted (a client retry after the response was lost).
	replay := takeoverChainForbiddenCalls()
	app := newTestApp(t, repository, replay)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "你根本没在听我说话。", inboxKey, turnID); err != nil {
		t.Fatal(err)
	}
	takeoverChainAssertNoModelCalls(t, replay)
	takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainTakeoverText)
	if status, _ := takeoverChainFrozenStatus(t, ctx, repository, inboxKey); status != "completed" {
		t.Fatalf("a replay must not disturb the settled turn, got status %q", status)
	}
}

// ---------------------------------------------------------------------------
// Concurrency: a stale snapshot must never overwrite newer state
// ---------------------------------------------------------------------------

// TestStaleSnapshotNeverOverwritesNewerState drives the exact race design.md
// 4.8 row 4 describes: a frozen candidate is recovered after a concurrent
// WakeUp advanced both the shared state revision and the persistent dominant
// profile. The stale snapshot must be refused instead of being committed.
func TestStaleSnapshotNeverOverwritesNewerState(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "stale-owner", "stale-fluctlight", "stale-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "stale-turn", "stale-turn-1"
	takeoverChainDeclineTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, "stale")

	takeoverChainRewind(t, ctx, repository, conversationID, turnID, inboxKey, turnStageWinnerReady, nil)

	// A concurrent WakeUp/Reflection advanced the fluctlight while the frozen
	// snapshot was in flight.
	if _, err := repository.Pool().Exec(ctx,
		`UPDATE public.fluctlight_inner_states SET revision=revision+1,last_updated_at=now() WHERE fluctlight_id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx,
		`UPDATE public.fluctlight_personality_runtime SET active_profile_id='twilight',revision=revision+1 WHERE fluctlight_id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	expectedRuntimeRevision := 1

	recovery := takeoverChainForbiddenCalls()
	app := newTestApp(t, repository, recovery)
	err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, takeoverChainDeclineUserText, inboxKey, turnID)
	if err == nil {
		t.Fatal("a stale frozen snapshot must not be committed over newer state")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Fatalf("a stale snapshot must be refused for staleness, got %v", err)
	}
	takeoverChainAssertNoModelCalls(t, recovery)
	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, turnID); len(texts) != 0 {
		t.Fatalf("a refused stale snapshot must deliver nothing, got %#v", texts)
	}
	// The newer state wins: the persistent dominant profile is still the one the
	// concurrent writer chose, at the same revision.
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "twilight" {
		t.Fatalf("the stale snapshot overwrote the persistent dominant profile: %q", active)
	}
	var runtimeRevision int
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&runtimeRevision); err != nil {
		t.Fatal(err)
	}
	if runtimeRevision != expectedRuntimeRevision {
		t.Fatalf("the stale snapshot advanced the persona revision to %d", runtimeRevision)
	}
	if status, code := takeoverChainFrozenStatus(t, ctx, repository, inboxKey); status != "failed" || code == "" {
		t.Fatalf("a refused settlement must quarantine with an exact code, got %q / %q", status, code)
	}
}

// ---------------------------------------------------------------------------
// Rejected candidate containment and the M10 consumer list
// ---------------------------------------------------------------------------

// TestRejectedCandidateLeavesNoFactTrail asserts the rejected candidate is kept
// for diagnostics and reaches no fact, memory or delivery surface.
func TestRejectedCandidateLeavesNoFactTrail(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "contain-owner", "contain-fluctlight", "contain-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "contain-turn", "contain-turn-1"

	// The candidate carries a full appraisal. It is evidence the model reasoned
	// about, not a fact: the Provider is structurally forbidden from proposing a
	// cognitive-state transition (context_reference.go), so a structured turn
	// cannot reach appraisal/state-revision persistence at all. The rejected
	// candidate therefore has no route into fact memory, whether it wins or not.
	candidate := map[string]any{}
	for _, field := range appraisalFields {
		candidate[field] = 0.5
	}
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(takeoverChainCandidateText, candidate))).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult(takeoverChainTakeoverText, nil))).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "你根本没在听我说话。", inboxKey, turnID); err != nil {
		t.Fatal(err)
	}

	takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainTakeoverText)
	for _, text := range takeoverChainAssistantTexts(t, ctx, repository, conversationID, turnID) {
		if strings.Contains(text, takeoverChainCandidateText) {
			t.Fatalf("the rejected candidate reached the conversation: %q", text)
		}
	}
	frozen := takeoverChainFrozenPayload(t, ctx, repository, inboxKey)
	if transition := stringValue(mapValue(frozen["decision"])["cognitive_state_transition"]); transition != "not_proposed" {
		t.Fatalf("a structured turn must never propose a cognitive-state transition, got %q", transition)
	}
	// The rejected candidate's proposal left no fact: no appraisal, no state
	// revision, no assistant message carrying its text.
	for _, query := range []string{
		`SELECT count(*) FROM public.cognition_appraisals WHERE source_fact_id=` + takeoverChainInboxPredicate,
		`SELECT count(*) FROM public.fluctlight_state_revisions WHERE source_event_id=` + takeoverChainInboxPredicate,
	} {
		if count := takeoverChainCount(t, ctx, repository, query, inboxKey); count != 0 {
			t.Fatalf("the rejected candidate left a fact behind (%d rows): %s", count, query)
		}
	}
	// It is still readable for diagnostics, and only there.
	rejected := mapValue(mapValue(frozen[turnTakeoverPayloadKey])["rejected_candidate"])
	if stringValue(rejected["visible_text"]) != takeoverChainCandidateText {
		t.Fatalf("the rejected candidate must stay readable for diagnostics: %#v", rejected)
	}
}

// TestAuditRowsHaveNoProductionConsumer pins M10: the rejected candidate's audit
// rows are written and never read, so they cannot be a path into reflection or
// memory. This is a structural fact about the production source, asserted as
// one, rather than a promise.
func TestAuditRowsHaveNoProductionConsumer(t *testing.T) {
	for _, name := range productionSourceFiles(t) {
		source, readErr := os.ReadFile(filepath.Clean(name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(source)
		for _, table := range []string{"cognition_assessments", "cognition_decision_proposals"} {
			if strings.Contains(text, "FROM public."+table) || strings.Contains(text, "JOIN public."+table) {
				t.Fatalf("%s is read by %s, but it is documented as a write-only audit row (M10)", table, name)
			}
		}
	}

	// The two writers must still exist: the guard above is only meaningful while
	// the audit rows are actually written.
	cognition, err := os.ReadFile("cognition.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"cognition_assessments", "cognition_decision_proposals"} {
		if !strings.Contains(string(cognition), "INSERT INTO public."+table) {
			t.Fatalf("%s is no longer written by the frozen-candidate path", table)
		}
	}

	// A rejected candidate must also never be READ by an execution or delivery
	// path: the only two occurrences are the diagnostics record and the payload
	// mirror that stores it. There is no payload lookup, so no execution or
	// delivery path can reach it.
	takeover, err := os.ReadFile("turn_takeover.go")
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(takeover), `["rejected_candidate"] =`); count != 2 {
		t.Fatalf("the rejected candidate is written by the record and mirrored into the payload: expected 2 assignments, found %d", count)
	}
	for _, name := range productionSourceFiles(t) {
		if name == "turn_takeover.go" {
			continue
		}
		source, readErr := os.ReadFile(filepath.Clean(name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(source), "rejected_candidate") {
			t.Fatalf("%s references the rejected candidate; only the diagnostics record may", name)
		}
	}
}

// productionSourceFiles lists the package's non-test Go sources.
func productionSourceFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		result = append(result, name)
	}
	if len(result) == 0 {
		t.Fatal("the production-source scan matched no file")
	}
	return result
}

// productionSourceText reads every non-test Go source of the package once, so a
// guard can make several precise per-file assertions without re-reading the
// tree. F10: a static count can only constrain *where* code lives — it cannot
// prove the absence of a loop, a re-entry or a Provider retry — so the count
// must at least be exact per file instead of a global "some file mentions it".
func productionSourceText(t *testing.T) map[string]string {
	t.Helper()
	result := make(map[string]string)
	for _, name := range productionSourceFiles(t) {
		source, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		result[name] = string(source)
	}
	return result
}

// assertProductionCount pins the exact occurrence count of one needle in one
// named production file.
func assertProductionCount(t *testing.T, sources map[string]string, file, needle string, want int) {
	t.Helper()
	source, ok := sources[file]
	if !ok {
		t.Fatalf("%s is not a production source of this package", file)
	}
	if got := strings.Count(source, needle); got != want {
		t.Fatalf("%s must contain %q exactly %d time(s), found %d", file, needle, want, got)
	}
}

// assertProductionOnlyIn pins a needle to exactly one production file: any other
// file mentioning it is a failure even before the total is compared. This is
// what distinguishes "the single call site moved" from "a second call site
// appeared" — the difference a global total cannot see.
func assertProductionOnlyIn(t *testing.T, sources map[string]string, needle, onlyFile string, want int) {
	t.Helper()
	total := 0
	for name, source := range sources {
		count := strings.Count(source, needle)
		if count == 0 {
			continue
		}
		if name != onlyFile {
			t.Fatalf("%q may only appear in %s, also found in %s (%d time(s))", needle, onlyFile, name, count)
		}
		total += count
	}
	if total != want {
		t.Fatalf("%q must appear exactly %d time(s) in %s, found %d", needle, want, onlyFile, total)
	}
}

// assertProductionAbsent fails when any production file contains the needle.
func assertProductionAbsent(t *testing.T, sources map[string]string, needle string) {
	t.Helper()
	for name, source := range sources {
		if strings.Contains(source, needle) {
			t.Fatalf("%q must not appear in production code, found in %s", needle, name)
		}
	}
}
