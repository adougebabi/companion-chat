package agent

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
	jsonschema "github.com/eino-contrib/jsonschema"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

// ADKCapabilityTrace is request-scoped state used to carry the canonical
// invocation/result pair from an ADK tool callback into Core's existing frozen
// settlement path. It is never persisted as an alternative envelope.
type ADKCapabilityTrace struct {
	Invocations []capability.CapabilityInvocation
	Results     []capability.CapabilityResult
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
			return result, fmt.Errorf("adk_run: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		message, getErr := event.Output.MessageOutput.GetMessage()
		if getErr != nil {
			return result, fmt.Errorf("adk_message_output: %w", getErr)
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
		}
		result.Iterations++
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
