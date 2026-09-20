package core

import (
	"context"
	"strings"
	"testing"
)

// This file is the phase-10 runtime budget and stage verification (implement.md
// phase 10 / design.md 8, F10).
//
// The static guards in capability_core_test.go can only constrain *where* code
// lives. They cannot prove the absence of a loop, of a re-entry, or of a
// Provider retry — so three runtime properties are asserted here instead:
//
//  1. the actual call ORDER and the stage machine's terminal state, not just
//     per-role totals;
//  2. the separation between logical stages and physical HTTP attempts, so a
//     retry cannot hide inside a "one call per stage" claim;
//  3. that a recovery never replays a stage that already completed, with a
//     control row proving the assertion is not vacuous.

// takeoverChainQuerySynthesisText is what the no-Tools synthesis returns, so a
// pure-query turn's delivered text is distinguishable from the candidate's.
const takeoverChainQuerySynthesisText = "我记得那件事。"

// turnBudgetSchemaSequence projects the complete wire sequence onto the
// interactive Main/Judge/Takeover/Query budget. The ADK tool-result round is
// a second physical conversation_turn_response request belonging to the same
// logical Main stage, while persistent_switch_assessment is an independent
// post-cognition stage that does not consume the main-generation budget.
func turnBudgetSchemaSequence(sequence []string) []string {
	projected := make([]string, 0, len(sequence))
	for _, schemaName := range sequence {
		if schemaName == persistentSwitchAssessmentSchemaName {
			continue
		}
		if schemaName == workingPersonaMainTurnSchema && len(projected) > 0 && projected[len(projected)-1] == schemaName {
			continue
		}
		projected = append(projected, schemaName)
	}
	return projected
}

