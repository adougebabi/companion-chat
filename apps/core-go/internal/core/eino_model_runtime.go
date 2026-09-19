package core

// This file owns the Eino boundary used by the Core model tasks.  Domain code
// continues to receive the bounded ProviderCompletion contract; Eino messages,
// tool schemas and stream readers never cross into persistence or BFF DTOs.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	embeddingopenai "github.com/cloudwego/eino-ext/components/embedding/openai"
	openaiext "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
)

// EinoModelConfig is the transport-neutral configuration needed to construct
// an official Eino model component. Secret resolution and assignment lookup
// happen outside this value object.
type EinoModelConfig struct {
	APIKey              string
	BaseURL             string
	Model               string
	Timeout             time.Duration
	MaxCompletionTokens int
	HTTPClient          *http.Client
	ResponseFormat      *openaiext.ChatCompletionResponseFormat
	ExtraFields         map[string]any
}

// EinoModelFactory constructs official Eino components. It deliberately has
// no database or domain dependency, which keeps model construction testable
// and prevents prompt/task code from reaching into App.
type EinoModelFactory struct {
	HTTPClient *http.Client
}

func NewEinoModelFactory(client *http.Client) EinoModelFactory {
	return EinoModelFactory{HTTPClient: client}
}

func (f EinoModelFactory) NewChatModel(ctx context.Context, config EinoModelConfig) (model.ToolCallingChatModel, error) {
	if strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("eino_model_required")
	}
	client := config.HTTPClient
	if client == nil {
		client = f.HTTPClient
	}
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	var maxCompletionTokens *int
	if config.MaxCompletionTokens > 0 {
		maxCompletionTokens = &config.MaxCompletionTokens
	}
	chat, err := openaiext.NewChatModel(ctx, &openaiext.ChatModelConfig{
		APIKey:              config.APIKey,
		BaseURL:             strings.TrimRight(config.BaseURL, "/"),
		Model:               config.Model,
		HTTPClient:          client,
		Timeout:             config.Timeout,
		MaxCompletionTokens: maxCompletionTokens,
		ResponseFormat:      config.ResponseFormat,
		ExtraFields:         cloneMap(config.ExtraFields),
	})
	if err != nil {
		return nil, fmt.Errorf("eino_chat_model_create: %w", err)
	}
	return chat, nil
}

func (f EinoModelFactory) NewEmbedder(ctx context.Context, config EinoModelConfig) (embedding.Embedder, error) {
	if strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("eino_embedding_model_required")
	}
	client := config.HTTPClient
	if client == nil {
		client = f.HTTPClient
	}
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	emb, err := embeddingopenai.NewEmbedder(ctx, &embeddingopenai.EmbeddingConfig{
		APIKey:     config.APIKey,
		BaseURL:    strings.TrimRight(config.BaseURL, "/"),
		Model:      config.Model,
		HTTPClient: client,
		Timeout:    config.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("eino_embedder_create: %w", err)
	}
	return emb, nil
}

// EinoModelCall captures one bounded Generate/Stream call. It is intentionally
// request-scoped; an ADK agent never retains queue permits or mutable persona
// state across calls.
type EinoModelCall struct {
	Assignment        providerAssignment
	Role              string
	Scenario          string
	Priority          int
	DiagnosticID      string
	CorrelationID     string
	Messages          []map[string]any
	Definitions       []CapabilityDefinition
	JSONMode          bool
	SchemaName        string
	ResponseSchema    map[string]any
	EnableThinking    bool
	ProviderRequestID string
}

type queuedToolCallingChatModel struct {
	inner         model.ToolCallingChatModel
	provider      *ProviderClient
	role          string
	scenario      string
	priority      int
	diagnosticID  string
	assignment    providerAssignment
	correlationID string
	sequence      *atomic.Uint64
}

