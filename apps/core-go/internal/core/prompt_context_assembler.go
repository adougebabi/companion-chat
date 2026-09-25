package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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
	defaultSystemTokensCap          = 16384
	defaultToolsSchemaTokensCap     = 28672
	defaultCurrentInputTokensCap    = 16384
	defaultPromptImageTokens        = 1536
	defaultPromptLowDetailImage     = 85
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

var dataImageRegex = regexp.MustCompile(`data:image/[a-zA-Z0-9.+_-]+;base64,[A-Za-z0-9+/=\r\n]+`)

func isImageContentPart(m map[string]any) bool {
	if m == nil {
		return false
	}
	if stringValue(m["type"]) == "image_url" {
		return true
	}
	if _, ok := m["image_url"]; ok {
		return true
	}
	return false
}

func estimateImagePartTokens(m map[string]any) int {
	detail := ""
	if imgObj, ok := m["image_url"].(map[string]any); ok {
		detail = strings.ToLower(strings.TrimSpace(stringValue(imgObj["detail"])))
	}
	if detail == "" {
		detail = strings.ToLower(strings.TrimSpace(stringValue(m["detail"])))
	}
	if detail == "low" {
		return defaultPromptLowDetailImage
	}
	return defaultPromptImageTokens
}

func sanitizeStringPromptTokens(s string) (string, int) {
	if !strings.Contains(s, "data:image/") {
		return s, 0
	}
	matches := dataImageRegex.FindAllStringIndex(s, -1)
	if len(matches) == 0 {
		return s, 0
	}
	imageTokens := len(matches) * defaultPromptImageTokens
	cleaned := dataImageRegex.ReplaceAllString(s, "[image]")
	return cleaned, imageTokens
}

func sanitizeForPromptTokenEstimation(value any) (any, int) {
	if value == nil {
		return nil, 0
	}
	switch v := value.(type) {
	case string:
		return sanitizeStringPromptTokens(v)
	case map[string]any:
		if isImageContentPart(v) {
			tokens := estimateImagePartTokens(v)
			return map[string]any{"type": "image_url"}, tokens
		}
		sanitizedMap := make(map[string]any, len(v))
		totalImageTokens := 0
		for k, val := range v {
			sVal, imgTok := sanitizeForPromptTokenEstimation(val)
			sanitizedMap[k] = sVal
			totalImageTokens += imgTok
		}
		return sanitizedMap, totalImageTokens
	case []map[string]any:
		sanitizedSlice := make([]map[string]any, len(v))
		totalImageTokens := 0
		for i, item := range v {
			sItem, imgTok := sanitizeForPromptTokenEstimation(item)
			if m, ok := sItem.(map[string]any); ok {
				sanitizedSlice[i] = m
			} else {
				sanitizedSlice[i] = item
			}
			totalImageTokens += imgTok
		}
		return sanitizedSlice, totalImageTokens
	case []any:
		sanitizedSlice := make([]any, len(v))
		totalImageTokens := 0
		for i, item := range v {
			sItem, imgTok := sanitizeForPromptTokenEstimation(item)
			sanitizedSlice[i] = sItem
			totalImageTokens += imgTok
		}
		return sanitizedSlice, totalImageTokens
	default:
		return value, 0
	}
}

func estimateRawTextTokens(text string) int {
	bytesUnits := (len([]byte(text)) + 2) / 3
	runeUnits := len([]rune(text))
	if runeUnits > bytesUnits {
		bytesUnits = runeUnits
	}
	return (bytesUnits*5 + 3) / 4
}

