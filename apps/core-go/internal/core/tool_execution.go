package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
	"github.com/jackc/pgx/v5"
)

// ToolExecutionRequest is the application-facing Tool boundary shared by
// direct callers and the Agent adapter. OperationID identifies the business
// command; NativeToolCallID is diagnostic correlation supplied only when a
// model actually emitted a ToolCall.
type ToolExecutionRequest struct {
	CorrelationID        string
	WorkingProfileID     string
	AuthorizationPolicy  string
	SubjectActorID       string
	AgentID              FormalAgentID
	RunID                string
	CapabilityName       string
	OperationID          string
	NativeToolCallID     string
	ProviderRequestID    string
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	TargetKind           string
	TargetRef            string
	EvidenceID           string
	Surface              CapabilitySurface
	Arguments            json.RawMessage
	// FrozenContextSnapshot is supplied only by the Core-owned native Agent
	// adapter. It preserves the exact opaque refs shown on that model request;
	// standalone Tool callers continue to resolve their own authorized scope.
	FrozenContextSnapshot map[string]any
	// ExpectedCorePersonaRevision binds read-only persona detail calls to the
	// source version that produced the Agent's prompt. Direct callers may omit it.
	ExpectedCorePersonaRevision *int
	ExpectedOverlayRevision     *int
}

// DirectToolTarget carries the application-authorized resource binding which
// is intentionally absent from model-owned Tool arguments.
type DirectToolTarget = capability.DirectToolTarget

// DirectToolCapability is the generic standalone execution seam for a
// Capability whose durable business effect needs an application target. Both
// ordinary callers and native ToolCall adapters reach this interface through
// ExecuteTool; implementations do not branch on surface or Tool name.
type DirectToolCapability interface {
	Capability
	ExecuteDirectTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext, DirectToolTarget) (CapabilityResult, error)
}

// ToolExecutionReceipt keeps business idempotency and model correlation
// visibly separate while returning the canonical CapabilityResult unchanged.
type ToolExecutionReceipt struct {
	OperationID        string                 `json:"operation_id"`
	NativeToolCallID   string                 `json:"native_tool_call_id,omitempty"`
	ExecutionCallID    string                 `json:"execution_call_id"`
	Result             CapabilityResult       `json:"result"`
	AuthorityRevisions ToolAuthorityRevisions `json:"authority_revisions,omitempty"`
	Replayed           bool                   `json:"replayed"`
}

func (request ToolExecutionRequest) validate() error {
	if request.AuthorizationPolicy != "" && request.AuthorizationPolicy != "autonomy" {
		return fmt.Errorf("%w: unknown authorization policy", ErrUnauthorized)
	}
	if strings.TrimSpace(request.CapabilityName) == "" {
		return fmt.Errorf("%w: capability name is required", ErrInvalidArguments)
	}
	if strings.TrimSpace(request.OperationID) == "" || len([]rune(request.OperationID)) > 256 {
		return fmt.Errorf("%w: operation id is required", ErrInvalidArguments)
	}
	if strings.TrimSpace(request.AuthorizationActorID) == "" || strings.TrimSpace(request.FluctlightID) == "" {
		return fmt.Errorf("%w: authorized resource scope is required", ErrUnauthorized)
	}
	if _, err := normalizeToolArguments(string(request.Arguments)); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArguments, err)
	}
	return nil
}

