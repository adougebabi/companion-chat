package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	einoschema "github.com/cloudwego/eino/schema"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ProviderDatabase is the minimal database access contract needed by ProviderClient.
type ProviderDatabase interface {
	Pool() *pgxpool.Pool
}

type ProviderClient struct {
	DB          ProviderDatabase
	SettingsKey []byte
	HTTP        *http.Client
	runtime     ProviderRuntimeSupport
	queueMu     sync.Mutex
	generated   *providerQueue
	embedding   *providerQueue
	redis       redis.UniversalClient
	redisID     string
}

// SetRuntimeSupport wires the narrow persistence/diagnostics support used by
// each physical model call. It must not receive *App or a domain repository.
func (p *ProviderClient) SetRuntimeSupport(support ProviderRuntimeSupport) {
	if p != nil {
		p.runtime = support
	}
}

func (p *ProviderClient) runtimeSupport() ProviderRuntimeSupport {
	if p != nil && p.runtime != nil {
		return p.runtime
	}
	if p == nil {
		return providerRuntimeSupport{}
	}
	return newProviderRuntimeSupport(p.DB)
}

// SetRedisClient enables the optional cross-process queue coordinator. Redis
// failures intentionally fall back to the existing local queue so synchronous
// provider calls remain available during a Redis outage.
func (p *ProviderClient) SetRedisClient(client redis.UniversalClient, processID string) {
	if p == nil {
		return
	}
	p.redis = client
	p.redisID = strings.TrimSpace(processID)
}

type providerAssignment struct {
	Role                      string
	EndpointID                string
	BaseURL                   string
	ModelID                   string
	Secret                    string
	Timeout                   time.Duration
	TokenBudget               int
	ContextWindowTokens       int
	MaxInputTokens            int
	PromptBudgetPolicyVersion string
}

const (
	initializationMinimumOutputReserveTokens = 8192
	initializationMinimumRequestTimeout      = 10 * time.Minute
)

// providerAssignmentForScenario applies operation-owned floors without
// changing the persisted generic binding. Initialization emits a substantially
// larger structured document than ordinary cognition and must not depend on an
// Owner guessing the output reserve/timeout needed by the configured local
// model. The existing context-window guard remains authoritative.
func providerAssignmentForScenario(assignment providerAssignment, scenario string) (providerAssignment, error) {
	if strings.TrimSpace(scenario) != "initialization" {
		return assignment, nil
	}
	if assignment.TokenBudget < initializationMinimumOutputReserveTokens {
		if err := validatePromptBudgetConfiguration(
			assignment.ContextWindowTokens,
			assignment.MaxInputTokens,
			initializationMinimumOutputReserveTokens,
			assignment.PromptBudgetPolicyVersion,
		); err != nil {
			return assignment, errors.New("initialization_output_reserve_unavailable")
		}
		assignment.TokenBudget = initializationMinimumOutputReserveTokens
	}
	if assignment.Timeout < initializationMinimumRequestTimeout {
		assignment.Timeout = initializationMinimumRequestTimeout
	}
	return assignment, nil
}

func (p *ProviderClient) assignment(ctx context.Context, role string) (providerAssignment, error) {
	if !validProviderRole(role) {
		return providerAssignment{}, fmt.Errorf("provider role %s invalid", role)
	}
	var endpointID, baseURL, modelID, purpose, capabilityStatus string
	var timeoutSeconds, tokenBudget, contextWindowTokens, maxInputTokens int
	var promptBudgetPolicyVersion string
	bindingRole := providerBindingRole(role)
	err := p.DB.Pool().QueryRow(ctx, `
		SELECT e.id,e.base_url,r.model_id,e.secret_purpose,r.timeout_seconds,r.token_budget,e.capability_status,
		       r.context_window_tokens,r.max_input_tokens,r.prompt_budget_policy_version
		FROM public.model_roles r
		JOIN public.provider_endpoints e ON e.id = r.provider_endpoint_id
		WHERE r.role = $1 OR ($1 = 'generic_llm' AND r.role IN ('action_realization','cognitive_assessment','interaction','reflection','initialization','media_prompt'))
		ORDER BY CASE WHEN r.role = $1 THEN 0 WHEN r.role = 'action_realization' THEN 1 WHEN r.role = 'cognitive_assessment' THEN 2 WHEN r.role = 'interaction' THEN 3 WHEN r.role = 'reflection' THEN 4 WHEN r.role = 'initialization' THEN 5 ELSE 6 END
		LIMIT 1
	`, bindingRole).Scan(&endpointID, &baseURL, &modelID, &purpose, &timeoutSeconds, &tokenBudget, &capabilityStatus, &contextWindowTokens, &maxInputTokens, &promptBudgetPolicyVersion)
	if err != nil {
		return providerAssignment{}, fmt.Errorf("provider role %s unavailable: %w", bindingRole, err)
	}
	if strings.EqualFold(capabilityStatus, "failed") {
		return providerAssignment{}, fmt.Errorf("provider role %s preflight failed", bindingRole)
	}
	secret, err := p.secret(ctx, purpose)
	if err != nil {
		return providerAssignment{}, err
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}
	if err := validatePromptBudgetConfiguration(contextWindowTokens, maxInputTokens, tokenBudget, promptBudgetPolicyVersion); err != nil {
		return providerAssignment{}, err
	}
	return providerAssignment{
		Role: bindingRole, EndpointID: endpointID, BaseURL: strings.TrimRight(baseURL, "/"), ModelID: modelID,
		Secret: secret, Timeout: time.Duration(timeoutSeconds) * time.Second, TokenBudget: tokenBudget,
		ContextWindowTokens: contextWindowTokens, MaxInputTokens: maxInputTokens, PromptBudgetPolicyVersion: promptBudgetPolicyVersion,
	}, nil
}

func validProviderRole(role string) bool {
	switch role {
	case "generic_llm", "initialization", "cognitive_assessment", "action_realization", "interaction", "reflection", "embedding", "media_prompt", "visual_identity_vision", "visual_identity_patch", takeoverJudgeRole:
		return true
	default:
		return false
	}
}

func (p *ProviderClient) secret(ctx context.Context, purpose string) (string, error) {
	var ciphertext, nonce []byte
	err := p.DB.Pool().QueryRow(ctx, `SELECT ciphertext, nonce FROM public.setting_secrets WHERE purpose = $1`, purpose).Scan(&ciphertext, &nonce)
	if err != nil {
		// Providers configured without authentication are explicitly supported.
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("read provider secret: %w", err)
	}
	return decryptSecret(p.SettingsKey, purpose, nonce, ciphertext)
}

