package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	workflowapi "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
)

// TemporalRuntime adapts the official Temporal client to the narrow Core
// management seam. It is used by the API process for inspection and commands;
// the Worker process remains the sole task-queue poller.
type TemporalRuntime struct {
	Client    client.Client
	Namespace string
	Identity  string
}

var _ core.WorkflowRuntime = (*TemporalRuntime)(nil)

func NewTemporalRuntime(temporalClient client.Client, namespace, identity string) *TemporalRuntime {
	if namespace == "" {
		namespace = "default"
	}
	if identity == "" {
		identity = "fluctlight-core-api"
	}
	return &TemporalRuntime{Client: temporalClient, Namespace: namespace, Identity: identity}
}

func (r *TemporalRuntime) List(ctx context.Context, query string, limit int) ([]core.WorkflowExecution, error) {
	if r == nil || r.Client == nil {
		return nil, errors.New("workflow runtime is unavailable")
	}
	if limit < 1 || limit > 200 {
		limit = 200
	}
	var token []byte
	result := make([]core.WorkflowExecution, 0, limit)
	for len(result) < limit {
		response, err := r.Client.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
			Namespace:     r.Namespace,
			PageSize:      int32(minInt(limit-len(result), 200)),
			NextPageToken: token,
			Query:         strings.TrimSpace(query),
		})
		if err != nil {
			return nil, err
		}
		for _, item := range response.Executions {
			if item == nil {
				continue
			}
			result = append(result, executionFromInfo(item))
			if len(result) >= limit {
				break
			}
		}
		token = response.NextPageToken
		if len(token) == 0 || len(response.Executions) == 0 {
			break
		}
	}
	return result, nil
}

func (r *TemporalRuntime) Status(ctx context.Context, workflowID, runID string) (core.WorkflowExecution, error) {
	if r == nil || r.Client == nil {
		return core.WorkflowExecution{}, errors.New("workflow runtime is unavailable")
	}
	workflowID = normalizedWorkflowID(workflowID)
	response, err := r.Client.DescribeWorkflowExecution(ctx, workflowID, runID)
	if err != nil {
		return core.WorkflowExecution{}, err
	}
	if response == nil || response.WorkflowExecutionInfo == nil {
		return core.WorkflowExecution{}, errors.New("workflow execution info is unavailable")
	}
	result := executionFromInfo(response.WorkflowExecutionInfo)
	if result.Status == "running" {
		if encoded, err := r.Client.QueryWorkflow(ctx, result.WorkflowID, result.RunID, "control_capabilities"); err == nil && encoded != nil {
			_ = encoded.Get(&result.ControlActions)
		}
		if encoded, queryErr := r.Client.QueryWorkflow(ctx, result.WorkflowID, result.RunID, "status"); queryErr == nil && encoded != nil && encoded.HasValue() {
			var workflowStatus string
			if encoded.Get(&workflowStatus) == nil && workflowStatus != "" {
				result.Status = workflowStatus
				result.ControlState = workflowStatus
			}
		}
	}
	return result, nil
}

