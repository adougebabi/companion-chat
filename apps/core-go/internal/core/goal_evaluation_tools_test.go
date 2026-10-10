package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestGoalEvaluationPrivateToolCatalogAndClosedArguments(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(&App{})...)
	for _, surface := range []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition, CapabilitySurfaceAutonomy, CapabilitySurfaceGoalPlanner, CapabilitySurfaceReflection} {
		for _, d := range registry.Catalog(surface) {
			if isGoalEvaluationTool(d.Name) || d.Name == "goal.decide" {
				t.Fatalf("state mutation leaked to %s: %s", surface, d.Name)
			}
		}
	}
	for _, d := range registry.Catalog(CapabilitySurfaceGoalEvaluation) {
		if !isGoalEvaluationTool(d.Name) {
			t.Fatal("unexpected task-private Tool", d.Name)
		}
		args := map[string]any{"goal_ref": "goal:1", "judgments": []any{}, "impact": "needs_evidence", "blocker": "", "wait_condition": "wait", "next_step": "", "residual_motivation": ""}
		if d.Name != goalEvaluationSubmit {
			continue
		}
		inv := CapabilityInvocation{CallID: "schema", CapabilityName: d.Name, SchemaVersion: CapabilityInvocationSchemaVersion, ProviderRequestID: "physical-schema", Arguments: jsonBytes(args)}
		if err := inv.Validate(d); err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"goal_id", "expected_revision", "claim_revision", "stage_evaluation", "commitment_evaluations", "status"} {
			bad := cloneMap(args)
			bad[forbidden] = "forged"
			inv.Arguments = jsonBytes(bad)
			if err := inv.Validate(d); err == nil {
				t.Fatal("accepted model-owned authority", forbidden)
			}
		}
	}
}

func TestGoalEvaluationPrivateSessionCannotBeForgedByDirectOrOtherAgent(t *testing.T) {
	input := richGoalEvaluationWireSnapshot()
	input.ClaimRevision = 8
	binding, err := newGoalEvaluationWireBinding(input)
	if err != nil {
		t.Fatal(err)
	}
	session := &goalEvaluationSession{binding: binding, runID: "request-secret:claim:8"}
	definition, _ := FormalAgentDefinitionByID(FormalAgentGoalEvaluation)
	ctx := context.WithValue(withFormalAgentDefinition(context.Background(), definition), goalEvaluationSessionKey{}, session)
	request := ToolExecutionRequest{AgentID: FormalAgentGoalEvaluation, RunID: session.runID, CapabilityName: goalEvaluationSubmit, OperationID: "native-op", NativeToolCallID: "native-7", ProviderRequestID: "physical-7", AuthorizationActorID: input.OwnerActorID, FluctlightID: input.FluctlightID, WorkingProfileID: input.ProfileID, TargetKind: "goal_evaluation_run", TargetRef: input.RequestID, EvidenceID: input.RequestID, Surface: CapabilitySurfaceGoalEvaluation, Arguments: json.RawMessage(`{}`)}
	if err := authorizeGoalEvaluationTool(ctx, request); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*ToolExecutionRequest){
		func(r *ToolExecutionRequest) { r.NativeToolCallID = "" }, func(r *ToolExecutionRequest) { r.ProviderRequestID = "" }, func(r *ToolExecutionRequest) { r.Surface = CapabilitySurfaceWakeUp },
		func(r *ToolExecutionRequest) { r.AgentID = FormalAgentConversationCognition }, func(r *ToolExecutionRequest) { r.RunID = "other-claim" }, func(r *ToolExecutionRequest) { r.WorkingProfileID = "other-profile" },
		func(r *ToolExecutionRequest) { r.AuthorizationActorID = "other-owner" }, func(r *ToolExecutionRequest) { r.TargetRef = "other-request" }, func(r *ToolExecutionRequest) { r.EvidenceID = "other-source" },
	} {
		bad := request
		change(&bad)
		if err := authorizeGoalEvaluationTool(ctx, bad); !errors.Is(err, ErrUnauthorized) {
			t.Fatal("forged authority accepted", err)
		}
	}
	for _, name := range []string{goalEvaluationSubmit, goalObjectSubmit, goalPlanSubmit} {
		bad := request
		bad.CapabilityName = name
		if _, err := (&App{}).ExecuteTool(context.Background(), bad); !errors.Is(err, ErrUnauthorized) {
			t.Fatal("public request reached receipt/database path", name, err)
		}
	}
}

