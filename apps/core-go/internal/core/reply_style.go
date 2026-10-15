package core

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type ReplyStyleDiagnosis struct {
	Status           string   `json:"status"`
	Reasons          []string `json:"reasons,omitempty"`
	ThirdPersonCount int      `json:"third_person_count"`
	SpeechFrames     int      `json:"speech_frames"`
	RequestedWork    bool     `json:"requested_work"`
}
type ReplyStyleTrace struct {
	RawText            string              `json:"raw_candidate_text,omitempty"`
	FirstText          string              `json:"first_text,omitempty"`
	Diagnosis          ReplyStyleDiagnosis `json:"diagnosis"`
	RepairCount        int                 `json:"repair_count"`
	RepairText         string              `json:"repair_text,omitempty"`
	RepairRawText      string              `json:"repair_raw_text,omitempty"`
	RepairFinishReason string              `json:"repair_finish_reason,omitempty"`
	RepairErrorCode    string              `json:"repair_error_code,omitempty"`
	FinalText          string              `json:"final_text,omitempty"`
	FinalDiagnosis     ReplyStyleDiagnosis `json:"final_diagnosis"`
	ErrorCode          string              `json:"error_code,omitempty"`
}

var replyWorkRequest = regexp.MustCompile(`(?i)(写|讲|编|创作|朗读|念).{0,16}(小说|故事|诗歌|诗|剧本|作品)|(小说|故事|诗歌|剧本).{0,12}(续写|继续)|write.{0,20}(story|poem|fiction)`)
var replyWorkNegation = regexp.MustCompile(`(不要|别|不许|不需要).{0,12}(写|讲|小说|故事|旁白)`)
var replyNarrationFrame = regexp.MustCompile(`(她|他).{0,20}(说|问|答|笑|抬起|歪|转向|走近|摩挲|听到|目光|手指|脖颈)`)
var replySpeechFrame = regexp.MustCompile(`(她|他).{0,24}(说|问|答|笑).{0,12}[：:“「]|[”」].{0,12}(她|他).{0,12}(说|问|答|笑)`)

func replyExplicitWork(input ReplyContextInput) bool {
	text := input.CurrentInput
	if replyWorkNegation.MatchString(text) {
		return false
	}
	if replyWorkRequest.MatchString(text) {
		return true
	}
	// A short continuation can refer to the last actual user's creative request.
	if !strings.Contains(text, "继续") && !strings.Contains(text, "下一段") && !strings.Contains(text, "接着") {
		return false
	}
	for index := len(input.Projection.RecentMessages) - 1; index >= 0; index-- {
		row := input.Projection.RecentMessages[index]
		if row["kind"] != "user" || stringValue(row["text"]) == text {
			continue
		}
		previous := stringValue(row["text"])
		return replyWorkRequest.MatchString(previous) && !replyWorkNegation.MatchString(previous)
	}
	return false
}

// Conservative multi-signal shape diagnosis, not a truth/personality judge.
// A name, pronoun, quote, bracket, length or action word alone never rejects.
func diagnoseReplyStyle(text string, input ReplyContextInput) ReplyStyleDiagnosis {
	diagnosis := ReplyStyleDiagnosis{Status: "normal", RequestedWork: replyExplicitWork(input)}
	if diagnosis.RequestedWork {
		diagnosis.Status = "requested_work"
		return diagnosis
	}
	name := firstString(mapValue(input.WorkingPersona["working_persona"])["name"], firstString(mapValue(input.WorkingPersona["identity"])["name"], stringValue(input.Projection.SelfActor["display_name"])))
	if name == "" {
		return diagnosis
	}
	diagnosis.ThirdPersonCount = strings.Count(text, "她") + strings.Count(text, "他") + strings.Count(text, name)
	diagnosis.SpeechFrames = len(replySpeechFrame.FindAllString(text, -1))
	frames := len(replyNarrationFrame.FindAllString(text, -1))
	anchored := strings.HasPrefix(strings.TrimSpace(text), name) && !strings.HasPrefix(strings.TrimSpace(text), name+"：")
	paragraphs := len(strings.Split(text, "\n\n"))
	interleavedSpeech := diagnosis.SpeechFrames >= 2 || (diagnosis.SpeechFrames >= 1 && strings.Count(text, "“") >= 3 && paragraphs >= 3)
	if anchored && diagnosis.ThirdPersonCount >= 5 && frames >= 3 && interleavedSpeech && paragraphs >= 2 {
		diagnosis.Status = "high_confidence_narration"
		diagnosis.Reasons = []string{"self_named_narrator_opening", "repeated_third_person_narration", "narration_interleaved_with_quoted_speech", "multi_paragraph_scene"}
	} else if anchored && frames >= 2 {
		diagnosis.Status = "observe_only"
		diagnosis.Reasons = []string{"ambiguous_narration_shape"}
	}
	return diagnosis
}

