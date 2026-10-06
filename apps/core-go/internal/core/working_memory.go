package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	aiprompt "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/prompt"
)

type PromptFragmentKind = aiprompt.PromptFragmentKind

const (
	PromptFragmentRuntimeFact     = aiprompt.PromptFragmentRuntimeFact
	PromptFragmentActiveMemory    = aiprompt.PromptFragmentActiveMemory
	PromptFragmentResidentMemory  = aiprompt.PromptFragmentResidentMemory
	PromptFragmentRecentMessage   = aiprompt.PromptFragmentRecentMessage
	PromptFragmentRetrievedMemory = aiprompt.PromptFragmentRetrievedMemory
	PromptFragmentSummary         = aiprompt.PromptFragmentSummary
)

type PromptFragment = aiprompt.PromptFragment

type WorkingMemoryInput struct {
	RuntimeFacts       []PromptFragment
	ActiveCandidates   []PromptFragment
	ResidentCandidates []PromptFragment
	RecentMessages     []PromptFragment
	RetrievedMemories  []PromptFragment
	Summaries          []PromptFragment
}

type WorkingMemoryPolicy struct {
	RuntimeFactTokens int
	ActiveTokens      int
	ResidentTokens    int
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
	Resident     []PromptFragment   `json:"resident"`
	Recent       []PromptFragment   `json:"recent"`
	Retrieved    []PromptFragment   `json:"retrieved"`
	Summaries    []PromptFragment   `json:"summaries"`
	Trace        WorkingMemoryTrace `json:"trace"`
}

func DefaultWorkingMemoryPolicy() WorkingMemoryPolicy {
	return WorkingMemoryPolicy{RuntimeFactTokens: 8144, ActiveTokens: 2048, ResidentTokens: 1024, RecentTokens: 12288, RetrievedTokens: 3072, SummaryTokens: 2048}
}

// Section quotas guide optional selection. Required state may borrow spare
// capacity from the total wire budget; the final composer still enforces that
// budget including system instructions, Tools and the current input.
func reserveRequiredWorkingMemory(input WorkingMemoryInput, policy WorkingMemoryPolicy, maxInput int) (WorkingMemoryPolicy, error) {
	sections := []struct {
		fragments []PromptFragment
		cap       *int
		recent    bool
	}{
		{input.RuntimeFacts, &policy.RuntimeFactTokens, false},
		{input.ActiveCandidates, &policy.ActiveTokens, false},
		{input.ResidentCandidates, &policy.ResidentTokens, false},
		{input.Summaries, &policy.SummaryTokens, false},
		{input.RecentMessages, &policy.RecentTokens, true},
		{input.RetrievedMemories, &policy.RetrievedTokens, false},
	}
	for _, section := range sections {
		requiredTokens := 0
		requiredTail := len(section.fragments)
		if section.recent {
			for index, fragment := range section.fragments {
				if fragment.Required {
					requiredTail = index
					for requiredTail > 0 && fragment.GroupKey != "" && section.fragments[requiredTail-1].GroupKey == fragment.GroupKey {
						requiredTail--
					}
					break
				}
			}
		}
		for index, fragment := range section.fragments {
			if !fragment.Required && !(section.recent && index >= requiredTail) {
				continue
			}
			normalized, err := normalizePromptFragment(fragment)
			if err != nil {
				return policy, err
			}
			requiredTokens += normalized.EstimatedTokens
		}
		if requiredTokens > maxInput {
			return policy, fmt.Errorf("%w: required section=%d max_input=%d", ErrPromptRequiredBudgetExceeded, requiredTokens, maxInput)
		}
		*section.cap = max(*section.cap, requiredTokens)
	}
	return policy, nil
}

func ResolveWorkingMemory(input WorkingMemoryInput, policy WorkingMemoryPolicy) (WorkingMemory, error) {
	if policy.RuntimeFactTokens <= 0 || policy.ActiveTokens <= 0 || policy.ResidentTokens <= 0 || policy.RecentTokens <= 0 || policy.RetrievedTokens <= 0 || policy.SummaryTokens <= 0 {
		return WorkingMemory{}, errors.New("working_memory_policy_invalid")
	}
	result := WorkingMemory{Trace: WorkingMemoryTrace{Selected: []WorkingMemoryDecision{}, Dropped: []WorkingMemoryDecision{}}}
	seen := make(map[string]struct{})
	var err error
	result.RuntimeFacts, err = selectRankedPromptFragments(input.RuntimeFacts, policy.RuntimeFactTokens, seen, &result.Trace)
	if err != nil {
		return WorkingMemory{}, err
	}
	result.Active, err = selectRankedPromptFragments(input.ActiveCandidates, policy.ActiveTokens, seen, &result.Trace)
	if err != nil {
		return WorkingMemory{}, err
	}
	result.Resident, err = selectRankedPromptFragments(input.ResidentCandidates, policy.ResidentTokens, seen, &result.Trace)
	if err != nil {
		return WorkingMemory{}, err
	}
	recentSeen := make(map[string]struct{}, len(seen))
	for ref := range seen {
		recentSeen[ref] = struct{}{}
	}
	// A selected Episode covers its original message refs. Select summaries
	// before raw turns, but preserve covered raw candidates until the final
	// Prompt assembler knows whether the Summary fits the total wire budget.
	result.Summaries, err = selectRankedPromptFragments(input.Summaries, policy.SummaryTokens, seen, &result.Trace)
	if err != nil {
		return WorkingMemory{}, err
	}
	result.Recent, err = selectRecentPromptFragments(input.RecentMessages, policy.RecentTokens, recentSeen, &result.Trace)
	if err != nil {
		return WorkingMemory{}, err
	}
	for ref := range recentSeen {
		seen[ref] = struct{}{}
	}
	result.Retrieved, err = selectRankedPromptFragments(input.RetrievedMemories, policy.RetrievedTokens, seen, &result.Trace)
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
		if fragments[i].Required != fragments[j].Required {
			return fragments[i].Required
		}
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
	budgetGap := false
	for index := len(groups) - 1; index >= 0; index-- {
		value := groups[index]
		reason := ""
		for _, fragment := range value.fragments {
			if duplicate := duplicatePromptFragmentReason(fragment, seen); duplicate != "" {
				reason = duplicate
				break
			}
		}
		if reason == "" {
			if budgetGap {
				reason = "recent_contiguity_excluded"
			} else if used+value.tokens > capTokens {
				reason = "section_cap"
				budgetGap = true
			}
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
