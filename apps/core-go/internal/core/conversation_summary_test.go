package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func conversationSummaryTestMessages(count int, text string) []ConversationSummarySourceMessage {
	result := make([]ConversationSummarySourceMessage, 0, count)
	for sequence := 1; sequence <= count; sequence++ {
		kind := "user"
		if sequence%2 == 0 {
			kind = "assistant"
		}
		result = append(result, ConversationSummarySourceMessage{
			ID: "message-" + numberString(sequence, 0), Sequence: sequence, AuthorActorID: kind + "-actor", Kind: kind,
			Text: text, AttachmentRefs: []any{}, CreatedAt: time.Date(2026, 9, 11, 0, sequence, 0, 0, time.UTC),
		})
	}
	return result
}

func TestConversationSummaryChunkRequiresThresholdAndEndsAtCompletedTurn(t *testing.T) {
	if chunk, ready := selectConversationSummaryChunk(conversationSummaryTestMessages(39, "短消息")); ready || len(chunk) != 0 {
		t.Fatalf("short chunk unexpectedly ready: ready=%t len=%d", ready, len(chunk))
	}
	chunk, ready := selectConversationSummaryChunk(conversationSummaryTestMessages(40, "短消息"))
	if !ready || len(chunk) != 40 || chunk[len(chunk)-1].Kind != "assistant" {
		t.Fatalf("40-message chunk = ready=%t len=%d last=%#v", ready, len(chunk), chunk[len(chunk)-1])
	}
	large := conversationSummaryTestMessages(4, strings.Repeat("长", conversationSummaryTargetTokens*2))
	chunk, ready = selectConversationSummaryChunk(large)
	if !ready || len(chunk) != 2 || chunk[len(chunk)-1].Kind != "assistant" {
		t.Fatalf("token-threshold chunk = ready=%t len=%d", ready, len(chunk))
	}
}

func TestConversationSummaryProviderInputUsesOnlyRawBoundedMessages(t *testing.T) {
	messages := conversationSummaryTestMessages(4, "原始消息")
	provider := conversationSummaryProviderMessages(messages)
	encoded := jsonString(provider)
	for _, forbidden := range []string{"message-1", "user-actor", "assistant-actor", "source_digest", "previous_summary", "request_digest"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("summary Provider input leaked or recursed through %q: %s", forbidden, encoded)
		}
	}
	for _, required := range []string{"原始消息", "user", "assistant", "occurred_at", "attachment_count"} {
		if !strings.Contains(encoded, required) {
			t.Fatalf("summary Provider input lost %q: %s", required, encoded)
		}
	}
}

func TestConversationSummarySourceRefsPreserveSequenceOrder(t *testing.T) {
	raw := []any{"message:summary-message-1", "message:summary-message-2", "message:summary-message-10"}
	got := conversationSummarySourceRefValues(raw)
	want := []string{"message:summary-message-1", "message:summary-message-2", "message:summary-message-10"}
	if !sameStringSlice(got, want) {
		t.Fatalf("ordered source refs=%#v want=%#v", got, want)
	}
}

func TestConversationSummaryResponseIsStrictAndBounded(t *testing.T) {
	valid, err := decodeConversationSummaryProviderResponse(map[string]any{"schema_version": conversationSummarySchemaVersion, "summary": "  保留事实与约定  "})
	if err != nil || valid.Summary != "保留事实与约定" {
		t.Fatalf("valid response = %#v err=%v", valid, err)
	}
	for name, value := range map[string]map[string]any{
		"wrong version": {"schema_version": "conversation-summary.v0", "summary": "summary"},
		"blank":         {"schema_version": conversationSummarySchemaVersion, "summary": "   "},
		"too long":      {"schema_version": conversationSummarySchemaVersion, "summary": strings.Repeat("界", conversationSummaryMaxRunes+1)},
		"unknown field": {"schema_version": conversationSummarySchemaVersion, "summary": "summary", "conversation_id": "internal"},
	} {
		if _, err := decodeConversationSummaryProviderResponse(value); err == nil {
			t.Fatalf("%s response was accepted", name)
		}
	}
}

func TestConversationSummaryStaticGuardKeepsProjectionWritesInOneModule(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "conversation_summary.go" {
			continue
		}
		content, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		if strings.Contains(text, "INSERT INTO public.conversation_summaries") || strings.Contains(text, "UPDATE public.conversation_summaries SET") {
			t.Fatalf("Conversation Summary projection write leaked into %s", name)
		}
	}
}