func (m *queuedToolCallingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	input = normalizeEinoToolMessageNames(input)
	sequence := uint64(1)
	if m.sequence != nil {
		sequence = m.sequence.Add(1)
	}
	callCorrelation := fmt.Sprintf("%s:adk:%d", m.correlationID, sequence)
	callRequestID := providerDiagnosticRequestID(m.role, callCorrelation)
	callCtx := withEinoRequestID(withoutProviderQueueBypass(ctx), callRequestID)
	// The parent turn attempt is intentionally not reused as the physical
	// model-call attempt. Diagnostics and cancellation state must distinguish
	// Generate #1 from Generate #2 while both share one turn correlation.
	callCtx = WithProviderAttemptIdentity(callCtx, randomID("provider_attempt_"))
	callDiagnosticID := ""
	if m.provider != nil && m.provider.DB != nil {
		callDiagnosticID = (&App{DB: m.provider.DB}).recordQueuedModelRun(callCtx, m.role, m.assignment.EndpointID, m.assignment.ModelID, callCorrelation, m.scenario, m.priority, einoDiagnosticMessages(input))
	}
	started := time.Now()
	result, err := runProviderQueued(m.provider, callCtx, m.role, m.scenario, m.priority, callDiagnosticID, func(runCtx context.Context) (*schema.Message, error) {
		return m.inner.Generate(runCtx, input, opts...)
	})
	if callDiagnosticID != "" {
		(&App{DB: m.provider.DB}).updateModelRunPromptMetrics(callCtx, callDiagnosticID, einoUsage(result), time.Since(started))
	}
	return result, err
}

func (m *queuedToolCallingChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	input = normalizeEinoToolMessageNames(input)
	sequence := uint64(1)
	if m.sequence != nil {
		sequence = m.sequence.Add(1)
	}
	callCorrelation := fmt.Sprintf("%s:adk:%d", m.correlationID, sequence)
	callRequestID := providerDiagnosticRequestID(m.role, callCorrelation)
	callCtx := withEinoRequestID(withoutProviderQueueBypass(ctx), callRequestID)
	callCtx = WithProviderAttemptIdentity(callCtx, randomID("provider_attempt_"))
	callDiagnosticID := ""
	if m.provider != nil && m.provider.DB != nil {
		callDiagnosticID = (&App{DB: m.provider.DB}).recordQueuedModelRun(callCtx, m.role, m.assignment.EndpointID, m.assignment.ModelID, callCorrelation, m.scenario, m.priority, einoDiagnosticMessages(input))
	}
	return runProviderQueued(m.provider, callCtx, m.role, m.scenario, m.priority, callDiagnosticID, func(runCtx context.Context) (*schema.StreamReader[*schema.Message], error) {
		return m.inner.Stream(runCtx, input, opts...)
	})
}

func (m *queuedToolCallingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	sequence := m.sequence
	if sequence == nil {
		sequence = &atomic.Uint64{}
	}
	return &queuedToolCallingChatModel{inner: bound, provider: m.provider, role: m.role, scenario: m.scenario, priority: m.priority, diagnosticID: m.diagnosticID, assignment: m.assignment, correlationID: m.correlationID, sequence: sequence}, nil
}

type einoModelResponse struct {
	Message      *schema.Message
	Usage        map[string]any
	FinishReason string
}

func einoDiagnosticMessages(messages []*schema.Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		result = append(result, einoMessageRaw(message))
		if len(result) >= 64 {
			break
		}
	}
	return result
}

type einoHeaderRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

type einoRequestIDContextKey struct{}

func withEinoRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, einoRequestIDContextKey{}, requestID)
}

func (t einoHeaderRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	for key, value := range t.headers {
		if strings.TrimSpace(value) != "" {
			clone.Header.Set(key, value)
		}
	}
	if requestID, _ := clone.Context().Value(einoRequestIDContextKey{}).(string); strings.TrimSpace(requestID) != "" {
		clone.Header.Set("Idempotency-Key", requestID)
		clone.Header.Set("X-Fluctlight-Provider-Request-Id", requestID)
	}
	return t.base.RoundTrip(clone)
}

func einoHTTPClientWithHeaders(base *http.Client, headers map[string]string) *http.Client {
	if base == nil {
		base = &http.Client{}
	}
	clone := *base
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clone.Transport = einoHeaderRoundTripper{base: transport, headers: headers}
	return &clone
}

