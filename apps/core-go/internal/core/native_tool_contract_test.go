package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func nativeGoalTestProjection() ContextProjection {
	return ContextProjection{ReferenceIndex: ContextReferenceIndex{ActiveProfileID: "current", ByRef: map[string]ContextReference{
		"goal:ctx_0123456789abcdef0123456789abcdef":   {Kind: ContextReferenceGoal, EntityID: "private-db-id", Revision: 25},
		"memory:ctx_0123456789abcdef0123456789abcdef": {Kind: ContextReferenceMemory, EntityID: "private-memory"},
	}}}
}
func TestNativeGoalContractBindsFrozenScopeAndProtectsCanonicalAPI(t *testing.T) {
	projection := nativeGoalTestProjection()
	c := goalCapability{name: "goal.evaluate"}
	ref := currentGoalRefs(projection.ReferenceIndex)[0]
	canonical := jsonString(c.Definition().InputSchema)
	if !strings.Contains(canonical, "goal_id") || !strings.Contains(canonical, "expected_revision") {
		t.Fatal(canonical)
	}
	native := c.NativeDefinition()
	if strings.Contains(jsonString(native.InputSchema), "goal_id") || strings.Contains(jsonString(native.InputSchema), "expected_revision") {
		t.Fatal(native)
	}
	args := jsonBytes(map[string]any{"goal_ref": ref, "reason": "new evidence"})
	if err := validateNativeArguments(args, native); err != nil {
		t.Fatal(err)
	}
	bound, err := c.BindNativeArguments(args, projection)
	if err != nil || stringValue(decodeObject(bound)["goal_id"]) != "private-db-id" || intValue(decodeObject(bound)["expected_revision"]) != 25 {
		t.Fatalf("%s %v", bound, err)
	}
	for _, wrong := range []string{"private-db-id", "goal:ctx_abcdef0123456789abcdef0123456789", "memory:ctx_0123456789abcdef0123456789abcdef"} {
		_, err := c.BindNativeArguments(jsonBytes(map[string]any{"goal_ref": wrong}), projection)
		if err == nil || !strings.Contains(err.Error(), ref) || strings.Contains(err.Error(), "private-db-id") {
			t.Fatalf("%s: %v", wrong, err)
		}
	}
	zeroRevision := nativeGoalTestProjection()
	entry := zeroRevision.ReferenceIndex.ByRef[ref]
	entry.Revision = 0
	zeroRevision.ReferenceIndex.ByRef[ref] = entry
	if _, err := c.BindNativeArguments(args, zeroRevision); err == nil {
		t.Fatal("zero revision must not bypass frozen revision validation")
	}
	inspect := goalCapability{name: "goal.inspect"}
	full := map[string]any{"goals": []any{map[string]any{"id": "private-db-id", "revision": 25, "desired_outcome": "a finite goal", "profile_id": "private", "reviews": []any{"hidden history"}}, map[string]any{"id": "foreign", "desired_outcome": "hidden"}}, "next_cursor": 10}
	result := jsonString(inspect.NativeOutput(full, projection))
	if !strings.Contains(result, ref) || !strings.Contains(result, "25") || strings.Contains(result, "private-db-id") || strings.Contains(result, "hidden") || strings.Contains(result, "90") {
		t.Fatal(result)
	}
	stale := map[string]any{"goals": []any{map[string]any{"id": "private-db-id", "revision": 26, "desired_outcome": "newer semantics"}}}
	if output := jsonString(inspect.NativeOutput(stale, projection)); strings.Contains(output, "newer semantics") || strings.Contains(output, ref) {
		t.Fatal(output)
	}

}
func TestNativeActorContractRequiresAssertionAndBindsCurrentSource(t *testing.T) {
	c := actorFactCapability{}
	args := jsonBytes(map[string]any{"operation": "assert", "attribute": "reading_preference", "value": "fiction", "reason": "current statement", "assertion_type": "explicit_statement"})
	if err := validateNativeArguments(args, c.NativeDefinition()); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []map[string]any{
		{"operation": "assert", "attribute": "reading_preference", "value": "fiction", "reason": "guess"},
		{"operation": "assert", "attribute": "reading_preference", "value": "fiction", "reason": "guess", "assertion_type": "explicit_statement", "source_message_id": "fabricated"},
	} {
		if err := validateNativeArguments(jsonBytes(invalid), c.NativeDefinition()); err == nil {
			t.Fatal(invalid)
		}
	}
	if !strings.Contains(jsonString(c.Definition().InputSchema), "source_message_id") {
		t.Fatal("direct source compatibility lost")
	}
}
func TestGoalCandidateContractTracksCurrentKindsAndRefresh(t *testing.T) {
	projection := nativeGoalTestProjection()
	codec, err := newProviderContextRefCodec(projection.ReferenceIndex)
	if err != nil {
		t.Fatal(err)
	}
	ref := currentGoalRefs(projection.ReferenceIndex)[0]
	candidate := func(ref string) map[string]any {
		return map[string]any{"goal_event_candidates": []any{map[string]any{"goal_ref": ref, "reason": "related"}}}
	}
	for _, wrong := range []string{"private-db-id", "memory:ctx_0123456789abcdef0123456789abcdef"} {
		if err := validateGoalEventCandidates(candidate(wrong), codec.currentIndex()); err == nil {
			t.Fatal(wrong)
		}
	}
	if err := validateGoalEventCandidates(candidate(ref), codec.currentIndex()); err != nil {
		t.Fatal(err)
	}
	if err := codec.registerIndex(ContextReferenceIndex{ByRef: map[string]ContextReference{}}); err != nil {
		t.Fatal(err)
	}
	if err := validateGoalEventCandidates(candidate(ref), codec.currentIndex()); err == nil {
		t.Fatal("prior profile reference retained authority")
	}
}

