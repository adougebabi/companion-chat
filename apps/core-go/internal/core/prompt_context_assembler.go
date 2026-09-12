package core

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultContextWindowTokens      = 131072
	defaultMaxInputTokens           = 98304
	defaultOutputReserveTokens      = 4096
	defaultPromptSafetyMarginTokens = 4096
	promptBudgetPolicyVersionV1     = "prompt-budget.v1"
	defaultSystemTokensCap          = 8192
	defaultToolsSchemaTokensCap     = 16384
	defaultCurrentInputTokensCap    = 16384
)

var ErrPromptRequiredBudgetExceeded = errors.New("prompt_required_budget_exceeded")

type PromptBudgetPolicy struct {
	Version               string `json:"version"`
	ContextWindowTokens   int    `json:"context_window_tokens"`
	MaxInputTokens        int    `json:"max_input_tokens"`
	OutputReserveTokens   int    `json:"output_reserve_tokens"`
	SafetyMarginTokens    int    `json:"safety_margin_tokens"`
	SystemTokensCap       int    `json:"system_tokens_cap"`
	ToolsSchemaTokensCap  int    `json:"tools_schema_tokens_cap"`
	CurrentInputTokensCap int    `json:"current_input_tokens_cap"`
}

func DefaultPromptBudgetPolicy(outputReserve int) PromptBudgetPolicy {
	if outputReserve <= 0 {
		outputReserve = defaultOutputReserveTokens
	}
	return PromptBudgetPolicy{
		Version: promptBudgetPolicyVersionV1, ContextWindowTokens: defaultContextWindowTokens,
		MaxInputTokens: defaultMaxInputTokens, OutputReserveTokens: outputReserve,
		SafetyMarginTokens: defaultPromptSafetyMarginTokens, SystemTokensCap: defaultSystemTokensCap,
		ToolsSchemaTokensCap: defaultToolsSchemaTokensCap, CurrentInputTokensCap: defaultCurrentInputTokensCap,
	}
}

func promptSafetyMargin(policyVersion string) (int, error) {
	if strings.TrimSpace(policyVersion) != promptBudgetPolicyVersionV1 {
		return 0, errors.New("prompt_budget_policy_unknown")
	}
	return defaultPromptSafetyMarginTokens, nil
}

func validatePromptBudgetConfiguration(contextWindow, maxInput, outputReserve int, policyVersion string) error {
	margin, err := promptSafetyMargin(policyVersion)
	if err != nil {
		return err
	}
	if contextWindow <= 0 || maxInput <= 0 || outputReserve <= 0 || maxInput > contextWindow || maxInput+outputReserve+margin > contextWindow {
		return errors.New("provider_prompt_budget_invalid")
	}
	return nil
}

func (policy PromptBudgetPolicy) Validate() error {
	if policy.SystemTokensCap <= 0 || policy.ToolsSchemaTokensCap <= 0 || policy.CurrentInputTokensCap <= 0 || policy.SafetyMarginTokens != defaultPromptSafetyMarginTokens {
		return errors.New("prompt_budget_policy_invalid")
	}
	return validatePromptBudgetConfiguration(policy.ContextWindowTokens, policy.MaxInputTokens, policy.OutputReserveTokens, policy.Version)
}

func EstimatePromptTokens(value any) int {
	var text string
	if raw, ok := value.(string); ok {
		text = raw
	} else {
		encoded, err := json.Marshal(value)
		if err != nil {
			return 0
		}
		text = string(encoded)
	}
	bytesUnits := (len([]byte(text)) + 2) / 3
	runeUnits := len([]rune(text))
	if runeUnits > bytesUnits {
		bytesUnits = runeUnits
	}
	return (bytesUnits*5 + 3) / 4
}

func estimateProviderMessageTokens(message map[string]any) int {
	return EstimatePromptTokens(message) + 4
}

type PromptAssemblyInput struct {
	Role           string
	OperationRules []string
	CorePersona    map[string]any
	WorkingMemory  WorkingMemory
	CurrentInput   string
	Tools          []map[string]any
	ResponseFormat map[string]any
	Policy         PromptBudgetPolicy
}

type PromptAssemblyDecision struct {
	Kind       PromptFragmentKind `json:"kind"`
	SourceRefs []string           `json:"source_refs"`
	Tokens     int                `json:"tokens"`
	Reason     string             `json:"reason"`
}

type PromptAssemblyTrace struct {
	PolicyVersion        string                   `json:"policy_version"`
	ContextWindowTokens  int                      `json:"context_window_tokens"`
	MaxInputTokens       int                      `json:"max_input_tokens"`
	OutputReserveTokens  int                      `json:"output_reserve_tokens"`
	SafetyMarginTokens   int                      `json:"safety_margin_tokens"`
	EstimatedInputTokens int                      `json:"estimated_input_tokens"`
	SectionTokens        map[string]int           `json:"section_tokens"`
	Selected             []PromptAssemblyDecision `json:"selected"`
	Dropped              []PromptAssemblyDecision `json:"dropped"`
}

