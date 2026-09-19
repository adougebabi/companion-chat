package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type adkConversationContextKey struct{}

// ADKCapabilityTrace is request-scoped state used to carry the canonical
// invocation/result pair from an ADK tool callback into Core's existing frozen
// settlement path. It is never persisted as an alternative envelope.
type ADKCapabilityTrace struct {
	Invocations []CapabilityInvocation
	Results     []CapabilityResult
}

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
	definition, ok := i.app.capabilityRegistry().Definition(capabilityName)
	if !ok {
		return "", fmt.Errorf("capability_not_found: %s", capabilityName)
	}
	arguments := json.RawMessage(strings.TrimSpace(argumentsJSON))
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return "", errors.New("adk_tool_call_id_required")
	}
	invocation := normalizeCapabilityInvocationMetadata(CapabilityInvocation{
		CallID: callID, CapabilityName: capabilityName, Arguments: arguments,
		SourceFactID: i.request.SourceFactID, ActionID: i.request.ActionID,
		ProviderRequestID: "provider:" + stableDigest(i.request.SourceFactID+":"+callID), Sequence: len(i.trace.Invocations),
		Metadata: InvocationMetadata{CorrelationID: firstString(providerCorrelation(ctx), firstString(i.request.CorrelationID, "turn:"+i.request.SourceFactID)), FluctlightID: i.request.FluctlightID, ConversationID: i.request.ConversationID, Surface: i.request.Surface, Source: "model_tool"},
	}, i.request.FluctlightID, i.request.ConversationID, i.request.SourceFactID, i.request.SourceFactID, len(i.trace.Invocations))
	invocation.ActionID = i.request.ActionID
	invocation.ContextSnapshot = capabilitySnapshotForProjection(i.request.Projection, definition.RequiredContext, i.request.ActionID)
	i.trace.Invocations = append(i.trace.Invocations, invocation)
	result := CapabilityResult{CallID: callID, CapabilityName: capabilityName, Status: "deferred", Retryable: true, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "capability:" + callID, RequiredContext: append([]ContextSlot(nil), definition.RequiredContext...), Output: map[string]any{"status": "deferred", "reason": "settlement_pending"}}
	capability, found := i.app.capabilityRegistry().LookupCapability(capabilityName)
	if !found {
		result.Status = "rejected"
		result.Retryable = false
		result.ErrorCode = "capability_not_found"
		i.trace.Results = append(i.trace.Results, result)
		return jsonString(result.Output), errors.New("capability_not_found")
	}
	executionClass, classErr := classifyCapabilityExecution(capability, definition)
	if classErr != nil {
		result.Status = "rejected"
		result.Retryable = false
		result.ErrorCode = "capability_execution_class_invalid"
		i.trace.Results = append(i.trace.Results, result)
		return jsonString(result.Output), classErr
	}
	if executionClass == CapabilityExecutionPureQuery {
		if err := validateCandidateCapabilityInvocation(ctx, invocation, candidateValidationContext{
			FluctlightID: i.request.FluctlightID, ConversationID: i.request.ConversationID, SourceFactID: i.request.SourceFactID,
			ActionID: i.request.ActionID, Surface: i.request.Surface, ContextSnapshot: invocation.ContextSnapshot, Context: ctx,
		}, i.app.capabilityRegistry()); err != nil {
			result.Status = "rejected"
			result.ErrorCode = "candidate_invalid"
			result.Retryable = false
			result.Output = map[string]any{"status": "rejected"}
			i.trace.Results = append(i.trace.Results, result)
			return jsonString(result.Output), err
		}
		runtime := i.app.capabilityRuntime()
		if runtime == nil {
			result.Status = "failed"
			result.Retryable = true
			result.ErrorCode = "capability_runtime_unavailable"
			i.trace.Results = append(i.trace.Results, result)
			return jsonString(map[string]any{"status": result.Status, "error_code": result.ErrorCode}), errors.New(result.ErrorCode)
		}
		executed, execErr := runtime.Execute(ctx, invocation)
		result = executed
		if execErr != nil {
			if result.Status == "" {
				result.Status = "failed"
			}
			i.trace.Results = append(i.trace.Results, result)
			return jsonString(map[string]any{"status": result.Status, "error_code": result.ErrorCode}), execErr
		}
	}
	i.trace.Results = append(i.trace.Results, result)
	return jsonString(map[string]any{"status": result.Status, "capability": capabilityName, "output": result.Output}), nil
}

