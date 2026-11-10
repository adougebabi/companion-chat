package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func assertDirectReplyWireDescriptions(t *testing.T, payload map[string]any) {
	t.Helper()
	if _, found := payload["response_format"]; found {
		t.Fatal("native tool request unexpectedly included final response_format")
	}
	for _, key := range []string{"max_tokens", "max_completion_tokens"} {
		if _, found := payload[key]; found {
			t.Fatalf("native tool request unexpectedly included %s", key)
		}
	}
	var function map[string]any
	for _, raw := range arrayValue(payload["tools"]) {
		candidate := mapValue(mapValue(raw)["function"])
		if strings.ReplaceAll(stringValue(candidate["name"]), ".", "_") == "conversation_reply" {
			function = candidate
			break
		}
	}
	parameters := mapValue(function["parameters"])
	decision := mapValue(mapValue(parameters["properties"])["decision"])
	properties := mapValue(decision["properties"])
	if stringValue(mapValue(mapValue(parameters["properties"])["text"])["description"]) == "" || stringValue(decision["description"]) == "" {
		t.Fatal("direct reply text or decision description was not sent")
	}
	if len(properties) != 18 || len(arrayValue(decision["required"])) != 18 {
		t.Fatalf("unexpected direct decision wire contract: %#v", decision)
	}
	for _, name := range []string{"action_type", "response_intent", "influences", "evidence_refs", "internal_intent", "claims", "goal_event_candidates", "appraisal", "attention", "thought", "desire", "agency", "response_plan", "personality_decision", "output_preference_decision", "core_alignment", "state_expression", "self_evaluation"} {
		field := mapValue(properties[name])
		description := strings.TrimSpace(stringValue(field["description"]))
		if description == "" {
			for _, raw := range arrayValue(field["anyOf"]) {
				if description = strings.TrimSpace(stringValue(mapValue(raw)["description"])); description != "" {
					break
				}
			}
		}
		if description == "" {
			t.Errorf("direct decision field %s lost its description on the physical request", name)
		}
	}
}

func TestDirectMainDecisionDescriptionsPreserveCognitionValidation(t *testing.T) {
	described := directMainDecisionSchema()
	original := cognitiveTurnResponseSchema()
	if stringValue(mapValue(mapValue(original["properties"])["internal_intent"])["description"]) != "" {
		t.Fatal("direct descriptions changed the shared cognition schema")
	}
	for _, decision := range []map[string]any{
		{"action_type": "reply", "response_intent": "回应", "influences": []any{}},
		{"action_type": "reply", "response_intent": "回应", "influences": []any{}, "claims": []any{map[string]any{"kind": "uncertain_hypothesis", "content": "可能有空", "confidence": 0.3, "evidence_refs": []any{}}}},
		{"action_type": "reply", "response_intent": "回应", "influences": []any{}, "internal_intent": map[string]any{"note": "类型不符"}},
		{"action_type": "reply", "response_intent": "回应", "influences": []any{}, "evidence_refs": []any{map[string]any{"ref": "不应是对象"}}},
	} {
		before := validateCapabilitySchemaValue(decision, original)
		after := validateCapabilitySchemaValue(decision, described)
		if (before == nil) != (after == nil) {
			t.Fatalf("descriptions changed decision validity: before=%v after=%v decision=%#v", before, after, decision)
		}
	}
}

