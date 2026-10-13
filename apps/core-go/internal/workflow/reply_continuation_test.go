package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestReplyContinuationWorkflowRetriesTransientFailureWithinBound(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(ProcessReplyContinuationActivity)
	attempts := 0
	env.OnActivity(ProcessReplyContinuationActivity, mock.Anything, "draft").Return(func(context.Context, string) (map[string]any, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("transient")
		}
		return map[string]any{"status": "completed"}, nil
	})
	env.ExecuteWorkflow(ReplyContinuationWorkflow, "draft")
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d", attempts)
	}
}
