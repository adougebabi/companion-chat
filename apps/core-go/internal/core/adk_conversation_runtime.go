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
	Refresh *runtimeContextRefresh
	Refs    *providerContextRefCodec
}

func WithADKCapabilityInvoker(ctx context.Context, invoker ADKCapabilityInvoker, trace *ADKCapabilityTrace) context.Context {
	return withADKCapabilityContext(ctx, invoker, trace, nil)
}

func withADKCapabilityContext(ctx context.Context, invoker ADKCapabilityInvoker, trace *ADKCapabilityTrace, refresh *runtimeContextRefresh) context.Context {
	return withADKCapabilityContextRefs(ctx, invoker, trace, refresh, nil)
}

func withADKCapabilityContextRefs(ctx context.Context, invoker ADKCapabilityInvoker, trace *ADKCapabilityTrace, refresh *runtimeContextRefresh, refs *providerContextRefCodec) context.Context {
	return context.WithValue(ctx, adkConversationContextKey{}, adkConversationContext{Invoker: invoker, Trace: trace, Refresh: refresh, Refs: refs})
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
	projection := i.request.Projection
	if adkContext, ok := adkCapabilityContext(ctx); ok {
		if fresh := adkContext.Refresh.latestProjection(); fresh != nil {
			projection = *fresh
		}
	}
	i.recordADKToolDiagnostic(ctx, "adk.tool.requested", callID, capabilityName, "requested", "", argumentsJSON)
	definition, ok := i.app.capabilityRegistry().Definition(capabilityName)
	if !ok {
		// Unknown native calls cannot execute. Preserve an Owner-reviewable
		// capability need when the run has an authenticated source fact.
		i.recordMissingCapabilityRequest(ctx, callID, capabilityName, projection)
		i.recordADKToolDiagnostic(ctx, "adk.tool.rejected", callID, capabilityName, "rejected", "capability_not_found", argumentsJSON)
		return "", fmt.Errorf("capability_not_found: %s", capabilityName)
	}
	arguments := json.RawMessage(strings.TrimSpace(argumentsJSON))
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	if adkContext, ok := adkCapabilityContext(ctx); ok {
		decoded, err := adkContext.Refs.decodeArguments(arguments)
		if err != nil {
			return "", err
		}
		arguments = decoded
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
				return modelFacingToolResultForContext(ctx, ToolExecutionReceipt{Result: previousResult}, definition)
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
		authorizationActorID = strings.TrimSpace(projection.OwnerActorID)
	}
	subjectActorID := firstString(i.request.SubjectActorID, firstString(projection.ReferenceIndex.SpeakerActorID, authorizationActorID))
	operationRoot := firstString(i.request.OperationID, firstString(i.request.ActionID, i.request.SourceFactID))
	if operationRoot == "" {
		return "", errors.New("adk_tool_operation_root_required")
	}
	operationID := "agent_tool_" + stableDigest(fmt.Sprintf("%s\x1f%d\x1f%d\x1f%s", operationRoot, modelIdentity.ModelCallSequence, modelIdentity.ToolIndex, capabilityName))
	workingProfileID := workingProfileForToolExecution(projection, i.trace)
	var expectedPersonaRevision *int
	var expectedOverlayRevision *int
	if capabilityName == personaDetailCapabilityName {
		revision, overlayRevision := personaDetailExpectedRevisions(projection, i.trace, workingProfileID)
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
		invocation.ContextSnapshot = capabilitySnapshotForProjection(projection, definition.RequiredContext, i.request.ActionID)
	}
	i.trace.AppendInvocation(invocation)
	if rejection := kevRejectNativeTool(ctx, capabilityName, arguments); rejection != "" {
		result := failedCapabilityResultDetail(invocation, rejection, false, "The capability was not admitted for this request")
		i.trace.AppendResult(result)
		return modelFacingToolResultForContext(ctx, ToolExecutionReceipt{Result: result}, definition)
	}
	if capabilityName == conversationReplyCapabilityName {
		if adkContext, ok := adkCapabilityContext(ctx); ok && adkContext.Refresh.replyNeedsFreshSchedule(modelIdentity.ModelCallSequence, callID) {
			result := CapabilityResult{CallID: callID, CapabilityName: capabilityName, Status: "rejected",
				ErrorCode: "schedule_context_refresh_required", Output: map[string]any{"detail": "Wait for the Schedule Tool result and refreshed context, then compose the reply in the next model turn."},
				ProviderRequestID: modelIdentity.ProviderRequestID, CorrelationID: "capability:" + callID}
			i.trace.AppendResult(result)
			i.recordADKToolDiagnostic(ctx, "adk.tool.rejected", callID, capabilityName, result.Status, result.ErrorCode, argumentsJSON)
			return modelFacingToolResultForContext(ctx, ToolExecutionReceipt{Result: result}, definition)
		}
	}
	i.recordADKToolDiagnostic(ctx, "adk.tool.dispatched", callID, capabilityName, "dispatched", "", argumentsJSON)
	if gate := kevPersona(ctx); gate != nil && capabilityName == personaSwitchCapabilityName {
		if revision, adopted := gate.adoptedRevision(ctx); adopted {
			ctx = context.WithValue(ctx, kevPersonaExpectedRevisionKey{}, revision)
		}
	}

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
		FrozenContextSnapshot:       invocation.ContextSnapshot,
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
	if gate := kevPersona(ctx); gate != nil && capabilityName == personaSwitchCapabilityName {
		gate.nativeResult(ctx, result)
	}
	if gate := kevPersona(ctx); gate != nil && capabilityName == personaSwitchCapabilityName && result.Status == "completed" && stringValue(mapValue(result.Output)["disposition"]) == "applied" {
		gate.mu.Lock()
		gate.switched = true
		gate.mu.Unlock()
	}
	if errors.Is(execErr, ErrLifeContextStale) || (execErr == nil && receipt.AuthorityRevisions.Before != nil && (result.Status == "completed" || result.Status == "accepted")) {
		if adkContext, ok := adkCapabilityContext(ctx); ok {
			if capabilityName == conversationReplyCapabilityName && errors.Is(execErr, ErrLifeContextStale) {
				adkContext.Refresh.markStaleReply(callID)
			} else {
				adkContext.Refresh.markDirty()
			}
		}
	}
	i.recordADKToolDiagnostic(ctx, "adk.tool.result", callID, capabilityName, result.Status, result.ErrorCode, argumentsJSON)
	serialized, encodeErr := modelFacingToolResultForContext(ctx, receipt, definition)
	if encodeErr != nil {
		return "", encodeErr
	}
	if execErr != nil && result.Retryable {
		// Runtime/dependency failures terminate this run while the trace retains
		// the invocation and receipt. A non-retryable business rejection is a
		// normal tool result and is returned to the model for another decision.
		return serialized, &agentRunFailure{stage: "tool", code: result.ErrorCode, cause: fmt.Errorf("tool execution %s: %w", result.ErrorCode, execErr)}
	}
	return serialized, nil
}

