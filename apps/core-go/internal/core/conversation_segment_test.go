package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func segmentIntentRows(t *testing.T, ctx context.Context, repository *PostgresRepository) []map[string]any {
	t.Helper()
	rows, err := repository.Pool().Query(ctx, `SELECT status,payload,next_attempt_at FROM public.platform_workflow_intents WHERE intent_type='conversation.segment' ORDER BY created_at,intent_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var status string
		var raw []byte
		var due time.Time
		if err := rows.Scan(&status, &raw, &due); err != nil {
			t.Fatal(err)
		}
		item := decodeObject(raw)
		item["status"] = status
		item["due"] = due
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestConversationSegmentToDailyMemoryAndRawCorrection(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 1, 4)
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	yesterday := time.Now().In(zone).AddDate(0, 0, -2)
	base := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 12, 0, 0, 0, zone).UTC()
	for sequence := 1; sequence <= 4; sequence++ {
		if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET created_at=$2 WHERE id=$1`, "summary-message-"+numberString(sequence, 0), base.Add(time.Duration(sequence)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('segment-provider','openai_compatible','http://segment.invalid','segment-secret','ready',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection','segment-provider','segment-model','structured_output',4096,10,'{}')`); err != nil {
		t.Fatal(err)
	}
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		value := map[string]any{"schema_version": conversationSegmentSchemaVersion, "summary": "这段讨论了一个尚未完成的计划。", "ending_state": "约定稍后再谈", "open_threads": []any{"计划待确认"}, "core_events": []any{"用户提出计划"}}
		if strings.Contains(string(body), "conversation_daily_memory_v1") {
			if !strings.Contains(string(body), "计划待确认") || !strings.Contains(string(body), "用户提出计划") {
				return nil, errors.New("daily model lost segment open threads or core events")
			}
			value = map[string]any{"schema_version": conversationDailyMemorySchemaVersion, "episode": "当天与用户讨论计划，结束时仍待确认。"}
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(value)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: providerHTTP}}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueConversationSegmentTx(ctx, tx, fluctlightID, conversationID, false)
	}); err != nil {
		t.Fatal(err)
	}
	var segmentIntentID string
	if err := repository.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.segment' AND status='pending'`).Scan(&segmentIntentID); err != nil {
		t.Fatal(err)
	}
	segment, err := app.ProcessConversationSegmentIntent(ctx, segmentIntentID)
	if err != nil || segment["status"] != "active" {
		t.Fatalf("segment=%#v err=%v", segment, err)
	}
	var summaryID, dailyIntentID, localDate string
	var started, ended time.Time
	if err := repository.Pool().QueryRow(ctx, `SELECT id,local_date::text,started_at,ended_at FROM public.conversation_summaries WHERE status='active'`).Scan(&summaryID, &localDate, &started, &ended); err != nil {
		t.Fatal(err)
	}
	if !started.Equal(base.Add(time.Minute)) || !ended.Equal(base.Add(4*time.Minute)) || localDate != base.In(zone).Format("2006-01-02") {
		t.Fatalf("source time/date=%s %s %s", started, ended, localDate)
	}
	legacyMessages, err := readConversationSummaryMessages(ctx, repository.Pool(), conversationID, 1, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	legacyWork := conversationSummaryWork{IntentID: "legacy-replay", FluctlightID: fluctlightID, ConversationID: conversationID, SourceMessageID: "summary-message-4", SourceSequence: 4, FromSequence: 1, ToSequence: 4, SourceDigest: conversationSummarySourceDigest(legacyMessages), SourceMessageRefs: conversationSummarySourceRefs(legacyMessages), Messages: legacyMessages}
	assignment, err := app.Provider.assignment(ctx, "reflection")
	if err != nil {
		t.Fatal(err)
	}
	legacyReplay, err := app.settleConversationSummary(ctx, legacyWork, conversationSummaryProviderResponse{SchemaVersion: conversationSummarySchemaVersion, Summary: "过时固定块"}, assignment, "legacy-provider-request", "legacy-request-digest")
	if err != nil || legacyReplay["status"] != "superseded" {
		t.Fatalf("legacy replay overwrote active segment=%#v err=%v", legacyReplay, err)
	}
	retrieved, err := app.retrieveConversationSummaries(ctx, ConversationSummaryQuery{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, Limit: 20, MaxRunes: 4000})
	var retrievedStart, retrievedEnd time.Time
	if err == nil && len(retrieved.Items) == 1 {
		retrievedStart, _ = time.Parse(time.RFC3339Nano, stringValue(retrieved.Items[0]["started_at"]))
		retrievedEnd, _ = time.Parse(time.RFC3339Nano, stringValue(retrieved.Items[0]["ended_at"]))
	}
	if err != nil || len(retrieved.Items) != 1 || !retrievedStart.Equal(started) || !retrievedEnd.Equal(ended) {
		t.Fatalf("source-bounded prompt summary=%#v err=%v", retrieved, err)
	}
	compact := compactSummaryForSurface(retrieved.Items[0], ProviderContextSurfaceConversationMain)
	if compact["started_at"] == nil || compact["ended_at"] == nil || compact["ending_state"] == nil || compact["completed_at"] != nil || compact["to_sequence"] != nil || compact["time_semantics"] != "historical_conversation" {
		t.Fatalf("summary prompt repeated storage metadata: %#v", compact)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.daily_memory'`).Scan(&dailyIntentID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `CREATE FUNCTION public.test_fail_daily_link() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected_daily_link_failure'; END $$;
		CREATE TRIGGER test_fail_daily_link BEFORE INSERT ON public.memory_source_links FOR EACH ROW EXECUTE FUNCTION public.test_fail_daily_link()`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessConversationDailyMemoryIntent(ctx, dailyIntentID); err == nil || !strings.Contains(err.Error(), "injected_daily_link_failure") {
		t.Fatalf("daily link failure was not surfaced: %v", err)
	}
	var activeCount, memoryCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_summaries WHERE status='active'`).Scan(&activeCount); err != nil || activeCount != 1 {
		t.Fatalf("failed daily retired segment=%d err=%v", activeCount, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memories`).Scan(&memoryCount); err != nil || memoryCount != 0 {
		t.Fatalf("failed daily left partial memory=%d err=%v", memoryCount, err)
	}
	if _, err := repository.Pool().Exec(ctx, `DROP TRIGGER test_fail_daily_link ON public.memory_source_links; DROP FUNCTION public.test_fail_daily_link()`); err != nil {
		t.Fatal(err)
	}
	daily, err := app.ProcessConversationDailyMemoryIntent(ctx, dailyIntentID)
	if err != nil || daily["status"] != "completed" {
		t.Fatalf("daily=%#v err=%v", daily, err)
	}
	var memoryID, summaryStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT memory_id FROM public.conversation_daily_memories WHERE conversation_id=$1`, conversationID).Scan(&memoryID); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.conversation_summaries WHERE id=$1`, summaryID).Scan(&summaryStatus); err != nil || summaryStatus != "consolidated" {
		t.Fatalf("summary status=%s err=%v", summaryStatus, err)
	}
	legacyReplay, err = app.settleConversationSummary(ctx, legacyWork, conversationSummaryProviderResponse{SchemaVersion: conversationSummarySchemaVersion, Summary: "过时固定块"}, assignment, "legacy-provider-request", "legacy-request-digest")
	if err != nil || legacyReplay["status"] != "superseded" {
		t.Fatalf("legacy replay overwrote daily memory=%#v err=%v", legacyReplay, err)
	}
	retrieved, err = app.retrieveConversationSummaries(ctx, ConversationSummaryQuery{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, Limit: 20, MaxRunes: 4000})
	if err != nil || len(retrieved.Items) != 0 {
		t.Fatalf("consolidated segment still active=%#v err=%v", retrieved, err)
	}
	projection := ContextProjection{ConversationID: conversationID, FluctlightID: fluctlightID,
		RecentMessages: []map[string]any{{"id": "summary-message-1"}, {"id": "summary-message-2"}},
		Memories:       []map[string]any{{"ref": "memory:ctx_daily", "effective_source_refs": []any{"conversation_summary:" + summaryID}}}}
	working := WorkingMemory{Retrieved: []PromptFragment{{Kind: PromptFragmentRetrievedMemory, Content: map[string]any{"ref": "memory:ctx_daily", "content": "当天与用户讨论计划"}, SourceRefs: []string{"memory:ctx_daily"}}}}
	if err := app.attachDailyMemoryMessageRefs(ctx, projection, &working); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonString(working.Retrieved[0].SourceRefs), "message:summary-message-1") {
		t.Fatalf("daily memory lost raw coverage refs: %#v", working.Retrieved[0].SourceRefs)
	}
	var live bool
	if err := repository.Pool().QueryRow(ctx, `SELECT public.memory_source_is_live(source_kind,source_id,source_revision,source_fingerprint) FROM public.memory_source_links WHERE memory_id=$1 AND memory_revision=0`, memoryID).Scan(&live); err != nil || !live {
		t.Fatalf("daily provenance live=%v err=%v", live, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='用户更正了计划' WHERE id='summary-message-1'`); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.conversation_summaries WHERE id=$1`, summaryID).Scan(&summaryStatus); err != nil || summaryStatus != "invalidated" {
		t.Fatalf("corrected summary status=%s err=%v", summaryStatus, err)
	}
	var provenanceStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT provenance_status FROM public.memories WHERE id=$1`, memoryID).Scan(&provenanceStatus); err != nil || provenanceStatus != "invalid" {
		t.Fatalf("corrected daily provenance=%s err=%v", provenanceStatus, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT public.memory_source_is_live(source_kind,source_id,source_revision,source_fingerprint) FROM public.memory_source_links WHERE memory_id=$1 AND memory_revision=0`, memoryID).Scan(&live); err != nil || live {
		t.Fatalf("stale daily provenance live=%v err=%v", live, err)
	}
	if repaired, err := app.EnsureConversationSegmentIntents(ctx, 100); err != nil || repaired != 1 {
		t.Fatalf("segment repair=%d err=%v", repaired, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.segment' AND status='pending'`).Scan(&segmentIntentID); err != nil {
		t.Fatal(err)
	}
	segment, err = app.ProcessConversationSegmentIntent(ctx, segmentIntentID)
	if err != nil || segment["status"] != "active" {
		t.Fatalf("corrected segment=%#v err=%v", segment, err)
	}
	daily, err = app.ProcessConversationDailyMemoryIntent(ctx, dailyIntentID)
	if err != nil || daily["status"] != "completed" {
		t.Fatalf("rebuilt daily=%#v err=%v", daily, err)
	}
	var revision int
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.memories WHERE id=$1`, memoryID).Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("daily revision=%d err=%v", revision, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT provenance_status FROM public.memories WHERE id=$1`, memoryID).Scan(&provenanceStatus); err != nil || provenanceStatus != "verified" {
		t.Fatalf("rebuilt daily provenance=%s err=%v", provenanceStatus, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT public.memory_source_is_live(source_kind,source_id,source_revision,source_fingerprint) FROM public.memory_source_links WHERE memory_id=$1 AND memory_revision=1`, memoryID).Scan(&live); err != nil || !live {
		t.Fatalf("rebuilt provenance live=%v err=%v", live, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='raw-summary-message-1' WHERE id='summary-message-1'`); err != nil {
		t.Fatal(err)
	}
	if repaired, err := app.EnsureConversationSegmentIntents(ctx, 100); err != nil || repaired != 1 {
		t.Fatalf("restored digest repair=%d err=%v", repaired, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.segment' AND status='pending'`).Scan(&segmentIntentID); err != nil {
		t.Fatal(err)
	}
	segment, err = app.ProcessConversationSegmentIntent(ctx, segmentIntentID)
	if err != nil || segment["status"] != "active" || segment["summary_id"] == summaryID {
		t.Fatalf("restored digest segment=%#v err=%v", segment, err)
	}
	daily, err = app.ProcessConversationDailyMemoryIntent(ctx, dailyIntentID)
	if err != nil || daily["status"] != "completed" {
		t.Fatalf("restored digest daily=%#v err=%v", daily, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.memories WHERE id=$1`, memoryID).Scan(&revision); err != nil || revision != 2 {
		t.Fatalf("restored daily revision=%d err=%v", revision, err)
	}
}

func TestConversationSegmentDebouncesToLatestUserAndSupersedesOlderSource(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 1, 4)
	t0 := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET created_at=$2 WHERE id=$1`, "summary-message-3", t0); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	enqueue := func() {
		t.Helper()
		if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
			return app.enqueueConversationSegmentTx(ctx, tx, fluctlightID, conversationID, false)
		}); err != nil {
			t.Fatal(err)
		}
	}
	enqueue()
	enqueue()
	rows := segmentIntentRows(t, ctx, repository)
	if len(rows) != 1 || rows[0]["status"] != "pending" || intValue(rows[0]["to_sequence"]) != 4 || !rows[0]["due"].(time.Time).Equal(t0.Add(10*time.Minute)) {
		t.Fatalf("initial segment intent = %#v", rows)
	}
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 5, 5)
	t1 := t0.Add(4 * time.Minute)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET created_at=$2 WHERE id=$1`, "summary-message-5", t1); err != nil {
		t.Fatal(err)
	}
	enqueue()
	rows = segmentIntentRows(t, ctx, repository)
	var pending int
	for _, row := range rows {
		if row["status"] == "pending" {
			pending++
			if row["idle_epoch"] != "summary-message-5" || intValue(row["to_sequence"]) != 4 || !row["due"].(time.Time).Equal(t1.Add(10*time.Minute)) {
				t.Fatalf("new epoch intent = %#v", row)
			}
		}
	}
	if pending != 1 {
		t.Fatalf("pending intents after new user = %#v", rows)
	}
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 6, 6)
	enqueue()
	rows = segmentIntentRows(t, ctx, repository)
	pending = 0
	for _, row := range rows {
		if row["status"] == "pending" {
			pending++
			if intValue(row["to_sequence"]) != 6 {
				t.Fatalf("latest complete turn not included: %#v", row)
			}
		}
	}
	if pending != 1 {
		t.Fatalf("duplicate active segment intents = %#v", rows)
	}
}

func TestConversationSegmentCutsLongMessagesAtCompletedTurn(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 1, 6)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text=$1 WHERE id='summary-message-1'`, strings.Repeat("长", conversationSummaryTargetTokens*2)); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueConversationSegmentTx(ctx, tx, fluctlightID, conversationID, false)
	}); err != nil {
		t.Fatal(err)
	}
	rows := segmentIntentRows(t, ctx, repository)
	if len(rows) != 1 || intValue(rows[0]["to_sequence"]) != 2 {
		t.Fatalf("long source was not cut at first complete turn: %#v", rows)
	}
}