func TestGoalEvaluationCoverageRequiresDurableRootSubmissions(t *testing.T) {
	snapshot := twoGoalWireSnapshot()
	records := map[string]goalSubmissionRecord{}
	// A plan/object and assistant prose cannot cover root assessments.
	records[goalSubmissionKey(goalPlanSubmit, snapshot.Goals[0].GoalID, "")] = goalSubmissionRecord{Output: map[string]any{"status": "completed"}}
	records[goalSubmissionKey(goalObjectSubmit, snapshot.Goals[1].GoalID, "stage:2.1")] = goalSubmissionRecord{}
	accepted, missing := goalEvaluationSubmissionCoverage(snapshot, records)
	if len(accepted) != 0 || len(missing) != 2 {
		t.Fatal(accepted, missing)
	}
	records[goalSubmissionKey(goalEvaluationSubmit, snapshot.Goals[0].GoalID, "")] = goalSubmissionRecord{Digest: "real-submit", Output: map[string]any{"status": "completed"}}
	var restored map[string]goalSubmissionRecord
	if err := json.Unmarshal(jsonBytes(records), &restored); err != nil {
		t.Fatal(err)
	}
	accepted, missing = goalEvaluationSubmissionCoverage(snapshot, restored)
	if len(accepted) != 1 || accepted[0] != snapshot.Goals[0].GoalID || len(missing) != 1 {
		t.Fatal("partial root lost during recovery", accepted, missing)
	}
	for _, name := range []string{goalEvaluationSubmit, goalObjectSubmit, goalPlanSubmit} {
		if goalOutcomeCarriesEvidence(ActionOutcome{CapabilityName: name, Status: ActionOutcomeCompleted}) {
			t.Fatal("evaluation acknowledgement became business proof", name)
		}
	}
}

func TestGoalEvaluationStaleReplacementRetainsDeferredTargets(t *testing.T) {
	snapshot := goalEvaluationSnapshot{DeferredGoalIDs: []string{"deferred", "unresolved"}}
	targets := goalEvaluationReplacementTargets(snapshot, map[string]any{"evaluated_goals": []string{"completed"}, "unresolved_goals": []string{"unresolved"}})
	if len(targets) != 2 || !containsString(targets, "deferred") || !containsString(targets, "unresolved") || containsString(targets, "completed") {
		t.Fatal("lost deferred root or requeued completed sibling", targets)
	}
}

// Controlled invoker tests native protocol + domain rules without claiming
// PostgreSQL persistence coverage. The isolated database tests exercise that.
type goalSemanticTestInvoker struct {
	binding *goalEvaluationWireBinding
	goals   map[string]GoalAuthority
	ids     []string
	errors  []string
}

func (i *goalSemanticTestInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return i.ExecuteWithID(ctx, "", name, args)
}
func (i *goalSemanticTestInvoker) ExecuteWithID(_ context.Context, id, name, args string) (string, error) {
	if id == "" {
		return "", errors.New("native identity required")
	}
	i.ids = append(i.ids, id)
	var value map[string]any
	_ = json.Unmarshal([]byte(args), &value)
	entry := i.binding.goalsByRef[stringValue(value["goal_ref"])]
	sources := map[string]GoalSource{}
	for _, s := range i.binding.snapshot.Sources {
		sources[s.Ref] = s
	}
	var err error
	if name == goalEvaluationSubmit {
		out, e := i.binding.hydrateOutput(map[string]any{"evaluations": []any{value}, "plans": []any{}})
		err = e
		if err == nil {
			next, _, _, e := ApplyGoalEvaluation(i.goals[entry.GoalID], out.Evaluations[0], sources, time.Now())
			err = e
			if err == nil {
				i.goals[entry.GoalID] = next
			}
		}
	} else {
		wire := cloneMap(value)
		delete(wire, "goal_ref")
		delete(wire, "object_kind")
		var candidate goalEvaluationWireObjectEvaluation
		_ = json.Unmarshal(jsonBytes(wire), &candidate)
		object, e := i.binding.hydrateObjectEvaluation(entry.GoalID, stringValue(value["object_kind"]), candidate)
		err = e
		if err == nil {
			_, err = validateGoalObjectEvaluation(entry.Goal, *object, stringValue(value["object_kind"]), entry.Stages, entry.Commitments, sources)
		}
	}
	if err != nil {
		i.errors = append(i.errors, err.Error())
		return jsonString(map[string]any{"status": "failed", "error_code": err.Error()}), nil
	}
	return jsonString(map[string]any{"status": "completed", "output": map[string]any{"goal_ref": value["goal_ref"], "status": i.goals[entry.GoalID].Status}}), nil
}

