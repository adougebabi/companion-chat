package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Exercise the real Provider/ADK transport with an in-memory RoundTripper only.
func TestOwnerLifeAcceptedWithoutToolRecovery(t *testing.T) {
	for _, kind := range []string{"wardrobe_wear", "moment_suggestion"} {
		for _, scenario := range []string{"text_then_execute", "json_then_execute", "repair_then_execute", "non_native_then_execute", "later_tool_ignored", "never_execute"} {
			t.Run(kind+"/"+scenario, func(t *testing.T) {
				action := ownerLifeActionNames(kind)[0]
				trace := &ADKCapabilityTrace{}
				invoker := adkTraceInvoker{trace: trace}
				ctx := WithADKCapabilityInvoker(context.WithValue(context.Background(), ownerLifeRequestKindKey{}, kind), invoker, trace)
				agent, _ := FormalAgentDefinitionByID(FormalAgentNativeCognition)
				calls := 0
				router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
					calls++
					accepted := fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "好的，这就为您执行。"}}
					if calls == 1 {
						if scenario == "json_then_execute" {
							return accepted
						}
						if scenario == "repair_then_execute" {
							return fakeProviderResult{Text: "我愿意帮您。"}
						}
						return fakeProviderResult{Text: "decision: accepted\nreason: 好的，这就为您执行。"}
					}
					executionCall := 2
					if scenario == "repair_then_execute" {
						executionCall++
						if calls == 2 {
							if len(arrayValue(payload["tools"])) != 0 {
								t.Error("an ambiguous decision must be repaired before requiring an action")
							}
							return accepted
						}
					}
					if calls == executionCall || (calls == executionCall+1 && (scenario == "non_native_then_execute" || scenario == "never_execute")) {
						if payload["tool_choice"] != "required" || len(arrayValue(payload["tools"])) != 1 ||
							!r6PayloadHasTool(payload, action) || payload["enable_thinking"] != false {
							t.Error("execution continuation must require just the scoped action")
						}
						if scenario == "non_native_then_execute" && calls == executionCall {
							return fakeProviderResult{Text: "<call:action/>"}
						}
						if strings.Contains(jsonString(payload["messages"]), "<call:action/>") {
							t.Error("retry reused the non-native pseudo-call")
						}
						if scenario != "never_execute" {
							return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("apply", action, map[string]any{})}}
						}
					} else {
						if len(arrayValue(payload["tools"])) != 0 {
							t.Error("committed execution must not offer more tools")
						}
						if scenario == "later_tool_ignored" {
							return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("repeat", action, map[string]any{})}}
						}
					}
					return accepted
				})
				// This invoker records authoritative fixture receipts; Moment requires a target.
				recording := &ownerLifeRecoveryInvoker{adkTraceInvoker: invoker}
				ctx = WithADKCapabilityInvoker(ctx, recording, trace)
				response, err := (&ProviderClient{HTTP: &http.Client{Transport: router}}).generateWithEino(ctx, EinoModelCall{
					Agent:      agent,
					Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: "http://fixture.invalid", ModelID: "fixture", Timeout: time.Second},
					Role:       "cognitive_assessment", Scenario: "native_cognition", Messages: []map[string]any{{"role": "user", "content": "请执行我的请求"}},
					Definitions: []CapabilityDefinition{
						{Name: capabilityCatalogName, Description: "Discover", InputSchema: objectSchema(map[string]any{}, nil, true)},
						{Name: action, Description: "Apply requested action", InputSchema: objectSchema(map[string]any{}, nil, true)},
					}, JSONMode: true, SchemaName: "owner_life_request_decision", ResponseSchema: ownerLifeDecisionSchema(), ProviderRequestID: "owner-recovery", CorrelationID: "owner-recovery",
				})
				if scenario == "never_execute" {
					if err == nil {
						t.Fatal("accepted without receipt must fail after bounded guidance")
					}
					reason := ownerRequestFailureReason(err)
					if !strings.Contains(reason, "未执行") || strings.Contains(reason, "这就") || strings.Contains(reason, "owner_") {
						t.Fatalf("unsafe failure reason: %q (%v)", reason, err)
					}
					if calls != 3 {
						t.Fatalf("unbounded guidance: %d", calls)
					}
					return
				}
				_, results := trace.Snapshot()
				applied, refs := ownerLifeEffects(kind, results)
				wantCalls := 3
				if scenario == "repair_then_execute" || scenario == "non_native_then_execute" {
					wantCalls++
				}
				if err != nil || !applied || len(refs) == 0 || len(response.ExecutedToolCalls) != 1 || calls != wantCalls {
					t.Fatalf("request did not recover with native receipts: calls=%d results=%#v response=%#v err=%v", calls, results, response, err)
				}
			})
		}
	}
}

type ownerLifeRecoveryInvoker struct{ adkTraceInvoker }

func (i *ownerLifeRecoveryInvoker) ExecuteWithID(ctx context.Context, id, name, args string) (string, error) {
	result, err := i.adkTraceInvoker.ExecuteWithID(ctx, id, name, args)
	if name == "moment.publish" {
		i.trace.Results[len(i.trace.Results)-1].Output = map[string]any{"target_kind": "moment", "target_ref": "fixture-moment"}
	}
	return result, err
}