func (p *ProviderClient) complete(ctx context.Context, role string, messages []map[string]any, jsonMode bool) (map[string]any, error) {
	completion, err := p.completeWithToolsSchema(ctx, role, messages, jsonMode, nil, "", nil, role == "cognitive_assessment")
	if err != nil {
		return nil, err
	}
	if len(completion.ToolCalls) > 0 {
		return nil, errors.New("provider_tool_call_unhandled: no capability catalog is attached to this call")
	}
	if jsonMode {
		return structuredResultForRole(role, completion)
	}
	return map[string]any{"text": completion.Text}, nil
}

func structuredResultForRole(role string, completion ProviderCompletion) (map[string]any, error) {
	if completion.Structured == nil {
		return nil, errors.New("provider structured response is empty")
	}
	return completion.Structured, nil
}

// StructuredWithTools requests a structured model assessment with the
// external capability catalog. Native provider calls and JSON sidecars are
// normalized into ProviderCompletion before the application sees them. The
// request always asks for the operation's strict JSON Schema; this helper keeps
// the cognitive-assessment thinking default for callers that use
// it directly. Assembled production paths pass the flag explicitly per
// operation so bounded judgement tasks remain no-thinking.
func (p *ProviderClient) StructuredWithTools(ctx context.Context, role string, messages []map[string]any, definitions []CapabilityDefinition) (ProviderCompletion, error) {
	return p.completeWithTools(ctx, role, messages, true, definitions)
}

func (p *ProviderClient) completeWithTools(ctx context.Context, role string, messages []map[string]any, jsonMode bool, definitions []CapabilityDefinition) (ProviderCompletion, error) {
	return p.completeWithToolsSchema(ctx, role, messages, jsonMode, definitions, "", nil, role == "cognitive_assessment")
}

func (p *ProviderClient) StructuredWithSchema(ctx context.Context, role string, messages []map[string]any, schemaName string, schema map[string]any, enableThinking bool) (map[string]any, error) {
	completion, err := p.completeWithToolsSchema(ctx, role, messages, true, nil, schemaName, schema, enableThinking)
	if err != nil {
		return nil, err
	}
	if len(completion.ToolCalls) > 0 {
		return nil, errors.New("provider_tool_call_unhandled: no capability catalog is attached to this call")
	}
	if completion.Structured == nil {
		return nil, errors.New("provider structured response is empty")
	}
	return completion.Structured, nil
}

func (p *ProviderClient) StructuredWithToolsSchema(ctx context.Context, role string, messages []map[string]any, definitions []CapabilityDefinition, schemaName string, schema map[string]any, enableThinking bool) (ProviderCompletion, error) {
	return p.completeWithToolsSchema(ctx, role, messages, true, definitions, schemaName, schema, enableThinking)
}

func (p *ProviderClient) StructuredAssembledWithToolsSchema(ctx context.Context, role string, messages []map[string]any, definitions []CapabilityDefinition, schemaName string, schema map[string]any, enableThinking bool) (ProviderCompletion, error) {
	return p.completeWithToolsSchemaMode(ctx, role, messages, true, definitions, schemaName, schema, enableThinking, true)
}

// AgentAssembledCompletion is the single Provider entry used by formal
// Agents. Structured and text tasks differ only in their declared final output
// contract; both enter the same Eino Runner and physical-call queue path.
func (p *ProviderClient) AgentAssembledCompletion(ctx context.Context, role string, messages []map[string]any, definitions []CapabilityDefinition, schemaName string, schema map[string]any, structured, enableThinking bool) (ProviderCompletion, error) {
	return p.completeWithToolsSchemaMode(ctx, role, messages, structured, definitions, schemaName, schema, enableThinking, true)
}

// structuredThinkingEnabledForSchema keeps the Provider thinking policy at the
// protocol boundary. Semantic cognition and native event appraisal may use the
// model's reasoning channel; the takeover Judge keeps its output in the
// visible, strictly bounded channel.
func structuredThinkingEnabledForSchema(schemaName string) bool {
	switch strings.TrimSpace(schemaName) {
	case "conversation_turn_response", "takeover_reply_response", "persistent_switch_assessment",
		"wake_up_response", "daily_review_response", "native_cognition_response", "reflection_proposal_v2":
		return true
	default:
		return false
	}
}

// StructuredAssembledJudgement calls a dedicated judge role on the assembled
// message path. It sends no tools and omits enable_thinking so the bounded
// verdict stays in the normal content channel. The main/native cognition
// surfaces explicitly enable thinking.
func (p *ProviderClient) StructuredAssembledJudgement(ctx context.Context, role string, messages []map[string]any, schemaName string, schema map[string]any) (ProviderCompletion, error) {
	return p.completeWithToolsSchemaMode(ctx, role, messages, true, nil, schemaName, schema, false, true)
}

func (p *ProviderClient) completeWithToolsSchema(ctx context.Context, role string, messages []map[string]any, jsonMode bool, definitions []CapabilityDefinition, schemaName string, schema map[string]any, enableThinking bool) (ProviderCompletion, error) {
	return p.completeWithToolsSchemaMode(ctx, role, messages, jsonMode, definitions, schemaName, schema, enableThinking, false)
}

