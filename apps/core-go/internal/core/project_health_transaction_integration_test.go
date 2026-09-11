package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func attachEmptyFrozenCausalityForTest(decision map[string]any, fluctlightID, ownerActorID, sourceFactID string) {
	projection, ok := contextProjectionFromValue(decision["context_projection"])
	if !ok {
		projection = ContextProjection{}
	}
	projection.SchemaVersion, projection.FluctlightID, projection.OwnerActorID, projection.SourceFactID = "fluctlight.context.v3", fluctlightID, ownerActorID, sourceFactID
	projection.CurrentSpeaker = map[string]any{"actor_id": ownerActorID}
	projection.ReferenceIndex = newContextReferenceIndex(projection)
	decision["context_reference_version"] = contextReferenceIndexVersion
	decision["context_reference_index"] = projection.ReferenceIndex
	decision["context_projection"] = projection
	decision["influences"] = []any{}
	decision["goal_refs"] = []any{}
	decision["intention_refs"] = []any{}
}

type projectHealthRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn projectHealthRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type nativeReplayTestCapability struct {
	prepareCalls atomic.Int32
	txCalls      atomic.Int32
	failFirst    atomic.Bool
}

type autonomyMutationTestCapability struct{ settingKey string }
type autonomyFailureTestCapability struct{}

func (capability autonomyMutationTestCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "test.autonomy.mutate", Version: "v1", Type: CapabilityTypeInternal,
		Description: "Write a test mutation through the caller transaction.",
		InputSchema: map[string]any{"type": "object", "additionalProperties": false}, OutputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []any{"applied"}, "properties": map[string]any{"applied": map[string]any{"type": "boolean"}}},
		Surfaces: []CapabilitySurface{CapabilitySurfaceAutonomy}, SuccessBoundary: "test_mutation_committed", FailurePolicy: FailurePolicyOptionalInternal,
	}
}

func (capability autonomyMutationTestCapability) RequiredContext() []ContextSlot { return nil }
func (capability autonomyMutationTestCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
}
func (capability autonomyMutationTestCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES($1,'{"applied":true}')`, capability.settingKey); err != nil {
		return failedCapabilityResult(invocation, "test_mutation_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"applied": true}, ProviderRequestID: invocation.ProviderRequestID}, nil
}

func (autonomyFailureTestCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "test.autonomy.fail", Version: "v1", Type: CapabilityTypeInternal,
		Description: "Fail a required autonomy mutation after a sibling executes.",
		InputSchema: map[string]any{"type": "object", "additionalProperties": false}, OutputSchema: map[string]any{"type": "object", "additionalProperties": false},
		Surfaces: []CapabilitySurface{CapabilitySurfaceAutonomy}, SuccessBoundary: "test_failure_never_commits", FailurePolicy: FailurePolicyRequiredForVisibleClaim,
	}
}

func (autonomyFailureTestCapability) RequiredContext() []ContextSlot { return nil }
func (autonomyFailureTestCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
}
func (autonomyFailureTestCapability) ExecuteTx(_ context.Context, _ pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return failedCapabilityResult(invocation, "autonomy_test_required_failure", false), newCapabilityError("autonomy_test_required_failure", false, fmt.Errorf("required test failure"))
}

func (capability *nativeReplayTestCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "test.native.transaction", Version: "v1", Type: CapabilityTypeInternal,
		Description:     "Exercise the native caller-owned transaction boundary.",
		InputSchema:     map[string]any{"type": "object", "additionalProperties": false},
		OutputSchema:    map[string]any{"type": "object", "additionalProperties": false, "required": []any{"applied"}, "properties": map[string]any{"applied": map[string]any{"type": "boolean"}}},
		Surfaces:        []CapabilitySurface{CapabilitySurfaceNativeCognition},
		SuccessBoundary: "native_test_applied",
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
	}
}

func (capability *nativeReplayTestCapability) RequiredContext() []ContextSlot { return nil }

func (capability *nativeReplayTestCapability) Prepare(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	capability.prepareCalls.Add(1)
	return withCapabilityPreparedData(invocation, "native_test_plan", map[string]any{"stable": true})
}

func (capability *nativeReplayTestCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return failedCapabilityResult(invocation, "native_test_wrong_execution_path", false), newCapabilityError("native_test_wrong_execution_path", false, fmt.Errorf("transactional capability executed outside caller transaction"))
}

func (capability *nativeReplayTestCapability) ExecuteTx(_ context.Context, _ pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	capability.txCalls.Add(1)
	if _, found, err := capabilityPreparedData(invocation, "native_test_plan"); err != nil || !found {
		if err == nil {
			err = fmt.Errorf("prepared plan missing")
		}
		return failedCapabilityResult(invocation, "native_test_plan_invalid", false), newCapabilityError("native_test_plan_invalid", false, err)
	}
	if capability.failFirst.CompareAndSwap(true, false) {
		return failedCapabilityResult(invocation, "native_test_retryable", true), newCapabilityError("native_test_retryable", true, fmt.Errorf("retryable native failure"))
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"applied": true}, ProviderRequestID: invocation.ProviderRequestID}, nil
}

func isolatedCoreTestRepository(t *testing.T) (context.Context, *PostgresRepository) {
	t.Helper()
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	adminConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	adminConfig.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	databaseName := "lac_core_" + stableDigest(fmt.Sprintf("%d", time.Now().UnixNano()))[:20]
	if !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(databaseName) {
		adminPool.Close()
		t.Fatalf("unsafe temporary database name %q", databaseName)
	}
	identifier := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}
	testConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP DATABASE "+identifier)
		adminPool.Close()
		t.Fatal(err)
	}
	testConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP DATABASE "+identifier)
		adminPool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if _, dropErr := adminPool.Exec(cleanupCtx, "DROP DATABASE "+identifier); dropErr != nil {
			t.Errorf("drop isolated PostgreSQL database %s: %v", databaseName, dropErr)
		}
		adminPool.Close()
	})
	if err := migrations.New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	return ctx, &PostgresRepository{pool: pool}
}

func TestActionOutcomePersistsPerCallAndAsyncCompletionExactlyOnce(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	fluctlightID := "outcome-fluctlight-" + suffix
	actionID := "outcome-action-" + suffix
	externalRef := "outcome-media-" + suffix
	app := &App{DB: repository}
	app.Capabilities = mustCapabilityRegistry(imageGenerateCapability{service: app})
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.platform_outbox_events WHERE aggregate_id IN ($1,$2) OR fluctlight_id=$3`, actionID, "outcome_"+stableDigest(actionID+"\x1fimage-call"), fluctlightID)
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.platform_workflow_intents WHERE payload->>'fluctlight_id'=$1`, fluctlightID)
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.cognition_inbox WHERE fluctlight_id=$1`, fluctlightID)
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.cognition_inbox_heads WHERE fluctlight_id=$1`, fluctlightID)
		_, _ = repository.Pool().Exec(cleanupCtx, `DELETE FROM public.cognition_action_outcomes WHERE action_id=$1`, actionID)
	})

	results := []CapabilityResult{{
		CallID: "image-call", CapabilityName: "media.image.generate", Status: "completed",
		Output: map[string]any{"media_intent_id": externalRef, "target_kind": "conversation_message", "target_ref": "message-1"},
	}}
	outcomes, err := buildActionOutcomes(actionID, fluctlightID, "source-fact", "reply", results, map[string]any{"status": "completed"}, app.Capabilities)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var primaryStatus, callStatus, callBoundary string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id=$2`, actionID, actionPrimaryCallID).Scan(&primaryStatus); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status,success_boundary FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id='image-call'`, actionID).Scan(&callStatus, &callBoundary); err != nil {
		t.Fatal(err)
	}
	if primaryStatus != "pending" || callStatus != "pending" || callBoundary != "durable_media_intent_created" {
		t.Fatalf("initial outcome boundary collapsed: primary=%s call=%s boundary=%s", primaryStatus, callStatus, callBoundary)
	}

	tx, err = repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := app.settleActionOutcomeByExternalRefTx(ctx, tx, externalRef, ActionOutcomeUnknown, map[string]any{"reason_code": "provider_result_ambiguous"}, "provider_result_ambiguous")
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if !changed {
		_ = tx.Rollback(ctx)
		t.Fatal("ambiguous async settlement did not change outcome")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var revision int
	if err := repository.Pool().QueryRow(ctx, `SELECT status,success_boundary,revision FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id='image-call'`, actionID).Scan(&callStatus, &callBoundary, &revision); err != nil {
		t.Fatal(err)
	}
	if callStatus != "unknown" || callBoundary != "durable_media_intent_created" || revision != 2 {
		t.Fatalf("ambiguous outcome=%s boundary=%s revision=%d", callStatus, callBoundary, revision)
	}

	tx, err = repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	changed, err = app.settleActionOutcomeByExternalRefTx(ctx, tx, externalRef, ActionOutcomeCompleted, map[string]any{"delivery_status": "asset_ready"}, "")
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if !changed {
		_ = tx.Rollback(ctx)
		t.Fatal("final async settlement did not reconcile unknown outcome")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status,success_boundary,revision FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id='image-call'`, actionID).Scan(&callStatus, &callBoundary, &revision); err != nil {
		t.Fatal(err)
	}
	if callStatus != "completed" || callBoundary != "final_media_asset_ready" || revision != 3 {
		t.Fatalf("final outcome=%s boundary=%s revision=%d", callStatus, callBoundary, revision)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id=$2`, actionID, actionPrimaryCallID).Scan(&primaryStatus); err != nil {
		t.Fatal(err)
	}
	if primaryStatus != "completed" {
		t.Fatalf("aggregate outcome did not wait for final async boundary: %s", primaryStatus)
	}

	tx, err = repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	changed, err = app.settleActionOutcomeByExternalRefTx(ctx, tx, externalRef, ActionOutcomeCompleted, map[string]any{"delivery_status": "asset_ready"}, "")
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if changed {
		_ = tx.Rollback(ctx)
		t.Fatal("duplicate async settlement changed terminal outcome")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.settleActionOutcomeByExternalRefTx(ctx, tx, externalRef, ActionOutcomeCompleted, map[string]any{"delivery_status": "different_asset"}, ""); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatal("conflicting terminal observation was accepted as an idempotent replay")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var factCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key IN ($2,$3)`, fluctlightID, "action-outcome:"+outcomes[1].ID+":2", "action-outcome:"+outcomes[1].ID+":3").Scan(&factCount); err != nil || factCount != 2 {
		t.Fatalf("async outcome fact count=%d err=%v", factCount, err)
	}
}

