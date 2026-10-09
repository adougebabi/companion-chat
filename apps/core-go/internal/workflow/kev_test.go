package workflow

import (
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func kevTestWaitWorkflow(ctx workflow.Context) (bool, error) { return kevWakeUpWait(ctx, time.Hour) }
func TestKevManualWakeUpInterruptsDurableDeferral(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	start := env.Now()
	env.RegisterDelayedCallback(func() { env.SignalWorkflow("kev.force_original", nil) }, time.Second)
	env.ExecuteWorkflow(kevTestWaitWorkflow)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var manual bool
	if err := env.GetWorkflowResult(&manual); err != nil {
		t.Fatal(err)
	}
	if !manual || env.Now().Sub(start) >= time.Hour {
		t.Fatal("manual execution waited for Kev deferral expiry")
	}
}
func TestKevAutomaticDeferralUsesDurableTimer(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	start := env.Now()
	env.ExecuteWorkflow(kevTestWaitWorkflow)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var manual bool
	if err := env.GetWorkflowResult(&manual); err != nil {
		t.Fatal(err)
	}
	if manual || env.Now().Sub(start) != time.Hour {
		t.Fatal("automatic deferral did not preserve its timer")
	}
}