func EstimatePromptTokens(value any) int {
	sanitized, imageTokens := sanitizeForPromptTokenEstimation(value)
	var text string
	if raw, ok := sanitized.(string); ok {
		text = raw
	} else {
		encoded, err := json.Marshal(sanitized)
		if err != nil {
			return imageTokens
		}
		text = string(encoded)
		if strings.Contains(text, "data:image/") {
			cleaned, extraTokens := sanitizeStringPromptTokens(text)
			text = cleaned
			imageTokens += extraTokens
		}
	}
	return estimateRawTextTokens(text) + imageTokens
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
	WireBytes            int                      `json:"wire_bytes"`
	WireChars            int                      `json:"wire_chars"`
	TokenEstimateMethod  string                   `json:"token_estimate_method"`
	SectionTokens        map[string]int           `json:"section_tokens"`
	SectionBytes         map[string]int           `json:"section_bytes"`
	SectionChars         map[string]int           `json:"section_chars"`
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
	position int
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
	candidates, err := promptOptionalCandidates(input.WorkingMemory, input.CurrentInput)
	if err != nil {
		return PromptAssemblyResult{}, err
	}
	systemTokens := estimateProviderMessageTokens(system)
	currentTokens := estimateProviderMessageTokens(current)
	toolsTokens := EstimatePromptTokens(input.Tools)
	schemaTokens := EstimatePromptTokens(input.ResponseFormat)
	if systemTokens > input.Policy.SystemTokensCap || currentTokens > input.Policy.CurrentInputTokensCap || toolsTokens+schemaTokens > input.Policy.ToolsSchemaTokensCap {
		return PromptAssemblyResult{}, fmt.Errorf("%w: required section cap exceeded system=%d current=%d tools=%d schema=%d", ErrPromptRequiredBudgetExceeded, systemTokens, currentTokens, toolsTokens, schemaTokens)
	}
	selected := make([]promptOptionalCandidate, 0, len(candidates))
	optional := make([]promptOptionalCandidate, 0, len(candidates))
	trace := PromptAssemblyTrace{
		PolicyVersion: input.Policy.Version, ContextWindowTokens: input.Policy.ContextWindowTokens,
		MaxInputTokens: input.Policy.MaxInputTokens, OutputReserveTokens: input.Policy.OutputReserveTokens,
		SafetyMarginTokens: input.Policy.SafetyMarginTokens, SectionTokens: map[string]int{},
		Selected: []PromptAssemblyDecision{}, Dropped: []PromptAssemblyDecision{},
	}
	for index, candidate := range candidates {
		candidate.position = index
		if candidate.fragment.Kind == PromptFragmentRecentMessage {
			candidate.fragment.EstimatedTokens = estimateProviderMessageTokens(mapValue(candidate.fragment.Content))
		}
		if candidate.fragment.Kind == PromptFragmentRuntimeFact {
			selected = append(selected, candidate)
		} else {
			optional = append(optional, candidate)
		}
	}
	orderedSelected := orderedPromptCandidates(selected)
	requiredTotal := estimatePromptWireInput(assemblePromptMessages(system, current, orderedSelected), input.Tools, input.ResponseFormat)
	if requiredTotal > input.Policy.MaxInputTokens {
		return PromptAssemblyResult{}, fmt.Errorf("%w: required wire estimate=%d max=%d", ErrPromptRequiredBudgetExceeded, requiredTotal, input.Policy.MaxInputTokens)
	}
	type optionalUnit struct {
		items []promptOptionalCandidate
		kind  PromptFragmentKind
		last  int
	}
	units := make([]optionalUnit, 0, len(optional))
	for index := 0; index < len(optional); {
		end := index + 1
		for end < len(optional) && optional[end].unitKey == optional[index].unitKey {
			end++
		}
		units = append(units, optionalUnit{items: append([]promptOptionalCandidate(nil), optional[index:end]...), kind: optional[index].fragment.Kind, last: optional[end-1].position})
		index = end
	}
	rank := func(kind PromptFragmentKind) int {
		switch kind {
		case PromptFragmentSummary:
			return 0
		case PromptFragmentRetrievedMemory:
			return 1
		case PromptFragmentResidentMemory:
			return 2
		case PromptFragmentRecentMessage:
			return 3
		case PromptFragmentActiveMemory:
			return 4
		default:
			return 5
		}
	}
	sort.SliceStable(units, func(i, j int) bool {
		if rank(units[i].kind) != rank(units[j].kind) {
			return rank(units[i].kind) < rank(units[j].kind)
		}
		if units[i].kind == PromptFragmentRecentMessage || units[i].kind == PromptFragmentSummary {
			return units[i].last > units[j].last
		}
		return units[i].last < units[j].last
	})
	recentGap := false
	for _, unit := range units {
		reason := "budget_excluded"
		if unit.kind == PromptFragmentRecentMessage && recentGap {
			reason = "recent_contiguity_excluded"
		} else {
			trial := orderedPromptCandidates(append(append([]promptOptionalCandidate(nil), selected...), unit.items...))
			if estimatePromptWireInput(assemblePromptMessages(system, current, trial), input.Tools, input.ResponseFormat) <= input.Policy.MaxInputTokens {
				selected = append(selected, unit.items...)
				continue
			}
			if unit.kind == PromptFragmentRecentMessage {
				recentGap = true
			}
		}
		for _, candidate := range unit.items {
			trace.Dropped = append(trace.Dropped, promptAssemblyDecision(candidate.fragment, reason))
		}
	}
	selected = orderedPromptCandidates(selected)
	for _, candidate := range selected {
		trace.Selected = append(trace.Selected, promptAssemblyDecision(candidate.fragment, "selected"))
	}
	messages := assemblePromptMessages(system, current, selected)
	total := estimatePromptWireInput(messages, input.Tools, input.ResponseFormat)
	trace.EstimatedInputTokens = total
	trace.SectionTokens = promptAssemblySectionTokens(system, current, selected, input.Tools, input.ResponseFormat)
	trace.SectionBytes, trace.SectionChars = promptAssemblySectionSizes(system, current, selected, input.Tools, input.ResponseFormat)
	if wire, err := json.Marshal(map[string]any{"messages": messages, "tools": input.Tools, "response_format": input.ResponseFormat}); err == nil {
		trace.WireBytes, trace.WireChars = len(wire), len([]rune(string(wire)))
	}
	trace.TokenEstimateMethod = "utf8_max_runes_or_bytes_div3_times1.25_plus_image_allowance"
	return PromptAssemblyResult{Messages: messages, Tools: cloneMapSlice(input.Tools), ResponseFormat: cloneMap(input.ResponseFormat), Trace: trace}, nil
}

