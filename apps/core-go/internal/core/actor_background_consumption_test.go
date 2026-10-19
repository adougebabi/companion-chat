package core

import (
	"strings"
	"testing"
)

func TestAboutYouConsumesCurrentHumanPreferencesAndKeepsOwnerSeparate(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	other := randomID("human_participant_")
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active');`, other); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'member','active')`, f.conversationID, other); err != nil {
		t.Fatal(err)
	}
	recordActorFact(t, f, "about-owner-name", "assert", "name", "Owner-only-name", "")
	initial, err := f.app.ActorContext(f.ctx, f.ownerID, f.fluctlightID, other)
	if err != nil {
		t.Fatal("active participant without a relationship could not edit background", err)
	}
	body := map[string]any{"background": map[string]any{"name": "Current-speaker-name", "interests": "Current-speaker-interest", "preferences": "Current-speaker-boundary"}, "operation": "correct", "reason": "Correct participant data", "expected_context_version": initial["context_version"], "idempotency_key": "about-participant-save"}
	if _, err := f.app.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, other, body); err != nil {
		t.Fatal(err)
	}
	projection, err := f.app.BuildContextProjectionFor(f.ctx, ContextProjectionRequest{AuthorizationActorID: f.ownerID, SpeakerActorID: other, TargetActorID: other, FluctlightID: f.fluctlightID, ConversationID: f.conversationID, CurrentUserText: "test", MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
	if err != nil {
		t.Fatal(err)
	}
	for _, cognition := range []bool{true, false} {
		background := jsonString(factOnlyContextFacts(projection, cognition).Facts["actor_background"])
		if !strings.Contains(background, "Current-speaker-name") || !strings.Contains(background, "Current-speaker-interest") || !strings.Contains(background, "Current-speaker-boundary") || strings.Contains(background, "Owner-only-name") {
			t.Fatalf("wrong subject or lost fields in context: %s", background)
		}
	}
	current, err := f.app.ActorContext(f.ctx, f.ownerID, f.fluctlightID, other)
	if err != nil {
		t.Fatal(err)
	}
	body["background"] = map[string]any{"preferences": "Latest-speaker-boundary"}
	body["expected_context_version"] = current["context_version"]
	body["idempotency_key"] = "about-participant-correction"
	if _, err := f.app.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, other, body); err != nil {
		t.Fatal(err)
	}
	projection, err = f.app.BuildContextProjectionFor(f.ctx, ContextProjectionRequest{AuthorizationActorID: f.ownerID, SpeakerActorID: other, TargetActorID: other, FluctlightID: f.fluctlightID, ConversationID: f.conversationID, CurrentUserText: "test", MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
	if err != nil {
		t.Fatal(err)
	}
	background := jsonString(factOnlyContextFacts(projection, false).Facts["actor_background"])
	if !strings.Contains(background, "Latest-speaker-boundary") || strings.Contains(background, "Current-speaker-boundary") {
		t.Fatal("correction not consumed", background)
	}
}
