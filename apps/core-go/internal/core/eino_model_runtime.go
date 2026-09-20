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
	// stopOnDeferredToolRound is enabled only for the bounded ADK bridge. A
	// mutation/output capability returns a deferred result because Core owns its
	// later settlement; asking the provider for another cognition round cannot
	// add semantic information and would turn one action turn into a second
	// physical Provider request. Pure-query results remain completed and still
	// trigger the normal ADK continuation.
	stopOnDeferredToolRound bool
	// Conversation and takeover replies still need a second model round when
	// the first action-only proposal carried no visible text. WakeUp may settle
	// an action-only proposal without synthesis; its policy sets this false.
	requireVisibleTextForDeferredStop bool
}

func (m *queuedToolCallingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	input = normalizeEinoToolMessageNames(input)
	if m.stopOnDeferredToolRound {
		if terminal, ok := deferredToolOnlyRoundMessage(input, m.requireVisibleTextForDeferredStop); ok {
			return terminal, nil
		}
	}
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
		callDiagnosticID = m.provider.runtimeSupport().RecordQueuedModelRun(callCtx, m.role, m.assignment.EndpointID, m.assignment.ModelID, callCorrelation, m.scenario, m.priority, einoDiagnosticMessages(input))
	}
	started := time.Now()
	result, err := runProviderQueued(m.provider, callCtx, m.role, m.scenario, m.priority, callDiagnosticID, func(runCtx context.Context) (*schema.Message, error) {
		return m.inner.Generate(runCtx, input, opts...)
	})
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
	callDiagnosticID := ""
	if m.provider != nil && m.provider.DB != nil {
		callDiagnosticID = m.provider.runtimeSupport().RecordQueuedModelRun(callCtx, m.role, m.assignment.EndpointID, m.assignment.ModelID, callCorrelation, m.scenario, m.priority, einoDiagnosticMessages(input))
	}
	return runProviderQueuedStream(m.provider, callCtx, m.role, m.scenario, m.priority, callDiagnosticID, func(runCtx context.Context) (*schema.StreamReader[*schema.Message], error) {
		return m.inner.Stream(runCtx, input, opts...)
	})
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
	return &queuedToolCallingChatModel{inner: bound, provider: m.provider, role: m.role, scenario: m.scenario, priority: m.priority, diagnosticID: m.diagnosticID, assignment: m.assignment, correlationID: m.correlationID, sequence: sequence, stopOnDeferredToolRound: m.stopOnDeferredToolRound, requireVisibleTextForDeferredStop: m.requireVisibleTextForDeferredStop}, nil
}