// ExecuteTool executes one business Tool without Main, an Agent run, or a
// caller-owned transaction. Provider/planner preparation is completed before
// the short mutation transaction is opened.
func (a *App) ExecuteTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionReceipt, error) {
	if err := request.validate(); err != nil {
		return ToolExecutionReceipt{}, err
	}
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return ToolExecutionReceipt{}, errors.New("tool execution database unavailable")
	}
	resource, authorizationErr := a.DB.GetFluctlight(ctx, strings.TrimSpace(request.FluctlightID), strings.TrimSpace(request.AuthorizationActorID))
	if authorizationErr != nil {
		return ToolExecutionReceipt{}, fmt.Errorf("authorize tool resource: %w", authorizationErr)
	}
	if request.CapabilityName == personaDetailCapabilityName && request.ExpectedCorePersonaRevision != nil {
		ctx = context.WithValue(ctx, personaDetailRevisionContextKey{}, *request.ExpectedCorePersonaRevision)
	}
	if request.CapabilityName == personaDetailCapabilityName && request.ExpectedOverlayRevision != nil {
		ctx = context.WithValue(ctx, personaDetailOverlayContextKey{}, *request.ExpectedOverlayRevision)
	}
	if request.WorkingProfileID != "" {
		profiles := personalityProfileIDs(resource.CorePersona)
		_, declared := profiles[request.WorkingProfileID]
		if !declared && request.WorkingProfileID != initialPersonalityProfileID(resource.CorePersona) {
			return ToolExecutionReceipt{}, newCapabilityError("working_profile_not_found", false, ErrNotFound)
		}
	}
	if len(request.FrozenContextSnapshot) > 0 {
		if strings.TrimSpace(request.NativeToolCallID) == "" || strings.TrimSpace(request.ProviderRequestID) == "" {
			return ToolExecutionReceipt{}, fmt.Errorf("%w: frozen Tool context requires a native call", ErrInvalidArguments)
		}
		if err := validateCandidateSnapshotIdentity(request.FrozenContextSnapshot, candidateValidationContext{
			FluctlightID: request.FluctlightID, ConversationID: request.ConversationID, SourceFactID: request.EvidenceID,
		}); err != nil {
			return ToolExecutionReceipt{}, fmt.Errorf("%w: %v", ErrInvalidArguments, err)
		}
	}
	// Assemble immutable execution dependencies for this call. Do not mutate
	// App.Runtime while concurrent direct callers are resolving their scope.
	resolver := a.ContextResolver
	if resolver == nil {
		resolver = NewAppContextResolver(a)
	}
	request.SubjectActorID = firstString(request.SubjectActorID, request.AuthorizationActorID)
	if request.SubjectActorID != request.AuthorizationActorID && request.SubjectActorID != request.FluctlightID {
		if request.ConversationID != "" {
			if err := a.authorizeActorTurn(ctx, request.AuthorizationActorID, request.SubjectActorID, request.FluctlightID, request.ConversationID); err != nil {
				return ToolExecutionReceipt{}, err
			}
		} else if _, err := a.DB.GetFluctlight(ctx, request.SubjectActorID, request.AuthorizationActorID); err != nil {
			return ToolExecutionReceipt{}, err
		}
	}
	if request.WorkingProfileID != "" {
		resolver = toolProfileContextResolver{base: resolver, profile: request.WorkingProfileID}
	}
	runtime, runtimeErr := NewCapabilityRuntime(a.capabilityRegistry(), resolver)
	if runtimeErr != nil {
		return ToolExecutionReceipt{}, runtimeErr
	}
	if runtime.Registry == nil {
		return ToolExecutionReceipt{}, ErrCapabilityNotFound
	}
	definition, ok := runtime.Registry.Definition(strings.TrimSpace(request.CapabilityName))
	if !ok {
		return ToolExecutionReceipt{}, fmt.Errorf("%w: %s", ErrCapabilityNotFound, request.CapabilityName)
	}
	callID := strings.TrimSpace(request.NativeToolCallID)
	source := "model_tool"
	if callID == "" {
		callID = "direct_call_" + stableDigest(request.FluctlightID + "\x1f" + request.CapabilityName + "\x1f" + request.OperationID)[:32]
		source = "direct"
	}
	evidenceID := strings.TrimSpace(request.EvidenceID)
	providerRequestID := strings.TrimSpace(request.ProviderRequestID)
	if source == "model_tool" && providerRequestID == "" {
		return ToolExecutionReceipt{}, fmt.Errorf("%w: native ToolCall requires its real provider request id", ErrInvalidArguments)
	}
	surface := request.Surface
	if surface == "" {
		surface = CapabilitySurfaceNativeCognition
	}
	// Agent definitions own tool selection. The Tool execution contract is
	// identical for direct and native calls and has no surface-stage gate.
	invocation := CapabilityInvocation{
		CallID: callID, CapabilityName: definition.Name, SchemaVersion: CapabilityInvocationSchemaVersion,
		Arguments: append(json.RawMessage(nil), request.Arguments...), SourceFactID: evidenceID,
		ProviderRequestID: providerRequestID, Sequence: 0,
		Metadata: InvocationMetadata{
			CorrelationID: firstString(request.CorrelationID, "tool-operation:"+request.OperationID), Source: source,
			AuthorizationActorID: request.AuthorizationActorID, SubjectActorID: request.SubjectActorID, WorkingProfileID: request.WorkingProfileID,
			OperationID: request.OperationID, Surface: surface,
			FluctlightID: request.FluctlightID, ConversationID: request.ConversationID,
		},
	}
	if len(request.FrozenContextSnapshot) > 0 {
		invocation.ContextSnapshot = mapValue(boundedSnapshotValue(request.FrozenContextSnapshot))
		if len(invocation.ContextSnapshot) == 0 {
			return ToolExecutionReceipt{}, fmt.Errorf("%w: frozen Tool context exceeds the snapshot limit", ErrInvalidArguments)
		}
	}
	implementation, ok := runtime.Registry.LookupCapability(definition.Name)
	if !ok {
		return ToolExecutionReceipt{}, ErrCapabilityNotFound
	}
	executionClass, err := classifyCapabilityExecution(implementation, definition)
	if err != nil {
		return ToolExecutionReceipt{}, err
	}
	if executionClass != CapabilityExecutionPureQuery {
		stored, authority, found, readErr := readToolExecution(ctx, a.DB.Pool(), request)
		if readErr != nil {
			code, retryable := capabilityErrorInfo(readErr, "tool_receipt_read_failed", true)
			return toolExecutionReceipt(request, callID, failedCapabilityResultDetail(invocation, code, retryable, readErr.Error())), readErr
		}
		if found {
			stored.CallID, stored.ProviderRequestID = callID, providerRequestID
			receipt := toolExecutionReceipt(request, callID, stored)
			receipt.AuthorityRevisions = authority
			receipt.Replayed = true
			return receipt, nil
		}
	}
	prepared, resolved, err := runtime.Prepare(ctx, invocation)
	if err != nil {
		code, retryable, detail := classifyToolPrepareError(err)
		return toolExecutionReceipt(request, callID, failedCapabilityResultDetail(invocation, code, retryable, detail)), err
	}
	var result CapabilityResult
	var authority ToolAuthorityRevisions
	var replayed bool
	if direct, directOK := implementation.(DirectToolCapability); directOK {
		target := DirectToolTarget{
			AuthorizationPolicy:  request.AuthorizationPolicy,
			AuthorizationActorID: strings.TrimSpace(request.AuthorizationActorID),
			FluctlightID:         strings.TrimSpace(request.FluctlightID),
			ConversationID:       strings.TrimSpace(request.ConversationID),
			Kind:                 strings.TrimSpace(request.TargetKind),
			Ref:                  strings.TrimSpace(request.TargetRef),
		}
		result, authority, replayed, err = a.executeToolMutation(ctx, request, func(tx pgx.Tx) (CapabilityResult, error) {
			result, err := direct.ExecuteDirectTx(ctx, tx, prepared, resolved, target)
			if err != nil {
				return result, err
			}
			if err := result.Validate(prepared); err != nil {
				return result, err
			}
			if result.Status == "completed" || result.Status == "accepted" {
				if err := definition.ValidateOutput(result.Output); err != nil {
					return result, fmt.Errorf("tool output contract invalid: %w", err)
				}
			}
			return result, nil
		})
	} else {
		switch executionClass {
		case CapabilityExecutionPureQuery:
			result, err = runtime.Execute(ctx, prepared)
		case CapabilityExecutionTransactionalMutation, CapabilityExecutionExternalAsyncIntent:
			result, authority, replayed, err = a.executeToolMutation(ctx, request, func(tx pgx.Tx) (CapabilityResult, error) {
				return runtime.ExecuteTransactional(ctx, tx, prepared)
			})
		default:
			err = newCapabilityError("direct_execution_not_implemented", false, fmt.Errorf("capability %q has no direct execution implementation", definition.Name))
			result = failedCapabilityResultDetail(prepared, "direct_execution_not_implemented", false, err.Error())
		}
	}
	if err != nil && (result.Status == "" || result.Status == "completed" || result.Status == "accepted") {
		code, retryable := capabilityErrorInfo(err, "tool_execution_dependency_failed", true)
		result = failedCapabilityResultDetail(prepared, code, retryable, err.Error())
	}
	if result.CallID == "" {
		result.CallID = prepared.CallID
	}
	if result.CapabilityName == "" {
		result.CapabilityName = prepared.CapabilityName
	}
	if result.ProviderRequestID == "" {
		result.ProviderRequestID = prepared.ProviderRequestID
	}
	if result.CorrelationID == "" {
		result.CorrelationID = prepared.Metadata.CorrelationID
	}
	result.ActingProfileID = request.WorkingProfileID
	result.RequiredContext = append([]ContextSlot(nil), definition.RequiredContext...)
	receipt := toolExecutionReceipt(request, callID, result)
	receipt.AuthorityRevisions = authority
	receipt.Replayed = replayed
	output := mapValue(result.Output)
	if err == nil && (result.Status == "completed" || result.Status == "accepted") && strings.TrimSpace(stringValue(output["inbox_id"])) != "" {
		if preemptErr := a.CancelLifecycleForCognition(ctx, request.FluctlightID, "cognition:"+stringValue(output["inbox_id"])); preemptErr != nil {
			a.recordDiagnosticEvent(ctx, "cognition.lifecycle_preemption.degraded", "warning", request.FluctlightID, stringValue(output["inbox_id"]), result.CorrelationID, map[string]any{"error_code": "lifecycle_preemption_failed"})
		}
	}
	if err == nil && result.Status == "completed" && stringValue(output["target_kind"]) == "conversation_message" && stringValue(output["delivery_status"]) != "duplicate_suppressed" {
		var followupErr error
		if request.Surface == CapabilitySurfaceWakeUp {
			followupErr = a.schedulePendingReflectionTrigger(ctx, request.FluctlightID)
		} else {
			followupErr = a.scheduleCognitionFollowups(ctx, request.FluctlightID)
		}
		if followupErr != nil {
			a.recordDiagnosticEvent(ctx, "conversation.publication.followup.degraded", "warning", request.FluctlightID, stringValue(output["target_ref"]), result.CorrelationID, map[string]any{"error_code": "followup_schedule_failed"})
		}
	}
	return receipt, err
}

