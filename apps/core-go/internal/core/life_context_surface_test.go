package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type capturingSceneCapability struct {
	delegate sceneEventCapability
	mu       sync.Mutex
	prepared []CapabilityInvocation
}

func (c *capturingSceneCapability) Definition() CapabilityDefinition { return c.delegate.Definition() }
func (c *capturingSceneCapability) RequiredContext() []ContextSlot {
	return c.delegate.RequiredContext()
}
func (c *capturingSceneCapability) Execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return c.delegate.Execute(ctx, invocation, resolved)
}
func (c *capturingSceneCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return c.delegate.ExecuteTx(ctx, tx, invocation, resolved)
}
func (c *capturingSceneCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	prepared, err := c.delegate.Prepare(ctx, invocation, resolved)
	if err == nil {
		c.mu.Lock()
		c.prepared = append(c.prepared, prepared)
		c.mu.Unlock()
	}
	return prepared, err
}
func (c *capturingSceneCapability) lastPrepared() (CapabilityInvocation, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.prepared) == 0 {
		return CapabilityInvocation{}, false
	}
	return c.prepared[len(c.prepared)-1], true
}

func TestImageDurableIntentRejectsPreCommitStaleContextAndKeepsPostCommitSnapshot(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "owner-media-life", "fluctlight-media-life"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	conversationID, messageID := "media-life-conversation", "media-life-message"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id) VALUES($1,$2)`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,idempotency_key) VALUES($1,$2,1,$3,'assistant','',$1)`, messageID, conversationID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	capability := imageGenerateCapability{service: app}
	initialLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	resolved := CapabilityContext{
		Visual: &VisualIdentityContext{Data: map[string]any{"status": "active", "ref": "visual:test"}},
		Life:   &CurrentLifeContext{Data: initialLife},
		Outfit: &AppearanceContext{Data: map[string]any{"outfit": "针织衫"}},
		State:  &CurrentStateContext{Data: map[string]any{"mood": map[string]any{"label": "平静"}}},
	}
	baseInvocation := CapabilityInvocation{
		CallID: "media-life-call", CapabilityName: "media.image.generate", SchemaVersion: CapabilityInvocationSchemaVersion,
		Arguments: json.RawMessage(`{"intent":"记录此刻的场景"}`), SourceFactID: "media-life-fact", ActionID: "media-life-action",
		ProviderRequestID: "media-life-provider", Metadata: InvocationMetadata{FluctlightID: fluctlightID, ConversationID: conversationID, Surface: CapabilitySurfaceConversation},
	}
	preparedInitial, err := capability.Prepare(ctx, baseInvocation, resolved)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	firstEvent, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, map[string]any{
		"kind": "owner_event", "start_at": now.Add(-time.Minute).Format(time.RFC3339), "end_at": now.Add(time.Hour).Format(time.RFC3339),
		"scene": "客厅", "activity": "交谈", "evidence_refs": []any{"media-life-event-a"},
		"expected_life_context_revision": initialLife["context_revision"], "idempotency_key": "media-life-event-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	liveAfterFirstEvent := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	rawConcept, found, err := capabilityPreparedData(preparedInitial, "media_concept")
	if err != nil || !found {
		t.Fatalf("prepared media concept found=%v err=%v", found, err)
	}
	forgedConcept := cloneMap(mapValue(rawConcept))
	mapValue(forgedConcept["context_binding"])["current_life"] = liveAfterFirstEvent
	forgedPrepared, err := withCapabilityPreparedData(preparedInitial, "media_concept", forgedConcept)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	forgedResult, forgedErr := capability.ExecuteDeferredTx(ctx, tx, forgedPrepared, resolved, OutputBindingV1{TargetKind: "conversation_message", TargetRef: messageID})
	_ = tx.Rollback(ctx)
	if forgedErr == nil || forgedResult.ErrorCode != "media_prepared_context_mismatch" {
		t.Fatalf("split media authority result=%#v err=%v", forgedResult, forgedErr)
	}
	tx, err = repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	staleResult, staleErr := capability.ExecuteDeferredTx(ctx, tx, preparedInitial, resolved, OutputBindingV1{TargetKind: "conversation_message", TargetRef: messageID})
	_ = tx.Rollback(ctx)
	if staleErr == nil || staleResult.ErrorCode != "media_context_stale" {
		t.Fatalf("stale media intent result=%#v err=%v", staleResult, staleErr)
	}
	var intentCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&intentCount); err != nil || intentCount != 0 {
		t.Fatalf("stale media intent rows=%d err=%v", intentCount, err)
	}

	frozenLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	resolved.Life = &CurrentLifeContext{Data: frozenLife}
	preparedFrozen, err := capability.Prepare(ctx, baseInvocation, resolved)
	if err != nil {
		t.Fatal(err)
	}
	tx, err = repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	committedResult, err := capability.ExecuteDeferredTx(ctx, tx, preparedFrozen, resolved, OutputBindingV1{TargetKind: "conversation_message", TargetRef: messageID})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	secondEvent, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, map[string]any{
		"kind": "owner_event", "start_at": time.Now().UTC().Add(-30 * time.Second).Format(time.RFC3339), "end_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
		"scene": "厨房", "activity": "做饭", "evidence_refs": []any{"media-life-event-b"},
		"expected_life_context_revision": frozenLife["context_revision"], "idempotency_key": "media-life-event-b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(firstEvent["id"]) == stringValue(secondEvent["id"]) {
		t.Fatal("test events unexpectedly share identity")
	}
	intentID := stringValue(mapValue(committedResult.Output)["media_intent_id"])
	var storedPrompt string
	if err := repository.Pool().QueryRow(ctx, `SELECT prompt FROM public.media_intents WHERE id=$1`, intentID).Scan(&storedPrompt); err != nil {
		t.Fatal(err)
	}
	workerInput := mediaPromptInput(mediaIntent{Prompt: storedPrompt})
	var workerConcept map[string]any
	if err := json.Unmarshal([]byte(workerInput), &workerConcept); err != nil {
		t.Fatalf("worker media input invalid: %v: %s", err, workerInput)
	}
	workerLife := mapValue(mapValue(workerConcept["context_binding"])["current_life"])
	liveLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	if stringValue(workerLife["context_revision"]) != stringValue(frozenLife["context_revision"]) || stringValue(workerLife["scene"]) != "客厅" || stringValue(workerLife["context_revision"]) == stringValue(liveLife["context_revision"]) || stringValue(liveLife["scene"]) != "厨房" {
		t.Fatalf("media worker did not keep committed snapshot: worker=%#v live=%#v", workerLife, liveLife)
	}
}

