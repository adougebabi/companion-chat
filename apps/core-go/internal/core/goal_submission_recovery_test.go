package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGoalSubmissionRecoveryAcceptedKinds(t *testing.T) {
	b, err := newGoalEvaluationWireBinding(twoGoalWireSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	id := b.goalsByRef["goal:1"].GoalID
	records := map[string]goalSubmissionRecord{}
	for _, name := range []string{goalPlanSubmit, goalEvaluationSubmit, goalObjectSubmit} {
		object := ""
		if name == goalObjectSubmit {
			object = "stage:1.1"
		}
		records[goalSubmissionKey(name, id, object)] = goalSubmissionRecord{Output: map[string]any{"goal_ref": "goal:1", "object_ref": object, "assessment_accepted": true}}
	}
	values := goalEvaluationAcceptedWireResults(b, map[string]any{"submissions": records})
	for n := 0; n < 25; n++ {
		if jsonString(values) != jsonString(goalEvaluationAcceptedWireResults(b, map[string]any{"submissions": records})) {
			t.Fatal("accepted projection order unstable")
		}
	}
	for _, v := range values {
		if stringValue(v["capability_name"]) == "" || stringValue(v["submission_kind"]) == "" {
			t.Fatalf("accepted target kind lost: %#v", values)
		}
	}
}

func recoveryRoot(ref string) map[string]any {
	return map[string]any{"goal_ref": ref, "judgments": []any{}, "impact": "no_change", "blocker": "", "wait_condition": "", "next_step": "", "residual_motivation": ""}
}

func TestGoalSubmissionRecoveryNormalizesUnknownWithoutInventingSuccess(t *testing.T) {
	b, _ := newGoalEvaluationWireBinding(twoGoalWireSnapshot())
	entry := b.goalsByRef["goal:2"]
	for _, explicit := range []bool{false, true} {
		args := recoveryRoot("goal:2")
		if explicit {
			args["judgments"] = []any{map[string]any{"criterion_ref": "criterion:2.1", "verdict": "unknown", "kind": "semantic", "subject": "actor_self", "discourse": "uncertain", "evidence_refs": []string{}, "reason": "unknown", "criterion_quote": "", "optional_improvement": ""}}
		}
		before := jsonString(args)
		// The baseline raw semantic validator rejects this exact recoverable form.
		original, err := b.hydrateOutput(map[string]any{"evaluations": []any{args}, "plans": []any{}})
		if err != nil {
			t.Fatal(err)
		}
		_, _, _, err = validateGoalCompletionSemantics(entry.Goal, original.Evaluations[0], nil)
		if err == nil || err.Error() != "goal_evaluation_unknown_requires_evidence" {
			t.Fatalf("baseline not reproduced: %v", err)
		}
		normalized, err := normalizeGoalSubmission(b, entry, goalEvaluationSubmit, args)
		if err != nil {
			t.Fatal(err)
		}
		output, err := b.hydrateOutput(map[string]any{"evaluations": []any{normalized}, "plans": []any{}})
		if err != nil {
			t.Fatal(err)
		}
		judgments, complete, progress, err := validateGoalCompletionSemantics(entry.Goal, output.Evaluations[0], nil)
		if err != nil || complete || progress != 0 || len(judgments) != 1 || judgments[0].Verdict != "unknown" || stringValue(normalized["wait_condition"]) == "" {
			t.Fatalf("unsafe normalization: %#v %t %v %v", judgments, complete, progress, err)
		}
		if jsonString(args) != before {
			t.Fatal("input mutated")
		}
	}
}

func TestGoalSubmissionRecoveryScopedRefsAndDuplicates(t *testing.T) {
	b, _ := newGoalEvaluationWireBinding(twoGoalWireSnapshot())
	entry := b.goalsByRef["goal:2"]
	judgment := map[string]any{"criterion_ref": " criterion:2.1 ", "verdict": "unknown", "kind": "semantic", "subject": "actor_self", "discourse": "uncertain", "evidence_refs": []string{}, "reason": "unknown", "criterion_quote": "", "optional_improvement": ""}
	args := recoveryRoot("goal:2")
	args["judgments"] = []any{judgment, cloneMap(judgment)}
	normalized, err := normalizeGoalSubmission(b, entry, goalEvaluationSubmit, args)
	if err != nil || len(arrayValue(normalized["judgments"])) != 1 || stringValue(mapValue(arrayValue(normalized["judgments"])[0])["criterion_ref"]) != "criterion:2.1" {
		t.Fatal(normalized, err)
	}
	for _, ref := range []string{"criterion:missing", "criterion:1.1", "criterion:1.s1.1"} {
		bad := cloneMap(judgment)
		bad["criterion_ref"] = ref
		args["judgments"] = []any{bad}
		_, err := normalizeGoalSubmission(b, entry, goalEvaluationSubmit, args)
		if err == nil {
			t.Fatal("cross scope/unknown accepted", ref)
		}
		receipt := goalSubmissionAwaitingEvidence(b, entry, goalEvaluationSubmit, args, err.Error())
		if receipt["assessment_accepted"] != false || receipt["mutation_committed"] != false || receipt["disposition"] != "awaiting_evidence" || strings.Contains(jsonString(receipt), entry.GoalID) {
			t.Fatal(receipt)
		}
	}
	conflict := cloneMap(judgment)
	conflict["verdict"] = "satisfied"
	args["judgments"] = []any{judgment, conflict}
	if _, err := normalizeGoalSubmission(b, entry, goalEvaluationSubmit, args); err == nil {
		t.Fatal("contradictory duplicate accepted")
	}
	// Evidence must still pass the strict boundary after deterministic ref repair.
	bad := cloneMap(judgment)
	bad["criterion_ref"] = "criterion:2.1"
	bad["evidence_refs"] = []string{"e-forged"}
	args["judgments"] = []any{bad}
	normalized, err = normalizeGoalSubmission(b, entry, goalEvaluationSubmit, args)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.hydrateOutput(map[string]any{"evaluations": []any{normalized}, "plans": []any{}}); err == nil {
		t.Fatal("forged evidence accepted")
	}
	bad["verdict"] = "satisfied"
	bad["discourse"] = "assertion"
	bad["evidence_refs"] = []string{}
	args["judgments"] = []any{bad}
	normalized, _ = normalizeGoalSubmission(b, entry, goalEvaluationSubmit, args)
	output, err := b.hydrateOutput(map[string]any{"evaluations": []any{normalized}, "plans": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = validateGoalCompletionSemantics(entry.Goal, output.Evaluations[0], nil); err == nil {
		t.Fatal("evidence-free satisfied accepted")
	}
}

func TestGoalSubmissionRecoveryReplayKeepsOriginal(t *testing.T) {
	prior := goalSubmissionRecord{Digest: "first", Output: map[string]any{"goal_ref": "goal:1", "status": "active", "assessment_accepted": true}}
	before := jsonString(prior)
	output := goalSubmissionReplay(prior, "changed", goalPlanSubmit)
	if output["ignored_payload_change"] != true || output["replayed"] != true || output["status"] != "active" || output["submission_kind"] != "plan" || jsonString(prior) != before {
		t.Fatal(output, prior)
	}
}

type recoveryNativeInvoker struct {
	binding  *goalEvaluationWireBinding
	records  map[string]goalSubmissionRecord
	trace    *ADKCapabilityTrace
	attempts int
}

func (i *recoveryNativeInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return i.ExecuteWithID(ctx, "", name, args)
}
func (i *recoveryNativeInvoker) ExecuteWithID(_ context.Context, id, name, raw string) (string, error) {
	i.attempts++
	var args map[string]any
	_ = json.Unmarshal([]byte(raw), &args)
	entry := i.binding.goalsByRef[stringValue(args["goal_ref"])]
	key := goalSubmissionKey(name, entry.GoalID, stringValue(args["object_ref"]))
	var output map[string]any
	if prior, ok := i.records[key]; ok {
		output = goalSubmissionReplay(prior, stableDigest(raw), name)
	} else {
		_, err := normalizeGoalSubmission(i.binding, entry, name, args)
		if err != nil {
			output = goalSubmissionAwaitingEvidence(i.binding, entry, name, args, err.Error())
		} else {
			output = map[string]any{"goal_ref": args["goal_ref"], "assessment_accepted": true, "submission_kind": goalSubmissionKind(name)}
			i.records[key] = goalSubmissionRecord{Digest: stableDigest(raw), Output: output}
		}
	}
	i.trace.AppendResult(CapabilityResult{CallID: id, CapabilityName: name, Status: "completed", Output: output})
	return jsonString(map[string]any{"status": "completed", "output": output}), nil
}

func TestGoalSubmissionRecoveryNativeHTTPBoundedAndProgress(t *testing.T) {
	for _, mode := range []string{"replay_plan", "bad_criterion", "distinct_targets", "replay_plan_ignores_tools", "bad_criterion_ignores_tools", "replay_plan_ignores_tools_stream", "provider_error_final", "cancelled_final"} {
		t.Run(mode, func(t *testing.T) {
			b, _ := newGoalEvaluationWireBinding(twoGoalWireSnapshot())
			trace := &ADKCapabilityTrace{}
			invoker := &recoveryNativeInvoker{binding: b, records: map[string]goalSubmissionRecord{}, trace: trace}
			progress := &goalEvaluationProgress{}
			baseCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := WithADKCapabilityInvoker(baseCtx, invoker, trace)
			ctx = withPhysicalModelRequestPolicy(ctx, physicalModelRequestPolicyFunc(func(ctx context.Context) (physicalModelRequestDecision, error) {
				_, missing := goalEvaluationSubmissionCoverage(b.snapshot, invoker.records)
				return progress.decide(len(invoker.records), goalEvaluationNativeAttempts(ctx), len(missing) > 0), nil
			}))
			physical := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				physical++
				if physical > 9 {
					t.Error("native loop exceeded finite request budget")
					http.Error(w, "request budget exceeded", http.StatusInternalServerError)
					return
				}
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				message := map[string]any{"role": "assistant", "content": jsonString(map[string]any{"summary": "awaiting available evidence"})}
				if len(arrayValue(body["tools"])) == 0 {
					for _, key := range []string{"tool_choice", "parallel_tool_calls"} {
						if _, ok := body[key]; ok {
							t.Error("tool-free summary retained tool semantics", body)
						}
					}
					if body["response_format"] == nil {
						t.Error("summary grammar lost")
					}
				}
				if len(arrayValue(body["tools"])) == 0 && mode == "provider_error_final" {
					http.Error(w, "provider unavailable", http.StatusServiceUnavailable)
					return
				}
				if len(arrayValue(body["tools"])) == 0 && mode == "cancelled_final" {
					cancel()
					return
				}
				if (len(arrayValue(body["tools"])) > 0 || strings.Contains(mode, "ignores_tools")) && (mode != "distinct_targets" || physical <= 4) {
					name := goalEvaluationSubmit
					args := recoveryRoot("goal:2")
					if strings.HasPrefix(mode, "replay_plan") {
						name = goalPlanSubmit
						args = map[string]any{"goal_ref": "goal:2", "reason": fmt.Sprint(physical), "next_step": "", "wait_condition": "wait"}
					}
					if strings.HasPrefix(mode, "bad_criterion") || mode == "provider_error_final" || mode == "cancelled_final" {
						args["judgments"] = []any{map[string]any{"criterion_ref": "criterion:missing", "verdict": "unknown", "kind": "semantic", "subject": "actor_self", "discourse": "uncertain", "evidence_refs": []string{}, "reason": "unknown", "criterion_quote": "", "optional_improvement": ""}}
					}
					if mode == "distinct_targets" {
						if physical%2 == 1 {
							name = goalPlanSubmit
							args = map[string]any{"goal_ref": fmt.Sprintf("goal:%d", (physical+1)/2), "reason": "plan", "next_step": "", "wait_condition": "wait"}
						} else {
							args = recoveryRoot(fmt.Sprintf("goal:%d", physical/2))
						}
					}
					message = map[string]any{"role": "assistant", "content": "", "tool_calls": fakeProviderNativeToolCalls([]map[string]any{{"call_id": fmt.Sprint("native-", physical), "capability_name": name, "arguments": args}})}
				}
				if strings.HasSuffix(mode, "_stream") {
					w.Header().Set("Content-Type", "text/event-stream")
					calls := arrayValue(message["tool_calls"])
					for n, call := range calls {
						mapValue(call)["index"] = n
					}
					chunk := map[string]any{"choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": "stop"}}}
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(chunk))
				} else {
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": "stop"}}})
				}
			}))
			defer server.Close()
			definition, _ := FormalAgentDefinitionByID(FormalAgentGoalEvaluation)
			defs := capabilityCatalog(mustCapabilityRegistry(builtinCapabilities(&App{})...), CapabilitySurfaceGoalEvaluation)
			response, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 4096}, Role: "cognitive_assessment", Scenario: "goal_evaluation", Messages: []map[string]any{{"role": "user", "content": "frozen goals"}}, Definitions: defs, JSONMode: true, SchemaName: "goal_evaluation_v1", ResponseSchema: objectSchema(map[string]any{"summary": stringSchema()}, []string{"summary"}, false), ProviderRequestID: "recovery", CorrelationID: "recovery", Agent: definition, EnableStreaming: strings.HasSuffix(mode, "_stream")})
			if mode == "provider_error_final" || mode == "cancelled_final" {
				if err == nil || (mode == "cancelled_final" && !errors.Is(err, context.Canceled)) {
					t.Fatalf("real terminal error lost: %v", err)
				}
				if physical != 4 {
					t.Fatalf("error path repeated: %d", physical)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(mode, "ignores_tools") && (!strings.Contains(response.Message.Content, "本轮已停止重复提交") || len(response.Message.ToolCalls) != 0 || invoker.attempts != physical-1) {
				t.Fatalf("unsolicited tool call escaped final fallback: %#v attempts=%d physical=%d", response.Message, invoker.attempts, physical)
			}
			accepted, missing := goalEvaluationSubmissionCoverage(b.snapshot, invoker.records)
			if mode == "distinct_targets" {
				if progress.awaitingEvidence() || len(accepted) != 2 || physical != 5 {
					t.Fatal("progress cut short", physical, accepted)
				}
			} else {
				if !progress.awaitingEvidence() || len(missing) != 2 || physical > 5 {
					t.Fatal("did not bound unavailable assessment", physical, accepted, missing)
				}
			}
		})
	}
}
