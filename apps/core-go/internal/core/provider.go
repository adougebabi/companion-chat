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
	completion, err := p.completeWithTools(ctx, role, messages, jsonMode, nil)
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

// StructuredWithTools requests a model assessment with the external
// capability catalog. Native provider calls and JSON sidecars are normalized
// into ProviderCompletion before the application sees them.
func (p *ProviderClient) StructuredWithTools(ctx context.Context, role string, messages []map[string]any, manifests []CapabilityManifest) (ProviderCompletion, error) {
	return p.completeWithTools(ctx, role, messages, false, manifests)
}

func (p *ProviderClient) completeWithTools(ctx context.Context, role string, messages []map[string]any, jsonMode bool, manifests []CapabilityManifest) (ProviderCompletion, error) {
	assignment, err := p.assignment(ctx, role)
	if err != nil {
		return ProviderCompletion{}, err
	}
	correlationID := diagnosticCorrelation(messages, "")
	providerRequestID := "provider:" + stableDigest(role+":"+correlationID)
	payload := providerChatPayload(assignment.ModelID, messages, assignment.TokenBudget, jsonMode, manifests)
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
	content, _ := message["content"].(string)
	content = strings.TrimSpace(content)
	completion := ProviderCompletion{Text: content, ToolCalls: calls, DoneSeen: true}
	if len(calls) > 0 {
		for index := range completion.ToolCalls {
			completion.ToolCalls[index].SourceFactID = ""
		}
		if content != "" {
			var structured map[string]any
			if json.Unmarshal([]byte(content), &structured) == nil && structured != nil {
				completion.Structured = structured
			}
		}
		p.recordProviderSuccess(ctx, assignment, correlationID, messages, map[string]any{"tool_calls": completion.ToolCalls, "text": content})
		return completion, nil
	}
	if content == "" {
		p.recordProviderFailure(ctx, assignment, correlationID, messages, "response_content_empty")
		return ProviderCompletion{}, fmt.Errorf("provider response content is empty")
	}
	if jsonMode || len(manifests) > 0 {
		var structured map[string]any
		if err := json.Unmarshal([]byte(content), &structured); err != nil {
			if jsonMode {
				p.recordProviderFailure(ctx, assignment, correlationID, messages, "structured_response_invalid")
				return ProviderCompletion{}, fmt.Errorf("provider structured response is invalid: %w", err)
			}
		} else if structured != nil {
			completion.Structured = structured
			if len(manifests) > 0 {
				calls, callErr := NormalizeProviderToolCalls(structured["tool_calls"], "", providerRequestID)
				if callErr != nil {
					p.recordProviderFailure(ctx, assignment, correlationID, messages, "tool_call_invalid")
					return ProviderCompletion{}, callErr
				}
				completion.ToolCalls = calls
			}
		}
	}
	p.recordProviderSuccess(ctx, assignment, correlationID, messages, map[string]any{"text": content, "structured": completion.Structured})
	return completion, nil
}

func providerChatPayload(model string, messages []map[string]any, tokenBudget int, jsonMode bool, manifests []CapabilityManifest) map[string]any {
	payload := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": 0.7,
		"stream":      false,
	}
	if len(manifests) > 0 {
		payload["tools"] = ToolCallPayload(manifests)
		payload["tool_choice"] = "auto"
	} else if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	if tokenBudget > 0 {
		payload["max_tokens"] = tokenBudget
	}
	return payload
}

func (p *ProviderClient) recordProviderSuccess(ctx context.Context, assignment providerAssignment, correlationID string, messages []map[string]any, response any) {
	app := &App{DB: p.DB}
	app.recordModelRun(ctx, assignment.Role, assignment.EndpointID, assignment.ModelID, correlationID, messages, response, "completed", "")
}

func (p *ProviderClient) recordProviderFailure(ctx context.Context, assignment providerAssignment, correlationID string, messages []map[string]any, code string) {
	app := &App{DB: p.DB}
	app.recordModelRun(ctx, assignment.Role, assignment.EndpointID, assignment.ModelID, correlationID, messages, nil, "failed", code)
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
