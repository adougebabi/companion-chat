package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func r5MomentFixture(t *testing.T, suffix string, router *fakeProviderRouter) (context.Context, *PostgresRepository, *App, string, string, string) {
	t.Helper()
	transport := projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		request.Body = io.NopCloser(strings.NewReader(string(raw)))
		response, err := router.RoundTrip(request)
		if err != nil || decodeObject(raw)["stream"] != true || response.StatusCode != http.StatusOK {
			return response, err
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			return nil, err
		}
		value := decodeObject(body)
		choice := mapValue(arrayValue(value["choices"])[0])
		message := mapValue(choice["message"])
		for index, tool := range arrayValue(message["tool_calls"]) {
			mapValue(tool)["index"] = index
		}
		choice["delta"] = message
		delete(choice, "message")
		response = embeddingHTTPResponse(request, http.StatusOK, "data: "+jsonString(value)+"\n\ndata: [DONE]\n\n")
		response.Header.Set("Content-Type", "text/event-stream")
		return response, nil
	})
	ctx, repo, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "r5-"+suffix, transport)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, self, conversation); err != nil {
		t.Fatal(err)
	}
	config := defaultConversationReplySettings()
	config.Mode = "fact_only"
	config.RetryLimit = 0
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json`, conversationReplySettingKey, jsonString(config)); err != nil {
		t.Fatal(err)
	}
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "r5-writer", Timeout: 3 * time.Second})
	return ctx, repo, app, owner, self, conversation
}

func TestR5MomentPublicationStopsRepeatGenerateAndStream(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, hostile := range []bool{false, true} {
			t.Run(fmt.Sprint(stream, hostile), func(t *testing.T) {
				cognition, writers := 0, 0
				router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
					if payload["model"] == "r5-writer" {
						writers++
						if strings.Contains(jsonString(payload["messages"]), "面向动态参与者") {
							return fakeProviderResult{Text: "受控测试中的动态正文。"}
						}
						return fakeProviderResult{Text: "动态已经发布。"}
					}
					cognition++
					if cognition > 2 {
						t.Error("conversation reopened satisfied publication")
						return fakeProviderResult{Status: 400}
					}
					name, args := "moment.publish", map[string]any{}
					if cognition == 2 {
						tools := arrayValue(payload["tools"])
						hasReply := false
						for _, tool := range tools {
							name := stringValue(mapValue(mapValue(tool)["function"])["name"])
							if capabilitycontract.CapabilityNamesMatch(name, "moment.publish") {
								t.Error("completed Moment still exposed")
							}
							hasReply = hasReply || capabilitycontract.CapabilityNamesMatch(name, conversationReplyCapabilityName)
						}
						if !hasReply {
							t.Error("private reply no longer available")
						}
						if !hostile {
							name = conversationReplyCapabilityName
						}
					}
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("fresh-native-%d", cognition), capabilitycontract.CapabilityWireName(name), args)}}
				})
				ctx, repo, app, owner, self, conversation := r5MomentFixture(t, fmt.Sprint(stream, hostile), router)
				result, err := app.handleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": "能发条动态吗？", "idempotency_key": "source", "turn_id": "source", "attachment_refs": []any{}}, turnCallbacks{}, stream)
				if hostile {
					if err == nil {
						t.Fatal("ignored acknowledgement catalog claimed private success")
					}
				} else if err != nil || stringValue(result.Assistant["text"]) != "动态已经发布。" {
					t.Fatal("acknowledgement failed", result, err)
				}
				var moments, outbox, replies int
				if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, self).Scan(&moments); err != nil {
					t.Fatal(err)
				}
				if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind='moment.published'`, self).Scan(&outbox); err != nil {
					t.Fatal(err)
				}
				if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&replies); err != nil {
					t.Fatal(err)
				}
				wantWriters, wantReplies := 2, 1
				if hostile {
					wantWriters, wantReplies = 1, 0
				}
				if moments != 1 || outbox != 1 || replies != wantReplies || writers != wantWriters || cognition != 2 {
					t.Fatalf("moments=%d outbox=%d replies=%d writers=%d cognition=%d err=%v", moments, outbox, replies, writers, cognition, err)
				}
			})
		}
	}
}