func (p *ProviderClient) completeWithToolsSchemaMode(ctx context.Context, role string, messages []map[string]any, jsonMode bool, definitions []CapabilityDefinition, schemaName string, schema map[string]any, enableThinking bool, assembled bool) (ProviderCompletion, error) {
	correlationID := providerCorrelation(ctx)
	if correlationID == "" {
		correlationID = diagnosticCorrelation(messages, "")
	}
	scenario := providerScenario(ctx, role, schemaName)
	ctx = WithProviderScenario(ctx, scenario)
	ctx = ensureProviderAttemptIdentity(ctx)
	assignment, err := p.assignment(ctx, role)
	if err != nil {
		p.recordProviderPreflightFailure(ctx, providerAssignment{}, role, correlationID, "assignment", messages, err)
		return ProviderCompletion{}, err
	}
	assignment, err = providerAssignmentForScenario(assignment, scenario)
	if err != nil {
		p.recordProviderPreflightFailure(ctx, assignment, role, correlationID, "scenario_budget", messages, err)
		return ProviderCompletion{}, err
	}
	if assembled {
		if !validAssembledProviderMessages(messages) {
			err := errors.New("provider_assembled_messages_invalid")
			p.recordProviderPreflightFailure(ctx, assignment, role, correlationID, "message_validation", messages, err)
			return ProviderCompletion{}, err
		}
	} else {
		messages = addVisualIdentityMediaPromptInstruction(role, messages)
		if role == "media_prompt" {
			messages = withChineseOutputInstruction(role, messages)
			messages = formatProviderMessagesForRole(messages, role)
		} else {
			messages = (&PromptComposer{}).ComposeTaskMessages(role, messages)
		}
	}
	providerRequestID := providerDiagnosticRequestID(role, correlationID)
	payload := providerChatPayloadWithSchema(assignment.ModelID, messages, assignment.TokenBudget, jsonMode, definitions, role, schemaName, schema, enableThinking)
	responseFormat := map[string]any{}
	if jsonMode {
		responseFormat = providerResponseFormatForSchema(role, schemaName, schema)
	}
	renderedTools := RenderCapabilityTools(definitions)
	wireEstimate := estimatePromptWireInput(messages, renderedTools, responseFormat)
	if role != "media_prompt" {
		if wireEstimate > assignment.MaxInputTokens {
			p.recordProviderPreflightFailure(ctx, assignment, role, correlationID, "wire_budget", messages, ErrPromptRequiredBudgetExceeded)
			return ProviderCompletion{}, ErrPromptRequiredBudgetExceeded
		}
	}
	diagnostics := providerPromptDiagnostics(ctx)
	if strings.TrimSpace(stringValue(diagnostics["run_id"])) == "" {
		// The business correlation is the parent run identity. Physical ADK
		// generations receive separate model_call_id values below.
		diagnostics["run_id"] = correlationID
	}
	diagnostics["prompt_budget"] = mergeProviderPromptBudgetDiagnostics(mapValue(diagnostics["prompt_budget"]), messages, renderedTools, responseFormat, assignment, wireEstimate)
	ctx = WithPromptDiagnostics(ctx, diagnostics)
	adkEnabled := false
	agentDefinition, formalAgent := formalAgentDefinitionFromContext(ctx)
	if _, enabled := adkCapabilityContext(ctx); enabled {
		if !formalAgent {
			err := errors.New("formal_agent_definition_required")
			p.recordProviderPreflightFailure(ctx, assignment, role, correlationID, "formal_agent", messages, err)
			return ProviderCompletion{}, err
		}
		adkEnabled = true
		// The ADK agent performs multiple model calls. Queue each call through
		// queuedToolCallingChatModel instead of holding one lease for the full
		// Agent loop.
		ctx = withProviderQueueBypass(ctx)
	}
	structuredSchema := schema
	if structuredSchema == nil {
		structuredSchema = providerSchemaForRole(role)
	}
	normalizationSchemaName := schemaName
	if normalizationSchemaName == "" {
		normalizationSchemaName = providerSchemaName(role)
	}
	_, err = json.Marshal(payload)
	if err != nil {
		p.recordProviderPreflightFailure(ctx, assignment, role, correlationID, "payload_encode", messages, err)
		return ProviderCompletion{}, err
	}
	priority := providerPriority(scenario)
	diagnosticID := ""
	if !adkEnabled {
		diagnosticID = p.runtimeSupport().RecordQueuedModelRun(ctx, role, assignment.EndpointID, assignment.ModelID, correlationID, scenario, priority, providerDiagnosticMessages(role, messages))
	}
	return runProviderQueued(p, ctx, assignment.Role, scenario, priority, diagnosticID, func(runCtx context.Context) (ProviderCompletion, error) {
		requestStarted := time.Now()
		usage := map[string]any{}
		defer func() {
			if diagnosticID != "" {
				p.runtimeSupport().UpdateModelRunPromptMetrics(ctx, diagnosticID, usage, time.Since(requestStarted))
			}
		}()
		requestCtx, cancel := context.WithTimeout(runCtx, assignment.Timeout)
		defer cancel()
		response, err := p.generateWithEino(requestCtx, EinoModelCall{
			Assignment: assignment, Role: role, Scenario: scenario, Priority: priority, DiagnosticID: diagnosticID, CorrelationID: correlationID,
			Messages: messages, Definitions: definitions,
			JSONMode: jsonMode, SchemaName: normalizationSchemaName, ResponseSchema: structuredSchema,
			EnableThinking: enableThinking, EnableStreaming: agentDefinition.EnableStreaming,
			ProviderRequestID: providerRequestID, Agent: agentDefinition,
		})
		if err != nil {
			p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, providerRunErrorCode(err))
			return ProviderCompletion{}, fmt.Errorf("provider request failed: %w", err)
		}
		usage = response.Usage
		message := einoMessageRaw(response.Message)
		finishReason := response.FinishReason
		if adkEnabled && jsonMode {
			if responseErr := validateADKStructuredResponse(response.Message, role); responseErr != nil {
				diagnostic := providerResponseDiagnostic(message, providerStructuredCandidates(message), 0)
				addStructuredParseFailureDiagnostic(diagnostic, providerStructuredCandidates(message), finishReason)
				p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, "adk_structured_response_invalid", diagnostic)
				return ProviderCompletion{}, responseErr
			}
		}
		// The Eino message is already the typed native boundary for both ADK
		// and fixed-task calls. Do not recover calls from content or sidecars;
		// valid typed siblings may survive a malformed sibling, but each accepted
		// call must retain its real Eino ID.
		callMessage := response.Message
		if adkEnabled {
			callMessage = &einoschema.Message{ToolCalls: append([]einoschema.ToolCall(nil), response.ExecutedToolCalls...)}
		}
		calls, err := normalizeEinoNativeToolCallsIndependently(callMessage, providerRequestID)
		if adkEnabled {
			if adkCtx, ok := adkCapabilityContext(ctx); ok && adkCtx.Trace != nil {
				for index := range calls {
					if identity, found := adkCtx.Trace.ModelIdentity(calls[index].CallID); found {
						calls[index].ProviderRequestID = identity.ProviderRequestID
					}
				}
			}
		}
		if err != nil {
			diagnostic := providerToolCallNormalizationDiagnostic(message["tool_calls"], "native", err)
			if len(calls) == 0 {
				p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, "tool_call_invalid", diagnostic)
				return ProviderCompletion{}, err
			}
			// Keep valid sibling calls; the malformed entry is retained only as a
			// bounded diagnostic and never becomes a second execution authority.
			logToolCallShapeNormalization(role, schemaName, "native_partial", diagnostic)
		}
		if len(calls) > 0 && len(definitions) == 0 {
			err := errors.New("provider_tool_call_unhandled: no capability catalog is attached to this call")
			p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, "tool_call_unhandled")
			return ProviderCompletion{}, err
		}
		logToolCallShapeNormalization(role, schemaName, "native", message["tool_calls"])
		content, _ := message["content"].(string)
		content = strings.TrimSpace(content)
		// Structured output is accepted only from the formal Content channel;
		// reasoning_content is diagnostics-only and can never become a DTO or
		// execution request.
		structuredCandidates := providerStructuredCandidates(message)
		var parsedStructured map[string]any
		var parsedStructuredOK bool
		var structuredParseErr error
		if jsonMode {
			parsedStructured, parsedStructuredOK, structuredParseErr = parseStructuredCandidatesForRole(role, structuredCandidates)
		}
		var structuredParseDiagnostic map[string]any
		if structuredParseErr != nil {
			diagnostic := providerResponseDiagnostic(message, structuredCandidates, len(calls))
			addStructuredParseFailureDiagnostic(diagnostic, structuredCandidates, finishReason)
			logStructuredParseFailure(role, normalizationSchemaName, diagnostic)
			errorCode := structuredParseErr.Error()
			if adkEnabled {
				errorCode = "adk_structured_response_invalid"
			}
			p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, errorCode, diagnostic)
			return ProviderCompletion{}, structuredParseErr
		}
		if adkEnabled && jsonMode && len(structuredCandidates) > 0 && !parsedStructuredOK {
			diagnostic := providerResponseDiagnostic(message, structuredCandidates, len(calls))
			addStructuredParseFailureDiagnostic(diagnostic, structuredCandidates, finishReason)
			p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, "adk_structured_response_invalid", diagnostic)
			return ProviderCompletion{}, errors.New("adk_structured_response_invalid")
		}
		if !adkEnabled && parsedStructuredOK && len(toolCallArrayValue(parsedStructured["tool_calls"])) > 0 {
			err := errors.New("provider_tool_call_unhandled: structured Content tool_calls are not an execution channel")
			p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, "tool_call_unhandled")
			return ProviderCompletion{}, err
		}
		completion := ProviderCompletion{Text: content, ToolCalls: calls, DoneSeen: true}
		if adkEnabled {
			// The Runner's last assistant message is the sole final-output
			// authority. Intermediate ToolCalls/results live in completion.ToolCalls
			// and the request trace; they never supply or repair this contract.
			if jsonMode {
				if !parsedStructuredOK {
					err := errors.New("adk_final_output_missing")
					p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, "adk_final_output_missing")
					return ProviderCompletion{}, err
				}
				if err := capabilitycontract.ValidateCapabilitySchemaValue(parsedStructured, structuredSchema); err != nil {
					p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, "adk_final_output_invalid")
					return ProviderCompletion{}, fmt.Errorf("adk_final_output_invalid: %w", err)
				}
				completion.Structured = parsedStructured
			} else if content == "" {
				err := errors.New("adk_final_text_missing")
				p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, "adk_final_text_missing")
				return ProviderCompletion{}, err
			}
			p.recordProviderSuccessBoundary(ctx, assignment, role, correlationID, messages, map[string]any{
				"text": content, "structured": completion.Structured, "executed_tool_calls": completion.ToolCalls,
			})
			return completion, nil
		}
		var normalizedFields []string
		if len(calls) > 0 {
			for index := range completion.ToolCalls {
				completion.ToolCalls[index].SourceFactID = ""
			}
			if parsedStructuredOK {
				completion.Structured, normalizedFields = normalizeProviderStructured(parsedStructured, normalizationSchemaName, structuredSchema)
				// Structured Content is a DTO channel, never a second ToolCall
				// authority. Native Eino calls remain in completion.ToolCalls.
				delete(completion.Structured, "tool_calls")
				logStructuredNormalization(role, normalizationSchemaName, normalizedFields, len(calls), len(structuredCandidates), false, message)
			} else if jsonMode {
				completion.Structured, normalizedFields = emptyProviderStructured(normalizationSchemaName, structuredSchema)
				completion.StructuredFallback = true
				logStructuredNormalization(role, normalizationSchemaName, normalizedFields, len(calls), len(structuredCandidates), true, message)
			}
			providerResponse := map[string]any{"tool_calls": completion.ToolCalls, "text": content, "structured": completion.Structured}
			if len(normalizedFields) > 0 {
				providerResponse["normalized_fields"] = normalizedFields
			}
			if structuredParseDiagnostic != nil {
				providerResponse["structured_diagnostic"] = structuredParseDiagnostic
			}
			p.recordProviderSuccessBoundary(ctx, assignment, role, correlationID, messages, providerResponse)
			return completion, nil
		}
		if len(structuredCandidates) == 0 {
			if jsonMode {
				completion.Structured, normalizedFields = emptyProviderStructured(normalizationSchemaName, structuredSchema)
				completion.StructuredFallback = true
				logStructuredNormalization(role, normalizationSchemaName, normalizedFields, 0, 0, true, message)
				p.recordProviderSuccessBoundary(ctx, assignment, role, correlationID, messages, map[string]any{"text": content, "structured": completion.Structured, "normalization": "empty"})
				return completion, nil
			}
			p.recordProviderFailureBoundary(ctx, assignment, role, correlationID, messages, "response_content_empty")
			return ProviderCompletion{}, fmt.Errorf("provider response content is empty")
		}
		if jsonMode || len(definitions) > 0 {
			if parsedStructuredOK {
				completion.Structured, normalizedFields = normalizeProviderStructured(parsedStructured, normalizationSchemaName, structuredSchema)
				if adkEnabled {
					delete(completion.Structured, "tool_calls")
				}
				logStructuredNormalization(role, normalizationSchemaName, normalizedFields, 0, len(structuredCandidates), false, message)
			} else if jsonMode {
				completion.Structured, normalizedFields = emptyProviderStructured(normalizationSchemaName, structuredSchema)
				completion.StructuredFallback = true
				logStructuredNormalization(role, normalizationSchemaName, normalizedFields, 0, len(structuredCandidates), true, message)
			}
		}
		providerResponse := map[string]any{"text": content, "structured": completion.Structured}
		if len(normalizedFields) > 0 {
			providerResponse["normalized_fields"] = normalizedFields
		}
		p.recordProviderSuccessBoundary(ctx, assignment, role, correlationID, messages, providerResponse)
		return completion, nil
	})
}

