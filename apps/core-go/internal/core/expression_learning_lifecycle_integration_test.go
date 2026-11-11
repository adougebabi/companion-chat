package core

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// The database, Owner surface and conversation entry are real test instances.
// Extraction and the main Agent use a controlled Provider, so receipt of an
// example is not evidence that a real model chose to imitate it.
func TestExpressionCandidateEnableUseAndDisableAcrossNewTurns(t *testing.T) {
	var requests []map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if providerWireSchemaName(payload) == expressionLearningSchema {
			return fakeProviderResult{Structured: map[string]any{"candidates": []any{map[string]any{"source_ref": "s1", "excerpt": "慢慢来呀", "situation": "讨论当前进度"}}}}
		}
		requests = append(requests, payload)
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("expression-turn-%d", len(requests)), conversationReplyCapabilityName, map[string]any{"text": "好，我知道了。", "decision": decision})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "expression-lifecycle", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,target_actor_id,role,metrics,revision) VALUES('expression-relationship',$1,$2,'{"label":"friend"}','{}',1)`, fid, owner); err != nil {
		t.Fatal(err)
	}
	var personaBefore, relationBefore string
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT core_persona::text FROM public.fluctlights WHERE id=$1),
		(SELECT jsonb_build_object('role',role,'metrics',metrics,'trend',trend,'summary',summary,'emotional_association',emotional_association,'revision',revision)::text FROM public.relationships WHERE id='expression-relationship')`, fid).Scan(&personaBefore, &relationBefore); err != nil {
		t.Fatal(err)
	}
	source, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "慢慢来呀", "turn_id": "expression-source-turn", "idempotency_key": "expression-source-turn", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	base := "/internal/fluctlights/" + fid + "/expression-learning"
	call := func(method, path string, body map[string]any) map[string]any {
		t.Helper()
		result, err := app.DispatchOwnerSurface(ctx, method, path, url.Values{}, owner, body)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return result
	}
	call("PUT", base+"/settings", map[string]any{"learning_enabled": true, "use_enabled": true, "expected_revision": 0, "idempotency_key": "expression-lifecycle-settings"})
	extracted := call("POST", base+"/extractions", map[string]any{"source_message_id": stringValue(source.UserMessage["id"]), "idempotency_key": "expression-lifecycle-extract"})
	ids := arrayValue(extracted["candidate_ids"])
	if len(ids) != 1 {
		t.Fatalf("candidate extraction=%#v", extracted)
	}
	id := stringValue(ids[0])
	call("POST", base+"/candidates/"+id+"/enable", map[string]any{"expected_revision": 1, "reason": "适合讨论进度", "trigger_text": "进度", "idempotency_key": "expression-lifecycle-enable"})
	// Turning learning off does not turn off use of an already approved example.
	call("PUT", base+"/settings", map[string]any{"learning_enabled": false, "use_enabled": true, "expected_revision": 1, "idempotency_key": "expression-lifecycle-learning-off"})
	turn := func(id string) {
		t.Helper()
		_, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "现在进度怎么样？", "turn_id": id, "idempotency_key": id, "attachment_refs": []any{}})
		if err != nil {
			t.Fatalf("turn %s: %v", id, err)
		}
	}
	turn("expression-enabled-turn")
	call("POST", base+"/candidates/"+id+"/disable", map[string]any{"expected_revision": 2, "reason": "停止参考", "idempotency_key": "expression-lifecycle-disable"})
	turn("expression-disabled-turn")
	if len(requests) != 2 {
		t.Fatalf("main Agent requests=%d", len(requests))
	}
	for index, payload := range requests {
		var suggestions []string
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			content := stringValue(message["content"])
			if strings.Contains(content, "expression_suggestions") {
				suggestions = append(suggestions, content)
			}
		}
		if index == 0 && (len(suggestions) != 1 || !strings.Contains(suggestions[0], "慢慢来呀") || !strings.Contains(suggestions[0], "讨论当前进度")) {
			t.Fatalf("enabled candidate absent from actual request: %#v", suggestions)
		}
		if index == 1 && len(suggestions) != 0 {
			t.Fatalf("disabled candidate was injected into a new turn: %#v", suggestions)
		}
	}
	var personaAfter, relationAfter string
	var interactions int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT core_persona::text FROM public.fluctlights WHERE id=$1),
		(SELECT jsonb_build_object('role',role,'metrics',metrics,'trend',trend,'summary',summary,'emotional_association',emotional_association,'revision',revision)::text FROM public.relationships WHERE id='expression-relationship'),
		(SELECT interaction_frequency::integer FROM public.relationships WHERE id='expression-relationship')`, fid).Scan(&personaAfter, &relationAfter, &interactions); err != nil {
		t.Fatal(err)
	}
	if personaAfter != personaBefore || relationAfter != relationBefore || interactions != 2 {
		t.Fatalf("expression learning changed personality or relationship semantics: persona_changed=%t relation_changed=%t ordinary_interactions=%d", personaAfter != personaBefore, relationAfter != relationBefore, interactions)
	}
}