func TestStructuredTurnSettlementRollsBackStateAssistantAndOutcomeTogether(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	fluctlightID := "atomic-fluctlight-" + suffix
	conversationID := "atomic-conversation-" + suffix
	inboxID := "atomic-inbox-" + suffix
	frozenID := "atomic-frozen-" + suffix
	assistantID := "atomic-assistant-" + suffix
	app := &App{DB: repository}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, statement := range []string{
			`DELETE FROM public.platform_outbox_events WHERE fluctlight_id=$1`,
			`DELETE FROM public.platform_workflow_intents WHERE payload->>'fluctlight_id'=$1`,
			`DELETE FROM public.cognition_action_outcomes WHERE fluctlight_id=$1`,
			`DELETE FROM public.cognition_internal_dynamics WHERE fluctlight_id=$1`,
			`DELETE FROM public.cognition_focus_cycles WHERE fluctlight_id=$1`,
			`DELETE FROM public.cognition_appraisals WHERE fluctlight_id=$1`,
			`DELETE FROM public.fluctlight_state_revisions WHERE fluctlight_id=$1`,
			`DELETE FROM public.cognition_frozen_actions WHERE fluctlight_id=$1`,
			`DELETE FROM public.cognition_inbox WHERE fluctlight_id=$1`,
			`DELETE FROM public.cognition_inbox_heads WHERE fluctlight_id=$1`,
			`DELETE FROM public.conversation_messages WHERE conversation_id=$1`,
			`DELETE FROM public.conversation_heads WHERE conversation_id=$1`,
			`DELETE FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`,
			`DELETE FROM public.fluctlight_affect_profiles WHERE fluctlight_id=$1`,
		} {
			argument := fluctlightID
			if statement == `DELETE FROM public.conversation_messages WHERE conversation_id=$1` || statement == `DELETE FROM public.conversation_heads WHERE conversation_id=$1` {
				argument = conversationID
			}
			_, _ = repository.Pool().Exec(cleanupCtx, statement, argument)
		}
	})
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0}','{"stability":0.5}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,2,0)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,1,'conversation.message','{}',$1,$1,$1,now(),'claimed')`, inboxID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	decision := map[string]any{"appraisal": map[string]any{
		"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.1, "social_threat": 0.1,
		"controllability": 0.5, "responsibility": 0.5, "relationship_significance": 0.1, "expected_effect": 0.5,
		"evidence_refs": []any{inboxID}, "event_kind": "conversation", "direction": "mixed",
	}}
	attachEmptyFrozenCausalityForTest(decision, fluctlightID, "owner-test", inboxID)
	frozenPayload := map[string]any{"capability_runtime_version": CapabilityRuntimePayloadVersion, "decision": decision, "capability_invocations": []CapabilityInvocation{}, "capability_results": []CapabilityResult{}}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_frozen_actions(id,decision_id,inbox_id,fluctlight_id,action_type,payload,state_revision,provider_request_id,status) VALUES($1,$2,$3,$4,'reply',$5,0,$6,'frozen')`, frozenID, "decision-"+suffix, inboxID, fluctlightID, json.RawMessage(jsonBytes(frozenPayload)), "provider-"+suffix); err != nil {
		t.Fatal(err)
	}

	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.applyFrozenCognitiveStagesTx(ctx, tx, fluctlightID, inboxID, decision, "reply", frozenID, 0); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	affectResult, err := app.applyAffectEventTx(ctx, tx, fluctlightID, inboxID, normalizedAffectEvent{Type: "sad", Confidence: 1, EvidenceRefs: []any{inboxID}, IdempotencyKey: "same-source-" + suffix}, map[string]any{"revision": 0})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if intValue(affectResult["revision"]) != 1 {
		_ = tx.Rollback(ctx)
		t.Fatalf("same-source affect created a second state revision: %#v", affectResult)
	}
	var insideRevision, insideStateRevisionCount int
	var insidePAD []byte
	var insideReason string
	if err := tx.QueryRow(ctx, `SELECT revision,pad FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&insideRevision, &insidePAD); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(max(reason_code),'') FROM public.fluctlight_state_revisions WHERE fluctlight_id=$1 AND source_event_id=$2`, fluctlightID, inboxID).Scan(&insideStateRevisionCount, &insideReason); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if insideRevision != 1 || insideStateRevisionCount != 1 || insideReason != "cognitive_growth+affect_event" {
		_ = tx.Rollback(ctx)
		t.Fatalf("same-source reducer split: revision=%d rows=%d reason=%s", insideRevision, insideStateRevisionCount, insideReason)
	}
	if pleasure := numberOrZero(decodeObject(insidePAD)["pleasure"]); pleasure >= 0 {
		_ = tx.Rollback(ctx)
		t.Fatalf("negative same-source affect was clamped away: pleasure=%v", pleasure)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,1,$3,'assistant','temporary','[]',$4)`, assistantID, conversationID, fluctlightID, "assistant:"+suffix); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := app.completeTurnCognitionTx(ctx, tx, inboxID, frozenID, map[string]any{"status": "completed", "capability_results": []CapabilityResult{}}, time.Now().UTC()); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	var stateRevision, appraisalCount, assistantCount, outcomeCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateRevision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_appraisals WHERE source_fact_id=$1`, inboxID).Scan(&appraisalCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE id=$1`, assistantID).Scan(&assistantCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_action_outcomes WHERE action_id=$1`, frozenID).Scan(&outcomeCount); err != nil {
		t.Fatal(err)
	}
	rollbackQueries := map[string]string{
		"state revisions":   `SELECT count(*) FROM public.fluctlight_state_revisions WHERE fluctlight_id=$1`,
		"affect events":     `SELECT count(*) FROM public.fluctlight_inner_state_events WHERE fluctlight_id=$1`,
		"internal dynamics": `SELECT count(*) FROM public.cognition_internal_dynamics WHERE fluctlight_id=$1`,
		"focus cycles":      `SELECT count(*) FROM public.cognition_focus_cycles WHERE fluctlight_id=$1`,
		"outbox":            `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1`,
		"workflow intents":  `SELECT count(*) FROM public.platform_workflow_intents WHERE payload->>'fluctlight_id'=$1`,
	}
	for authority, query := range rollbackQueries {
		var count int
		if err := repository.Pool().QueryRow(ctx, query, fluctlightID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s escaped rollback: count=%d err=%v", authority, count, err)
		}
	}
	var frozenStatus, inboxStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_frozen_actions WHERE id=$1`, frozenID).Scan(&frozenStatus); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&inboxStatus); err != nil {
		t.Fatal(err)
	}
	if stateRevision != 0 || appraisalCount != 0 || assistantCount != 0 || outcomeCount != 0 || frozenStatus != "frozen" || inboxStatus != "claimed" {
		t.Fatalf("split commit escaped rollback: state=%d appraisal=%d assistant=%d outcomes=%d frozen=%s inbox=%s", stateRevision, appraisalCount, assistantCount, outcomeCount, frozenStatus, inboxStatus)
	}
}

func TestProjectHealthNativeMutationsUseCallerOwnedTransaction(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID := "native-owner-" + suffix
	fluctlightID := "native-fluctlight-" + suffix
	app := &App{DB: repository}
	registry := mustCapabilityRegistry(
		sceneEventCapability{service: app},
		presenceEventCapability{service: app},
		capabilityRequestCapability{service: &capabilityRequestService{app: app}},
	)
	resolver := NewAppContextResolver(app)
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Capabilities = registry
	app.ContextResolver = resolver
	app.Runtime = runtime
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, statement := range []string{
			`DELETE FROM public.platform_outbox_events WHERE fluctlight_id=$1`,
			`DELETE FROM public.platform_workflow_intents WHERE payload->>'fluctlight_id'=$1`,
			`DELETE FROM public.cognition_inbox WHERE fluctlight_id=$1`,
			`DELETE FROM public.cognition_inbox_heads WHERE fluctlight_id=$1`,
			`DELETE FROM public.capability_requests WHERE fluctlight_id=$1`,
			`DELETE FROM public.life_presence_overlays WHERE fluctlight_id=$1`,
			`DELETE FROM public.life_schedule_items WHERE schedule_id IN (SELECT id FROM public.life_schedules WHERE fluctlight_id=$1)`,
			`DELETE FROM public.life_schedules WHERE fluctlight_id=$1`,
			`DELETE FROM public.life_events WHERE fluctlight_id=$1`,
			`DELETE FROM public.fluctlights WHERE id=$1`,
			`DELETE FROM public.actors WHERE id IN ($1,$2)`,
		} {
			if statement == `DELETE FROM public.actors WHERE id IN ($1,$2)` {
				_, _ = repository.Pool().Exec(cleanupCtx, statement, ownerID, fluctlightID)
			} else {
				_, _ = repository.Pool().Exec(cleanupCtx, statement, fluctlightID)
			}
		}
	})
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'llm_defined','active','{}',$3,'{}','{}','{}','{}')`, fluctlightID, ownerID, json.RawMessage(`{"name":"native-test","timezone":"Asia/Shanghai"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}

	invocations := []CapabilityInvocation{
		{CallID: "scene-" + suffix, CapabilityName: "scene_event", Arguments: json.RawMessage(`{"operation":"start","scene":"书房","activity":"阅读","confidence":0.9}`), SourceFactID: "fact-scene-" + suffix, ProviderRequestID: "provider-scene-" + suffix, Metadata: InvocationMetadata{FluctlightID: fluctlightID}},
		{CallID: "presence-" + suffix, CapabilityName: "presence_event", Arguments: json.RawMessage(`{"current_task":"一起阅读","confidence":0.8}`), SourceFactID: "fact-presence-" + suffix, ProviderRequestID: "provider-presence-" + suffix, Metadata: InvocationMetadata{FluctlightID: fluctlightID}},
		{CallID: "request-" + suffix, CapabilityName: "capability.request", Arguments: json.RawMessage(`{"capability_key":"calendar.read","title":"读取日历","description":"读取授权日历","rationale":"安排后续行动"}`), SourceFactID: "fact-request-" + suffix, ProviderRequestID: "provider-request-" + suffix, Metadata: InvocationMetadata{FluctlightID: fluctlightID}},
	}
	for _, invocation := range invocations {
		projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID, SourceFactID: invocation.SourceFactID, MemoryOperation: MemoryForNativeCognition, MemoryConversationMode: MemoryConversationGlobalOnly})
		if err != nil {
			t.Fatal(err)
		}
		bound, err := app.bindCapabilityInvocationsToProjection([]CapabilityInvocation{invocation}, projection, "action-"+invocation.CallID, invocation.SourceFactID, CapabilitySurfaceNativeCognition)
		if err != nil {
			t.Fatal(err)
		}
		prepared, _, err := runtime.Prepare(ctx, bound[0])
		if err != nil {
			t.Fatal(err)
		}
		tx, err := repository.Pool().Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		result, err := runtime.ExecuteTransactional(ctx, tx, prepared)
		if err != nil || result.Status != "completed" {
			_ = tx.Rollback(ctx)
			t.Fatalf("transactional capability %s result=%#v err=%v", invocation.CapabilityName, result, err)
		}
		query := map[string]string{
			"scene_event":        `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1`,
			"presence_event":     `SELECT count(*) FROM public.life_presence_overlays WHERE fluctlight_id=$1`,
			"capability.request": `SELECT count(*) FROM public.capability_requests WHERE fluctlight_id=$1`,
		}[invocation.CapabilityName]
		var count int
		if err := tx.QueryRow(ctx, query, fluctlightID).Scan(&count); err != nil || count != 1 {
			_ = tx.Rollback(ctx)
			t.Fatalf("%s not visible in caller transaction: count=%d err=%v", invocation.CapabilityName, count, err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		if err := repository.Pool().QueryRow(ctx, query, fluctlightID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s escaped caller rollback: count=%d err=%v", invocation.CapabilityName, count, err)
		}
	}
	_, lifeBeforeSchedule, err := app.readLifeContextSnapshotAt(ctx, fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	localDate := time.Now().In(time.FixedZone("CST", 8*60*60))
	day := localDate.Format("2006-01-02")
	start := day + "T00:00:00+08:00"
	endDate := localDate.AddDate(0, 0, 1).Format("2006-01-02")
	end := endDate + "T00:00:00+08:00"
	if _, err := app.acceptScheduleTx(ctx, tx, ownerID, fluctlightID, map[string]any{
		"local_date": day, "timezone": "Asia/Shanghai", "idempotency_key": "schedule-" + suffix,
		"expected_revision": 0, "expected_life_context_revision": lifeBeforeSchedule["context_revision"],
		"items": []any{map[string]any{"start_at": start, "end_at": end, "activity": "阅读", "scene": "书房", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}},
	}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("transactional schedule: %v", err)
	}
	for table, query := range map[string]string{"schedule": `SELECT count(*) FROM public.life_schedules WHERE fluctlight_id=$1`} {
		var count int
		if err := tx.QueryRow(ctx, query, fluctlightID).Scan(&count); err != nil || count != 1 {
			_ = tx.Rollback(ctx)
			t.Fatalf("%s not visible in caller transaction: count=%d err=%v", table, count, err)
		}
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for table, query := range map[string]string{"schedule": `SELECT count(*) FROM public.life_schedules WHERE fluctlight_id=$1`} {
		var count int
		if err := repository.Pool().QueryRow(ctx, query, fluctlightID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s escaped caller rollback: count=%d err=%v", table, count, err)
		}
	}
}

func TestNativeCognitionRequiredFailureReplaysFrozenDecisionWithoutSecondProviderCall(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID := "native-replay-owner-" + suffix
	fluctlightID := "native-replay-fluctlight-" + suffix
	inboxID := "native-replay-inbox-" + suffix
	endpointID := "native-replay-endpoint-" + suffix

	capability := &nativeReplayTestCapability{}
	capability.failFirst.Store(true)
	registry := mustCapabilityRegistry(capability)
	var providerCalls atomic.Int32
	var providerRef string
	var providerDriveRef string
	stateRefPattern := regexp.MustCompile(`state:ctx_[a-f0-9]{32}`)
	driveRefPattern := regexp.MustCompile(`drive:ctx_[a-f0-9]{32}`)
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		providerCalls.Add(1)
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		matched := stateRefPattern.Find(body)
		if len(matched) == 0 {
			return nil, fmt.Errorf("native Provider request did not contain a state context ref")
		}
		providerRef = string(matched)
		driveMatched := driveRefPattern.Find(body)
		if len(driveMatched) == 0 {
			return nil, fmt.Errorf("native Provider request did not contain a drive context ref")
		}
		providerDriveRef = string(driveMatched)
		structured := map[string]any{
			"appraisal": map[string]any{
				"relevance": 0.6, "goal_congruence": 0.4, "reward": 0.3, "loss": 0.7, "social_threat": 0.2,
				"controllability": 0.5, "responsibility": 0.5, "relationship_significance": 0.1, "expected_effect": 0.4,
				"evidence_refs": []any{}, "event_kind": "life_observation", "direction": "negative",
				"drive_signals": []any{map[string]any{"ref": providerDriveRef, "direction": "increase", "strength": 0.8, "confidence": 0.9, "evidence_refs": []any{providerDriveRef, providerRef}}},
			},
			"attention": "observe", "thought": "assess", "desire": "adapt", "agency": "act",
			"influences": []any{
				map[string]any{"ref": providerRef, "role": "grounds", "confidence": 0.9, "note": "当前状态影响了本次内部决策"},
				map[string]any{"ref": providerDriveRef, "role": "motivates", "confidence": 0.9, "note": "当前需要影响了本次内部决策"},
			},
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{
			"content":    string(jsonBytes(structured)),
			"tool_calls": []any{map[string]any{"id": "native-call-1", "type": "function", "function": map[string]any{"name": capability.Definition().Name, "arguments": `{}`}}},
		}}}}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(jsonBytes(response)))), Request: request}, nil
	})}

	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: providerHTTP}, Capabilities: registry}
	app.ContextResolver = NewAppContextResolver(app)
	runtime, err := NewCapabilityRuntime(registry, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime

	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://native-replay.invalid',$2,'ready',now())`, endpointID, "native-replay-secret-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('generic_llm',$1,'native-replay-model','structured_output',4096,5,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	identityJSON := json.RawMessage(`{"name":"native-replay","timezone":"Asia/Shanghai"}`)
	corePersonaJSON := json.RawMessage(`{"personality_system":{"active_profile_id":"default"}}`)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'llm_defined','active',$3,$4,'{}','{}','{}','{}')`, fluctlightID, ownerID, corePersonaJSON, identityJSON); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0}','{"stability":0.5}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,2,0)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	inboxPayload := jsonBytes(map[string]any{"event_type": "life.test.observed", "fluctlight_id": fluctlightID})
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,1,'life.test.observed',$3,$1,$1,$1,now(),'pending')`, inboxID, fluctlightID, inboxPayload); err != nil {
		t.Fatal(err)
	}

	if _, err := app.ProcessCognitionInbox(ctx, inboxID); err == nil {
		t.Fatal("first native settlement should expose the retryable required capability failure")
	}
	if providerCalls.Load() != 1 || capability.prepareCalls.Load() != 1 || capability.txCalls.Load() != 1 {
		t.Fatalf("first attempt calls provider=%d prepare=%d tx=%d", providerCalls.Load(), capability.prepareCalls.Load(), capability.txCalls.Load())
	}
	var stateRevision, appraisalCount int
	var inboxStatus, frozenStatus string
	var drives []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT revision,drives FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateRevision, &drives); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_appraisals WHERE source_fact_id=$1`, inboxID).Scan(&appraisalCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&inboxStatus); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_frozen_actions WHERE inbox_id=$1`, inboxID).Scan(&frozenStatus); err != nil {
		t.Fatal(err)
	}
	if stateRevision != 0 || appraisalCount != 0 || inboxStatus != "pending" || frozenStatus != "frozen" {
		t.Fatalf("retryable failure escaped rollback: state=%d appraisals=%d inbox=%s frozen=%s", stateRevision, appraisalCount, inboxStatus, frozenStatus)
	}
	for authority, query := range map[string]string{
		"state revisions":   `SELECT count(*) FROM public.fluctlight_state_revisions WHERE fluctlight_id=$1`,
		"affect events":     `SELECT count(*) FROM public.fluctlight_inner_state_events WHERE fluctlight_id=$1`,
		"internal dynamics": `SELECT count(*) FROM public.cognition_internal_dynamics WHERE fluctlight_id=$1`,
		"focus cycles":      `SELECT count(*) FROM public.cognition_focus_cycles WHERE fluctlight_id=$1`,
		"outcomes":          `SELECT count(*) FROM public.cognition_action_outcomes WHERE fluctlight_id=$1`,
		"outbox":            `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1`,
		"reflection intents": `SELECT count(*) FROM public.platform_workflow_intents
			WHERE intent_type='reflection.run' AND payload->>'fluctlight_id'=$1`,
	} {
		var count int
		if err := repository.Pool().QueryRow(ctx, query, fluctlightID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s escaped required-failure rollback: count=%d err=%v", authority, count, err)
		}
	}

	if _, err := app.ProcessCognitionInbox(ctx, inboxID); err != nil {
		t.Fatalf("frozen native retry failed: %v", err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, inboxID); err != nil {
		t.Fatalf("completed native replay failed: %v", err)
	}
	if providerCalls.Load() != 1 || capability.prepareCalls.Load() != 1 || capability.txCalls.Load() != 2 {
		t.Fatalf("replay repeated semantic/planner work: provider=%d prepare=%d tx=%d", providerCalls.Load(), capability.prepareCalls.Load(), capability.txCalls.Load())
	}

	var frozenPayload []byte
	var claimedBy *string
	if err := repository.Pool().QueryRow(ctx, `SELECT status,claimed_by FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&inboxStatus, &claimedBy); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status,payload FROM public.cognition_frozen_actions WHERE inbox_id=$1`, inboxID).Scan(&frozenStatus, &frozenPayload); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT revision,drives FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateRevision, &drives); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_appraisals WHERE source_fact_id=$1`, inboxID).Scan(&appraisalCount); err != nil {
		t.Fatal(err)
	}
	if inboxStatus != "processed" || claimedBy != nil || frozenStatus != "completed" || stateRevision != 1 || appraisalCount != 1 {
		t.Fatalf("native completion state inbox=%s claimed=%v frozen=%s state=%d appraisals=%d", inboxStatus, claimedBy, frozenStatus, stateRevision, appraisalCount)
	}
	var driven bool
	for _, raw := range decodeArray(drives) {
		drive := mapValue(raw)
		if stringValue(drive["key"]) == "exploration" && numberOrZero(drive["pressure"]) > 0 {
			driven = true
		}
	}
	if !driven || providerDriveRef == "" {
		t.Fatalf("semantic Drive signal did not change persisted pressure: %s", drives)
	}
	invocations, err := capabilityInvocationsFromValue(decodeObject(frozenPayload)["capability_invocations"])
	if err != nil || len(invocations) != 1 {
		t.Fatalf("frozen invocations=%#v err=%v", invocations, err)
	}
	snapshotIdentity := mapValue(invocations[0].ContextSnapshot["identity"])
	if invocations[0].ActionID == "" || stringValue(snapshotIdentity["action_id"]) != invocations[0].ActionID || len(invocations[0].PreparedPayload) == 0 {
		t.Fatalf("invocation was not frozen against its action: %#v", invocations[0])
	}
	resultingState := mapValue(decodeObject(frozenPayload)["resulting_state"])
	resultingStateRef := stringValue(resultingState["ref"])
	if intValue(resultingState["revision"]) != 1 || !stateRefPattern.MatchString(resultingStateRef) || resultingStateRef == providerRef {
		t.Fatalf("frozen action lost post-transition state causality: %#v", resultingState)
	}
	var outcomeCount, reflectionIntentCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_action_outcomes WHERE action_id=$1`, invocations[0].ActionID).Scan(&outcomeCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='reflection.run' AND payload->>'source_fact_id'=$1`, inboxID).Scan(&reflectionIntentCount); err != nil {
		t.Fatal(err)
	}
	var primaryObserved, primaryReferences []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT observed,context_references FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id=$2`, invocations[0].ActionID, actionPrimaryCallID).Scan(&primaryObserved, &primaryReferences); err != nil {
		t.Fatal(err)
	}
	if stringValue(decodeObject(primaryObserved)["resulting_state_ref"]) != resultingStateRef {
		t.Fatalf("primary Outcome lost resulting state ref: %s", primaryObserved)
	}
	if _, ok := decodeObject(primaryReferences)[resultingStateRef]; !ok {
		t.Fatalf("primary Outcome lost resulting state mapping: %s", primaryReferences)
	}
	if outcomeCount != 2 || reflectionIntentCount != 1 || providerRef == "" {
		t.Fatalf("native settlement outcomes=%d reflection_intents=%d provider_ref=%q", outcomeCount, reflectionIntentCount, providerRef)
	}
	for authority, query := range map[string]string{
		"state revisions":   `SELECT count(*) FROM public.fluctlight_state_revisions WHERE fluctlight_id=$1`,
		"internal dynamics": `SELECT count(*) FROM public.cognition_internal_dynamics WHERE fluctlight_id=$1`,
		"focus cycles":      `SELECT count(*) FROM public.cognition_focus_cycles WHERE fluctlight_id=$1`,
	} {
		var count int
		if err := repository.Pool().QueryRow(ctx, query, fluctlightID).Scan(&count); err != nil || count != 1 {
			t.Fatalf("native replay %s count=%d err=%v", authority, count, err)
		}
	}
}

