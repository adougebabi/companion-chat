package core

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// Exercise the actual direct request and settlement, not a fabricated Core
// reference. The Provider and all test records are controlled fixtures.
func TestM2DirectRelationshipInfluenceUsesOfferedReference(t *testing.T) {
	const roleLabel = "关系引用验收朋友"
	relationshipID := "m2-private-relationship-id"
	calls := 0
	var offeredRef, requestBody string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		requestBody = jsonString(payload)
		offeredRef = regexp.MustCompile(`relationship:ctx_[a-f0-9]{32}`).FindString(requestBody)
		influences := []any{}
		if offeredRef != "" {
			influences = append(influences, map[string]any{"ref": offeredRef, "role": "grounds", "confidence": 0.9, "note": "既有关系影响本次回应"})
		}
		decision := map[string]any{"action_type": "reply", "response_intent": "自然回应", "influences": influences}
		args := map[string]any{"text": "很高兴和你聊聊。", "decision": decision}
		return fakeProviderResult{ToolCalls: []map[string]any{{"id": "m2-relationship-reply", "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": jsonString(args)}}}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-relationship-influence", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,target_actor_id,role,metrics,revision) VALUES($1,$2,$3,$4,'{}',1)`, relationshipID, fid, owner, jsonBytes(map[string]any{"label": roleLabel})); err != nil {
		t.Fatal(err)
	}
	turnInput := map[string]any{"fluctlight_id": fid, "text": "今天想聊聊吗？", "turn_id": "m2-relationship-turn", "idempotency_key": "m2-relationship-turn", "attachment_refs": []any{}}
	turn, err := app.HandleTurn(ctx, owner, conversation, turnInput)
	if err != nil || calls != 1 || stringValue(turn.Assistant["text"]) != "很高兴和你聊聊。" {
		t.Fatalf("direct turn=%#v calls=%d err=%v", turn, calls, err)
	}
	if !strings.Contains(requestBody, roleLabel) {
		t.Fatal("fixture relationship was absent from the actual direct request")
	}
	if strings.Contains(requestBody, relationshipID) {
		t.Fatal("raw relationship database ID crossed the Provider boundary")
	}
	if offeredRef == "" {
		t.Fatal("actual direct request included the relationship but omitted its usable influence reference")
	}
	checkSettlement := func() {
		t.Helper()
		var messages, assessments int
		var referencesRaw, influencesRaw []byte
		err := repo.Pool().QueryRow(ctx, `SELECT
			(SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'),
			(SELECT count(*) FROM public.cognition_assessments WHERE fluctlight_id=$2),
			(SELECT payload->'agent_result'->'context_references' FROM public.cognition_inbox WHERE fluctlight_id=$2 AND payload->>'turn_id'=$3),
			(SELECT payload->'influences' FROM public.cognition_assessments WHERE fluctlight_id=$2)`, conversation, fid, turnInput["turn_id"]).Scan(&messages, &assessments, &referencesRaw, &influencesRaw)
		if err != nil || messages != 1 || assessments != 1 {
			t.Fatalf("direct settlement messages=%d assessments=%d err=%v", messages, assessments, err)
		}
		reference := mapValue(decodeObject(referencesRaw)[offeredRef])
		if stringValue(reference["kind"]) != string(ContextReferenceRelationship) || stringValue(reference["entity_id"]) != relationshipID {
			t.Fatalf("frozen influence did not resolve to the current relationship: %#v", reference)
		}
		if !strings.Contains(string(influencesRaw), offeredRef) {
			t.Fatalf("assessment did not retain its relationship influence: %s", influencesRaw)
		}
	}
	checkSettlement()
	if _, err := app.HandleTurn(ctx, owner, conversation, turnInput); err != nil || calls != 1 {
		t.Fatalf("replay unexpectedly requested the Provider again: calls=%d err=%v", calls, err)
	}
	checkSettlement()
}