const replyStyleRepairInstruction = `[发送前一次文体修复]
下面 private_candidate_text 是本轮尚未提交的候选消息，不是已经发生的对话或事实。当前任务需要人物通过聊天软件发送正文，不需要第三人称小说旁白。
只修正表达媒介：保留原本要表达的意图、态度、主动程度、邀约或拒绝，以及有依据的信息；不要新增事实、承诺或改变目标策略。
叙述中无依据的现场动作不能转写成第一人称真实行动。以本轮权威状态和真实工具结果为准。
不执行工具、不新增或重发媒体；已有媒体结果保持不变。只输出修复后的待发送正文。`

func (a *App) validateAndRepairReplyStyle(ctx context.Context, input ConversationReplyTaskInput, draft ReplyDraft, raw string) (result ReplyDraft, resultErr error) {
	trace := ReplyStyleTrace{RawText: raw, FirstText: draft.Text, Diagnosis: diagnoseReplyStyle(draft.Text, input.Context), FinalText: draft.Text}
	trace.FinalDiagnosis = trace.Diagnosis
	defer func() {
		if resultErr != nil {
			trace.ErrorCode = replyErrorCode(resultErr)
		}
		result.Style = trace
		result.Candidates = []ReplyStyleTrace{trace}
		a.recordDiagnosticEvent(withProviderDiagnosticSecrets(ctx, input.Assignment.Secret), "reply.style.diagnosed", "info", input.Context.Projection.FluctlightID, input.DraftRef, input.CorrelationID, map[string]any{"draft_ref": input.DraftRef, "visibility": "not_published", "trace": trace, "prompt_version": draft.ExpressionVersion})
		if trace.RepairCount > 0 && input.RecordStyleTrace != nil {
			input.RecordStyleTrace(ctx, trace)
		}
	}()
	if trace.Diagnosis.Status == "high_confidence_narration" {
		if input.Mode == "legacy_fallback" {
			return draft, newCapabilityError("reply_style_repair_unavailable", false, errors.New("fallback model cannot perform Cydonia style repair"))
		}
		if input.ClaimStyleRepair == nil {
			return draft, newCapabilityError("reply_style_repair_unavailable", false, errors.New("durable style repair claim required"))
		}
		if err := input.ClaimStyleRepair(ctx, trace); err != nil {
			return draft, newCapabilityError("reply_style_repair_unavailable", false, fmt.Errorf("style quota claim: %w", err))
		}
		trace.RepairCount = 1
		repair := input
		repair.Context.StyleRepair = true
		repair.Context.OutputCorrection = replyStyleRepairInstruction
		repair.Context.ToolFacts = append([]map[string]any(nil), input.Context.ToolFacts...)
		// The candidate is data in the existing private draft slot, never assistant
		// history. Media requests remain frozen data and cannot execute in repair.
		filtered := repair.Context.ToolFacts[:0]
		for _, fact := range repair.Context.ToolFacts {
			if fact["kind"] != "private_candidate_text" {
				filtered = append(filtered, fact)
			}
		}
		repair.Context.ToolFacts = append(filtered, map[string]any{"kind": "private_candidate_text", "value": draft.Text})
		for _, request := range draft.MediaRequests {
			repair.Context.ToolFacts = append(repair.Context.ToolFacts, map[string]any{"kind": "media_action_result", "value": map[string]any{"kind": request.Kind, "intent": request.Intent, "status": "requested_not_generated"}})
		}
		repair.CorrelationID = input.CorrelationID // physical request IDs still differ
		repaired, err := a.RunConversationReplyTask(ctx, repair)
		trace.RepairText, trace.RepairRawText, trace.RepairFinishReason = repaired.Text, repaired.RawCandidate, repaired.FinishReason
		if err != nil {
			trace.RepairErrorCode = replyErrorCode(err)
			return draft, newCapabilityError("reply_style_repair_failed", false, fmt.Errorf("style repair: %w", err))
		}
		if len(repaired.MediaRequests) > 0 {
			return draft, newCapabilityError("reply_style_repair_tools_forbidden", false, ErrUnauthorized)
		}
		trace.RepairText, trace.FinalText = repaired.Text, repaired.Text
		trace.FinalDiagnosis = diagnoseReplyStyle(repaired.Text, input.Context)
		if trace.FinalDiagnosis.Status == "high_confidence_narration" {
			return draft, newCapabilityError("reply_style_repair_failed", false, errors.New("narration remains after one repair"))
		}
		draft.Text = repaired.Text
	}
	return draft, nil
}
