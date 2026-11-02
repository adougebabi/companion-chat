package workflow

import (
	"errors"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	temporalworkflow "go.temporal.io/sdk/workflow"
	"testing"
)

func TestWardrobeLifecycleUsesMachineReasonAndPreservesHumanExplanation(t *testing.T) {
	for _, result := range []map[string]any{
		{"status": "awaiting_wear", "reason_code": "wardrobe_inventory_prepared", "reason": "衣柜已准备，接下来结合当前情境换装"},
		{"status": "awaiting_wear", "reason_code": "awaiting_wake", "reason": "方案已准备，等待清醒后结合当前情境换装"},
		// Historical completed/noop results still contain human prose only.
		{"status": "awaiting_wear", "reason": "模型生成的说明不是机器标识"},
		{"status": "completed", "reason": "这套搭配适合今天"},
	} {
		original := result["reason"]
		transition, status, reason := wardrobeLifecycleOutcome(result)
		if status == "awaiting_wear" && transition != core.LifecycleTransitionCompletedNoop {
			t.Fatal(transition)
		}
		if reason == original || result["reason"] != original {
			t.Fatal("human prose was used as a machine reason or overwritten", result)
		}
		diagnostic := activityLifecycleDiagnostic(Input{IntentID: "wardrobe:fixture", FluctlightID: "role"}, "wardrobe", transition, status, reason, nil, activity.Info{})
		if err := diagnostic.Validate(); err != nil {
			t.Fatal("real lifecycle envelope rejected wardrobe outcome", diagnostic, err)
		}
		if code, exists := result["reason_code"]; exists && reason != code {
			t.Fatal("domain reason lost", reason, code)
		}
	}
}

func TestWardrobePreparationWorkflowWaitsDurablyAndDoesNotPollModel(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(ProcessWardrobePreparationActivity)
	env.OnActivity(ProcessWardrobePreparationActivity, mock.Anything, mock.Anything).Return(map[string]any{"status": "awaiting_wear", "reason": "sleeping"}, nil).Once()
	env.ExecuteWorkflow(WardrobePreparationWorkflow, Input{SessionID: "prepared", FluctlightID: "fid"})
	var continued *temporalworkflow.ContinueAsNewError
	if !errors.As(env.GetWorkflowError(), &continued) {
		t.Fatal("waiting must continue the durable workflow", env.GetWorkflowError())
	}
	env.AssertExpectations(t)
}
func TestWardrobePreparationWorkflowReturnsActualCompletedResult(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(ProcessWardrobePreparationActivity)
	env.OnActivity(ProcessWardrobePreparationActivity, mock.Anything, mock.Anything).Return(map[string]any{"status": "completed", "wear_receipt": "real-result-pointer"}, nil).Once()
	env.ExecuteWorkflow(WardrobePreparationWorkflow, Input{SessionID: "prepared", FluctlightID: "fid"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := env.GetWorkflowResult(&result); err != nil || result["status"] != "completed" {
		t.Fatal(result, err)
	}
	env.AssertExpectations(t)
}
