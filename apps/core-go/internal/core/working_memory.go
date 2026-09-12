package core

import (
	"errors"
	"sort"
	"strings"
)

type PromptFragmentKind string

const (
	PromptFragmentRuntimeFact     PromptFragmentKind = "runtime_fact"
	PromptFragmentActiveMemory    PromptFragmentKind = "active_memory"
	PromptFragmentRecentMessage   PromptFragmentKind = "recent_message"
	PromptFragmentRetrievedMemory PromptFragmentKind = "retrieved_memory"
	PromptFragmentSummary         PromptFragmentKind = "conversation_summary"
)

type PromptFragment struct {
	Kind            PromptFragmentKind `json:"kind"`
	Priority        int                `json:"priority"`
	Required        bool               `json:"required"`
	Content         any                `json:"content"`
	EstimatedTokens int                `json:"estimated_tokens"`
	SourceRefs      []string           `json:"source_refs"`
	GroupKey        string             `json:"group_key,omitempty"`
}

type WorkingMemoryInput struct {
	RuntimeFacts      []PromptFragment
	ActiveCandidates  []PromptFragment
	RecentMessages    []PromptFragment
	RetrievedMemories []PromptFragment
	Summaries         []PromptFragment
}

type WorkingMemoryPolicy struct {
	RuntimeFactTokens int
	ActiveTokens      int
	RecentTokens      int
	RetrievedTokens   int
	SummaryTokens     int
}

type WorkingMemoryDecision struct {
	Kind       PromptFragmentKind `json:"kind"`
	SourceRefs []string           `json:"source_refs"`
	Tokens     int                `json:"tokens"`
	Selected   bool               `json:"selected"`
	Reason     string             `json:"reason"`
}

type WorkingMemoryTrace struct {
	Selected []WorkingMemoryDecision `json:"selected"`
	Dropped  []WorkingMemoryDecision `json:"dropped"`
}

type WorkingMemory struct {
	RuntimeFacts []PromptFragment   `json:"runtime_facts"`
	Active       []PromptFragment   `json:"active"`
	Recent       []PromptFragment   `json:"recent"`
	Retrieved    []PromptFragment   `json:"retrieved"`
	Summaries    []PromptFragment   `json:"summaries"`
	Trace        WorkingMemoryTrace `json:"trace"`
}

func DefaultWorkingMemoryPolicy() WorkingMemoryPolicy {
	return WorkingMemoryPolicy{RuntimeFactTokens: 6144, ActiveTokens: 2048, RecentTokens: 8192, RetrievedTokens: 3072, SummaryTokens: 2048}
}

func ResolveWorkingMemory(input WorkingMemoryInput, policy WorkingMemoryPolicy) (WorkingMemory, error) {
	if policy.RuntimeFactTokens <= 0 || policy.ActiveTokens <= 0 || policy.RecentTokens <= 0 || policy.RetrievedTokens <= 0 || policy.SummaryTokens <= 0 {
		return WorkingMemory{}, errors.New("working_memory_policy_invalid")
	}
	result := WorkingMemory{Trace: WorkingMemoryTrace{Selected: []WorkingMemoryDecision{}, Dropped: []WorkingMemoryDecision{}}}
	seen := make(map[string]struct{})
	var err error
	result.Active, err = selectRankedPromptFragments(input.ActiveCandidates, policy.ActiveTokens, seen, &result.Trace)
	if err != nil {
		return WorkingMemory{}, err
	}
	result.RuntimeFacts, err = selectRankedPromptFragments(input.RuntimeFacts, policy.RuntimeFactTokens, seen, &result.Trace)
	if err != nil {
		return WorkingMemory{}, err
	}
	result.Recent, err = selectRecentPromptFragments(input.RecentMessages, policy.RecentTokens, seen, &result.Trace)
	if err != nil {
		return WorkingMemory{}, err
	}
	result.Retrieved, err = selectRankedPromptFragments(input.RetrievedMemories, policy.RetrievedTokens, seen, &result.Trace)
	if err != nil {
		return WorkingMemory{}, err
	}
	result.Summaries, err = selectRankedPromptFragments(input.Summaries, policy.SummaryTokens, seen, &result.Trace)
	if err != nil {
		return WorkingMemory{}, err
	}
	return result, nil
}

func normalizePromptFragment(fragment PromptFragment) (PromptFragment, error) {
	if fragment.Kind == "" || fragment.Content == nil {
		return PromptFragment{}, errors.New("prompt_fragment_invalid")
	}
	if fragment.EstimatedTokens <= 0 {
		fragment.EstimatedTokens = EstimatePromptTokens(fragment.Content) + 4
	}
	if fragment.EstimatedTokens <= 0 {
		return PromptFragment{}, errors.New("prompt_fragment_estimate_invalid")
	}
	fragment.SourceRefs = sortedUniqueStrings(fragment.SourceRefs)
	for _, ref := range fragment.SourceRefs {
		if strings.TrimSpace(ref) == "" || len([]rune(ref)) > 256 {
			return PromptFragment{}, errors.New("prompt_fragment_source_invalid")
		}
	}
	return fragment, nil
}