func (r *TemporalRuntime) History(ctx context.Context, workflowID, runID string, maxEvents int) (core.WorkflowHistory, error) {
	if r == nil || r.Client == nil {
		return core.WorkflowHistory{}, errors.New("workflow runtime is unavailable")
	}
	workflowID = normalizedWorkflowID(workflowID)
	if runID == "" {
		execution, err := r.Status(ctx, workflowID, "")
		if err != nil {
			return core.WorkflowHistory{}, err
		}
		runID = execution.RunID
	}
	if maxEvents < 1 || maxEvents > 200 {
		maxEvents = 50
	}
	iter := r.Client.GetWorkflowHistory(ctx, workflowID, runID, false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	result := core.WorkflowHistory{WorkflowID: workflowID, RunID: runID, EventTypes: make([]string, 0, maxEvents)}
	for iter.HasNext() && len(result.EventTypes) < maxEvents {
		event, err := iter.Next()
		if err != nil {
			return core.WorkflowHistory{}, err
		}
		if event == nil {
			continue
		}
		result.EventTypes = append(result.EventTypes, event.GetEventType().String())
	}
	result.EventCount = len(result.EventTypes)
	return result, nil
}

func (r *TemporalRuntime) Signal(ctx context.Context, workflowID, runID, signalName, requestID string) error {
	if strings.TrimSpace(signalName) == "" {
		return errors.New("workflow signal is required")
	}
	workflowID = normalizedWorkflowID(workflowID)
	input, err := converter.GetDefaultDataConverter().ToPayloads(nil)
	if err != nil {
		return err
	}
	_, err = r.service().SignalWorkflowExecution(ctx, &workflowservice.SignalWorkflowExecutionRequest{
		Namespace:         r.Namespace,
		WorkflowExecution: executionRef(workflowID, runID),
		SignalName:        signalName,
		Input:             input,
		Identity:          r.Identity,
		RequestId:         stableRequestID("signal", workflowID, runID, signalName, requestID),
	})
	return classifiedControlError(err)
}

func (r *TemporalRuntime) Cancel(ctx context.Context, workflowID, runID, requestID string) error {
	workflowID = normalizedWorkflowID(workflowID)
	_, err := r.service().RequestCancelWorkflowExecution(ctx, &workflowservice.RequestCancelWorkflowExecutionRequest{
		Namespace:         r.Namespace,
		WorkflowExecution: executionRef(workflowID, runID),
		Identity:          r.Identity,
		RequestId:         stableRequestID("cancel", workflowID, runID, requestID),
		Reason:            "owner requested workflow cancellation",
	})
	return classifiedControlError(err)
}

func (r *TemporalRuntime) Terminate(ctx context.Context, workflowID, runID, reason, requestID string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("workflow termination reason is required")
	}
	workflowID = normalizedWorkflowID(workflowID)
	details, err := converter.GetDefaultDataConverter().ToPayloads(map[string]string{"request_id": requestID})
	if err != nil {
		return err
	}
	_, err = r.service().TerminateWorkflowExecution(ctx, &workflowservice.TerminateWorkflowExecutionRequest{
		Namespace:         r.Namespace,
		WorkflowExecution: executionRef(workflowID, runID),
		Reason:            reason,
		Details:           details,
		Identity:          r.Identity,
	})
	return err
}

func (r *TemporalRuntime) Reset(ctx context.Context, workflowID, runID string, historyPoint int64, reason, requestID string) (core.WorkflowExecution, error) {
	if historyPoint < 1 {
		return core.WorkflowExecution{}, errors.New("history_point must be greater than zero")
	}
	workflowID = normalizedWorkflowID(workflowID)
	if runID == "" {
		execution, err := r.Status(ctx, workflowID, "")
		if err != nil {
			return core.WorkflowExecution{}, err
		}
		runID = execution.RunID
	}
	// Temporal only permits reset at a completed workflow-task boundary. Do
	// this validation before issuing the destructive reset request so a caller
	// cannot accidentally fork a run from an arbitrary activity/timer event.
	iter := r.Client.GetWorkflowHistory(ctx, workflowID, runID, false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	validPoint := false
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			return core.WorkflowExecution{}, err
		}
		if event != nil && event.GetEventId() == historyPoint {
			validPoint = event.GetEventType() == enumspb.EVENT_TYPE_WORKFLOW_TASK_COMPLETED
			break
		}
	}
	if !validPoint {
		return core.WorkflowExecution{}, fmt.Errorf("%w: history_point must reference a completed workflow task", core.ErrWorkflowCommandRejected)
	}
	response, err := r.service().ResetWorkflowExecution(ctx, &workflowservice.ResetWorkflowExecutionRequest{
		Namespace:                 r.Namespace,
		WorkflowExecution:         executionRef(workflowID, runID),
		Reason:                    reason,
		WorkflowTaskFinishEventId: historyPoint,
		RequestId:                 stableRequestID("reset", workflowID, runID, requestID),
		Identity:                  r.Identity,
		ResetReapplyType:          enumspb.RESET_REAPPLY_TYPE_NONE,
	})
	if err != nil {
		return core.WorkflowExecution{}, classifiedControlError(err)
	}
	newRunID := ""
	if response != nil && response.RunId != "" {
		newRunID = response.RunId
	}
	return r.Status(ctx, workflowID, newRunID)
}