func TestConversationSegmentKeepsCrossMidnightReplyWithUserStartDate(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 1, 4)
	times := []time.Time{
		time.Date(2026, 9, 27, 15, 58, 0, 0, time.UTC),
		time.Date(2026, 9, 27, 16, 2, 0, 0, time.UTC),
		time.Date(2026, 9, 27, 16, 3, 0, 0, time.UTC),
		time.Date(2026, 9, 27, 16, 4, 0, 0, time.UTC),
	}
	for index, at := range times {
		if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET created_at=$2 WHERE id=$1`, "summary-message-"+numberString(index+1, 0), at); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{DB: repository}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueConversationSegmentTx(ctx, tx, fluctlightID, conversationID, false)
	}); err != nil {
		t.Fatal(err)
	}
	rows := segmentIntentRows(t, ctx, repository)
	if len(rows) != 1 || rows[0]["local_date"] != "2026-09-27" || intValue(rows[0]["to_sequence"]) != 2 {
		t.Fatalf("cross-midnight turn split incorrectly: %#v", rows)
	}
}

func TestConversationSegmentCrossMidnightConsecutiveUsersRemainCoverable(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	for _, message := range []struct {
		id           string
		sequence     int
		author, kind string
		at           time.Time
	}{
		{"cross-user-1", 1, ownerID, "user", time.Date(2026, 9, 27, 15, 59, 0, 0, time.UTC)},
		{"cross-user-2", 2, ownerID, "user", time.Date(2026, 9, 27, 16, 1, 0, 0, time.UTC)},
		{"cross-assistant-3", 3, fluctlightID, "assistant", time.Date(2026, 9, 27, 16, 2, 0, 0, time.UTC)},
	} {
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,created_at) VALUES($1,$2,$3,$4,$5,$6,'[]',$1,$7)`, message.id, conversationID, message.sequence, message.author, message.kind, message.id, message.at); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{DB: repository}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueConversationSegmentTx(ctx, tx, fluctlightID, conversationID, false)
	}); err != nil {
		t.Fatal(err)
	}
	rows := segmentIntentRows(t, ctx, repository)
	if len(rows) != 1 || rows[0]["local_date"] != "2026-09-28" || intValue(rows[0]["from_sequence"]) != 1 || intValue(rows[0]["to_sequence"]) != 3 {
		t.Fatalf("consecutive cross-midnight users stranded: %#v", rows)
	}
}