// This uses a real isolated database and a controlled native ToolCall. It
// verifies both the visible effect and the old cognition settlement contract.
func TestDirectMainReplyPublishesAndSettlesAtomically(t *testing.T) {
	for _, failSettlement := range []bool{false, true} {
		name := "commit"
		if failSettlement {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				assertDirectReplyWireDescriptions(t, payload)
				decision := map[string]any{"action_type": "reply", "response_intent": "回应当前对话", "influences": []any{}, "claims": []any{map[string]any{"kind": "uncertain_hypothesis", "content": "对方可能想休息", "confidence": 0.3, "evidence_refs": []any{}}}}
				args := map[string]any{"text": "今天还不错，你呢？", "decision": decision}
				return fakeProviderResult{ToolCalls: []map[string]any{{"id": "direct-main-call", "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": jsonString(args)}}}}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "atomic-"+name, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, "direct")
			if failSettlement {
				if _, err := repo.Pool().Exec(ctx, `CREATE FUNCTION public.test_fail_direct_settlement() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected_direct_settlement_failure'; END $$;
					CREATE TRIGGER test_fail_direct_settlement BEFORE INSERT ON public.cognition_assessments FOR EACH ROW EXECUTE FUNCTION public.test_fail_direct_settlement()`); err != nil {
					t.Fatal(err)
				}
			}
			payload := map[string]any{"fluctlight_id": fid, "text": "你好，今天过得怎么样？", "idempotency_key": "atomic-direct-" + name, "turn_id": "atomic-direct-" + name, "attachment_refs": []any{}}
			result, err := app.HandleTurn(ctx, owner, conversation, payload)
			var messages, assessments, claims, receipts int
			if queryErr := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'),
				(SELECT count(*) FROM public.cognition_assessments WHERE fluctlight_id=$2),
				(SELECT count(*) FROM public.cognition_claims WHERE fluctlight_id=$2 AND content='对方可能想休息' AND status='uncertain'),
				(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='conversation.reply')`, conversation, fid).Scan(&messages, &assessments, &claims, &receipts); queryErr != nil {
				t.Fatal(queryErr)
			}
			if failSettlement {
				if err == nil || messages != 0 || assessments != 0 || claims != 0 || receipts != 0 {
					t.Fatalf("failed settlement left a partial publication: result=%#v err=%v messages=%d assessments=%d claims=%d receipts=%d", result, err, messages, assessments, claims, receipts)
				}
				return
			}
			if err != nil || stringValue(result.Assistant["text"]) != "今天还不错，你呢？" || calls != 1 || messages != 1 || assessments != 1 || claims != 1 || receipts != 1 {
				t.Fatalf("direct result=%#v err=%v calls=%d messages=%d assessments=%d claims=%d receipts=%d", result, err, calls, messages, assessments, claims, receipts)
			}
			var status, contract string
			if err := repo.Pool().QueryRow(ctx, `SELECT status,COALESCE(payload->'agent_result'->>'settlement_contract','') FROM public.cognition_inbox WHERE fluctlight_id=$1 AND payload->>'turn_id'=$2`, fid, payload["turn_id"]).Scan(&status, &contract); err != nil || status != "processed" || contract != "direct-main.v1" {
				t.Fatalf("direct settlement status=%s contract=%s err=%v", status, contract, err)
			}
			replayed, err := app.HandleTurn(ctx, owner, conversation, payload)
			if err != nil || stringValue(replayed.Assistant["text"]) != "今天还不错，你呢？" || calls != 1 {
				t.Fatalf("direct replay result=%#v err=%v calls=%d", replayed, err, calls)
			}
			if strings.TrimSpace(stringValue(replayed.Assistant["id"])) == "" {
				t.Fatal("replay lost the committed message")
			}
		})
	}
}

func TestDirectMainReplyRetainsExistingDraftContract(t *testing.T) {
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		t.Fatal("a stored draft unexpectedly called the model")
		return fakeProviderResult{}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-old-draft", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	scope := ToolExecutionRequest{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: fid, ConversationID: conversation, EvidenceID: "old-source", RunID: "old-run", Surface: CapabilitySurfaceConversation}
	inv := CapabilityInvocation{SourceFactID: scope.EvidenceID, Metadata: InvocationMetadata{FluctlightID: fid, ConversationID: conversation, AuthorizationActorID: owner}}
	key := privateReplyLogicalKey(ctx, inv)
	ref := "reply_draft_" + stableDigest(key)
	oldSettings := defaultConversationReplySettings()
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_reply_drafts(id,logical_key,fluctlight_id,conversation_id,authorization_actor_id,subject_actor_id,source_fact_id,profile_id,status,config_snapshot,config_version,binding_snapshot,payload)
		VALUES($1,$2,$3,$4,$5,$5,$6,'','ready',$7,7,'{}',$8)`, ref, key, fid, conversation, owner, scope.EvidenceID, jsonBytes(oldSettings), jsonBytes(ReplyDraft{Text: "旧草稿保持原样", ExpressionVersion: factOnlyReplyPromptVersion})); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"generating", "prepared", "needs_action", "waiting_media", "ready", "published", "failed", "cancelled", "legacy_fallback_required"} {
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_reply_drafts SET status=$2 WHERE id=$1`, ref, status); err != nil {
			t.Fatal(err)
		}
		runCtx, err := app.withPrivateConversationReplyRun(context.WithValue(ctx, directMainConversationKey{}, true), scope, "旧输入")
		if err != nil {
			t.Fatal(err)
		}
		capability, _ := app.capabilityRegistryForContext(runCtx).LookupCapability(conversationReplyCapabilityName)
		if _, ok := capability.(factOnlyReplyCapability); !ok || replyRunFromContext(runCtx).Settings.Mode != "fact_only" {
			t.Fatalf("old %s draft was sent to a new direct contract", status)
		}
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_reply_drafts SET status='ready' WHERE id=$1`, ref); err != nil {
		t.Fatal(err)
	}
	runCtx, err := app.withPrivateConversationReplyRun(context.WithValue(ctx, directMainConversationKey{}, true), scope, "旧输入")
	if err != nil {
		t.Fatal(err)
	}
	inv.Arguments = jsonBytes(map[string]any{})
	draft, gotRef, err := newFactOnlyReplyService(app).prepare(runCtx, inv, replyRunFromContext(runCtx))
	if err != nil || gotRef != ref || draft.Text != "旧草稿保持原样" {
		t.Fatalf("stored draft was not recovered: %#v ref=%s err=%v", draft, gotRef, err)
	}
	scope.EvidenceID = "new-source"
	newCtx, err := app.withPrivateConversationReplyRun(context.WithValue(ctx, directMainConversationKey{}, true), scope, "新输入")
	if err != nil {
		t.Fatal(err)
	}
	capability, _ := app.capabilityRegistryForContext(newCtx).LookupCapability(conversationReplyCapabilityName)
	if _, ok := capability.(directMainReplyCapability); !ok {
		t.Fatal("a new source did not select the direct contract")
	}
}