// This transport fixture owns fake effects; production App binding is tested
// independently above and the PostgreSQL adapter suites exercise persistence.
type nativeContractWireInvoker struct {
	trace      *ADKCapabilityTrace
	projection ContextProjection
	calls      int
	replies    int
}

func (i *nativeContractWireInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return i.ExecuteWithID(ctx, "", name, args)
}
func (i *nativeContractWireInvoker) NativeDefinitions(ds []CapabilityDefinition) []CapabilityDefinition {
	out := append([]CapabilityDefinition(nil), ds...)
	for n, d := range out {
		if d.Name == "goal.evaluate" {
			out[n] = (goalCapability{name: d.Name}).NativeDefinition()
		}
		if d.Name == actorFactCapabilityName {
			out[n] = (actorFactCapability{}).NativeDefinition()
		}
	}
	return out
}
func (i *nativeContractWireInvoker) ExecuteWithID(_ context.Context, id, name, args string) (string, error) {
	if name == conversationReplyCapabilityName {
		i.replies++
		i.trace.AppendResult(CapabilityResult{CallID: id, CapabilityName: name, Status: "completed", Output: map[string]any{"message_id": "committed-reply"}})
		return `{"status":"completed"}`, nil
	}
	i.calls++
	c := goalCapability{name: name}
	if err := validateNativeArguments(json.RawMessage(args), c.NativeDefinition()); err != nil {
		return "", err
	}
	bound, err := c.BindNativeArguments(json.RawMessage(args), i.projection)
	if err != nil {
		return "", err
	}
	if stringValue(decodeObject(bound)["goal_id"]) != "private-db-id" || intValue(decodeObject(bound)["expected_revision"]) != 25 {
		return "", fmt.Errorf("frozen binding failed")
	}
	i.trace.AppendResult(CapabilityResult{CallID: id, CapabilityName: name, Status: "completed", Output: map[string]any{"status": "queued"}})
	return `{"status":"completed","output":{"status":"queued"}}`, nil
}
func TestNativeGoalPhysicalSchemasAndFinalReferenceRepair(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			projection := nativeGoalTestProjection()
			ref := currentGoalRefs(projection.ReferenceIndex)[0]
			codec, _ := newProviderContextRefCodec(projection.ReferenceIndex)
			requests := []map[string]any{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
					return
				}
				requests = append(requests, payload)
				message := map[string]any{"role": "assistant"}
				finish := "stop"
				switch len(requests) {
				case 1:
					message["tool_calls"] = []any{map[string]any{"index": 0, "id": "native-evaluate", "type": "function", "function": map[string]any{"name": "goal.evaluate", "arguments": jsonString(map[string]any{"goal_ref": ref, "reason": "new evidence"})}}}
					calls := arrayValue(message["tool_calls"])
					calls = append(calls, map[string]any{"index": 1, "id": "native-reply", "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": `{"text":"actual native reply"}`}})
					message["tool_calls"] = calls
					finish = "tool_calls"
				case 2:
					message["content"] = jsonString(map[string]any{"goal_event_candidates": []any{map[string]any{"goal_ref": "private-db-id", "reason": "related"}}})
				default:
					message["content"] = jsonString(map[string]any{"goal_event_candidates": []any{map[string]any{"goal_ref": ref, "reason": "related"}}})
				}
				if payload["stream"] == true {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": finish}}}))
				} else {
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": finish}}})
				}
			}))
			defer server.Close()
			trace := &ADKCapabilityTrace{}
			invoker := &nativeContractWireInvoker{trace: trace, projection: projection}
			ctx := withADKCapabilityContextRefs(context.Background(), invoker, trace, nil, codec)
			p := &ProviderClient{HTTP: server.Client()}
			outputSchema := objectSchema(map[string]any{"goal_event_candidates": goalEventCandidatesSchema()}, []string{"goal_event_candidates"}, false)
			response, err := p.generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: time.Second, TokenBudget: 4096, ContextWindowTokens: 131072, MaxInputTokens: 98304, PromptBudgetPolicyVersion: "prompt-budget.v1"}, Role: "cognitive_assessment", Messages: []map[string]any{{"role": "user", "content": "evaluate current goal"}}, Definitions: []CapabilityDefinition{(goalCapability{name: "goal.evaluate"}).Definition(), actorFactDefinition(), conversationReplyCapabilityDefinition()}, JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: outputSchema, EnableStreaming: streaming, CorrelationID: "native-ref-test"})
			if err != nil || len(requests) != 3 || invoker.calls != 1 || invoker.replies != 1 || response.Message == nil {
				t.Fatalf("calls=%d requests=%d response=%#v err=%v", invoker.calls, len(requests), response, err)
			}
			for _, request := range requests[:2] {
				tools := jsonString(request["tools"])
				if strings.Contains(tools, "goal_id") || strings.Contains(tools, "source_message_id") || !strings.Contains(tools, "goal_ref") || !strings.Contains(tools, "assertion_type") {
					t.Fatal(tools)
				}
			}
			repair := requests[2]
			if len(arrayValue(repair["tools"])) != 0 || repair["tool_choice"] != nil || !strings.Contains(jsonString(repair["messages"]), "allowed_goal_refs") {
				t.Fatalf("repair wire: %#v", repair)
			}
		})
	}
}