func (p *ProviderClient) generateWithEino(ctx context.Context, call EinoModelCall) (einoModelResponse, error) {
	if p == nil {
		return einoModelResponse{}, errors.New("eino_provider_unavailable")
	}
	if adkContext, ok := adkCapabilityContext(ctx); ok && isADKLoopSchema(call.SchemaName) {
		return p.generateWithADK(ctx, call, adkContext)
	}
	input, err := providerMessagesToEino(call.Messages)
	if err != nil {
		return einoModelResponse{}, fmt.Errorf("eino_message_encode: %w", err)
	}
	responseFormat, err := einoResponseFormatForRole(call.Role, call.JSONMode, call.SchemaName, call.ResponseSchema)
	if err != nil {
		return einoModelResponse{}, err
	}
	extra := map[string]any{}
	if call.EnableThinking {
		extra["enable_thinking"] = true
	}
	if len(call.Definitions) > 1 {
		extra["parallel_tool_calls"] = true
	}
	factory := NewEinoModelFactory(p.HTTP)
	chat, err := factory.NewChatModel(ctx, EinoModelConfig{
		APIKey: call.Assignment.Secret, BaseURL: call.Assignment.BaseURL,
		Model: call.Assignment.ModelID, Timeout: call.Assignment.Timeout,
		MaxCompletionTokens: call.Assignment.TokenBudget, HTTPClient: p.HTTP,
		ResponseFormat: responseFormat, ExtraFields: extra,
	})
	if err != nil {
		return einoModelResponse{}, err
	}
	if len(call.Definitions) > 0 {
		infos, infoErr := capabilityToolInfos(call.Definitions)
		if infoErr != nil {
			return einoModelResponse{}, infoErr
		}
		chat, err = chat.WithTools(infos)
		if err != nil {
			return einoModelResponse{}, fmt.Errorf("eino_tools_bind: %w", err)
		}
	}
	options := []model.Option{
		openaiext.WithExtraHeader(map[string]string{
			"Idempotency-Key":                  call.ProviderRequestID,
			"X-Fluctlight-Provider-Request-Id": call.ProviderRequestID,
		}),
	}
	if len(extra) > 0 {
		options = append(options, openaiext.WithExtraFields(extra))
	}
	message, err := chat.Generate(ctx, input, options...)
	if err != nil {
		return einoModelResponse{}, err
	}
	if message == nil {
		return einoModelResponse{}, errors.New("eino_empty_message")
	}
	result := einoModelResponse{Message: message, Usage: einoUsage(message), FinishReason: "stop"}
	if message.ResponseMeta != nil && strings.TrimSpace(message.ResponseMeta.FinishReason) != "" {
		result.FinishReason = strings.TrimSpace(message.ResponseMeta.FinishReason)
	}
	return result, nil
}

func (p *ProviderClient) generateWithADK(ctx context.Context, call EinoModelCall, adkContext adkConversationContext) (einoModelResponse, error) {
	input, err := providerMessagesToEino(call.Messages)
	if err != nil {
		return einoModelResponse{}, fmt.Errorf("eino_message_encode: %w", err)
	}
	factory := NewEinoModelFactory(p.HTTP)
	responseFormat, formatErr := einoResponseFormatForRole(call.Role, call.JSONMode, call.SchemaName, call.ResponseSchema)
	if formatErr != nil {
		return einoModelResponse{}, formatErr
	}
	extra := map[string]any{}
	if call.EnableThinking {
		extra["enable_thinking"] = true
	}
	if len(call.Definitions) > 1 {
		extra["parallel_tool_calls"] = true
	}
	requestHTTP := einoHTTPClientWithHeaders(p.HTTP, map[string]string{
		"Idempotency-Key": call.ProviderRequestID, "X-Fluctlight-Provider-Request-Id": call.ProviderRequestID,
	})
	chat, err := factory.NewChatModel(ctx, EinoModelConfig{
		APIKey: call.Assignment.Secret, BaseURL: call.Assignment.BaseURL,
		Model: call.Assignment.ModelID, Timeout: call.Assignment.Timeout,
		MaxCompletionTokens: call.Assignment.TokenBudget, HTTPClient: requestHTTP,
		ResponseFormat: responseFormat, ExtraFields: extra,
	})
	if err != nil {
		return einoModelResponse{}, err
	}
	chat = &queuedToolCallingChatModel{inner: chat, provider: p, role: call.Role, scenario: call.Scenario, priority: call.Priority, diagnosticID: call.DiagnosticID, assignment: call.Assignment, correlationID: call.CorrelationID, sequence: &atomic.Uint64{}}
	tools, err := NewADKCapabilityTools(call.Definitions, adkContext.Invoker)
	if err != nil {
		return einoModelResponse{}, err
	}
	infos := make([]*schema.ToolInfo, 0, len(tools))
	for _, candidate := range tools {
		info, infoErr := candidate.Info(ctx)
		if infoErr != nil {
			return einoModelResponse{}, infoErr
		}
		infos = append(infos, info)
	}
	if len(infos) > 0 {
		chat, err = chat.WithTools(infos)
		if err != nil {
			return einoModelResponse{}, fmt.Errorf("eino_tools_bind: %w", err)
		}
	}
	// ADK itself owns the model/tool/result loop; each underlying model call
	// still receives the same bounded request identity and no retry policy.
	result, err := RunADKLoop(ctx, ADKLoopConfig{
		Name: "fluctlight-conversation", Description: "Fluctlight bounded model and capability runtime",
		Model: chat, Tools: tools, MaxIterations: 2,
	}, input)
	if err != nil {
		return einoModelResponse{}, err
	}
	if result.FinalMessage == nil {
		return einoModelResponse{}, errors.New("adk_final_message_missing")
	}
	final := result.FinalMessage
	if len(result.ToolCalls) > 0 {
		final.ToolCalls = append([]schema.ToolCall(nil), result.ToolCalls...)
	}
	return einoModelResponse{Message: final, Usage: einoUsage(final), FinishReason: "stop"}, nil
}

