package core

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestDailyMemoryOlderGenerationCannotSettleReopenedIntent(t *testing.T) {
	ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 2)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversation, owner, fid); err != nil {
		t.Fatal(err)
	}
	day := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET created_at=$2::date + sequence * interval '1 minute' WHERE conversation_id=$1`, conversation, day); err != nil {
		t.Fatal(err)
	}
	messages, err := readConversationSummaryMessages(ctx, repo.Pool(), conversation, 1, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('daily-generation-provider','openai_compatible','http://daily.invalid','fixture','ready',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key,local_date,timezone,started_at,ended_at)
		VALUES('daily-generation-source',$1,$2,1,2,$3,$4,'a real source','active',1,'daily-generation-provider','fixture','fixture','fixture','fixture','fixture','fixture','daily-generation-source',$5::date,'UTC',$6,$7)`, fid, conversation, jsonBytes(conversationSummarySourceRefs(messages)), conversationSummarySourceDigest(messages), day, messages[0].CreatedAt, messages[1].CreatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection','daily-generation-provider','fixture','structured_output',4096,10,'{}')`); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo}
	enqueue := func() {
		t.Helper()
		if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
			return app.enqueueConversationDailyMemoryTx(ctx, tx, fid, conversation, day, "UTC")
		}); err != nil {
			t.Fatal(err)
		}
	}
	enqueue()
	var intentID string
	if err := repo.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.daily_memory'`).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	calls := 0
	app.Provider = &ProviderClient{DB: repo, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed' WHERE intent_id=$1`, intentID); err != nil {
				t.Fatal(err)
			}
			enqueue()
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(map[string]any{"schema_version": conversationDailyMemorySchemaVersion, "episode": "一段记忆"})}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}}
	if _, err := app.ProcessConversationDailyMemoryIntent(context.Background(), intentID); !errors.Is(err, errConversationDailyGenerationChanged) {
		t.Fatalf("old generation was not fenced after the intent was reopened: %v", err)
	}
	var status string
	var generation, memories int
	if err := repo.Pool().QueryRow(ctx, `SELECT status,(payload->>'source_generation')::integer,(SELECT count(*) FROM public.conversation_daily_memories) FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status, &generation, &memories); err != nil || status != "retry" || generation != 1 || memories != 0 || calls != 1 {
		t.Fatalf("new generation changed: status=%q generation=%d memories=%d calls=%d err=%v", status, generation, memories, calls, err)
	}
	if _, err := app.ProcessConversationDailyMemoryIntentAt(ctx, intentID, 0); !errors.Is(err, errConversationDailyGenerationChanged) || calls != 1 {
		t.Fatalf("an old activity read the new generation: calls=%d err=%v", calls, err)
	}
	result, err := app.ProcessConversationDailyMemoryIntent(ctx, intentID)
	if err != nil || result["status"] != "completed" || calls != 2 {
		t.Fatalf("new generation did not complete: result=%v calls=%d err=%v", result, calls, err)
	}
}

func TestDailyMemoryLegacyGenerationCannotFollowReopen(t *testing.T) {
	ctx, repo, _, fid, conversation := seedConversationSummaryAuthority(t)
	app := &App{DB: repo}
	day := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")
	enqueue := func() {
		t.Helper()
		if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
			return app.enqueueConversationDailyMemoryTx(ctx, tx, fid, conversation, day, "UTC")
		}); err != nil {
			t.Fatal(err)
		}
	}
	enqueue()
	var intentID string
	if err := repo.Pool().QueryRow(ctx, `UPDATE public.platform_workflow_intents SET payload=payload-'source_generation' WHERE intent_type='conversation.daily_memory' RETURNING intent_id`).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	if result, err := app.ProcessConversationDailyMemoryIntentAt(ctx, intentID, 0); err != nil || result["status"] != "no_source" {
		t.Fatalf("legacy generation zero did not complete: result=%v err=%v", result, err)
	}
	enqueue()
	if _, err := app.ProcessConversationDailyMemoryIntentAt(ctx, intentID, 0); !errors.Is(err, errConversationDailyGenerationChanged) {
		t.Fatalf("legacy activity followed a reopened intent: %v", err)
	}
	if result, err := app.ProcessConversationDailyMemoryIntentAt(ctx, intentID, 1); err != nil || result["status"] != "no_source" {
		t.Fatalf("new generation could not complete: result=%v err=%v", result, err)
	}
}