func selectRankedPromptFragments(input []PromptFragment, capTokens int, seen map[string]struct{}, trace *WorkingMemoryTrace) ([]PromptFragment, error) {
	fragments := append([]PromptFragment(nil), input...)
	for index := range fragments {
		normalized, err := normalizePromptFragment(fragments[index])
		if err != nil {
			return nil, err
		}
		fragments[index] = normalized
	}
	sort.SliceStable(fragments, func(i, j int) bool {
		if fragments[i].Priority != fragments[j].Priority {
			return fragments[i].Priority > fragments[j].Priority
		}
		return jsonString(fragments[i].SourceRefs) < jsonString(fragments[j].SourceRefs)
	})
	selected := make([]PromptFragment, 0, len(fragments))
	used := 0
	for _, fragment := range fragments {
		reason := duplicatePromptFragmentReason(fragment, seen)
		if reason == "" && used+fragment.EstimatedTokens > capTokens {
			reason = "section_cap"
		}
		if reason != "" {
			if fragment.Required {
				return nil, errors.New("working_memory_required_budget_exceeded")
			}
			trace.Dropped = append(trace.Dropped, workingMemoryDecision(fragment, false, reason))
			continue
		}
		used += fragment.EstimatedTokens
		selected = append(selected, fragment)
		markPromptFragmentSources(fragment, seen)
		trace.Selected = append(trace.Selected, workingMemoryDecision(fragment, true, "selected"))
	}
	return selected, nil
}

func selectRecentPromptFragments(input []PromptFragment, capTokens int, seen map[string]struct{}, trace *WorkingMemoryTrace) ([]PromptFragment, error) {
	fragments := make([]PromptFragment, len(input))
	for index := range input {
		normalized, err := normalizePromptFragment(input[index])
		if err != nil {
			return nil, err
		}
		if normalized.Kind != PromptFragmentRecentMessage {
			return nil, errors.New("working_memory_recent_kind_invalid")
		}
		message := mapValue(normalized.Content)
		role := strings.TrimSpace(stringValue(message["role"]))
		if (role != "user" && role != "assistant") || strings.TrimSpace(stringValue(message["content"])) == "" {
			return nil, errors.New("working_memory_recent_message_invalid")
		}
		fragments[index] = normalized
	}
	type group struct {
		fragments []PromptFragment
		tokens    int
	}
	groups := make([]group, 0)
	for index := 0; index < len(fragments); {
		key := strings.TrimSpace(fragments[index].GroupKey)
		if key == "" {
			key = "message:" + jsonString(fragments[index].SourceRefs)
		}
		end := index + 1
		for end < len(fragments) && strings.TrimSpace(fragments[end].GroupKey) == key && strings.TrimSpace(fragments[end].GroupKey) != "" {
			end++
		}
		value := group{fragments: append([]PromptFragment(nil), fragments[index:end]...)}
		for _, fragment := range value.fragments {
			value.tokens += fragment.EstimatedTokens
		}
		groups = append(groups, value)
		index = end
	}
	selectedGroups := make([]group, 0, len(groups))
	used := 0
	for index := len(groups) - 1; index >= 0; index-- {
		value := groups[index]
		reason := ""
		for _, fragment := range value.fragments {
			if duplicate := duplicatePromptFragmentReason(fragment, seen); duplicate != "" {
				reason = duplicate
				break
			}
		}
		if reason == "" && used+value.tokens > capTokens {
			reason = "section_cap"
		}
		if reason != "" {
			for _, fragment := range value.fragments {
				if fragment.Required {
					return nil, errors.New("working_memory_required_budget_exceeded")
				}
				trace.Dropped = append(trace.Dropped, workingMemoryDecision(fragment, false, reason))
			}
			continue
		}
		used += value.tokens
		selectedGroups = append(selectedGroups, value)
		for _, fragment := range value.fragments {
			markPromptFragmentSources(fragment, seen)
			trace.Selected = append(trace.Selected, workingMemoryDecision(fragment, true, "selected"))
		}
	}
	selected := make([]PromptFragment, 0)
	for index := len(selectedGroups) - 1; index >= 0; index-- {
		selected = append(selected, selectedGroups[index].fragments...)
	}
	return selected, nil
}

func duplicatePromptFragmentReason(fragment PromptFragment, seen map[string]struct{}) string {
	for _, ref := range fragment.SourceRefs {
		if _, duplicate := seen[ref]; duplicate {
			return "deduplicated"
		}
	}
	return ""
}

func markPromptFragmentSources(fragment PromptFragment, seen map[string]struct{}) {
	for _, ref := range fragment.SourceRefs {
		seen[ref] = struct{}{}
	}
}

func workingMemoryDecision(fragment PromptFragment, selected bool, reason string) WorkingMemoryDecision {
	return WorkingMemoryDecision{Kind: fragment.Kind, SourceRefs: append([]string(nil), fragment.SourceRefs...), Tokens: fragment.EstimatedTokens, Selected: selected, Reason: reason}
}
