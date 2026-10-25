package core

import (
	"context"
	"strings"
	"testing"
)

func TestR7WriterReceivesScopedActualActionFailureWithoutSelectedRefs(t *testing.T) {
	app := &App{}
	service := newFactOnlyReplyService(app)
	reply := CapabilityInvocation{CallID: "reply", CapabilityName: conversationReplyCapabilityName, SourceFactID: "current-fact", Metadata: InvocationMetadata{AuthorizationActorID: "owner", FluctlightID: "self", ConversationID: "conversation"}}
	trace := &ADKCapabilityTrace{}
	for _, scenario := range []string{"actual", "foreign_owner", "foreign_self", "foreign_conversation", "old_source", "inspection"} {
		call := reply
		call.CallID = scenario
		call.CapabilityName = wardrobeWearCapabilityName
		call.Arguments = jsonBytes(map[string]any{"text": "FORGED_ACTION_SUCCESS", "mutation_committed": true})
		switch scenario {
		case "foreign_owner":
			call.Metadata.AuthorizationActorID = "other"
		case "foreign_self":
			call.Metadata.FluctlightID = "other"
		case "foreign_conversation":
			call.Metadata.ConversationID = "other"
		case "old_source":
			call.SourceFactID = "old"
		case "inspection":
			call.CapabilityName = wardrobeInspectCapabilityName
		}
		trace.AppendInvocation(call)
		trace.AppendResult(CapabilityResult{CallID: scenario, CapabilityName: call.CapabilityName, Status: "handled", Output: map[string]any{"disposition": "awaiting_state", "warning_code": "wardrobe_item_unavailable", "effect_applied": false, "mutation_committed": false, "retry_same_arguments": false, "detail": "DO_NOT_FORWARD_RAW_DETAIL"}})
	}
	ctx := WithADKCapabilityInvoker(context.Background(), &appADKCapabilityInvoker{app: app}, trace)
	facts := factOnlyReplyToolFacts(service.currentActionFacts(ctx, reply))
	if len(facts) != 1 {
		t.Fatalf("wrong scoped action facts: %#v", facts)
	}
	value := mapValue(facts[0]["value"])
	observed := mapValue(value["observed"])
	if value["status"] != "handled" || observed["warning_code"] != "wardrobe_item_unavailable" || observed["mutation_committed"] != false || observed["effect_applied"] != false {
		t.Fatal("receipt truth was narrowed away", facts)
	}
	if strings.Contains(jsonString(facts), "FORGED") || strings.Contains(jsonString(facts), "RAW_DETAIL") {
		t.Fatal("model arguments or raw error detail entered Writer facts")
	}
	if len(service.currentActionFacts(context.Background(), reply)) != 0 {
		t.Fatal("missing native trace invented evidence")
	}
}

func TestR7ActionReviewOnlyChecksUnsuccessfulWear(t *testing.T) {
	for _, sample := range []struct {
		name, status, disposition, capability string
		failedEffect, want                    bool
	}{
		{"failed", "failed", "", wardrobeWearCapabilityName, false, true},
		{"unavailable", "handled", "awaiting_state", wardrobeWearCapabilityName, true, true},
		{"already_worn", "handled", "no_change", wardrobeWearCapabilityName, true, false},
		{"success", "completed", "committed", wardrobeWearCapabilityName, false, false},
		{"accepted", "accepted", "", wardrobeWearCapabilityName, true, false},
		{"other_domain", "failed", "", "memory_event", true, false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			observed := map[string]any{"disposition": sample.disposition}
			if sample.failedEffect {
				observed["effect_applied"] = false
			}
			facts := []map[string]any{{"kind": "current_action_result", "value": map[string]any{"capability_name": sample.capability, "status": sample.status, "observed": observed}}}
			if replyNeedsActionCheck(facts) != sample.want {
				t.Fatal("wrong review boundary", facts)
			}
		})
	}
}

func TestR7FactCorrectionPreservesStyleHistoryAndFrozenMedia(t *testing.T) {
	input := privateChatExpressionFixture(t, true)
	input.Projection.CorePersona = map[string]any{"identity": map[string]any{"name": "苏晚"}}
	original := ReplyDraft{Text: "原草稿", Style: ReplyStyleTrace{RepairCount: 1, FirstText: "原文体候选"}, Candidates: []ReplyStyleTrace{{FirstText: "原文体候选"}}, MediaRequests: []ReplyMediaRequest{{Kind: "image", Intent: "已冻结画面"}}, AttachmentRefs: []string{"actual-asset"}}
	valid, err := adoptReplyActionCorrection(input, original, ReplyDraft{Text: "这次还没能换上，先保持原样。"})
	if err != nil || valid.Style.RepairCount != 1 || len(valid.Candidates) != 2 || len(valid.MediaRequests) != 1 || valid.AttachmentRefs[0] != "actual-asset" {
		t.Fatal("repair history or media changed", valid, err)
	}
	narration := "苏晚抬起眼看着她。她笑着说：\"好了\"。\n\n她转向窗外，苏晚摩挲着手指。她问：\"现在呢\"。\n\n她听到声音，苏晚走近，她笑着说：\"来吧\"。"
	if diagnoseReplyStyle(narration, input).Status != "high_confidence_narration" {
		t.Fatal("invalid style fixture")
	}
	if _, err := adoptReplyActionCorrection(input, original, ReplyDraft{Text: narration}); err == nil {
		t.Fatal("fact correction bypassed the style gate")
	}
	input.ExpressionVersion = factOnlyReplyLegacyPromptVersion
	if _, err := adoptReplyActionCorrection(input, original, ReplyDraft{Text: narration}); err != nil {
		t.Fatal("fact correction imposed v5 style on a frozen legacy contract", err)
	}
}
