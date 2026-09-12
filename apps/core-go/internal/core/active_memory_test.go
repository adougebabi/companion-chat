package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestActiveMemoryCapabilityDefinitionIsThinOptionalTransactionalAction(t *testing.T) {
	definition := activeMemoryEventCapabilityDefinition()
	if err := definition.Validate(); err != nil {
		t.Fatalf("definition.Validate() error = %v", err)
	}
	if definition.Name != "active_memory_event" || definition.Type != CapabilityTypeAction || definition.SideEffectClass != "native_projection" || definition.FailurePolicy != FailurePolicyOptionalInternal {
		t.Fatalf("unexpected definition policy: %#v", definition)
	}
	if len(definition.RequiredContext) != 2 || definition.RequiredContext[0] != SlotMemoryScope || definition.RequiredContext[1] != SlotCurrentLife {
		t.Fatalf("required context = %#v", definition.RequiredContext)
	}
	validFuture := map[string]any{
		"operation": "create", "kind": "future_event", "content": "明早七点赶飞机", "confidence": 0.9,
		"original_time_expression": "明天早上7点", "time_precision": "part_of_day", "valid_until": "2026-09-13T23:59:59+08:00",
	}
	if err := validateCapabilitySchemaValue(validFuture, definition.InputSchema); err != nil {
		t.Fatalf("valid future-event input rejected: %v", err)
	}
	for name, field := range map[string]string{"missing original expression": "original_time_expression", "missing expiry": "valid_until", "missing precision": "time_precision"} {
		invalid := cloneMap(validFuture)
		delete(invalid, field)
		if err := validateCapabilitySchemaValue(invalid, definition.InputSchema); err == nil {
			t.Fatalf("%s accepted: %#v", name, invalid)
		}
	}
	encoded := jsonString(definition.InputSchema)
	for _, forbidden := range []string{"owner_fluctlight_id", "owner_actor_id", "actor_refs", "conversation_id", "source_fact_id", "evidence_refs", "revision", "canonical_key", "idempotency_key", "timezone"} {
		if strings.Contains(encoded, `"`+forbidden+`"`) {
			t.Fatalf("provider input exposes Core-owned field %q: %s", forbidden, encoded)
		}
	}
	if class, err := classifyCapabilityExecution(activeMemoryEventCapability{}, definition); err != nil || class != CapabilityExecutionTransactionalMutation {
		t.Fatalf("execution class = %q, %v", class, err)
	}
}

