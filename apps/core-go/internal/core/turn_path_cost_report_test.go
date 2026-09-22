package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// This report measures the real wire payload of three native conversation
// Agent paths. Repeated calls to the same schema stay as separate physical
// requests: a Tool result round is model work and must never be collapsed.
// Provider output is scripted, so output tokens remain an estimate and no
// latency or cache claim is made here.

const turnPathCostReportPath = "testdata/turn_path_cost_report.json"

type turnPathCallMetric struct {
	RequestIndex         int    `json:"request_index"`
	Schema               string `json:"schema"`
	EstimatedInputTokens int    `json:"estimated_input_tokens"`
	Chars                int    `json:"chars"`
	Bytes                int    `json:"bytes"`
}

type turnPathReport struct {
	Path                 string               `json:"path"`
	PhysicalRequests     int                  `json:"physical_requests"`
	SchemaSequence       []string             `json:"schema_sequence"`
	Calls                []turnPathCallMetric `json:"calls"`
	InputTokens          int                  `json:"total_estimated_input_tokens"`
	ScriptedOutputTokens int                  `json:"scripted_output_estimated_tokens"`
	Chars                int                  `json:"total_chars"`
	Bytes                int                  `json:"total_bytes"`
	Notes                []string             `json:"notes"`
}

func turnPathWireMetric(t *testing.T, router *fakeProviderRouter, scriptedOutputTokens int) turnPathReport {
	t.Helper()
	report := turnPathReport{SchemaSequence: router.schemaSequence(), ScriptedOutputTokens: scriptedOutputTokens}
	report.PhysicalRequests = len(report.SchemaSequence)
	payloadIndex := map[string]int{}
	for requestIndex, schemaName := range report.SchemaSequence {
		payloads := router.payloads(schemaName)
		index := payloadIndex[schemaName]
		if index >= len(payloads) {
			t.Fatalf("request %d schema %q has no captured wire payload", requestIndex+1, schemaName)
		}
		payloadIndex[schemaName] = index + 1
		wire := jsonString(payloads[index])
		metric := turnPathCallMetric{
			RequestIndex:         requestIndex + 1,
			Schema:               schemaName,
			EstimatedInputTokens: EstimatePromptTokens(payloads[index]),
			Chars:                utf8.RuneCountInString(wire),
			Bytes:                len(wire),
		}
		report.Calls = append(report.Calls, metric)
		report.InputTokens += metric.EstimatedInputTokens
		report.Chars += metric.Chars
		report.Bytes += metric.Bytes
	}
	return report
}

func turnPathScriptedOutputTokens(results ...fakeProviderResult) int {
	total := 0
	for _, result := range results {
		total += EstimatePromptTokens(jsonString(map[string]any{
			"structured": result.Structured,
			"tool_calls": result.ToolCalls,
		}))
	}
	return total
}

func turnPathRunTurn(t *testing.T, name string, router *fakeProviderRouter, expectedReceipts ...string) turnPathReport {
	t.Helper()
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "cost-owner-"+name, "cost-fluctlight-"+name, "cost-conversation-"+name
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "成本测量输入", "cost-turn-"+name, "cost-turn-"+name+"-1")); err != nil {
		t.Fatal(err)
	}
	if len(expectedReceipts) > 0 {
		results := nativePersonaTrace(t, ctx, repository, "cost-turn-"+name)
		for _, capabilityName := range expectedReceipts {
			result := nativePersonaResultByName(t, results, capabilityName)
			if firstString(result["status"], stringValue(result["Status"])) != "completed" {
				t.Fatalf("%s receipt was not committed: %#v", capabilityName, result)
			}
		}
	}
	report := turnPathWireMetric(t, router, 0)
	report.Path = name
	return report
}

func TestTurnPathCostMeasurement(t *testing.T) {
	naturalFinal := fakeProviderResult{Structured: map[string]any{
		"action_type": "reply", "response_intent": "natural final", "visible_text": "自然回复", "influences": []any{},
	}}
	replyCall := fakeProviderResult{ToolCalls: []map[string]any{
		nativePersonaToolCall("cost-reply", "conversation.reply", map[string]any{"text": "已提交回复"}),
	}}
	takeoverCall := fakeProviderResult{ToolCalls: []map[string]any{
		nativePersonaToolCall("cost-takeover", personaTakeoverCapabilityName, map[string]any{
			"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt",
			"source_profile_id": "spark", "target_profile_id": "twilight", "reason": "cost path",
		}),
	}}
	takeoverReplyCall := fakeProviderResult{ToolCalls: []map[string]any{
		nativePersonaToolCall("cost-takeover-reply", "conversation.reply", map[string]any{"text": "接管后已提交回复"}),
	}}
	finalAfterTool := nativePersonaFinal()

	reports := []turnPathReport{
		func() turnPathReport {
			router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult { return naturalFinal })
			report := turnPathRunTurn(t, "natural_final", router)
			report.ScriptedOutputTokens = turnPathScriptedOutputTokens(naturalFinal)
			return report
		}(),
		func() turnPathReport {
			step := 0
			results := []fakeProviderResult{replyCall, finalAfterTool}
			router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
				result := results[step]
				step++
				return result
			})
			report := turnPathRunTurn(t, "reply_tool_then_final", router, "conversation.reply")
			report.ScriptedOutputTokens = turnPathScriptedOutputTokens(results...)
			return report
		}(),
		func() turnPathReport {
			step := 0
			results := []fakeProviderResult{takeoverCall, takeoverReplyCall, finalAfterTool}
			router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
				result := results[step]
				step++
				return result
			})
			report := turnPathRunTurn(t, "takeover_reply_then_final", router, personaTakeoverCapabilityName, "conversation.reply")
			report.ScriptedOutputTokens = turnPathScriptedOutputTokens(results...)
			return report
		}(),
	}

	for index, want := range []int{1, 2, 3} {
		report := reports[index]
		if report.PhysicalRequests != want || len(report.Calls) != want || len(report.SchemaSequence) != want {
			t.Fatalf("%s physical requests=%d calls=%d schemas=%d, want %d each", report.Path, report.PhysicalRequests, len(report.Calls), len(report.SchemaSequence), want)
		}
		for callIndex, call := range report.Calls {
			if call.RequestIndex != callIndex+1 || call.Schema != workingPersonaMainTurnSchema {
				t.Fatalf("%s physical call %d was collapsed or relabeled: %#v", report.Path, callIndex+1, call)
			}
		}
	}

	notes := []string{
		"input side: estimated independently for every captured physical wire request (EstimatePromptTokens, ~1.25 tokens per rune heuristic)",
		"output side: scripted native ToolCalls/final DTOs, not real model output",
		"repeated conversation_turn_response entries are distinct model decisions and are never collapsed",
		"chars/bytes: re-serialized captured payload (jsonString); key order may differ from the raw HTTP body but content is identical",
		"latency and cache hits are not measurable against a fake provider",
	}
	for index := range reports {
		reports[index].Notes = notes
	}

	if err := os.MkdirAll(filepath.Dir(turnPathCostReportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(turnPathCostReportPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, report := range reports {
		var parts []string
		for _, call := range report.Calls {
			parts = append(parts, "#"+strconv.Itoa(call.RequestIndex)+"="+strconv.Itoa(call.EstimatedInputTokens)+"tok/"+strconv.Itoa(call.Bytes)+"B")
		}
		t.Logf("%-26s physical=%d tokens(in)=%d tokens(out~)=%d bytes=%d %s", report.Path, report.PhysicalRequests, report.InputTokens, report.ScriptedOutputTokens, report.Bytes, strings.Join(parts, " "))
	}
}