// deferredToolOnlyRoundMessage returns the previous assistant tool proposal
// when every tool result in the immediately following round is deferred or
// rejected. Such a round is already complete from Core's perspective: there is
// no query result for the model to incorporate, and the action/output call is
// frozen for later settlement. Returning the proposal lets ADK close the loop
// without a second physical Provider request while keeping the canonical call
// identity and trace intact.
func deferredToolOnlyRoundMessage(messages []*schema.Message, requireVisibleText bool) (*schema.Message, bool) {
	assistantIndex := -1
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if message != nil && message.Role == schema.Assistant && len(message.ToolCalls) > 0 {
			assistantIndex = index
			break
		}
	}
	if assistantIndex < 0 {
		return nil, false
	}
	hasToolResult := false
	for _, message := range messages[assistantIndex+1:] {
		if message == nil {
			continue
		}
		if message.Role != schema.Tool {
			return nil, false
		}
		hasToolResult = true
		var envelope map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(message.Content)), &envelope); err != nil {
			return nil, false
		}
		status := strings.TrimSpace(stringValue(envelope["status"]))
		if status != "deferred" && status != "rejected" {
			return nil, false
		}
	}
	if !hasToolResult {
		return nil, false
	}
	proposal := messages[assistantIndex]
	if requireVisibleText && !assistantMessageDeclaresVisibleText(proposal) {
		return nil, false
	}
	return schema.AssistantMessage(proposal.Content, append([]schema.ToolCall(nil), proposal.ToolCalls...)), true
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
	chat = &queuedToolCallingChatModel{
		inner: chat, provider: p, role: call.Role, scenario: call.Scenario, priority: call.Priority,
		diagnosticID: call.DiagnosticID, assignment: call.Assignment, correlationID: call.CorrelationID,
		sequence: &atomic.Uint64{}, stopOnDeferredToolRound: true,
		requireVisibleTextForDeferredStop: call.SchemaName == "conversation_turn_response" || call.SchemaName == takeoverReplySchemaName,
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
		Name: "fluctlight-conversation", Description: "Fluctlight bounded model and capability runtime",
		Model: chat, Tools: tools, MaxIterations: 2,
		ToolOnlyTermination: func(calls []schema.ToolCall) bool {
			if len(calls) == 0 {
				return false
			}
			definitions := make(map[string]CapabilityDefinition, len(call.Definitions))
			for _, definition := range call.Definitions {
				definitions[definition.Name] = definition
			}
			for _, toolCall := range calls {
				definition, ok := definitions[toolCall.Function.Name]
				if !ok {
					return false
				}
				if definition.Type == CapabilityTypeQuery && definition.SideEffectClass == "read_only" {
					return false
				}
			}
			return true
		},
	}, input)
	if err != nil {
		return einoModelResponse{}, err
	}
	if result.FinalMessage == nil {
		return einoModelResponse{}, errors.New("adk_final_message_missing")
	}
	final := result.FinalMessage
	// Intermediate ADK generations may use a deferred conversation.reply as a
	// tool round before producing the authoritative structured final reply. That
	// intermediate output proposal is not a second visible-text source: keeping
	// it on the final ProviderCompletion would make the normalizer reject a
	// valid final root text as a false source conflict. Preserve other
	// intermediate capability calls (for example pure queries), and retain a
	// conversation.reply only when it belongs to the final assistant event.
	finalRoundIDs := make(map[string]struct{}, len(final.ToolCalls))
	for _, call := range final.ToolCalls {
		if strings.TrimSpace(call.ID) != "" {
			finalRoundIDs[call.ID] = struct{}{}
		}
	}
	// Keep track of whether a non-final assistant round already carried a root
	// visible_text proposal.  That distinction matters for F-01: a reply call
	// from an empty intermediate tool round is only an implementation detail,
	// while a reply call in the same round as root visible_text is a genuine
	// second proposal and must remain available to the conflict validator.
	replyCallRootVisibleText := make(map[string]bool)
	for _, message := range result.Messages {
		if message == nil || message.Role != schema.Assistant || len(message.ToolCalls) == 0 {
			continue
		}
		for _, toolCall := range message.ToolCalls {
			if toolCall.Function.Name == conversationReplyCapabilityName && strings.TrimSpace(toolCall.ID) != "" {
				replyCallRootVisibleText[toolCall.ID] = assistantMessageDeclaresVisibleText(message)
			}
		}
	}
	finalDeclaresVisibleText := assistantMessageDeclaresVisibleText(final)
	completionCalls := make([]schema.ToolCall, 0, len(result.ToolCalls))
	for _, call := range result.ToolCalls {
		if finalDeclaresVisibleText && call.Function.Name == conversationReplyCapabilityName {
			if _, finalRound := finalRoundIDs[call.ID]; !finalRound {
				if !replyCallRootVisibleText[call.ID] {
					continue
				}
			}
		}
		completionCalls = append(completionCalls, call)
	}
	final.ToolCalls = completionCalls
	filterADKTraceToToolCalls(adkContext.Trace, completionCalls)
	return einoModelResponse{Message: final, Usage: einoUsage(final), FinishReason: "stop"}, nil
}