// ADKCapabilityInvoker is the only dependency an ADK tool adapter receives.
// The caller binds a request-scoped CapabilityRuntime closure that owns
// authorization, context snapshots, idempotency and settlement; the adapter
// never receives *App, a repository or a transaction.
type ADKCapabilityInvoker interface {
	Execute(ctx context.Context, capabilityName string, argumentsJSON string) (string, error)
}

// ADKCapabilityInvokerWithID is the identity-preserving variant used by the
// production bridge. Eino exposes the model's formal tool-call ID in the tool
// context; callers must persist that ID rather than deriving a replacement.
type ADKCapabilityInvokerWithID interface {
	ExecuteWithID(ctx context.Context, callID, capabilityName string, argumentsJSON string) (string, error)
}

type ADKLoopConfig struct {
	Name            string
	Description     string
	Instruction     string
	Model           model.ToolCallingChatModel
	Tools           []tool.BaseTool
	MaxIterations   int
	EnableStreaming bool
}

type ADKLoopResult struct {
	FinalMessage *schema.Message
	Messages     []*schema.Message
	ToolCalls    []schema.ToolCall
	ToolResults  []*schema.Message
	Iterations   int
}

// isADKLoopSchema is the explicit allowlist for bounded ADK model/tool loops.
// Query continuation, Judge, Daily Review, Native Cognition and Reflection
// remain dedicated single-task boundaries unless a later phase adds a real
// feedback contract for them.
func isADKLoopSchema(schemaName string) bool {
	switch strings.TrimSpace(schemaName) {
	case "conversation_turn_response", takeoverReplySchemaName, "wake_up_response":
		return true
	default:
		return false
	}
}

// NewADKCapabilityTools exposes only the installed capability schemas and a
// request-scoped executor. It is deliberately an adapter, not a second tool
// protocol or execution runtime.
func NewADKCapabilityTools(definitions []CapabilityDefinition, invoker ADKCapabilityInvoker) ([]tool.BaseTool, error) {
	if invoker == nil {
		return nil, errors.New("adk_capability_invoker_required")
	}
	if len(definitions) > 0 {
		if _, ok := invoker.(ADKCapabilityInvokerWithID); !ok {
			// A model tool call is not executable without its native identity. Fail
			// while constructing the adapter, before a model can emit a call that
			// would otherwise fail after the provider round-trip.
			return nil, errors.New("adk_tool_call_identity_invoker_required")
		}
	}
	infos, err := capabilityToolInfos(definitions)
	if err != nil {
		return nil, err
	}
	tools := make([]tool.BaseTool, 0, len(infos))
	for _, info := range infos {
		tools = append(tools, &adkCapabilityTool{info: info, invoker: invoker})
	}
	return tools, nil
}

type adkCapabilityTool struct {
	info    *schema.ToolInfo
	invoker ADKCapabilityInvoker
}

func (t *adkCapabilityTool) Info(context.Context) (*schema.ToolInfo, error) {
	if t == nil || t.info == nil {
		return nil, errors.New("adk_capability_tool_unavailable")
	}
	return t.info, nil
}

func (t *adkCapabilityTool) InvokableRun(ctx context.Context, argumentsJSON string, _ ...tool.Option) (string, error) {
	if t == nil || t.invoker == nil || t.info == nil {
		return "", errors.New("adk_capability_tool_unavailable")
	}
	argumentsJSON = strings.TrimSpace(argumentsJSON)
	if argumentsJSON == "" {
		argumentsJSON = "{}"
	}
	callID := strings.TrimSpace(compose.GetToolCallID(ctx))
	if callID == "" {
		return "", errors.New("adk_tool_call_id_missing")
	}
	identityInvoker, ok := t.invoker.(ADKCapabilityInvokerWithID)
	if !ok {
		return "", errors.New("adk_tool_call_identity_invoker_required")
	}
	return identityInvoker.ExecuteWithID(ctx, callID, t.info.Name, argumentsJSON)
}

