package core

import (
	"context"
	"strings"
	"testing"
)

const (
	takeoverChainCandidateText = "星火的候选回复"
	takeoverChainTakeoverText  = "暮光的接管回复"
)

func takeoverChainRunTurn(t *testing.T, app *App, ctx context.Context, ownerID, conversationID, fluctlightID, text, idempotencyKey, turnID string) error {
	t.Helper()
	_, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, text, idempotencyKey, turnID))
	return err
}

func TestCompletedNativePersonaTurnReplaysCommittedAssistantWithoutProvider(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "completed-replay-owner", "completed-replay-fluctlight", "completed-replay-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	payload := takeoverChainTurnPayload(fluctlightID, "由暮光回答。", "completed-replay-turn", "completed-replay-turn-1")
	step := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("replay-takeover", personaTakeoverCapabilityName, map[string]any{
				"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "spark", "target_profile_id": "twilight",
			})}}
		case 2:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("replay-reply", "conversation.reply", map[string]any{"text": "暮光的已提交回复"})}}
		default:
			return nativePersonaFinal()
		}
	})
	app := newTestApp(t, repository, router)
	first, err := app.HandleTurn(ctx, ownerID, conversationID, payload)
	if err != nil {
		t.Fatal(err)
	}
	replayRouter := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		t.Fatal("completed turn replay reached Provider")
		return fakeProviderResult{Status: 500}
	})
	replayed, err := newTestApp(t, repository, replayRouter).HandleTurn(ctx, ownerID, conversationID, payload)
	if err != nil {
		t.Fatal(err)
	}
	if replayRouter.totalRequests() != 0 || stringValue(replayed.Assistant["id"]) != stringValue(first.Assistant["id"]) {
		t.Fatalf("replay=%#v first=%#v requests=%d", replayed, first, replayRouter.totalRequests())
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID); count != 1 {
		t.Fatalf("assistant message replayed %d times", count)
	}
}

func TestCommittedReplySurvivesInvalidFinalAndFailedRunDoesNotReplay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "failed-replay-owner", "failed-replay-fluctlight", "failed-replay-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	payload := takeoverChainTurnPayload(fluctlightID, "请回复。", "failed-replay-turn", "failed-replay-turn-1")
	step := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
		step++
		if step == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("failed-reply", "conversation.reply", map[string]any{"text": "已经真实提交"})}}
		}
		return fakeProviderResult{Structured: map[string]any{"action_type": "reply", "response_intent": "invalid", "influences": []any{}, "legacy_candidate": "不得回退"}}
	})
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, payload); err == nil || !strings.Contains(err.Error(), "adk_final_output_invalid") {
		t.Fatalf("invalid final error=%v", err)
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='已经真实提交'`, conversationID); count != 1 {
		t.Fatalf("committed reply count=%d", count)
	}
	replayRouter := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		t.Fatal("failed run replay reached Provider")
		return fakeProviderResult{Status: 500}
	})
	if _, err := newTestApp(t, repository, replayRouter).HandleTurn(ctx, ownerID, conversationID, payload); err == nil {
		t.Fatal("failed run replay was reported as success")
	}
	if replayRouter.totalRequests() != 0 {
		t.Fatalf("failed run replay made %d Provider requests", replayRouter.totalRequests())
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID); count != 1 {
		t.Fatalf("failed run duplicated committed reply: %d", count)
	}
}