// TestTurnStageBudgetNeverExceedsTwoMainGenerations asserts the budget table of
// design.md 4.7 against the ACTUAL wire sequence. Counts alone would accept a
// turn that generated A twice, or that consulted the Judge after deciding to
// hand over; the ordered sequence makes both observable.
func TestTurnStageBudgetNeverExceedsTwoMainGenerations(t *testing.T) {
	cases := []struct {
		name                 string
		rules                []any
		candidate            fakeProviderResult
		judge                bool
		wantPhysicalSequence []string
		wantBudgetSequence   []string
		wantDelivered        string
	}{
		{
			name:      "no rule pays for one generation and no judge",
			rules:     nil,
			candidate: takeoverChainMainResult(takeoverChainCandidateText, nil),
			// [1] of the budget table: no Judge, no second generation.
			wantPhysicalSequence: []string{workingPersonaMainTurnSchema, persistentSwitchAssessmentSchemaName},
			wantBudgetSequence:   []string{workingPersonaMainTurnSchema},
			wantDelivered:        takeoverChainCandidateText,
		},
		{
			name:      "a declined judge is metered outside the main budget",
			rules:     takeoverChainDefaultRules(),
			candidate: takeoverChainMainResult(takeoverChainCandidateText, nil),
			judge:     false,
			// [2]: A + Judge. The Judge is a separate role and does not consume
			// one of the two main generations.
			wantPhysicalSequence: []string{workingPersonaMainTurnSchema, persistentSwitchAssessmentSchemaName, takeoverJudgeSchemaName},
			wantBudgetSequence:   []string{workingPersonaMainTurnSchema, takeoverJudgeSchemaName},
			wantDelivered:        takeoverChainCandidateText,
		},
		{
			name:      "an approved judge adds exactly one second generation",
			rules:     takeoverChainDefaultRules(),
			candidate: takeoverChainMainResult(takeoverChainCandidateText, nil),
			judge:     true,
			// [3]: A + Judge + B, and the Judge is never consulted again.
			wantPhysicalSequence: []string{workingPersonaMainTurnSchema, persistentSwitchAssessmentSchemaName, takeoverJudgeSchemaName, takeoverReplySchemaName},
			wantBudgetSequence:   []string{workingPersonaMainTurnSchema, takeoverJudgeSchemaName, takeoverReplySchemaName},
			wantDelivered:        takeoverChainTakeoverText,
		},
		{
			name:      "a pure query skips arbitration and pays only for synthesis",
			rules:     takeoverChainDefaultRules(),
			candidate: takeoverChainPureQueryCandidate(),
			// [4]: the result-dependent continuation path skips the whole
			// arbitration block, so no Judge and no takeover generation appear.
			wantPhysicalSequence: []string{workingPersonaMainTurnSchema, workingPersonaMainTurnSchema, persistentSwitchAssessmentSchemaName, queryContinuationTestSchemaName},
			wantBudgetSequence:   []string{workingPersonaMainTurnSchema, queryContinuationTestSchemaName},
			wantDelivered:        takeoverChainQuerySynthesisText,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			slug := strings.ReplaceAll(testCase.name, " ", "-")
			ownerID, fluctlightID, conversationID := "stage-budget-owner-"+slug, "stage-budget-fluctlight-"+slug, "stage-budget-conversation-"+slug
			takeoverChainSeedWithTakeoverRules(t, ctx, repository, ownerID, fluctlightID, conversationID, testCase.rules)

			router := newFakeProviderRouter().
				on(workingPersonaMainTurnSchema, takeoverChainSequence(testCase.candidate)).
				on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult(takeoverChainTakeoverText, nil))).
				on(takeoverJudgeSchemaName, takeoverChainJudge(testCase.judge)).
				on(persistentSwitchAssessmentSchemaName, takeoverChainPersistentSwitchKeep()).
				on(queryContinuationTestSchemaName, func(map[string]any) fakeProviderResult {
					return fakeProviderResult{Structured: map[string]any{"visible_text": takeoverChainQuerySynthesisText}}
				})
			app := newTestApp(t, repository, router)
			inboxKey, turnID := "stage-budget-turn-"+slug, "stage-budget-turn-"+slug+"-1"
			if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "你在敷衍我吗？", inboxKey, turnID); err != nil {
				t.Fatal(err)
			}

			// First retain the exact physical wire sequence, including the
			// independent post-cognition assessment and any ADK tool-result round.
			// equalStrings is shared with settings_provider_models_test.go.
			physical := router.schemaSequence()
			if !equalStrings(physical, testCase.wantPhysicalSequence) {
				t.Fatalf("the turn issued physical sequence %#v, expected exactly %#v", physical, testCase.wantPhysicalSequence)
			}
			// Then project that sequence onto the Main/Judge/Takeover/Query
			// budget. This deliberately removes only the known auxiliary stage
			// and collapses the ADK tool-result round; logical stage counters below
			// still make any repeated Main stage observable.
			if budget := turnBudgetSchemaSequence(physical); !equalStrings(budget, testCase.wantBudgetSequence) {
				t.Fatalf("the turn budget sequence %#v, expected exactly %#v", budget, testCase.wantBudgetSequence)
			}
			takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, testCase.wantDelivered)

			// The main-generation budget is two, always.
			mainGenerations := router.logicalRequestCount(workingPersonaMainTurnSchema) + router.logicalRequestCount(takeoverReplySchemaName)
			if mainGenerations > 2 {
				t.Fatalf("the main-generation budget is two, spent %d", mainGenerations)
			}
			// Every stage runs at most once: a repeated stage would be a replay
			// or an arbitration loop wearing a correct total.
			for _, schemaName := range takeoverChainModelRoles() {
				if count := router.logicalRequestCount(schemaName); count > 1 {
					t.Fatalf("stage %s ran %d times; the stage machine allows one run each", schemaName, count)
				}
			}
			if count := router.logicalRequestCount(persistentSwitchAssessmentSchemaName); count != 1 {
				t.Fatalf("the independent post-cognition assessment must run once, got %d", count)
			}
			// The turn really entered the side-effect window and settled.
			frozen := takeoverChainFrozenPayload(t, ctx, repository, inboxKey)
			takeoverChainAssertExecutionWindow(t, frozen)
			if status, _ := takeoverChainFrozenStatus(t, ctx, repository, inboxKey); status != "completed" {
				t.Fatalf("a settled turn must be completed, got %q", status)
			}
		})
	}
}