func TestAffectEventSerializesIdempotencySourceAndFrozenProfile(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "affect-serial-fluctlight"
	ownerID := "affect-serial-owner"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	_, frozenProfile, err := app.readAffectProfile(ctx, fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	firstState := map[string]any{"revision": 0, "affect_profile": frozenProfile}
	firstEvent := normalizedAffectEvent{Type: "sad", Confidence: 1, EvidenceRefs: []any{"source-a"}, IdempotencyKey: "event-a-1"}

	type result struct {
		value map[string]any
		err   error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			value, applyErr := app.applyAffectEvent(ctx, fluctlightID, "source-a", firstEvent, firstState)
			results <- result{value: value, err: applyErr}
		}()
	}
	for range 2 {
		result := <-results
		if result.err != nil || intValue(result.value["revision"]) != 1 {
			t.Fatalf("concurrent idempotent affect result=%#v err=%v", result.value, result.err)
		}
	}
	var revision, eventCount int
	var pad []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT revision,pad FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&revision, &pad); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_inner_state_events WHERE fluctlight_id=$1`, fluctlightID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if revision != 1 || eventCount != 1 || numberOrZero(decodeObject(pad)["pleasure"]) != -0.1 {
		t.Fatalf("concurrent affect applied more than once: revision=%d events=%d pad=%s", revision, eventCount, pad)
	}
	changedPayload := firstEvent
	changedPayload.Type = "happy"
	if _, err := app.applyAffectEvent(ctx, fluctlightID, "source-a", changedPayload, firstState); err == nil {
		t.Fatal("changed idempotency payload was accepted")
	} else if code, retryable := capabilityErrorInfo(err, "", true); code != "affect_idempotency_conflict" || retryable {
		t.Fatalf("changed idempotency payload code=%q retryable=%v err=%v", code, retryable, err)
	}

	secondEvent := normalizedAffectEvent{Type: "calm", Confidence: 0.5, EvidenceRefs: []any{"source-b"}, IdempotencyKey: "event-b"}
	secondState := map[string]any{"revision": 1, "affect_profile": frozenProfile}
	if _, err := app.applyAffectEvent(ctx, fluctlightID, "source-b", secondEvent, secondState); err != nil {
		t.Fatal(err)
	}
	lateSource := normalizedAffectEvent{Type: "happy", Confidence: 0.5, EvidenceRefs: []any{"source-a"}, IdempotencyKey: "event-a-2"}
	if _, err := app.applyAffectEvent(ctx, fluctlightID, "source-a", lateSource, map[string]any{"revision": 2, "affect_profile": frozenProfile}); err == nil {
		t.Fatal("late same-source event was accepted")
	} else if code, retryable := capabilityErrorInfo(err, "", true); code != "affect_source_revision_conflict" || retryable {
		t.Fatalf("late same-source code=%q retryable=%v err=%v", code, retryable, err)
	}

	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlight_affect_profiles SET revision=1,regulation_policy='{"strength":0.2}',updated_at=now() WHERE fluctlight_id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	_, currentProfile, err := app.readAffectProfile(ctx, fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	stateStaleEvent := normalizedAffectEvent{Type: "happy", Confidence: 0.5, EvidenceRefs: []any{"source-state-stale"}, IdempotencyKey: "event-state-stale"}
	if _, err := app.applyAffectEvent(ctx, fluctlightID, "source-state-stale", stateStaleEvent, map[string]any{"revision": 0, "affect_profile": currentProfile}); err == nil {
		t.Fatal("stale Affect state apply was accepted")
	} else if code, retryable := capabilityErrorInfo(err, "", true); code != "affect_state_revision_conflict" || retryable {
		t.Fatalf("stale Affect state code=%q retryable=%v err=%v", code, retryable, err)
	}
	profileStaleEvent := normalizedAffectEvent{Type: "relieved", Confidence: 0.5, EvidenceRefs: []any{"source-c"}, IdempotencyKey: "event-c"}
	if _, err := app.applyAffectEvent(ctx, fluctlightID, "source-c", profileStaleEvent, map[string]any{"revision": 2, "affect_profile": frozenProfile}); err == nil {
		t.Fatal("stale AffectProfile apply was accepted")
	} else if code, retryable := capabilityErrorInfo(err, "", true); code != "affect_profile_revision_conflict" || retryable {
		t.Fatalf("stale AffectProfile code=%q retryable=%v err=%v", code, retryable, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_inner_state_events WHERE fluctlight_id=$1`, fluctlightID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if revision != 2 || eventCount != 2 {
		t.Fatalf("conflicted affect mutated state: revision=%d events=%d", revision, eventCount)
	}
	staleDecision := map[string]any{
		"context_projection": ContextProjection{SchemaVersion: "fluctlight.context.v2", FluctlightID: fluctlightID, AffectProfile: frozenProfile},
		"appraisal": map[string]any{
			"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.1, "social_threat": 0.1,
			"controllability": 0.5, "responsibility": 0.5, "relationship_significance": 0.1, "expected_effect": 0.5,
			"evidence_refs": []any{"source-d"}, "event_kind": "life_observation", "direction": "mixed",
		},
	}
	attachEmptyFrozenCausalityForTest(staleDecision, fluctlightID, ownerID, "source-d")
	staleTx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.applyFrozenCognitiveStagesTx(ctx, staleTx, fluctlightID, "source-d", staleDecision, "no_op", "stale-profile-action", 2); err == nil {
		_ = staleTx.Rollback(ctx)
		t.Fatal("stale AffectProfile cognition was accepted")
	} else if code, retryable := capabilityErrorInfo(err, "", true); code != "affect_profile_revision_conflict" || retryable {
		_ = staleTx.Rollback(ctx)
		t.Fatalf("stale cognition profile code=%q retryable=%v err=%v", code, retryable, err)
	}
	if err := staleTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var staleAppraisalCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_appraisals WHERE source_fact_id='source-d'`).Scan(&staleAppraisalCount); err != nil || staleAppraisalCount != 0 {
		t.Fatalf("stale cognition left appraisal count=%d err=%v", staleAppraisalCount, err)
	}
}

func TestAppraisalAndAffectEventCommitOneRevisionVisibleToNextProjection(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "affect-commit-owner"
	fluctlightID := "affect-commit-fluctlight"
	sourceFactID := "affect-commit-source"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	corePersona := json.RawMessage(`{"personality_system":{"active_profile_id":"default"}}`)
	identity := json.RawMessage(`{"name":"affect-commit","timezone":"Asia/Shanghai"}`)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'llm_defined','active',$3,$4,'{}','{}','{}','{}')`, fluctlightID, ownerID, corePersona, identity); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id,baseline_pad,decay_policy,regulation_policy,emotional_summary,revision) VALUES($1,'{"pleasure":-0.2,"arousal":0.1,"dominance":0}','{"pad_half_life_seconds":3600,"momentum_half_life_seconds":1800,"mood_half_life_seconds":2400,"drive_half_life_seconds":3000}','{"strength":0.1}','{"dominant_patterns":["slow recovery"]}',3)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	preProjection, err := app.BuildContextProjection(ctx, ownerID, fluctlightID, "", sourceFactID, "")
	if err != nil {
		t.Fatal(err)
	}
	preStateRef := stringValue(preProjection.InnerState["ref"])
	preProfileRef := stringValue(preProjection.AffectProfile["ref"])
	decision := map[string]any{"appraisal": map[string]any{
		"relevance": 0.8, "goal_congruence": 0.2, "reward": 0.2, "loss": 0.8, "social_threat": 0.3,
		"controllability": 0.4, "responsibility": 0.5, "relationship_significance": 0.1, "expected_effect": 0.3,
		"evidence_refs": []any{sourceFactID}, "event_kind": "life_observation", "direction": "negative",
	}}
	attachEmptyFrozenCausalityForTest(decision, fluctlightID, ownerID, sourceFactID)
	affectEvent := normalizedAffectEvent{Type: "sad", Confidence: 0.8, EvidenceRefs: []any{sourceFactID}, IdempotencyKey: "affect-commit-event"}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.applyFrozenCognitiveStagesTx(ctx, tx, fluctlightID, sourceFactID, decision, "no_op", "affect-commit-action", 0); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	result, err := app.applyAffectEventTx(ctx, tx, fluctlightID, sourceFactID, affectEvent, currentStateSnapshotFromProjection(preProjection))
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if intValue(result["revision"]) != 1 {
		_ = tx.Rollback(ctx)
		t.Fatalf("combined revision=%#v", result)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var stateRevision, revisionRows, appraisalRows, affectRows int
	var reason string
	var requested, applied []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateRevision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*),COALESCE(max(reason_code),''),COALESCE(max(requested_delta::text),'{}'),COALESCE(max(applied_delta::text),'{}') FROM public.fluctlight_state_revisions WHERE fluctlight_id=$1 AND source_event_id=$2`, fluctlightID, sourceFactID).Scan(&revisionRows, &reason, &requested, &applied); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_appraisals WHERE source_fact_id=$1`, sourceFactID).Scan(&appraisalRows); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_inner_state_events WHERE fluctlight_id=$1`, fluctlightID).Scan(&affectRows); err != nil {
		t.Fatal(err)
	}
	requestedDelta := decodeObject(requested)
	appliedDelta := decodeObject(applied)
	if stateRevision != 1 || revisionRows != 1 || appraisalRows != 1 || affectRows != 1 || reason != "cognitive_growth+affect_event" || numberOrZero(requestedDelta["pad.pleasure"]) == 0 || numberOrZero(appliedDelta["pad.pleasure"]) == 0 {
		t.Fatalf("combined commit state=%d revisions=%d appraisals=%d affects=%d reason=%s requested=%s applied=%s", stateRevision, revisionRows, appraisalRows, affectRows, reason, requested, applied)
	}

	replayTx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.applyFrozenCognitiveStagesTx(ctx, replayTx, fluctlightID, sourceFactID, decision, "no_op", "affect-commit-action", 0); err != nil {
		_ = replayTx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := app.applyAffectEventTx(ctx, replayTx, fluctlightID, sourceFactID, affectEvent, currentStateSnapshotFromProjection(preProjection)); err != nil {
		_ = replayTx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := replayTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	postProjection, err := app.BuildContextProjection(ctx, ownerID, fluctlightID, "", "next-source", "")
	if err != nil {
		t.Fatal(err)
	}
	postStateRef := stringValue(postProjection.InnerState["ref"])
	postProfileRef := stringValue(postProjection.AffectProfile["ref"])
	if postProjection.CurrentStateRevision != 1 || postStateRef == "" || postStateRef == preStateRef || postProfileRef != preProfileRef {
		t.Fatalf("next projection state_revision=%d pre_state=%q post_state=%q pre_profile=%q post_profile=%q", postProjection.CurrentStateRevision, preStateRef, postStateRef, preProfileRef, postProfileRef)
	}
	providerJSON := string(jsonBytes(compactCognitionContext(postProjection)))
	for _, forbidden := range []string{"baseline_pad", "decay_policy", "regulation_policy", "emotional_summary", "slow recovery"} {
		if strings.Contains(providerJSON, forbidden) {
			t.Fatalf("AffectProfile policy leaked to Provider: %s", providerJSON)
		}
	}
	if !strings.Contains(providerJSON, postStateRef) || !strings.Contains(providerJSON, postProfileRef) {
		t.Fatalf("next Provider projection lost state/profile refs: %s", providerJSON)
	}
	laterInboxID := "affect-later-decision"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,2,0)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,1,'life.test.observed',$3,$1,$1,$1,now(),'claimed')`, laterInboxID, fluctlightID, jsonBytes(map[string]any{"event_type": "life.test.observed"})); err != nil {
		t.Fatal(err)
	}
	laterDecision := map[string]any{
		"context_projection": postProjection,
		"influences":         []any{map[string]any{"ref": postStateRef, "role": "grounds", "confidence": 0.9, "note": "更新后的情绪状态影响了后续决定"}},
	}
	if _, err := freezeDecisionInfluences(laterDecision, postProjection, false); err != nil {
		t.Fatal(err)
	}
	laterDecision["cognitive_state_transition"] = "not_proposed"
	laterFrozen, err := app.PersistTurnDecision(ctx, laterInboxID, fluctlightID, "", "affect-later-turn", "no_op", laterDecision)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		if err := app.applyFrozenCognitiveStagesTx(ctx, tx, fluctlightID, laterInboxID, laterDecision, "no_op", laterFrozen.ID, laterFrozen.StateRev); err != nil {
			return err
		}
		_, err := app.completeTurnCognitionTx(ctx, tx, laterInboxID, laterFrozen.ID, map[string]any{"status": "completed", "capability_results": []CapabilityResult{}}, time.Now().UTC())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var laterReferences []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT context_references FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id=$2`, laterFrozen.ID, actionPrimaryCallID).Scan(&laterReferences); err != nil {
		t.Fatal(err)
	}
	if _, cited := decodeObject(laterReferences)[postStateRef]; !cited {
		t.Fatalf("later frozen decision/outcome did not cite new state ref: %s", laterReferences)
	}
	var laterAppraisals int
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateRevision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_appraisals WHERE source_fact_id=$1`, laterInboxID).Scan(&laterAppraisals); err != nil {
		t.Fatal(err)
	}
	if stateRevision != 1 || laterAppraisals != 0 {
		t.Fatalf("capability-only/no-appraisal settlement fabricated state: revision=%d appraisals=%d", stateRevision, laterAppraisals)
	}
}