func TestNativeActorCurrentInboundSourcePostgres(t *testing.T) {
	f := seedWardrobeToolFixture(t) // isolated opt-in fixture; SKIP without PG
	source := actorFactSourceMessage(t, f, "native-current", f.ownerID, "我喜欢小说")
	factID := "native-actor-current-" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) SELECT $1,$2,COALESCE(max(sequence),0)+1,'conversation.turn',$3,$1,$1,$1,now(),'processed' FROM public.cognition_inbox WHERE fluctlight_id=$2`, factID, f.fluctlightID, jsonBytes(map[string]any{"text": "我喜欢小说", "conversation_id": f.conversationID, "actor_id": f.ownerID})); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.conversation_messages SET source_fact_id=$2 WHERE id=$1`, source, factID); err != nil {
		t.Fatal(err)
	}
	projection, err := f.app.BuildContextProjection(f.ctx, f.ownerID, f.fluctlightID, f.conversationID, factID, "我喜欢小说")
	if err != nil {
		t.Fatal(err)
	}
	trace := &ADKCapabilityTrace{}
	invoker := newAppADKCapabilityInvoker(f.app, ADKCapabilityRequest{FluctlightID: f.fluctlightID, ConversationID: f.conversationID, SourceFactID: factID, OperationID: factID, AuthorizationActorID: f.ownerID, Projection: projection}, trace)
	ctx := withADKCapabilityContext(f.ctx, invoker, trace, nil)
	definition, _ := FormalAgentDefinitionByID(FormalAgentConversationCognition)
	ctx = withFormalAgentDefinition(ctx, definition)
	for position, assertion := range []string{"explicit_statement", "inference"} {
		id := "native-actor-" + assertion
		trace.RecordModelToolCalls("model-actor-"+assertion, uint64(position+1), []schema.ToolCall{{ID: id, Function: schema.FunctionCall{Name: actorFactCapabilityName}}})
		output, err := invoker.(ADKCapabilityInvokerWithID).ExecuteWithID(ctx, id, actorFactCapabilityName, jsonString(map[string]any{"operation": "assert", "attribute": "reading_preference", "value": "fiction", "reason": "current source", "assertion_type": assertion}))
		if err != nil || !strings.Contains(output, `"status":"completed"`) {
			t.Fatalf("%s %v", output, err)
		}
		var actualSource, kind, status string
		if err := f.repository.Pool().QueryRow(ctx, `SELECT source_message_id,epistemic_kind,status FROM public.actor_facts WHERE owner_fluctlight_id=$1 AND operation_id=$2`, f.fluctlightID, trace.Invocations[len(trace.Invocations)-1].Metadata.OperationID).Scan(&actualSource, &kind, &status); err != nil {
			t.Fatal(err)
		}
		if actualSource != source {
			t.Fatalf("source %s", actualSource)
		}
		if assertion == "inference" && (kind != "inference" || status != "uncertain") {
			t.Fatalf("%s %s", kind, status)
		}
		if assertion == "explicit_statement" && (kind != "user_statement" || status != "active") {
			t.Fatalf("%s %s", kind, status)
		}
	}
}

