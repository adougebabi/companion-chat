package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

type agentRecoveryFixture struct {
	ctx                      context.Context
	repo                     *PostgresRepository
	app                      *App
	owner, fid, conversation string
	definition               FormalAgentDefinition
	input                    ADKStructuredTaskInput
	record                   *formalRunRecord
	invocation               CapabilityInvocation
	result                   CapabilityResult
}

func seedAgentRecovery(t *testing.T, suffix string) agentRecoveryFixture {
	t.Helper()
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "recovery-"+suffix)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture') ON CONFLICT DO NOTHING`, owner); err != nil {
		t.Fatal(err)
	}
	ctx = WithProviderCorrelation(ctx, "turn:recovery-"+suffix)
	definition, _ := FormalAgentDefinitionByID(FormalAgentConversationCognition)
	input := ADKStructuredTaskInput{AgentID: definition.ID, Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "immutable recovery request"}}}, Capability: &ADKCapabilityRequest{AuthorizationActorID: owner, FluctlightID: fid, OperationID: "run-" + suffix}}
	record, prior, err := app.admitFormalRun(ctx, definition, input)
	if err != nil || record == nil || prior != nil {
		t.Fatalf("admission %v %v", record, err)
	}
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{AgentID: definition.ID, RunID: record.RunID, CapabilityName: "conversation.reply", OperationID: "effect-" + suffix, AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: json.RawMessage(`{"text":"已真实提交的恢复测试消息"}`)})
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT invocation FROM public.tool_executions WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, fid, definition.ID, record.RunID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var invocation CapabilityInvocation
	if err := json.Unmarshal(raw, &invocation); err != nil {
		t.Fatal(err)
	}
	return agentRecoveryFixture{ctx, repo, app, owner, fid, conversation, definition, input, record, invocation, receipt.Result}
}
func (f agentRecoveryFixture) expire(t *testing.T) {
	t.Helper()
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.agent_runs SET started_at=now()-interval '6 minutes' WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, f.fid, f.definition.ID, f.record.RunID); err != nil {
		t.Fatal(err)
	}
}
func (f agentRecoveryFixture) messages(t *testing.T) int {
	t.Helper()
	var count int
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, f.conversation).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
func (f agentRecoveryFixture) stored(t *testing.T) (string, ADKStructuredTaskResult) {
	t.Helper()
	var status string
	var raw []byte
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT status,result FROM public.agent_runs WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, f.fid, f.definition.ID, f.record.RunID).Scan(&status, &raw); err != nil {
		t.Fatal(err)
	}
	var result ADKStructuredTaskResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return status, result
}

func TestStaleAgentRunRecoversAndPersistsCommittedTraceWithoutReplay(t *testing.T) {
	f := seedAgentRecovery(t, "stale")
	f.expire(t)
	for i := 0; i < 2; i++ {
		record, result, err := f.app.admitFormalRun(f.ctx, f.definition, f.input)
		if record != nil || err == nil || !strings.Contains(err.Error(), "stale_run_timeout") || result == nil || result.Trace == nil || len(result.Trace.Results) != 1 || result.Trace.Results[0].CallID != f.result.CallID {
			t.Fatalf("timeout lost committed receipt: result=%#v err=%v", result, err)
		}
	}
	status, stored := f.stored(t)
	if status != "failed" || stored.Trace == nil || len(stored.Trace.Results) != 1 {
		t.Fatalf("timeout record lost Trace: status=%s result=%#v", status, stored)
	}
	if err := f.app.finishFormalRun(f.ctx, f.record, ADKStructuredTaskResult{Trace: &ADKCapabilityTrace{}}, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("late finish reopened failed run: %v", err)
	}
	f.app.DiagnosticDebugReadEnabled = true
	page, err := f.app.OwnerDiagnosticRecords(f.ctx, f.owner, "agent-runs", url.Values{"correlationId": {"turn:recovery-stale"}})
	if err != nil || !strings.Contains(jsonString(page), f.result.CallID) {
		t.Fatalf("diagnostic record lacks committed receipt: err=%v", err)
	}
	if f.messages(t) != 1 {
		t.Fatal("recovery repeated the published message")
	}
}

func TestFailedAgentRunHydratesLegacyEmptyAndInvocationOnlyTrace(t *testing.T) {
	f := seedAgentRecovery(t, "legacy")
	only := &ADKCapabilityTrace{}
	only.AppendInvocation(f.invocation)
	for _, raw := range [][]byte{[]byte(`{}`), []byte(`{"Trace":null}`), jsonBytes(ADKStructuredTaskResult{Trace: only})} {
		if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.agent_runs SET status='failed',error_detail='interrupted',result=$4 WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, f.fid, f.definition.ID, f.record.RunID, raw); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			_, result, err := f.app.admitFormalRun(f.ctx, f.definition, f.input)
			if err == nil || result == nil || result.Trace == nil || len(result.Trace.Results) != 1 || len(result.Trace.Invocations) != 1 {
				t.Fatalf("legacy recovery duplicated/lost data: %#v %v", result, err)
			}
		}
		_, stored := f.stored(t)
		if stored.Trace == nil || len(stored.Trace.Results) != 1 {
			t.Fatal("legacy recovered Trace was not persisted")
		}
	}
	if f.messages(t) != 1 {
		t.Fatal("legacy recovery repeated effects")
	}
}

