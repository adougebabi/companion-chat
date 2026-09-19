package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	app            *App
	fluctlightID   string
	conversationID string
	sourceFactID   string
	actionID       string
	projection     ContextProjection
	trace          *ADKCapabilityTrace
}

func newAppADKCapabilityInvoker(app *App, fluctlightID, conversationID, sourceFactID, actionID string, projection ContextProjection, trace *ADKCapabilityTrace) ADKCapabilityInvoker {
	return &appADKCapabilityInvoker{app: app, fluctlightID: fluctlightID, conversationID: conversationID, sourceFactID: sourceFactID, actionID: actionID, projection: projection, trace: trace}
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
		SourceFactID: i.sourceFactID, ActionID: i.actionID,
		ProviderRequestID: "provider:" + stableDigest(i.sourceFactID+":"+callID), Sequence: len(i.trace.Invocations),
		Metadata: InvocationMetadata{CorrelationID: firstString(providerCorrelation(ctx), "turn:"+i.sourceFactID), FluctlightID: i.fluctlightID, ConversationID: i.conversationID, Surface: CapabilitySurfaceConversation, Source: "model_tool"},
	}, i.fluctlightID, i.conversationID, i.sourceFactID, i.sourceFactID, len(i.trace.Invocations))
	invocation.ActionID = i.actionID
	invocation.ContextSnapshot = capabilitySnapshotForProjection(i.projection, definition.RequiredContext, i.actionID)
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
			FluctlightID: i.fluctlightID, ConversationID: i.conversationID, SourceFactID: i.sourceFactID,
			ActionID: i.actionID, Surface: CapabilitySurfaceConversation, ContextSnapshot: invocation.ContextSnapshot, Context: ctx,
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

type ADKConversationConfig struct {
	Name            string
	Description     string
	Instruction     string
	Model           model.ToolCallingChatModel
	Tools           []tool.BaseTool
	MaxIterations   int
	EnableStreaming bool
}

type ADKConversationResult struct {
	FinalMessage *schema.Message
	Messages     []*schema.Message
	ToolCalls    []schema.ToolCall
	ToolResults  []*schema.Message
	Iterations   int
}

// isADKConversationSchema keeps the ADK loop restricted to the two
// user-visible conversation generations. Query continuation and the takeover
// Judge remain dedicated no-tools model tasks, while both Main and takeover B
// use the same request-scoped model→tool→result→model boundary.
func isADKConversationSchema(schemaName string) bool {
	switch strings.TrimSpace(schemaName) {
	case "conversation_turn_response", takeoverReplySchemaName:
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

// RunADKConversation is the production ADK entry point for a direct
// conversation. Eino owns the model→tool→tool-result→model loop; the caller
// only receives bounded messages and then hands them to the existing frozen
// decision/settlement path.
func RunADKConversation(ctx context.Context, config ADKConversationConfig, messages []*schema.Message) (ADKConversationResult, error) {
	if config.Model == nil {
		return ADKConversationResult{}, errors.New("adk_model_required")
	}
	if strings.TrimSpace(config.Name) == "" || strings.TrimSpace(config.Description) == "" {
		return ADKConversationResult{}, errors.New("adk_agent_identity_required")
	}
	if len(messages) == 0 {
		return ADKConversationResult{}, errors.New("adk_messages_required")
	}
	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 2
	}
	if maxIterations > 2 {
		return ADKConversationResult{}, errors.New("adk_iteration_limit_invalid")
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
		return ADKConversationResult{}, fmt.Errorf("adk_agent_create: %w", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: config.EnableStreaming})
	input := make([]adk.Message, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			return ADKConversationResult{}, errors.New("adk_message_nil")
		}
		input = append(input, message)
	}
	iter := runner.Run(ctx, input)
	result := ADKConversationResult{Messages: []*schema.Message{}, ToolCalls: []schema.ToolCall{}, ToolResults: []*schema.Message{}}
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
			return ADKConversationResult{}, fmt.Errorf("adk_run: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		message, getErr := event.Output.MessageOutput.GetMessage()
		if getErr != nil {
			return ADKConversationResult{}, fmt.Errorf("adk_message_output: %w", getErr)
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
		return ADKConversationResult{}, errors.New("adk_final_message_missing")
	}
	if strings.TrimSpace(result.FinalMessage.Content) == "" && len(result.FinalMessage.ToolCalls) == 0 {
		return ADKConversationResult{}, errors.New("adk_final_message_empty")
	}
	return result, nil
}
