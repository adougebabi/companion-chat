package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestADKFinalContractTreatsOmittedAppraisalEvidenceAsEmpty(t *testing.T) {
	appraisal := map[string]any{"event_kind": "ordinary", "direction": "neutral"}
	for _, key := range []string{"relevance", "goal_congruence", "reward", "loss", "social_threat", "controllability", "responsibility", "relationship_significance", "expected_effect"} {
		appraisal[key] = 0.0
	}
	message := schema.AssistantMessage(jsonString(map[string]any{"appraisal": appraisal}), nil)
	responseSchema := objectSchema(map[string]any{"appraisal": appraisalResponseSchema()}, []string{"appraisal"}, false)
	if err := validateADKFinalContract(message, "cognitive_assessment", responseSchema, nil); err != nil {
		t.Fatalf("missing evidence should mean an explicit empty list, not a fabricated ref: %v", err)
	}
	if _, exists := appraisal["evidence_refs"]; exists {
		t.Fatal("validation mutated the caller's appraisal")
	}
}

func TestADKFinalContractCatchesInfluenceRefShapeBeforeSettlement(t *testing.T) {
	responseSchema := objectSchema(map[string]any{"influences": decisionInfluencesSchema()}, []string{"influences"}, false)
	message := schema.AssistantMessage(jsonString(map[string]any{"influences": []any{map[string]any{"ref": "a natural-language label", "role": "grounds", "confidence": 0.7, "note": "context"}}}), nil)
	err := validateADKFinalContract(message, "cognitive_assessment", responseSchema, nil)
	if err == nil || !strings.Contains(err.Error(), "decision_influence_0_ref_invalid") {
		t.Fatalf("invalid influence must enter bounded final repair: %v", err)
	}
}

func TestAgentRunFailureClassifiesFinalOutputBoundary(t *testing.T) {
	for _, code := range []string{"adk_structured_response_invalid", "adk_final_output_invalid", "provider_context_ref_alias_unknown"} {
		stage, got := classifyAgentRunFailure(errors.New("adk: " + code))
		if stage != "model_output" || got != code {
			t.Fatalf("%s classified as %s/%s", code, stage, got)
		}
	}
}