func TestAgentRecoveryPreservesExistingQueryFailureCoordinatesAndExactNumbers(t *testing.T) {
	f := seedAgentRecovery(t, "mixed")
	f.expire(t)
	trace := &ADKCapabilityTrace{}
	trace.Append(CapabilityInvocation{CallID: "query", CapabilityName: "fixture.query"}, CapabilityResult{CallID: "query", CapabilityName: "fixture.query", Status: "completed", Output: map[string]any{"value": int64(9007199254740993)}})
	trace.AppendInvocation(CapabilityInvocation{CallID: "failed", CapabilityName: "fixture.failure"})
	retry := f.invocation
	retry.CallID = "retry-native-id"
	trace.AppendInvocation(retry)
	replay := f.result
	replay.CallID = retry.CallID
	trace.AppendResult(replay)
	trace.AppendResultWithFailure(CapabilityResult{CallID: "failed", CapabilityName: "fixture.failure", Status: "failed", Output: map[string]any{"detail": "Authorization: Bearer " + t.Name(), "ordinary": "keep"}}, errors.New("preserved failure"))
	trace.RecordModelToolCalls("fixture-physical-call", 2, []schema.ToolCall{{ID: "query"}, {ID: "failed"}, {ID: retry.CallID}})
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.agent_runs SET result=$4::jsonb||'{"future_extension":{"value":9007199254740993}}'::jsonb WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, f.fid, f.definition.ID, f.record.RunID, jsonBytes(ADKStructuredTaskResult{Trace: trace})); err != nil {
		t.Fatal(err)
	}
	_, result, err := f.app.admitFormalRun(f.ctx, f.definition, f.input)
	if err == nil || result == nil || result.Trace == nil || len(result.Trace.Invocations) != 3 || len(result.Trace.Results) != 3 {
		t.Fatalf("existing Trace replaced/duplicated: %#v %v", result, err)
	}
	if !strings.Contains(jsonString(result.Trace.Results[0].Output), "9007199254740993") || mapValue(result.Trace.Results[2].Output)["detail"] != "[REDACTED]" {
		t.Fatalf("numeric evidence or failure privacy damaged: %#v", result.Trace.Results)
	}
	identity, ok := result.Trace.ModelIdentity(retry.CallID)
	if !ok || identity.ProviderRequestID != "fixture-physical-call" {
		t.Fatal("model coordinates lost")
	}
	var future, number string
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT result#>>'{future_extension,value}',result#>>'{Trace,Results,0,output,value}' FROM public.agent_runs WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, f.fid, f.definition.ID, f.record.RunID).Scan(&future, &number); err != nil || future != "9007199254740993" || number != "9007199254740993" {
		t.Fatalf("persist overwrote unrelated/exact data: future=%s number=%s err=%v", future, number, err)
	}
	if f.messages(t) != 1 {
		t.Fatal("mixed recovery repeated effects")
	}
}

func TestAgentRecoveryInputConflictDoesNotConsumeCorruptLedger(t *testing.T) {
	f := seedAgentRecovery(t, "conflict")
	f.expire(t)
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.tool_executions SET result='{"call_id":[]}'::jsonb WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, f.fid, f.definition.ID, f.record.RunID); err != nil {
		t.Fatal(err)
	}
	changed := f.input
	changed.Prompt.Messages = []map[string]any{{"role": "user", "content": "changed immutable input"}}
	if _, _, err := f.app.admitFormalRun(f.ctx, f.definition, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("input conflict lost precedence: %v", err)
	}
	status, _ := f.stored(t)
	if status != "running" || f.messages(t) != 1 {
		t.Fatal("conflicting input altered run/effect")
	}
	if _, _, err := f.app.admitFormalRun(f.ctx, f.definition, f.input); err == nil {
		t.Fatal("malformed ledger was silently ignored")
	} else {
		var malformed *json.UnmarshalTypeError
		if !errors.As(err, &malformed) {
			t.Fatalf("corruption reported as unrelated failure: %v", err)
		}
	}
}