func TestNativeCognitionCycleGuardBoundsCapabilityProducedLifeFacts(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "native-cycle-fluctlight"
	parentID := "native-cycle-parent"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,2,1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	parentPayload := jsonBytes(map[string]any{"event_type": "life.scene.updated", "native_cognition_depth": maxNativeCognitionDepth})
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES($1,$2,1,'life.scene.updated',$3,$1,$1,$1,now(),'processed',now())`, parentID, fluctlightID, parentPayload); err != nil {
		t.Fatal(err)
	}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	childID, err := (&App{DB: repository}).enqueueNativeFactTx(ctx, tx, fluctlightID, "", parentID, "life.scene.updated", "native-cycle-child", map[string]any{"scene": "same"})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	result, err := app.ProcessCognitionInbox(ctx, childID)
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(result["reason_code"]) != "native_cognition_cycle_guarded" {
		t.Fatalf("cycle result=%#v", result)
	}
	var status string
	var payload []byte
	var frozenCount, reflectionCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT status,payload FROM public.cognition_inbox WHERE id=$1`, childID).Scan(&status, &payload); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_frozen_actions WHERE inbox_id=$1`, childID).Scan(&frozenCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='reflection.run' AND payload->>'source_fact_id'=$1`, childID).Scan(&reflectionCount); err != nil {
		t.Fatal(err)
	}
	guard := mapValue(decodeObject(payload)["cycle_guard"])
	if status != "processed" || intValue(decodeObject(payload)["native_cognition_depth"]) != maxNativeCognitionDepth+1 || stringValue(guard["reason_code"]) != "native_cognition_cycle_guarded" || frozenCount != 0 || reflectionCount != 1 {
		t.Fatalf("guard settlement status=%s payload=%s frozen=%d reflection=%d", status, payload, frozenCount, reflectionCount)
	}
}

