package core

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
)

func seedConversationSegmentSource(t *testing.T) (context.Context, *PostgresRepository, *App, string, func()) {
	t.Helper()
	ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 2)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET created_at=now()-interval '2 days' + sequence * interval '1 minute' WHERE conversation_id=$1`, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('segment-generation-provider','openai_compatible','http://segment.invalid','fixture','ready',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,timeout_seconds) VALUES('reflection','segment-generation-provider','fixture','structured_output',10)`); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo}
	enqueue := func() {
		t.Helper()
		if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
			return app.enqueueConversationSegmentTx(ctx, tx, fid, conversation, true)
		}); err != nil {
			t.Fatal(err)
		}
	}
	enqueue()
	var intentID string
	if err := repo.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.segment'`).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	return ctx, repo, app, intentID, enqueue
}

func TestConversationSegmentOlderGenerationCannotSettleReopenedIntent(t *testing.T) {
	ctx, repo, app, intentID, enqueue := seedConversationSegmentSource(t)
	// Old Temporal histories omitted generation; they belong to generation zero.
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=payload-'source_generation' WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	calls := 0
	app.Provider = &ProviderClient{DB: repo, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='superseded' WHERE intent_id=$1`, intentID); err != nil {
				t.Fatal(err)
			}
			enqueue()
		}
		value := map[string]any{"schema_version": conversationSegmentSchemaVersion, "summary": "一段对话", "ending_state": "稍后继续", "open_threads": []any{}, "core_events": []any{}}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(value)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}}
	if _, err := app.ProcessConversationSegmentIntent(ctx, intentID); !errors.Is(err, errConversationSegmentGenerationChanged) {
		t.Fatalf("old segment generation was not fenced after reopen: %v", err)
	}
	var status string
	var generation, summaries int
	if err := repo.Pool().QueryRow(ctx, `SELECT status,(payload->>'source_generation')::integer,(SELECT count(*) FROM public.conversation_summaries) FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status, &generation, &summaries); err != nil || status != "pending" || generation != 1 || summaries != 0 || calls != 1 {
		t.Fatalf("new generation changed: status=%q generation=%d summaries=%d calls=%d err=%v", status, generation, summaries, calls, err)
	}
	if _, err := app.ProcessConversationSegmentIntentAt(ctx, intentID, 0); !errors.Is(err, errConversationSegmentGenerationChanged) || calls != 1 {
		t.Fatalf("old segment activity read the new generation: calls=%d err=%v", calls, err)
	}
	result, err := app.ProcessConversationSegmentIntent(ctx, intentID)
	if err != nil || result["status"] != "active" || calls != 2 {
		t.Fatalf("new generation did not complete: result=%v calls=%d err=%v", result, calls, err)
	}
}