func TestAgentTimeoutStorageFailureIsNotSilentlyAccepted(t *testing.T) {
	f := seedAgentRecovery(t, "storage")
	f.expire(t)
	if _, err := f.repo.Pool().Exec(f.ctx, `CREATE FUNCTION public.reject_timeout() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.error_detail='stale_run_timeout' THEN RAISE EXCEPTION 'isolated timeout storage failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_timeout BEFORE UPDATE ON public.agent_runs FOR EACH ROW EXECUTE FUNCTION public.reject_timeout()`); err != nil {
		t.Fatal(err)
	}
	defer f.repo.Pool().Exec(f.ctx, `DROP TRIGGER reject_timeout ON public.agent_runs; DROP FUNCTION public.reject_timeout()`)
	_, result, err := f.app.admitFormalRun(f.ctx, f.definition, f.input)
	if err == nil || !strings.Contains(err.Error(), "isolated timeout storage failure") || result == nil || result.Trace == nil || len(result.Trace.Results) != 1 {
		t.Fatalf("storage failure hidden or committed receipt lost: %#v %v", result, err)
	}
	status, _ := f.stored(t)
	if status != "running" || f.messages(t) != 1 {
		t.Fatal("failed write terminalized/repeated run")
	}
}

func TestAgentConcurrentFinishWinsOverStaleTimeoutCAS(t *testing.T) {
	f := seedAgentRecovery(t, "finish-race")
	f.expire(t)
	barrier, err := f.repo.Pool().Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer barrier.Rollback(f.ctx)
	if _, err := barrier.Exec(f.ctx, `LOCK TABLE public.tool_executions IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	type observed struct {
		result *ADKStructuredTaskResult
		err    error
	}
	done := make(chan observed, 1)
	go func() {
		_, result, err := f.app.admitFormalRun(f.ctx, f.definition, f.input)
		done <- observed{result, err}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var blocked bool
		err := f.repo.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%FROM public.tool_executions%' AND query NOT LIKE '%pg_stat_activity%')`).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("admission did not reach ledger barrier")
		}
		time.Sleep(5 * time.Millisecond)
	}
	live := &ADKCapabilityTrace{}
	live.Append(f.invocation, f.result)
	if err := f.app.finishFormalRun(f.ctx, f.record, ADKStructuredTaskResult{Completion: ProviderCompletion{Structured: map[string]any{"winner": "normal-finish"}}, Trace: live}, nil); err != nil {
		t.Fatal(err)
	}
	if err := barrier.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil || got.result == nil || got.result.Completion.Structured["winner"] != "normal-finish" {
		t.Fatalf("finished run misreported as timeout: %#v %v", got.result, got.err)
	}
	status, stored := f.stored(t)
	if status != "completed" || stored.Completion.Structured["winner"] != "normal-finish" || f.messages(t) != 1 {
		t.Fatal("timeout overwrote completed result")
	}
}

func TestAgentRecordedTraceDecodesOpaqueNumbersWithoutFloatConversion(t *testing.T) {
	raw := []byte(`{"Completion":{"Structured":{"ordinary":3}},"Trace":{"Results":[{"call_id":"opaque","capability_name":"fixture.query","status":"completed","output":{"huge":1e400,"exact":9007199254740993,"zero":0,"flag":false}}]}}`)
	result, err := decodeRecordedAgentResult(raw)
	if err != nil || result.Trace == nil || len(result.Trace.Results) != 1 {
		t.Fatalf("valid opaque trace rejected: %v", err)
	}
	output := jsonString(result.Trace.Results[0].Output)
	if !strings.Contains(output, "1e400") || !strings.Contains(output, "9007199254740993") || !strings.Contains(output, `"zero":0`) || !strings.Contains(output, `"flag":false`) {
		t.Fatalf("opaque values changed: %s", output)
	}
	if _, ok := result.Completion.Structured["ordinary"].(float64); !ok {
		t.Fatal("unrelated Completion decoding changed")
	}
}

func TestCommittedAgentTraceMergeUsesBusinessIdentityAndReceiptData(t *testing.T) {
	first := CapabilityInvocation{CallID: "reused-native-id", CapabilityName: "effect", Metadata: InvocationMetadata{OperationID: "first-operation"}}
	second := first
	second.Metadata.OperationID = "second-operation"
	one := CapabilityResult{CallID: first.CallID, CapabilityName: first.CapabilityName, Status: "completed", Output: map[string]any{"resource": "one"}}
	two := one
	two.Output = map[string]any{"resource": "two"}
	trace := &ADKCapabilityTrace{}
	trace.Append(second, two)
	for i := 0; i < 2; i++ {
		used := map[int]bool{}
		mergeCommittedAgentReceipt(trace, first, one, used, false)
		mergeCommittedAgentReceipt(trace, second, two, used, false)
	}
	if len(trace.Invocations) != 2 || len(trace.Results) != 2 {
		t.Fatalf("native id reused across operations hid/duplicated facts: %#v", trace)
	}
	found := map[string]bool{}
	for _, result := range trace.Results {
		found[stringValue(mapValue(result.Output)["resource"])] = true
	}
	if !found["one"] || !found["two"] {
		t.Fatalf("committed output lost: %#v", trace.Results)
	}
}