// RunADKLoop is the only Eino ADK model/tool event iterator used by Core.
// Callers receive bounded messages and hand the structured result to their
// existing domain freeze/settlement boundary.
func RunADKLoop(ctx context.Context, config ADKLoopConfig, messages []*schema.Message) (ADKLoopResult, error) {
	if config.Model == nil {
		return ADKLoopResult{}, errors.New("adk_model_required")
	}
	if strings.TrimSpace(config.Name) == "" || strings.TrimSpace(config.Description) == "" {
		return ADKLoopResult{}, errors.New("adk_agent_identity_required")
	}
	if len(messages) == 0 {
		return ADKLoopResult{}, errors.New("adk_messages_required")
	}
	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 2
	}
	if maxIterations > 2 {
		return ADKLoopResult{}, errors.New("adk_iteration_limit_invalid")
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: config.Name, Description: config.Description,
		Instruction: config.Instruction, Model: config.Model,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: config.Tools, ExecuteSequentially: true,
		}},
		MaxIterations: maxIterations,
	})
	if err != nil {
		return ADKLoopResult{}, fmt.Errorf("adk_agent_create: %w", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: config.EnableStreaming})
	input := make([]adk.Message, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			return ADKLoopResult{}, errors.New("adk_message_nil")
		}
		input = append(input, message)
	}
	iter := runner.Run(ctx, input)
	result := ADKLoopResult{Messages: []*schema.Message{}, ToolCalls: []schema.ToolCall{}, ToolResults: []*schema.Message{}}
	var lastAssistant *schema.Message
	seenToolCallIDs := make(map[string]struct{})
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			return ADKLoopResult{}, fmt.Errorf("adk_run: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		message, getErr := event.Output.MessageOutput.GetMessage()
		if getErr != nil {
			return ADKLoopResult{}, fmt.Errorf("adk_message_output: %w", getErr)
		}
		if message == nil {
			continue
		}
		copyMessage := message
		result.Messages = append(result.Messages, copyMessage)
		if len(copyMessage.ToolCalls) > 0 {
			for _, call := range copyMessage.ToolCalls {
				if strings.TrimSpace(call.ID) != "" {
					if _, seen := seenToolCallIDs[call.ID]; seen {
						continue
					}
					seenToolCallIDs[call.ID] = struct{}{}
				}
				result.ToolCalls = append(result.ToolCalls, call)
			}
		}
		if copyMessage.Role == schema.Tool {
			result.ToolResults = append(result.ToolResults, copyMessage)
		}
		if copyMessage.Role == schema.Assistant {
			lastAssistant = copyMessage
			if strings.TrimSpace(copyMessage.Content) != "" {
				result.FinalMessage = copyMessage
			}
		}
		result.Iterations++
	}
	if result.FinalMessage == nil {
		result.FinalMessage = lastAssistant
	}
	if result.FinalMessage == nil {
		return ADKLoopResult{}, errors.New("adk_final_message_missing")
	}
	if strings.TrimSpace(result.FinalMessage.Content) == "" && len(result.FinalMessage.ToolCalls) == 0 {
		return ADKLoopResult{}, errors.New("adk_final_message_empty")
	}
	return result, nil
}

// ADKStructuredTaskInput is the shared Provider-facing background/conversation
// boundary. Domain callers keep their own result and settlement contracts.
type ADKStructuredTaskInput struct {
	Role           string
	Scenario       string
	Messages       []map[string]any
	Definitions    []CapabilityDefinition
	SchemaName     string
	Schema         map[string]any
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
	completion, err := a.Provider.StructuredAssembledWithToolsSchema(ctx, input.Role, input.Messages, input.Definitions, input.SchemaName, input.Schema, input.EnableThinking)
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