func TestM2DirectRelationshipInfluenceDropsForgedReference(t *testing.T) {
	calls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		calls++
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{map[string]any{"ref": "relationship:ctx_00000000000000000000000000000000", "role": "grounds", "confidence": 0.9, "note": "伪造关系"}}}
		args := map[string]any{"text": "你好，我们聊聊。", "decision": decision}
		return fakeProviderResult{ToolCalls: []map[string]any{{"id": fmt.Sprintf("m2-forged-relationship-%d", calls), "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": jsonString(args)}}}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-forged-relationship", router)
	app.Provider.HTTP = &http.Client{Transport: router}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	turn, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "m2-forged-relationship", "idempotency_key": "m2-forged-relationship", "attachment_refs": []any{}})
	var messages, assessments int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'), (SELECT count(*) FROM public.cognition_assessments WHERE fluctlight_id=$2)`, conversation, fid).Scan(&messages, &assessments); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err != nil || calls != 3 || messages != 1 || assessments != 1 || stringValue(turn.Assistant["text"]) != "你好，我们聊聊。" {
		t.Fatalf("unresolved influence blocked valid reply: err=%v calls=%d messages=%d assessments=%d", err, calls, messages, assessments)
	}
	var influences []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT payload->'influences' FROM public.cognition_assessments WHERE fluctlight_id=$1`, fid).Scan(&influences); err != nil || string(influences) != "[]" {
		t.Fatalf("forged reference entered settlement: influences=%s err=%v", influences, err)
	}
}

func TestM2RelationshipReferenceRemainsScopedToConversationMain(t *testing.T) {
	projection := func(conversation string) ContextProjection {
		return ContextProjection{
			SchemaVersion: "fluctlight.context.v3", FluctlightID: "relationship-fluctlight", OwnerActorID: "relationship-owner",
			ConversationID: conversation, CurrentSpeaker: map[string]any{"actor_id": "relationship-owner"},
			Relationships: []map[string]any{{"id": "private-relationship", "target_actor_id": "relationship-owner", "revision": 1, "role": map[string]any{"label": "朋友"}}},
		}
	}
	first, second := projection("conversation-first"), projection("conversation-second")
	if err := buildContextReferenceIndex(&first); err != nil {
		t.Fatal(err)
	}
	if err := buildContextReferenceIndex(&second); err != nil {
		t.Fatal(err)
	}
	firstRef := stringValue(first.Relationships[0]["ref"])
	secondRef := stringValue(second.Relationships[0]["ref"])
	if firstRef == "" || secondRef == "" || firstRef == secondRef {
		t.Fatalf("references were not scoped to their conversations: first=%q second=%q", firstRef, secondRef)
	}
	direct := compactRelationshipsForSurface(first.Relationships, first.Actors, first.ReferenceIndex, ProviderContextSurfaceConversationMain, true)
	takeover := compactRelationshipsForSurface(first.Relationships, first.Actors, first.ReferenceIndex, ProviderContextSurfaceTakeoverReply, false)
	if len(direct) != 1 || stringValue(direct[0]["ref"]) != firstRef || len(takeover) != 1 || stringValue(takeover[0]["ref"]) != "" {
		t.Fatalf("reference escaped its intended surface: direct=%#v takeover=%#v", direct, takeover)
	}
	legacyInput := workingMemoryInputFromProjectionForSurface(first, ProviderContextSurfaceConversationMain, nil, nil)
	directInput := workingMemoryInputForSurface(first, ProviderContextSurfaceConversationMain, nil, nil, true)
	if strings.Contains(jsonString(legacyInput.RuntimeFacts), firstRef) || !strings.Contains(jsonString(directInput.RuntimeFacts), firstRef) {
		t.Fatal("direct-only relationship reference changed the legacy projection")
	}
	foreign := cloneMap(second.Relationships[0])
	foreign["ref"] = firstRef
	if got := compactRelationshipsForSurface([]map[string]any{foreign}, second.Actors, second.ReferenceIndex, ProviderContextSurfaceConversationMain, true); len(got) != 1 || stringValue(got[0]["ref"]) != "" {
		t.Fatalf("foreign conversation reference reached the request: %#v", got)
	}
	decision := map[string]any{"influences": []any{map[string]any{"ref": firstRef, "role": "grounds", "confidence": 0.9, "note": "其他会话的关系引用"}}}
	if _, err := freezeDecisionInfluences(decision, second, false); err == nil {
		t.Fatal("foreign conversation reference passed the Core settlement boundary")
	}
}