func seedConversationSummaryMessages(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID string, from, to int) {
	t.Helper()
	for sequence := from; sequence <= to; sequence++ {
		kind, author := "user", ownerID
		if sequence%2 == 0 {
			kind, author = "assistant", fluctlightID
		}
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,'[]',$7)`, "summary-message-"+numberString(sequence, 0), conversationID, sequence, author, kind, "raw-summary-message-"+numberString(sequence, 0), "summary-message:"+numberString(sequence, 0)); err != nil {
			t.Fatal(err)
		}
	}
}

func seedConversationSummaryAuthority(t *testing.T) (context.Context, *PostgresRepository, string, string, string) {
	t.Helper()
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "summary-owner", "summary-fluctlight", "summary-conversation"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'summary')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	return ctx, repository, ownerID, fluctlightID, conversationID
}

// PostgreSQL integration gate: intentionally deferred to S12 by implement.md.
func TestPostgresConversationSummaryIntentIsStableChunkedAndRawPreserving(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 1, 64)
	app := &App{DB: repository}
	for attempt := 0; attempt < 2; attempt++ {
		if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
			return app.enqueueConversationSummaryIntentTx(ctx, tx, fluctlightID, conversationID, "summary-message-64")
		}); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	var payloadRaw []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='conversation.summary'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("summary intent count=%d err=%v", count, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_type='conversation.summary'`).Scan(&payloadRaw); err != nil {
		t.Fatal(err)
	}
	payload := decodeObject(payloadRaw)
	if intValue(payload["from_sequence"]) != 1 || intValue(payload["to_sequence"]) != 40 || intValue(payload["source_sequence"]) != 64 || len(arrayValue(payload["source_message_refs"])) != 40 {
		t.Fatalf("summary intent payload = %#v", payload)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('summary-chunk-provider','openai_compatible','http://summary.invalid','summary-secret','ready',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key) VALUES('summary-chunk-1',$1,$2,1,40,$3,$4,'第一段摘要','active',1,'summary-chunk-provider','summary-model','summary-request-1',$5,$6,$7,'summary-digest-1','summary-idempotency-1')`, fluctlightID, conversationID, jsonBytes(conversationSummarySourceRefValues(payload["source_message_refs"])), stringValue(payload["source_digest"]), conversationSummaryPromptVersion, conversationSummarySchemaVersion, conversationSummaryPolicyVersion); err != nil {
		t.Fatal(err)
	}
	dropped, err := app.retrieveConversationSummaries(ctx, ConversationSummaryQuery{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, Limit: 20, MaxRunes: 2})
	if err != nil || len(dropped.Items) != 0 || len(dropped.Trace.Selections) != 1 || dropped.Trace.Selections[0].Reason != "summary_budget" {
		t.Fatalf("summary budget drop=%#v err=%v", dropped, err)
	}
	selected, err := app.retrieveConversationSummaries(ctx, ConversationSummaryQuery{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, Limit: 20, MaxRunes: 100})
	if err != nil || len(selected.Items) != 1 || !strings.HasPrefix(stringValue(selected.Items[0]["ref"]), "summary:ctx_") || strings.Contains(jsonString(selected.Items), "summary-chunk-1") {
		t.Fatalf("summary selection=%#v err=%v", selected, err)
	}
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 65, 104)
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueConversationSummaryIntentTx(ctx, tx, fluctlightID, conversationID, "summary-message-104")
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='conversation.summary'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("multi-chunk intent count=%d err=%v", count, err)
	}
	var secondPayloadRaw []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_type='conversation.summary' ORDER BY (payload->>'from_sequence')::integer DESC LIMIT 1`).Scan(&secondPayloadRaw); err != nil {
		t.Fatal(err)
	}
	secondPayload := decodeObject(secondPayloadRaw)
	if intValue(secondPayload["from_sequence"]) != 41 || intValue(secondPayload["to_sequence"]) != 80 || intValue(secondPayload["source_sequence"]) != 104 {
		t.Fatalf("second summary intent payload = %#v", secondPayload)
	}
	var rawCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1`, conversationID).Scan(&rawCount); err != nil || rawCount != 104 {
		t.Fatalf("Raw messages changed: count=%d err=%v", rawCount, err)
	}
}

// PostgreSQL integration gate: intentionally deferred to S12 by implement.md.
func TestPostgresConversationSummaryProviderFailureRetryAndCommittedReplay(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 1, 64)
	endpointID := "summary-provider"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://summary.invalid','summary-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection',$1,'summary-model','structured_output',4096,10,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempt := calls.Add(1)
		body, _ := io.ReadAll(request.Body)
		if strings.Contains(string(body), "raw-summary-message-41") || !strings.Contains(string(body), "raw-summary-message-40") {
			return nil, errors.New("conversation summary source bound violated")
		}
		if attempt == 1 {
			return embeddingHTTPResponse(request, http.StatusInternalServerError, `{"error":"provider unavailable"}`), nil
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(map[string]any{"schema_version": conversationSummarySchemaVersion, "summary": "用户与摇光讨论了前二十个稳定回合。"})}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: providerHTTP}}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueConversationSummaryIntentTx(ctx, tx, fluctlightID, conversationID, "summary-message-64")
	}); err != nil {
		t.Fatal(err)
	}
	var intentID string
	var payloadRaw []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT intent_id,payload FROM public.platform_workflow_intents WHERE intent_type='conversation.summary'`).Scan(&intentID, &payloadRaw); err != nil {
		t.Fatal(err)
	}
	payload := decodeObject(payloadRaw)
	process := func() (map[string]any, error) {
		return app.ProcessConversationSummaryIntent(ctx, intentID, fluctlightID, conversationID, stringValue(payload["source_message_id"]), intValue(payload["source_sequence"]), intValue(payload["from_sequence"]), intValue(payload["to_sequence"]), stringValue(payload["source_digest"]), conversationSummarySourceRefValues(payload["source_message_refs"]))
	}
	_, err := process()
	if err == nil {
		t.Fatal("Provider failure unexpectedly produced a summary")
	}
	if calls.Load() != 1 {
		t.Fatalf("Provider calls = %d", calls.Load())
	}
	var summaries int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_summaries WHERE conversation_id=$1`, conversationID).Scan(&summaries); err != nil || summaries != 0 {
		t.Fatalf("Provider failure changed projection: count=%d err=%v", summaries, err)
	}
	result, err := process()
	if err != nil || stringValue(result["status"]) != "active" || boolValue(result["replayed"]) {
		t.Fatalf("retry result=%#v err=%v", result, err)
	}
	replayed, err := process()
	if err != nil || !boolValue(replayed["replayed"]) {
		t.Fatalf("committed replay=%#v err=%v", replayed, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("committed replay called Provider again: calls=%d", calls.Load())
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_summaries WHERE conversation_id=$1 AND status='active'`, conversationID).Scan(&summaries); err != nil || summaries != 1 {
		t.Fatalf("summary projection count=%d err=%v", summaries, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='drifted source' WHERE conversation_id=$1 AND sequence=1`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := process(); err == nil || !strings.Contains(err.Error(), "source_drift") {
		t.Fatalf("source drift error=%v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("source drift reached Provider: calls=%d", calls.Load())
	}
}

// PostgreSQL integration gate: intentionally deferred to S12 by implement.md.
func TestPostgresConversationSummaryRebuildSupersedesAndOverlapCASFailsClosed(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 1, 64)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('summary-rebuild-provider','openai_compatible','http://summary.invalid','summary-secret','ready',now())`); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueConversationSummaryIntentTx(ctx, tx, fluctlightID, conversationID, "summary-message-64")
	}); err != nil {
		t.Fatal(err)
	}
	var intentID string
	var payloadRaw []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT intent_id,payload FROM public.platform_workflow_intents WHERE intent_type='conversation.summary'`).Scan(&intentID, &payloadRaw); err != nil {
		t.Fatal(err)
	}
	payload := decodeObject(payloadRaw)
	work := conversationSummaryWork{
		IntentID: intentID, FluctlightID: fluctlightID, ConversationID: conversationID,
		SourceMessageID: stringValue(payload["source_message_id"]), SourceSequence: intValue(payload["source_sequence"]),
		FromSequence: intValue(payload["from_sequence"]), ToSequence: intValue(payload["to_sequence"]),
		SourceDigest: stringValue(payload["source_digest"]), SourceMessageRefs: conversationSummarySourceRefValues(payload["source_message_refs"]),
	}
	messages, err := readConversationSummaryMessages(ctx, repository.Pool(), conversationID, work.FromSequence, work.ToSequence, conversationSummaryMaxMessages)
	if err != nil {
		t.Fatal(err)
	}
	work.Messages = messages
	assignment := providerAssignment{EndpointID: "summary-rebuild-provider", ModelID: "summary-model"}
	first, err := app.settleConversationSummary(ctx, work, conversationSummaryProviderResponse{SchemaVersion: conversationSummarySchemaVersion, Summary: "第一版摘要"}, assignment, "provider:summary-rebuild-1", "request-digest-1")
	if err != nil || intValue(first["revision"]) != 1 {
		t.Fatalf("first summary=%#v err=%v", first, err)
	}
	second, err := app.settleConversationSummary(ctx, work, conversationSummaryProviderResponse{SchemaVersion: conversationSummarySchemaVersion, Summary: "重建后的摘要"}, assignment, "provider:summary-rebuild-2", "request-digest-2")
	if err != nil || intValue(second["revision"]) != 2 {
		t.Fatalf("rebuilt summary=%#v err=%v", second, err)
	}
	var active, superseded int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='active'),count(*) FILTER (WHERE status='superseded') FROM public.conversation_summaries WHERE conversation_id=$1`, conversationID).Scan(&active, &superseded); err != nil || active != 1 || superseded != 1 {
		t.Fatalf("rebuild lineage active=%d superseded=%d err=%v", active, superseded, err)
	}
	overlap := work
	overlap.FromSequence = 21
	overlap.Messages, err = readConversationSummaryMessages(ctx, repository.Pool(), conversationID, 21, 40, conversationSummaryMaxMessages)
	if err != nil {
		t.Fatal(err)
	}
	overlap.SourceMessageRefs = conversationSummarySourceRefs(overlap.Messages)
	overlap.SourceDigest = conversationSummarySourceDigest(overlap.Messages)
	if _, err := app.settleConversationSummary(ctx, overlap, conversationSummaryProviderResponse{SchemaVersion: conversationSummarySchemaVersion, Summary: "重叠摘要"}, assignment, "provider:overlap", "request-digest-overlap"); err == nil || !strings.Contains(err.Error(), "projection_stale") {
		t.Fatalf("overlap CAS error=%v", err)
	}
}
