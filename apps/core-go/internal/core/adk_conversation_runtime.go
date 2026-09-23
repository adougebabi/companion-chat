package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
)

type adkConversationContextKey struct{}

// ADKCapabilityTrace is request-scoped state used to carry the canonical
// invocation/result pair from an ADK tool callback into Core's existing frozen
// settlement path.
type ADKCapabilityTrace = aiagent.ADKCapabilityTrace

type adkConversationContext struct {
	Invoker ADKCapabilityInvoker
	Trace   *ADKCapabilityTrace
}

func WithADKCapabilityInvoker(ctx context.Context, invoker ADKCapabilityInvoker, trace *ADKCapabilityTrace) context.Context {
	return context.WithValue(ctx, adkConversationContextKey{}, adkConversationContext{Invoker: invoker, Trace: trace})
}

func adkCapabilityContext(ctx context.Context) (adkConversationContext, bool) {
	value, ok := ctx.Value(adkConversationContextKey{}).(adkConversationContext)
	if !ok || value.Invoker == nil {
		return adkConversationContext{}, false
	}
	return value, true
}

type appADKCapabilityInvoker struct {
	app     *App
	request ADKCapabilityRequest
	trace   *ADKCapabilityTrace
}

// ADKCapabilityRequest is the request-scoped identity and projection supplied
// to a surface-specific ADK capability bridge. It contains no App, database
// handle or transaction; the bridge resolves/executes through CapabilityRuntime.
type ADKCapabilityRequest struct {
	AuthorizationPolicy  string
	TargetKind           string
	TargetRef            string
	AuthorizationActorID string
	SubjectActorID       string
	FluctlightID         string
	ConversationID       string
	SourceFactID         string
	ActionID             string
	OperationID          string
	CorrelationID        string
	Surface              CapabilitySurface
	Projection           ContextProjection
}

func newAppADKCapabilityInvoker(app *App, request ADKCapabilityRequest, trace *ADKCapabilityTrace) ADKCapabilityInvoker {
	if request.Surface == "" {
		request.Surface = CapabilitySurfaceConversation
	}
	if request.TargetKind == "" && request.TargetRef == "" && request.ConversationID != "" {
		request.TargetKind, request.TargetRef = "conversation", request.ConversationID
	}
	return &appADKCapabilityInvoker{app: app, request: request, trace: trace}
}

func (i *appADKCapabilityInvoker) Execute(ctx context.Context, capabilityName string, argumentsJSON string) (string, error) {
	return i.ExecuteWithID(ctx, "", capabilityName, argumentsJSON)
}