// PostgreSQL required: verify admission, dynamic catalog loading, durable effects,
// and the public request record rather than only the Agent's final DTO.
func TestOwnerLifeWearAcceptedWithoutToolSettlement(t *testing.T) {
	for _, execute := range []bool{true, false} {
		name := "never_execute"
		if execute {
			name = "text_then_execute"
		}
		t.Run(name, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			boots := wardrobeItemID(t, f, "黑色短靴")
			revision := wardrobeRevision(t, f)
			calls := 0
			applied := false
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				if calls == 1 {
					return fakeProviderResult{Text: "decision: accepted\nreason: 好的，这就为您换上。"}
				}
				if execute && !applied {
					if strings.Contains(jsonString(payload["messages"]), "Do not call tools") {
						t.Error("required execution prohibited")
					}
					if !r6PayloadHasTool(payload, wardrobeWearCapabilityName) {
						if !r6PayloadHasTool(payload, capabilityCatalogName) {
							t.Fatal("missing catalog")
						}
						return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("load-wear", capabilityCatalogName, map[string]any{"names": []string{wardrobeWearCapabilityName}})}}
					}
					applied = true
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("wear", wardrobeWearCapabilityName, map[string]any{})}}
				}
				return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "好的，这就为您换上。"}}
			})
			f.app.Provider.HTTP = &http.Client{Transport: router}
			seedCognitiveProviderRole(t, f.ctx, f.repository, "owner-recovery-"+f.suffix)
			admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/wardrobe/wear", nil, f.ownerID, map[string]any{"item_id": boots, "slot": "shoes", "expected_revision": revision, "reason": "请穿上靴子", "idempotency_key": "owner-recovery-" + name})
			if err != nil {
				t.Fatal(err)
			}
			request := mapValue(admitted["request"])
			if _, err = f.app.ProcessCognitionInbox(f.ctx, stringValue(request["source_fact_id"])); err != nil {
				t.Fatal(err)
			}
			read, err := f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/owner-requests/"+stringValue(request["id"]), nil, f.ownerID, nil)
			if err != nil {
				t.Fatal(err)
			}
			result := mapValue(read["result"])
			if execute {
				if read["status"] != "completed" || result["mutation_committed"] != true || len(arrayValue(read["result_refs"])) == 0 || wardrobeRevision(t, f) != revision+1 {
					t.Fatalf("missing committed result: %#v", read)
				}
			} else {
				reason := stringValue(read["decision_reason"])
				if read["status"] != "failed" || result["mutation_committed"] != false || len(arrayValue(read["result_refs"])) != 0 || wardrobeRevision(t, f) != revision || !strings.Contains(reason, "未执行") || reason != result["reason"] || strings.Contains(reason, "这就") || strings.Contains(reason, "owner_") {
					t.Fatalf("misleading failure: %#v", read)
				}
			}
		})
	}
}

func TestOwnerLifeExecutionRepairPreservesCommittedEffectsAndDecisions(t *testing.T) {
	for _, scenario := range []string{"committed", "no_change", "rejected", "deferred", "fenced_rejected"} {
		t.Run(scenario, func(t *testing.T) {
			trace := &ADKCapabilityTrace{}
			if scenario == "committed" {
				trace.AppendResult(CapabilityResult{CapabilityName: wardrobeWearCapabilityName, CallID: "prior", Status: "completed"})
			}
			if scenario == "no_change" {
				trace.AppendResult(CapabilityResult{CapabilityName: wardrobeWearCapabilityName, CallID: "prior", Status: "handled", Output: map[string]any{"disposition": "no_change", "warning_code": "wearing_unchanged"}})
			}
			ctx := WithADKCapabilityInvoker(context.WithValue(context.Background(), ownerLifeRequestKindKey{}, "wardrobe_wear"), &adkFakeInvoker{}, trace)
			calls := 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				if scenario == "fenced_rejected" {
					return fakeProviderResult{Text: "```json\n{\"decision\":\"rejected\",\"reason\":\"暂时不想换装\"}\n```"}
				}
				if scenario == "rejected" || scenario == "deferred" {
					return fakeProviderResult{Structured: map[string]any{"decision": scenario, "reason": "暂时不想换装"}}
				}
				if calls == 1 {
					return fakeProviderResult{Text: "decision: accepted\nreason: 已经处理"}
				}
				if len(arrayValue(payload["tools"])) != 0 || !strings.Contains(jsonString(payload["messages"]), "Do not call tools") {
					t.Error("format repair can repeat a committed effect")
				}
				return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "已经处理"}}
			})
			agent, _ := FormalAgentDefinitionByID(FormalAgentNativeCognition)
			_, err := (&ProviderClient{HTTP: &http.Client{Transport: router}}).generateWithEino(ctx, EinoModelCall{
				Agent: agent, Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: "http://fixture.invalid", ModelID: "fixture", Timeout: time.Second},
				Role: "cognitive_assessment", Scenario: "native_cognition", Messages: []map[string]any{{"role": "user", "content": "换装"}},
				Definitions: []CapabilityDefinition{{Name: wardrobeWearCapabilityName, InputSchema: objectSchema(map[string]any{}, nil, true)}},
				JSONMode:    true, SchemaName: "owner_life_request_decision", ResponseSchema: ownerLifeDecisionSchema(),
			})
			want := 2
			if scenario == "rejected" || scenario == "deferred" || scenario == "fenced_rejected" {
				want = 1
			}
			if err != nil || calls != want {
				t.Fatalf("unexpected continuation: calls=%d err=%v", calls, err)
			}
		})
	}
}