type PromptAssemblyResult struct {
	Messages       []map[string]any    `json:"messages"`
	Tools          []map[string]any    `json:"tools"`
	ResponseFormat map[string]any      `json:"response_format"`
	Trace          PromptAssemblyTrace `json:"trace"`
	Diagnostics    map[string]any      `json:"-"`
}

type promptOptionalCandidate struct {
	fragment PromptFragment
	order    int
	unitKey  string
}

func AssemblePromptContext(input PromptAssemblyInput) (PromptAssemblyResult, error) {
	if err := input.Policy.Validate(); err != nil {
		return PromptAssemblyResult{}, err
	}
	input.CurrentInput = strings.TrimSpace(input.CurrentInput)
	if input.CurrentInput == "" {
		return PromptAssemblyResult{}, errors.New("prompt_current_input_required")
	}
	system := map[string]any{"role": "system", "content": renderProviderSystem(input.OperationRules, filterCorePersona(input.CorePersona), nil, input.Role)}
	current := map[string]any{"role": "user", "content": input.CurrentInput}
	systemTokens := estimateProviderMessageTokens(system)
	currentTokens := estimateProviderMessageTokens(current)
	toolsTokens := EstimatePromptTokens(input.Tools)
	schemaTokens := EstimatePromptTokens(input.ResponseFormat)
	if systemTokens > input.Policy.SystemTokensCap || currentTokens > input.Policy.CurrentInputTokensCap || toolsTokens+schemaTokens > input.Policy.ToolsSchemaTokensCap {
		return PromptAssemblyResult{}, ErrPromptRequiredBudgetExceeded
	}
	requiredTokens := systemTokens + currentTokens + toolsTokens + schemaTokens + 16
	if requiredTokens > input.Policy.MaxInputTokens {
		return PromptAssemblyResult{}, ErrPromptRequiredBudgetExceeded
	}
	candidates, err := promptOptionalCandidates(input.WorkingMemory, input.CurrentInput)
	if err != nil {
		return PromptAssemblyResult{}, err
	}
	selected := make([]promptOptionalCandidate, 0, len(candidates))
	trace := PromptAssemblyTrace{
		PolicyVersion: input.Policy.Version, ContextWindowTokens: input.Policy.ContextWindowTokens,
		MaxInputTokens: input.Policy.MaxInputTokens, OutputReserveTokens: input.Policy.OutputReserveTokens,
		SafetyMarginTokens: input.Policy.SafetyMarginTokens, SectionTokens: map[string]int{},
		Selected: []PromptAssemblyDecision{}, Dropped: []PromptAssemblyDecision{},
	}
	used := requiredTokens
	for index := 0; index < len(candidates); {
		end := index + 1
		for end < len(candidates) && candidates[end].unitKey == candidates[index].unitKey {
			end++
		}
		unit := append([]promptOptionalCandidate(nil), candidates[index:end]...)
		unitCost := 0
		for position := range unit {
			cost := unit[position].fragment.EstimatedTokens
			if unit[position].fragment.Kind == PromptFragmentRecentMessage {
				cost = estimateProviderMessageTokens(mapValue(unit[position].fragment.Content))
			}
			unit[position].fragment.EstimatedTokens = cost
			unitCost += cost
		}
		if used+unitCost > input.Policy.MaxInputTokens {
			for _, candidate := range unit {
				trace.Dropped = append(trace.Dropped, promptAssemblyDecision(candidate.fragment, "total_cap"))
			}
			index = end
			continue
		}
		used += unitCost
		selected = append(selected, unit...)
		for _, candidate := range unit {
			trace.Selected = append(trace.Selected, promptAssemblyDecision(candidate.fragment, "selected"))
		}
		index = end
	}
	messages := assemblePromptMessages(system, current, selected)
	total := estimatePromptWireInput(messages, input.Tools, input.ResponseFormat)
	for total > input.Policy.MaxInputTokens && len(selected) > 0 {
		unitKey := selected[len(selected)-1].unitKey
		start := len(selected) - 1
		for start > 0 && selected[start-1].unitKey == unitKey {
			start--
		}
		dropped := append([]promptOptionalCandidate(nil), selected[start:]...)
		selected = selected[:start]
		for _, candidate := range dropped {
			trace.Dropped = append(trace.Dropped, promptAssemblyDecision(candidate.fragment, "total_cap_final_wire"))
			removePromptAssemblySelected(&trace, candidate.fragment)
		}
		messages = assemblePromptMessages(system, current, selected)
		total = estimatePromptWireInput(messages, input.Tools, input.ResponseFormat)
	}
	if total > input.Policy.MaxInputTokens {
		return PromptAssemblyResult{}, ErrPromptRequiredBudgetExceeded
	}
	trace.EstimatedInputTokens = total
	trace.SectionTokens = promptAssemblySectionTokens(system, current, selected, input.Tools, input.ResponseFormat)
	return PromptAssemblyResult{Messages: messages, Tools: cloneMapSlice(input.Tools), ResponseFormat: cloneMap(input.ResponseFormat), Trace: trace}, nil
}