func (i *appADKCapabilityInvoker) ExecuteWithID(ctx context.Context, callID, capabilityName string, argumentsJSON string) (string, error) {
	if i == nil || i.app == nil || i.trace == nil {
		return "", errors.New("adk_capability_invoker_unavailable")
	}
	i.recordADKToolDiagnostic(ctx, "adk.tool.requested", callID, capabilityName, "requested", "", argumentsJSON)
	definition, ok := i.app.capabilityRegistry().Definition(capabilityName)
	if !ok {
		i.recordADKToolDiagnostic(ctx, "adk.tool.rejected", callID, capabilityName, "rejected", "capability_not_found", argumentsJSON)
		return "", fmt.Errorf("capability_not_found: %s", capabilityName)
	}
	arguments := json.RawMessage(strings.TrimSpace(argumentsJSON))
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	callID = strings.TrimSpace(callID)
	if callID == "" {
		i.recordADKToolDiagnostic(ctx, "adk.tool.rejected", callID, capabilityName, "rejected", "adk_tool_call_id_required", argumentsJSON)
		return "", errors.New("adk_tool_call_id_required")
	}
	// Native call identity is diagnostic correlation only. The stable business
	// operation below is derived from run coordinates and remains unchanged when
	// a retried model emits another native call ID for the same operation.
	if normalizedArguments, normalizeErr := normalizeToolArguments(arguments); normalizeErr == nil {
		if previous, found := i.trace.FindInvocation(callID); found {
			previousArguments, previousErr := normalizeToolArguments(previous.Arguments)
			if previous.CapabilityName != capabilityName || previousErr != nil || string(previousArguments) != string(normalizedArguments) {
				i.recordADKToolDiagnostic(ctx, "adk.tool.rejected", callID, capabilityName, "rejected", "adk_tool_call_id_reused", argumentsJSON)
				return "", errors.New("adk_tool_call_id_reused")
			}
			if previousResult, resultFound := i.trace.FindResult(callID); resultFound {
				return jsonString(map[string]any{
					"status": previousResult.Status, "capability": previousResult.CapabilityName,
					"error_code": previousResult.ErrorCode, "output": previousResult.Output,
				}), nil
			}
			return "", errors.New("adk_tool_result_missing")
		}
	}
	modelIdentity, ok := i.trace.ModelIdentity(callID)
	if !ok || strings.TrimSpace(modelIdentity.ProviderRequestID) == "" {
		i.recordADKToolDiagnostic(ctx, "adk.tool.rejected", callID, capabilityName, "rejected", "adk_tool_provider_request_identity_missing", argumentsJSON)
		return "", errors.New("adk_tool_provider_request_identity_missing")
	}
	authorizationActorID := strings.TrimSpace(i.request.AuthorizationActorID)
	if authorizationActorID == "" {
		authorizationActorID = strings.TrimSpace(i.request.Projection.OwnerActorID)
	}
	subjectActorID := firstString(i.request.SubjectActorID, firstString(i.request.Projection.ReferenceIndex.SpeakerActorID, authorizationActorID))
	operationRoot := firstString(i.request.OperationID, firstString(i.request.ActionID, i.request.SourceFactID))
	if operationRoot == "" {
		return "", errors.New("adk_tool_operation_root_required")
	}
	operationID := "agent_tool_" + stableDigest(fmt.Sprintf("%s\x1f%d\x1f%d\x1f%s", operationRoot, modelIdentity.ModelCallSequence, modelIdentity.ToolIndex, capabilityName))
	workingProfileID := workingProfileForToolExecution(i.request.Projection, i.trace)
	var expectedPersonaRevision *int
	var expectedOverlayRevision *int
	if capabilityName == personaDetailCapabilityName {
		revision, overlayRevision := personaDetailExpectedRevisions(i.request.Projection, i.trace, workingProfileID)
		expectedPersonaRevision = &revision
		expectedOverlayRevision = &overlayRevision
	}
	invocations, _ := i.trace.Snapshot()
	invocation := normalizeCapabilityInvocationMetadata(CapabilityInvocation{
		CallID: callID, CapabilityName: capabilityName, Arguments: arguments,
		SourceFactID: i.request.SourceFactID, ActionID: i.request.ActionID,
		ProviderRequestID: modelIdentity.ProviderRequestID, Sequence: len(invocations),
		Metadata: InvocationMetadata{WorkingProfileID: workingProfileID, AuthorizationActorID: authorizationActorID, SubjectActorID: subjectActorID, CorrelationID: firstString(providerCorrelation(ctx), firstString(i.request.CorrelationID, "turn:"+i.request.SourceFactID)), OperationID: operationID, FluctlightID: i.request.FluctlightID, ConversationID: i.request.ConversationID, Surface: i.request.Surface, Source: "model_tool"},
	}, i.request.FluctlightID, i.request.ConversationID, i.request.SourceFactID, i.request.SourceFactID, len(invocations))
	invocation.ActionID = i.request.ActionID
	// Context snapshots are required for contextful capabilities. A
	// contextless capability must not receive a partial identity-only snapshot
	// assembled from an optional/empty Projection: candidate validation would
	// (correctly) reject that incomplete snapshot even though the capability
	// does not need any context slot. The later freeze/bind boundary still
	// attaches the full Core-owned snapshot to every persisted invocation.
	if len(definition.RequiredContext) > 0 {
		invocation.ContextSnapshot = capabilitySnapshotForProjection(i.request.Projection, definition.RequiredContext, i.request.ActionID)
	}
	i.trace.AppendInvocation(invocation)
	i.recordADKToolDiagnostic(ctx, "adk.tool.dispatched", callID, capabilityName, "dispatched", "", argumentsJSON)
	agentDefinition, _ := formalAgentDefinitionFromContext(ctx)
	receipt, execErr := i.app.ExecuteTool(ctx, ToolExecutionRequest{
		AgentID: agentDefinition.ID, RunID: operationRoot,
		CorrelationID:       i.request.CorrelationID,
		AuthorizationPolicy: i.request.AuthorizationPolicy,
		WorkingProfileID:    workingProfileID,
		CapabilityName:      capabilityName, OperationID: operationID,
		NativeToolCallID: callID, ProviderRequestID: modelIdentity.ProviderRequestID,
		AuthorizationActorID: authorizationActorID, FluctlightID: i.request.FluctlightID,
		SubjectActorID: subjectActorID,
		ConversationID: i.request.ConversationID, EvidenceID: i.request.SourceFactID,
		Surface: i.request.Surface, Arguments: arguments,
		ExpectedCorePersonaRevision: expectedPersonaRevision,
		ExpectedOverlayRevision:     expectedOverlayRevision,
		TargetKind:                  i.request.TargetKind, TargetRef: i.request.TargetRef,
	})
	result := receipt.Result
	if result.CallID == "" {
		result = CapabilityResult{
			CallID: callID, CapabilityName: capabilityName, Status: "failed", Retryable: true,
			ErrorCode: "tool_execution_failed", ProviderRequestID: modelIdentity.ProviderRequestID,
			CorrelationID: "capability:" + callID, RequiredContext: append([]ContextSlot(nil), definition.RequiredContext...),
		}
		if execErr != nil {
			result.Output = map[string]any{"detail": execErr.Error()}
		}
		receipt = ToolExecutionReceipt{OperationID: operationID, NativeToolCallID: callID, ExecutionCallID: callID, Result: result}
	}
	i.trace.AppendResult(result)
	i.recordADKToolDiagnostic(ctx, "adk.tool.result", callID, capabilityName, result.Status, result.ErrorCode, argumentsJSON)
	serialized := jsonString(receipt)
	if execErr != nil && result.Retryable {
		// Runtime/dependency failures terminate this run while the trace retains
		// the invocation and receipt. A non-retryable business rejection is a
		// normal tool result and is returned to the model for another decision.
		return serialized, fmt.Errorf("tool execution %s: %w", result.ErrorCode, execErr)
	}
	return serialized, nil
}

