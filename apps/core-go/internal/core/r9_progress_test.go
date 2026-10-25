package core

import (
	"context"
	"fmt"
	"testing"
)

func TestR9DiscoveryReadsBeforeReplyAndActualProgressReopensMedia(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	invoker := &appADKCapabilityInvoker{trace: trace, scopeRegistry: mustCapabilityRegistry(builtinCapabilities(nil)...)}
	ctx := withADKCapabilityContext(context.Background(), invoker, trace, nil)
	appendResult := func(name, status string, args, output map[string]any) {
		id := fmt.Sprint(len(trace.Results))
		trace.Append(CapabilityInvocation{CallID: id, CapabilityName: name, Arguments: jsonBytes(args)}, CapabilityResult{CallID: id, CapabilityName: name, Status: status, Output: output})
	}
	for i := 0; i < 2; i++ {
		appendResult(capabilityCatalogName, "completed", map[string]any{"names": []any{"moment.inspect"}}, map[string]any{"loaded": []any{"moment.inspect"}, "unavailable": []any{}})
	}
	if conversationLoopNeedsReply(ctx) {
		t.Fatal("catalog became facts for Writer")
	}
	choice, err := (conversationPublicationPhysicalRequestPolicy{}).DecidePhysicalModelRequest(ctx)
	if err != nil || len(choice.AllowedToolNames) != 1 || choice.AllowedToolNames[0] != "moment.inspect" {
		t.Fatal(choice, err)
	}
	for i := 0; i < 3; i++ {
		appendResult("moment.inspect", "completed", map[string]any{"limit": 10}, map[string]any{"items": []any{map[string]any{"text": "实际原文"}}})
	}
	if len(conversationLoadedQueryPhase(ctx)) != 0 || !conversationLoopNeedsReply(ctx) {
		t.Fatal("actual query failed to advance phase")
	}
	choice, err = (conversationPublicationPhysicalRequestPolicy{}).DecidePhysicalModelRequest(ctx)
	if err != nil || len(choice.AllowedToolNames) != 1 || choice.AllowedToolNames[0] != conversationReplyCapabilityName {
		t.Fatal(choice, err)
	}
	appendResult(conversationReplyCapabilityName, "accepted", nil, map[string]any{"reply_status": "needs_action", "draft_ref": "real-draft"})
	choice, err = (conversationPublicationPhysicalRequestPolicy{}).DecidePhysicalModelRequest(ctx)
	if err != nil || choice.AllowedToolNames != nil || conversationLoopNeedsReply(ctx) {
		t.Fatal("real media continuation blocked", choice, err)
	}
	appendResult("moment.inspect", "completed", map[string]any{"limit": 10}, map[string]any{"items": []any{}})
	appendResult("moment.inspect", "completed", map[string]any{"limit": 10}, map[string]any{"items": []any{}})
	appendResult(lifeActivityStartCapabilityName, "accepted", nil, map[string]any{"activity_id": "new-real-activity", "status": "in_progress"})
	appendResult("moment.inspect", "completed", map[string]any{"limit": 10}, map[string]any{"items": []any{}})
	if conversationLoopNeedsReply(ctx) {
		t.Fatal("accepted real action did not reset earlier reads")
	}
	for i := 0; i < 2; i++ {
		appendResult("uninstalled", "handled", nil, map[string]any{"warning_code": "turn_action_window_closed", "effect_applied": false})
	}
	if conversationClosedActionAttempts(trace) != 2 {
		t.Fatal("ignored phase not counted")
	}
	appendResult("wardrobe.inspect", "completed", nil, map[string]any{"items": []any{}})
	if conversationClosedActionAttempts(trace) != 0 {
		t.Fatal("actual progress did not reset rejected attempts")
	}
}
