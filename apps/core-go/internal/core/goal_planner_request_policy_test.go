package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

// Controlled HTTP evidence: the invoker records protocol receipts only; it does
// not claim PostgreSQL effects or production semantic acceptance.
type plannerPolicyTestInvoker struct {
	mu         sync.Mutex
	trace      *ADKCapabilityTrace
	executions int
	commits    int
}

func (i *plannerPolicyTestInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return i.ExecuteWithID(ctx, "", name, args)
}
func (i *plannerPolicyTestInvoker) ExecuteWithID(_ context.Context, id, name, args string) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if result, ok := i.trace.FindResult(id); ok {
		return jsonString(map[string]any{"status": result.Status, "output": result.Output}), nil
	}
	i.executions++
	arguments := decodeObject([]byte(args))
	i.trace.AppendInvocation(CapabilityInvocation{CallID: id, CapabilityName: name, Arguments: jsonBytes(arguments)})
	status := "completed"
	if id == "rejected-query" {
		status = "failed"
	}
	if name == goalPlannerCommit {
		i.commits++
	}
	result := CapabilityResult{CallID: id, CapabilityName: name, Status: status, Output: map[string]any{"controlled_receipt": id}}
	i.trace.AppendResult(result)
	return jsonString(map[string]any{"status": status, "output": result.Output}), nil
}

func TestGoalPlannerPhysicalPolicyHTTPQueryAndCommitContinuation(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, decision := range []string{"applied", "no_change", "awaiting_facts"} {
			t.Run(fmt.Sprintf("stream=%t/%s", streaming, decision), func(t *testing.T) {
				trace := &ADKCapabilityTrace{}
				invoker := &plannerPolicyTestInvoker{trace: trace}
				ctx := WithADKCapabilityInvoker(context.Background(), invoker, trace)
				ctx = withPhysicalModelRequestPolicy(ctx, goalPlannerPhysicalRequestPolicy{})
				requests := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
						return
					}
					if id := r.Header.Get("Idempotency-Key"); id == "" || r.Header.Get("X-Fluctlight-Provider-Request-Id") != id {
						t.Error("missing physical request identity")
					}

					_, hasFormat := payload["response_format"]
					names := []string{}
					for _, raw := range arrayValue(payload["tools"]) {
						function := mapValue(mapValue(raw)["function"])
						names = append(names, stringValue(function["name"]))
						if mapValue(function["parameters"])["additionalProperties"] != false {
							t.Error("noncanonical schema")
						}
					}
					if payload["tool_choice"] != "auto" || hasFormat || len(names) != 2 {
						t.Fatalf("recommendation became a query-count gate: %#v", payload)
					}

					if boolValue(payload["stream"]) != streaming {
						t.Error("stream wire mismatch")
					}
					if requests > 1 && !strings.Contains(jsonString(payload["messages"]), "controlled_receipt") {
						t.Error("actual Tool result feedback missing")
					}
					calls := []map[string]any{}
					switch requests {
					case 1:
						calls = append(calls, map[string]any{"call_id": "rejected-query", "capability_name": goalPlannerQuery, "arguments": map[string]any{"section": "snapshot"}})
					case 2, 3:
						calls = append(calls, map[string]any{"call_id": "snapshot-query", "capability_name": goalPlannerQuery, "arguments": map[string]any{"section": "snapshot"}})
					case 4:
						calls = append(calls, map[string]any{"call_id": "actor-query", "capability_name": goalPlannerQuery, "arguments": map[string]any{"section": "actor"}})
					case 5:
						if decision == "applied" {
							calls = append(calls, map[string]any{"call_id": "commit", "capability_name": goalPlannerCommit, "arguments": map[string]any{"expected_version": 1, "expected_facts_revision": "facts", "idempotency_key": "controlled", "reason": "review"}}, map[string]any{"call_id": "same-batch-snapshot", "capability_name": goalPlannerQuery, "arguments": map[string]any{"section": "snapshot"}})
						}
					case 7:
						if decision == "applied" {
							calls = append(calls, map[string]any{"call_id": "commit", "capability_name": goalPlannerCommit, "arguments": map[string]any{"expected_version": 1, "expected_facts_revision": "facts", "idempotency_key": "controlled", "reason": "review"}})
						}
					case 6:
						if decision == "applied" {
							calls = append(calls, map[string]any{"call_id": "fresh-snapshot", "capability_name": goalPlannerQuery, "arguments": map[string]any{"section": "snapshot"}})
						}
					}
					message := map[string]any{"role": "assistant", "content": jsonString(map[string]any{"decision": decision, "reason": "observed receipts", "review_condition": "new facts", "suggestions": []any{}})}
					finish := "stop"
					if len(calls) > 0 {
						message["content"] = ""
						message["tool_calls"] = fakeProviderNativeToolCalls(calls)
						finish = "tool_calls"
					}
					if streaming {
						w.Header().Set("Content-Type", "text/event-stream")
						delta := cloneMap(message)
						delete(delta, "role")
						for index, raw := range arrayValue(delta["tool_calls"]) {
							mapValue(raw)["index"] = index
						}
						chunk := map[string]any{"id": "planner-policy", "model": "fake", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
						_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(chunk))
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": finish}}})
				}))
				defer server.Close()
				definition, _ := FormalAgentDefinitionByID(FormalAgentGoalPlanner)
				defs := capabilityCatalog(mustCapabilityRegistry(builtinCapabilities(&App{})...), CapabilitySurfaceGoalPlanner)
				response, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 4096}, Role: "cognitive_assessment", Scenario: "goal_planner", Messages: []map[string]any{{"role": "user", "content": "review"}}, Definitions: defs, JSONMode: true, SchemaName: "goal_planner_v1", ResponseSchema: objectSchema(map[string]any{"decision": stringSchema(), "reason": stringSchema(), "review_condition": stringSchema(), "suggestions": arraySchema(openObjectSchema())}, []string{"decision", "reason", "review_condition", "suggestions"}, false), ProviderRequestID: "planner-policy", CorrelationID: "planner-policy", Agent: definition, EnableStreaming: streaming})
				if err != nil {
					t.Fatal(err)
				}
				queries, commits, reread := goalPlannerQueryCoverage(trace)
				if response.Message == nil || queries < 2 {
					t.Fatal("premature final", queries)
				}
				if decision == "applied" {
					if requests != 8 || commits != 1 || invoker.commits != 1 || !reread {
						t.Fatal(requests, commits, invoker.commits, reread)
					}
				} else if requests != 5 || commits != 0 {
					t.Fatal("forced semantic commit", requests, commits)
				}
			})
		}
	}
}

func TestGoalPlannerQueryCoverageRejectsRepeatedAndFailedQueries(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	trace.AppendResult(CapabilityResult{CallID: "same", CapabilityName: goalPlannerQuery, Status: "completed"})
	trace.AppendResult(CapabilityResult{CallID: "same", CapabilityName: goalPlannerQuery, Status: "completed"})
	trace.AppendResult(CapabilityResult{CallID: "failed", CapabilityName: goalPlannerQuery, Status: "failed"})
	trace.RecordModelToolCalls("physical", 1, []schema.ToolCall{{ID: "same"}})
	queries, commits, reread := goalPlannerQueryCoverage(trace)
	if queries != 1 || commits != 0 || reread {
		t.Fatal(queries, commits, reread)
	}
}