func (i *appADKCapabilityInvoker) recordADKToolDiagnostic(ctx context.Context, eventType, callID, capabilityName, status, errorCode, arguments string) {
	if i == nil || i.app == nil || i.app.DB == nil || i.app.DB.Pool() == nil {
		return
	}
	correlationID := firstString(providerCorrelation(ctx), firstString(i.request.CorrelationID, "turn:"+i.request.SourceFactID))
	diagnostics := providerPromptDiagnostics(ctx)
	runID := stringValue(diagnostics["run_id"])
	if runID == "" {
		runID = correlationID
	}
	newProviderRuntimeSupport(i.app.DB).RecordDiagnosticEvent(ctx, eventType, statusSeverity(status), i.request.FluctlightID, i.request.SourceFactID, correlationID, map[string]any{
		"run_id": runID, "stage": "tool", "surface": i.request.Surface,
		"model_call_id": stringValue(diagnostics["model_call_id"]),
		"call_id":       strings.TrimSpace(callID), "capability": strings.TrimSpace(capabilityName),
		"status": strings.TrimSpace(status), "error_code": strings.TrimSpace(errorCode),
		"arguments_digest": stableDigest(strings.TrimSpace(arguments)),
	})
}

func statusSeverity(status string) string {
	switch strings.TrimSpace(status) {
	case "failed", "rejected", "cancelled", "timeout":
		return "error"
	case "deferred", "accepted_pending":
		return "notice"
	default:
		return "info"
	}
}

// ADKCapabilityInvoker is the only dependency an ADK tool adapter receives.
// The caller binds a request-scoped CapabilityRuntime closure that owns
// authorization, context snapshots, idempotency and settlement; the adapter
// never receives *App, a repository or a transaction.
type ADKCapabilityInvoker = aiagent.ADKCapabilityInvoker
type ADKCapabilityInvokerWithID = aiagent.ADKCapabilityInvokerWithID
type ADKLoopConfig = aiagent.ADKLoopConfig
type ADKLoopResult = aiagent.ADKLoopResult
type adkCapabilityTool = aiagent.ADKCapabilityTool

// NewADKCapabilityTools exposes only the installed capability schemas and a
// request-scoped executor.
func NewADKCapabilityTools(definitions []CapabilityDefinition, invoker ADKCapabilityInvoker) ([]tool.BaseTool, error) {
	return aiagent.NewADKCapabilityTools(definitions, invoker)
}

// RunADKLoop is the only Eino ADK model/tool event iterator used by Core.
func RunADKLoop(ctx context.Context, config ADKLoopConfig, messages []*schema.Message) (ADKLoopResult, error) {
	return aiagent.RunADKLoop(ctx, config, messages)
}

// ADKStructuredTaskInput is the shared Provider-facing background/conversation
// boundary. Domain callers keep their own result and settlement contracts.
type ADKStructuredTaskInput struct {
	AgentID         FormalAgentID
	Role            string
	Scenario        string
	Prompt          PromptAssemblyResult
	Definitions     []CapabilityDefinition
	SchemaName      string
	EnableThinking  bool
	EnableStreaming bool
	TextOutput      bool
	Capability      *ADKCapabilityRequest
}

type ADKStructuredTaskResult struct {
	Completion ProviderCompletion
	Trace      *ADKCapabilityTrace
}

