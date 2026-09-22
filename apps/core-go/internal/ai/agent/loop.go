package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

// ADKCapabilityTrace is request-scoped state used to carry the canonical
// invocation/result pair from an ADK tool callback into Core's existing frozen
// settlement path. It is never persisted as an alternative envelope.
type ADKCapabilityTrace struct {
	mu          sync.RWMutex
	Invocations []capability.CapabilityInvocation
	Results     []capability.CapabilityResult
	ModelCalls  map[string]ADKToolCallModelIdentity
}

// ADKToolCallModelIdentity binds a native ToolCall to the physical Provider
// request that emitted it. ProviderRequestID is transport provenance;
// ModelCallSequence/ToolIndex are stable run coordinates used to derive a
// business operation identity independently of the model's call ID.
type ADKToolCallModelIdentity struct {
	ProviderRequestID string
	ModelCallSequence uint64
	ToolIndex         int
}

func (trace *ADKCapabilityTrace) RecordModelToolCalls(providerRequestID string, sequence uint64, calls []schema.ToolCall) {
	if trace == nil {
		return
	}
	trace.mu.Lock()
	defer trace.mu.Unlock()
	if trace.ModelCalls == nil {
		trace.ModelCalls = make(map[string]ADKToolCallModelIdentity)
	}
	for index, call := range calls {
		callID := strings.TrimSpace(call.ID)
		if callID == "" {
			continue
		}
		// Repeated delivery of a native call reuses its original execution
		// receipt. Never rewrite that receipt's physical model provenance.
		if _, exists := trace.ModelCalls[callID]; exists {
			continue
		}
		trace.ModelCalls[callID] = ADKToolCallModelIdentity{
			ProviderRequestID: strings.TrimSpace(providerRequestID), ModelCallSequence: sequence, ToolIndex: index,
		}
	}
}

func (trace *ADKCapabilityTrace) ModelIdentity(callID string) (ADKToolCallModelIdentity, bool) {
	if trace == nil {
		return ADKToolCallModelIdentity{}, false
	}
	trace.mu.RLock()
	defer trace.mu.RUnlock()
	identity, ok := trace.ModelCalls[strings.TrimSpace(callID)]
	return identity, ok
}

func (trace *ADKCapabilityTrace) Snapshot() ([]capability.CapabilityInvocation, []capability.CapabilityResult) {
	if trace == nil {
		return nil, nil
	}
	trace.mu.RLock()
	defer trace.mu.RUnlock()
	return append([]capability.CapabilityInvocation(nil), trace.Invocations...), append([]capability.CapabilityResult(nil), trace.Results...)
}

func (trace *ADKCapabilityTrace) FindInvocation(callID string) (capability.CapabilityInvocation, bool) {
	if trace == nil {
		return capability.CapabilityInvocation{}, false
	}
	trace.mu.RLock()
	defer trace.mu.RUnlock()
	for _, invocation := range trace.Invocations {
		if invocation.CallID == callID {
			return invocation, true
		}
	}
	return capability.CapabilityInvocation{}, false
}

func (trace *ADKCapabilityTrace) FindResult(callID string) (capability.CapabilityResult, bool) {
	if trace == nil {
		return capability.CapabilityResult{}, false
	}
	trace.mu.RLock()
	defer trace.mu.RUnlock()
	for _, result := range trace.Results {
		if result.CallID == callID {
			return result, true
		}
	}
	return capability.CapabilityResult{}, false
}

func (trace *ADKCapabilityTrace) Append(invocation capability.CapabilityInvocation, result capability.CapabilityResult) {
	if trace == nil {
		return
	}
	trace.mu.Lock()
	defer trace.mu.Unlock()
	trace.Invocations = append(trace.Invocations, invocation)
	trace.Results = append(trace.Results, result)
}

func (trace *ADKCapabilityTrace) AppendInvocation(invocation capability.CapabilityInvocation) {
	if trace == nil {
		return
	}
	trace.mu.Lock()
	defer trace.mu.Unlock()
	trace.Invocations = append(trace.Invocations, invocation)
}

func (trace *ADKCapabilityTrace) AppendResult(result capability.CapabilityResult) {
	if trace == nil {
		return
	}
	trace.mu.Lock()
	defer trace.mu.Unlock()
	trace.Results = append(trace.Results, result)
}

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

