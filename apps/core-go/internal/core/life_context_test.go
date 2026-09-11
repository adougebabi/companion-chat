package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func seedLifeContextFluctlight(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID string) {
	t.Helper()
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{"personality_system":{"active_profile_id":"default"}}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
}

func TestLifeContextSnapshotResolvesPriorityPresenceAndTimeBoundaries(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "life-context-owner", "life-context-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	at := time.Date(2026, 9, 11, 2, 0, 0, 0, time.UTC)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,reschedule_policy,idempotency_key,request_digest,result) VALUES('life-schedule',$1,'2026-09-11','Asia/Shanghai','accepted','test','["fact"]',1,'{}','life-schedule',$2,'{"id":"life-schedule","status":"accepted","revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_after","replayed":false}')`, fluctlightID, stableDigest("life-schedule")); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.life_schedule_items(id,schedule_id,start_at,end_at,activity,scene,location,item_type,status,priority,flexibility,interruption_cost) VALUES('life-item-a','life-schedule',$1,$2,'阅读','书房','家','planned','planned','0.5','0.5','0.5'),('life-item-b','life-schedule',$2,$3,'散步','公园','社区','planned','planned','0.5','0.5','0.5')`, at.Add(-time.Hour), at.Add(30*time.Minute), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	schedule, life, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), fluctlightID, at)
	if err != nil || stringValue(life["source"]) != "schedule" || stringValue(life["schedule_item_id"]) != "life-item-a" || stringValue(life["location"]) != "家" || stringValue(life["context_revision"]) == "" {
		t.Fatalf("schedule context=%#v schedule=%#v err=%v", life, schedule, err)
	}
	initialRevision := stringValue(life["context_revision"])
	_, stable, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), fluctlightID, at.Add(time.Minute))
	if err != nil || stringValue(stable["context_revision"]) != initialRevision {
		t.Fatalf("revision changed inside interval: before=%q after=%q err=%v", initialRevision, stable["context_revision"], err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,scene,activity,location,status,revision,evidence_refs,idempotency_key,request_digest,result) VALUES('life-inferred',$1,'scene_inferred',$2,$3,'阳台','休息','家','inferred',1,'["fact"]','life-inferred',$4,'{"id":"life-inferred","status":"inferred","revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_inferred","replayed":false}')`, fluctlightID, at.Add(-time.Minute), at.Add(20*time.Minute), stableDigest("life-inferred")); err != nil {
		t.Fatal(err)
	}
	_, inferred, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), fluctlightID, at)
	if err != nil || stringValue(inferred["source"]) != "event" || stringValue(inferred["authority_status"]) != "inferred" || stringValue(inferred["event_id"]) != "life-inferred" {
		t.Fatalf("inferred context=%#v err=%v", inferred, err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,scene,activity,location,status,revision,evidence_refs,idempotency_key,request_digest,result) VALUES('life-confirmed',$1,'owner_event',$2,$3,'客厅','交谈','家','confirmed',1,'["fact"]','life-confirmed',$4,'{"id":"life-confirmed","status":"confirmed","revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_confirmed","replayed":false}')`, fluctlightID, at.Add(-2*time.Minute), at.Add(15*time.Minute), stableDigest("life-confirmed")); err != nil {
		t.Fatal(err)
	}
	_, confirmed, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), fluctlightID, at)
	if err != nil || stringValue(confirmed["event_id"]) != "life-confirmed" || stringValue(confirmed["authority_status"]) != "confirmed" {
		t.Fatalf("confirmed priority context=%#v err=%v", confirmed, err)
	}
	confirmedRevision := stringValue(confirmed["context_revision"])
	if stringValue(confirmed["schedule_id"]) != "life-schedule" || stringValue(confirmed["schedule_item_id"]) != "life-item-a" {
		t.Fatalf("Event-authoritative context lost the underlying Schedule tuple: %#v", confirmed)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.life_schedules SET revision=2,result=jsonb_set(result,'{revision}','2'::jsonb) WHERE id='life-schedule'`); err != nil {
		t.Fatal(err)
	}
	_, scheduleChangedUnderEvent, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), fluctlightID, at)
	if err != nil || stringValue(scheduleChangedUnderEvent["event_id"]) != "life-confirmed" || stringValue(scheduleChangedUnderEvent["context_revision"]) == confirmedRevision {
		t.Fatalf("Schedule change under authoritative Event did not invalidate Life Context: before=%q after=%#v err=%v", confirmedRevision, scheduleChangedUnderEvent, err)
	}
	confirmed = scheduleChangedUnderEvent
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.life_presence_overlays(id,fluctlight_id,actor_id,current_task,user_presence,status,revision,idempotency_key,request_digest,result,expires_at,created_at) VALUES('life-presence',$1,$2,'讨论设计','online','active',1,'life-presence',$3,'{"id":"life-presence","status":"active","revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_presence","replayed":false}',$4,$5)`, fluctlightID, ownerID, stableDigest("life-presence"), at.Add(10*time.Minute), at.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, withPresence, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), fluctlightID, at)
	presence := mapValue(withPresence["presence"])
	if err != nil || stringValue(withPresence["scene"]) != "客厅" || stringValue(presence["actor_id"]) != ownerID || intValue(presence["revision"]) != 1 || stringValue(withPresence["context_revision"]) == stringValue(confirmed["context_revision"]) {
		t.Fatalf("presence overlay context=%#v err=%v", withPresence, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.life_events SET status='cancelled',revision=revision+1,updated_at=now() WHERE id='life-confirmed'`); err != nil {
		t.Fatal(err)
	}
	_, afterCancel, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), fluctlightID, at)
	if err != nil || stringValue(afterCancel["event_id"]) != "life-inferred" {
		t.Fatalf("cancelled confirmed event still authoritative: %#v err=%v", afterCancel, err)
	}
	_, afterBoundaries, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), fluctlightID, at.Add(40*time.Minute))
	if err != nil || stringValue(afterBoundaries["source"]) != "schedule" || stringValue(afterBoundaries["schedule_item_id"]) != "life-item-b" || len(mapValue(afterBoundaries["presence"])) != 0 || stringValue(afterBoundaries["context_revision"]) == stringValue(afterCancel["context_revision"]) {
		t.Fatalf("time-boundary context=%#v err=%v", afterBoundaries, err)
	}
	pendingOwner, pendingFluctlight := "life-pending-owner", "life-pending-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, pendingOwner, pendingFluctlight)
	_, pending, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), pendingFluctlight, at)
	if err != nil || stringValue(pending["source"]) != "pending" || stringValue(pending["authority_status"]) != "pending" || stringValue(pending["context_revision"]) == "" || stringValue(pending["effective_at"]) == "" || stringValue(pending["expires_at"]) == "" {
		t.Fatalf("pending context=%#v err=%v", pending, err)
	}
}

func TestSceneAndPresenceCapabilitiesUseFrozenLifeContextAndActor(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "life-cap-owner", "life-cap-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	app.ContextResolver = NewAppContextResolver(app)
	registry := app.capabilityRegistry()
	runtime, err := NewCapabilityRuntime(registry, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		SourceFactID: "life-cap-source", MemoryOperation: MemoryForNativeCognition, MemoryConversationMode: MemoryConversationGlobalOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref := stringValue(projection.LifeContext["ref"]); ref == "" || projection.ReferenceIndex.ByRef[ref].Kind != ContextReferenceLifeContext {
		t.Fatalf("pending Life Context ref=%#v", projection.LifeContext)
	}
	sceneInvocation := CapabilityInvocation{
		CallID: "life-scene-start", CapabilityName: "scene_event", SchemaVersion: CapabilityInvocationSchemaVersion,
		Arguments:    json.RawMessage(`{"operation":"start","scene":"书房","activity":"阅读","confidence":0.9}`),
		SourceFactID: "life-cap-source", ActionID: "life-action-start", ProviderRequestID: "provider-life-start",
		Metadata:        InvocationMetadata{FluctlightID: fluctlightID, Surface: CapabilitySurfaceNativeCognition},
		ContextSnapshot: capabilitySnapshotForProjection(projection, sceneCapabilityDefinition().RequiredContext, "life-action-start"),
	}
	preparedScene, _, err := runtime.Prepare(ctx, sceneInvocation)
	if err != nil {
		t.Fatal(err)
	}
	var sceneResult CapabilityResult
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		var executeErr error
		sceneResult, executeErr = runtime.ExecuteTransactional(ctx, tx, preparedScene)
		return executeErr
	}); err != nil {
		t.Fatal(err)
	}
	if sceneResult.Status != "completed" || stringValue(mapValue(sceneResult.Output)["resulting_context_revision"]) == projection.LifeContextRevision {
		t.Fatalf("scene result=%#v", sceneResult)
	}
	var sceneRows int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1 AND kind='scene_inferred'`, fluctlightID).Scan(&sceneRows); err != nil || sceneRows != 1 {
		t.Fatalf("scene rows=%d err=%v", sceneRows, err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		replayed, replayErr := runtime.ExecuteTransactional(ctx, tx, preparedScene)
		if replayErr != nil || !boolValueForTest(mapValue(replayed.Output)["replayed"]) {
			t.Fatalf("scene replay=%#v err=%v", replayed, replayErr)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	projection, err = app.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		SourceFactID: "life-presence-source", MemoryOperation: MemoryForNativeCognition, MemoryConversationMode: MemoryConversationGlobalOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	presenceInvocation := CapabilityInvocation{
		CallID: "life-presence-set", CapabilityName: "presence_event", SchemaVersion: CapabilityInvocationSchemaVersion,
		Arguments:    json.RawMessage(`{"operation":"set","current_task":"一起阅读","user_presence":"online","confidence":0.9}`),
		SourceFactID: "life-presence-source", ActionID: "life-action-presence", ProviderRequestID: "provider-life-presence",
		Metadata:        InvocationMetadata{FluctlightID: fluctlightID, Surface: CapabilitySurfaceNativeCognition},
		ContextSnapshot: capabilitySnapshotForProjection(projection, presenceCapabilityDefinition().RequiredContext, "life-action-presence"),
	}
	preparedPresence, _, err := runtime.Prepare(ctx, presenceInvocation)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, executeErr := runtime.ExecuteTransactional(ctx, tx, preparedPresence)
		return executeErr
	}); err != nil {
		t.Fatal(err)
	}
	var storedActor, presenceStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT actor_id,status FROM public.life_presence_overlays WHERE id=$1`, "presence_overlay_"+stableDigest(fluctlightID+":"+mustPresencePlan(t, preparedPresence).IdempotencyKey)).Scan(&storedActor, &presenceStatus); err != nil || storedActor != ownerID || presenceStatus != "active" {
		t.Fatalf("Presence actor=%q status=%q err=%v", storedActor, presenceStatus, err)
	}
	staleProjection := projection
	_, currentLife, err := app.readLifeContextSnapshotAt(ctx, fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetPresence(ctx, ownerID, fluctlightID, map[string]any{"current_task": "newer task", "user_presence": "online", "expires_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339), "idempotency_key": "owner-newer-presence", "expected_life_context_revision": currentLife["context_revision"]}); err != nil {
		t.Fatal(err)
	}
	staleScene := sceneInvocation
	staleScene.CallID = "life-scene-stale"
	staleScene.ActionID = "life-action-stale"
	staleScene.SourceFactID = staleProjection.SourceFactID
	staleScene.Arguments = json.RawMessage(`{"operation":"switch","scene":"阳台","activity":"休息","confidence":0.8}`)
	staleScene.ContextSnapshot = capabilitySnapshotForProjection(staleProjection, sceneCapabilityDefinition().RequiredContext, staleScene.ActionID)
	preparedStale, _, err := runtime.Prepare(ctx, staleScene)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		result, executeErr := runtime.ExecuteTransactional(ctx, tx, preparedStale)
		if executeErr == nil || len(mapValue(result.Output)) != 0 || result.ErrorCode != "scene_context_stale" {
			t.Fatalf("stale scene result=%#v err=%v", result, executeErr)
		}
		return executeErr
	}); err == nil {
		t.Fatal("stale scene transaction unexpectedly committed")
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1 AND scene='阳台'`, fluctlightID).Scan(&sceneRows); err != nil || sceneRows != 0 {
		t.Fatalf("stale scene inserted rows=%d err=%v", sceneRows, err)
	}
	clearProjection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		SourceFactID: "life-presence-clear-source", MemoryOperation: MemoryForNativeCognition, MemoryConversationMode: MemoryConversationGlobalOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	clearInvocation := CapabilityInvocation{
		CallID: "life-presence-clear", CapabilityName: "presence_event", SchemaVersion: CapabilityInvocationSchemaVersion,
		Arguments: json.RawMessage(`{"operation":"clear","confidence":0.9}`), SourceFactID: clearProjection.SourceFactID,
		ActionID: "life-action-presence-clear", ProviderRequestID: "provider-life-presence-clear",
		Metadata:        InvocationMetadata{FluctlightID: fluctlightID, Surface: CapabilitySurfaceNativeCognition},
		ContextSnapshot: capabilitySnapshotForProjection(clearProjection, presenceCapabilityDefinition().RequiredContext, "life-action-presence-clear"),
	}
	preparedClear, _, err := runtime.Prepare(ctx, clearInvocation)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, executeErr := runtime.ExecuteTransactional(ctx, tx, preparedClear)
		return executeErr
	}); err != nil {
		t.Fatal(err)
	}
	var activePresence int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_presence_overlays WHERE fluctlight_id=$1 AND status='active'`, fluctlightID).Scan(&activePresence); err != nil || activePresence != 0 {
		t.Fatalf("Presence clear left active rows=%d err=%v", activePresence, err)
	}
	_, afterClear, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), fluctlightID, time.Now().UTC())
	if err != nil || len(mapValue(afterClear["presence"])) != 0 {
		t.Fatalf("cleared Presence resurfaced: %#v err=%v", afterClear, err)
	}
}

func mustPresencePlan(t *testing.T, invocation CapabilityInvocation) preparedPresenceMutation {
	t.Helper()
	plan, err := presencePlanFromInvocation(invocation)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestLifeContextStaticGuardHasNoSemanticSceneActionHeuristic(t *testing.T) {
	for _, path := range []string{"mutations.go", "wakeup.go", "autonomy.go", "workflow_ops.go", "native_capabilities.go", "builtin_capabilities.go"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ToLower(string(content))
		for _, forbidden := range []string{"strings.contains(scene", "strings.contains(activity", "strings.contains(location", "regexp.matchstring(scene", "regexp.matchstring(activity", "regexp.matchstring(location"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("semantic Life Context action heuristic %q found in %s", forbidden, path)
			}
		}
	}
}

func TestCognitionSurfacesBindFrozenProjectionBeforeCapabilityPrepare(t *testing.T) {
	app := &App{}
	projection := ContextProjection{
		SchemaVersion: "fluctlight.context.v3", FluctlightID: "life-bind-fl", OwnerActorID: "life-bind-owner",
		SourceFactID: "life-bind-source", LifeContextRevision: "life_ctx_0123456789abcdef0123456789abcdef",
		LifeContext:    map[string]any{"source": "pending", "authority_status": "pending", "context_revision": "life_ctx_0123456789abcdef0123456789abcdef", "revision": 7},
		ReferenceIndex: ContextReferenceIndex{SchemaVersion: contextReferenceIndexVersion, FluctlightID: "life-bind-fl", OwnerActorID: "life-bind-owner", SpeakerActorID: "life-bind-owner", ByRef: map[string]ContextReference{}},
	}
	invocation := CapabilityInvocation{CallID: "life-bind-scene", CapabilityName: "scene_event", Arguments: json.RawMessage(`{"operation":"start","scene":"书房","activity":"阅读","confidence":0.9}`)}
	for _, surface := range []CapabilitySurface{CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition} {
		bound, err := app.bindCapabilityInvocationsToProjection([]CapabilityInvocation{invocation}, projection, "life-action-"+string(surface), projection.SourceFactID, surface)
		if err != nil || len(bound) != 1 {
			t.Fatalf("surface %s bound=%#v err=%v", surface, bound, err)
		}
		life := mapValue(bound[0].ContextSnapshot["current_life"])
		identity := mapValue(bound[0].ContextSnapshot["identity"])
		if stringValue(life["context_revision"]) != projection.LifeContextRevision || stringValue(identity["action_id"]) != bound[0].ActionID || bound[0].Metadata.Surface != surface {
			t.Fatalf("surface %s snapshot=%#v invocation=%#v", surface, bound[0].ContextSnapshot, bound[0])
		}
	}
	reply := CapabilityInvocation{CallID: "life-bind-reply", CapabilityName: "conversation.reply", Arguments: json.RawMessage(`{"text":"我在。"}`)}
	boundReply, err := app.bindCapabilityInvocationsToProjection([]CapabilityInvocation{reply}, projection, "life-action-conversation", projection.SourceFactID, CapabilitySurfaceConversation)
	if err != nil || len(boundReply) != 1 || stringValue(mapValue(boundReply[0].ContextSnapshot["current_life"])["context_revision"]) != projection.LifeContextRevision {
		t.Fatalf("conversation reply did not freeze current Life Context: bound=%#v err=%v", boundReply, err)
	}
	for _, path := range []string{"wakeup.go", "autonomy.go"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		bindAt := strings.Index(text, "bindCapabilityInvocationsToProjection")
		prepareAt := strings.Index(text, "prepareCapabilityInvocations")
		if bindAt < 0 || prepareAt < 0 || bindAt > prepareAt {
			t.Fatalf("%s does not bind the model-visible projection before Prepare", path)
		}
	}
	conversationSource, err := os.ReadFile("mutations.go")
	if err != nil {
		t.Fatal(err)
	}
	if reloadAt, prepareAt := strings.Index(string(conversationSource), `capabilityInvocationsFromValue(frozen.Payload["capability_invocations"])`), strings.Index(string(conversationSource), "prepareCapabilityInvocations"); reloadAt < 0 || prepareAt < 0 || reloadAt > prepareAt {
		t.Fatal("conversation does not reload projection-bound frozen invocations before Prepare")
	}
}

func TestConversationRejectsLifeContextChangeBetweenDecisionAndSettlement(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "life-turn-owner", "life-turn-fluctlight", "life-turn-conversation"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'life context stale')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	endpointID := "life-turn-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://life-turn.invalid','life-turn-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'life-turn-model','structured_output',4096,5,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	var providerCalls atomic.Int32
	var decisionLifeRevision string
	app := &App{DB: repository}
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		providerCalls.Add(1)
		body, _ := io.ReadAll(request.Body)
		lifeRef := regexp.MustCompile(`life_context:ctx_[a-f0-9]{32}`).FindString(string(body))
		decisionLifeRevision = regexp.MustCompile(`life_ctx_[a-f0-9]{32}`).FindString(string(body))
		if lifeRef == "" || decisionLifeRevision == "" {
			t.Fatalf("Provider request omitted frozen Life Context ref: %s", body)
		}
		if _, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, map[string]any{
			"kind": "interruption", "start_at": time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
			"end_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339), "scene": "客厅", "activity": "临时交谈",
			"location": "家", "evidence_refs": []any{"external-fact"}, "idempotency_key": "life-turn-interruption",
			"expected_life_context_revision": decisionLifeRevision,
		}); err != nil {
			t.Fatal(err)
		}
		structured := map[string]any{
			"action_type": "reply", "response_intent": "observe", "visible_text": "我会按刚才看到的场景处理。", "tool_calls": []any{},
			"appraisal": map[string]any{
				"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.5, "social_threat": 0.0,
				"controllability": 0.5, "responsibility": 0.5, "relationship_significance": 0.0, "expected_effect": 0.5,
				"evidence_refs": []any{}, "event_kind": "conversation", "direction": "mixed", "drive_signals": []any{},
			},
			"attention": "listen", "thought": "hold", "desire": "wait", "agency": "no action",
			"influences": []any{map[string]any{"ref": lifeRef, "role": "constrains", "confidence": 0.9, "note": "决策基于当时尚未发生新事件的生活上下文"}},
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{
			"content": jsonString(structured),
			"tool_calls": []any{
				map[string]any{"id": "life-turn-scene-call", "type": "function", "function": map[string]any{"name": "scene_event", "arguments": `{"operation":"start","scene":"书房","activity":"阅读","confidence":0.9}`}},
				map[string]any{"id": "life-turn-reply-call", "type": "function", "function": map[string]any{"name": "conversation.reply", "arguments": `{"text":"我会按刚才看到的场景处理。"}`}},
			},
		}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}
	app.Provider = &ProviderClient{DB: repository, HTTP: providerHTTP}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = app.capabilityRegistry()
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	_, err = app.HandleTurn(ctx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "先等一下", "idempotency_key": "life-turn-user", "turn_id": "life-turn-1", "attachment_refs": []any{},
	})
	if err == nil || !errors.Is(err, ErrLifeContextStale) {
		t.Fatalf("stale conversation err=%v", err)
	}
	if providerCalls.Load() != 1 {
		t.Fatalf("Provider calls=%d", providerCalls.Load())
	}
	var assistantCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID).Scan(&assistantCount); err != nil || assistantCount != 0 {
		t.Fatalf("stale turn persisted assistant count=%d err=%v", assistantCount, err)
	}
	var frozenStatus, frozenError string
	var frozenPayload []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,''),payload FROM public.cognition_frozen_actions WHERE inbox_id=(SELECT id FROM public.cognition_inbox WHERE idempotency_key='life-turn-user' AND event_type='conversation.turn')`).Scan(&frozenStatus, &frozenError, &frozenPayload); err != nil || frozenStatus != "failed" || frozenError != "life_context_stale" {
		t.Fatalf("stale frozen status=%q error=%q err=%v", frozenStatus, frozenError, err)
	}
	invocations, err := capabilityInvocationsFromValue(decodeObject(frozenPayload)["capability_invocations"])
	if err != nil || len(invocations) != 2 {
		t.Fatalf("frozen invocations=%#v err=%v", invocations, err)
	}
	var sceneInvocation CapabilityInvocation
	for _, invocation := range invocations {
		if invocation.CapabilityName == "scene_event" {
			sceneInvocation = invocation
		}
	}
	plan, err := scenePlanFromInvocation(sceneInvocation)
	if err != nil || plan.ExpectedLifeContextRevision != decisionLifeRevision || stringValue(mapValue(sceneInvocation.ContextSnapshot["current_life"])["context_revision"]) != decisionLifeRevision {
		t.Fatalf("conversation capability did not preserve decision context plan=%#v snapshot=%#v err=%v", plan, sceneInvocation.ContextSnapshot, err)
	}
	var staleCapabilitySceneCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1 AND scene='书房'`, fluctlightID).Scan(&staleCapabilitySceneCount); err != nil || staleCapabilitySceneCount != 0 {
		t.Fatalf("stale conversation capability mutated scene count=%d err=%v", staleCapabilitySceneCount, err)
	}
}
