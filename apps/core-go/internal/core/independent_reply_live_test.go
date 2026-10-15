package core

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in only; never run during local implementation checks. The independent
// endpoint/model are explicit so this row cannot silently test the main model.
func testFormalAgentE2EIndependentReply(t *testing.T) {
	base := strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_REPLY_PROVIDER_BASE_URL"))
	model := strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_REPLY_PROVIDER_MODEL"))
	if base == "" || model == "" {
		t.Skip("independent reply live endpoint/model not configured")
	}
	fixture := newFormalAgentE2EFixture(t)
	projection, err := fixture.app.BuildContextProjectionFor(fixture.ctx, ContextProjectionRequest{AuthorizationActorID: fixture.ownerID, SpeakerActorID: fixture.ownerID, FluctlightID: fixture.fluctlightID, ConversationID: fixture.conversationID, CurrentUserText: "继续聊聊你的近况。", MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
	if err != nil {
		t.Fatal(err)
	}
	working, err := fixture.app.loadCompiledWorkingPersona(fixture.ctx, projection)
	if err != nil {
		t.Fatal(err)
	}
	persona := systemPersonaForProjectionWithWorking(projection, "conversation_reply_text", renderCompiledWorkingPersona(working))
	draft, err := fixture.app.RunConversationReplyTask(fixture.ctx, ConversationReplyTaskInput{Context: ReplyContextInput{Projection: projection, WorkingPersona: persona, CurrentInput: "继续聊聊你的近况。"}, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: strings.TrimRight(base, "/"), ModelID: model, Secret: os.Getenv("FLUCTLIGHT_LIVE_REPLY_PROVIDER_API_KEY"), Timeout: 5 * time.Minute, TokenBudget: 1024, ContextWindowTokens: 32768, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}, ConfigVersion: 1, CorrelationID: "live-independent-reply", Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(draft.Text) == "" || draft.Mode != "fact_only" || draft.ModelRef != model {
		t.Fatalf("invalid independent reply: mode=%s, model=%s", draft.Mode, draft.ModelRef)
	}
}