func orderedPromptCandidates(candidates []promptOptionalCandidate) []promptOptionalCandidate {
	ordered := append([]promptOptionalCandidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].position < ordered[j].position })
	return ordered
}

func promptOptionalCandidates(memory WorkingMemory, currentInput string) ([]promptOptionalCandidate, error) {
	groups := []struct {
		order int
		items []PromptFragment
	}{
		{0, memory.RuntimeFacts}, {1, memory.Active}, {2, memory.Resident}, {3, memory.Recent}, {4, memory.Retrieved}, {5, memory.Summaries},
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
			fact := mapValue(fragment.Content)
			kind := stringValue(fact["kind"])
			if kind != "" {
				runtimeContext[kind] = boundedSnapshotValue(fact["value"])
			} else {
				runtimeContext["facts"] = append(arrayValue(runtimeContext["facts"]), boundedSnapshotValue(fragment.Content))
			}
		case PromptFragmentActiveMemory:
			runtimeContext["active_memory"] = append(arrayValue(runtimeContext["active_memory"]), boundedSnapshotValue(fragment.Content))
		case PromptFragmentResidentMemory:
			runtimeContext["resident_memory"] = append(arrayValue(runtimeContext["resident_memory"]), boundedSnapshotValue(fragment.Content))
		case PromptFragmentRetrievedMemory:
			runtimeContext["retrieved_memory"] = append(arrayValue(runtimeContext["retrieved_memory"]), boundedSnapshotValue(fragment.Content))
		case PromptFragmentSummary:
			runtimeContext["conversation_summaries"] = append(arrayValue(runtimeContext["conversation_summaries"]), boundedSnapshotValue(fragment.Content))
		}
	}
	messages := []map[string]any{cloneMap(system)}
	if len(runtimeContext) > 0 {
		messages = append(messages, map[string]any{"role": "user", "content": "[RUNTIME CONTEXT]\n" + renderProviderYAMLWithMode(runtimeContext, true) + "\n[/RUNTIME CONTEXT]"})
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
	if content := stringValue(system["content"]); content != "" {
		if index := strings.Index(content, "\n# 人格设定\n"); index >= 0 {
			result["protocol"] = EstimatePromptTokens(content[:index])
			result["working_persona"] = EstimatePromptTokens(content[index:])
		}
	}
	for _, candidate := range selected {
		section := promptFragmentSection(candidate.fragment.Kind)
		result[section] += candidate.fragment.EstimatedTokens
	}
	return result
}

func promptFragmentSection(kind PromptFragmentKind) string {
	switch kind {
	case PromptFragmentRuntimeFact:
		return "runtime_facts"
	case PromptFragmentActiveMemory:
		return "active_memory"
	case PromptFragmentResidentMemory:
		return "resident_memory"
	case PromptFragmentRecentMessage:
		return "recent"
	case PromptFragmentRetrievedMemory:
		return "retrieved_memory"
	case PromptFragmentSummary:
		return "conversation_summary"
	default:
		return string(kind)
	}
}

func promptAssemblySectionSizes(system, current map[string]any, selected []promptOptionalCandidate, tools []map[string]any, responseFormat map[string]any) (map[string]int, map[string]int) {
	bytesBySection := map[string]int{}
	charsBySection := map[string]int{}
	add := func(section string, value any) {
		encoded, err := json.Marshal(value)
		if err != nil {
			return
		}
		bytesBySection[section] += len(encoded)
		charsBySection[section] += len([]rune(string(encoded)))
	}
	add("system", system)
	add("current_input", current)
	add("tools", tools)
	add("response_schema", responseFormat)
	if content := stringValue(system["content"]); content != "" {
		if index := strings.Index(content, "\n# 人格设定\n"); index >= 0 {
			add("protocol", content[:index])
			add("working_persona", content[index:])
		}
	}
	for _, candidate := range selected {
		add(promptFragmentSection(candidate.fragment.Kind), candidate.fragment.Content)
	}
	return bytesBySection, charsBySection
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