func addVisualIdentityMediaPromptInstruction(role string, messages []map[string]any) []map[string]any {
	if role != "media_prompt" {
		return messages
	}
	for _, message := range messages {
		if stringValue(message["role"]) != "user" {
			continue
		}
		content := stringValue(message["content"])
		var concept map[string]any
		if json.Unmarshal([]byte(content), &concept) != nil || stringValue(concept["purpose"]) != "visual_identity" {
			continue
		}
		stage := stringValue(concept["stage"])
		if stage == "character_sheet" {
			return prependSystemMessage(messages, map[string]any{"role": "system", "content": "This is a Visual Identity CHARACTER PROFILE card render. Follow the supplied prompt exactly: white minimalist background, premium fashion-editorial layout, 3:4 vertical composition, visual sections for basic information, front/side/back model views, six expressions, outfit breakdown, accessories, detail close-ups, color palette, character introduction, personality keywords and a signature area. Keep one consistent real human face across all views. Do not render any text, letters, numbers, titles, labels, signatures or pseudo-text; use layout, figures, diagrams, swatches and blank fields instead. Do not make it anime, chibi, an unrelated collage or a scene. Return only the final English image prompt."})
		}
		return prependSystemMessage(messages, map[string]any{"role": "system", "content": "This is the first Visual Identity CHARACTER PROFILE card render. Follow the supplied prompt exactly: white minimalist background, premium fashion-editorial layout, 3:4 vertical composition, visual sections for basic information, front/side/back model views, six expressions, outfit breakdown, accessories, detail close-ups, color palette, character introduction, personality keywords and a signature area. Keep one consistent real human face across all views. Do not render any text, letters, numbers, titles, labels, signatures or pseudo-text; use layout, figures, diagrams, swatches and blank fields instead. Do not make it anime, chibi, an unrelated collage or a scene, and do not omit the requested sections. Return only the final English image prompt."})
	}
	return messages
}

