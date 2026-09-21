package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
)

func TestPhase8ProductionCapabilityMatrixIsExplicitAndStable(t *testing.T) {
	registry, err := NewCapabilityRegistry(builtinCapabilities(&App{})...)
	if err != nil {
		t.Fatal(err)
	}
	expectedAll := []string{
		"active_memory_event", "affect_event", "capability.request", "conversation.reply",
		"media.image.generate", "memory.recall", "memory_event", "moment.publish",
		"persona.switch", "persona.takeover", "presence_event", "relationship.lookup",
		"scene_event", "schedule.replan", "visual_identity.initialize",
	}
	if got := capabilityDefinitionNames(registry.Definitions()); !phase8EqualStrings(got, expectedAll) {
		t.Fatalf("registered capabilities drifted: got=%v want=%v", got, expectedAll)
	}

	expectedBySurface := map[CapabilitySurface][]string{
		CapabilitySurfaceConversation:    {"active_memory_event", "affect_event", "capability.request", "conversation.reply", "media.image.generate", "memory.recall", "memory_event", "presence_event", "relationship.lookup", "scene_event", "schedule.replan"},
		CapabilitySurfaceWakeUp:          {"active_memory_event", "affect_event", "capability.request", "conversation.reply", "media.image.generate", "memory_event", "moment.publish", "presence_event", "relationship.lookup", "scene_event", "schedule.replan", "visual_identity.initialize"},
		CapabilitySurfaceAutonomy:        {"active_memory_event", "affect_event", "capability.request", "conversation.reply", "media.image.generate", "memory_event", "moment.publish", "presence_event", "relationship.lookup", "scene_event", "schedule.replan"},
		CapabilitySurfaceNativeCognition: {"active_memory_event", "capability.request", "media.image.generate", "memory_event", "presence_event", "relationship.lookup", "scene_event", "schedule.replan", "visual_identity.initialize"},
		CapabilitySurfaceReflection:      {},
	}
	for surface, expected := range expectedBySurface {
		if got := capabilityDefinitionNames(registry.Catalog(surface)); !phase8EqualStrings(got, expected) {
			t.Errorf("surface %s drifted: got=%v want=%v", surface, got, expected)
		}
	}

	for _, definition := range registry.Definitions() {
		if definition.InternalOnly {
			for _, surface := range []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition, CapabilitySurfaceReflection} {
				for _, visible := range registry.Catalog(surface) {
					if visible.Name == definition.Name {
						t.Fatalf("internal-only capability %q leaked into %s catalog", definition.Name, surface)
					}
				}
			}
		}
	}
}

func TestPhase8ModelVisibleCapabilitiesHaveRealEinoAdapters(t *testing.T) {
	registry, err := NewCapabilityRegistry(builtinCapabilities(&App{})...)
	if err != nil {
		t.Fatal(err)
	}
	for _, surface := range []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition} {
		definitions := registry.Catalog(surface)
		tools, err := aiagent.NewADKCapabilityTools(definitions, adkFailingInvoker{})
		if err != nil {
			t.Fatalf("surface %s adapter construction: %v", surface, err)
		}
		if len(tools) != len(definitions) {
			t.Fatalf("surface %s adapter count = %d, definitions = %d", surface, len(tools), len(definitions))
		}
		seen := make(map[string]struct{}, len(tools))
		for _, candidate := range tools {
			info, infoErr := candidate.Info(nil)
			if infoErr != nil {
				t.Fatalf("surface %s tool info: %v", surface, infoErr)
			}
			if info == nil || info.Name == "" {
				t.Fatalf("surface %s has empty Eino ToolInfo", surface)
			}
			if _, exists := seen[info.Name]; exists {
				t.Fatalf("surface %s duplicates Eino ToolInfo %q", surface, info.Name)
			}
			seen[info.Name] = struct{}{}
		}
	}
}

func TestPhase8ADKAllowlistDoesNotExpandFixedTasks(t *testing.T) {
	wantADK := []string{"conversation_turn_response", takeoverReplySchemaName, "wake_up_response"}
	for _, schemaName := range wantADK {
		if !isADKLoopSchema(schemaName) {
			t.Fatalf("expected ADK schema %q is not allowlisted", schemaName)
		}
	}
	fixedTasks := []string{
		"initialization", "media_prompt", "media_quality_acceptance_response", "visual_identity_vision_response",
		"visual_identity_patch_response", "conversation_summary_v1", "schedule_response", "native_cognition_response",
		"daily_review_response", "persistent_switch_assessment", "reflection_proposal_v2", "schedule_replan_plan",
		"query_continuation_response", "takeover_judgement_response",
	}
	for _, schemaName := range fixedTasks {
		if isADKLoopSchema(schemaName) {
			t.Fatalf("fixed task %q unexpectedly entered the ADK loop", schemaName)
		}
	}
}