func TestGoalEvaluationNativeHTTPRejectsStageAndCompletesBothRoots(t *testing.T) {
	snapshot := twoGoalWireSnapshot()
	for n := range snapshot.Goals {
		g := &snapshot.Goals[n].Goal
		g.Scope = "general"
		g.TargetActorID = ""
		snapshot.Goals[n].TargetActorID = ""
		g.SchemaVersion = goalAuthoritySchemaVersion
		g.Ref = "goal:ctx_" + stableDigest(g.EntityID)
		g.CriterionIDs = []string{fmt.Sprintf("criterion_test_%d", n)}
		g.Deadline = nil
		g.DeadlinePolicy = "soft"
		g.CriteriaPolicy = map[string]any{"mode": "all"}
		g.SuccessCriteria = g.SuccessCriteria[:1]
		g.CriterionIDs = g.CriterionIDs[:1]
	}
	snapshot.Sources[0].GoalIDs = nil
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	root := func(ref, criterion string) map[string]any {
		return map[string]any{"goal_ref": ref, "judgments": []any{map[string]any{"criterion_ref": criterion, "criterion_quote": "", "optional_improvement": "", "verdict": "satisfied", "kind": "communication", "subject": "actor_self", "discourse": "assertion", "evidence_refs": []string{"e1"}, "reason": "actual message"}}, "impact": "completed", "blocker": "", "wait_condition": "", "next_step": "目标完成，等待反馈属于后续交流", "residual_motivation": ""}
	}
	stage := map[string]any{"goal_ref": "goal:1", "object_kind": "stage", "object_ref": "stage:1.1", "completed": true, "reason": "assistant reported information", "judgments": []any{map[string]any{"criterion_ref": "criterion:1.s1.1", "criterion_quote": "", "optional_improvement": "", "verdict": "satisfied", "kind": "information", "subject": "actor_self", "discourse": "assertion", "evidence_refs": []string{"e1"}, "reason": "self-report"}}}
	calls := []map[string]any{{"call_id": "stage-invalid", "capability_name": goalObjectSubmit, "arguments": stage}, {"call_id": "root-one", "capability_name": goalEvaluationSubmit, "arguments": root("goal:1", "criterion:1.1")}, {"call_id": "root-two", "capability_name": goalEvaluationSubmit, "arguments": root("goal:2", "criterion:2.1")}}
	physical := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["enable_thinking"] != false {
			t.Error("thinking enabled")
		}
		if physical > 0 && !strings.Contains(jsonString(payload["messages"]), "stage-invalid") {
			t.Error("native result pairing lost")
		}
		if physical == 1 && !strings.Contains(jsonString(payload["messages"]), "goal_judgment_self_report_not_business_fact") {
			t.Error("rejection never reached next decision")
		}
		message := map[string]any{"role": "assistant", "content": ""}
		if physical < len(calls) {
			message["tool_calls"] = fakeProviderNativeToolCalls([]map[string]any{calls[physical]})
		} else {
			message["content"] = jsonString(map[string]any{"summary": "consult real submissions"})
		}
		physical++
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	invoker := &goalSemanticTestInvoker{binding: binding, goals: map[string]GoalAuthority{}}
	for _, entry := range snapshot.Goals {
		invoker.goals[entry.GoalID] = entry.Goal
	}
	trace := &ADKCapabilityTrace{}
	ctx := WithADKCapabilityInvoker(context.Background(), invoker, trace)
	p := &ProviderClient{HTTP: server.Client()}
	defs := capabilityCatalog(mustCapabilityRegistry(builtinCapabilities(&App{})...), CapabilitySurfaceGoalEvaluation)
	_, err = p.generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 4096}, Role: "cognitive_assessment", Scenario: "goal_evaluation", Messages: []map[string]any{{"role": "user", "content": "frozen evaluation"}}, Definitions: defs, JSONMode: true, SchemaName: "goal_evaluation_v1", ResponseSchema: objectSchema(map[string]any{"summary": stringSchema()}, []string{"summary"}, false), ProviderRequestID: "physical-goal", CorrelationID: "goal-request"})
	if err != nil {
		t.Fatal(err)
	}
	if physical != 4 || len(invoker.ids) != 3 || len(invoker.errors) != 1 || invoker.errors[0] != "goal_judgment_self_report_not_business_fact" {
		t.Fatal(physical, invoker.ids, invoker.errors)
	}
	for _, g := range invoker.goals {
		if g.Status != GoalCompleted || g.Progress != 1 {
			t.Fatal("root remained incomplete after separate stage rejection", g.Status, g.Progress)
		}
	}
}

