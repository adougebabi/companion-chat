package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// Phase 11.4/11.5 (implement.md) — the cost and serialization-size
// measurement over the four turn paths. Every input-side number comes from
// the real wire payload the fake Provider captured; the output side is the
// scripted structured reply, so the report labels it as such (a fake script
// is not a real model response and real latency is not measurable here —
// see the 11.6 annotations in implement.md).
//
// Paths (design.md 18):
//   1. plain_no_judge   — A settles, no takeover rule declared;
//   2. judge_keeps_a    — A generated, Judge declines, A settles;
//   3. judge_takeover_b — A generated, Judge approves, B generated (A's
//     output is discarded but still counted as cost);
//   4. pure_query       — main generation + one continuation synthesis, no
//     Judge (F06 mutual exclusion).

const turnPathCostReportPath = "testdata/turn_path_cost_report.json"

type turnPathCallMetric struct {
	Schema               string `json:"schema"`
	Requests             int    `json:"requests"`
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

// turnPathWireMetric collects the wire metrics of one finished scenario.
func turnPathWireMetric(t *testing.T, router *fakeProviderRouter, scriptedOutputTokens int) turnPathReport {
	t.Helper()
	report := turnPathReport{SchemaSequence: router.schemaSequence(), ScriptedOutputTokens: scriptedOutputTokens}
	report.PhysicalRequests = len(report.SchemaSequence)

	bySchema := map[string]*turnPathCallMetric{}
	order := []string{}
	for _, schema := range report.SchemaSequence {
		if _, exists := bySchema[schema]; !exists {
			bySchema[schema] = &turnPathCallMetric{Schema: schema}
			order = append(order, schema)
		}
	}
	for _, schema := range order {
		metric := bySchema[schema]
		for _, payload := range router.payloads(schema) {
			wire := jsonString(payload)
			metric.Requests++
			metric.EstimatedInputTokens += EstimatePromptTokens(payload)
			metric.Chars += utf8.RuneCountInString(wire)
			metric.Bytes += len(wire)
		}
		report.Calls = append(report.Calls, *metric)
		report.InputTokens += metric.EstimatedInputTokens
		report.Chars += metric.Chars
		report.Bytes += metric.Bytes
	}
	sort.Slice(report.Calls, func(left, right int) bool { return report.Calls[left].Schema < report.Calls[right].Schema })
	return report
}

// turnPathRunTurn drives one HandleTurn with the scenario's router and seed.
func turnPathRunTurn(t *testing.T, name string, takeoverRules []any, router *fakeProviderRouter) turnPathReport {
	t.Helper()
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "cost-owner-"+name, "cost-fluctlight-"+name, "cost-conversation-"+name
	takeoverChainSeedWithTakeoverRules(t, ctx, repository, ownerID, fluctlightID, conversationID, takeoverRules)
	app := newTestApp(t, repository, router)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, "成本测量输入", "cost-turn-"+name, "cost-turn-"+name+"-1"); err != nil {
		t.Fatal(err)
	}
	report := turnPathWireMetric(t, router, 0)
	report.Path = name
	return report
}