func (p *ProviderClient) streamWithEino(ctx context.Context, assignment providerAssignment, messages []map[string]any, requestID string, onChunk func(string) error) (string, error) {
	input, err := providerMessagesToEino(messages)
	if err != nil {
		return "", fmt.Errorf("eino_message_encode: %w", err)
	}
	factory := NewEinoModelFactory(p.HTTP)
	chat, err := factory.NewChatModel(ctx, EinoModelConfig{
		APIKey: assignment.Secret, BaseURL: assignment.BaseURL, Model: assignment.ModelID,
		Timeout: assignment.Timeout, MaxCompletionTokens: assignment.TokenBudget, HTTPClient: p.HTTP,
	})
	if err != nil {
		return "", err
	}
	stream, err := chat.Stream(ctx, input, openaiext.WithExtraHeader(map[string]string{
		"Idempotency-Key": requestID, "X-Fluctlight-Provider-Request-Id": requestID,
	}))
	if err != nil {
		return "", err
	}
	defer stream.Close()
	var builder strings.Builder
	for {
		message, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return "", recvErr
		}
		if message == nil || message.Content == "" {
			continue
		}
		builder.WriteString(message.Content)
		if onChunk != nil {
			if err := onChunk(message.Content); err != nil {
				return "", err
			}
		}
	}
	return builder.String(), nil
}

func einoUsage(message *schema.Message) map[string]any {
	if message == nil || message.ResponseMeta == nil || message.ResponseMeta.Usage == nil {
		return map[string]any{}
	}
	usage := message.ResponseMeta.Usage
	return map[string]any{
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
		"total_tokens":      usage.TotalTokens,
	}
}

func einoMessageRaw(message *schema.Message) map[string]any {
	if message == nil {
		return map[string]any{}
	}
	toolCalls := make([]any, 0, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		toolCalls = append(toolCalls, map[string]any{
			"id": call.ID, "type": firstString(call.Type, "function"),
			"function": map[string]any{"name": call.Function.Name, "arguments": call.Function.Arguments},
		})
	}
	return map[string]any{
		"role":              string(message.Role),
		"content":           message.Content,
		"tool_calls":        toolCalls,
		"reasoning_content": message.ReasoningContent,
	}
}

// normalizeEinoToolMessageNames bridges Eino's ToolName field to the OpenAI
// wire adapter's Name field. Eino's ToolMessage constructor stores the formal
// capability name in ToolName, while the OpenAI chat envelope serializes Name;
// copying it at the model boundary preserves the call/result name alongside
// the formal tool_call_id without changing the request-scoped ADK messages.
func normalizeEinoToolMessageNames(messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}
	result := make([]*schema.Message, len(messages))
	for index, message := range messages {
		if message == nil {
			continue
		}
		copyMessage := *message
		if copyMessage.Role == schema.Tool && strings.TrimSpace(copyMessage.Name) == "" {
			copyMessage.Name = strings.TrimSpace(copyMessage.ToolName)
		}
		result[index] = &copyMessage
	}
	return result
}

