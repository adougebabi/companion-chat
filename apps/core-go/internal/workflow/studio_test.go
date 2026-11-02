package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestStudioWorkflowLeavesFailedAttemptRecoveryToDomainReconciliation(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.OnActivity(ProcessStudioActivity, mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded).Once()
	env.ExecuteWorkflow(StudioWorkflow, Input{IntentID: "studio_intent:job_attempt_1"})
	if env.GetWorkflowError() == nil {
		t.Fatal("failed attempt must remain terminal for domain successor reconciliation")
	}
	env.AssertExpectations(t)
}