type goalRequestPolicyInvoker struct {
	mu       sync.Mutex
	accepted int
	rejected bool
}

func (i *goalRequestPolicyInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return i.ExecuteWithID(ctx, "", name, args)
}

func (i *goalRequestPolicyInvoker) ExecuteWithID(_ context.Context, id, name, _ string) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if id == "" || name != goalEvaluationSubmit {
		return "", errors.New("unexpected native call")
	}
	if !i.rejected {
		i.rejected = true
		return jsonString(map[string]any{"status": "failed", "error_code": "controlled_rejection"}), nil
	}
	i.accepted++
	return jsonString(map[string]any{"status": "completed", "output": map[string]any{"accepted": true}}), nil
}

func (i *goalRequestPolicyInvoker) coverage() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.accepted
}

func TestGoalEvaluationPhysicalRequestPolicyRequiresNativeRootsBeforeSummary(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, initialCoverage := range []int{0, 1} {
			name := fmt.Sprintf("stream=%t/accepted=%d", streaming, initialCoverage)
			t.Run(name, func(t *testing.T) {
				invoker := &goalRequestPolicyInvoker{accepted: initialCoverage}
				defs := capabilityCatalog(mustCapabilityRegistry(builtinCapabilities(&App{})...), CapabilitySurfaceGoalEvaluation)
				trace := &ADKCapabilityTrace{}
				ctx := WithADKCapabilityInvoker(context.Background(), invoker, trace)
				ctx = withPhysicalModelRequestPolicy(ctx, physicalModelRequestPolicyFunc(func(context.Context) (physicalModelRequestDecision, error) {
					if invoker.coverage() < 2 {
						return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true}, nil
					}
					return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceAllowed}, nil
				}))
				var mu sync.Mutex
				requests := []map[string]any{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					idempotencyID := r.Header.Get("Idempotency-Key")
					if idempotencyID == "" || r.Header.Get("X-Fluctlight-Provider-Request-Id") != idempotencyID {
						t.Error("physical request identity headers missing or inconsistent")
					}
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
						return
					}
					mu.Lock()
					requests = append(requests, payload)
					mu.Unlock()
					toolNames := []string{}
					for _, raw := range arrayValue(payload["tools"]) {
						function := mapValue(mapValue(raw)["function"])
						toolNames = append(toolNames, stringValue(function["name"]))
						parameters := mapValue(function["parameters"])
						if parameters["additionalProperties"] != false || len(arrayValue(parameters["required"])) == 0 {
							t.Error("canonical closed Tool schema lost required fields", function["name"])
						}
					}
					sort.Strings(toolNames)
					wantNames := []string{goalEvaluationSubmit, goalObjectSubmit, goalPlanSubmit}
					sort.Strings(wantNames)
					canonicalTools := strings.Join(toolNames, ",") == strings.Join(wantNames, ",")
					coverage := invoker.coverage()
					if payload["enable_thinking"] != false {
						t.Error("Goal Evaluation thinking policy changed", payload["enable_thinking"])
					}
					if boolValue(payload["stream"]) != streaming {
						t.Errorf("physical streaming wire mismatch: stream=%#v want=%t", payload["stream"], streaming)
					}
					if len(requests) > 1 && coverage == initialCoverage && !strings.Contains(jsonString(payload["messages"]), "controlled_rejection") {
						t.Error("rejected root feedback missing before retry")
					}
					required := payload["tool_choice"] == "required"
					_, hasFormat := payload["response_format"]
					message := map[string]any{"role": "assistant", "content": jsonString(map[string]any{"summary": "premature"})}
					finish := "stop"
					if coverage < 2 && canonicalTools && required && !hasFormat {
						callID := fmt.Sprintf("root-%d-%d", coverage, len(requests))
						message = map[string]any{"role": "assistant", "content": "", "tool_calls": fakeProviderNativeToolCalls([]map[string]any{{"call_id": callID, "capability_name": goalEvaluationSubmit, "arguments": map[string]any{"goal_ref": fmt.Sprintf("goal:%d", coverage+1), "judgments": []any{}, "impact": "needs_evidence", "blocker": "", "wait_condition": "wait", "next_step": "wait", "residual_motivation": ""}}})}
						finish = "tool_calls"
					}
					if streaming {
						w.Header().Set("Content-Type", "text/event-stream")
						delta := cloneMap(message)
						delete(delta, "role")
						if calls := arrayValue(delta["tool_calls"]); len(calls) > 0 {
							for index := range calls {
								mapValue(calls[index])["index"] = index
							}
						}
						chunk := map[string]any{"id": "goal-policy", "model": "fake", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
						_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(chunk))
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": finish}}})
				}))
				defer server.Close()
				definition, _ := FormalAgentDefinitionByID(FormalAgentGoalEvaluation)
				response, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, EinoModelCall{
					Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 4096},
					Role:       "cognitive_assessment", Scenario: "goal_evaluation", Messages: []map[string]any{{"role": "user", "content": "submit both roots"}},
					Definitions: defs, JSONMode: true, SchemaName: "goal_evaluation_v1", ResponseSchema: objectSchema(map[string]any{"summary": stringSchema()}, []string{"summary"}, false),
					ProviderRequestID: "goal-policy", CorrelationID: "goal-policy-run", Agent: definition, EnableStreaming: streaming,
				})
				if err != nil {
					t.Fatal(err)
				}
				if response.Message == nil || invoker.coverage() != 2 {
					t.Fatalf("premature final: coverage=%d response=%#v requests=%#v", invoker.coverage(), response.Message, requests)
				}
				for index, payload := range requests {
					isFinal := index == len(requests)-1
					_, hasFormat := payload["response_format"]
					if isFinal {
						if payload["tool_choice"] != "auto" || !hasFormat {
							t.Fatalf("final request did not restore summary contract: %#v", payload)
						}
					} else if payload["tool_choice"] != "required" || hasFormat {
						t.Fatalf("submission request allowed premature final: %#v", payload)
					}
				}
			})
		}
	}
}

