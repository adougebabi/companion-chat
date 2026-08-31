package core

import (
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
	Role    string
	BaseURL string
	ModelID string
	Secret  string
	Timeout time.Duration
}

func (p *ProviderClient) assignment(ctx context.Context, role string) (providerAssignment, error) {
	var baseURL, modelID, purpose string
	var timeoutSeconds int
	err := p.DB.Pool().QueryRow(ctx, `
		SELECT e.base_url, r.model_id, e.secret_purpose, r.timeout_seconds
		FROM public.model_roles r
		JOIN public.provider_endpoints e ON e.id = r.provider_endpoint_id
		WHERE r.role = $1
	`, role).Scan(&baseURL, &modelID, &purpose, &timeoutSeconds)
	if err != nil {
		return providerAssignment{}, fmt.Errorf("provider role %s unavailable: %w", role, err)
	}
	secret, err := p.secret(ctx, purpose)
	if err != nil {
		return providerAssignment{}, err
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}
	return providerAssignment{Role: role, BaseURL: strings.TrimRight(baseURL, "/"), ModelID: modelID, Secret: secret, Timeout: time.Duration(timeoutSeconds) * time.Second}, nil
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
	assignment, err := p.assignment(ctx, role)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model":       assignment.ModelID,
		"messages":    messages,
		"temperature": 0.7,
		"stream":      false,
	}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, assignment.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, assignment.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if assignment.Secret != "" {
		request.Header.Set("Authorization", "Bearer "+assignment.Secret)
	}
	client := p.HTTP
	if client == nil {
		client = &http.Client{}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("provider request failed: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("provider request returned HTTP %d", response.StatusCode)
	}
	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("provider response is not JSON: %w", err)
	}
	choices, ok := envelope["choices"].([]any)
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("provider response has no choices")
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("provider response choice is invalid")
	}
	message, ok := choice["message"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("provider response message is invalid")
	}
	content, ok := message["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("provider response content is empty")
	}
	if jsonMode {
		var structured map[string]any
		if err := json.Unmarshal([]byte(content), &structured); err != nil {
			return nil, fmt.Errorf("provider structured response is invalid: %w", err)
		}
		return structured, nil
	}
	return map[string]any{"text": content}, nil
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

func (p *ProviderClient) Embed(ctx context.Context, text string) (string, []float64, error) {
	assignment, err := p.assignment(ctx, "embedding")
	if err != nil {
		return "", nil, err
	}
	body, err := json.Marshal(map[string]any{"model": assignment.ModelID, "input": text})
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
	if assignment.Secret != "" {
		request.Header.Set("Authorization", "Bearer "+assignment.Secret)
	}
	client := p.HTTP
	if client == nil {
		client = &http.Client{}
	}
	response, err := client.Do(request)
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", nil, fmt.Errorf("embedding request returned HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&envelope); err != nil {
		return "", nil, err
	}
	if len(envelope.Data) == 0 || len(envelope.Data[0].Embedding) == 0 {
		return "", nil, fmt.Errorf("embedding response is empty")
	}
	return assignment.ModelID, envelope.Data[0].Embedding, nil
}
