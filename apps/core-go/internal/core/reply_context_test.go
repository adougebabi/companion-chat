package core

import (
	"errors"
	"fmt"
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

func TestFactOnlyReplyHistoryIsBoundedAndUsesExistingSummary(t *testing.T) {
	for _, prose := range []string{"旧轮对话内容，需要从候选窗口中裁掉。", "Older conversational material should be trimmed as whole turns. "} {
		t.Run(prose[:6], func(t *testing.T) {
			input := replyContextFixture()
			input.Projection.RecentMessages = nil
			for i := 0; i < 100; i++ {
				for _, role := range []string{"user", "assistant"} {
					input.Projection.RecentMessages = append(input.Projection.RecentMessages, map[string]any{"id": fmt.Sprintf("old-%d-%s", i, role), "turn_id": fmt.Sprint(i), "kind": role, "text": fmt.Sprintf("OLD-%d-%s %s", i, role, strings.Repeat(prose, 8))})
				}
			}
			input.Projection.RecentMessages = append(input.Projection.RecentMessages,
				map[string]any{"id": "correction", "turn_id": "recent", "kind": "user", "text": "最近明确拒绝：不要安排我参加活动。"},
				map[string]any{"id": "ack", "turn_id": "recent", "kind": "assistant", "text": "明白，我保留这个约束。"},
				map[string]any{"id": "current", "turn_id": "current", "kind": "user", "text": input.CurrentInput})
			input.Summaries = []map[string]any{{"summary": "历史约定：读书讨论；不安排用户参加活动。", "required": true, "to_sequence": 200,
				"source_message_refs": []any{"message:old-0-user", "message:old-0-assistant"}}}
			before := jsonString(input)
			prompt, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
			if err != nil {
				t.Fatal(err)
			}
			wire := jsonString(prompt.Messages)
			if prompt.Trace.EstimatedInputTokens > factOnlyReplyInputTargetTokens {
				t.Fatalf("Writer input not bounded: %+v", prompt.Trace)
			}
			for _, required := range []string{"最近明确拒绝", "不要安排我参加活动", "历史约定", "双方明确确认讨论时间", "严谨直白"} {
				if !strings.Contains(wire, required) {
					t.Fatalf("protected context missing: %s", required)
				}
			}
			if strings.Contains(wire, "OLD-0-") || strings.Count(wire, input.CurrentInput) != 1 || before != jsonString(input) || len(prompt.Tools) != 1 {
				t.Fatal("history trim/current-input/isolation contract lost")
			}
			selection := mapValue(prompt.Diagnostics["reply_context_selection"])
			if len(prompt.Trace.Dropped) == 0 || intValue(selection["history_selected_chars"])*5 >= intValue(selection["history_candidate_chars"]) {
				t.Fatalf("history not reduced: %v", selection)
			}
			t.Logf("history chars %d -> %d; final wire chars=%d estimated tokens=%d", intValue(selection["history_candidate_chars"]), intValue(selection["history_selected_chars"]), prompt.Trace.WireChars, prompt.Trace.EstimatedInputTokens)
		})
	}
}

func TestFactOnlyReplySummaryReplacesCoveredWholeTurn(t *testing.T) {
	input := replyContextFixture()
	input.Projection.RecentMessages = append([]map[string]any{{"id": "covered-user", "kind": "user", "turn_id": "covered", "text": "OLD-COVERED-USER"}, {"id": "covered-assistant", "kind": "assistant", "turn_id": "covered", "text": "OLD-COVERED-ASSISTANT"}}, input.Projection.RecentMessages...)
	input.Projection.RecentMessages = append(input.Projection.RecentMessages, map[string]any{"id": "latest", "kind": "user", "turn_id": "latest", "text": "最新的补充"})
	input.Summaries = []map[string]any{{"summary": "有来源的历史摘要", "required": true, "source_message_refs": []any{"message:covered-user", "message:covered-assistant"}}}
	prompt, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if wire := jsonString(prompt.Messages); strings.Contains(wire, "OLD-COVERED") || !strings.Contains(wire, "有来源的历史摘要") {
		t.Fatal(wire)
	}
	found := false
	for _, dropped := range prompt.Trace.Dropped {
		found = found || dropped.Reason == "summarized_source"
	}
	if !found {
		t.Fatal("summary coverage not recorded")
	}
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
	if stringValue(prompt.Messages[len(prompt.Messages)-1]["content"]) != input.CurrentInput || len(prompt.Tools) != 1 {
		t.Fatal("current input/private image tool contract lost")
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