// TestLogicalInvocationsVsPhysicalAttempts pins F10's separation: a turn's
// logical stages are counted apart from the HTTP attempts those stages cost,
// and a failing stage is a bounded failure rather than a retry amplifier.
//
// The distinction matters because "one call per stage" is only meaningful if
// the transport cannot silently turn it into three. The recovery tests assert
// "zero physical attempts" on the replay paths; this test asserts the positive
// direction on the live paths.
func TestLogicalInvocationsVsPhysicalAttempts(t *testing.T) {
	t.Run("a deterministic turn spends one attempt per logical stage", func(t *testing.T) {
		ctx, repository := isolatedCoreTestRepository(t)
		ownerID, fluctlightID, conversationID := "physical-ok-owner", "physical-ok-fluctlight", "physical-ok-conversation"
		takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

		router := newFakeProviderRouter().
			on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(takeoverChainCandidateText, nil))).
			on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult(takeoverChainTakeoverText, nil))).
			on(takeoverJudgeSchemaName, takeoverChainJudge(true)).
			on(persistentSwitchAssessmentSchemaName, takeoverChainPersistentSwitchKeep())
		app := newTestApp(t, repository, router)
		if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "你根本没在听我说话。", "physical-ok-turn", "physical-ok-turn-1"); err != nil {
			t.Fatal(err)
		}

		logical := map[string]int{
			workingPersonaMainTurnSchema:         router.logicalRequestCount(workingPersonaMainTurnSchema),
			takeoverJudgeSchemaName:              router.requestCount(takeoverJudgeSchemaName),
			takeoverReplySchemaName:              router.logicalRequestCount(takeoverReplySchemaName),
			persistentSwitchAssessmentSchemaName: router.logicalRequestCount(persistentSwitchAssessmentSchemaName),
		}
		if logical[workingPersonaMainTurnSchema] != 1 || logical[takeoverJudgeSchemaName] != 1 || logical[takeoverReplySchemaName] != 1 || logical[persistentSwitchAssessmentSchemaName] != 1 {
			t.Fatalf("expected one attempt per logical stage, got %#v", logical)
		}
		attributed := 0
		for _, count := range logical {
			attributed += count
		}
		// The physical counter is independent of the logical one. It agrees
		// only because nothing retried; an unattributed attempt would make the
		// per-stage counters lie about cost.
		if physical := router.totalRequests(); physical != attributed {
			t.Fatalf("physical attempts=%d, logical stages=%d; an unattributed attempt means the stage counters are not authoritative", physical, attributed)
		}
		if unattributed := router.unattributedRequests(); unattributed != 0 {
			t.Fatalf("%d HTTP attempts belonged to no logical stage", unattributed)
		}
	})

	// A Judge that fails must degrade, not retry. A transport failure and a
	// timeout are both checked because they take different paths inside the
	// Judge client but must share the same bounded cost.
	for _, testCase := range []struct {
		name    string
		judge   fakeProviderScript
		outcome string
	}{
		{name: "unavailable", judge: takeoverChainFailGeneration(), outcome: takeoverJudgeOutcomeUnavailable},
		{name: "timeout", judge: func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Err: context.DeadlineExceeded}
		}, outcome: takeoverJudgeOutcomeTimeout},
	} {
		t.Run("a "+testCase.name+" judge keeps the candidate at bounded cost", func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			slug := strings.ReplaceAll(testCase.name, " ", "-")
			ownerID, fluctlightID, conversationID := "physical-judge-owner-"+slug, "physical-judge-fluctlight-"+slug, "physical-judge-conversation-"+slug
			takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

			router := newFakeProviderRouter().
				on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(takeoverChainCandidateText, nil))).
				on(takeoverReplySchemaName, takeoverChainFailGeneration()).
				on(takeoverJudgeSchemaName, testCase.judge).
				on(persistentSwitchAssessmentSchemaName, takeoverChainPersistentSwitchKeep())
			app := newTestApp(t, repository, router)
			inboxKey, turnID := "physical-judge-turn-"+slug, "physical-judge-turn-"+slug+"-1"
			if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "你在敷衍我吗？", inboxKey, turnID); err != nil {
				t.Fatal(err)
			}

			// Exactly one Judge attempt: the failure is not retried, so a
			// flaky Judge cannot multiply cost or latency.
			if count := router.requestCount(takeoverJudgeSchemaName); count != 1 {
				t.Fatalf("a failing judge must be attempted exactly once, got %d", count)
			}
			if count := router.requestCount(takeoverReplySchemaName); count != 0 {
				t.Fatalf("a degraded judge must never grow into a second generation, got %d", count)
			}
			if unattributed := router.unattributedRequests(); unattributed != 0 {
				t.Fatalf("%d HTTP attempts belonged to no logical stage", unattributed)
			}
			// The physical attempt count is bounded by the number of stages the
			// turn may run: no retry was appended to the failure.
			if physical := router.totalRequests(); physical != 3 {
				t.Fatalf("a failing judge must cost A + one post-cognition assessment + one judge attempt, saw %d physical attempts", physical)
			}
			takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainCandidateText)

			frozen := takeoverChainFrozenPayload(t, ctx, repository, inboxKey)
			record := mapValue(frozen[turnTakeoverPayloadKey])
			if decision := stringValue(record["decision"]); decision != takeoverDecisionJudgeDegraded {
				t.Fatalf("a judge failure must be recorded as %q, got %q", takeoverDecisionJudgeDegraded, decision)
			}
			if outcome := stringValue(mapValue(record["judge"])["outcome"]); outcome != testCase.outcome {
				t.Fatalf("judge outcome=%q, expected %q", outcome, testCase.outcome)
			}
		})
	}

	// The strongest budget enforcement is the one that never spends an attempt:
	// an over-budget Judge packet is refused locally, before the network, and the
	// validated candidate is kept. This is also why the packet is never
	// truncated silently — a truncated packet would be a cheaper but dishonest
	// verdict.
	t.Run("an over-budget judge packet is refused before the network", func(t *testing.T) {
		ctx, repository := isolatedCoreTestRepository(t)
		ownerID, fluctlightID, conversationID := "physical-overbudget-owner", "physical-overbudget-fluctlight", "physical-overbudget-conversation"
		takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

		// estimateRawTextTokens charges ~1.25 tokens per rune, and the Judge
		// input budget is 2048 tokens, so this reply is far over budget. The
		// canonical visible text is trimmed, so the expectation is trimmed too.
		longReply := strings.TrimSpace(strings.Repeat("this reply is intentionally far too long. ", 100))
		router := newFakeProviderRouter().
			on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(longReply, nil))).
			on(takeoverReplySchemaName, takeoverChainFailGeneration()).
			on(takeoverJudgeSchemaName, takeoverChainFailGeneration()).
			on(persistentSwitchAssessmentSchemaName, takeoverChainPersistentSwitchKeep())
		app := newTestApp(t, repository, router)
		inboxKey, turnID := "physical-overbudget-turn", "physical-overbudget-turn-1"
		if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "你在敷衍我吗？", inboxKey, turnID); err != nil {
			t.Fatal(err)
		}

		if count := router.requestCount(takeoverJudgeSchemaName); count != 0 {
			t.Fatalf("an over-budget judge packet must not cost a physical attempt, saw %d", count)
		}
		if physical := router.totalRequests(); physical != 2 {
			t.Fatalf("the turn must spend its main generation plus the independent post-cognition assessment, saw %d physical attempts", physical)
		}
		takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, longReply)

		record := mapValue(takeoverChainFrozenPayload(t, ctx, repository, inboxKey)[turnTakeoverPayloadKey])
		if decision := stringValue(record["decision"]); decision != takeoverDecisionJudgeDegraded {
			t.Fatalf("an over-budget judge must degrade, got decision %q", decision)
		}
		if outcome := stringValue(mapValue(record["judge"])["outcome"]); outcome != takeoverJudgeOutcomeBudgetExceeded {
			t.Fatalf("judge outcome=%q, expected %q", outcome, takeoverJudgeOutcomeBudgetExceeded)
		}
	})

	// The remaining degradation, "invalid output", is pinned here as a fact
	// rather than assumed: the Provider normalizes the response against the
	// declared schema and fills required fields with defaults, so a wrong-typed
	// `takeover` arrives as a valid boolean and is recorded as a DECLINE — not
	// as an invalid output. `takeoverJudgeOutcomeInvalidOutput` is therefore a
	// defensive branch, reachable only if a future schema stops declaring a
	// required boolean. Asserting the coercion is what keeps that honest.
	t.Run("a schema-invalid judge reply is coerced, not reported as invalid output", func(t *testing.T) {
		ctx, repository := isolatedCoreTestRepository(t)
		ownerID, fluctlightID, conversationID := "physical-coerced-owner", "physical-coerced-fluctlight", "physical-coerced-conversation"
		takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

		router := newFakeProviderRouter().
			on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(takeoverChainCandidateText, nil))).
			on(takeoverReplySchemaName, takeoverChainFailGeneration()).
			on(takeoverJudgeSchemaName, func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Structured: map[string]any{"takeover": "yes"}}
			}).
			on(persistentSwitchAssessmentSchemaName, takeoverChainPersistentSwitchKeep())
		app := newTestApp(t, repository, router)
		inboxKey, turnID := "physical-coerced-turn", "physical-coerced-turn-1"
		if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "你在敷衍我吗？", inboxKey, turnID); err != nil {
			t.Fatal(err)
		}

		takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainCandidateText)
		record := mapValue(takeoverChainFrozenPayload(t, ctx, repository, inboxKey)[turnTakeoverPayloadKey])
		if decision := stringValue(record["decision"]); decision != takeoverDecisionJudgeKeptA {
			t.Fatalf("a coerced decline keeps the candidate, got decision %q", decision)
		}
		if outcome := stringValue(mapValue(record["judge"])["outcome"]); outcome != takeoverJudgeOutcomeDeclined {
			t.Fatalf("judge outcome=%q, expected the coerced %q", outcome, takeoverJudgeOutcomeDeclined)
		}
	})
}