func TestTurnPathCostMeasurement(t *testing.T) {
	candidate := takeoverChainMainResult(takeoverChainCandidateText, nil)
	reply := takeoverChainMainResult(takeoverChainTakeoverText, nil)
	pure := takeoverChainPureQueryCandidate()
	continuation := fakeProviderResult{Structured: map[string]any{"visible_text": "我记得那件事。"}}

	reports := []turnPathReport{
		// 1. Plain: no takeover_rules declared, so no Judge is reachable.
		func() turnPathReport {
			report := turnPathRunTurn(t, "plain_no_judge", nil, newFakeProviderRouter().
				on(workingPersonaMainTurnSchema, takeoverChainSequence(candidate)))
			report.ScriptedOutputTokens = EstimatePromptTokens(jsonString(candidate.Structured))
			return report
		}(),
		// 2. Judge keeps A.
		func() turnPathReport {
			report := turnPathRunTurn(t, "judge_keeps_a", takeoverChainDefaultRules(), newFakeProviderRouter().
				on(workingPersonaMainTurnSchema, takeoverChainSequence(candidate)).
				on(takeoverJudgeSchemaName, takeoverChainJudge(false)))
			report.ScriptedOutputTokens = EstimatePromptTokens(jsonString(candidate.Structured))
			return report
		}(),
		// 3. Judge takes over B: A's output is discarded but still costed.
		func() turnPathReport {
			report := turnPathRunTurn(t, "judge_takeover_b", takeoverChainDefaultRules(), newFakeProviderRouter().
				on(workingPersonaMainTurnSchema, takeoverChainSequence(candidate)).
				on(takeoverJudgeSchemaName, takeoverChainJudge(true)).
				on(takeoverReplySchemaName, takeoverChainSequence(reply)))
			report.ScriptedOutputTokens = EstimatePromptTokens(jsonString(candidate.Structured)) + EstimatePromptTokens(jsonString(reply.Structured))
			return report
		}(),
		// 4. Pure query: continuation synthesis instead of a second stage.
		func() turnPathReport {
			report := turnPathRunTurn(t, "pure_query", takeoverChainDefaultRules(), newFakeProviderRouter().
				on(workingPersonaMainTurnSchema, takeoverChainSequence(pure)).
				on(takeoverJudgeSchemaName, takeoverChainJudge(true)).
				on(queryContinuationTestSchemaName, func(map[string]any) fakeProviderResult {
					return continuation
				}))
			report.ScriptedOutputTokens = EstimatePromptTokens(jsonString(continuation.Structured))
			return report
		}(),
	}

	// Budget invariants double as the measurement's sanity gate.
	counts := func(report turnPathReport) map[string]int {
		result := map[string]int{}
		for _, call := range report.Calls {
			result[call.Schema] = call.Requests
		}
		return result
	}
	if got := counts(reports[0]); got[workingPersonaMainTurnSchema] != 1 || got[takeoverJudgeSchemaName] != 0 || got[takeoverReplySchemaName] != 0 {
		t.Fatalf("plain path budget violated: %#v", got)
	}
	if got := counts(reports[1]); got[workingPersonaMainTurnSchema] != 1 || got[takeoverJudgeSchemaName] != 1 || got[takeoverReplySchemaName] != 0 {
		t.Fatalf("judge-keeps-a path budget violated: %#v", got)
	}
	if got := counts(reports[2]); got[workingPersonaMainTurnSchema] != 1 || got[takeoverJudgeSchemaName] != 1 || got[takeoverReplySchemaName] != 1 {
		t.Fatalf("judge-takeover-b path budget violated: %#v", got)
	}
	if got := counts(reports[3]); got[workingPersonaMainTurnSchema] != 1 || got[takeoverJudgeSchemaName] != 0 || got[queryContinuationTestSchemaName] != 1 {
		t.Fatalf("pure-query path budget violated: %#v", got)
	}
	if total := len(reports[2].SchemaSequence); total != 3 {
		t.Fatalf("the takeover path must be exactly A, Judge, B — got %d physical requests", total)
	}

	notes := []string{
		"input side: estimated from the captured wire payload (EstimatePromptTokens, ~1.25 tokens per rune heuristic)",
		"output side: scripted structured replies, not real model output; on judge_takeover_b the discarded A output is included",
		"chars/bytes: re-serialized captured payload (jsonString); key order may differ from the raw HTTP body but content is identical",
		"latency and cache hits are not measurable against a fake provider (see 11.6)",
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
			parts = append(parts, call.Schema+"×"+strconv.Itoa(call.Requests)+"="+strconv.Itoa(call.EstimatedInputTokens)+"tok/"+strconv.Itoa(call.Bytes)+"B")
		}
		t.Logf("%-16s physical=%d tokens(in)=%d tokens(out~)=%d bytes=%d %s", report.Path, report.PhysicalRequests, report.InputTokens, report.ScriptedOutputTokens, report.Bytes, strings.Join(parts, " "))
	}
}