func providerStructuredContent(message map[string]any) string {
	candidates := providerStructuredCandidates(message)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func providerStructuredCandidates(message map[string]any) []string {
	if message == nil {
		return nil
	}
	content, ok := message["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		return nil
	}
	return []string{strings.TrimSpace(content)}
}

func parseStructuredCandidates(candidates []string) (map[string]any, bool) {
	for _, candidate := range candidates {
		if structured, ok := parseStructuredCandidate(candidate, 0); ok {
			return structured, true
		}
	}
	return nil, false
}

func parseStructuredCandidatesForRole(role string, candidates []string) (map[string]any, bool, error) {
	structured, ok := parseStructuredCandidates(candidates)
	if ok {
		return structured, true, nil
	}
	if len(candidates) > 0 {
		if role == "initialization" {
			return nil, false, errors.New("initialization_response_invalid_json")
		}
		return nil, false, errors.New("structured_response_invalid_json")
	}
	return nil, false, nil
}

// parseStructuredCandidate accepts only a complete JSON object from the formal
// Content channel. Reasoning, <think>, Markdown fences, encoded strings and
// trailing prose are transport guesses and are deliberately rejected.
func parseStructuredCandidate(candidate string, _ int) (map[string]any, bool) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return nil, false
	}

	var structured map[string]any
	if json.Unmarshal([]byte(candidate), &structured) == nil && structured != nil {
		return structured, true
	}
	return nil, false
}

func providerResponseDiagnostic(message map[string]any, candidates []string, toolCallCount int) map[string]any {
	result := map[string]any{
		"content_present":           false,
		"content_length":            0,
		"reasoning_content_present": false,
		"reasoning_content_length":  0,
		"candidate_count":           len(candidates),
		"candidate_lengths":         make([]any, len(candidates)),
		"tool_call_count":           toolCallCount,
		"parse_error":               "structured_response_invalid",
	}
	if message == nil {
		return result
	}
	if content, ok := message["content"].(string); ok {
		result["content_present"] = strings.TrimSpace(content) != ""
		result["content_length"] = len([]rune(content))
	}
	if reasoning, ok := message["reasoning_content"].(string); ok {
		result["reasoning_content_present"] = strings.TrimSpace(reasoning) != ""
		result["reasoning_content_length"] = len([]rune(reasoning))
	}
	lengths := result["candidate_lengths"].([]any)
	for index, candidate := range candidates {
		lengths[index] = len([]rune(candidate))
	}
	return result
}

func addStructuredParseFailureDiagnostic(diagnostic map[string]any, candidates []string, finishReason string) {
	if diagnostic == nil {
		return
	}
	finishReason = strings.TrimSpace(finishReason)
	if finishReason == "" || !validLifecycleToken(finishReason, 64, false) {
		finishReason = "unknown"
	}
	diagnostic["finish_reason"] = finishReason
	if len(candidates) == 0 {
		diagnostic["parse_error"] = "structured_response_empty"
		return
	}
	candidate := strings.TrimSpace(candidates[0])
	framing := "plain"
	if strings.Contains(candidate, "```") {
		framing = "markdown_fence"
	} else if strings.Contains(candidate, "<think>") || strings.Contains(candidate, "</think>") {
		framing = "thinking_wrapper"
	} else if strings.HasPrefix(candidate, `"`) {
		framing = "encoded_string"
	}
	diagnostic["framing"] = framing
	balanced := structuredDelimitersBalanced(candidate)
	diagnostic["delimiters_balanced"] = balanced
	if finishReason == "length" || !balanced || (framing == "markdown_fence" && strings.Count(candidate, "```")%2 != 0) {
		diagnostic["parse_error"] = "structured_response_truncated"
		return
	}
	var decoded any
	err := json.Unmarshal([]byte(candidate), &decoded)
	if err == nil {
		diagnostic["parse_error"] = "structured_root_not_object"
		return
	}
	var syntaxError *json.SyntaxError
	if errors.As(err, &syntaxError) {
		diagnostic["syntax_offset"] = int(syntaxError.Offset)
	}
	diagnostic["parse_error"] = "structured_response_invalid_json"
}

