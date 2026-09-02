package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ProviderClient struct {
	DB          *PostgresRepository
	SettingsKey []byte
	HTTP        *http.Client
}

type providerAssignment struct {
	Role        string
	EndpointID  string
	BaseURL     string
	ModelID     string
	Secret      string
	Timeout     time.Duration
	TokenBudget int
}

func (p *ProviderClient) assignment(ctx context.Context, role string) (providerAssignment, error) {
	var endpointID, baseURL, modelID, purpose, capabilityStatus string
	var timeoutSeconds, tokenBudget int
	err := p.DB.Pool().QueryRow(ctx, `
		SELECT e.id,e.base_url, r.model_id, e.secret_purpose, r.timeout_seconds,r.token_budget,e.capability_status
		FROM public.model_roles r
		JOIN public.provider_endpoints e ON e.id = r.provider_endpoint_id
		WHERE r.role = $1
	`, role).Scan(&endpointID, &baseURL, &modelID, &purpose, &timeoutSeconds, &tokenBudget, &capabilityStatus)
	if err != nil {
		return providerAssignment{}, fmt.Errorf("provider role %s unavailable: %w", role, err)
	}
	if !validProviderRole(role) {
		return providerAssignment{}, fmt.Errorf("provider role %s invalid", role)
	}
	if strings.EqualFold(capabilityStatus, "failed") {
		return providerAssignment{}, fmt.Errorf("provider role %s preflight failed", role)
	}
	secret, err := p.secret(ctx, purpose)
	if err != nil {
		return providerAssignment{}, err
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}
	return providerAssignment{Role: role, EndpointID: endpointID, BaseURL: strings.TrimRight(baseURL, "/"), ModelID: modelID, Secret: secret, Timeout: time.Duration(timeoutSeconds) * time.Second, TokenBudget: tokenBudget}, nil
}