// NewADKCapabilityTools exposes only the installed capability schemas and a
// request-scoped executor.
func NewADKCapabilityTools(definitions []capability.CapabilityDefinition, invoker ADKCapabilityInvoker) ([]tool.BaseTool, error) {
	if invoker == nil {
		return nil, errors.New("adk_capability_invoker_required")
	}
	if len(definitions) > 0 {
		if _, ok := invoker.(ADKCapabilityInvokerWithID); !ok {
			return nil, errors.New("adk_tool_call_identity_invoker_required")
		}
	}
	infos, err := CapabilityToolInfos(definitions)
	if err != nil {
		return nil, err
	}
	tools := make([]tool.BaseTool, 0, len(infos))
	for _, info := range infos {
		tools = append(tools, &ADKCapabilityTool{info: info, invoker: invoker})
	}
	return tools, nil
}

type ADKCapabilityTool struct {
	info    *schema.ToolInfo
	invoker ADKCapabilityInvoker
}

var _ tool.BaseTool = (*ADKCapabilityTool)(nil)

func (t *ADKCapabilityTool) Info(context.Context) (*schema.ToolInfo, error) {
	if t == nil || t.info == nil {
		return nil, errors.New("adk_capability_tool_unavailable")
	}
	return t.info, nil
}

func (t *ADKCapabilityTool) InvokableRun(ctx context.Context, argumentsJSON string, _ ...tool.Option) (string, error) {
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

func CapabilityToolInfos(definitions []capability.CapabilityDefinition) ([]*schema.ToolInfo, error) {
	tools := make([]*schema.ToolInfo, 0, len(definitions))
	for _, definition := range definitions {
		if strings.TrimSpace(definition.Name) == "" {
			return nil, errors.New("eino_tool_name_required")
		}
		var params *schema.ParamsOneOf
		if len(definition.InputSchema) > 0 {
			encoded, err := json.Marshal(definition.InputSchema)
			if err != nil {
				return nil, fmt.Errorf("eino_tool_schema_encode: %w", err)
			}
			var js jsonschema.Schema
			if err := json.Unmarshal(encoded, &js); err != nil {
				return nil, fmt.Errorf("eino_tool_schema_invalid: %w", err)
			}
			params = schema.NewParamsOneOfByJSONSchema(&js)
		}
		tools = append(tools, &schema.ToolInfo{Name: definition.Name, Desc: definition.Description, ParamsOneOf: params})
	}
	return tools, nil
}

// RunADKLoop is the only Eino ADK model/tool event iterator used by Core.
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
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: config.Name, Description: config.Description,
		Instruction: config.Instruction, Model: config.Model,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: config.Tools, ExecuteSequentially: true,
		}},
		// Production Agents do not impose a model/tool round limit. The
		// framework requires a positive value and otherwise defaults to 20, so
		// use the largest representable int when the caller did not explicitly
		// request a bounded test loop. Cancellation and request lifetime remain
		// the only production termination controls.
		MaxIterations: unboundedIterations(config.MaxIterations),
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
			result.FinalMessage = lastAssistant
			return result, fmt.Errorf("adk_run: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		message, getErr := event.Output.MessageOutput.GetMessage()
		if getErr != nil {
			result.FinalMessage = lastAssistant
			return result, fmt.Errorf("adk_message_output: %w", getErr)
		}
		if message == nil {
			continue
		}
		copyMessage := message
		result.Messages = append(result.Messages, copyMessage)
		if len(copyMessage.ToolCalls) > 0 {
			for _, call := range copyMessage.ToolCalls {
				// Eino may publish the same assistant event once from the model
				// callback and once as the graph output. The formal call ID is the
				// identity boundary, so count that physical call exactly once while
				// retaining distinct calls from every round.
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
			result.Iterations++
		}
	}
	result.FinalMessage = lastAssistant
	if result.FinalMessage == nil {
		return result, errors.New("adk_final_message_missing")
	}
	if strings.TrimSpace(result.FinalMessage.Content) == "" && len(result.FinalMessage.ToolCalls) == 0 {
		return result, errors.New("adk_final_message_empty")
	}
	return result, nil
}

func unboundedIterations(requested int) int {
	if requested > 0 {
		return requested
	}
	return int(^uint(0) >> 1)
}
