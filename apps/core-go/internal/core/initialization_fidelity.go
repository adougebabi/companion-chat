package core

import (
	"fmt"
	"strconv"
	"strings"
)

type InitializationSemanticAssertion struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Contains    string   `json:"contains,omitempty"`
	ContainsAny []string `json:"contains_any,omitempty"`
	Basis       string   `json:"basis"`
}

type InitializationInventionGuard struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type InitializationExpectationManifest struct {
	Classification string                            `json:"classification"`
	Assertions     []InitializationSemanticAssertion `json:"assertions"`
	Forbidden      []InitializationInventionGuard    `json:"forbidden"`
}

type InitializationCoverageItem struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Category string `json:"category"`
	Basis    string `json:"basis,omitempty"`
}

type InitializationCoverageReport struct {
	Classification string                       `json:"classification"`
	Counts         map[string]int               `json:"counts"`
	Items          []InitializationCoverageItem `json:"items"`
}

func SafeInitializationCoverageSummary(report InitializationCoverageReport, providerModel string, latencyMS int64) map[string]any {
	failingCategories := make([]any, 0, 4)
	for _, category := range []string{"missing", "default_only", "contradicted", "invented"} {
		if report.Counts[category] > 0 {
			failingCategories = append(failingCategories, category)
		}
	}
	failedAssertions := make([]any, 0, min(len(report.Items), 128))
	for index, item := range report.Items {
		if item.Category != "missing" && item.Category != "default_only" && item.Category != "contradicted" && item.Category != "invented" {
			continue
		}
		if len(failedAssertions) >= 128 {
			break
		}
		failedAssertions = append(failedAssertions, map[string]any{
			"index":    index,
			"category": boundedLifecycleString(item.Category),
		})
	}
	return map[string]any{"classification": report.Classification, "counts": report.Counts, "failing_categories": failingCategories, "failed_assertions": failedAssertions, "provider_model": boundedLifecycleString(providerModel), "latency_ms": max(latencyMS, int64(0))}
}

func EvaluateInitializationCoverage(value map[string]any, manifest InitializationExpectationManifest) InitializationCoverageReport {
	report := InitializationCoverageReport{Classification: stringValue(mapValue(mapValue(value["core_persona"])["personality_system"])["mode"]), Counts: map[string]int{}, Items: []InitializationCoverageItem{}}
	for _, assertion := range manifest.Assertions {
		actual, exists := initializationValueAtPath(value, assertion.Path)
		category := "missing"
		if exists && !initializationSemanticEmpty(actual) {
			if initializationAssertionMatches(actual, assertion) {
				category = "preserved"
			} else {
				category = "contradicted"
			}
		} else if initializationExtensionMatches(value, assertion) {
			category = "moved_to_extension"
		} else if exists {
			category = "default_only"
		}
		report.Counts[category]++
		report.Items = append(report.Items, InitializationCoverageItem{ID: assertion.ID, Path: assertion.Path, Category: category, Basis: assertion.Basis})
	}
	if manifest.Classification != "" && report.Classification != manifest.Classification {
		report.Counts["contradicted"]++
		report.Items = append(report.Items, InitializationCoverageItem{ID: "classification", Path: "core_persona.personality_system.mode", Category: "contradicted", Basis: "explicit"})
	}
	for _, guard := range manifest.Forbidden {
		if actual, exists := initializationValueAtPath(value, guard.Path); exists && !initializationSemanticEmpty(actual) {
			report.Counts["invented"]++
			report.Items = append(report.Items, InitializationCoverageItem{ID: guard.ID, Path: guard.Path, Category: "invented"})
		}
	}
	return report
}

func initializationAssertionMatches(actual any, assertion InitializationSemanticAssertion) bool {
	anchors := append([]string(nil), assertion.ContainsAny...)
	if strings.TrimSpace(assertion.Contains) != "" {
		anchors = append(anchors, assertion.Contains)
	}
	if len(anchors) == 0 {
		return true
	}
	actualText := strings.ToLower(jsonString(actual))
	for _, anchor := range anchors {
		anchor = strings.ToLower(strings.TrimSpace(anchor))
		if anchor != "" && strings.Contains(actualText, anchor) {
			return true
		}
	}
	return false
}

func initializationExtensionMatches(value map[string]any, assertion InitializationSemanticAssertion) bool {
	if initializationExtensionContains(value, assertion.Contains) {
		return true
	}
	for _, anchor := range assertion.ContainsAny {
		if initializationExtensionContains(value, anchor) {
			return true
		}
	}
	return false
}

func initializationValueAtPath(value any, path string) (any, bool) {
	current := value
	for _, segment := range strings.Split(path, ".") {
		name, index, indexed, err := initializationPathSegment(segment)
		if err != nil {
			return nil, false
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[name]
		if !ok {
			return nil, false
		}
		if indexed {
			items := arrayValue(current)
			if index < 0 || index >= len(items) {
				return nil, false
			}
			current = items[index]
		}
	}
	return current, true
}

func initializationPathSegment(segment string) (string, int, bool, error) {
	open := strings.Index(segment, "[")
	if open < 0 {
		return segment, 0, false, nil
	}
	if !strings.HasSuffix(segment, "]") {
		return "", 0, false, fmt.Errorf("initialization expectation path invalid")
	}
	index, err := strconv.Atoi(segment[open+1 : len(segment)-1])
	return segment[:open], index, true, err
}

func initializationSemanticEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func initializationExtensionContains(value map[string]any, expected string) bool {
	if strings.TrimSpace(expected) == "" {
		return false
	}
	extensions := mapValue(value["extensions"])
	return strings.Contains(strings.ToLower(jsonString(extensions)), strings.ToLower(expected))
}