func validProviderRole(role string) bool {
	switch role {
	case "initialization", "cognitive_assessment", "action_realization", "reflection", "embedding", "media_prompt":
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
	if jsonMode {
		if completion.Structured == nil {
			return nil, errors.New("provider structured response is empty")
		}
		return completion.Structured, nil
	}
	return map[string]any{"text": completion.Text}, nil
}

// StructuredWithTools requests a structured model assessment with the
// external capability catalog. Native provider calls and JSON sidecars are
// normalized into ProviderCompletion before the application sees them. The
// request always asks for the operation's strict JSON Schema; the cognitive
// assessment default additionally enables the provider's thinking mode.
func (p *ProviderClient) StructuredWithTools(ctx context.Context, role string, messages []map[string]any, manifests []CapabilityManifest) (ProviderCompletion, error) {
	return p.completeWithTools(ctx, role, messages, true, manifests)
}

func (p *ProviderClient) completeWithTools(ctx context.Context, role string, messages []map[string]any, jsonMode bool, manifests []CapabilityManifest) (ProviderCompletion, error) {
	return p.completeWithToolsSchema(ctx, role, messages, jsonMode, manifests, "", nil, role == "cognitive_assessment")
}

func (p *ProviderClient) StructuredWithSchema(ctx context.Context, role string, messages []map[string]any, schemaName string, schema map[string]any, enableThinking bool) (map[string]any, error) {
	completion, err := p.completeWithToolsSchema(ctx, role, messages, true, nil, schemaName, schema, enableThinking)
	if err != nil {
		return nil, err
	}
	if completion.Structured == nil {
		return nil, errors.New("provider structured response is empty")
	}
	return completion.Structured, nil
}

func (p *ProviderClient) StructuredWithToolsSchema(ctx context.Context, role string, messages []map[string]any, manifests []CapabilityManifest, schemaName string, schema map[string]any, enableThinking bool) (ProviderCompletion, error) {
	return p.completeWithToolsSchema(ctx, role, messages, true, manifests, schemaName, schema, enableThinking)
}

func (p *ProviderClient) completeWithToolsSchema(ctx context.Context, role string, messages []map[string]any, jsonMode bool, manifests []CapabilityManifest, schemaName string, schema map[string]any, enableThinking bool) (ProviderCompletion, error) {
	assignment, err := p.assignment(ctx, role)
	if err != nil {
		return ProviderCompletion{}, err
	}
	correlationID := diagnosticCorrelation(messages, "")
	providerRequestID := "provider:" + stableDigest(role+":"+correlationID)
	payload := providerChatPayloadWithSchema(assignment.ModelID, messages, assignment.TokenBudget, jsonMode, manifests, role, schemaName, schema, enableThinking)
	structuredSchema := schema
	if structuredSchema == nil {
		structuredSchema = providerSchemaForRole(role)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ProviderCompletion{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, assignment.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, assignment.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ProviderCompletion{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", providerRequestID)
	request.Header.Set("X-Fluctlight-Provider-Request-Id", providerRequestID)
	if assignment.Secret != "" {
		request.Header.Set("Authorization", "Bearer "+assignment.Secret)
	}
	client := p.HTTP
	if client == nil {
		client = &http.Client{}
	}
	response, err := client.Do(request)
	if err != nil {
		p.recordProviderFailure(ctx, assignment, correlationID, messages, err.Error())
		return ProviderCompletion{}, fmt.Errorf("provider request failed: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		p.recordProviderFailure(ctx, assignment, correlationID, messages, err.Error())
		return ProviderCompletion{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		p.recordProviderFailure(ctx, assignment, correlationID, messages, fmt.Sprintf("http_%d", response.StatusCode))
		return ProviderCompletion{}, fmt.Errorf("provider request returned HTTP %d", response.StatusCode)
	}
	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		p.recordProviderFailure(ctx, assignment, correlationID, messages, "response_not_json")
		return ProviderCompletion{}, fmt.Errorf("provider response is not JSON: %w", err)
	}
	choices, ok := envelope["choices"].([]any)
	if !ok || len(choices) == 0 {
		p.recordProviderFailure(ctx, assignment, correlationID, messages, "response_no_choices")
		return ProviderCompletion{}, fmt.Errorf("provider response has no choices")
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		p.recordProviderFailure(ctx, assignment, correlationID, messages, "response_choice_invalid")
		return ProviderCompletion{}, fmt.Errorf("provider response choice is invalid")
	}
	message, ok := choice["message"].(map[string]any)
	if !ok {
		p.recordProviderFailure(ctx, assignment, correlationID, messages, "response_message_invalid")
		return ProviderCompletion{}, fmt.Errorf("provider response message is invalid")
	}
	calls, err := NormalizeProviderToolCalls(message["tool_calls"], "", providerRequestID)
	if err != nil {
		p.recordProviderFailure(ctx, assignment, correlationID, messages, "tool_call_invalid")
		return ProviderCompletion{}, err
	}
	logToolCallShapeNormalization(role, schemaName, "native", message["tool_calls"])
	content, _ := message["content"].(string)
	content = strings.TrimSpace(content)
	// mlx-serve places structured JSON in reasoning_content when thinking is
	// enabled, while leaving message.content empty. Treat that field as a
	// structured control channel only; it is never exposed as visible text.
	structuredCandidates := providerStructuredCandidates(message)
	completion := ProviderCompletion{Text: content, ToolCalls: calls, DoneSeen: true}
	var normalizedFields []string
	if len(calls) > 0 {
		for index := range completion.ToolCalls {
			completion.ToolCalls[index].SourceFactID = ""
		}
		if structured, ok := parseStructuredCandidates(structuredCandidates); ok {
			completion.Structured, normalizedFields = normalizeProviderStructured(structured, schemaName, structuredSchema)
			logStructuredNormalization(role, schemaName, normalizedFields, len(calls), len(structuredCandidates), false, message)
		} else if jsonMode {
			completion.Structured, normalizedFields = emptyProviderStructured(schemaName, structuredSchema)
			completion.StructuredFallback = true
			logStructuredNormalization(role, schemaName, normalizedFields, len(calls), len(structuredCandidates), true, message)
		}
		providerResponse := map[string]any{"tool_calls": completion.ToolCalls, "text": content, "structured": completion.Structured}
		if len(normalizedFields) > 0 {
			providerResponse["normalized_fields"] = normalizedFields
		}
		p.recordProviderSuccess(ctx, assignment, correlationID, messages, providerResponse)
		return completion, nil
	}
	if len(structuredCandidates) == 0 {
		if jsonMode {
			completion.Structured, normalizedFields = emptyProviderStructured(schemaName, structuredSchema)
			completion.StructuredFallback = true
			logStructuredNormalization(role, schemaName, normalizedFields, 0, 0, true, message)
			p.recordProviderSuccess(ctx, assignment, correlationID, messages, map[string]any{"text": content, "structured": completion.Structured, "normalization": "empty"})
			return completion, nil
		}
		p.recordProviderFailure(ctx, assignment, correlationID, messages, "response_content_empty")
		return ProviderCompletion{}, fmt.Errorf("provider response content is empty")
	}
	if jsonMode || len(manifests) > 0 {
		if structured, ok := parseStructuredCandidates(structuredCandidates); ok {
			completion.Structured, normalizedFields = normalizeProviderStructured(structured, schemaName, structuredSchema)
			logStructuredNormalization(role, schemaName, normalizedFields, 0, len(structuredCandidates), false, message)
			if len(manifests) > 0 {
				logToolCallShapeNormalization(role, schemaName, "structured", structured["tool_calls"])
				calls, callErr := NormalizeProviderToolCalls(completion.Structured["tool_calls"], "", providerRequestID)
				if callErr != nil {
					p.recordProviderFailure(ctx, assignment, correlationID, messages, "tool_call_invalid")
					return ProviderCompletion{}, callErr
				}
				completion.ToolCalls = calls
			}
		} else if jsonMode {
			completion.Structured, normalizedFields = emptyProviderStructured(schemaName, structuredSchema)
			completion.StructuredFallback = true
			logStructuredNormalization(role, schemaName, normalizedFields, 0, len(structuredCandidates), true, message)
		}
	}
	providerResponse := map[string]any{"text": content, "structured": completion.Structured}
	if len(normalizedFields) > 0 {
		providerResponse["normalized_fields"] = normalizedFields
	}
	p.recordProviderSuccess(ctx, assignment, correlationID, messages, providerResponse)
	return completion, nil
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
	result := make([]string, 0, 2)
	appendCandidate := func(value any) {
		var candidate string
		switch typed := value.(type) {
		case string:
			candidate = strings.TrimSpace(typed)
		case map[string]any, []any:
			encoded, err := json.Marshal(typed)
			if err == nil {
				candidate = strings.TrimSpace(string(encoded))
			}
		}
		if candidate == "" {
			return
		}
		for _, existing := range result {
			if existing == candidate {
				return
			}
		}
		result = append(result, candidate)
	}
	appendCandidate(message["content"])
	appendCandidate(message["reasoning_content"])
	return result
}

func parseStructuredCandidates(candidates []string) (map[string]any, bool) {
	for _, candidate := range candidates {
		if structured, ok := parseStructuredCandidate(candidate, 0); ok {
			return structured, true
		}
	}
	return nil, false
}

// parseStructuredCandidate handles protocol framing added by otherwise
// OpenAI-compatible Providers. In particular, thinking-enabled local models
// may wrap their JSON in a <think> block, a Markdown JSON fence, or encode the
// JSON object as a JSON string. These are transport wrappers, not semantic
// fallbacks: prose without a complete terminal structured object remains
// invalid and is never interpreted as a decision.
func parseStructuredCandidate(candidate string, depth int) (map[string]any, bool) {
	if depth > 3 {
		return nil, false
	}
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return nil, false
	}

	var structured map[string]any
	if json.Unmarshal([]byte(candidate), &structured) == nil && structured != nil {
		return structured, true
	}

	// Some gateways serialize the provider's JSON string one additional time.
	var encoded string
	if json.Unmarshal([]byte(candidate), &encoded) == nil && strings.TrimSpace(encoded) != candidate {
		if structured, ok := parseStructuredCandidate(encoded, depth+1); ok {
			return structured, true
		}
	}

	// mlx-serve and compatible thinking adapters can place a transport-only
	// explanation before or around the structured payload. Only strip complete,
	// known wrappers; do not scan arbitrary prose for an embedded object.
	if start := strings.Index(candidate, "<think>"); start == 0 {
		if end := strings.Index(candidate[len("<think>"):], "</think>"); end >= 0 {
			end += len("<think>")
			if structured, ok := parseStructuredCandidate(candidate[len("<think>"):end], depth+1); ok {
				return structured, true
			}
			if structured, ok := parseStructuredCandidate(candidate[end+len("</think>"):], depth+1); ok {
				return structured, true
			}
		}
	} else if end := strings.Index(candidate, "</think>"); end >= 0 {
		if structured, ok := parseStructuredCandidate(candidate[end+len("</think>"):], depth+1); ok {
			return structured, true
		}
	}

	if strings.HasPrefix(candidate, "```") {
		lines := strings.Split(candidate, "\n")
		if len(lines) >= 3 {
			last := len(lines) - 1
			if strings.TrimSpace(lines[last]) == "```" {
				first := strings.TrimSpace(lines[0])
				if first == "```" || strings.EqualFold(first, "```json") {
					if structured, ok := parseStructuredCandidate(strings.Join(lines[1:last], "\n"), depth+1); ok {
						return structured, true
					}
				}
			}
		}
	}

	// A few thinking adapters omit the XML/fence marker and leave a short
	// transport prelude before the final object. Accept only a balanced object
	// that extends to the end of the designated structured channel; an object
	// embedded in trailing prose is still rejected.
	if trailing := trailingJSONObject(candidate); trailing != "" && trailing != candidate {
		if structured, ok := parseStructuredCandidate(trailing, depth+1); ok {
			return structured, true
		}
	}
	return nil, false
}

func trailingJSONObject(value string) string {
	start := -1
	depth := 0
	inString := false
	escaped := false
	lastEnd := -1
	for index := 0; index < len(value); index++ {
		char := value[index]
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = index
			}
			depth++
		case '}':
			if depth == 0 {
				return ""
			}
			depth--
			if depth == 0 {
				lastEnd = index + 1
			}
		}
	}
	if depth != 0 || start < 0 || lastEnd < 0 || strings.TrimSpace(value[lastEnd:]) != "" {
		return ""
	}
	return strings.TrimSpace(value[start:lastEnd])
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

func providerChatPayload(model string, messages []map[string]any, tokenBudget int, jsonMode bool, manifests []CapabilityManifest) map[string]any {
	return providerChatPayloadForRole(model, messages, tokenBudget, jsonMode, manifests, "")
}

func providerChatPayloadForRole(model string, messages []map[string]any, tokenBudget int, jsonMode bool, manifests []CapabilityManifest, role string) map[string]any {
	return providerChatPayloadWithSchema(model, messages, tokenBudget, jsonMode, manifests, role, "", nil, role == "cognitive_assessment")
}

func providerChatPayloadWithSchema(model string, messages []map[string]any, tokenBudget int, jsonMode bool, manifests []CapabilityManifest, role, schemaName string, schema map[string]any, enableThinking bool) map[string]any {
	payload := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": 0.7,
		"stream":      false,
	}
	if len(manifests) > 0 {
		payload["tools"] = ToolCallPayload(manifests)
		payload["tool_choice"] = "auto"
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
		return reflectionResponseSchema()
	default:
		return objectSchema(map[string]any{"result": openObjectSchema()}, nil, true)
	}
}

func (p *ProviderClient) recordProviderSuccess(ctx context.Context, assignment providerAssignment, correlationID string, messages []map[string]any, response any) {
	app := &App{DB: p.DB}
	app.recordModelRun(ctx, assignment.Role, assignment.EndpointID, assignment.ModelID, correlationID, messages, response, "completed", "")
}

func (p *ProviderClient) recordProviderFailure(ctx context.Context, assignment providerAssignment, correlationID string, messages []map[string]any, code string, diagnostic ...any) {
	app := &App{DB: p.DB}
	var response any
	if len(diagnostic) > 0 {
		response = diagnostic[0]
	}
	app.recordModelRun(ctx, assignment.Role, assignment.EndpointID, assignment.ModelID, correlationID, messages, response, "failed", code)
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
	assignment, err := p.assignment(ctx, role)
	if err != nil {
		return "", err
	}
	correlationID := diagnosticCorrelation(messages, "")
	body, err := json.Marshal(map[string]any{"model": assignment.ModelID, "messages": messages, "temperature": 0.7, "stream": true})
	if err != nil {
		return "", err
	}
	requestCtx, cancel := context.WithTimeout(ctx, assignment.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, assignment.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Idempotency-Key", "provider:"+stableDigest(role+":"+correlationID))
	request.Header.Set("X-Fluctlight-Provider-Request-Id", "provider:"+stableDigest(role+":"+correlationID))
	if assignment.Secret != "" {
		request.Header.Set("Authorization", "Bearer "+assignment.Secret)
	}
	client := p.HTTP
	if client == nil {
		client = &http.Client{}
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("provider stream returned HTTP %d", response.StatusCode)
	}
	scanner := bufio.NewScanner(io.LimitReader(response.Body, 16<<20))
	scanner.Buffer(make([]byte, 4<<10), 1<<20)
	var builder strings.Builder
	done := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			done = true
			break
		}
		var envelope struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &envelope); err != nil {
			return "", fmt.Errorf("provider stream frame invalid: %w", err)
		}
		if len(envelope.Choices) == 0 || envelope.Choices[0].Delta.Content == "" {
			continue
		}
		chunk := envelope.Choices[0].Delta.Content
		builder.WriteString(chunk)
		if onChunk != nil {
			if err := onChunk(chunk); err != nil {
				return "", err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if !done {
		return "", errors.New("provider stream incomplete")
	}
	result := builder.String()
	p.recordProviderSuccess(ctx, assignment, correlationID, messages, map[string]any{"text": result, "streamed": true})
	return result, nil
}

func (p *ProviderClient) Embed(ctx context.Context, text string) (string, []float64, error) {
	assignment, err := p.assignment(ctx, "embedding")
	if err != nil {
		return "", nil, err
	}
	correlationID := "embedding:" + stableDigest(assignment.ModelID+":"+text)
	body, err := json.Marshal(map[string]any{"model": assignment.ModelID, "input": []string{text}, "encoding_format": "float"})
	if err != nil {
		return "", nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, assignment.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, assignment.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "provider:"+stableDigest(correlationID))
	request.Header.Set("X-Fluctlight-Provider-Request-Id", "provider:"+stableDigest(correlationID))
	if assignment.Secret != "" {
		request.Header.Set("Authorization", "Bearer "+assignment.Secret)
	}
	client := p.HTTP
	if client == nil {
		client = &http.Client{}
	}
	response, err := client.Do(request)
	if err != nil {
		p.recordProviderFailure(ctx, assignment, correlationID, []map[string]any{{"role": "user", "content": text}}, err.Error())
		return "", nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		p.recordProviderFailure(ctx, assignment, correlationID, []map[string]any{{"role": "user", "content": text}}, fmt.Sprintf("http_%d", response.StatusCode))
		return "", nil, fmt.Errorf("embedding request returned HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&envelope); err != nil {
		p.recordProviderFailure(ctx, assignment, correlationID, []map[string]any{{"role": "user", "content": text}}, "embedding_response_invalid")
		return "", nil, err
	}
	if len(envelope.Data) == 0 || len(envelope.Data[0].Embedding) == 0 {
		p.recordProviderFailure(ctx, assignment, correlationID, []map[string]any{{"role": "user", "content": text}}, "embedding_response_empty")
		return "", nil, fmt.Errorf("embedding response is empty")
	}
	p.recordProviderSuccess(ctx, assignment, correlationID, []map[string]any{{"role": "user", "content": text}}, map[string]any{"dimensions": len(envelope.Data[0].Embedding)})
	return assignment.ModelID, envelope.Data[0].Embedding, nil
}