func (i *appADKCapabilityInvoker) recordMissingCapabilityRequest(ctx context.Context, callID, capabilityName string, projection ContextProjection) {
	if i == nil || i.app == nil || i.trace == nil || !toolNamePattern.MatchString(capabilityName) || strings.TrimSpace(callID) == "" || strings.TrimSpace(i.request.SourceFactID) == "" {
		return
	}
	identity, ok := i.trace.ModelIdentity(callID)
	if !ok || strings.TrimSpace(identity.ProviderRequestID) == "" {
		return
	}
	operationRoot := firstString(i.request.OperationID, firstString(i.request.ActionID, i.request.SourceFactID))
	if operationRoot == "" {
		return
	}
	actorID := firstString(i.request.AuthorizationActorID, projection.OwnerActorID)
	if actorID == "" || i.request.FluctlightID == "" {
		return
	}
	arguments := jsonBytes(map[string]any{
		"capability_key":    capabilityName,
		"title":             capabilityName,
		"description":       "The Agent requested this unavailable tool during an authorized run.",
		"rationale":         "The requested action could not be executed because the capability is not installed.",
		"desired_contract":  map[string]any{"requested_tool": capabilityName},
		"side_effect_class": "unknown",
	})
	agentDefinition, _ := formalAgentDefinitionFromContext(ctx)
	_, _ = i.app.ExecuteTool(ctx, ToolExecutionRequest{
		AgentID: agentDefinition.ID, RunID: operationRoot,
		CapabilityName: "capability.request", OperationID: "missing_capability_" + stableDigest(operationRoot+"\x1f"+callID+"\x1f"+capabilityName),
		ProviderRequestID:    identity.ProviderRequestID,
		AuthorizationActorID: actorID, FluctlightID: i.request.FluctlightID,
		SubjectActorID: firstString(i.request.SubjectActorID, actorID), ConversationID: i.request.ConversationID,
		EvidenceID: i.request.SourceFactID, Surface: i.request.Surface, Arguments: arguments,
		AuthorizationPolicy: i.request.AuthorizationPolicy,
		TargetKind:          i.request.TargetKind, TargetRef: i.request.TargetRef,
	})
}

func modelFacingToolResultForContext(ctx context.Context, receipt ToolExecutionReceipt, definition CapabilityDefinition) (string, error) {
	visible := modelFacingToolResult(receipt, definition)
	if adkContext, ok := adkCapabilityContext(ctx); ok && adkContext.Refs != nil {
		encoded, err := adkContext.Refs.encodeResult(visible)
		if err != nil {
			return "", err
		}
		visible = encoded.(map[string]any)
	}
	return jsonString(visible), nil
}

// The native ToolCall ID and the Core receipt remain in the execution trace.
// The model only needs the business result to decide what to do next.
func modelFacingToolResult(receipt ToolExecutionReceipt, definition CapabilityDefinition) map[string]any {
	result := receipt.Result
	visible := map[string]any{"status": result.Status}
	if result.ErrorCode != "" {
		visible["error_code"] = result.ErrorCode
	}
	if result.Retryable {
		visible["retryable"] = true
	}
	if result.Output != nil {
		if outputMap, ok := result.Output.(map[string]any); ok {
			output := cloneMap(outputMap)
			for _, field := range definition.ModelResultOmitFields {
				omitModelResultPath(output, strings.Split(field, "."))
			}
			if definition.Name == wardrobeInspectCapabilityName && stringValue(output["operation"]) == "list" {
				boundModelWardrobeList(output)
			}
			if len(output) > 0 {
				visible["output"] = output
			}
		} else {
			visible["output"] = result.Output
		}
	}
	return visible
}

