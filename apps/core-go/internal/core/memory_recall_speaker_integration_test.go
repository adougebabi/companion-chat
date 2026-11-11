package core

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestMemoryRecallDistinguishesActualMessageSpeakers(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	owner, self, conversation := "recall-owner", "recall-self", "recall-conversation"
	seedTurnConversation(t, ctx, repo, owner, self, conversation)
	for _, actor := range []string{"recall-other-a", "recall-other-b"} {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, actor); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'member','active')`, conversation, actor); err != nil {
			t.Fatal(err)
		}
	}
	for index, item := range []struct{ actor, kind string }{{owner, "user"}, {self, "assistant"}, {"recall-other-a", "user"}, {"recall-other-b", "user"}} {
		id := fmt.Sprintf("recall-source-%d", index)
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,created_at)
			VALUES($1,$2,$3::integer,$4,$5,'ticket same statement','[]',$1,now()+$3::integer*interval '1 second')`, id, conversation, index+1, item.actor, item.kind); err != nil {
			t.Fatal(err)
		}
	}
	otherConversation := "recall-other-conversation"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'other')`, otherConversation, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, otherConversation, owner, self); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('recall-foreign-message',$1,1,$2,'user','ticket foreign conversation','[]','foreign')`, otherConversation, owner); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repo, nil)
	items, _, err := newMemoryRecallService(app).Recall(ctx, MemoryRecallRequest{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, ViewerActorIDs: []string{owner}, SpeakerActorID: owner, ConversationMode: MemoryConversationExact, ActiveProfileID: "default", Intent: "ticket"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, item := range items {
		if item["source_kind"] != "conversation_record" {
			continue
		}
		if item["content"] != "ticket same statement" || item["occurred_at"] == nil || item["message_kind"] == nil {
			t.Fatalf("conversation fact lost its actual text, type or time: %#v", item)
		}
		seen[stringValue(item["speaker"])]++
	}
	if seen["current_speaker"] != 1 || seen["actor_self"] != 1 || len(seen) != 4 {
		t.Fatalf("speakers of identical statements were not distinguishable: %#v", seen)
	}
	for speaker := range seen {
		if speaker != "current_speaker" && speaker != "actor_self" && !strings.HasPrefix(speaker, "other_actor_") {
			t.Fatalf("other speaker is not represented safely: %s", speaker)
		}
	}
	encoded := jsonString(items)
	for _, forbidden := range []string{owner, self, "recall-other-a", "recall-other-b", "recall-source-", "foreign conversation"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("recall leaked identity or another conversation: %q", forbidden)
		}
	}
	unknown := recallRawItem(RawHistoryEvent{Kind: RawHistoryConversationMessage, Content: map[string]any{"text": "ticket unknown", "kind": "user"}}, MemoryRecallRequest{FluctlightID: self, ConversationID: conversation})
	if unknown["speaker"] != "unknown" {
		t.Fatalf("missing speaker was invented: %#v", unknown)
	}
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "memory.recall", OperationID: "recall-speakers", AuthorizationActorID: owner, SubjectActorID: owner,
		FluctlightID: self, ConversationID: conversation, Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{"intent": "ticket"})})
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("real recall Tool failed: %#v err=%v", receipt.Result, err)
	}
	definition := memoryRecallCapabilityDefinition()
	if err := validateCapabilitySchemaValue(receipt.Result.Output, definition.OutputSchema); err != nil {
		t.Fatalf("speaker results violate Tool output schema: %v", err)
	}
	wire := jsonString(modelFacingToolResult(receipt, definition))
	for _, value := range []string{`"speaker":"actor_self"`, `"speaker":"current_speaker"`, `"speaker":"other_actor_`, `"message_kind":"assistant"`} {
		if !strings.Contains(wire, value) {
			t.Fatalf("native Tool result omitted %s: %s", value, wire)
		}
	}
	for _, forbidden := range []string{owner, self, "recall-other-a", "recall-other-b", "foreign conversation"} {
		if strings.Contains(wire, forbidden) {
			t.Fatalf("native Tool result leaked %q", forbidden)
		}
	}
	otherReceipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "memory.recall", OperationID: "recall-other-speaker", AuthorizationActorID: owner, SubjectActorID: "recall-other-a",
		FluctlightID: self, ConversationID: conversation, Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{"intent": "ticket"})})
	if err != nil {
		t.Fatalf("authorized other-speaker recall failed: %v", err)
	}
	scoped := MemoryRecallRequest{FluctlightID: self, ConversationID: conversation}
	speakers := map[string]string{}
	for _, raw := range arrayValue(decodeObject(jsonBytes(otherReceipt.Result.Output))["items"]) {
		item := mapValue(raw)
		speakers[stringValue(item["ref"])] = stringValue(item["speaker"])
	}
	if speakers[recallOpaqueRef("conversation_record", "message:recall-source-0", scoped)] != "actor_owner" ||
		speakers[recallOpaqueRef("conversation_record", "message:recall-source-2", scoped)] != "current_speaker" {
		t.Fatalf("direct Tool confused owner and current speaker: %#v", speakers)
	}
	seedCognitiveProviderRole(t, ctx, repo, "recall-speaker-endpoint")
	var toolMessage string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] == "tool" && message["tool_call_id"] == "recall-speaker-call" {
				toolMessage = stringValue(message["content"])
			}
		}
		if toolMessage != "" {
			return fakeProviderResult{Structured: map[string]any{"answer": "记录的发言者不同"}}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("recall-speaker-call", "memory.recall", map[string]any{"intent": "ticket"})}}
	})
	app.Provider.HTTP = &http.Client{Transport: router}
	projection := ContextProjection{OwnerActorID: owner, FluctlightID: self, ConversationID: conversation, SourceFactID: "recall-speaker-fact",
		CurrentSpeaker: map[string]any{"actor_id": owner}, MemoryRetrievalTrace: MemoryRetrievalTrace{ConversationMode: string(MemoryConversationExact)},
		ReferenceIndex: ContextReferenceIndex{SchemaVersion: contextReferenceIndexVersion, FluctlightID: self, OwnerActorID: owner, SpeakerActorID: owner, ConversationID: conversation, ByRef: map[string]ContextReference{}}}
	schema := objectSchema(map[string]any{"answer": stringSchema()}, []string{"answer"}, false)
	_, err = app.RunFormalAgent(ctx, FormalAgentConversationCognition, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "system", "content": "Use the memory Tool."}, {"role": "user", "content": "查找之前的发言"}}, ResponseFormat: schema},
		Definitions: []CapabilityDefinition{definition}, SchemaName: "conversation_turn_response",
		Capability: &ADKCapabilityRequest{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, SourceFactID: "recall-speaker-fact", ActionID: "recall-speaker-action", Surface: CapabilitySurfaceConversation, Projection: projection}})
	if err != nil {
		t.Fatalf("native recall continuation failed: %v", err)
	}
	for _, value := range []string{`"speaker":"actor_self"`, `"speaker":"current_speaker"`, `"speaker":"other_actor_`} {
		if !strings.Contains(toolMessage, value) {
			t.Fatalf("physical continuation lost %s: %s", value, toolMessage)
		}
	}
	for _, forbidden := range []string{owner, self, "recall-other-a", "recall-other-b", "foreign conversation"} {
		if strings.Contains(toolMessage, forbidden) {
			t.Fatalf("physical continuation leaked %q", forbidden)
		}
	}
	const otherSelf = "recall-other-fluctlight"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'fluctlight','active')`, otherSelf); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, otherSelf, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'member','active')`, conversation, otherSelf); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('recall-other-self-message',$1,5,$2,'user','ticket same statement','[]','other-self')`, conversation, otherSelf); err != nil {
		t.Fatal(err)
	}
	projection.CurrentSpeaker = map[string]any{"actor_id": otherSelf, "ref": "actor_b", "type": "fluctlight"}
	projection.SelfActor = map[string]any{"actor_id": self, "ref": "actor_self", "type": "fluctlight"}
	projection.Actors = []map[string]any{projection.SelfActor, projection.CurrentSpeaker, {"actor_id": owner, "ref": "actor_c", "type": "human"}}
	projection.ReferenceIndex.SpeakerActorID = otherSelf
	toolMessage = ""
	_, err = app.RunFormalAgent(ctx, FormalAgentConversationCognition, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "system", "content": "Use the memory Tool."}, {"role": "user", "content": "查找之前的发言"}}, ResponseFormat: schema},
		Definitions: []CapabilityDefinition{definition}, SchemaName: "conversation_turn_response",
		Capability: &ADKCapabilityRequest{AuthorizationActorID: owner, SubjectActorID: otherSelf, FluctlightID: self, ConversationID: conversation, SourceFactID: "recall-speaker-fact", ActionID: "recall-speaker-other-action", Surface: CapabilitySurfaceConversation, Projection: projection}})
	if err != nil {
		t.Fatalf("native other-actor recall failed: %v", err)
	}
	for _, value := range []string{`"speaker":"actor_self"`, `"speaker":"current_speaker"`, `"speaker":"actor_owner"`} {
		if !strings.Contains(toolMessage, value) {
			t.Fatalf("physical continuation lost the stable speaker label %s: %s", value, toolMessage)
		}
	}
	if strings.Contains(toolMessage, `"speaker":"actor_user"`) || strings.Contains(toolMessage, otherSelf) {
		t.Fatalf("another Fluctlight was mislabeled or its database ID leaked: %s", toolMessage)
	}
}
