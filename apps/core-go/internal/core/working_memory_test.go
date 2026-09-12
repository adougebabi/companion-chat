package core

import (
	"fmt"
	"testing"
)

func TestWorkingMemorySelectsNewestWholeRecentTurnsChronologically(t *testing.T) {
	recent := make([]PromptFragment, 0, 6)
	for turn := 1; turn <= 3; turn++ {
		group := fmt.Sprintf("turn-%d", turn)
		recent = append(recent,
			PromptFragment{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "user", "content": fmt.Sprintf("user-%d", turn)}, EstimatedTokens: 10, SourceRefs: []string{fmt.Sprintf("message:%d-user", turn)}, GroupKey: group},
			PromptFragment{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": "assistant", "content": fmt.Sprintf("assistant-%d", turn)}, EstimatedTokens: 10, SourceRefs: []string{fmt.Sprintf("message:%d-assistant", turn)}, GroupKey: group},
		)
	}
	policy := DefaultWorkingMemoryPolicy()
	policy.RecentTokens = 25
	result, err := ResolveWorkingMemory(WorkingMemoryInput{RecentMessages: recent}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recent) != 2 || stringValue(mapValue(result.Recent[0].Content)["content"]) != "user-3" || stringValue(mapValue(result.Recent[1].Content)["content"]) != "assistant-3" {
		t.Fatalf("recent selection = %#v", result.Recent)
	}
	if len(result.Trace.Dropped) != 4 {
		t.Fatalf("dropped decisions = %#v", result.Trace.Dropped)
	}
}

func TestWorkingMemoryDeduplicatesAcrossAuthorityLayers(t *testing.T) {
	shared := "fact:shared"
	result, err := ResolveWorkingMemory(WorkingMemoryInput{
		ActiveCandidates:  []PromptFragment{{Kind: PromptFragmentActiveMemory, Priority: 100, Content: map[string]any{"content": "active"}, EstimatedTokens: 10, SourceRefs: []string{shared}}},
		RetrievedMemories: []PromptFragment{{Kind: PromptFragmentRetrievedMemory, Priority: 100, Content: map[string]any{"content": "durable duplicate"}, EstimatedTokens: 10, SourceRefs: []string{shared}}},
		Summaries:         []PromptFragment{{Kind: PromptFragmentSummary, Priority: 10, Content: map[string]any{"summary": "summary duplicate"}, EstimatedTokens: 10, SourceRefs: []string{shared}}},
	}, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Active) != 1 || len(result.Retrieved) != 0 || len(result.Summaries) != 0 {
		t.Fatalf("authority dedupe = %#v", result)
	}
	if len(result.Trace.Dropped) != 2 || result.Trace.Dropped[0].Reason != "deduplicated" || result.Trace.Dropped[1].Reason != "deduplicated" {
		t.Fatalf("dedupe trace = %#v", result.Trace.Dropped)
	}
}

func TestWorkingMemoryPressureIsBoundedWithoutDeletingSources(t *testing.T) {
	input := WorkingMemoryInput{}
	for index := 0; index < 1000; index++ {
		role := "user"
		if index%2 == 1 {
			role = "assistant"
		}
		input.RecentMessages = append(input.RecentMessages, PromptFragment{Kind: PromptFragmentRecentMessage, Content: map[string]any{"role": role, "content": fmt.Sprintf("message-%d", index)}, EstimatedTokens: 32, SourceRefs: []string{fmt.Sprintf("message:%d", index)}, GroupKey: fmt.Sprintf("turn:%d", index/2)})
	}
	for index := 0; index < 100; index++ {
		input.RetrievedMemories = append(input.RetrievedMemories, PromptFragment{Kind: PromptFragmentRetrievedMemory, Priority: 100 - index, Content: map[string]any{"content": fmt.Sprintf("memory-%d", index)}, EstimatedTokens: 64, SourceRefs: []string{fmt.Sprintf("memory:%d", index)}})
	}
	for index := 0; index < 30; index++ {
		input.ActiveCandidates = append(input.ActiveCandidates, PromptFragment{Kind: PromptFragmentActiveMemory, Priority: 100 - index, Content: map[string]any{"content": fmt.Sprintf("active-%d", index)}, EstimatedTokens: 64, SourceRefs: []string{fmt.Sprintf("active:%d", index)}})
	}
	result, err := ResolveWorkingMemory(input, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if len(input.RecentMessages) != 1000 || len(input.RetrievedMemories) != 100 || len(input.ActiveCandidates) != 30 {
		t.Fatal("resolver mutated source collections")
	}
	if len(result.Recent) >= len(input.RecentMessages) || len(result.Retrieved) >= len(input.RetrievedMemories) || len(result.Trace.Dropped) == 0 {
		t.Fatalf("pressure selection is not bounded: recent=%d retrieved=%d active=%d dropped=%d", len(result.Recent), len(result.Retrieved), len(result.Active), len(result.Trace.Dropped))
	}
}