// TestRecoveryDoesNotReplayCompletedStages is the consolidated form of the
// design.md 4.8 recovery enumeration. Each row rewinds a finished turn to one
// stage boundary and asserts which model stages the recovery may still run;
// every other stage is scripted to fail loudly so an unexpected call is counted
// rather than absorbed.
//
// The last row is the control: a crash at a_frozen persisted no verdict, so the
// turn must arbitrate again. Without it, "the recovery issued no model call"
// could pass for the wrong reason.
func TestRecoveryDoesNotReplayCompletedStages(t *testing.T) {
	cases := []struct {
		name string
		// prefix is the idempotency-key prefix the baseline turn and the rewind
		// share; the turn id is prefix+"-turn-1".
		prefix string
		// prepare runs the baseline turn and the crash rewind, leaving the
		// durable state exactly as a crash at this boundary would.
		prepare func(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID, prefix string)
		// userText must be the baseline turn's text: the inbox fact's stored
		// text is part of its idempotency identity.
		userText string
		// allowed is the model stages the recovery may still run, and how many
		// times. An empty map means the recovery must reach a verdict and a
		// delivery with zero model calls.
		allowed map[string]int
		// wantDelivered is the text the recovery must deliver exactly once.
		wantDelivered string
	}{
		{
			name:   "an already decided winner is executed, never re-arbitrated",
			prefix: "replay-wr",
			prepare: func(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID, prefix string) {
				takeoverChainApproveTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, prefix)
				takeoverChainRewind(t, ctx, repository, conversationID, prefix+"-turn-1", prefix+"-turn", turnStageWinnerReady, nil)
			},
			userText:      takeoverChainApproveUserText,
			allowed:       map[string]int{},
			wantDelivered: takeoverChainTakeoverText,
		},
		{
			name:   "an already generated takeover reply is promoted, never regenerated",
			prefix: "replay-bf",
			prepare: func(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID, prefix string) {
				takeoverChainApproveTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, prefix)
				takeoverChainRewind(t, ctx, repository, conversationID, prefix+"-turn-1", prefix+"-turn", turnStageBFrozen, nil)
			},
			userText:      takeoverChainApproveUserText,
			allowed:       map[string]int{},
			wantDelivered: takeoverChainTakeoverText,
		},
		{
			name:   "a turn admitted into the execution window is resumed idempotently",
			prefix: "replay-ex",
			prepare: func(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID, prefix string) {
				takeoverChainApproveTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, prefix)
				takeoverChainRewind(t, ctx, repository, conversationID, prefix+"-turn-1", prefix+"-turn", turnStageExecuting, nil)
			},
			userText:      takeoverChainApproveUserText,
			allowed:       map[string]int{},
			wantDelivered: takeoverChainTakeoverText,
		},
		{
			name:   "a persisted judge_kept_a promotes the candidate without a new verdict",
			prefix: "replay-ka",
			prepare: func(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID, prefix string) {
				takeoverChainDeclineTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, prefix)
				takeoverChainRewind(t, ctx, repository, conversationID, prefix+"-turn-1", prefix+"-turn", turnStageArbitrationDecided, func(payload map[string]any) {
					record := mapValue(payload[turnTakeoverPayloadKey])
					record["decision"] = takeoverDecisionJudgeKeptA
					payload[turnTakeoverPayloadKey] = record
					delete(payload, turnWinnerPayloadKey)
					delete(payload, turnExpectedPayloadKey)
				})
			},
			userText:      takeoverChainDeclineUserText,
			allowed:       map[string]int{},
			wantDelivered: takeoverChainCandidateText,
		},
		{
			name:   "a persisted takeover verdict finishes the second generation only",
			prefix: "replay-pd",
			prepare: func(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID, prefix string) {
				// The first attempt dies inside the takeover generation, which
				// is exactly the window the stage machine exists for.
				first := newFakeProviderRouter().
					on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(takeoverChainCandidateText, nil))).
					on(takeoverReplySchemaName, takeoverChainFailGeneration()).
					on(takeoverJudgeSchemaName, takeoverChainJudge(true))
				app := newTestApp(t, repository, first)
				if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, takeoverChainPendingUserText, prefix+"-turn", prefix+"-turn-1"); err == nil {
					t.Fatal("a failing takeover generation must fail the turn")
				}
				if count := first.requestCount(takeoverJudgeSchemaName); count != 1 {
					t.Fatalf("the first attempt must consult the Judge once, got %d", count)
				}
				takeoverChainRewind(t, ctx, repository, conversationID, prefix+"-turn-1", prefix+"-turn", turnStageArbitrationDecided, nil)
			},
			userText:      takeoverChainPendingUserText,
			allowed:       map[string]int{takeoverReplySchemaName: 1},
			wantDelivered: takeoverChainTakeoverText,
		},
		{
			// Control row: nothing was decided, so arbitrating again is correct.
			name:   "an unarbitrated candidate arbitrates exactly once",
			prefix: "replay-af",
			prepare: func(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID, prefix string) {
				takeoverChainDeclineTurn(t, ctx, repository, ownerID, fluctlightID, conversationID, prefix)
				takeoverChainRewind(t, ctx, repository, conversationID, prefix+"-turn-1", prefix+"-turn", turnStageAFrozen, func(payload map[string]any) {
					delete(payload, turnTakeoverPayloadKey)
					delete(payload, turnWinnerPayloadKey)
					delete(payload, turnExpectedPayloadKey)
				})
			},
			userText:      takeoverChainDeclineUserText,
			allowed:       map[string]int{takeoverJudgeSchemaName: 1},
			wantDelivered: takeoverChainCandidateText,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			ownerID := "replay-owner-" + testCase.prefix
			fluctlightID := "replay-fluctlight-" + testCase.prefix
			conversationID := "replay-conversation-" + testCase.prefix
			takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

			testCase.prepare(t, ctx, repository, ownerID, fluctlightID, conversationID, testCase.prefix)
			inboxKey, turnID := testCase.prefix+"-turn", testCase.prefix+"-turn-1"

			// Every stage outside `allowed` fails loudly, so an unexpected call
			// is both counted and visible.
			recovery := takeoverChainForbiddenCalls()
			if testCase.allowed[takeoverReplySchemaName] > 0 {
				recovery = recovery.on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult(takeoverChainTakeoverText, nil)))
			}
			if testCase.allowed[takeoverJudgeSchemaName] > 0 {
				recovery = recovery.on(takeoverJudgeSchemaName, takeoverChainJudge(false))
			}
			app := newTestApp(t, repository, recovery)
			if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, testCase.userText, inboxKey, turnID); err != nil {
				t.Fatal(err)
			}

			// The recovery ran exactly the stages it was still allowed to run.
			for _, schemaName := range takeoverChainModelRoles() {
				got := recovery.requestCount(schemaName)
				if want := testCase.allowed[schemaName]; got != want {
					t.Fatalf("the recovery ran %s %d time(s), expected %d", schemaName, got, want)
				}
			}
			// It also spent no unattributed physical attempt, so a failure
			// hidden behind a stage counter is impossible.
			expectedAttempts := 0
			for _, count := range testCase.allowed {
				expectedAttempts += count
			}
			if got := recovery.totalRequests(); got != expectedAttempts {
				t.Fatalf("the recovery issued %d HTTP attempt(s), expected %d", got, expectedAttempts)
			}
			// A completed verdict is never re-decided: every row except the
			// control must reach its verdict with zero Judge calls.
			judgeCalls := recovery.requestCount(takeoverJudgeSchemaName)
			if testCase.allowed[takeoverJudgeSchemaName] == 0 && judgeCalls != 0 {
				t.Fatalf("a persisted verdict was re-judged %d time(s)", judgeCalls)
			}
			takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, testCase.wantDelivered)
			if status, code := takeoverChainFrozenStatus(t, ctx, repository, inboxKey); status != "completed" {
				t.Fatalf("the recovered turn must settle, got status %q code %q", status, code)
			}
			// Nothing was delivered twice: the rewind removed the assistant
			// message, so a duplicate here would be a real double send.
			if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, turnID); len(texts) != 1 {
				t.Fatalf("the recovery delivered %d assistant messages, expected exactly 1", len(texts))
			}
		})
	}
}