func TestAutonomyCapabilityActionRollsBackTransactionalSiblingOnRequiredFailure(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "autonomy-atomic-owner"
	fluctlightID := "autonomy-atomic-fluctlight"
	actionID := "autonomy-atomic-action"
	settingKey := "test.autonomy.atomic"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	registry := mustCapabilityRegistry(autonomyMutationTestCapability{settingKey: settingKey}, autonomyFailureTestCapability{})
	resolver := NewStaticContextResolver(nil)
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository, Capabilities: registry, ContextResolver: resolver, Runtime: runtime}
	calls := []CapabilityInvocation{
		{CallID: "autonomy-mutate", CapabilityName: "test.autonomy.mutate", SchemaVersion: CapabilityInvocationSchemaVersion, Arguments: json.RawMessage(`{}`), SourceFactID: "autonomy-source", ProviderRequestID: "autonomy-provider-mutate", Sequence: 0, Metadata: InvocationMetadata{FluctlightID: fluctlightID, Surface: CapabilitySurfaceAutonomy}},
		{CallID: "autonomy-fail", CapabilityName: "test.autonomy.fail", SchemaVersion: CapabilityInvocationSchemaVersion, Arguments: json.RawMessage(`{}`), SourceFactID: "autonomy-source", ProviderRequestID: "autonomy-provider-fail", Sequence: 1, Metadata: InvocationMetadata{FluctlightID: fluctlightID, Surface: CapabilitySurfaceAutonomy}},
	}
	_, life, err := app.readLifeContextSnapshotAt(ctx, fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"capability_runtime_version": CapabilityRuntimePayloadVersion, "source_fact_id": "autonomy-source", "capability_invocations": calls, "capability_results": []CapabilityResult{}}
	attachEmptyFrozenCausalityForTest(payload, fluctlightID, ownerID, "autonomy-source")
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.autonomy_actions(id,fluctlight_id,action_type,payload,policy_snapshot,expected_revisions,status,workflow_id,provider_request_id) VALUES($1,$2,'capability',$3,'{"budget_reserved":true}',$4,'frozen',$5,$6)`, actionID, fluctlightID, jsonBytes(payload), jsonBytes(map[string]any{"foundation_revision": 0, "current_state_revision": 0, "life_context_revision": life["context_revision"]}), "autonomy-atomic-workflow", "autonomy-atomic-provider"); err != nil {
		t.Fatal(err)
	}
	result, err := app.ProcessCapabilityAction(ctx, actionID)
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(result["status"]) != "failed" {
		t.Fatalf("required autonomy failure result=%#v", result)
	}
	var settingCount, outcomeCount int
	var status, errorCode string
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.runtime_settings WHERE key=$1`, settingKey).Scan(&settingCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&status, &errorCode); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_action_outcomes WHERE action_id=$1`, actionID).Scan(&outcomeCount); err != nil {
		t.Fatal(err)
	}
	if settingCount != 0 || status != "failed" || errorCode != "autonomy_test_required_failure" || outcomeCount != 3 {
		t.Fatalf("autonomy split commit: setting=%d status=%s code=%s outcomes=%d", settingCount, status, errorCode, outcomeCount)
	}
}