func promptOptionalCandidates(memory WorkingMemory, currentInput string) ([]promptOptionalCandidate, error) {
	groups := []struct {
		order int
		items []PromptFragment
	}{
		{0, memory.Active}, {1, memory.RuntimeFacts}, {2, memory.Recent}, {3, memory.Retrieved}, {4, memory.Summaries},
	}
	result := make([]promptOptionalCandidate, 0)
	for _, group := range groups {
		for _, fragment := range group.items {
			normalized, err := normalizePromptFragment(fragment)
			if err != nil {
				return nil, err
			}
			if normalized.Kind == PromptFragmentRecentMessage {
				message := mapValue(normalized.Content)
				if stringValue(message["role"]) == "user" && strings.TrimSpace(stringValue(message["content"])) == currentInput {
					continue
				}
			}
			unitKey := "fragment:" + strconv.Itoa(len(result))
			if normalized.Kind == PromptFragmentRecentMessage {
				unitKey = strings.TrimSpace(normalized.GroupKey)
				if unitKey == "" {
					unitKey = "recent:" + jsonString(normalized.SourceRefs)
				}
			}
			result = append(result, promptOptionalCandidate{fragment: normalized, order: group.order, unitKey: unitKey})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].order != result[j].order {
			return result[i].order < result[j].order
		}
		if result[i].fragment.Kind == PromptFragmentRecentMessage {
			return false
		}
		return result[i].fragment.Priority > result[j].fragment.Priority
	})
	return result, nil
}

func assemblePromptMessages(system, current map[string]any, selected []promptOptionalCandidate) []map[string]any {
	runtimeContext := map[string]any{}
	recent := make([]map[string]any, 0)
	for _, candidate := range selected {
		fragment := candidate.fragment
		switch fragment.Kind {
		case PromptFragmentRecentMessage:
			recent = append(recent, cloneMap(mapValue(fragment.Content)))
		case PromptFragmentRuntimeFact:
			runtimeContext["facts"] = append(arrayValue(runtimeContext["facts"]), boundedSnapshotValue(fragment.Content))
		case PromptFragmentActiveMemory:
			runtimeContext["active_memory"] = append(arrayValue(runtimeContext["active_memory"]), boundedSnapshotValue(fragment.Content))
		case PromptFragmentRetrievedMemory:
			runtimeContext["retrieved_memory"] = append(arrayValue(runtimeContext["retrieved_memory"]), boundedSnapshotValue(fragment.Content))
		case PromptFragmentSummary:
			runtimeContext["conversation_summaries"] = append(arrayValue(runtimeContext["conversation_summaries"]), boundedSnapshotValue(fragment.Content))
		}
	}
	messages := []map[string]any{cloneMap(system)}
	if len(runtimeContext) > 0 {
		messages = append(messages, map[string]any{"role": "user", "content": "[RUNTIME CONTEXT]\n" + jsonString(runtimeContext) + "\n[/RUNTIME CONTEXT]"})
	}
	messages = append(messages, recent...)
	messages = append(messages, cloneMap(current))
	return messages
}

func estimatePromptWireInput(messages, tools []map[string]any, responseFormat map[string]any) int {
	return EstimatePromptTokens(messages) + EstimatePromptTokens(tools) + EstimatePromptTokens(responseFormat) + 16
}

func promptAssemblySectionTokens(system, current map[string]any, selected []promptOptionalCandidate, tools []map[string]any, responseFormat map[string]any) map[string]int {
	result := map[string]int{
		"system": estimateProviderMessageTokens(system), "current_input": estimateProviderMessageTokens(current),
		"tools": EstimatePromptTokens(tools), "response_schema": EstimatePromptTokens(responseFormat),
		"runtime_facts": 0, "active_memory": 0, "recent": 0, "retrieved_memory": 0, "conversation_summary": 0,
	}
	for _, candidate := range selected {
		section := string(candidate.fragment.Kind)
		if candidate.fragment.Kind == PromptFragmentRuntimeFact {
			section = "runtime_facts"
		} else if candidate.fragment.Kind == PromptFragmentRecentMessage {
			section = "recent"
		}
		result[section] += candidate.fragment.EstimatedTokens
	}
	return result
}

func promptAssemblyDecision(fragment PromptFragment, reason string) PromptAssemblyDecision {
	return PromptAssemblyDecision{Kind: fragment.Kind, SourceRefs: append([]string(nil), fragment.SourceRefs...), Tokens: fragment.EstimatedTokens, Reason: reason}
}

func removePromptAssemblySelected(trace *PromptAssemblyTrace, fragment PromptFragment) {
	for index := len(trace.Selected) - 1; index >= 0; index-- {
		if trace.Selected[index].Kind == fragment.Kind && jsonString(trace.Selected[index].SourceRefs) == jsonString(fragment.SourceRefs) {
			trace.Selected = append(trace.Selected[:index], trace.Selected[index+1:]...)
			return
		}
	}
}

func cloneMapSlice(values []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		result = append(result, cloneMap(value))
	}
	return result
}