func TestActiveMemorySemanticValidationPreservesUnknownTimeAndChecksOffset(t *testing.T) {
	occurredAt := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	unknown := ActiveMemorySemanticInput{Kind: "temporary_context", Content: "本周等待结果", Confidence: 0.8, Importance: 0.7, TimePrecision: "unknown", Timezone: "Asia/Shanghai"}
	if err := validateActiveMemorySemantic(unknown, occurredAt); err != nil {
		t.Fatalf("unknown precision rejected: %v", err)
	}
	forged := unknown
	forged.ValidFrom = timePointer(occurredAt.Add(time.Hour))
	if err := validateActiveMemorySemantic(forged, occurredAt); err == nil || err.Error() != "active_memory_unknown_time_must_be_unresolved" {
		t.Fatalf("unknown precision with anchor error = %v", err)
	}
	wrongOffset, err := time.Parse(time.RFC3339, "2026-09-12T07:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	exact := ActiveMemorySemanticInput{Kind: "future_event", Content: "明早七点赶飞机", Confidence: 0.95, Importance: 1, OriginalTimeExpression: "明早七点", ValidFrom: &wrongOffset, TimePrecision: "exact", Timezone: "Asia/Shanghai"}
	if err := validateActiveMemorySemantic(exact, occurredAt); err == nil || err.Error() != "active_memory_time_offset_invalid" {
		t.Fatalf("wrong timezone offset error = %v", err)
	}
	correctOffset, err := time.Parse(time.RFC3339, "2026-09-12T07:00:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	exact.ValidFrom = &correctOffset
	if err := validateActiveMemorySemantic(exact, occurredAt); err != nil {
		t.Fatalf("valid exact semantic rejected: %v", err)
	}
}

func TestActiveMemoryPreparedCommandDigestFreezesSourceTime(t *testing.T) {
	occurredAt := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	semantic := ActiveMemorySemanticInput{Kind: "commitment", Content: "今晚早点睡", Confidence: 0.9, Importance: 0.8, TimePrecision: "unknown", Timezone: "Asia/Shanghai"}
	command := PreparedActiveMemoryMutation{
		SchemaVersion: activeMemoryLifecycleSchemaVersion, Operation: ActiveMemoryCreate,
		OwnerFluctlightID: "fluctlight_a", OwnerActorID: "actor_owner", ActorID: "actor_owner",
		SourceFactID: "fact_a", EvidenceRefs: []string{"fact_a"}, OccurredAt: occurredAt,
		Semantic: &semantic, SemanticReason: "test", IdempotencyKey: "active-memory:test:a",
	}
	command.RequestDigest = activeMemoryCommandDigest(command)
	if err := validatePreparedActiveMemoryMutation(command); err != nil {
		t.Fatalf("valid command rejected: %v", err)
	}
	mutated := command
	mutated.OccurredAt = mutated.OccurredAt.Add(time.Second)
	if err := validatePreparedActiveMemoryMutation(mutated); err == nil || err.Error() != "active_memory_command_digest_invalid" {
		t.Fatalf("mutated source time digest error = %v", err)
	}
}

func TestActiveMemoryOpaqueReferenceAndProviderCompaction(t *testing.T) {
	projection := ContextProjection{
		FluctlightID: "fluctlight_a", OwnerActorID: "actor_owner", ConversationID: "conversation_a",
		CurrentSpeaker: map[string]any{"actor_id": "actor_owner"}, PersonalityRuntime: map[string]any{"active_profile_id": "default"},
	}
	index := newContextReferenceIndex(projection)
	rows := []map[string]any{{
		"id": "active_memory_internal_a", "owner_fluctlight_id": "fluctlight_a", "conversation_id": "conversation_a",
		"kind": "future_event", "content": "明早七点赶飞机", "status": "active", "confidence": 0.95, "importance": 1.0,
		"source_fact_id": "fact_internal_a", "evidence_refs": []any{"fact_internal_a"}, "revision": 3,
		"original_time_expression": "明早七点", "valid_until": "2026-09-12T07:00:00+08:00", "time_precision": "exact", "timezone": "Asia/Shanghai",
	}}
	if err := addActiveMemoryReferences(&index, rows); err != nil {
		t.Fatalf("addActiveMemoryReferences() error = %v", err)
	}
	ref := stringValue(rows[0]["ref"])
	target, snapshot, err := activeMemoryTargetFromRef(ref, index)
	if err != nil {
		t.Fatalf("activeMemoryTargetFromRef() error = %v", err)
	}
	if target.ActiveMemoryID != "active_memory_internal_a" || target.ExpectedRevision != 3 || stringValue(snapshot["content"]) != "明早七点赶飞机" {
		t.Fatalf("target = %#v snapshot = %#v", target, snapshot)
	}
	compact := compactActiveMemories(rows)
	if len(compact) != 1 || stringValue(compact[0]["ref"]) != ref || stringValue(compact[0]["content"]) != "明早七点赶飞机" {
		t.Fatalf("compact = %#v", compact)
	}
	encoded := jsonString(compact)
	for _, forbidden := range []string{"active_memory_internal_a", "fact_internal_a", "owner_fluctlight_id", "source_fact_id", "evidence_refs", "revision"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("provider projection leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestActiveMemoryRankingPrioritizesCriticalRelevantFact(t *testing.T) {
	now := time.Date(2026, 9, 11, 20, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	flightAt := now.Add(11 * time.Hour)
	flight := activeMemoryAuthorityRow{Content: "明早七点赶飞机", Confidence: 0.95, Importance: 1, ValidUntil: &flightAt, LastRelevantAt: now.Add(-6 * time.Hour)}
	ordinary := activeMemoryAuthorityRow{Content: "本周等待普通结果", Confidence: 0.9, Importance: 0.7, LastRelevantAt: now.Add(-time.Hour)}
	flightComponents := activeMemoryRankComponents(flight, "赶飞机", now)
	ordinaryComponents := activeMemoryRankComponents(ordinary, "赶飞机", now)
	if flightComponents["critical_window"] <= 0 || flightComponents["lexical_relevance"] <= 0 {
		t.Fatalf("flight components = %#v", flightComponents)
	}
	flightScore, ordinaryScore := 0.0, 0.0
	for _, value := range flightComponents {
		flightScore += value
	}
	for _, value := range ordinaryComponents {
		ordinaryScore += value
	}
	if flightScore <= ordinaryScore {
		t.Fatalf("flight score %v <= ordinary score %v", flightScore, ordinaryScore)
	}
}

func TestActiveMemoryLifecycleStaticGuardKeepsOneProductionSQLAuthority(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "active_memory_lifecycle.go" {
			continue
		}
		content, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		if strings.Contains(text, "INSERT INTO public.active_memories") || strings.Contains(text, "UPDATE public.active_memories SET") || strings.Contains(text, "INSERT INTO public.active_memory_revisions") || strings.Contains(text, "INSERT INTO public.active_memory_commands") {
			t.Fatalf("Active Memory lifecycle SQL authority leaked into %s", name)
		}
	}
}

func activeMemoryLifecycleTestCommand(operation ActiveMemoryOperation, key string, occurredAt time.Time, semantic *ActiveMemorySemanticInput, target *ActiveMemoryTarget) PreparedActiveMemoryMutation {
	command := PreparedActiveMemoryMutation{
		SchemaVersion: activeMemoryLifecycleSchemaVersion, Operation: operation,
		OwnerFluctlightID: "active-memory-fluctlight", OwnerActorID: "active-memory-owner", ActorID: "active-memory-fluctlight", ActorRefs: []string{"active-memory-owner"},
		ConversationID: "active-memory-conversation", SourceFactID: "active-memory-fact",
		EvidenceRefs: []string{"active-memory-fact"}, OccurredAt: occurredAt,
		Target: target, Semantic: semantic, SemanticReason: "integration lifecycle", IdempotencyKey: "active-memory:test:" + key,
	}
	command.RequestDigest = activeMemoryCommandDigest(command)
	return command
}

func applyActiveMemoryLifecycleTestCommand(t *testing.T, ctx context.Context, app *App, command PreparedActiveMemoryMutation) ActiveMemoryApplyResult {
	t.Helper()
	var result ActiveMemoryApplyResult
	if err := withTransaction(ctx, app.DB.Pool(), func(tx pgx.Tx) error {
		var err error
		result, err = app.applyActiveMemoryCommandTx(ctx, tx, command)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

// PostgreSQL integration gate: intentionally deferred to S12 by implement.md.
func TestPostgresActiveMemoryLifecycleReplayCASExpiryAndSupersede(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "active-memory-owner", "active-memory-fluctlight", "active-memory-conversation"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{"timezone":"UTC"}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'active memory')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	occurredAt := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES('active-memory-fact',$1,1,'conversation.turn',$2,'active-memory-test','active-memory-test','active-memory-fact',$3,'processed')`, fluctlightID, jsonBytes(map[string]any{"conversation_id": conversationID, "text": "明早七点赶飞机"}), occurredAt); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	flightUntil := occurredAt.Add(19 * time.Hour)
	flight := &ActiveMemorySemanticInput{Kind: "future_event", Content: "明早七点赶飞机", Confidence: 0.95, Importance: 1, OriginalTimeExpression: "明早七点", ValidUntil: &flightUntil, TimePrecision: "exact", Timezone: "UTC"}
	create := activeMemoryLifecycleTestCommand(ActiveMemoryCreate, "create-flight", occurredAt, flight, nil)
	created := applyActiveMemoryLifecycleTestCommand(t, ctx, app, create)
	if created.Disposition != "applied" || created.Revision != 1 || created.Status != "active" {
		t.Fatalf("create = %#v", created)
	}
	replayed := applyActiveMemoryLifecycleTestCommand(t, ctx, app, create)
	if !replayed.Replayed || replayed.ActiveMemoryID != created.ActiveMemoryID {
		t.Fatalf("replay = %#v", replayed)
	}
	conflict := create
	conflict.Semantic = &ActiveMemorySemanticInput{Kind: "future_event", Content: "different", Confidence: 0.9, Importance: 1, TimePrecision: "unknown", Timezone: "UTC"}
	conflict.RequestDigest = activeMemoryCommandDigest(conflict)
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.applyActiveMemoryCommandTx(ctx, tx, conflict)
		return err
	}); err == nil || err.Error() != "active_memory_idempotency_conflict" {
		t.Fatalf("idempotency conflict error = %v", err)
	}
	confirm := activeMemoryLifecycleTestCommand(ActiveMemoryConfirm, "confirm-flight", occurredAt.Add(time.Minute), nil, &ActiveMemoryTarget{Ref: "active-memory-ref", ActiveMemoryID: created.ActiveMemoryID, ExpectedRevision: 1})
	confirmed := applyActiveMemoryLifecycleTestCommand(t, ctx, app, confirm)
	if confirmed.Revision != 2 || confirmed.ReasonCode != "confirmed" {
		t.Fatalf("confirm = %#v", confirmed)
	}
	stale := activeMemoryLifecycleTestCommand(ActiveMemoryComplete, "stale-flight", occurredAt.Add(2*time.Minute), nil, &ActiveMemoryTarget{Ref: "active-memory-ref", ActiveMemoryID: created.ActiveMemoryID, ExpectedRevision: 1})
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.applyActiveMemoryCommandTx(ctx, tx, stale)
		return err
	}); err == nil || !strings.Contains(err.Error(), "revision_conflict") {
		t.Fatalf("stale CAS error = %v", err)
	}
	revisedSemantic := &ActiveMemorySemanticInput{Kind: "future_event", Content: "明早七点从新航站楼赶飞机", Confidence: 0.97, Importance: 1, TimePrecision: "unknown", Timezone: "UTC"}
	revise := activeMemoryLifecycleTestCommand(ActiveMemoryRevise, "revise-flight", occurredAt.Add(3*time.Minute), revisedSemantic, &ActiveMemoryTarget{Ref: "active-memory-ref", ActiveMemoryID: created.ActiveMemoryID, ExpectedRevision: 2})
	revised := applyActiveMemoryLifecycleTestCommand(t, ctx, app, revise)
	if revised.Revision != 3 || revised.ReasonCode != "revised" {
		t.Fatalf("revise = %#v", revised)
	}
	replacementSemantic := &ActiveMemorySemanticInput{Kind: "future_event", Content: "航班改到明早八点", Confidence: 0.98, Importance: 1, TimePrecision: "unknown", Timezone: "UTC"}
	supersede := activeMemoryLifecycleTestCommand(ActiveMemorySupersede, "supersede-flight", occurredAt.Add(4*time.Minute), replacementSemantic, &ActiveMemoryTarget{Ref: "active-memory-ref", ActiveMemoryID: created.ActiveMemoryID, ExpectedRevision: 3})
	superseded := applyActiveMemoryLifecycleTestCommand(t, ctx, app, supersede)
	if superseded.ActiveMemoryID == created.ActiveMemoryID || superseded.ResultingRevisions[created.ActiveMemoryID] != 4 || superseded.Revision != 1 {
		t.Fatalf("supersede = %#v", superseded)
	}
	complete := activeMemoryLifecycleTestCommand(ActiveMemoryComplete, "complete-flight", occurredAt.Add(5*time.Minute), nil, &ActiveMemoryTarget{Ref: "replacement-ref", ActiveMemoryID: superseded.ActiveMemoryID, ExpectedRevision: 1})
	completed := applyActiveMemoryLifecycleTestCommand(t, ctx, app, complete)
	if completed.Status != "completed" || completed.Revision != 2 {
		t.Fatalf("complete = %#v", completed)
	}

	expiredUntil := occurredAt.Add(-time.Hour)
	expiredSemantic := &ActiveMemorySemanticInput{Kind: "temporary_context", Content: "已经失效的临时上下文", Confidence: 0.8, Importance: 0.6, OriginalTimeExpression: "一小时前", ValidUntil: &expiredUntil, TimePrecision: "exact", Timezone: "UTC"}
	expiredCreate := activeMemoryLifecycleTestCommand(ActiveMemoryCreate, "create-expired", occurredAt, expiredSemantic, nil)
	expired := applyActiveMemoryLifecycleTestCommand(t, ctx, app, expiredCreate)
	retrieved, err := app.retrieveActiveMemories(ctx, ActiveMemoryQuery{AuthorizationActorID: ownerID, OwnerFluctlightID: fluctlightID, ConversationID: conversationID, Cue: "临时上下文", At: occurredAt, Limit: 30})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range retrieved.Items {
		if stringValue(item["id"]) == expired.ActiveMemoryID || stringValue(item["id"]) == completed.ActiveMemoryID {
			t.Fatalf("inactive or read-time-expired Active Memory was selected: %#v", item)
		}
	}
	expire := activeMemoryLifecycleTestCommand(ActiveMemoryExpire, "expire-stale", occurredAt.Add(6*time.Minute), nil, &ActiveMemoryTarget{Ref: "expired-ref", ActiveMemoryID: expired.ActiveMemoryID, ExpectedRevision: 1})
	expiredClosed := applyActiveMemoryLifecycleTestCommand(t, ctx, app, expire)
	if expiredClosed.Status != "expired" || expiredClosed.Revision != 2 {
		t.Fatalf("expire = %#v", expiredClosed)
	}
	var revisions, commands int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.active_memory_revisions r JOIN public.active_memories m ON m.id=r.active_memory_id WHERE m.owner_fluctlight_id=$1`, fluctlightID).Scan(&revisions); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.active_memory_commands WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if revisions != 8 || commands != 7 {
		t.Fatalf("audit counts revisions=%d commands=%d", revisions, commands)
	}
}

// PostgreSQL integration gate: intentionally deferred to S12 by implement.md.
func TestPostgresActiveMemoryEventFreezesRuntimeAuthorityWithoutMemoryProvider(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "active-event-owner", "active-event-fluctlight", "active-event-conversation"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'active event')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	occurredAt := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES('active-event-fact',$1,1,'conversation.turn',$2,'active-event','active-event','active-event-fact',$3,'processed')`, fluctlightID, jsonBytes(map[string]any{"conversation_id": conversationID, "text": "明早七点赶飞机"}), occurredAt); err != nil {
		t.Fatal(err)
	}
	index := ContextReferenceIndex{SchemaVersion: contextReferenceIndexVersion, FluctlightID: fluctlightID, OwnerActorID: ownerID, SpeakerActorID: ownerID, ConversationID: conversationID, ByRef: map[string]ContextReference{}}
	snapshot := map[string]any{
		"identity":                map[string]any{"fluctlight_id": fluctlightID, "conversation_id": conversationID, "source_fact_id": "active-event-fact", "action_id": "active-event-action"},
		"memory_scope":            map[string]any{"owner_actor_id": ownerID, "viewer_actor_ids": []any{ownerID}, "conversation_mode": "exact", "memories": []any{}},
		"current_life":            map[string]any{"timezone": "Asia/Shanghai", "current_time": "2026-09-11 20:00:00 CST"},
		"context_reference_index": index,
	}
	app := &App{DB: repository}
	capability := activeMemoryEventCapability{service: app}
	registry := mustCapabilityRegistry(capability)
	runtime, err := NewCapabilityRuntime(registry, NewSnapshotContextResolver(snapshot))
	if err != nil {
		t.Fatal(err)
	}
	invocation := CapabilityInvocation{
		CallID: "active-event-call", CapabilityName: "active_memory_event", SchemaVersion: CapabilityInvocationSchemaVersion,
		Arguments:    jsonBytes(map[string]any{"operation": "create", "kind": "future_event", "content": "明早七点赶飞机", "confidence": 0.95, "importance": 1.0, "original_time_expression": "明早七点", "valid_until": "2026-09-12T07:00:00+08:00", "time_precision": "exact"}),
		SourceFactID: "active-event-fact", ActionID: "active-event-action", ProviderRequestID: "main-cognition-provider",
		Metadata: InvocationMetadata{FluctlightID: fluctlightID, ConversationID: conversationID, Surface: CapabilitySurfaceConversation}, ContextSnapshot: snapshot,
	}
	prepared, _, err := runtime.Prepare(ctx, invocation)
	if err != nil {
		t.Fatal(err)
	}
	rawPlan, found, err := capabilityPreparedData(prepared, "active_memory_plan")
	if err != nil || !found {
		t.Fatalf("prepared Active Memory plan missing: found=%v err=%v", found, err)
	}
	var plan PreparedActiveMemoryMutation
	if jsonUnmarshal(jsonBytes(rawPlan), &plan) != nil || plan.ActorID != fluctlightID || len(plan.ActorRefs) != 1 || plan.ActorRefs[0] != ownerID || !plan.OccurredAt.Equal(occurredAt) {
		t.Fatalf("prepared Active Memory authority = %#v", plan)
	}
	if app.Provider != nil {
		t.Fatal("Active Memory preparation unexpectedly requires a second Memory Provider")
	}
	var result CapabilityResult
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		var executeErr error
		result, executeErr = runtime.ExecuteTransactional(ctx, tx, prepared)
		return executeErr
	}); err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || !mapValue(result.Output)["recorded"].(bool) {
		t.Fatalf("result = %#v", result)
	}
	for _, forbidden := range []string{"active_memory_id", "revision", "source_fact_id", "evidence_refs"} {
		if strings.Contains(jsonString(result.Output), forbidden) {
			t.Fatalf("provider-safe capability result leaked %q: %#v", forbidden, result.Output)
		}
	}
}