func TestPhysicalModelRequestPolicyIsSuppressedForFinalRepair(t *testing.T) {
	called := false
	ctx := withPhysicalModelRequestPolicy(context.Background(), physicalModelRequestPolicyFunc(func(context.Context) (physicalModelRequestDecision, error) {
		called = true
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true}, nil
	}))
	format := map[string]any{"type": "json_schema"}
	opts, effective, err := applyPhysicalModelRequestPolicy(withoutPhysicalModelRequestPolicy(ctx), nil, format)
	if err != nil || called || len(opts) != 0 || effective["type"] != "json_schema" {
		t.Fatalf("tool-free repair inherited business request policy: called=%t opts=%d effective=%#v err=%v", called, len(opts), effective, err)
	}
}

func TestGoalEvaluationFixtureRoutesExecutionWithoutFinalFormat(t *testing.T) {
	called := 0
	router := newFakeProviderRouter().onGoalEvaluation(func(map[string]any) fakeProviderResult {
		called++
		return fakeProviderResult{Structured: map[string]any{"evaluations": []any{map[string]any{"goal_ref": "goal:1", "judgments": []any{}, "impact": "needs_evidence", "blocker": "", "wait_condition": "wait", "next_step": "", "residual_motivation": ""}}, "plans": []any{}}}
	})
	defs := capabilityCatalog(mustCapabilityRegistry(builtinCapabilities(&App{})...), CapabilitySurfaceGoalEvaluation)
	payload := map[string]any{"messages": []any{map[string]any{"role": "user", "content": "evaluate frozen goals"}}, "tools": RenderCapabilityTools(defs), "tool_choice": "required"}
	request := httptest.NewRequest(http.MethodPost, "http://fake/chat/completions", strings.NewReader(jsonString(payload)))
	response, err := router.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	message := mapValue(mapValue(arrayValue(body["choices"])[0])["message"])
	if called != 1 || len(arrayValue(message["tool_calls"])) != 1 {
		t.Fatalf("execution phase reached wrong fixture route: called=%d message=%#v", called, message)
	}
}

func TestGoalEvaluationFrozenConflictCanExitButJudgmentRejectionCannot(t *testing.T) {
	for _, tc := range []struct {
		code  string
		fresh bool
	}{
		{"goal_evaluation_source_stale", true},
		{"goal_submission_authority_stale", true},
		{"life_context_stale", true},
		{"goal_judgment_self_report_not_business_fact", false},
		{"invalid_arguments", false},
	} {
		t.Run(tc.code, func(t *testing.T) {
			trace := &ADKCapabilityTrace{}
			trace.AppendResult(CapabilityResult{CallID: "failed-native", CapabilityName: goalEvaluationSubmit, Status: "failed", ErrorCode: tc.code})
			ctx := WithADKCapabilityInvoker(context.Background(), adkFailingInvoker{}, trace)
			if actual := goalEvaluationTraceNeedsFreshSnapshot(ctx); actual != tc.fresh {
				t.Fatalf("frozen recovery=%t want=%t", actual, tc.fresh)
			}
		})
	}
}