func TestConversationSegmentSealsPreviousDayDuringContinuousChat(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(zone)
	yesterday := now.AddDate(0, 0, -1)
	old := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 55, 0, 0, zone).UTC()
	for _, message := range []struct {
		id           string
		sequence     int
		author, kind string
		at           time.Time
	}{
		{"roll-user-1", 1, ownerID, "user", old},
		{"roll-assistant-2", 2, fluctlightID, "assistant", old.Add(time.Minute)},
		{"roll-user-3", 3, ownerID, "user", time.Now().UTC()},
	} {
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,created_at) VALUES($1,$2,$3,$4,$5,$6,'[]',$1,$7)`, message.id, conversationID, message.sequence, message.author, message.kind, message.id, message.at); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{DB: repository}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueConversationSegmentTx(ctx, tx, fluctlightID, conversationID, false)
	}); err != nil {
		t.Fatal(err)
	}
	rows := segmentIntentRows(t, ctx, repository)
	if len(rows) != 1 || rows[0]["boundary_kind"] != "day_rollover" || intValue(rows[0]["to_sequence"]) != 2 || rows[0]["due"].(time.Time).After(time.Now().Add(5*time.Second)) {
		t.Fatalf("previous day not sealed promptly: %#v", rows)
	}
	var intentID string
	if err := repository.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.segment'`).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessConversationSegmentIntent(ctx, intentID); err == nil || !strings.Contains(err.Error(), "conversation_segment_provider_unavailable") {
		t.Fatalf("rollover incorrectly waited for idle: %v", err)
	}
}

