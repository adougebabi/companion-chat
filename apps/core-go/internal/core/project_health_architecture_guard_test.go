package core

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestProjectHealthArchitectureGuardHasNoReflectionV1ProductionSurface(t *testing.T) {
	forbidden := regexp.MustCompile(`\b(?:reflectionResponseSchema|normalizeReflectionProposal|filterReflectionEvidence|validateReflectionProposal|validateReflectionRelationshipTargets|resolveReflectionActorAliases|applyReflectionCandidates|reflectionEvidenceRefs|compactReflectionEvidence)\b|ReflectionProposalV1|fluctlight\.reflection\.v1|"relationship_candidates"`)
	walkProductionGo(t, []string{".", "../workflow", "../httpapi/browser"}, func(path string, source []byte) {
		if match := forbidden.Find(source); match != nil {
			t.Fatalf("legacy Reflection production surface %q in %s", match, path)
		}
	})
}

func TestProjectHealthArchitectureGuardRejectsCallerOwnedToolResultMessages(t *testing.T) {
	toolRole := regexp.MustCompile(`(?i)(?:\\?"role\\?"\s*:|\bRole\s*:|\brole\s*(?::=|=))\s*\\?"tool\\?"`)
	walkProductionGo(t, []string{".", "../workflow", "../httpapi/browser"}, func(path string, source []byte) {
		if match := toolRole.Find(source); match != nil {
			t.Fatalf("caller-owned role=tool message %q in %s", match, path)
		}
	})
	for _, path := range []string{"mutations.go", "composite_actions.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{`decision["effects"]`, `decision["decision"]`, `mapValue(decision["action"])`, "resolveDecisionAction"} {
			if strings.Contains(string(source), forbidden) {
				t.Fatalf("legacy decision fallback %q remains in %s", forbidden, path)
			}
		}
	}
}

func TestRetiredCallerOwnedToolDispatchIsPhysicallyAbsent(t *testing.T) {
	if _, err := os.Stat("query_continuation.go"); !os.IsNotExist(err) {
		t.Fatalf("retired query continuation file still exists: %v", err)
	}
	for _, path := range []string{
		"mutations.go", "wakeup.go", "cognition_growth.go", "autonomy.go",
		"capability_runtime.go", "workflow_ops.go", "conversation_runtime.go",
		"provider.go", "provider_schemas.go",
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{
			"handleTurnLegacy", "processWakeUpLegacy", "processNativeCognitionFactLegacy",
			"processDailyReviewLegacy", "prepareCapabilityInvocations",
			"planCapabilitiesForTransaction", "settleDeferredCapabilitiesTx",
			"StructuredQueryContinuation", "query_continuation",
		} {
			if strings.Contains(string(source), forbidden) {
				t.Fatalf("retired execution mechanism %q remains in %s", forbidden, path)
			}
		}
	}
	workflowSource, err := os.ReadFile("workflow_ops.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflowSource), "a.ExecuteTool(ctx, ToolExecutionRequest{") {
		t.Fatal("durable Action recovery does not use the formal Tool boundary")
	}
}

func TestProjectHealthArchitectureGuardHasNoScheduleSemanticSplitter(t *testing.T) {
	source := readSourceFile(t, "schedule_generation.go")
	for _, forbidden := range []string{"splitAmbiguousScheduleEntries", "splitScheduleAlternatives", `ReplaceAll(value, "或"`} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("Go schedule semantic heuristic %q remains", forbidden)
		}
	}
}

func TestExecutableReplayRejectsMissingWrongOrDualAuthorityVersion(t *testing.T) {
	canonical := map[string]any{
		"capability_runtime_version": CapabilityRuntimePayloadVersion,
		"capability_invocations":     []any{}, "capability_results": []any{},
		"decision": map[string]any{},
	}
	if err := validateExecutableCapabilityPayload(canonical); err != nil {
		t.Fatalf("canonical payload rejected: %v", err)
	}
	fixtures := []map[string]any{
		{"capability_invocations": []any{}, "capability_results": []any{}},
		{"capability_runtime_version": "v1", "capability_invocations": []any{}, "capability_results": []any{}},
		{"capability_runtime_version": CapabilityRuntimePayloadVersion, "capability_invocations": []any{}, "capability_results": []any{}, "tool_calls": []any{}},
		{"capability_runtime_version": CapabilityRuntimePayloadVersion, "capability_invocations": []any{}, "capability_results": []any{}, "decision": map[string]any{"tool_calls": []any{}}},
		{"capability_runtime_version": CapabilityRuntimePayloadVersion, "capability_invocations": map[string]any{}, "capability_results": []any{}},
	}
	for index, fixture := range fixtures {
		if err := validateExecutableCapabilityPayload(fixture); err == nil {
			t.Fatalf("invalid executable fixture %d accepted: %#v", index, fixture)
		}
	}
}

func TestActiveFrozenReplayRequiresCurrentContextReferenceAuthority(t *testing.T) {
	if err := validateFrozenDecisionInfluences(map[string]any{}); err == nil || err.Error() != "frozen_context_reference_version_invalid" {
		t.Fatalf("missing context authority error=%v", err)
	}
	if _, err := frozenDecisionCausality(map[string]any{}); err == nil {
		t.Fatal("missing context authority produced empty causality")
	}
	if _, err := frozenDecisionDriveSignals(map[string]any{}); err == nil {
		t.Fatal("missing context authority produced empty drive signals")
	}
}

func TestAffectStaticGuardPreservesBipolarAndUnitClampOwnership(t *testing.T) {
	if clampBipolar(-0.5) != -0.5 || clampUnit(-0.5) != 0 {
		t.Fatal("bipolar and unit ranges collapsed")
	}
	source := readSourceFile(t, "affect_reducer.go")
	text := string(source)
	for _, required := range []string{`case "pad":`, `target, clamp = pad, clampBipolar`, `case "momentum":`, `target, clamp = momentum, clampBipolar`} {
		if !strings.Contains(text, required) {
			t.Fatalf("Affect reducer lost bipolar ownership %q", required)
		}
	}
}

func walkProductionGo(t *testing.T, roots []string, inspect func(string, []byte)) {
	t.Helper()
	for _, root := range roots {
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, string(filepath.Separator)+"migrations"+string(filepath.Separator)) {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			inspect(path, source)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}