func einoResponseFormat(jsonMode bool, schemaName string, raw map[string]any) (*openaiext.ChatCompletionResponseFormat, error) {
	if !jsonMode {
		return nil, nil
	}
	if strings.TrimSpace(schemaName) == "" || len(raw) == 0 {
		return &openaiext.ChatCompletionResponseFormat{Type: openaiext.ChatCompletionResponseFormatTypeJSONObject}, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("eino_response_schema_encode: %w", err)
	}
	var parsed jsonschema.Schema
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		return nil, fmt.Errorf("eino_response_schema_invalid: %w", err)
	}
	return &openaiext.ChatCompletionResponseFormat{
		Type: openaiext.ChatCompletionResponseFormatTypeJSONSchema,
		JSONSchema: &openaiext.ChatCompletionResponseFormatJSONSchema{
			Name: schemaName, JSONSchema: &parsed, Strict: true,
		},
	}, nil
}

func einoResponseFormatForRole(role string, jsonMode bool, schemaName string, raw map[string]any) (*openaiext.ChatCompletionResponseFormat, error) {
	if strings.TrimSpace(role) == "initialization" {
		if !jsonMode {
			return nil, nil
		}
		return &openaiext.ChatCompletionResponseFormat{Type: openaiext.ChatCompletionResponseFormatTypeJSONObject}, nil
	}
	return einoResponseFormat(jsonMode, schemaName, raw)
}

func capabilityToolInfos(definitions []CapabilityDefinition) ([]*schema.ToolInfo, error) {
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

func providerMessagesToEino(messages []map[string]any) ([]*schema.Message, error) {
	result := make([]*schema.Message, 0, len(messages))
	for index, raw := range messages {
		role := strings.ToLower(strings.TrimSpace(stringValue(raw["role"])))
		if role == "" {
			return nil, fmt.Errorf("message_%d_role_required", index)
		}
		message := &schema.Message{Role: schema.RoleType(role)}
		if calls := raw["tool_calls"]; calls != nil {
			message.ToolCalls = einoToolCalls(calls)
		}
		message.ToolCallID = strings.TrimSpace(stringValue(raw["tool_call_id"]))
		message.ToolName = strings.TrimSpace(stringValue(raw["name"]))
		if role == "tool" {
			message.Name = message.ToolName
		}
		content := raw["content"]
		if parts, ok := content.([]any); ok {
			multi, text, partErr := einoInputParts(parts)
			if partErr != nil {
				return nil, fmt.Errorf("message_%d_parts_invalid: %w", index, partErr)
			}
			message.UserInputMultiContent = multi
			message.Content = text
		} else if parts, ok := content.([]map[string]any); ok {
			asAny := make([]any, len(parts))
			for i := range parts {
				asAny[i] = parts[i]
			}
			multi, text, partErr := einoInputParts(asAny)
			if partErr != nil {
				return nil, fmt.Errorf("message_%d_parts_invalid: %w", index, partErr)
			}
			message.UserInputMultiContent = multi
			message.Content = text
		} else {
			message.Content = stringValue(content)
		}
		if role == "assistant" && len(message.UserInputMultiContent) > 0 {
			message.AssistantGenMultiContent = nil
		}
		result = append(result, message)
	}
	return result, nil
}

func einoToolCalls(raw any) []schema.ToolCall {
	items := arrayValue(raw)
	result := make([]schema.ToolCall, 0, len(items))
	for _, item := range items {
		value := mapValue(item)
		function := mapValue(value["function"])
		name := stringValue(function["name"])
		args := stringValue(function["arguments"])
		if name == "" {
			name = stringValue(value["name"])
		}
		if args == "" {
			args = stringValue(value["arguments"])
		}
		result = append(result, schema.ToolCall{ID: stringValue(value["id"]), Type: firstString(stringValue(value["type"]), "function"), Function: schema.FunctionCall{Name: name, Arguments: args}})
	}
	return result
}

func einoInputParts(parts []any) ([]schema.MessageInputPart, string, error) {
	result := make([]schema.MessageInputPart, 0, len(parts))
	var text strings.Builder
	for _, raw := range parts {
		part := mapValue(raw)
		switch stringValue(part["type"]) {
		case "text":
			value := stringValue(part["text"])
			result = append(result, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: value})
			text.WriteString(value)
		case "image_url":
			image := mapValue(part["image_url"])
			url := stringValue(image["url"])
			detail := schema.ImageURLDetail(strings.ToLower(stringValue(image["detail"])))
			result = append(result, schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &url}, Detail: detail}})
		case "image":
			url := stringValue(part["url"])
			mime := stringValue(part["mime_type"])
			result = append(result, schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &url, MIMEType: mime}}})
		default:
			return nil, "", fmt.Errorf("unsupported_part_type:%s", stringValue(part["type"]))
		}
	}
	return result, text.String(), nil
}