func (r *TemporalRuntime) Restart(ctx context.Context, spec core.WorkflowStart) (core.WorkflowExecution, error) {
	if strings.TrimSpace(spec.WorkflowID) == "" || strings.TrimSpace(spec.TaskQueue) == "" {
		return core.WorkflowExecution{}, errors.New("workflow restart identity is incomplete")
	}
	fn, err := workflowFunction(spec.IntentType)
	if err != nil {
		return core.WorkflowExecution{}, err
	}
	var input Input
	if err := json.Unmarshal(spec.Payload, &input); err != nil {
		return core.WorkflowExecution{}, fmt.Errorf("workflow payload is invalid: %w", err)
	}
	if input.IntentID == "" {
		input.IntentID = spec.WorkflowID
	}
	workflowID := normalizedWorkflowID(spec.WorkflowID)
	reusePolicy := enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE
	if spec.OwnerCommandID != "" {
		reusePolicy = enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE
	}
	run, err := r.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                                       workflowID,
		TaskQueue:                                spec.TaskQueue,
		WorkflowIDReusePolicy:                    reusePolicy,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
		Memo:                                     map[string]interface{}{"owner_control_command_id": spec.OwnerCommandID},
		WorkflowExecutionTimeout:                 30 * time.Minute,
	}, fn, input)
	if err != nil {
		if temporal.IsWorkflowExecutionAlreadyStartedError(err) && spec.OwnerCommandID != "" {
			existing, lookupErr := r.Status(ctx, workflowID, "")
			if lookupErr != nil {
				return core.WorkflowExecution{}, lookupErr
			}
			if existing.OwnerCommandID == spec.OwnerCommandID {
				return existing, nil
			}
		}
		return core.WorkflowExecution{}, classifiedControlError(err)
	}
	runID := ""
	if run != nil {
		runID = run.GetRunID()
	}
	return r.Status(ctx, workflowID, runID)
}

func (r *TemporalRuntime) service() workflowservice.WorkflowServiceClient {
	return r.Client.WorkflowService()
}

func workflowFunction(intentType string) (any, error) {
	switch intentType {
	case "media.delete":
		return MediaDeletionWorkflow, nil
	case "studio.generate":
		return StudioWorkflow, nil
	case "diary.generate":
		return DiaryWorkflow, nil
	case "visual_identity.initialize":
		return VisualIdentityWorkflow, nil
	case "wake_up.current":
		return WakeUpWorkflow, nil
	case "daily_review.current_day":
		return DailyReviewWorkflow, nil
	case "schedule.current_day":
		return CurrentDayScheduleWorkflow, nil
	case "autonomy.action":
		return AutonomyActionWorkflow, nil
	case "capability.action":
		return CapabilityActionWorkflow, nil
	case "media.generation":
		return MediaWorkflow, nil
	case "goal.plan":
		return GoalPlanningWorkflow, nil
	case "goal.evaluate":
		return GoalEvaluationWorkflow, nil
	case "reflection.run":
		return ReflectionWorkflow, nil
	case "intention.trigger":
		return IntentionTriggerWorkflow, nil
	case "memory.embedding":
		return MemoryEmbeddingWorkflow, nil
	case "persona.owner_switch":
		return OwnerPersonaSwitchWorkflow, nil
	case "conversation.summary":
		return ConversationSummaryWorkflow, nil
	case "conversation.segment":
		return ConversationSegmentWorkflow, nil
	case "conversation.daily_memory":
		return ConversationDailyMemoryWorkflow, nil
	case "cognition.processing":
		return CognitionProcessingWorkflow, nil
	case "platform.control":
		return PlatformControlWorkflow, nil
	default:
		return nil, fmt.Errorf("unsupported workflow intent type %q", intentType)
	}
}