func classifyToolPrepareError(err error) (code string, retryable bool, detail string) {
	code, retryable = capabilityErrorInfo(err, "capability_prepare_failed", true)
	if !errors.Is(err, ErrInvalidArguments) && !errors.Is(err, capability.ErrInvalidArguments) {
		return code, retryable, err.Error()
	}
	if code == "capability_prepare_failed" {
		code = "invalid_arguments"
	}
	return code, false, safeToolArgumentFeedback(err)
}

func safeToolArgumentFeedback(err error) string {
	if err == nil {
		return "arguments do not match tool schema"
	}
	detail := strings.TrimSpace(err.Error())
	for strings.HasPrefix(detail, "invalid capability arguments: ") {
		detail = strings.TrimPrefix(detail, "invalid capability arguments: ")
	}
	if strings.Contains(detail, "additional property") {
		return "unexpected argument field; use the published tool schema"
	}
	if !strings.HasPrefix(detail, `field "`) && !strings.HasPrefix(detail, `required field "`) &&
		!strings.HasPrefix(detail, "value must ") && !strings.HasPrefix(detail, "arguments_") &&
		!strings.HasPrefix(detail, "arguments must ") && !strings.HasPrefix(detail, "array ") &&
		!strings.HasPrefix(detail, "string ") && !strings.HasPrefix(detail, "number ") &&
		!strings.HasPrefix(detail, "integer ") {
		return "arguments do not match tool schema"
	}
	const maxFeedbackRunes = 240
	runes := []rune(detail)
	if len(runes) > maxFeedbackRunes {
		detail = string(runes[:maxFeedbackRunes])
	}
	return detail
}

func toolExecutionReceipt(request ToolExecutionRequest, callID string, result CapabilityResult) ToolExecutionReceipt {
	return ToolExecutionReceipt{
		OperationID: strings.TrimSpace(request.OperationID), NativeToolCallID: strings.TrimSpace(request.NativeToolCallID),
		ExecutionCallID: callID, Result: result,
	}
}