func TestPhase8CandidateGateRejectsPolicyOnlyCapabilities(t *testing.T) {
	registry, err := NewCapabilityRegistry(builtinCapabilities(&App{})...)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{Capabilities: registry}
	err = app.validateCandidateCapabilityInvocations([]CapabilityInvocation{{
		CallID: "policy-forged", CapabilityName: personaSwitchCapabilityName,
		Arguments: json.RawMessage(`{"decision":"switch"}`), SourceFactID: "fact-1",
		ProviderRequestID: "provider-1", SchemaVersion: CapabilityInvocationSchemaVersion,
		Metadata: InvocationMetadata{FluctlightID: "fl-1", Surface: CapabilitySurfaceAutonomy},
	}}, candidateValidationContext{FluctlightID: "fl-1", SourceFactID: "fact-1", Surface: CapabilitySurfaceAutonomy})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("policy-only capability validation error = %v", err)
	}
}

func TestPhase8CapabilityExecutionHasOneProductionRegistryAuthority(t *testing.T) {
	root := filepath.Join("..", "..", "internal")
	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, string(filepath.Separator)+"capability"+string(filepath.Separator)) {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(content)
		if strings.Contains(text, "capability.NewCapabilityRegistry(") || strings.Contains(text, "capability.NewCapabilityRuntime(") {
			offenders = append(offenders, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("non-core production registry/runtime callers found: %v", offenders)
	}
	appSource, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(appSource), "NewCapabilityRegistry(builtinCapabilities(app)...)") || !strings.Contains(string(appSource), "NewCapabilityRuntime(app.Capabilities") {
		t.Fatal("App composition root is not the single production capability registry/runtime authority")
	}
}

func TestPhase8DiagnosticsExportSupportsRunIDCorrelation(t *testing.T) {
	operations, err := os.ReadFile("operations.go")
	if err != nil {
		t.Fatal(err)
	}
	modelRuns, err := os.ReadFile("diagnostic_model_runs_filter.go")
	if err != nil {
		t.Fatal(err)
	}
	operationsText := string(operations)
	modelRunsText := string(modelRuns)
	for _, required := range []string{"normalized.RunID", "diagnosticsFiltered", "modelRunsFiltered", "DiagnosticsFiltered", "ModelRunsFiltered"} {
		if !strings.Contains(operationsText, required) {
			t.Fatalf("diagnostic export missing run-id path %q", required)
		}
	}
	if !strings.Contains(modelRunsText, "metrics->>'run_id'") {
		t.Fatal("model-run export does not filter the bounded run_id metric")
	}
}

func TestPhase8FixedTaskInventoryMatchesProductionFunctions(t *testing.T) {
	modelTasks, err := os.ReadFile("model_tasks.go")
	if err != nil {
		t.Fatal(err)
	}
	persistentSwitch, err := os.ReadFile("persistent_switch_assessment.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(modelTasks) + string(persistentSwitch)
	expected := []string{
		"RunInitializationTask", "RunMediaPromptTask", "RunMediaQualityTask",
		"RunVisualIdentityVisionTask", "RunVisualIdentityPatchTask", "RunConversationSummaryTask",
		"RunScheduleGenerationTask", "RunNativeCognitionTask", "RunDailyReviewTask",
		"RunPersistentSwitchTask", "RunReflectionProposalTask", "RunScheduleReplanTask",
		"RunEmbeddingTask", "RunFrozenEmbeddingTask",
		"initialization", "media_prompt", "media_quality_acceptance_response",
		"visual_identity_vision_response", "visual_identity_patch_response", "conversation_summary_v1",
		"schedule_response", "native_cognition_response", "daily_review_response",
		"persistent_switch_assessment", "reflection_proposal_v2", "schedule_replan_plan",
	}
	for _, marker := range expected {
		if !strings.Contains(text, marker) {
			t.Errorf("fixed task inventory marker %q is missing from model_tasks.go", marker)
		}
	}
}

func capabilityDefinitionNames(definitions []CapabilityDefinition) []string {
	result := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, definition.Name)
	}
	sort.Strings(result)
	return result
}

func phase8EqualStrings(left, right []string) bool {
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