func structuredDelimitersBalanced(value string) bool {
	stack := make([]byte, 0, 8)
	inString := false
	escaped := false
	for index := 0; index < len(value); index++ {
		character := value[index]
		if inString {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '"' {
				inString = false
			}
			continue
		}
		if character == '"' {
			inString = true
			continue
		}
		switch character {
		case '{', '[':
			stack = append(stack, character)
		case '}', ']':
			if len(stack) == 0 {
				return false
			}
			expected := byte('{')
			if character == ']' {
				expected = '['
			}
			if stack[len(stack)-1] != expected {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}
	return !inString && len(stack) == 0
}

func logStructuredParseFailure(role, schemaName string, diagnostic map[string]any) {
	slog.Default().Warn("Go Core Provider structured response rejected",
		"role", role,
		"schema", schemaName,
		"parse_error", diagnostic["parse_error"],
		"finish_reason", diagnostic["finish_reason"],
		"framing", diagnostic["framing"],
		"candidate_count", diagnostic["candidate_count"],
		"candidate_lengths", diagnostic["candidate_lengths"],
		"delimiters_balanced", diagnostic["delimiters_balanced"],
		"syntax_offset", diagnostic["syntax_offset"],
		"content_present", diagnostic["content_present"],
		"content_length", diagnostic["content_length"],
		"reasoning_content_present", diagnostic["reasoning_content_present"],
		"reasoning_content_length", diagnostic["reasoning_content_length"],
	)
}

func providerChatPayload(model string, messages []map[string]any, tokenBudget int, jsonMode bool, definitions []CapabilityDefinition) map[string]any {
	return providerChatPayloadForRole(model, messages, tokenBudget, jsonMode, definitions, "")
}

func providerStreamingPayload(model string, messages []map[string]any, outputReserve int) map[string]any {
	payload := map[string]any{"model": model, "messages": messages, "temperature": 0.7, "stream": true}
	if outputReserve > 0 {
		payload["max_tokens"] = outputReserve
	}
	return payload
}

func mergeProviderPromptBudgetDiagnostics(existing map[string]any, messages, tools []map[string]any, responseFormat map[string]any, assignment providerAssignment, wireEstimate int) map[string]any {
	result := cloneMap(existing)
	if result == nil {
		result = map[string]any{}
	}
	result["policy_version"] = assignment.PromptBudgetPolicyVersion
	result["context_window_tokens"] = assignment.ContextWindowTokens
	result["max_input_tokens"] = assignment.MaxInputTokens
	result["output_reserve_tokens"] = assignment.TokenBudget
	result["safety_margin_tokens"] = defaultPromptSafetyMarginTokens
	result["estimated_input_tokens"] = wireEstimate
	sectionTokens := map[string]any{"system": 0, "runtime": 0, "recent": 0, "current_input": 0, "tools": EstimatePromptTokens(tools), "response_schema": EstimatePromptTokens(responseFormat)}
	sectionCounts := map[string]any{"system": 0, "runtime": 0, "recent": 0, "current_input": 0, "tools": len(tools), "response_schema": 0}
	if len(responseFormat) > 0 {
		sectionCounts["response_schema"] = 1
	}
	for index, message := range messages {
		section, tokens := "recent", estimateProviderMessageTokens(message)
		if index == 0 && stringValue(message["role"]) == "system" {
			section = "system"
		} else if index == len(messages)-1 && stringValue(message["role"]) == "user" {
			section = "current_input"
		} else if strings.Contains(stringValue(message["content"]), "[RUNTIME CONTEXT]") {
			section = "runtime"
		}
		sectionTokens[section] = intValue(sectionTokens[section]) + tokens
		sectionCounts[section] = intValue(sectionCounts[section]) + 1
	}
	result["section_tokens"] = sectionTokens
	result["section_counts"] = sectionCounts
	return result
}

func providerChatPayloadForRole(model string, messages []map[string]any, tokenBudget int, jsonMode bool, definitions []CapabilityDefinition, role string) map[string]any {
	// Structured cognition keeps its control object in message.content. Thinking
	// adapters may otherwise consume the output reserve in reasoning_content and
	// leave the structured decision incomplete; callers that explicitly need a
	// thinking-enabled protocol can still pass true to providerChatPayloadWithSchema.
	return providerChatPayloadWithSchema(model, messages, tokenBudget, jsonMode, definitions, role, "", nil, false)
}

func providerChatPayloadWithSchema(model string, messages []map[string]any, tokenBudget int, jsonMode bool, definitions []CapabilityDefinition, role, schemaName string, schema map[string]any, enableThinking bool) map[string]any {
	definitionList := definitions
	payload := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": 0.7,
		"stream":      false,
	}
	if len(definitionList) > 0 {
		payload["tools"] = RenderCapabilityTools(definitionList)
		payload["tool_choice"] = "auto"
		// A direct conversation may need to emit an output capability (for
		// example conversation.reply) alongside an action capability such as
		// media.image.generate in the same Main cognition. OpenAI-compatible
		// Providers otherwise commonly stop after the first native call, which
		// leaves the turn without its required visible output. Ask explicitly
		// for parallel tool calls when there is more than one registered choice;
		// a single-tool request has no parallelism to enable.
		if len(definitionList) > 1 {
			payload["parallel_tool_calls"] = true
		}
		if jsonMode {
			payload["response_format"] = providerResponseFormatForSchema(role, schemaName, schema)
		}
	} else if jsonMode {
		payload["response_format"] = providerResponseFormatForSchema(role, schemaName, schema)
	}
	if enableThinking {
		payload["enable_thinking"] = true
	}
	if tokenBudget > 0 {
		payload["max_tokens"] = tokenBudget
	}
	return payload
}

func providerResponseFormat(role string) map[string]any {
	return providerResponseFormatForSchema(role, "", nil)
}

func providerResponseFormatForSchema(role, schemaName string, schema map[string]any) map[string]any {
	if role == "initialization" {
		return map[string]any{"type": "json_object"}
	}
	if schema == nil {
		schema = providerSchemaForRole(role)
	}
	if schemaName == "" {
		schemaName = providerSchemaName(role)
	}
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   schemaName,
			"strict": true,
			"schema": schema,
		},
	}
}

func providerSchemaName(role string) string {
	if strings.TrimSpace(role) == "" {
		return "structured_response"
	}
	return strings.ReplaceAll(role, "-", "_") + "_response"
}

func providerSchemaForRole(role string) map[string]any {
	switch role {
	case "cognitive_assessment":
		return cognitiveTurnResponseSchema()
	case "initialization":
		return initializationResponseSchema()
	case "reflection":
		return reflectionProposalV2ProviderSchema()
	case takeoverJudgeRole:
		return takeoverJudgementSchema()
	case "visual_identity_vision":
		return visualIdentityVisionResponseSchema()
	case "visual_identity_patch":
		return visualIdentityPatchResponseSchema()
	default:
		return objectSchema(map[string]any{"result": openObjectSchema()}, nil, true)
	}
}

func (p *ProviderClient) recordProviderSuccess(ctx context.Context, assignment providerAssignment, role, correlationID string, messages []map[string]any, response any) {
	p.runtimeSupport().RecordModelRun(ctx, role, assignment.EndpointID, assignment.ModelID, correlationID, providerDiagnosticMessages(role, messages), providerDiagnosticResponse(role, response), "completed", "")
}

func (p *ProviderClient) recordProviderFailure(ctx context.Context, assignment providerAssignment, role, correlationID string, messages []map[string]any, code string, diagnostic ...any) {
	var response any
	if len(diagnostic) > 0 {
		response = diagnostic[0]
	}
	status := providerRunFailed
	if code == "request_timeout" {
		status = providerRunTimeout
	} else if code == "request_cancelled" {
		status = providerRunCancelled
	}
	diagnosticCtx, cancel := boundedDiagnosticWriteContext(ctx)
	defer cancel()
	p.runtimeSupport().RecordModelRun(diagnosticCtx, role, assignment.EndpointID, assignment.ModelID, correlationID, providerDiagnosticMessages(role, messages), providerDiagnosticResponse(role, response), status, code)
}

func (p *ProviderClient) recordProviderSuccessBoundary(ctx context.Context, assignment providerAssignment, role, correlationID string, messages []map[string]any, response any) {
	// ADK uses several low-level model calls, but each completed call still
	// needs a response snapshot in Diagnostics. The ADK capability trace owns
	// execution semantics; this record is observability only and is safe to
	// persist as a separate model-run row.
	p.recordProviderSuccess(ctx, assignment, role, correlationID, messages, response)
}

func (p *ProviderClient) recordProviderFailureBoundary(ctx context.Context, assignment providerAssignment, role, correlationID string, messages []map[string]any, code string, diagnostic ...any) {
	p.recordProviderFailure(ctx, assignment, role, correlationID, messages, code, diagnostic...)
}