// RunADKStructuredTask uses the shared surface-aware ADK bridge. It does not
// freeze, settle, publish or open a transaction; the caller owns those rules.
func (a *App) RunADKStructuredTask(ctx context.Context, input ADKStructuredTaskInput) (ADKStructuredTaskResult, error) {
	if a == nil || a.Provider == nil {
		return ADKStructuredTaskResult{}, errors.New("adk_structured_task_provider_unavailable")
	}
	definition, ok := formalAgentDefinitionFromContext(ctx)
	if !ok {
		agentID := input.AgentID
		if agentID == "" {
			agentID, ok = formalAgentForSchema(input.SchemaName)
		}
		if agentID == "" {
			return ADKStructuredTaskResult{}, fmt.Errorf("formal_agent_not_registered_for_schema: %s", strings.TrimSpace(input.SchemaName))
		}
		definition, ok = FormalAgentDefinitionByID(agentID)
		if !ok {
			return ADKStructuredTaskResult{}, fmt.Errorf("formal_agent_not_registered: %s", agentID)
		}
	}
	definition.EnableStreaming = input.EnableStreaming || definition.EnableStreaming
	ctx = withFormalAgentDefinition(ctx, definition)
	if input.AgentID != "" && definition.ID != input.AgentID {
		return ADKStructuredTaskResult{}, errors.New("formal_agent_identity_mismatch")
	}
	if strings.TrimSpace(input.Role) == "" {
		input.Role = definition.Role
	}
	if input.Role != definition.Role {
		return ADKStructuredTaskResult{}, errors.New("formal_agent_role_mismatch")
	}
	if input.TextOutput != (definition.OutputKind == FormalAgentOutputText) {
		return ADKStructuredTaskResult{}, errors.New("formal_agent_output_contract_mismatch")
	}
	if len(input.Definitions) > 0 && input.Capability == nil {
		return ADKStructuredTaskResult{}, errors.New("adk_structured_task_capability_context_required")
	}
	if err := validateADKCapabilityDefinitions(a, input.Definitions, firstCapabilitySurface(input.Capability)); err != nil {
		return ADKStructuredTaskResult{}, err
	}
	trace := &ADKCapabilityTrace{}
	request := ADKCapabilityRequest{Surface: definition.DefaultSurface}
	if input.Capability != nil {
		request = *input.Capability
	}
	invoker := newAppADKCapabilityInvoker(a, request, trace)
	ctx = WithADKCapabilityInvoker(ctx, invoker, trace)
	if strings.TrimSpace(input.Scenario) != "" {
		ctx = WithProviderScenario(ctx, input.Scenario)
	}
	if !validAssembledProviderMessages(input.Prompt.Messages) {
		return ADKStructuredTaskResult{Trace: trace}, errors.New("adk_structured_task_prompt_invalid")
	}
	record, prior, admissionErr := a.admitFormalRun(ctx, definition, input)
	if admissionErr != nil || prior != nil {
		if prior != nil {
			return *prior, admissionErr
		}
		return ADKStructuredTaskResult{Trace: trace}, admissionErr
	}
	completion, err := a.Provider.AgentAssembledCompletion(ctx, input.Role, input.Prompt.Messages, input.Definitions, input.SchemaName, input.Prompt.ResponseFormat, !input.TextOutput, input.EnableThinking)
	result := ADKStructuredTaskResult{Completion: completion, Trace: trace}
	if persistErr := a.finishFormalRun(ctx, record, result, err); persistErr != nil {
		return result, errors.Join(err, persistErr)
	}
	return result, err
}

func firstCapabilitySurface(request *ADKCapabilityRequest) CapabilitySurface {
	if request == nil || request.Surface == "" {
		return CapabilitySurfaceConversation
	}
	return request.Surface
}

func validateADKCapabilityDefinitions(app *App, definitions []CapabilityDefinition, surface CapabilitySurface) error {
	if len(definitions) == 0 {
		return nil
	}
	if app == nil {
		return errors.New("adk_capability_registry_unavailable")
	}
	registry := app.capabilityRegistry()
	if registry == nil {
		return errors.New("adk_capability_registry_unavailable")
	}
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if _, duplicate := seen[definition.Name]; duplicate {
			return fmt.Errorf("adk_capability_definition_duplicate: %s", definition.Name)
		}
		seen[definition.Name] = struct{}{}
		canonical, ok := registry.Definition(definition.Name)
		if !ok {
			return fmt.Errorf("adk_capability_definition_unknown: %s", definition.Name)
		}
		if canonical.InternalOnly {
			return fmt.Errorf("adk_capability_definition_internal: %s", definition.Name)
		}
		if !reflect.DeepEqual(canonical, definition) {
			return fmt.Errorf("adk_capability_definition_mismatch: %s", definition.Name)
		}
	}
	return nil
}
