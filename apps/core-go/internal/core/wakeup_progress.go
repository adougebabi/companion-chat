package core

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cloudwego/eino/schema"
)

// WakeUp owns its termination policy. A stalled background run must never be
// forced into conversation.reply merely to finish a private-conversation turn.
type wakeUpPhysicalRequestPolicy struct{}

func (wakeUpPhysicalRequestPolicy) DecidePhysicalModelRequest(ctx context.Context) (physicalModelRequestDecision, error) {
	if err := ctx.Err(); err != nil {
		return physicalModelRequestDecision{}, err
	}
	if conversationLoopNeedsReply(ctx) {
		return physicalModelRequestDecision{}, newCapabilityError("tool_progress_stalled", false, errors.New("tool_progress_stalled"))
	}
	return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceAllowed}, nil
}

func wakeUpRepeatedRejectedArguments(trace *ADKCapabilityTrace, name string, arguments json.RawMessage) bool {
	if trace == nil {
		return false
	}
	calls, results := trace.Snapshot()
	byID := map[string]CapabilityInvocation{}
	for _, call := range calls {
		byID[call.CallID] = call
	}
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		// A committed effect can change a precondition; permit reconsideration.
		if result.Status == "accepted" || result.Status == "completed" && result.CapabilityName != capabilityCatalogName {
			return false
		}
		out := mapValue(result.Output)
		if result.CapabilityName == name && result.Status == "handled" && out["effect_applied"] == false && out["retry_same_arguments"] == false {
			call, exists := byID[result.CallID]
			if exists && jsonString(decodeObject(call.Arguments)) == jsonString(decodeObject(arguments)) {
				return true
			}
		}
	}
	return false
}