func providerDiagnosticMessages(role string, messages []map[string]any) []map[string]any {
	if role != "initialization" {
		return messages
	}
	return []map[string]any{{"role": "diagnostic", "content": providerPreflightDiagnosticPrompt(messages)}}
}

func providerDiagnosticResponse(role string, response any) any {
	if role != "initialization" {
		return response
	}
	encoded, _ := json.Marshal(response)
	result := map[string]any{"diagnostic_scope": "metadata_only", "response_bytes": len(encoded), "response_digest": stableDigest(string(encoded))}
	diagnostic := mapValue(response)
	for _, key := range []string{
		"content_present", "content_length", "reasoning_content_present", "reasoning_content_length",
		"candidate_count", "tool_call_count", "delimiters_balanced", "syntax_offset",
		"call_count", "failed_item_index", "id_present", "name_present", "name_length", "name_valid",
		"arguments_present", "arguments_length",
	} {
		switch value := diagnostic[key].(type) {
		case bool:
			result[key] = value
		case int:
			result[key] = value
		case int64:
			result[key] = value
		case float64:
			if value >= 0 {
				result[key] = value
			}
		}
	}
	for _, key := range []string{"parse_error", "finish_reason", "framing", "source", "value_shape", "item_shape", "function_shape", "id_type", "type_value", "arguments_shape", "normalization_reason"} {
		if value := strings.TrimSpace(stringValue(diagnostic[key])); validLifecycleToken(value, 128, false) && value != "" {
			result[key] = value
		}
	}
	lengths := arrayValue(diagnostic["candidate_lengths"])
	if len(lengths) > 4 {
		lengths = lengths[:4]
	}
	if len(lengths) > 0 {
		result["candidate_lengths"] = lengths
	}
	return result
}

func (p *ProviderClient) recordProviderPreflightFailure(ctx context.Context, assignment providerAssignment, role, correlationID, stage string, messages []map[string]any, preflightErr error) {
	if p == nil || p.DB == nil || preflightErr == nil {
		return
	}
	stage = strings.TrimSpace(stage)
	category, code, retryable := classifyProviderPreflightError(stage, preflightErr)
	support := p.runtimeSupport()
	scenario := providerScenario(ctx, role, "")
	support.RecordDiagnosticEvent(ctx, "provider.preflight.failed", "error", strings.TrimSpace(stringValue(providerPromptDiagnostics(ctx)["fluctlight_id"])), "", correlationID, map[string]any{
		"role": role, "scenario": scenario, "stage": stage,
		"error_category": category, "error_code": code, "error_type": fmt.Sprintf("%T", preflightErr), "retryable": retryable,
		"provider_attempt_id": providerAttemptIdentity(ctx),
	})
	if strings.TrimSpace(assignment.EndpointID) != "" && strings.TrimSpace(assignment.ModelID) != "" {
		support.RecordModelRun(ctx, role, assignment.EndpointID, assignment.ModelID, correlationID, providerPreflightDiagnosticPrompt(messages), map[string]any{
			"stage": stage, "error_type": fmt.Sprintf("%T", preflightErr), "retryable": retryable,
		}, providerRunFailed, code)
	}
	if scenario == "wake_up" || scenario == "reflection" {
		support.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
			Surface: scenario, Transition: LifecycleTransitionFailed, Severity: "error",
			FluctlightID:  strings.TrimSpace(stringValue(providerPromptDiagnostics(ctx)["fluctlight_id"])),
			CorrelationID: correlationID, ProviderRequestID: providerDiagnosticRequestID(role, correlationID),
			ProviderAttemptID: providerAttemptIdentity(ctx),
			Stage:             "provider_preflight", Status: "failed", ReasonCode: code,
			ErrorCategory: category, ErrorCode: code, Retryable: retryable,
			SafeCause: preflightErr.Error(),
		})
	}
}

func classifyProviderPreflightError(stage string, preflightErr error) (category, code string, retryable bool) {
	message := strings.ToLower(strings.TrimSpace(preflightErr.Error()))
	switch {
	case errors.Is(preflightErr, context.Canceled):
		return "request", "provider_request_cancelled", false
	case errors.Is(preflightErr, context.DeadlineExceeded):
		return "infrastructure", "provider_store_timeout", true
	case errors.Is(preflightErr, ErrPromptRequiredBudgetExceeded):
		return "budget", "prompt_required_budget_exceeded", false
	case errors.Is(preflightErr, pgx.ErrNoRows):
		return "configuration", "provider_role_unassigned", false
	case strings.Contains(message, "provider role") && strings.Contains(message, " invalid"):
		return "configuration", "provider_role_invalid", false
	case strings.Contains(message, "preflight failed"):
		return "configuration", "provider_role_preflight_failed", false
	case strings.Contains(message, "prompt_budget") || strings.Contains(message, "prompt budget"):
		return "configuration", "provider_prompt_budget_invalid", false
	case strings.Contains(message, "secret") || strings.Contains(message, "decrypt"):
		return "configuration", "provider_secret_unavailable", false
	case stage == "message_validation":
		return "validation", "provider_assembled_messages_invalid", false
	case stage == "payload_encode":
		return "encoding", "provider_payload_encode_failed", false
	case stage == "assignment":
		return "infrastructure", "provider_store_unavailable", true
	default:
		return "provider", "provider_preflight_failed", false
	}
}

func providerPreflightDiagnosticPrompt(messages []map[string]any) map[string]any {
	encoded, _ := json.Marshal(messages)
	return map[string]any{
		"diagnostic_scope":       "metadata_only",
		"message_count":          len(messages),
		"estimated_input_tokens": EstimatePromptTokens(messages),
		"prompt_digest":          stableDigest(string(encoded)),
	}
}

func (p *ProviderClient) Structured(ctx context.Context, role string, messages []map[string]any) (map[string]any, error) {
	return p.complete(ctx, role, messages, true)
}

func (p *ProviderClient) Text(ctx context.Context, role string, messages []map[string]any) (string, error) {
	value, err := p.complete(ctx, role, messages, false)
	if err != nil {
		return "", err
	}
	text, _ := value["text"].(string)
	return strings.TrimSpace(text), nil
}