func TestConversationDailyMemoryDayWindowHandlesDST(t *testing.T) {
	for _, test := range []struct{ name, date, expected string }{
		{"spring", "2026-03-08", "23h0m0s"},
		{"autumn", "2026-11-01", "25h0m0s"},
	} {
		t.Run(test.name, func(t *testing.T) {
			location, err := time.LoadLocation("America/New_York")
			if err != nil {
				t.Fatal(err)
			}
			day, err := time.ParseInLocation("2006-01-02", test.date, location)
			if err != nil {
				t.Fatal(err)
			}
			if got := day.AddDate(0, 0, 1).Sub(day).String(); got != test.expected {
				t.Fatalf("local day duration = %s", got)
			}
		})
	}
}

func TestConversationDailyIntentsFreezeSeparateTimezoneDayWindows(t *testing.T) {
	ctx, repository, _, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	app := &App{DB: repository}
	for _, zone := range []string{"America/New_York", "Asia/Tokyo"} {
		if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
			return app.enqueueConversationDailyMemoryTx(ctx, tx, fluctlightID, conversationID, "2026-03-08", zone)
		}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := repository.Pool().Query(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_type='conversation.daily_memory' ORDER BY intent_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string]time.Duration{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		payload := decodeObject(raw)
		start, err := time.Parse(time.RFC3339Nano, stringValue(payload["day_start_utc"]))
		if err != nil {
			t.Fatal(err)
		}
		end, err := time.Parse(time.RFC3339Nano, stringValue(payload["day_end_utc"]))
		if err != nil {
			t.Fatal(err)
		}
		seen[stringValue(payload["timezone"])] = end.Sub(start)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen["America/New_York"] != 23*time.Hour || seen["Asia/Tokyo"] != 24*time.Hour {
		t.Fatalf("frozen local day windows=%#v", seen)
	}
}

func TestLegacyCrossDayChunkRebuildsFromRawAndSameDayKeepsSourceTimes(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 1, 8)
	base := time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC)
	for sequence := 1; sequence <= 8; sequence++ {
		at := base.Add(time.Duration(sequence) * time.Minute)
		if sequence >= 5 {
			at = time.Date(2026, 9, 27, 15, 54, 0, 0, time.UTC).Add(time.Duration(sequence) * time.Minute)
		}
		if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET created_at=$2 WHERE id=$1`, "summary-message-"+numberString(sequence, 0), at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('legacy-segment-provider','openai_compatible','http://legacy.invalid','legacy-secret','ready',now())`); err != nil {
		t.Fatal(err)
	}
	first, err := readConversationSummaryMessages(ctx, repository.Pool(), conversationID, 1, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	second, err := readConversationSummaryMessages(ctx, repository.Pool(), conversationID, 5, 8, 4)
	if err != nil {
		t.Fatal(err)
	}
	for index, source := range [][]ConversationSummarySourceMessage{first, second} {
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key)
		VALUES($1,$2,$3,$4,$5,$6,$7,'旧摘要','active',1,'legacy-segment-provider','legacy-model',$8,$9,$10,$11,$12,$13)`, "legacy-summary-"+numberString(index, 0), fluctlightID, conversationID, source[0].Sequence, source[len(source)-1].Sequence, jsonBytes(conversationSummarySourceRefs(source)), conversationSummarySourceDigest(source), "legacy-request-"+numberString(index, 0), conversationSummaryPromptVersion, conversationSummarySchemaVersion, conversationSummaryPolicyVersion, "legacy-digest-"+numberString(index, 0), "legacy-idempotency-"+numberString(index, 0)); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{DB: repository}
	if count, err := app.ReconcileLegacyConversationSummaries(ctx, 10); err != nil || count != 2 {
		t.Fatalf("legacy backfill=%d err=%v", count, err)
	}
	var status, localDate string
	var started, ended time.Time
	if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(local_date::text,''),started_at,ended_at FROM public.conversation_summaries WHERE id='legacy-summary-0'`).Scan(&status, &localDate, &started, &ended); err != nil {
		t.Fatal(err)
	}
	if status != "active" || localDate != first[0].CreatedAt.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02") || !started.Equal(first[0].CreatedAt) || !ended.Equal(first[len(first)-1].CreatedAt) {
		t.Fatalf("legacy same-day metadata=%s %s %s %s", status, localDate, started, ended)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.conversation_summaries WHERE id='legacy-summary-1'`).Scan(&status); err != nil || status != "invalidated" {
		t.Fatalf("cross-day legacy status=%s err=%v", status, err)
	}
	rows := segmentIntentRows(t, ctx, repository)
	if len(rows) != 1 || intValue(rows[0]["from_sequence"]) != 5 || intValue(rows[0]["to_sequence"]) != 6 {
		t.Fatalf("legacy rebuilt segment=%#v", rows)
	}
}

func TestLongIdleConversationDrainsAllSegmentsIntoOneDayMemory(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 1, 140)
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	oldDay := time.Now().In(zone).AddDate(0, 0, -2)
	base := time.Date(oldDay.Year(), oldDay.Month(), oldDay.Day(), 12, 0, 0, 0, zone).UTC()
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET created_at=$2::timestamptz + sequence * interval '1 minute' WHERE conversation_id=$1`, conversationID, base); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('long-segment-provider','openai_compatible','http://segment.invalid','segment-secret','ready',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection','long-segment-provider','segment-model','structured_output',4096,10,'{}')`); err != nil {
		t.Fatal(err)
	}
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		value := map[string]any{"schema_version": conversationSegmentSchemaVersion, "summary": "本阶段继续讨论同一主题。", "ending_state": "暂告一段落", "open_threads": []any{}, "core_events": []any{}}
		if strings.Contains(string(body), "conversation_daily_memory_v1") {
			value = map[string]any{"schema_version": conversationDailyMemorySchemaVersion, "episode": "当天长谈同一主题，结束时暂告一段落。"}
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(value)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: providerHTTP}}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueConversationSegmentTx(ctx, tx, fluctlightID, conversationID, false)
	}); err != nil {
		t.Fatal(err)
	}
	for cycle := 0; cycle < 4; cycle++ {
		var intentID string
		if err := repository.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.segment' AND status='pending' ORDER BY created_at LIMIT 1`).Scan(&intentID); err != nil {
			t.Fatalf("pending segment %d: %v", cycle, err)
		}
		if result, err := app.ProcessConversationSegmentIntent(ctx, intentID); err != nil || result["status"] != "active" {
			t.Fatalf("segment %d=%#v err=%v", cycle, result, err)
		}
	}
	var count, covered int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*),MAX(to_sequence) FROM public.conversation_summaries WHERE conversation_id=$1 AND status='active'`, conversationID).Scan(&count, &covered); err != nil || count != 4 || covered != 140 {
		t.Fatalf("segment coverage=%d to=%d err=%v", count, covered, err)
	}
	var dailyIntentID string
	if err := repository.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='conversation.daily_memory'`).Scan(&dailyIntentID); err != nil {
		t.Fatal(err)
	}
	if result, err := app.ProcessConversationDailyMemoryIntent(ctx, dailyIntentID); err != nil || result["status"] != "completed" || intValue(result["source_count"]) != 4 {
		t.Fatalf("daily=%#v err=%v", result, err)
	}
	var memoryID string
	if err := repository.Pool().QueryRow(ctx, `SELECT memory_id FROM public.conversation_daily_memories WHERE conversation_id=$1`, conversationID).Scan(&memoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='更正的原文' WHERE id='summary-message-1'`); err != nil {
		t.Fatal(err)
	}
	var liveCount, totalCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FILTER(WHERE public.memory_source_is_live(source_kind,source_id,source_revision,source_fingerprint)),count(*) FROM public.memory_source_links WHERE memory_id=$1 AND memory_revision=0`, memoryID).Scan(&liveCount, &totalCount); err != nil || liveCount != 3 || totalCount != 4 {
		t.Fatalf("partial daily sources=%d/%d err=%v", liveCount, totalCount, err)
	}
	valid, err := memorySourcesStillValid(ctx, repository.Pool(), memoryID, 0, "verified")
	if err != nil || valid {
		t.Fatalf("partial daily memory leaked as valid=%v err=%v", valid, err)
	}
}
