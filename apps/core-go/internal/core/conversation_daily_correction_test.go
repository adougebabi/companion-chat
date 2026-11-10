package core

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Corrections leave the raw conversation intact. A daily job must therefore
// consult Memory authority both before the model and again before settlement.
func TestDailyMemoryBlocksCorrectedSummarySources(t *testing.T) {
	for _, duringModel := range []bool{false, true} {
		name := "before_model"
		if duringModel {
			name = "during_model"
		}
		t.Run(name, func(t *testing.T) {
			ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
			seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 4)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversation, owner, fid); err != nil {
				t.Fatal(err)
			}
			day := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET created_at=$2::date + sequence * interval '1 minute' WHERE conversation_id=$1`, conversation, day); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('daily-correction-provider','openai_compatible','http://daily.invalid','fixture','ready',now())`); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection','daily-correction-provider','fixture','structured_output',4096,10,'{}')`); err != nil {
				t.Fatal(err)
			}
			for _, segment := range []struct {
				id, status string
				from, to   int
			}{{"daily-corrected", "consolidated", 1, 2}, {"daily-clean", "active", 3, 4}} {
				messages, err := readConversationSummaryMessages(ctx, repo.Pool(), conversation, segment.from, segment.to, segment.to-segment.from+1)
				if err != nil {
					t.Fatal(err)
				}
				_, err = repo.Pool().Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key,local_date,timezone,started_at,ended_at)
					VALUES($1::text,$2,$3,$4,$5,$6,$7,$1::text,$8,1,'daily-correction-provider','fixture',$1::text,'fixture','fixture','fixture',$1::text,$1::text,$9::date,'UTC',$10,$11)`, segment.id, fid, conversation, segment.from, segment.to, jsonBytes(conversationSummarySourceRefs(messages)), conversationSummarySourceDigest(messages), segment.status, day, messages[0].CreatedAt, messages[len(messages)-1].CreatedAt)
				if err != nil {
					t.Fatal(err)
				}
			}
			app := &App{DB: repo}
			command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ConversationID: conversation, ActorRefs: []string{}, EventRefs: []string{}, EvidenceRefs: []string{"message:summary-message-1"}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "summary-message-1", SemanticReason: "source of old conclusion", IdempotencyKey: "daily-correction-memory", Semantic: &MemorySemanticInput{Type: "semantic", Content: "旧结论", Confidence: 0.9, Importance: 0.8}}
			command.RequestDigest = memoryCommandDigest(command)
			original := applyMemoryLifecycleTestCommand(t, ctx, app, command)
			correct := func() {
				t.Helper()
				revision := original.Revision
				if _, err := app.ReviseMemory(ctx, owner, original.MemoryID, "更正后的结论", &revision, []any{"owner-confirmation"}); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			app.Provider = &ProviderClient{DB: repo, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				body, _ := io.ReadAll(request.Body)
				if !strings.Contains(string(body), "daily-corrected") || !strings.Contains(string(body), "daily-clean") {
					t.Fatalf("daily request did not contain both segments")
				}
				if duringModel {
					correct()
				}
				response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(map[string]any{"schema_version": conversationDailyMemorySchemaVersion, "episode": "旧日记忆"})}}}}
				return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
			})}}
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
			if !duringModel {
				correct()
			}
			result, err := app.ProcessConversationDailyMemoryIntent(ctx, intentID)
			if err != nil || result["status"] != "blocked" || result["reason"] != "corrected_memory_source" {
				t.Fatalf("daily correction result=%#v err=%v", result, err)
			}
			wantCalls := 0
			if duringModel {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("model calls=%d want=%d", calls, wantCalls)
			}
			var status, lastError, segmentStatuses string
			var dailyCount int
			if err := repo.Pool().QueryRow(ctx, `SELECT status,COALESCE(last_error,'') FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status, &lastError); err != nil {
				t.Fatal(err)
			}
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_daily_memories`).Scan(&dailyCount); err != nil {
				t.Fatal(err)
			}
			if err := repo.Pool().QueryRow(ctx, `SELECT string_agg(status,',' ORDER BY id) FROM public.conversation_summaries`).Scan(&segmentStatuses); err != nil {
				t.Fatal(err)
			}
			if status != "completed" || lastError != errConversationDailyCorrectedSource.Error() || dailyCount != 0 || segmentStatuses != "active,consolidated" {
				t.Fatalf("blocked state status=%s error=%s daily=%d segments=%s", status, lastError, dailyCount, segmentStatuses)
			}
			replay, err := app.ProcessConversationDailyMemoryIntent(ctx, intentID)
			if err != nil || replay["status"] != "blocked" || replay["replayed"] != true || calls != wantCalls {
				t.Fatalf("blocked replay=%#v calls=%d err=%v", replay, calls, err)
			}
			enqueue()
			var generation int
			if err := repo.Pool().QueryRow(ctx, `SELECT (payload->>'source_generation')::integer FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&generation); err != nil || generation != 1 {
				t.Fatalf("reopened source generation=%d err=%v", generation, err)
			}
			reopened, err := app.ProcessConversationDailyMemoryIntent(ctx, intentID)
			if err != nil || reopened["status"] != "blocked" || calls != wantCalls {
				t.Fatalf("reopened blocked intent=%#v calls=%d err=%v", reopened, calls, err)
			}
		})
	}
}

func TestDailyMemoryQuietResultSettlesPendingIntent(t *testing.T) {
	for _, inactive := range []bool{false, true} {
		name := "no_source"
		if inactive {
			name = "inactive"
		}
		t.Run(name, func(t *testing.T) {
			ctx, repo, _, fid, conversation := seedConversationSummaryAuthority(t)
			app := &App{DB: repo}
			day := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")
			if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
				return app.enqueueConversationDailyMemoryTx(ctx, tx, fid, conversation, day, "UTC")
			}); err != nil {
				t.Fatal(err)
			}
			var intent string
			if err := repo.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.daily_memory'`).Scan(&intent); err != nil {
				t.Fatal(err)
			}
			if inactive {
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlights SET status='paused' WHERE id=$1`, fid); err != nil {
					t.Fatal(err)
				}
			}
			result, err := app.ProcessConversationDailyMemoryIntent(ctx, intent)
			if err != nil || result["status"] != name {
				t.Fatalf("quiet result=%#v err=%v", result, err)
			}
			var status string
			if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status); err != nil || status != "completed" {
				t.Fatalf("quiet result was not settled: status=%s err=%v", status, err)
			}
		})
	}
}