func filterADKTraceToToolCalls(trace *ADKCapabilityTrace, calls []schema.ToolCall) {
	if trace == nil {
		return
	}
	allowed := make(map[string]struct{}, len(calls))
	for _, call := range calls {
		if strings.TrimSpace(call.ID) != "" {
			allowed[call.ID] = struct{}{}
		}
	}
	// Keep rejected calls even when they came from an intermediate ADK round.
	// They are not visible-output authorities, but dropping them would erase the
	// evidence needed by the frozen candidate-invalid gate.
	rejected := make(map[string]struct{})
	for _, result := range trace.Results {
		if strings.TrimSpace(result.CallID) != "" && (result.ErrorCode != "" || result.Status == "rejected") {
			rejected[result.CallID] = struct{}{}
		}
	}
	invocations := trace.Invocations[:0]
	for _, invocation := range trace.Invocations {
		if _, ok := allowed[invocation.CallID]; ok {
			invocations = append(invocations, invocation)
			continue
		}
		if _, rejectedCall := rejected[invocation.CallID]; rejectedCall {
			invocations = append(invocations, invocation)
		}
	}
	trace.Invocations = invocations
	results := trace.Results[:0]
	for _, result := range trace.Results {
		if _, ok := allowed[result.CallID]; ok {
			results = append(results, result)
			continue
		}
		if _, rejectedCall := rejected[result.CallID]; rejectedCall {
			results = append(results, result)
		}
	}
	trace.Results = results
}

// assistantMessageDeclaresVisibleText recognizes only a root/response-plan
// visible-text proposal.  A structured object that contains no such field is
// a control/result envelope and must not suppress an intermediate
// conversation.reply capability.
func assistantMessageDeclaresVisibleText(message *schema.Message) bool {
	if message == nil {
		return false
	}
	for _, candidate := range []string{message.Content, message.ReasoningContent} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if structured, ok := parseStructuredCandidate(candidate, 0); ok {
			if normalizeVisibleReply(stringValue(structured["visible_text"])) != "" {
				return true
			}
			plan := mapValue(structured["response_plan"])
			if normalizeVisibleReply(stringValue(plan["visible_text"])) != "" {
				return true
			}
			continue
		}
		// Non-JSON assistant content is a normal visible response.
		return true
	}
	return false
}

// mergeADKTraceInvocations joins calls retained by the final assistant event
// with rejected calls preserved from intermediate ADK rounds. The completion's
// call order remains authoritative; trace-only calls are appended once by their
// formal call ID so they can still pass through the candidate validator.
func mergeADKTraceInvocations(completion []CapabilityInvocation, trace *ADKCapabilityTrace) []CapabilityInvocation {
	if trace == nil || len(trace.Invocations) == 0 {
		return completion
	}
	result := append([]CapabilityInvocation(nil), completion...)
	seen := make(map[string]struct{}, len(result)+len(trace.Invocations))
	for _, invocation := range result {
		if id := strings.TrimSpace(invocation.CallID); id != "" {
			seen[id] = struct{}{}
		}
	}
	for _, invocation := range trace.Invocations {
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

func einoToolCalls(raw any) []schema.ToolCall {
	items := arrayValue(raw)
	result := make([]schema.ToolCall, 0, len(items))
	for _, item := range items {
		value := mapValue(item)
		function := mapValue(value["function"])
		name := firstString(stringValue(function["name"]), firstString(value["name"], stringValue(value["capability_name"])))
		args := einoToolCallArguments(function["arguments"])
		if args == "" {
			args = einoToolCallArguments(value["arguments"])
		}
		result = append(result, schema.ToolCall{ID: firstString(value["id"], stringValue(value["call_id"])), Type: firstString(value["type"], "function"), Function: schema.FunctionCall{Name: name, Arguments: args}})
	}
	return result
}

func einoToolCallArguments(value any) string {
	if value == nil {
		return ""
	}
	if text := strings.TrimSpace(stringValue(value)); text != "" {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
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