func TestWakeUpAndDailyReviewPrepareAgainstModelVisibleLifeThenFailStale(t *testing.T) {
	for _, surface := range []string{"wake_up", "daily_review"} {
		surface := surface
		t.Run(surface, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			ownerID, fluctlightID := "owner-"+surface+"-life", "fluctlight-"+surface+"-life"
			seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
			conversationID := "conversation-" + surface + "-life"
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id) VALUES($1,$2)`, conversationID, ownerID); err != nil {
				t.Fatal(err)
			}
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
				t.Fatal(err)
			}
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
				t.Fatal(err)
			}
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, ownerID, fluctlightID, conversationID); err != nil {
				t.Fatal(err)
			}
			app := &App{DB: repository}
			initialLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
			schedulePayload := fullDaySchedulePayloadForTest(time.Now().UTC(), "surface-schedule-"+surface, stringValue(initialLife["context_revision"]))
			if _, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, schedulePayload); err != nil {
				t.Fatal(err)
			}
			spy := &capturingSceneCapability{delegate: sceneEventCapability{service: app}}
			app.ContextResolver = NewAppContextResolver(app)
			app.Capabilities = mustCapabilityRegistry(spy)
			runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
			if err != nil {
				t.Fatal(err)
			}
			app.Runtime = runtime
			endpointID := "endpoint-" + surface + "-life"
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://surface-life.invalid','surface-life-secret','ready',now())`, endpointID); err != nil {
				t.Fatal(err)
			}
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'surface-life-model','structured_output,tool_calling',4096,5,'{}')`, endpointID); err != nil {
				t.Fatal(err)
			}
			var providerCalls atomic.Int32
			var decisionRevision string
			providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				providerCalls.Add(1)
				body, _ := io.ReadAll(request.Body)
				lifeRef := regexp.MustCompile(`life_context:ctx_[a-f0-9]{32}`).FindString(string(body))
				decisionRevision = regexp.MustCompile(`life_ctx_[a-f0-9]{32}`).FindString(string(body))
				if lifeRef == "" || decisionRevision == "" {
					return nil, errors.New("model-visible Life Context authority missing")
				}
				now := time.Now().UTC()
				if _, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, map[string]any{
					"kind": "owner_event", "start_at": now.Add(-time.Minute).Format(time.RFC3339), "end_at": now.Add(time.Hour).Format(time.RFC3339),
					"scene": "变更后的客厅", "activity": "临时交谈", "evidence_refs": []any{"surface-life-external-" + surface},
					"expected_life_context_revision": decisionRevision, "idempotency_key": "surface-life-change-" + surface,
				}); err != nil {
					return nil, err
				}
				structured := map[string]any{
					"action_type": "no_op", "response_intent": "根据原快照更新场景", "tool_calls": []any{},
					"influences": []any{map[string]any{"ref": lifeRef, "role": "constrains", "confidence": 0.9, "note": "使用模型实际看到的生活上下文"}},
				}
				if surface == "wake_up" {
					structured["action_type"] = "scene_change"
					structured["evidence_refs"] = []any{}
				}
				response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{
					"content":    jsonString(structured),
					"tool_calls": []any{map[string]any{"id": "surface-scene-" + surface, "type": "function", "function": map[string]any{"name": "scene_event", "arguments": `{"operation":"start","scene":"原快照书房","activity":"阅读","confidence":0.9}`}}},
				}}}}
				return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
			})}
			app.Provider = &ProviderClient{DB: repository, HTTP: providerHTTP}
			var runErr error
			if surface == "wake_up" {
				_, runErr = app.ProcessWakeUp(ctx, fluctlightID, 7)
			} else {
				_, runErr = app.ProcessDailyReview(ctx, fluctlightID, stringValue(schedulePayload["local_date"]))
			}
			if !errors.Is(runErr, ErrLifeContextStale) {
				t.Fatalf("%s stale run err=%v", surface, runErr)
			}
			if providerCalls.Load() != 1 {
				t.Fatalf("%s Provider calls=%d", surface, providerCalls.Load())
			}
			prepared, ok := spy.lastPrepared()
			if !ok {
				t.Fatalf("%s did not prepare the frozen scene invocation", surface)
			}
			plan, err := scenePlanFromInvocation(prepared)
			if err != nil {
				t.Fatal(err)
			}
			frozenLife := mapValue(prepared.ContextSnapshot["current_life"])
			liveLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
			if plan.ExpectedLifeContextRevision != decisionRevision || stringValue(frozenLife["context_revision"]) != decisionRevision || stringValue(liveLife["context_revision"]) == decisionRevision || prepared.ActionID == "" {
				t.Fatalf("%s mixed model/frozen/live context: plan=%#v snapshot=%#v live=%#v", surface, plan, frozenLife, liveLife)
			}
			var wakeCount, actionCount int
			if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_wakeups WHERE fluctlight_id=$1`, fluctlightID).Scan(&wakeCount); err != nil {
				t.Fatal(err)
			}
			if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.autonomy_actions WHERE fluctlight_id=$1`, fluctlightID).Scan(&actionCount); err != nil {
				t.Fatal(err)
			}
			if wakeCount != 0 || actionCount != 0 {
				t.Fatalf("%s stale decision persisted wake=%d action=%d", surface, wakeCount, actionCount)
			}
		})
	}
}

func TestScheduleReplayCannotUsePreparedLifeRevisionDifferentFromFrozenSnapshot(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "owner-schedule-split", "fluctlight-schedule-split"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	initialLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	base := fullDaySchedulePayloadForTest(time.Now().UTC(), "schedule-split-base", stringValue(initialLife["context_revision"]))
	if _, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, base); err != nil {
		t.Fatal(err)
	}
	projection := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "schedule-split-fact")
	app.SchedulePlanner = fakeSchedulePlanner{plan: map[string]any{
		"local_date": projection.Schedule["local_date"], "timezone": projection.Schedule["timezone"],
		"expected_revision": projection.Schedule["revision"], "completed_before": projection.Schedule["completed_before"],
		"items": projection.Schedule["items"], "reschedule_policy": map[string]any{},
	}}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = app.capabilityRegistry()
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	prepared := prepareLifeCapabilityForTest(t, ctx, app, runtime, projection, "schedule.replan", "schedule-split-call", "schedule-split-action", `{"intent":"保持计划但验证冻结 authority"}`)
	if _, err := app.SetPresence(ctx, ownerID, fluctlightID, map[string]any{
		"current_task": "外部状态变化", "expected_life_context_revision": projection.LifeContextRevision, "idempotency_key": "schedule-split-presence",
	}); err != nil {
		t.Fatal(err)
	}
	liveLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	rawPlan, found, err := capabilityPreparedData(prepared, "schedule_plan")
	if err != nil || !found {
		t.Fatalf("schedule plan found=%v err=%v", found, err)
	}
	forgedPlan := cloneMap(mapValue(rawPlan))
	forgedPlan["expected_life_context_revision"] = liveLife["context_revision"]
	prepared, err = withCapabilityPreparedData(prepared, "schedule_plan", forgedPlan)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, executeErr := runtime.ExecuteTransactional(ctx, tx, prepared)
	_ = tx.Rollback(ctx)
	if executeErr == nil || result.ErrorCode != "schedule_prepared_context_mismatch" {
		t.Fatalf("split Schedule authority result=%#v err=%v", result, executeErr)
	}
	var scheduleCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_schedules WHERE fluctlight_id=$1`, fluctlightID).Scan(&scheduleCount); err != nil || scheduleCount != 1 {
		t.Fatalf("split Schedule authority mutated versions=%d err=%v", scheduleCount, err)
	}
}

