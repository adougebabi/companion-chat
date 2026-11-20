package workflow

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/stretchr/testify/mock"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	historypb "go.temporal.io/api/history/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/mocks"
	"go.temporal.io/sdk/temporal"
)

const capacityFailureFixture = `provider HTTP 400: {"error":{"message":"Prompt (2340 tokens) requires ~1687MB GPU memory but only ~0MB is available"}}`

func capacityFailureClient(bodyReadFailure ...bool) *mocks.Client {
	failure := &failurepb.Failure{Message: capacityFailureFixture}
	if len(bodyReadFailure) > 0 && bodyReadFailure[0] {
		failure.Cause = &failurepb.Failure{Message: "response body interrupted"}
	}
	iterator := &mocks.HistoryEventIterator{}
	iterator.On("HasNext").Return(true).Once()
	iterator.On("Next").Return(&historypb.HistoryEvent{EventType: enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_FAILED, Attributes: &historypb.HistoryEvent_WorkflowExecutionFailedEventAttributes{WorkflowExecutionFailedEventAttributes: &historypb.WorkflowExecutionFailedEventAttributes{Failure: &failurepb.Failure{Message: "activity task failed", Cause: failure}}}}, nil).Once()
	iterator.On("HasNext").Return(false)
	temporalClient := &mocks.Client{}
	temporalClient.On("DescribeWorkflowExecution", mock.Anything, "go:schedule:retry-role", "").Return(&workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: enumspb.WORKFLOW_EXECUTION_STATUS_FAILED, Execution: &commonpb.WorkflowExecution{RunId: "capacity-run"}}}, nil).Once()
	temporalClient.On("GetWorkflowHistory", mock.Anything, "go:schedule:retry-role", "capacity-run", false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT).Return(iterator).Once()
	return temporalClient
}