// StreamText preserves the action-realization streaming contract for callers
// that need incremental visible output. The callback is invoked for each
// provider delta and the accumulated text is returned only after [DONE].
func (p *ProviderClient) StreamText(ctx context.Context, role string, messages []map[string]any, onChunk func(string) error) (string, error) {
	if role != "action_realization" {
		return "", errors.New("provider streaming is only available for action_realization")
	}
	correlationID := providerCorrelation(ctx)
	if correlationID == "" {
		correlationID = diagnosticCorrelation(messages, "")
	}
	ctx = ensureProviderAttemptIdentity(ctx)
	assignment, err := p.assignment(ctx, role)
	if err != nil {
		p.recordProviderPreflightFailure(ctx, providerAssignment{}, role, correlationID, "assignment", messages, err)
		return "", err
	}
	messages = (&PromptComposer{}).ComposeTaskMessages(role, messages)
	providerRequestID := providerDiagnosticRequestID(role, correlationID)
	payload := providerStreamingPayload(assignment.ModelID, messages, assignment.TokenBudget)
	wireEstimate := estimatePromptWireInput(messages, nil, nil)
	if wireEstimate > assignment.MaxInputTokens {
		p.recordProviderPreflightFailure(ctx, assignment, role, correlationID, "wire_budget", messages, ErrPromptRequiredBudgetExceeded)
		return "", ErrPromptRequiredBudgetExceeded
	}
	ctx = WithPromptDiagnostics(ctx, map[string]any{"prompt_budget": mergeProviderPromptBudgetDiagnostics(nil, messages, nil, nil, assignment, wireEstimate)})
	_, err = json.Marshal(payload)
	if err != nil {
		p.recordProviderPreflightFailure(ctx, assignment, role, correlationID, "payload_encode", messages, err)
		return "", err
	}
	scenario := providerScenario(ctx, role, "")
	priority := providerPriority(scenario)
	ctx = WithProviderScenario(ctx, scenario)
	diagnosticID := p.runtimeSupport().RecordQueuedModelRun(ctx, role, assignment.EndpointID, assignment.ModelID, correlationID, scenario, priority, messages)
	return runProviderQueued(p, ctx, assignment.Role, scenario, priority, diagnosticID, func(runCtx context.Context) (string, error) {
		requestStarted := time.Now()
		defer func() {
			p.runtimeSupport().UpdateModelRunPromptMetrics(ctx, diagnosticID, map[string]any{}, time.Since(requestStarted))
		}()
		requestCtx, cancel := context.WithTimeout(runCtx, assignment.Timeout)
		defer cancel()
		result, err := p.streamWithEino(requestCtx, assignment, messages, providerRequestID, onChunk)
		if err != nil {
			return "", err
		}
		p.recordProviderSuccess(ctx, assignment, role, correlationID, messages, map[string]any{"text": result, "streamed": true})
		return result, nil
	})
}

func (p *ProviderClient) Embed(ctx context.Context, text string) (string, []float64, error) {
	correlationID := "embedding:" + stableDigest(text)
	ctx = ensureProviderAttemptIdentity(ctx)
	assignment, err := p.assignment(ctx, "embedding")
	if err != nil {
		p.recordProviderPreflightFailure(ctx, providerAssignment{}, "embedding", correlationID, "assignment", nil, err)
		return "", nil, err
	}
	return p.embedWithAssignment(ctx, text, assignment)
}

func (p *ProviderClient) embedWithAssignment(ctx context.Context, text string, assignment providerAssignment) (string, []float64, error) {
	if assignment.Role != "embedding" || strings.TrimSpace(assignment.EndpointID) == "" || strings.TrimSpace(assignment.ModelID) == "" || strings.TrimSpace(assignment.BaseURL) == "" {
		return "", nil, errors.New("embedding_assignment_invalid")
	}
	correlationID := "embedding:" + stableDigest(assignment.ModelID+":"+text)
	ctx = ensureProviderAttemptIdentity(ctx)
	providerRequestID := providerDiagnosticRequestID("embedding", correlationID)
	prompt := []map[string]any{{"role": "user", "content": text}}
	scenario := providerScenario(ctx, "embedding", "")
	diagnosticID := p.runtimeSupport().RecordQueuedModelRun(ctx, "embedding", assignment.EndpointID, assignment.ModelID, correlationID, scenario, 0, prompt)
	queuedResult, queuedErr := runProviderQueued(p, ctx, assignment.Role, scenario, 0, diagnosticID, func(runCtx context.Context) (struct {
		model  string
		vector []float64
	}, error) {
		requestCtx, cancel := context.WithTimeout(runCtx, assignment.Timeout)
		defer cancel()
		factory := NewEinoModelFactory(p.HTTP)
		embedHTTP := einoHTTPClientWithHeaders(p.HTTP, map[string]string{
			"Idempotency-Key": providerRequestID, "X-Fluctlight-Provider-Request-Id": providerRequestID,
		})
		embedder, err := factory.NewEmbedder(requestCtx, EinoModelConfig{
			APIKey: assignment.Secret, BaseURL: assignment.BaseURL, Model: assignment.ModelID,
			Timeout: assignment.Timeout, HTTPClient: embedHTTP,
		})
		if err != nil {
			p.recordProviderFailure(runCtx, assignment, "embedding", correlationID, prompt, providerRunErrorCode(err))
			return struct {
				model  string
				vector []float64
			}{}, err
		}
		vectors, err := embedder.EmbedStrings(requestCtx, []string{text})
		if err != nil {
			p.recordProviderFailure(runCtx, assignment, "embedding", correlationID, prompt, providerRunErrorCode(err))
			return struct {
				model  string
				vector []float64
			}{}, err
		}
		if len(vectors) == 0 || len(vectors[0]) == 0 {
			p.recordProviderFailure(runCtx, assignment, "embedding", correlationID, prompt, "embedding_response_empty")
			return struct {
				model  string
				vector []float64
			}{}, fmt.Errorf("embedding response is empty")
		}
		p.recordProviderSuccess(runCtx, assignment, "embedding", correlationID, prompt, map[string]any{"dimensions": len(vectors[0])})
		return struct {
			model  string
			vector []float64
		}{model: assignment.ModelID, vector: vectors[0]}, nil
	})
	return queuedResult.model, queuedResult.vector, queuedErr
}

func (p *ProviderClient) embeddingAssignmentByID(ctx context.Context, endpointID, modelID string) (providerAssignment, error) {
	endpointID = strings.TrimSpace(endpointID)
	modelID = strings.TrimSpace(modelID)
	if endpointID == "" || modelID == "" {
		return providerAssignment{}, errors.New("embedding_assignment_identity_invalid")
	}
	var baseURL, purpose, capabilityStatus string
	if err := p.DB.Pool().QueryRow(ctx, `SELECT base_url,secret_purpose,capability_status FROM public.provider_endpoints WHERE id=$1`, endpointID).Scan(&baseURL, &purpose, &capabilityStatus); err != nil {
		return providerAssignment{}, err
	}
	if strings.EqualFold(capabilityStatus, "failed") {
		return providerAssignment{}, errors.New("embedding_assignment_preflight_failed")
	}
	secret, err := p.secret(ctx, purpose)
	if err != nil {
		return providerAssignment{}, err
	}
	return providerAssignment{Role: "embedding", EndpointID: endpointID, BaseURL: strings.TrimRight(baseURL, "/"), ModelID: modelID, Secret: secret, Timeout: 120 * time.Second}, nil
}