// The canonical receipt remains complete. A list can be continued with the
// same authorized Tool and the real ID of the final item exposed here.
func boundModelWardrobeList(output map[string]any) {
	const maxItems = 12
	const maxDescriptionRunes = 160
	items := arrayValue(output["items"])
	if len(items) > maxItems {
		items = items[:maxItems]
		output["items"] = items
		output["has_more"] = true
		output["next_cursor"] = stringValue(mapValue(items[len(items)-1])["id"])
		output["can_conclude_absent"] = false
	}
	for _, raw := range items {
		item := mapValue(raw)
		description := []rune(stringValue(item["description"]))
		if len(description) > maxDescriptionRunes {
			item["description"] = string(description[:maxDescriptionRunes])
			item["description_truncated"] = true
		}
	}
}

func omitModelResultPath(value any, path []string) {
	if len(path) == 0 {
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		if len(path) == 1 {
			delete(typed, path[0])
			return
		}
		omitModelResultPath(typed[path[0]], path[1:])
	case []any:
		for _, item := range typed {
			omitModelResultPath(item, path)
		}
	}
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
	modelCallID := stringValue(diagnostics["model_call_id"])
	if i.trace != nil && strings.TrimSpace(callID) != "" {
		if identity, found := i.trace.ModelIdentity(strings.TrimSpace(callID)); found && strings.TrimSpace(identity.ProviderRequestID) != "" {
			modelCallID = identity.ProviderRequestID
		}
	}
	newProviderRuntimeSupport(i.app.DB).RecordDiagnosticEvent(ctx, eventType, statusSeverity(status), i.request.FluctlightID, i.request.SourceFactID, correlationID, map[string]any{
		"run_id": runID, "stage": "tool", "surface": i.request.Surface,
		"model_call_id": modelCallID,
		"call_id":       strings.TrimSpace(callID), "capability": strings.TrimSpace(capabilityName),
		"status": strings.TrimSpace(status), "error_code": strings.TrimSpace(errorCode),
		"arguments": arguments, "arguments_digest": stableDigest(strings.TrimSpace(arguments)),
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
	Projection *ContextProjection `json:"projection,omitempty"`
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
	var selectionErr error
	ctx, input.Definitions, selectionErr = a.prepareKevTools(ctx, input)
	if selectionErr != nil {
		return ADKStructuredTaskResult{}, selectionErr
	}
	trace := &ADKCapabilityTrace{}
	request := ADKCapabilityRequest{Surface: definition.DefaultSurface}
	if input.Capability != nil {
		request = *input.Capability
	}
	invoker := newAppADKCapabilityInvoker(a, request, trace)
	var refs *providerContextRefCodec
	if input.Capability != nil {
		var refErr error
		refs, refErr = newProviderContextRefCodec(request.Projection.ReferenceIndex)
		if refErr != nil {
			return ADKStructuredTaskResult{Trace: trace}, refErr
		}
	}
	var refresh *runtimeContextRefresh
	if plan := runtimeContextRefreshPlan(ctx); plan != nil {
		refresh = &runtimeContextRefresh{refresh: plan, base: request.Projection, kevVersion: request.Projection.KevContextVersion}
	}
	ctx = withADKCapabilityContextRefs(ctx, invoker, trace, refresh, refs)
	if input.Capability != nil && refresh != nil && kevPersonaInstalled(input.Definitions) && a.kevService().Enabled(ctx, "persona.switch") {
		ctx = context.WithValue(ctx, kevPersonaKey{}, &kevPersonaAdmission{app: a, request: request, denied: map[string]bool{}})
	}
	if strings.TrimSpace(input.Scenario) != "" {
		ctx = WithProviderScenario(ctx, input.Scenario)
	}
	if !validAssembledProviderMessages(input.Prompt.Messages) {
		return ADKStructuredTaskResult{Trace: trace}, errors.New("adk_structured_task_prompt_invalid")
	}
	ctx, err := projectFormalPromptBoundary(ctx, input)
	if err != nil {
		return ADKStructuredTaskResult{Trace: trace}, err
	}
	record, prior, admissionErr := a.admitFormalRun(ctx, definition, input)
	if admissionErr != nil || prior != nil {
		if prior != nil {
			return *prior, admissionErr
		}
		return ADKStructuredTaskResult{Trace: trace}, admissionErr
	}
	completion, err := a.Provider.AgentAssembledCompletion(ctx, input.Role, input.Prompt.Messages, input.Definitions, input.SchemaName, input.Prompt.ResponseFormat, !input.TextOutput, input.EnableThinking)
	result := ADKStructuredTaskResult{Completion: completion, Trace: trace, Projection: refresh.latestProjection()}
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