func executionFromInfo(info *workflowapi.WorkflowExecutionInfo) core.WorkflowExecution {
	if info == nil {
		return core.WorkflowExecution{}
	}
	result := core.WorkflowExecution{
		WorkflowID:    info.Execution.GetWorkflowId(),
		RunID:         info.Execution.GetRunId(),
		WorkflowType:  info.Type.GetName(),
		TaskQueue:     info.TaskQueue,
		Status:        strings.ToLower(info.Status.String()),
		HistoryLength: info.HistoryLength,
	}
	if payload := info.GetMemo().GetFields()["owner_control_command_id"]; payload != nil {
		_ = converter.GetDefaultDataConverter().FromPayload(payload, &result.OwnerCommandID)
	}
	if info.StartTime != nil {
		value := info.StartTime.AsTime()
		result.StartTime = &value
	}
	if info.CloseTime != nil {
		value := info.CloseTime.AsTime()
		result.CloseTime = &value
	}
	return result
}

func executionRef(workflowID, runID string) *commonpb.WorkflowExecution {
	return &commonpb.WorkflowExecution{WorkflowId: workflowID, RunId: runID}
}

func stableRequestID(kind, workflowID, runID string, values ...string) string {
	parts := append([]string{kind, workflowID, runID}, values...)
	return strings.Join(parts, ":")
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func (r *TemporalRuntime) HistoryPage(ctx context.Context, wf, run string, token []byte, limit int) (core.WorkflowHistoryPage, error) {
	if r == nil || r.Client == nil {
		return core.WorkflowHistoryPage{}, errors.New("workflow runtime unavailable")
	}
	if run == "" {
		return core.WorkflowHistoryPage{}, errors.New("concrete run is required")
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	response, err := r.service().GetWorkflowExecutionHistory(ctx, &workflowservice.GetWorkflowExecutionHistoryRequest{Namespace: r.Namespace, Execution: executionRef(normalizedWorkflowID(wf), run), MaximumPageSize: int32(limit), NextPageToken: token, WaitNewEvent: false, HistoryEventFilterType: enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT})
	if err != nil {
		return core.WorkflowHistoryPage{}, err
	}
	result := core.WorkflowHistoryPage{WorkflowID: normalizedWorkflowID(wf), RunID: run, Events: []core.WorkflowHistoryEvent{}, NextToken: response.NextPageToken}
	for _, event := range response.GetHistory().GetEvents() {
		item := core.WorkflowHistoryEvent{ID: event.GetEventId(), Type: event.GetEventType().String(), Resettable: event.GetEventType() == enumspb.EVENT_TYPE_WORKFLOW_TASK_COMPLETED, Details: map[string]any{"version": event.GetVersion(), "task_id": event.GetTaskId()}}
		if event.EventTime != nil {
			at := event.EventTime.AsTime()
			item.At = &at
		}
		if v := event.GetActivityTaskScheduledEventAttributes(); v != nil {
			item.Details["activity_id"] = v.ActivityId
			item.Details["activity_type"] = v.ActivityType.GetName()
			item.Details["task_queue"] = v.TaskQueue.GetName()
		}
		if v := event.GetActivityTaskFailedEventAttributes(); v != nil {
			item.Details["retry_state"] = v.RetryState.String()
			item.Details["failure_type"] = v.Failure.GetApplicationFailureInfo().GetType()
			item.Details["scheduled_event_id"] = v.ScheduledEventId
		}
		if v := event.GetWorkflowTaskFailedEventAttributes(); v != nil {
			item.Details["cause"] = v.Cause.String()
		}
		if v := event.GetTimerStartedEventAttributes(); v != nil {
			item.Details["timer_id"] = v.TimerId
		}
		if v := event.GetWorkflowExecutionSignaledEventAttributes(); v != nil {
			item.Details["signal_name"] = v.SignalName
		}
		result.Events = append(result.Events, item)
	}
	return result, nil
}

func classifiedControlError(err error) error {
	if err == nil {
		return nil
	}
	var invalid *serviceerror.InvalidArgument
	var precondition *serviceerror.FailedPrecondition
	var missing *serviceerror.NotFound
	var started *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &invalid) || errors.As(err, &precondition) || errors.As(err, &missing) || errors.As(err, &started) {
		return fmt.Errorf("%w: %v", core.ErrWorkflowCommandRejected, err)
	}
	return err
}
