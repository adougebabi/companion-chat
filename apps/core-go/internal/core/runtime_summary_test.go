package core

import (
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRuntimeSummaryConcurrentNewMessagesCarryForwardAndStayBounded(t *testing.T) {
	ctx, repository, owner, fluctlight, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, owner, fluctlight, conversation, 1, 64)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('runtime-summary-provider','openai_compatible','http://summary.invalid','runtime-summary-secret','ready',now());INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection','runtime-summary-provider','summary-model','structured_output',4096,10,'{}')`); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	calls := 0
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(request.Body)
		if calls == 1 {
			if strings.Contains(string(body), "raw-summary-message-41") {
				t.Fatal("first range was not frozen")
			}
			seedConversationSummaryMessages(t, ctx, repository, owner, fluctlight, conversation, 65, 104)
		} else if !strings.Contains(string(body), "持续约定") {
			t.Fatal("previous cumulative summary missing")
		}
		response := fmt.Sprintf(`{"choices":[{"message":{"content":%q}}]}`, jsonString(map[string]any{"schema_version": conversationSummarySchemaVersion, "summary": "持续约定；用户纠正仍有效，购买尚未完成。"}))
		return embeddingHTTPResponse(request, http.StatusOK, response), nil
	})}
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: providerHTTP}, Clock: func() time.Time { return at }}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueRuntimeSummaryTx(ctx, tx, fluctlight, conversation, "summary-message-64")
	}); err != nil {
		t.Fatal(err)
	}
	processNext := func() {
		t.Helper()
		var id string
		var raw []byte
		if err := repository.Pool().QueryRow(ctx, `SELECT intent_id,payload FROM public.platform_workflow_intents WHERE intent_type='conversation.summary' AND status IN ('pending','retry') ORDER BY (payload->>'from_sequence')::integer DESC LIMIT 1`).Scan(&id, &raw); err != nil {
			t.Fatal(err)
		}
		payload := decodeObject(raw)
		result, err := app.ProcessConversationSummaryIntent(ctx, id, fluctlight, conversation, stringValue(payload["source_message_id"]), intValue(payload["source_sequence"]), intValue(payload["from_sequence"]), intValue(payload["to_sequence"]), stringValue(payload["source_digest"]), conversationSummarySourceRefValues(payload["source_message_refs"]))
		if err != nil || result["status"] != "active" {
			t.Fatalf("process %#v %v", result, err)
		}
		if _, err := repository.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed' WHERE intent_id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	processNext()
	state, err := readRuntimeSummaryState(ctx, repository.Pool(), fluctlight, conversation)
	if err != nil || state.Covered != 40 {
		t.Fatalf("first cursor %#v %v", state, err)
	}
	// The next intent must use source 104, not stale source 64.
	var source int
	if err := repository.Pool().QueryRow(ctx, `SELECT (payload->>'source_sequence')::integer FROM public.platform_workflow_intents WHERE status='pending' AND intent_type='conversation.summary' ORDER BY (payload->>'from_sequence')::integer DESC LIMIT 1`).Scan(&source); err != nil || source != 104 {
		t.Fatalf("new highwater lost: %d %v", source, err)
	}
	at = at.Add(6 * time.Minute)
	processNext()
	at = at.Add(6 * time.Minute)
	processNext()
	state, err = readRuntimeSummaryState(ctx, repository.Pool(), fluctlight, conversation)
	if err != nil || state.Covered != 100 || len([]rune(state.Summary)) > runtimeSummaryMaxRunes {
		t.Fatalf("final cursor %#v %v", state, err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.enqueueRuntimeSummaryTx(ctx, tx, fluctlight, conversation, "summary-message-104")
	}); err != nil {
		t.Fatal(err)
	}
	var pending int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='conversation.summary' AND status='pending'`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("no-new requeued %d %v", pending, err)
	}
}
func TestRequiredRuntimeSummaryReplacesCoveredRawMessages(t *testing.T) {
	projection := ContextProjection{RecentMessages: []map[string]any{{"id": "one", "sequence": 1, "kind": "user", "text": "被覆盖的旧问题", "turn_id": "old"}, {"id": "two", "sequence": 2, "kind": "assistant", "text": "被覆盖的旧回答", "turn_id": "old"}, {"id": "three", "sequence": 3, "kind": "user", "text": "新的问题", "turn_id": "new"}, {"id": "four", "sequence": 4, "kind": "assistant", "text": "新的回答", "turn_id": "new"}}}
	summaries := []map[string]any{{"summary": "持续约定和明确纠正", "from_sequence": 1, "to_sequence": 2, "required": true, "source_message_refs": []any{"message:one", "message:two"}}}
	input := workingMemoryInputFromProjectionForSurface(projection, ProviderContextSurfaceConversationMain, nil, summaries)
	memory, err := ResolveWorkingMemory(input, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	result, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", WorkingMemory: memory, CurrentInput: "当前请求", Policy: DefaultPromptBudgetPolicy(4096)})
	if err != nil {
		t.Fatal(err)
	}
	wire := jsonString(result.Messages)
	if strings.Contains(wire, "被覆盖的旧问题") || strings.Contains(wire, "被覆盖的旧回答") || !strings.Contains(wire, "持续约定") || !strings.Contains(wire, "新的问题") {
		t.Fatalf("summary did not replace history: %s", wire)
	}
}

func TestRuntimeSummaryFailureDeleteRebuildAndConfiguredOutputBudget(t *testing.T) {
	ctx, repo, owner, fl, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repo, owner, fl, conversation, 1, 64)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('recovery-summary-provider','openai_compatible','http://summary-recovery.invalid','summary-secret','ready',now());INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection','recovery-summary-provider','summary-model','structured_output',4096,10,'{}');INSERT INTO public.runtime_settings(key,value_json) VALUES('product.summary','{"interval_seconds":600,"max_runes":4096}')`); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	responseText := strings.Repeat("约定尚待实际完成。", 300)
	app := &App{DB: repo, Provider: &ProviderClient{DB: repo}, Clock: func() time.Time { return at }}
	app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"enable_thinking":false`) {
			t.Fatal("summary did not explicitly request no thinking")
		}
		if !strings.Contains(string(body), `"maxLength":4096`) {
			t.Fatal("summary schema ignored configured budget")
		}
		response := fmt.Sprintf(`{"choices":[{"message":{"content":%q}}]}`, jsonString(map[string]any{"schema_version": conversationSummarySchemaVersion, "summary": responseText}))
		return embeddingHTTPResponse(request, http.StatusOK, response), nil
	})}
	enqueue := func() {
		t.Helper()
		if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
			return app.enqueueRuntimeSummaryTx(ctx, tx, fl, conversation, "summary-message-64")
		}); err != nil {
			t.Fatal(err)
		}
	}
	next := func() conversationSummaryWork {
		t.Helper()
		var id string
		var raw []byte
		if err := repo.Pool().QueryRow(ctx, `SELECT intent_id,payload FROM public.platform_workflow_intents WHERE intent_type='conversation.summary' AND status='pending' ORDER BY created_at DESC,intent_id DESC LIMIT 1`).Scan(&id, &raw); err != nil {
			t.Fatal(err)
		}
		p := decodeObject(raw)
		return conversationSummaryWork{IntentID: id, FluctlightID: fl, ConversationID: conversation, SourceMessageID: stringValue(p["source_message_id"]), SourceSequence: intValue(p["source_sequence"]), FromSequence: intValue(p["from_sequence"]), ToSequence: intValue(p["to_sequence"]), SourceDigest: stringValue(p["source_digest"]), SourceMessageRefs: conversationSummarySourceRefValues(p["source_message_refs"])}
	}
	process := func(work conversationSummaryWork) (map[string]any, error) {
		return app.ProcessConversationSummaryIntent(ctx, work.IntentID, fl, conversation, work.SourceMessageID, work.SourceSequence, work.FromSequence, work.ToSequence, work.SourceDigest, work.SourceMessageRefs)
	}
	enqueue()
	work := next()
	if _, err := repo.Pool().Exec(ctx, `CREATE FUNCTION public.reject_summary_commit_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.covered_through>OLD.covered_through THEN RAISE EXCEPTION 'forced summary commit failure'; END IF; RETURN NEW; END $$;CREATE TRIGGER reject_summary_commit_test BEFORE UPDATE ON public.conversation_runtime_summaries FOR EACH ROW EXECUTE FUNCTION public.reject_summary_commit_test()`); err != nil {
		t.Fatal(err)
	}
	if _, err := process(work); err == nil {
		t.Fatal("forced transaction failure succeeded")
	}
	state, err := readRuntimeSummaryState(ctx, repo.Pool(), fl, conversation)
	if err != nil || state.Covered != 0 || state.Revision != 0 {
		t.Fatalf("failure advanced cursor %#v %v", state, err)
	}
	if _, err := repo.Pool().Exec(ctx, `DROP TRIGGER reject_summary_commit_test ON public.conversation_runtime_summaries`); err != nil {
		t.Fatal(err)
	}
	if result, err := process(work); err != nil || result["status"] != "active" {
		t.Fatalf("retry %#v %v", result, err)
	}
	if len([]rune(responseText)) <= runtimeSummaryMaxRunes {
		t.Fatal("fixture did not exceed old fixed budget")
	}
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.conversation_messages WHERE conversation_id=$1 AND sequence IN (1,5)`, conversation); err != nil {
		t.Fatal(err)
	}
	enqueue()
	at = at.Add(11 * time.Minute)
	responseText = "重建保留当前明确事实与尚未完成的约定。"
	rebuilt := next()
	if rebuilt.FromSequence != 2 {
		t.Fatalf("DELETE rebuild demanded nonexistent sequence1: %#v", rebuilt)
	}
	if result, err := process(rebuilt); err != nil || result["status"] != "active" {
		t.Fatalf("DELETE rebuild %#v %v", result, err)
	}
	state, err = readRuntimeSummaryState(ctx, repo.Pool(), fl, conversation)
	if err != nil || state.Covered <= work.ToSequence || state.Summary != responseText {
		t.Fatalf("rebuild cursor %#v %v", state, err)
	}
	var originals int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1`, conversation).Scan(&originals); err != nil || originals != 62 {
		t.Fatal("summary erased original history", originals, err)
	}
}

func TestRuntimeSummarySameDatasetFullWireBudgetEvidence(t *testing.T) {
	projection := ContextProjection{}
	refs := []any{}
	for n := 1; n <= 40; n++ {
		kind := "user"
		if n%2 == 0 {
			kind = "assistant"
		}
		id := fmt.Sprintf("budget-message-%d", n)
		projection.RecentMessages = append(projection.RecentMessages, map[string]any{"id": id, "sequence": n, "kind": kind, "text": fmt.Sprintf("第%d条：", n) + strings.Repeat("计划仍未完成，用户在国外，具体时区未知。", 4), "turn_id": fmt.Sprintf("turn-%d", (n+1)/2)})
		if n <= 36 {
			refs = append(refs, "message:"+id)
		}
	}
	summaries := []map[string]any{{"summary": "用户明确说在国外，具体时区未知。购买计划尚未执行，承诺继续保留。", "from_sequence": 1, "to_sequence": 36, "required": true, "source_message_refs": refs}}
	assemble := func(summary []map[string]any) PromptAssemblyResult {
		memory, err := ResolveWorkingMemory(workingMemoryInputFromProjectionForSurface(projection, ProviderContextSurfaceConversationMain, nil, summary), DefaultWorkingMemoryPolicy())
		if err != nil {
			t.Fatal(err)
		}
		result, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", OperationRules: []string{"事实必须有来源"}, CorePersona: map[string]any{"identity": map[string]any{"name": "摇光"}}, WorkingMemory: memory, CurrentInput: "请继续核对购买计划", Tools: []map[string]any{{"type": "function", "function": map[string]any{"name": "wardrobe.inspect", "parameters": map[string]any{"type": "object"}}}}, ResponseFormat: map[string]any{"type": "json_object"}, Policy: DefaultPromptBudgetPolicy(4096)})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	before, after := assemble(nil), assemble(summaries)
	if after.Trace.EstimatedInputTokens >= before.Trace.EstimatedInputTokens || !strings.Contains(jsonString(after.Messages), "具体时区未知") {
		t.Fatalf("budget/correction lost: before=%#v after=%#v", before.Trace, after.Trace)
	}
	t.Logf("BUDGET_EVIDENCE=%s", jsonString(map[string]any{"dataset_messages": 40, "covered_through": 36, "tail_messages": 4, "summary_runes": len([]rune(stringValue(summaries[0]["summary"]))), "before": before.Trace, "after": after.Trace, "measurement": "estimated tokens; no live tokenizer/usage"}))
}
