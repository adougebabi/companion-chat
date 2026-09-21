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
	FluctlightID   string
	ConversationID string
	SourceFactID   string
	ActionID       string
	CorrelationID  string
	Surface        CapabilitySurface
	Projection     ContextProjection
}

func newAppADKCapabilityInvoker(app *App, request ADKCapabilityRequest, trace *ADKCapabilityTrace) ADKCapabilityInvoker {
	if request.Surface == "" {
		request.Surface = CapabilitySurfaceConversation
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
	// ADK may replay the same native call while the model is failing to emit a
	// terminal assistant message.  A formal call ID is the idempotency boundary:
	// identical replays return the original bounded result and never create a
	// second invocation/result row; a changed payload under the same ID fails
	// closed instead of silently merging two authorities.
	if normalizedArguments, normalizeErr := normalizeToolArguments(arguments); normalizeErr == nil {
		for _, previous := range i.trace.Invocations {
			if previous.CallID != callID {
				continue
			}
			previousArguments, previousErr := normalizeToolArguments(previous.Arguments)
			if previous.CapabilityName != capabilityName || previousErr != nil || string(previousArguments) != string(normalizedArguments) {
				i.recordADKToolDiagnostic(ctx, "adk.tool.rejected", callID, capabilityName, "rejected", "adk_tool_call_id_reused", argumentsJSON)
				return "", errors.New("adk_tool_call_id_reused")
			}
			for _, previousResult := range i.trace.Results {
				if previousResult.CallID != callID {
					continue
				}
				return jsonString(map[string]any{
					"status": previousResult.Status, "capability": previousResult.CapabilityName,
					"error_code": previousResult.ErrorCode, "output": previousResult.Output,
				}), nil
			}
			i.recordADKToolDiagnostic(ctx, "adk.tool.dispatched", callID, capabilityName, "deferred", "", argumentsJSON)
			return jsonString(map[string]any{"status": "deferred", "reason": "settlement_pending"}), nil
		}
	}
	invocation := normalizeCapabilityInvocationMetadata(CapabilityInvocation{
		CallID: callID, CapabilityName: capabilityName, Arguments: arguments,
		SourceFactID: i.request.SourceFactID, ActionID: i.request.ActionID,
		ProviderRequestID: "provider:" + stableDigest(i.request.SourceFactID+":"+callID), Sequence: len(i.trace.Invocations),
		Metadata: InvocationMetadata{CorrelationID: firstString(providerCorrelation(ctx), firstString(i.request.CorrelationID, "turn:"+i.request.SourceFactID)), FluctlightID: i.request.FluctlightID, ConversationID: i.request.ConversationID, Surface: i.request.Surface, Source: "model_tool"},
	}, i.request.FluctlightID, i.request.ConversationID, i.request.SourceFactID, i.request.SourceFactID, len(i.trace.Invocations))
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
	i.trace.Invocations = append(i.trace.Invocations, invocation)
	i.recordADKToolDiagnostic(ctx, "adk.tool.dispatched", callID, capabilityName, "dispatched", "", argumentsJSON)
	result := CapabilityResult{CallID: callID, CapabilityName: capabilityName, Status: "deferred", Retryable: true, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "capability:" + callID, RequiredContext: append([]ContextSlot(nil), definition.RequiredContext...), Output: map[string]any{"status": "deferred", "reason": "settlement_pending"}}
	capability, found := i.app.capabilityRegistry().LookupCapability(capabilityName)
	if !found {
		result.Status = "rejected"
		result.Retryable = false
		result.ErrorCode = "capability_not_found"
		i.trace.Results = append(i.trace.Results, result)
		i.recordADKToolDiagnostic(ctx, "adk.tool.rejected", callID, capabilityName, result.Status, result.ErrorCode, argumentsJSON)
		return jsonString(result.Output), errors.New("capability_not_found")
	}
	executionClass, classErr := classifyCapabilityExecution(capability, definition)
	if classErr != nil {
		result.Status = "rejected"
		result.Retryable = false
		result.ErrorCode = "capability_execution_class_invalid"
		i.trace.Results = append(i.trace.Results, result)
		i.recordADKToolDiagnostic(ctx, "adk.tool.rejected", callID, capabilityName, result.Status, result.ErrorCode, argumentsJSON)
		return jsonString(result.Output), classErr
	}
	// Validate every model-proposed invocation before ADK can complete the
	// round. The post-freeze gate remains authoritative, but validating here
	// also covers a native tool call that ADK emits in an intermediate round and
	// whose call is intentionally excluded from the final visible-output list.
	// Without this early gate, an invalid conversation.reply could disappear
	// during output filtering and let a valid root sidecar reach the Judge.
	if err := validateCandidateCapabilityInvocation(ctx, invocation, candidateValidationContext{
		FluctlightID: i.request.FluctlightID, ConversationID: i.request.ConversationID, SourceFactID: i.request.SourceFactID,
		ActionID: i.request.ActionID, Surface: i.request.Surface, ContextSnapshot: invocation.ContextSnapshot, Context: ctx,
	}, i.app.capabilityRegistry()); err != nil {
		result.Status = "rejected"
		result.ErrorCode = "candidate_invalid"
		result.Retryable = false
		result.Output = map[string]any{"status": "rejected", "error_code": result.ErrorCode}
		i.trace.Results = append(i.trace.Results, result)
		i.recordADKToolDiagnostic(ctx, "adk.tool.rejected", callID, capabilityName, result.Status, result.ErrorCode, argumentsJSON)
		// Return a bounded tool result without surfacing a Go error to ADK. The
		// agent must be allowed to finish its current physical round so Core can
		// persist the frozen candidate and reject it before Judge/Prepare/Execute.
		return jsonString(result.Output), nil
	}
	if executionClass == CapabilityExecutionPureQuery {
		runtime := i.app.capabilityRuntime()
		if runtime == nil {
			result.Status = "failed"
			result.Retryable = true
			result.ErrorCode = "capability_runtime_unavailable"
			i.trace.Results = append(i.trace.Results, result)
			i.recordADKToolDiagnostic(ctx, "adk.tool.result", callID, capabilityName, result.Status, result.ErrorCode, argumentsJSON)
			return jsonString(map[string]any{"status": result.Status, "error_code": result.ErrorCode}), errors.New(result.ErrorCode)
		}
		executed, execErr := runtime.Execute(ctx, invocation)
		result = executed
		if execErr != nil {
			if result.Status == "" {
				result.Status = "failed"
			}
			i.trace.Results = append(i.trace.Results, result)
			i.recordADKToolDiagnostic(ctx, "adk.tool.result", callID, capabilityName, result.Status, result.ErrorCode, argumentsJSON)
			return jsonString(map[string]any{"status": result.Status, "error_code": result.ErrorCode}), execErr
		}
	}
	i.trace.Results = append(i.trace.Results, result)
	i.recordADKToolDiagnostic(ctx, "adk.tool.result", callID, capabilityName, result.Status, result.ErrorCode, argumentsJSON)
	return jsonString(map[string]any{"status": result.Status, "capability": capabilityName, "output": result.Output}), nil
}

func (i *appADKCapabilityInvoker) recordADKToolDiagnostic(ctx context.Context, eventType, callID, capabilityName, status, errorCode, arguments string) {
	if i == nil || i.app == nil || i.app.DB == nil || i.app.DB.Pool() == nil {
		return
	}
	correlationID := firstString(providerCorrelation(ctx), firstString(i.request.CorrelationID, "turn:"+i.request.SourceFactID))
	newProviderRuntimeSupport(i.app.DB).RecordDiagnosticEvent(ctx, eventType, statusSeverity(status), i.request.FluctlightID, i.request.SourceFactID, correlationID, map[string]any{
		"run_id": correlationID, "stage": "tool", "surface": i.request.Surface,
		"call_id": strings.TrimSpace(callID), "capability": strings.TrimSpace(capabilityName),
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

// isADKLoopSchema is the explicit allowlist for bounded ADK model/tool loops.
func isADKLoopSchema(schemaName string) bool {
	switch strings.TrimSpace(schemaName) {
	case "conversation_turn_response", takeoverReplySchemaName, "wake_up_response":
		return true
	default:
		return false
	}
}

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
	Role           string
	Scenario       string
	Prompt         PromptAssemblyResult
	Definitions    []CapabilityDefinition
	SchemaName     string
	EnableThinking bool
	Capability     *ADKCapabilityRequest
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
	if !isADKLoopSchema(input.SchemaName) {
		return ADKStructuredTaskResult{}, fmt.Errorf("adk_schema_not_allowed: %s", strings.TrimSpace(input.SchemaName))
	}
	if len(input.Definitions) > 0 && input.Capability == nil {
		return ADKStructuredTaskResult{}, errors.New("adk_structured_task_capability_context_required")
	}
	if err := validateADKCapabilityDefinitions(a, input.Definitions, firstCapabilitySurface(input.Capability)); err != nil {
		return ADKStructuredTaskResult{}, err
	}
	trace := &ADKCapabilityTrace{}
	if input.Capability != nil {
		invoker := newAppADKCapabilityInvoker(a, *input.Capability, trace)
		ctx = WithADKCapabilityInvoker(ctx, invoker, trace)
	}
	if strings.TrimSpace(input.Scenario) != "" {
		ctx = WithProviderScenario(ctx, input.Scenario)
	}
	if !validAssembledProviderMessages(input.Prompt.Messages) {
		return ADKStructuredTaskResult{Trace: trace}, errors.New("adk_structured_task_prompt_invalid")
	}
	completion, err := a.Provider.StructuredAssembledWithToolsSchema(ctx, input.Role, input.Prompt.Messages, input.Definitions, input.SchemaName, input.Prompt.ResponseFormat, input.EnableThinking)
	if err != nil {
		return ADKStructuredTaskResult{Trace: trace}, err
	}
	return ADKStructuredTaskResult{Completion: completion, Trace: trace}, nil
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
		if !canonical.SupportsSurface(surface) {
			return fmt.Errorf("adk_capability_definition_surface_forbidden: %s:%s", definition.Name, surface)
		}
		if !reflect.DeepEqual(canonical, definition) {
			return fmt.Errorf("adk_capability_definition_mismatch: %s", definition.Name)
		}
	}
	return nil
}