func TestNativeArgumentRejectionIsTracedAndCannotReuseChangedCall(t *testing.T) {
	app := &App{}
	registry, err := NewCapabilityRegistry(builtinCapabilities(app)...)
	if err != nil {
		t.Fatal(err)
	}
	app.Capabilities = registry
	for _, args := range []string{`{"goal_ref":"foreign","reason":"new"}`, `{"goal_id":"private","reason":"new"}`} {
		trace := &ADKCapabilityTrace{}
		trace.RecordModelToolCalls("physical-request", 1, []schema.ToolCall{{ID: "bad-call"}})
		invoker := newAppADKCapabilityInvoker(app, ADKCapabilityRequest{OperationID: "test-run", Projection: nativeGoalTestProjection()}, trace).(ADKCapabilityInvokerWithID)
		result, err := invoker.ExecuteWithID(context.Background(), "bad-call", "goal.evaluate", args)
		if err != nil || !strings.Contains(result, `"error_code":"invalid_arguments"`) {
			t.Fatalf("%s %v", result, err)
		}
		invocations, results := trace.Snapshot()
		if len(invocations) != 1 || len(results) != 1 || results[0].Retryable || invocations[0].ProviderRequestID != "physical-request" {
			t.Fatalf("%#v %#v", invocations, results)
		}
		repeated, err := invoker.ExecuteWithID(context.Background(), "bad-call", "goal.evaluate", args)
		if err != nil || repeated != result {
			t.Fatalf("%s %v", repeated, err)
		}
		_, err = invoker.ExecuteWithID(context.Background(), "bad-call", "goal.evaluate", `{"goal_ref":"another","reason":"new"}`)
		if err == nil || err.Error() != "adk_tool_call_id_reused" {
			t.Fatalf("%v", err)
		}
		invocations, results = trace.Snapshot()
		if len(invocations) != 1 || len(results) != 1 {
			t.Fatal("duplicate rejected trace")
		}
	}
}

func TestNativeGoalInspectPreservesScopedPagination(t *testing.T) {
	c := goalCapability{name: "goal.inspect"}
	args := jsonBytes(map[string]any{"operation": "list", "cursor": 10, "include_closed": true})
	if err := validateNativeArguments(args, c.NativeDefinition()); err != nil {
		t.Fatal(err)
	}
	output := mapValue(c.NativeOutput(map[string]any{"goals": []any{}, "has_more": true, "next_cursor": 20}, nativeGoalTestProjection()))
	if output["has_more"] != true || intValue(output["next_cursor"]) != 20 {
		t.Fatal(output)
	}
}
