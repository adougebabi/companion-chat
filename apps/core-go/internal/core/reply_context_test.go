package core

import (
	"errors"
	"strings"
	"testing"
)

func replyContextFixture() ReplyContextInput {
	return ReplyContextInput{WorkingPersona: map[string]any{"identity": map[string]any{"name": "林岚"}, "personality": map[string]any{"style": "严谨直白，喜欢详细分析"}},
		CurrentInput: "我愿意继续聊，也请不要再提那个旧话题。",
		Projection: ContextProjection{FluctlightID: "internal-self-id", OwnerActorID: "internal-owner-id", ConversationID: "internal-conversation-id",
			SelfActor:      map[string]any{"id": "internal-self-id", "name": "林岚", "type": "fluctlight"},
			CurrentSpeaker: map[string]any{"id": "internal-owner-id", "name": "读者", "type": "human"},
			CurrentState:   map[string]any{"data": map[string]any{"life_context": map[string]any{"scene": "家里", "location": "书桌", "activity": "阅读"}}},
			Goals:          []map[string]any{{"id": "internal-goal-id", "description": "约定读书讨论", "desired_outcome": "提出具体讨论时间", "success_criteria": []any{"双方明确确认讨论时间"}, "state": "active", "importance": 0.9, "execution": map[string]any{"last_evaluation": "用户冷淡所以暂时不推进"}}},
			RecentMessages: []map[string]any{{"kind": "user", "text": "这个设备型号是 e6。", "id": "internal-message-id", "turn_id": "first"}, {"kind": "assistant", "text": "我记得你说过它。", "turn_id": "first"}},
			Memories:       []map[string]any{{"id": "internal-memory-id", "content": "据之前转述可能有一场活动，尚未确认", "confidence": 0.4}}},
		CapabilityFacts: map[string]any{"image_generation_registered": true, "sticker_attachments": false, "vision_understanding": "disabled"}}
}

func TestFactOnlyReplyContextUsesAuthorityAndOriginalHistory(t *testing.T) {
	input := replyContextFixture()
	assignment := providerAssignment{TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
	prompt, err := buildFactOnlyReplyContext(input, assignment)
	if err != nil {
		t.Fatal(err)
	}
	wire := jsonString(prompt.Messages)
	for _, text := range []string{"林岚", "严谨直白", "约定读书讨论", "双方明确确认讨论时间", "不要再提那个旧话题", "设备型号是 e6", "尚未确认"} {
		if !strings.Contains(wire, text) {
			t.Errorf("required persona/goal/history missing: %s", text)
		}
	}
	for _, text := range []string{"internal-self-id", "internal-goal-id", "internal-message-id", "用户冷淡所以暂时不推进", "evidence_refs", "response_plan"} {
		if strings.Contains(wire, text) {
			t.Errorf("internal metadata or social plan leaked: %s", text)
		}
	}
	if stringValue(prompt.Messages[len(prompt.Messages)-1]["content"]) != input.CurrentInput || len(prompt.Tools) != 0 {
		t.Fatal("current input/tool-free contract lost")
	}
	if strings.Contains(stringValue(prompt.Messages[0]["content"]), "认知与生成准则") {
		t.Fatal("parent cognitive protocol inherited")
	}
	if stringValue(prompt.Messages[2]["role"]) != "user" || stringValue(prompt.Messages[3]["role"]) != "assistant" {
		t.Fatalf("history roles rewritten: %#v", prompt.Messages)
	}
}

func TestFactOnlyRequiredGoalOverflowIsExplicit(t *testing.T) {
	input := replyContextFixture()
	input.Projection.Goals[0]["success_criteria"] = []any{strings.Repeat("不可丢失的原始完成条件", 3000)}
	_, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 2048, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1})
	if !errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("required goal silently omitted: %v", err)
	}
}