// PostgreSQL only; Temporal and the provider are both replaced with local fixtures.
func TestPostgresProviderCapacityRefundsAttemptAndDelaysRetry(t *testing.T) {
	for _, kind := range []string{"schedule.current_day", "wardrobe.initialize", "visual_identity.initialize", "wake_up.current"} {
		for _, bodyReadFailure := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/body_read_error=%t", kind, bodyReadFailure), func(t *testing.T) {
				ctx, app := scheduleRecoveryRepository(t)
				const intent = "schedule_intent:retry-role"
				if kind == "visual_identity.initialize" {
					// Use a real session reference rather than reusing a schedule payload
					// with no session_id (SQL NULL cannot be scanned into a Go string).
					if _, err := app.DB.Pool().Exec(ctx, `DELETE FROM public.fluctlight_visual_identity_sessions WHERE fluctlight_id='retry-role';
INSERT INTO public.fluctlight_visual_identities(id,fluctlight_id) VALUES('capacity-profile','retry-role') ON CONFLICT(fluctlight_id) DO NOTHING;
INSERT INTO public.fluctlight_visual_identity_sessions(id,visual_identity_id,fluctlight_id,trigger_type,workflow_id,status)
SELECT 'capacity-session',id,'retry-role','initialize','capacity-visual-session','running' FROM public.fluctlight_visual_identities WHERE fluctlight_id='retry-role';
UPDATE public.platform_workflow_intents SET payload=payload||'{"session_id":"capacity-session"}'::jsonb WHERE intent_id='schedule_intent:retry-role'`); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := app.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET intent_type=$2,status='started',attempt_count=3,completed_at=NULL WHERE intent_id=$1`, intent, kind); err != nil {
					t.Fatal(err)
				}
				temporalClient := capacityFailureClient(bodyReadFailure)
				dispatcher := &Dispatcher{App: app, Client: temporalClient}
				if _, err := dispatcher.ReconcileOnce(ctx, 100); err != nil {
					t.Fatal(err)
				}
				var status string
				var attempts, capacityRetries int
				var next *time.Time
				if err := app.DB.Pool().QueryRow(ctx, `SELECT status,attempt_count,next_attempt_at,COALESCE((payload->>'provider_capacity_retries')::int,0) FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status, &attempts, &next, &capacityRetries); err != nil {
					t.Fatal(err)
				}
				if status != "retry" || attempts != 2 || capacityRetries != 1 || next == nil || time.Until(*next) < 25*time.Second {
					t.Fatalf("capacity consumed business attempt or retried immediately: status=%s attempts=%d capacity=%d next=%v", status, attempts, capacityRetries, next)
				}
				if n, err := dispatcher.DispatchOnce(ctx, 100); err != nil || n != 0 {
					t.Fatalf("capacity delay bypassed: starts=%d err=%v", n, err)
				}
				// A second reconciliation, including once due, must not refund the same Run twice.
				if _, err := app.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET next_attempt_at=now() WHERE intent_id=$1`, intent); err != nil {
					t.Fatal(err)
				}
				if _, err := dispatcher.ReconcileOnce(ctx, 100); err != nil {
					t.Fatal(err)
				}
				if err := app.DB.Pool().QueryRow(ctx, `SELECT attempt_count,(payload->>'provider_capacity_retries')::int FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&attempts, &capacityRetries); err != nil || attempts != 2 || capacityRetries != 1 {
					t.Fatalf("same Run refunded twice: attempts=%d capacity=%d err=%v", attempts, capacityRetries, err)
				}
				temporalClient.AssertExpectations(t)
				// Simulate restart after the last allowed capacity wait.
				if _, err := app.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='started',attempt_count=3,payload=payload||'{"provider_capacity_retries":8}'::jsonb WHERE intent_id=$1`, intent); err != nil {
					t.Fatal(err)
				}
				temporalClient = capacityFailureClient(bodyReadFailure)
				dispatcher.Client = temporalClient
				if _, err := dispatcher.ReconcileOnce(ctx, 100); err != nil {
					t.Fatal(err)
				}
				if err := app.DB.Pool().QueryRow(ctx, `SELECT status,attempt_count,next_attempt_at,(payload->>'provider_capacity_retries')::int FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status, &attempts, &next, &capacityRetries); err != nil || status != "dead_letter" || attempts != 2 || next != nil || capacityRetries != 9 {
					t.Fatalf("capacity wait not bounded: status=%s attempts=%d capacity=%d next=%v err=%v", status, attempts, capacityRetries, next, err)
				}
				// Automatic ledger repair may reopen a dead letter, but cannot get
				// another model execution once the capacity allowance is exhausted.
				if _, err := app.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',attempt_count=0,next_attempt_at=now() WHERE intent_id=$1`, intent); err != nil {
					t.Fatal(err)
				}
				if _, err := dispatcher.DispatchOnce(ctx, 100); err != nil {
					t.Fatal(err)
				}
				if err := app.DB.Pool().QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status); err != nil || status != "dead_letter" {
					t.Fatalf("automatic repair bypassed capacity cap: status=%s err=%v", status, err)
				}
				temporalClient.AssertExpectations(t)
				if kind == "schedule.current_day" {
					recovered, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/retry-role/schedules/recover", nil, "retry-owner", map[string]any{"reason": "容量已恢复", "idempotency_key": "capacity-recover"})
					if err != nil || recovered["restarted"] != true {
						t.Fatal("explicit recovery failed", recovered, err)
					}
					if err := app.DB.Pool().QueryRow(ctx, `SELECT COALESCE((payload->>'provider_capacity_retries')::int,0) FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&capacityRetries); err != nil || capacityRetries != 0 {
						t.Fatalf("explicit recovery did not reset capacity allowance: retries=%d err=%v", capacityRetries, err)
					}
				}
			})
		}
	}
}

// Use the SDK's real failure conversion before reading the history event.
func TestProviderCapacitySurvivesActualFailureWrapping(t *testing.T) {
	for _, readErr := range []error{nil, errors.New("response body interrupted")} {
		t.Run(fmt.Sprint(readErr), func(t *testing.T) {
			providerErr := temporal.NewApplicationErrorWithCause(capacityFailureFixture, "providerHTTPResponseError", readErr)
			wrapped := temporal.NewApplicationErrorWithCause("activity task failed", "activity", fmt.Errorf("initialize: %w", providerErr))
			failure := temporal.GetDefaultFailureConverter().ErrorToFailure(wrapped)
			iterator := &mocks.HistoryEventIterator{}
			iterator.On("HasNext").Return(true).Once()
			iterator.On("Next").Return(&historypb.HistoryEvent{EventType: enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_FAILED, Attributes: &historypb.HistoryEvent_WorkflowExecutionFailedEventAttributes{WorkflowExecutionFailedEventAttributes: &historypb.WorkflowExecutionFailedEventAttributes{Failure: failure}}}, nil).Once()
			iterator.On("HasNext").Return(false).Once()
			temporalClient := &mocks.Client{}
			temporalClient.On("GetWorkflowHistory", mock.Anything, "capacity-workflow", "capacity-run", false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT).Return(iterator).Once()
			dispatcher := &Dispatcher{Client: temporalClient}
			reason, err := dispatcher.terminalFailureReason(context.Background(), "capacity-workflow", "capacity-run")
			if err != nil || !core.ProviderCapacityFailure(reason) {
				t.Fatalf("wrapped capacity became business failure: reason=%q err=%v", reason, err)
			}
			temporalClient.AssertExpectations(t)
			iterator.AssertExpectations(t)
		})
	}
}

func TestProviderCapacityHistoryFixtureHasTerminalEventType(t *testing.T) {
	dispatcher := &Dispatcher{Client: capacityFailureClient()}
	reason, err := dispatcher.terminalFailureReason(context.Background(), "go:schedule:retry-role", "capacity-run")
	if err != nil || !core.ProviderCapacityFailure(reason) {
		t.Fatalf("fixture lost terminal capacity failure: reason=%q err=%v", reason, err)
	}
}