func TestDirectMainReplyStopsAfterRepeatedInvalidNativeCalls(t *testing.T) {
	for _, semantic := range []bool{false, true} {
		name := "schema"
		if semantic {
			name = "foreign_goal"
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				decision := map[string]any{"action_type": "reply"}
				if semantic {
					decision = map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}, "goal_event_candidates": []any{map[string]any{"goal_ref": "goal:ctx_nonexistent", "reason": "不属于当前上下文"}}}
				}
				return fakeProviderResult{ToolCalls: []map[string]any{{"id": fmt.Sprintf("invalid-direct-%d", calls), "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": jsonString(map[string]any{"text": "未提交", "decision": decision})}}}}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "invalid-direct-"+name, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, "direct")
			_, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你好", "idempotency_key": "invalid-direct", "turn_id": "invalid-direct", "attachment_refs": []any{}})
			if err == nil || calls != 3 {
				t.Fatalf("invalid native calls did not stop after bounded feedback: calls=%d err=%v", calls, err)
			}
			var messages int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&messages); err != nil || messages != 0 {
				t.Fatalf("invalid reply was published: count=%d err=%v", messages, err)
			}
		})
	}
}

func TestDirectMainReplyRetainsPriorNativeAction(t *testing.T) {
	for _, failSettlement := range []bool{false, true} {
		name := "success"
		if failSettlement {
			name = "settlement_failure"
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				name := capabilityCatalogName
				args := map[string]any{"names": []any{"affect_event"}}
				switch calls {
				case 2:
					name = "affect_event"
					args = map[string]any{"event": map[string]any{"type": "calm", "confidence": 0.8}}
				case 3:
					name = conversationReplyCapabilityName
					args = map[string]any{"text": "我现在平静一些了。", "decision": map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}}
				}
				return fakeProviderResult{ToolCalls: []map[string]any{{"id": fmt.Sprintf("direct-action-%d", calls), "type": "function", "function": map[string]any{"name": name, "arguments": jsonString(args)}}}}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-prior-action-"+name, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, "direct")
			if failSettlement {
				if _, err := repo.Pool().Exec(ctx, `CREATE FUNCTION public.test_fail_action_settlement() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected_action_settlement_failure'; END $$;
			CREATE TRIGGER test_fail_action_settlement BEFORE INSERT ON public.cognition_assessments FOR EACH ROW EXECUTE FUNCTION public.test_fail_action_settlement()`); err != nil {
					t.Fatal(err)
				}
			}
			payload := map[string]any{"fluctlight_id": fid, "text": "慢慢平静下来吧。", "idempotency_key": "direct-action", "turn_id": "direct-action", "attachment_refs": []any{}}
			result, err := app.HandleTurn(ctx, owner, conversation, payload)
			if (!failSettlement && (err != nil || stringValue(result.Assistant["text"]) != "我现在平静一些了。")) || (failSettlement && err == nil) || calls != 3 {
				t.Fatalf("native action then reply: result=%#v calls=%d err=%v", result, calls, err)
			}
			wantMessages := 1
			if failSettlement {
				wantMessages = 0
			}
			var events, receipts, messages int
			if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.fluctlight_inner_state_events WHERE fluctlight_id=$1 AND event_type='affect.event'),
		(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='affect_event'),
		(SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$2 AND kind='assistant')`, fid, conversation).Scan(&events, &receipts, &messages); err != nil || events != 1 || receipts != 1 || messages != wantMessages {
				t.Fatalf("native action was lost or repeated: events=%d receipts=%d messages=%d err=%v", events, receipts, messages, err)
			}
			_, err = app.HandleTurn(ctx, owner, conversation, payload)
			if (!failSettlement && err != nil) || (failSettlement && err == nil) || calls != 3 {
				t.Fatalf("replay repeated a native action or model call: calls=%d err=%v", calls, err)
			}
		})
	}
}
