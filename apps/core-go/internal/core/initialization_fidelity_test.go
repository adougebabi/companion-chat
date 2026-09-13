package core

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func loadInitializationManifest(t *testing.T, path string) InitializationExpectationManifest {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest InitializationExpectationManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestInitializationCoverageEvaluatorUsesSemanticCategories(t *testing.T) {
	manifest := loadInitializationManifest(t, "testdata/initialization/dense_single_expectations.json")
	value := map[string]any{
		"core_persona": map[string]any{
			"identity":           map[string]any{"name": "林澜", "nickname": "", "birthplace": "南京", "background_story": "她在古籍书店长大"},
			"life_profile":       map[string]any{"appearance": map[string]any{"description": "乌黑长发", "daily_outfit_preferences": []any{"\u4e9a\u9ebb\u886c\u886b"}}, "media_preferences": map[string]any{}},
			"personality_system": map[string]any{"mode": "single", "profiles": []any{map[string]any{"id": "default"}}},
		},
		"extensions": map[string]any{"core_persona.life_profile.media_preferences": "完成水彩时发画作"},
	}
	report := EvaluateInitializationCoverage(value, manifest)
	for _, category := range []string{"preserved", "default_only", "missing", "moved_to_extension", "contradicted"} {
		if report.Counts[category] == 0 {
			t.Fatalf("coverage report lacks %s: %#v", category, report)
		}
	}
	value["core_persona"].(map[string]any)["identity"].(map[string]any)["trauma"] = "invented"
	report = EvaluateInitializationCoverage(value, manifest)
	if report.Counts["invented"] != 1 {
		t.Fatalf("invention guard failed: %#v", report)
	}
}

func TestInitializationCoverageEvaluatorAcceptsAnyExplicitSemanticAnchor(t *testing.T) {
	manifest := InitializationExpectationManifest{Classification: "multiple", Assertions: []InitializationSemanticAssertion{{
		ID: "relationship", Path: "core_persona.personality_system.core_relationship", ContainsAny: []string{"共同创作搭档", "合作伙伴", "互相信任"}, Basis: "explicit",
	}}}
	value := map[string]any{"core_persona": map[string]any{"personality_system": map[string]any{"mode": "multiple", "core_relationship": "两个 profile 是互相信任的长期合作伙伴"}}}
	report := EvaluateInitializationCoverage(value, manifest)
	if report.Counts["preserved"] != 1 || report.Counts["contradicted"] != 0 {
		t.Fatalf("contains_any semantic anchor was not preserved: %#v", report)
	}
	manifest.Assertions[0].ContainsAny = []string{"无关语义", "不存在的描述"}
	report = EvaluateInitializationCoverage(value, manifest)
	if report.Counts["contradicted"] != 1 {
		t.Fatalf("contains_any accepted an unrelated non-empty value: %#v", report)
	}
}

func TestDenseInitializationFixturesCoverSingleAndMultipleModules(t *testing.T) {
	single := loadInitializationManifest(t, "testdata/initialization/dense_single_expectations.json")
	multi := loadInitializationManifest(t, "testdata/initialization/dense_multi_expectations.json")
	if single.Classification != "single" || multi.Classification != "multiple" || len(single.Assertions) < 8 || len(multi.Assertions) < 8 {
		t.Fatalf("dense fixture manifests are incomplete: single=%#v multi=%#v", single, multi)
	}
	for _, path := range []string{"testdata/initialization/dense_single_card.txt", "testdata/initialization/dense_multi_card.txt"} {
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) < 300 {
			t.Fatalf("dense fixture %s is missing or sparse", path)
		}
	}
}

func TestInitializationCoverageSummaryDoesNotLeakSourceOrExpectations(t *testing.T) {
	const canary = "PRIVATE_EXTERNAL_CARD_CANARY"
	report := InitializationCoverageReport{Classification: "multiple", Counts: map[string]int{"missing": 1}, Items: []InitializationCoverageItem{{ID: canary, Path: canary, Category: "missing", Basis: canary}}}
	encoded := jsonString(SafeInitializationCoverageSummary(report, "local-model", 123))
	if strings.Contains(encoded, canary) || strings.Contains(encoded, "source") || !strings.Contains(encoded, "missing") {
		t.Fatalf("coverage summary leaked private fixture data: %s", encoded)
	}
}

func TestPrivateInitializationLiveHarnessNeverPrintsSourceOrResponse(t *testing.T) {
	source, err := os.ReadFile("provider_live_tool_test.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func liveProviderInitializationCase")
	end := strings.Index(text, "// TestLiveProviderRecognizesImageGenerationIntent")
	if start < 0 || end <= start {
		t.Fatal("private initialization Live harness boundary missing")
	}
	body := text[start:end]
	for _, forbidden := range []string{"boundedLiveProviderBody", "jsonString(raw)", "response=%", "body=%s", "message=%"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("private initialization Live harness can leak %q", forbidden)
		}
	}
}
