package core

// This file owns the Eino boundary used by the Core model tasks.  Domain code
// continues to receive the bounded ProviderCompletion contract; Eino messages,
// tool schemas and stream readers never cross into persistence or browser DTOs.

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

	openaiext "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
	aimodel "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/model"
)

// EinoModelConfig is the transport-neutral configuration needed to construct
// an official Eino model component.
type EinoModelConfig = aimodel.EinoModelConfig

// EinoModelFactory constructs official Eino components.
type EinoModelFactory = aimodel.EinoModelFactory

func NewEinoModelFactory(client *http.Client) EinoModelFactory {
	return aimodel.NewEinoModelFactory(client)
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
	EnableStreaming   bool
	ProviderRequestID string
	Agent             FormalAgentDefinition
}

type queuedToolCallingChatModel struct {
	inner          model.ToolCallingChatModel
	provider       *ProviderClient
	role           string
	scenario       string
	priority       int
	diagnosticID   string
	assignment     providerAssignment
	correlationID  string
	sequence       *atomic.Uint64
	definitions    []CapabilityDefinition
	responseFormat map[string]any
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
	callCtx = withPhysicalModelCallDiagnostics(callCtx, m.correlationID, callRequestID)
	recordEinoModelInputDiagnostic(callCtx, m.provider, m.role, callCorrelation, sequence, input)
	// The parent turn attempt is intentionally not reused as the physical
	// model-call attempt. Diagnostics and cancellation state must distinguish
	// Generate #1 from Generate #2 while both share one turn correlation.
	callCtx = WithProviderAttemptIdentity(callCtx, randomID("provider_attempt_"))
	callDiagnosticID := ""
	if m.provider != nil && m.provider.DB != nil {
		callDiagnosticID = m.provider.runtimeSupport().RecordQueuedModelRun(callCtx, m.role, m.assignment.EndpointID, m.assignment.ModelID, m.correlationID, m.scenario, m.priority, einoDiagnosticMessages(input))
	}
	started := time.Now()
	result, err := runProviderQueued(m.provider, callCtx, m.role, m.scenario, m.priority, callDiagnosticID, func(runCtx context.Context) (*schema.Message, error) {
		return m.inner.Generate(runCtx, input, opts...)
	})
	if callDiagnosticID != "" && result != nil {
		m.provider.runtimeSupport().UpdateModelRunResponse(callCtx, callDiagnosticID, einoMessageRaw(result))
	}
	if adkContext, ok := adkCapabilityContext(ctx); ok && adkContext.Trace != nil && result != nil {
		adkContext.Trace.RecordModelToolCalls(callRequestID, sequence, result.ToolCalls)
	}
	recordEinoModelOutputDiagnostic(callCtx, m.provider, m.role, callCorrelation, sequence, result, err)
	if callDiagnosticID != "" {
		m.provider.runtimeSupport().UpdateModelRunPromptMetrics(callCtx, callDiagnosticID, einoUsage(result), time.Since(started))
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
	callCtx = withPhysicalModelCallDiagnostics(callCtx, m.correlationID, callRequestID)
	recordEinoModelInputDiagnostic(callCtx, m.provider, m.role, callCorrelation, sequence, input)
	callDiagnosticID := ""
	if m.provider != nil && m.provider.DB != nil {
		callDiagnosticID = m.provider.runtimeSupport().RecordQueuedModelRun(callCtx, m.role, m.assignment.EndpointID, m.assignment.ModelID, m.correlationID, m.scenario, m.priority, einoDiagnosticMessages(input))
	}
	stream, err := runProviderQueuedStream(m.provider, callCtx, m.role, m.scenario, m.priority, callDiagnosticID, func(runCtx context.Context) (*schema.StreamReader[*schema.Message], error) {
		return m.inner.Stream(runCtx, input, opts...)
	})
	if err != nil || stream == nil {
		return stream, err
	}
	adkContext, traceEnabled := adkCapabilityContext(ctx)
	if !traceEnabled || adkContext.Trace == nil {
		return stream, nil
	}
	// Keep the native stream intact while recording the formal ToolCall ID as
	// soon as Eino exposes it. Later chunks may repeat/complete the same call;
	// the trace map is an idempotent overwrite for the same physical request.
	return schema.StreamReaderWithConvert(stream, func(message *schema.Message) (*schema.Message, error) {
		if message != nil {
			adkContext.Trace.RecordModelToolCalls(callRequestID, sequence, message.ToolCalls)
			if callDiagnosticID != "" && (message.Content != "" || len(message.ToolCalls) > 0 || message.ReasoningContent != "") {
				m.provider.runtimeSupport().UpdateModelRunResponse(callCtx, callDiagnosticID, einoMessageRaw(message))
			}
		}
		return message, nil
	}), nil
}

func withPhysicalModelCallDiagnostics(ctx context.Context, runID, modelCallID string) context.Context {
	diagnostics := providerPromptDiagnostics(ctx)
	if strings.TrimSpace(runID) != "" {
		diagnostics["run_id"] = strings.TrimSpace(runID)
	}
	if strings.TrimSpace(modelCallID) != "" {
		diagnostics["model_call_id"] = strings.TrimSpace(modelCallID)
	}
	return WithPromptDiagnostics(ctx, diagnostics)
}

func recordEinoModelInputDiagnostic(ctx context.Context, provider *ProviderClient, role, correlationID string, sequence uint64, input []*schema.Message) {
	if provider == nil || provider.DB == nil || provider.DB.Pool() == nil {
		return
	}
	assistantCalls := make(map[string]struct{})
	toolResults := make([]string, 0, 8)
	messageCount := 0
	for _, message := range input {
		if message == nil {
			continue
		}
		messageCount++
		if message.Role == schema.Assistant {
			for _, call := range message.ToolCalls {
				if id := strings.TrimSpace(call.ID); id != "" {
					assistantCalls[id] = struct{}{}
				}
			}
		}
		if message.Role == schema.Tool {
			if id := strings.TrimSpace(message.ToolCallID); id != "" {
				toolResults = append(toolResults, id)
			}
		}
	}
	matched := 0
	for _, id := range toolResults {
		if _, ok := assistantCalls[id]; ok {
			matched++
		}
	}
	diagnostics := providerPromptDiagnostics(ctx)
	fluctlightID := stringValue(diagnostics["fluctlight_id"])
	runID := stringValue(diagnostics["run_id"])
	if runID == "" {
		runID = correlationID
	}
	modelCallID := stringValue(diagnostics["model_call_id"])
	provider.runtimeSupport().RecordDiagnosticEvent(ctx, "adk.model.input", "info", fluctlightID, "", correlationID, map[string]any{
		"run_id": runID, "model_call_id": modelCallID, "stage": "model_input", "role": role, "sequence": sequence,
		"message_count": messageCount, "tool_result_ids": boundedDiagnosticStrings(toolResults, 32),
		"tool_result_pair_count": matched, "tool_result_pair_status": map[bool]string{true: "present", false: "absent"}[matched > 0],
	})
}

func recordEinoModelOutputDiagnostic(ctx context.Context, provider *ProviderClient, role, correlationID string, sequence uint64, message *schema.Message, runErr error) {
	if provider == nil || provider.DB == nil || provider.DB.Pool() == nil {
		return
	}
	diagnostics := providerPromptDiagnostics(ctx)
	fluctlightID := stringValue(diagnostics["fluctlight_id"])
	runID := stringValue(diagnostics["run_id"])
	if runID == "" {
		runID = correlationID
	}
	modelCallID := stringValue(diagnostics["model_call_id"])
	callIDs := make([]string, 0, 8)
	if message != nil {
		for _, call := range message.ToolCalls {
			if id := strings.TrimSpace(call.ID); id != "" {
				callIDs = append(callIDs, id)
			}
		}
	}
	status := "completed"
	if runErr != nil {
		status = "failed"
	}
	provider.runtimeSupport().RecordDiagnosticEvent(ctx, "adk.model.output", "info", fluctlightID, "", correlationID, map[string]any{
		"run_id": runID, "model_call_id": modelCallID, "stage": "model_output", "role": role, "sequence": sequence,
		"status": status, "tool_call_ids": boundedDiagnosticStrings(callIDs, 32),
		"error_code": providerRunErrorCode(runErr),
	})
}

func boundedDiagnosticStrings(values []string, limit int) []string {
	if limit < 1 {
		return []string{}
	}
	if len(values) > limit {
		values = values[:limit]
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// runProviderQueuedStream keeps the physical queue lease and cancellation
// watcher alive until the returned StreamReader reaches EOF or is closed. A
// plain runProviderQueued call cannot be used here: it would release its
// defer-cancel immediately after receiving the reader, before the provider has
// emitted any bytes.
func runProviderQueuedStream(p *ProviderClient, ctx context.Context, role, scenario string, priority int, diagnosticID string, fn func(context.Context) (*schema.StreamReader[*schema.Message], error)) (*schema.StreamReader[*schema.Message], error) {
	if p == nil {
		return nil, errors.New("provider_unavailable")
	}
	if providerQueueBypassed(ctx) {
		return fn(ctx)
	}
	p.refreshQueueLimits(ctx)
	reader, writer := schema.Pipe[*schema.Message](1)
	runCtx, cancel := context.WithCancel(ctx)
	stopWatch := p.watchProviderCancellation(runCtx, cancel)
	go func() {
		defer stopWatch()
		defer cancel()
		releaseRedis, redisEnabled, redisErr := p.acquireProviderRedisSlot(runCtx, role, priority, p.queueFor(role).currentLimit(), diagnosticID)
		if redisErr != nil {
			if diagnosticID != "" {
				p.runtimeSupport().UpdateModelRunState(ctx, diagnosticID, providerRunFailed, redisErr)
			}
			writer.Send(nil, redisErr)
			writer.Close()
			return
		}
		if redisEnabled {
			defer releaseRedis()
		}
		err := p.queueFor(role).submit(runCtx, priority, func(taskCtx context.Context) error {
			if guard := providerExecutionGuard(taskCtx); guard != nil {
				if guardErr := guard(taskCtx); guardErr != nil {
					return guardErr
				}
			}
			inner, streamErr := fn(taskCtx)
			if streamErr != nil {
				return streamErr
			}
			if inner == nil {
				return errors.New("provider_stream_reader_missing")
			}
			defer inner.Close()
			innerDone := make(chan struct{})
			defer close(innerDone)
			go func() {
				select {
				case <-taskCtx.Done():
					inner.Close()
				case <-innerDone:
				}
			}()
			for {
				message, recvErr := inner.Recv()
				if errors.Is(recvErr, io.EOF) {
					return nil
				}
				if recvErr != nil {
					return recvErr
				}
				if writer.Send(message, nil) {
					inner.Close()
					return context.Canceled
				}
			}
		}, func(status string, runErr error) {
			if p.DB == nil || diagnosticID == "" {
				return
			}
			p.runtimeSupport().UpdateModelRunState(ctx, diagnosticID, status, runErr)
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			writer.Send(nil, err)
		}
		writer.Close()
	}()
	return reader, nil
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
	return &queuedToolCallingChatModel{inner: bound, provider: m.provider, role: m.role, scenario: m.scenario, priority: m.priority, diagnosticID: m.diagnosticID, assignment: m.assignment, correlationID: m.correlationID, sequence: sequence, definitions: m.definitions, responseFormat: m.responseFormat}, nil
}

type einoModelResponse struct {
	Message           *schema.Message
	ExecutedToolCalls []schema.ToolCall
	Usage             map[string]any
	FinishReason      string
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
	if adkContext, ok := adkCapabilityContext(ctx); ok {
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
		MaxCompletionTokens: 0, HTTPClient: p.HTTP,
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
	if strings.TrimSpace(call.Agent.Name) == "" {
		id, ok := formalAgentForSchema(call.SchemaName)
		if !ok {
			return einoModelResponse{}, fmt.Errorf("formal_agent_not_registered_for_schema: %s", call.SchemaName)
		}
		call.Agent, _ = FormalAgentDefinitionByID(id)
	}
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
		MaxCompletionTokens: 0, HTTPClient: requestHTTP,
		ResponseFormat: responseFormat, ExtraFields: extra,
	})
	if err != nil {
		return einoModelResponse{}, err
	}
	budgetResponseFormat := map[string]any{}
	if call.JSONMode {
		budgetResponseFormat = providerResponseFormatForSchema(call.Role, call.SchemaName, call.ResponseSchema)
	}
	chat = &queuedToolCallingChatModel{
		inner: chat, provider: p, role: call.Role, scenario: call.Scenario, priority: call.Priority,
		diagnosticID: call.DiagnosticID, assignment: call.Assignment, correlationID: call.CorrelationID,
		sequence: &atomic.Uint64{}, definitions: call.Definitions, responseFormat: budgetResponseFormat,
	}
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
		Name: call.Agent.Name, Description: call.Agent.Description,
		Model: chat, Tools: tools, MaxIterations: call.Agent.MaxIterations,
		EnableStreaming: call.EnableStreaming,
	}, input)
	recordADKTerminationDiagnostic(ctx, p, call, result, err)
	if err != nil {
		return einoModelResponse{}, err
	}
	if result.FinalMessage == nil {
		return einoModelResponse{}, errors.New("adk_final_message_missing")
	}
	final := result.FinalMessage
	// Final output and executed calls are separate contracts. Intermediate
	// calls remain in the execution trace and must never be copied onto the
	// final assistant message, where they could bypass the final output schema.
	return einoModelResponse{
		Message: final, ExecutedToolCalls: append([]schema.ToolCall(nil), result.ToolCalls...),
		Usage: einoUsage(final), FinishReason: "stop",
	}, nil
}

func recordADKTerminationDiagnostic(ctx context.Context, provider *ProviderClient, call EinoModelCall, result ADKLoopResult, runErr error) {
	if provider == nil || provider.DB == nil || provider.DB.Pool() == nil {
		return
	}
	status := "completed"
	reason := "final_message"
	if runErr != nil {
		status = "failed"
		reason = providerRunErrorCode(runErr)
	}
	diagnostics := providerPromptDiagnostics(ctx)
	runID := stringValue(diagnostics["run_id"])
	if runID == "" {
		runID = call.CorrelationID
	}
	provider.runtimeSupport().RecordDiagnosticEvent(ctx, "adk.run.termination", statusSeverity(status), stringValue(diagnostics["fluctlight_id"]), "", call.CorrelationID, map[string]any{
		"run_id": runID, "stage": "termination", "status": status, "reason": reason,
		"iterations": result.Iterations, "tool_call_count": len(result.ToolCalls), "tool_result_count": len(result.ToolResults),
	})
}

// mergeADKTraceInvocations joins calls retained by the final assistant event
// with request-scoped native calls from every intermediate ADK round. The
// completion's call order remains authoritative; trace-only calls are appended
// once by their formal call ID.
func mergeADKTraceInvocations(completion []CapabilityInvocation, trace *ADKCapabilityTrace) []CapabilityInvocation {
	invocations, _ := trace.Snapshot()
	if len(invocations) == 0 {
		return completion
	}
	result := append([]CapabilityInvocation(nil), completion...)
	seen := make(map[string]struct{}, len(result)+len(invocations))
	for _, invocation := range result {
		if id := strings.TrimSpace(invocation.CallID); id != "" {
			seen[id] = struct{}{}
		}
	}
	for _, invocation := range invocations {
		id := strings.TrimSpace(invocation.CallID)
		if id != "" {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
		}
		result = append(result, invocation)
	}
	return result
}

func (p *ProviderClient) streamWithEino(ctx context.Context, assignment providerAssignment, messages []map[string]any, requestID string, onChunk func(string) error) (string, error) {
	input, err := providerMessagesToEino(messages)
	if err != nil {
		return "", fmt.Errorf("eino_message_encode: %w", err)
	}
	factory := NewEinoModelFactory(p.HTTP)
	chat, err := factory.NewChatModel(ctx, EinoModelConfig{
		APIKey: assignment.Secret, BaseURL: assignment.BaseURL, Model: assignment.ModelID,
		Timeout: assignment.Timeout, MaxCompletionTokens: 0, HTTPClient: p.HTTP,
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

// validateADKStructuredResponse is the post-Runner semantic boundary for an
// ADK response. The Runner owns model/tool orchestration; the Provider owns
// the structured output contract. Keeping this check separate lets a tool
// fact remain in the request-scoped trace when the final model response is
// malformed.
func validateADKStructuredResponse(message *schema.Message, role string) error {
	candidates := providerStructuredCandidates(einoMessageRaw(message))
	if len(candidates) == 0 {
		return nil
	}
	_, ok, parseErr := parseStructuredCandidatesForRole(role, candidates)
	if parseErr != nil {
		return fmt.Errorf("adk_structured_response_invalid: %w", parseErr)
	}
	if !ok {
		return errors.New("adk_structured_response_invalid")
	}
	return nil
}

// normalizeEinoNativeToolCalls is the ADK-only bridge from Eino's typed
// schema.Message to the Core invocation contract. It deliberately does not
// inspect Content or ReasoningContent and never derives an ID: Eino's formal
// ToolCall identity is the only execution authority for an ADK round.
func normalizeEinoNativeToolCalls(message *schema.Message, providerRequestID string) ([]CapabilityInvocation, error) {
	if message == nil || len(message.ToolCalls) == 0 {
		return []CapabilityInvocation{}, nil
	}
	raw := make([]any, 0, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		raw = append(raw, map[string]any{
			"id":       call.ID,
			"type":     firstString(call.Type, "function"),
			"function": map[string]any{"name": call.Function.Name, "arguments": call.Function.Arguments},
		})
	}
	return normalizeProviderToolCalls(raw, "", providerRequestID)
}

// normalizeEinoNativeToolCallsIndependently keeps valid typed siblings while
// retaining a bounded error for malformed siblings. It still requires every
// accepted sibling to carry Eino's real ID; no per-position or request-derived
// identity is ever manufactured.
func normalizeEinoNativeToolCallsIndependently(message *schema.Message, providerRequestID string) ([]CapabilityInvocation, error) {
	if message == nil || len(message.ToolCalls) == 0 {
		return []CapabilityInvocation{}, nil
	}
	result := make([]CapabilityInvocation, 0, len(message.ToolCalls))
	var firstErr error
	for index, call := range message.ToolCalls {
		raw := []any{map[string]any{
			"id":       call.ID,
			"type":     firstString(call.Type, "function"),
			"function": map[string]any{"name": call.Function.Name, "arguments": call.Function.Arguments},
		}}
		calls, err := normalizeProviderToolCalls(raw, "", providerRequestID)
		if err != nil {
			if firstErr == nil {
				firstErr = newProviderToolCallNormalizationError(index, providerToolCallNormalizationReasonFromError(err), err)
			}
			continue
		}
		for callIndex := range calls {
			calls[callIndex].Sequence = index
		}
		result = append(result, calls...)
	}
	return result, firstErr
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
			toolCalls, toolErr := einoToolCalls(calls)
			if toolErr != nil {
				return nil, fmt.Errorf("message_%d_tool_calls_invalid: %w", index, toolErr)
			}
			message.ToolCalls = toolCalls
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
			// Eino's OpenAI adapter maps UserInputMultiContent to the wire
			// MultiContent field. Content and MultiContent are mutually exclusive
			// in openai.ChatCompletionMessage; keeping the flattened text here
			// makes multimodal requests fail during JSON marshaling before they
			// reach the Provider. Text-only messages still use Content below.
			if len(multi) == 0 {
				message.Content = text
			}
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
			if len(multi) == 0 {
				message.Content = text
			}
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

// einoToolCalls decodes the already-canonical assistant/tool envelope used to
// seed a subsequent Eino request. It intentionally accepts only the formal
// id/name/arguments fields; a missing ID is not repaired from the capability
// name or request position because doing so would create a second execution
// identity outside Eino's native ToolCall channel.
func einoToolCalls(raw any) ([]schema.ToolCall, error) {
	items := toolCallArrayValue(raw)
	result := make([]schema.ToolCall, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		value := mapValue(item)
		if len(value) == 0 {
			return nil, fmt.Errorf("tool call %d must be an object", index)
		}
		function := mapValue(value["function"])
		if kind := strings.TrimSpace(stringValue(value["type"])); kind != "" && kind != "function" {
			return nil, fmt.Errorf("tool call %d type is unsupported", index)
		}
		id := strings.TrimSpace(stringValue(value["id"]))
		canonicalID := strings.TrimSpace(stringValue(value["call_id"]))
		if id != "" && canonicalID != "" && id != canonicalID {
			return nil, fmt.Errorf("tool call %d id fields disagree", index)
		}
		if id == "" {
			id = canonicalID
		}
		if id == "" {
			return nil, fmt.Errorf("tool call %d id is required", index)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("tool call id %q is duplicated", id)
		}
		seen[id] = struct{}{}
		name := strings.TrimSpace(stringValue(function["name"]))
		outerName := strings.TrimSpace(stringValue(value["name"]))
		canonicalName := strings.TrimSpace(stringValue(value["capability_name"]))
		if name != "" && outerName != "" && name != outerName {
			return nil, fmt.Errorf("tool call %q name fields disagree", id)
		}
		if outerName != "" && canonicalName != "" && outerName != canonicalName {
			return nil, fmt.Errorf("tool call %q name fields disagree", id)
		}
		if name == "" {
			name = outerName
		}
		if name == "" {
			name = canonicalName
		}
		if name == "" || len(name) > maxToolNameLength || !toolNamePattern.MatchString(name) {
			return nil, fmt.Errorf("tool call %q name is invalid", id)
		}
		arguments := function["arguments"]
		if arguments == nil {
			arguments = value["arguments"]
		}
		if arguments == nil {
			return nil, fmt.Errorf("tool call %q arguments are required", id)
		}
		normalizedArguments, argumentsErr := normalizeToolArguments(arguments)
		if argumentsErr != nil {
			return nil, fmt.Errorf("tool call %q arguments invalid: %w", id, argumentsErr)
		}
		result = append(result, schema.ToolCall{ID: id, Type: firstString(value["type"], "function"), Function: schema.FunctionCall{Name: name, Arguments: string(normalizedArguments)}})
	}
	return result, nil
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