func TestAgentLegacyNativeCallCannotClaimTwoBusinessOperations(t *testing.T) {
	legacy := CapabilityInvocation{CallID: "reused", CapabilityName: "effect"}
	one, two := legacy, legacy
	one.Metadata.OperationID = "one"
	two.Metadata.OperationID = "two"
	r1 := CapabilityResult{CallID: "reused", CapabilityName: "effect", Status: "completed", Output: map[string]any{"resource": "one"}}
	r2 := r1
	r2.Output = map[string]any{"resource": "two"}
	trace := &ADKCapabilityTrace{}
	trace.AppendInvocation(legacy)
	for i := 0; i < 2; i++ {
		used := map[int]bool{}
		mergeCommittedAgentReceipt(trace, one, r1, used, false)
		mergeCommittedAgentReceipt(trace, two, r2, used, false)
	}
	identities := map[string]bool{}
	for _, invocation := range trace.Invocations {
		identities[invocation.Metadata.OperationID] = true
	}
	if !identities["one"] || !identities["two"] || len(trace.Results) != 2 || len(trace.Invocations) != 3 {
		t.Fatalf("legacy unknown call hid business identities: %#v", trace)
	}
}

func TestAgentInputConflictPrecedesBrokenResultAndDecodeErrorsArePrivate(t *testing.T) {
	f := seedAgentRecovery(t, "broken-stored")
	f.expire(t)
	raw := map[string]any{"Trace": map[string]any{"Results": []any{map[string]any{"call_id": "failed", "capability_name": "failure", "status": "failed", "output": map[string]any{"detail": "Authorization: Bearer " + t.Name()}}, map[string]any{"call_id": []any{}}}}}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.agent_runs SET result=$4 WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, f.fid, f.definition.ID, f.record.RunID, jsonBytes(raw)); err != nil {
		t.Fatal(err)
	}
	changed := f.input
	changed.Prompt.Messages = []map[string]any{{"role": "user", "content": "different input"}}
	if _, result, err := f.app.admitFormalRun(f.ctx, f.definition, changed); !errors.Is(err, ErrConflict) || strings.Contains(jsonString(result), "Bearer") {
		t.Fatalf("conflict masked or leaked data: %v", err)
	}
	_, result, err := f.app.admitFormalRun(f.ctx, f.definition, f.input)
	var malformed *json.UnmarshalTypeError
	if !errors.As(err, &malformed) || strings.Contains(jsonString(result), "Bearer") {
		t.Fatalf("partial decoded result leaked or lost error: %v", err)
	}
	if f.messages(t) != 1 {
		t.Fatal("broken result replayed effect")
	}
}

func TestAgentLegacyAmbiguousNativeIDKeepsBothCommittedOperations(t *testing.T) {
	f := seedAgentRecovery(t, "ambiguous-native")
	second, err := f.app.ExecuteTool(f.ctx, ToolExecutionRequest{AgentID: f.definition.ID, RunID: f.record.RunID, CapabilityName: "conversation.reply", OperationID: "another-committed-effect", NativeToolCallID: f.result.CallID, ProviderRequestID: "provider-ambiguous-native", AuthorizationActorID: f.owner, FluctlightID: f.fid, ConversationID: f.conversation, Arguments: json.RawMessage(`{"text":"另一笔已提交消息"}`)})
	if err != nil || second.Result.CallID != f.result.CallID {
		t.Fatalf("shared native ID fixture=%#v err=%v", second, err)
	}
	legacy := &ADKCapabilityTrace{}
	legacy.AppendInvocation(CapabilityInvocation{CallID: f.result.CallID, CapabilityName: "conversation.reply"})
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.agent_runs SET status='failed',error_detail='legacy interruption',result=$4 WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, f.fid, f.definition.ID, f.record.RunID, jsonBytes(ADKStructuredTaskResult{Trace: legacy})); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		_, result, err := f.app.admitFormalRun(f.ctx, f.definition, f.input)
		if err == nil || result == nil || result.Trace == nil {
			t.Fatalf("recovery=%#v %v", result, err)
		}
		identities := map[string]bool{}
		for _, invocation := range result.Trace.Invocations {
			identities[invocation.Metadata.OperationID] = true
		}
		if !identities[f.invocation.Metadata.OperationID] || !identities["another-committed-effect"] || len(result.Trace.Results) != 2 || len(result.Trace.Invocations) != 3 {
			t.Fatalf("ambiguous native ID lost a business receipt: %#v", result.Trace)
		}
	}
	if f.messages(t) != 2 {
		t.Fatal("ambiguous recovery replayed business effects")
	}
}