func TestR5MomentFreshNativeCoordinatesReplaySameSourceBeforeWriter(t *testing.T) {
	writers := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		writers++
		return fakeProviderResult{Text: "当前来源的唯一动态。"}
	})
	base, repo, app, owner, self, conversation := r5MomentFixture(t, "replay", router)
	first, err := app.AcceptTurn(base, owner, conversation, map[string]any{"fluctlight_id": self, "text": "发一条动态", "turn_id": "first-source", "idempotency_key": "first-source", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	invoke := func(source, root, call, args string, sequence uint64) (CapabilityResult, error) {
		ctx, err := app.withPrivateReplyRun(base, ToolExecutionRequest{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, EvidenceID: source, RunID: root, OperationID: root, Surface: CapabilitySurfaceConversation}, "发一条动态")
		if err != nil {
			return CapabilityResult{}, err
		}
		projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, SourceFactID: source, FluctlightID: self, ConversationID: conversation, CurrentUserText: "发一条动态", MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
		if err != nil {
			return CapabilityResult{}, err
		}
		trace := &ADKCapabilityTrace{}
		codec, err := newProviderContextRefCodec(projection.ReferenceIndex)
		if err != nil {
			return CapabilityResult{}, err
		}
		invoker := newAppADKCapabilityInvokerForContext(ctx, app, ADKCapabilityRequest{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, SourceFactID: source, OperationID: root, Surface: CapabilitySurfaceConversation, Projection: projection}, trace)
		ctx = withADKCapabilityContextRefs(ctx, invoker, trace, nil, codec)
		trace.RecordModelToolCalls("physical:"+call, sequence, []schema.ToolCall{{ID: call}})
		_, err = invoker.(ADKCapabilityInvokerWithID).ExecuteWithID(ctx, call, "moment.publish", args)
		result, _ := trace.FindResult(call)
		return result, err
	}
	a, err := invoke(first.InboxID, "run-a", "native-a", `{"evidence_refs":null}`, 1)
	if err != nil || a.Status != "completed" {
		t.Fatal(a, err)
	}
	life := currentLifeForTest(t, base, app, self, app.now().UTC())
	if _, err := app.SetPresence(base, owner, self, map[string]any{"current_task": "发布后的新生活记录", "user_presence": "online", "expires_at": app.now().UTC().Add(time.Hour).Format(time.RFC3339), "idempotency_key": "after-publication", "expected_life_context_revision": life["context_revision"]}); err != nil {
		t.Fatal(err)
	}
	b, err := invoke(first.InboxID, "run-b-retry", "native-b", `{"evidence_refs":[]}`, 99)
	if err != nil || b.Status != "completed" || mapValue(a.Output)["target_ref"] != mapValue(b.Output)["target_ref"] || mapValue(b.Output)["replayed"] != true || writers != 1 {
		t.Fatal("source replay regenerated/published", a, b, writers, err)
	}
	second, err := app.AcceptTurn(base, owner, conversation, map[string]any{"fluctlight_id": self, "text": "再发一条新的动态", "turn_id": "second-source", "idempotency_key": "second-source", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	c, err := invoke(second.InboxID, "run-c", "native-c", `{}`, 1)
	if err != nil || c.Status != "completed" || mapValue(c.Output)["target_ref"] == mapValue(a.Output)["target_ref"] || writers != 2 {
		t.Fatal("new source was incorrectly suppressed", c, writers, err)
	}
	var moments int
	if err := repo.Pool().QueryRow(base, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, self).Scan(&moments); err != nil || moments != 2 {
		t.Fatal(moments, err)
	}
}

func TestR5MomentObligationDoesNotFakeReplyOrCloseMediaActions(t *testing.T) {
	moment := CapabilityResult{CapabilityName: "moment.publish", Status: "completed", Output: map[string]any{"target_kind": "moment", "target_ref": "actual-moment"}}
	trace := &ADKCapabilityTrace{Results: []CapabilityResult{moment}}
	if _, committed := committedMomentResult(trace); !committed || conversationHasAuthoritativeOutput(trace) {
		t.Fatal("Moment became a private reply")
	}
	trace.Results = append(trace.Results, CapabilityResult{CapabilityName: conversationReplyCapabilityName, Status: "accepted", Output: map[string]any{"draft_ref": "actual-draft", "reply_status": "needs_action"}})
	ctx := withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace}, trace, nil)
	decision, err := (conversationPublicationPhysicalRequestPolicy{}).DecidePhysicalModelRequest(ctx)
	if err != nil || decision.AllowedToolNames != nil || len(decision.ExcludedToolNames) != 1 {
		t.Fatal("real media action catalog closed", decision, err)
	}
	ctx = withPhysicalModelRequestPolicy(ctx, conversationPublicationPhysicalRequestPolicy{})
	if err := validateConversationPublicationCorrection(ctx, &schema.Message{ToolCalls: []schema.ToolCall{{Function: schema.FunctionCall{Name: capabilitycontract.CapabilityWireName("moment.publish")}}}}); err == nil {
		t.Fatal("Provider ignored exclusion and reopened publication")
	}
	trace.Results = append(trace.Results, CapabilityResult{CapabilityName: conversationReplyCapabilityName, Status: "accepted", Output: map[string]any{"draft_ref": "actual-draft", "reply_status": "waiting"}})
	if !conversationHasAuthoritativeOutput(trace) {
		t.Fatal("accepted waiting handoff reopened")
	}
	trace.Results = []CapabilityResult{{CapabilityName: "moment.publish", Status: "accepted", Output: moment.Output}}
	if _, committed := committedMomentResult(trace); committed {
		t.Fatal("uncommitted Moment closed action")
	}
	request := ADKCapabilityRequest{Surface: CapabilitySurfaceConversation, ConversationID: "C", SourceFactID: "S", ActionID: "A"}
	if conversationMomentOperationID(request, "physical1") != conversationMomentOperationID(request, "physical2") {
		t.Fatal("physical IDs entered business identity")
	}
	first := conversationMomentOperationID(request, "physical1")
	request.TargetKind, request.TargetRef = "conversation", "C"
	if conversationMomentOperationID(request, "physical2") != first {
		t.Fatal("default conversation target changed logical action")
	}
	request.ActionID = "another-admitted-action"
	if conversationMomentOperationID(request, "physical2") == first {
		t.Fatal("independent admitted action was suppressed")
	}
	request.TargetKind, request.TargetRef = "owner_moment_request", "explicit-owner-request"
	if conversationMomentOperationID(request, "physical2") != "physical2" {
		t.Fatal("explicit target identity changed")
	}
	request.Surface = CapabilitySurfaceWakeUp
	if conversationMomentOperationID(request, "physical1") != "physical1" {
		t.Fatal("autonomous identity changed")
	}
}
