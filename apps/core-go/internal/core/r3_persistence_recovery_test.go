package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestR3ScheduleFailureDoesNotBecomePerpetualPending(t *testing.T) {
	calls := 0
	router := newFakeProviderRouter().on("schedule_response", func(_ map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Structured: map[string]any{"items": []any{}, "reschedule_policy": map[string]any{}}}
	})
	ctx, repo, app := personaPreparationFixture(t, router)
	if _, err := app.ProcessPersonaPreparationIntent(ctx, latestPreparationID(t, ctx, repo)); err != nil {
		t.Fatal(err)
	}
	result, err := app.EnsureCurrentDaySchedule(ctx, "prep-role")
	if err == nil || result != nil || calls == 0 {
		t.Fatalf("invalid schedule became pending: result=%v calls=%d err=%v", result, calls, err)
	}
	var count int
	if err = repo.Pool().QueryRow(ctx, "SELECT count(*) FROM public.life_schedules WHERE fluctlight_id='prep-role'").Scan(&count); err != nil || count != 0 {
		t.Fatal("invalid schedule persisted", count, err)
	}
}
func TestR3ConversationSegmentConcurrentReplayUsesOneModel(t *testing.T) {
	ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 4)
	base := time.Now().UTC().Add(-48 * time.Hour)
	base = time.Date(base.Year(), base.Month(), base.Day(), 12, 0, 0, 0, time.UTC)
	for sequence := 1; sequence <= 4; sequence++ {
		if _, err := repo.Pool().Exec(ctx, "UPDATE public.conversation_messages SET created_at=$2 WHERE id=$1", "summary-message-"+numberString(sequence, 0), base.Add(time.Duration(sequence)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('segment-provider','openai_compatible','http://segment.invalid','segment-secret','ready',now());
 INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection','segment-provider','segment-model','structured_output',4096,10,'{}')`); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	extra := make(chan struct{}, 1)
	var calls atomic.Int32
	httpClient := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		_, _ = io.ReadAll(request.Body)
		if calls.Add(1) == 1 {
			close(started)
			select {
			case <-release:
			case <-request.Context().Done():
				return nil, request.Context().Err()
			}
		} else {
			select {
			case extra <- struct{}{}:
			default:
			}
		}
		value := map[string]any{"schema_version": conversationSegmentSchemaVersion, "summary": "真实对话片段", "ending_state": "计划尚待确认", "open_threads": []any{"计划"}, "core_events": []any{"讨论计划"}}
		return embeddingHTTPResponse(request, http.StatusOK, jsonString(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(value)}}}})), nil
	})}
	app := &App{DB: repo, Provider: &ProviderClient{DB: repo, HTTP: httpClient}}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error { return app.enqueueConversationSegmentTx(ctx, tx, fid, conversation, false) }); err != nil {
		t.Fatal(err)
	}
	var intent string
	if err := repo.Pool().QueryRow(ctx, "SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.segment' AND status='pending'").Scan(&intent); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	go func() { _, err := app.ProcessConversationSegmentIntent(ctx, intent); done <- err }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("segment did not reach model")
	}
	go func() { _, err := app.ProcessConversationSegmentIntent(ctx, intent); done <- err }()
	select {
	case <-extra:
		t.Error("second segment model ran before first completed")
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	for index := 0; index < 2; index++ {
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("models=%d", calls.Load())
	}
	var summaries int
	if err := repo.Pool().QueryRow(ctx, "SELECT count(*) FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND status='active'", fid).Scan(&summaries); err != nil || summaries != 1 {
		t.Fatal(summaries, err)
	}
}
