package core

import "testing"

func TestConversationCognitionShowsActorFactRecordFromFirstRound(t *testing.T) {
	selection := &kevRunSelection{mandatory: formalAgentMandatoryTools(FormalAgentConversationCognition)}
	if !selection.baselineTool(actorFactCapabilityName) {
		t.Fatal("actor.fact.record still requires catalog discovery in chat")
	}
	if !selection.baselineTool(conversationReplyCapabilityName) || selection.baselineTool("wardrobe.discard") {
		t.Fatal("baseline membership changed unexpectedly")
	}
	var none *kevRunSelection
	if none.baselineTool(actorFactCapabilityName) || !none.baselineTool(capabilityCatalogName) {
		t.Fatal("nil selection must fall back to the public baseline")
	}
}

func TestUserLocationFactsStayInChatBackground(t *testing.T) {
	projection := ContextProjection{
		Actors: []map[string]any{{"actor_id": "user-1", "ref": "actor_user", "type": "human"}},
		ActorFacts: []map[string]any{
			{"actor_id": "user-1", "attribute": "country", "value": "国外"},
			{"actor_id": "user-1", "attribute": "location", "value": "伦敦"},
			{"actor_id": "user-1", "attribute": "dislikes", "value": "香菜"},
		},
	}
	background := compactActorBackground(projection)
	if len(background) != 3 {
		t.Fatalf("user facts were filtered out of 关于对方: %#v", background)
	}
}