func TestConcurrentDailyReviewUsesOneMainProviderCall(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "owner-daily-single", "fluctlight-daily-single", "conversation-daily-single"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id) VALUES($1,$2)`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, ownerID, fluctlightID, conversationID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	initialLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	schedulePayload := fullDaySchedulePayloadForTest(time.Now().UTC(), "daily-single-schedule", stringValue(initialLife["context_revision"]))
	if _, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, schedulePayload); err != nil {
		t.Fatal(err)
	}
	endpointID := "daily-single-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://daily-single.invalid','daily-single-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'daily-single-model','structured_output,tool_calling',4096,10,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	providerReceived := make(chan struct{})
	releaseProvider := make(chan struct{})
	var providerCalls atomic.Int32
	app.Provider = &ProviderClient{DB: repository, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		providerCalls.Add(1)
		_, _ = io.ReadAll(request.Body)
		close(providerReceived)
		select {
		case <-releaseProvider:
		case <-request.Context().Done():
			return nil, request.Context().Err()
		}
		structured := map[string]any{"action_type": "no_op", "response_intent": "今日无需主动行动", "tool_calls": []any{}, "influences": []any{}}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(structured), "tool_calls": []any{}}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = app.capabilityRegistry()
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	localDate := stringValue(schedulePayload["local_date"])
	firstDone := make(chan error, 1)
	go func() {
		_, err := app.ProcessDailyReview(ctx, fluctlightID, localDate)
		firstDone <- err
	}()
	select {
	case <-providerReceived:
	case <-time.After(5 * time.Second):
		close(releaseProvider)
		t.Fatal("first daily review did not reach Provider")
	}
	second, err := app.ProcessDailyReview(ctx, fluctlightID, localDate)
	if err != nil || stringValue(second["status"]) != "in_progress" {
		close(releaseProvider)
		t.Fatalf("concurrent daily review=%#v err=%v", second, err)
	}
	close(releaseProvider)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if providerCalls.Load() != 1 {
		t.Fatalf("daily review Provider calls=%d, want 1", providerCalls.Load())
	}
	var actionCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.autonomy_actions WHERE fluctlight_id=$1`, fluctlightID).Scan(&actionCount); err != nil || actionCount != 1 {
		t.Fatalf("daily review actions=%d err=%v", actionCount, err)
	}
}
