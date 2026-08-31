package bff

// This file is the deliberately small HTTP-only Core adapter.  It knows
// about Core transport conventions (service identity, human session and
// snake_case payloads), but contains no domain or storage code.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const serviceKeyHeader = "x-fluctlight-service-key"
const humanSessionHeader = "x-fluctlight-human-session"

// CoreError is a typed, bounded representation of a failed Core request.
// Route code must branch on Status/Code rather than on message text.
type CoreError struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *CoreError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("core request failed: %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("core request failed: %d %s", e.Status, e.Code)
}

type coreClient struct {
	baseURL    *url.URL
	serviceKey string
	client     *http.Client
}

func newCoreClient(baseURL *url.URL, serviceKey string, client *http.Client) *coreClient {
	if client == nil {
		client = &http.Client{}
	}
	return &coreClient{baseURL: baseURL, serviceKey: serviceKey, client: client}
}

func (c *coreClient) request(ctx context.Context, method, endpoint, humanSession string, body any, extra http.Header) (*http.Response, error) {
	if c.baseURL == nil {
		return nil, errors.New("core base URL is not configured")
	}
	target := *c.baseURL
	basePath := strings.TrimRight(target.Path, "/")
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	target.Path = basePath + parsedEndpoint.Path
	// Preserve an escaped path segment (for example an ID containing a slash)
	// while allowing net/url to serialize it exactly once.
	target.RawPath = basePath + parsedEndpoint.EscapedPath()
	target.RawQuery = parsedEndpoint.RawQuery
	target.Fragment = ""

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set(serviceKeyHeader, c.serviceKey)
	if humanSession != "" {
		req.Header.Set(humanSessionHeader, humanSession)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, values := range extra {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := decodeCoreError(resp)
		_ = resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func decodeCoreError(resp *http.Response) *CoreError {
	defaultMessage := true
	err := &CoreError{
		Status:  resp.StatusCode,
		Code:    "core_request_failed",
		Message: fmt.Sprintf("Core request failed: %d core_request_failed", resp.StatusCode),
		Details: map[string]any{},
	}
	const maxErrorBody = 64 << 10
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if readErr != nil || len(bytes.TrimSpace(data)) == 0 {
		return err
	}
	var payload struct {
		Detail  any            `json:"detail"`
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return err
	}
	if payload.Code != "" {
		err.Code = payload.Code
	}
	if payload.Message != "" {
		err.Message = payload.Message
		defaultMessage = false
	}
	if payload.Details != nil {
		err.Details = payload.Details
	}
	switch detail := payload.Detail.(type) {
	case string:
		if detail != "" {
			err.Code = detail
		}
	case map[string]any:
		if value, ok := detail["code"].(string); ok && value != "" {
			err.Code = value
		}
		if value, ok := detail["message"].(string); ok && value != "" {
			err.Message = value
			defaultMessage = false
		}
		if value, ok := detail["details"].(map[string]any); ok {
			err.Details = value
		}
	case []any:
		// Structured validation errors are preserved under a stable key so the
		// browser boundary can expose bounded field-level diagnostics.
		if len(detail) > 0 {
			err.Code = "core_request_validation_failed"
			err.Message = "Core request validation failed"
			defaultMessage = false
			err.Details = map[string]any{"validation_errors": detail}
		}
	}
	if defaultMessage {
		err.Message = fmt.Sprintf("Core request failed: %d %s", err.Status, err.Code)
	}
	return err
}

func (c *coreClient) json(ctx context.Context, method, endpoint, humanSession string, body any, extra http.Header, out any) error {
	resp, err := c.request(ctx, method, endpoint, humanSession, body, extra)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode core response: %w", err)
	}
	return nil
}

func (c *coreClient) health(ctx context.Context, endpoint string) (map[string]any, error) {
	var out map[string]any
	err := c.json(ctx, http.MethodGet, endpoint, "", nil, nil, &out)
	return out, err
}

func (c *coreClient) doJSON(ctx context.Context, method, endpoint, humanSession string, body any) (map[string]any, error) {
	var out map[string]any
	return out, c.json(ctx, method, endpoint, humanSession, body, nil, &out)
}

func (c *coreClient) doAny(ctx context.Context, method, endpoint, humanSession string, body any) (any, error) {
	var out any
	return out, c.json(ctx, method, endpoint, humanSession, body, nil, &out)
}

func (c *coreClient) doValue(ctx context.Context, method, endpoint, humanSession string, body any, out any) error {
	return c.json(ctx, method, endpoint, humanSession, body, nil, out)
}

func (c *coreClient) queryEndpoint(endpoint string, query url.Values) string {
	if len(query) == 0 {
		return endpoint
	}
	return endpoint + "?" + query.Encode()
}

func intQuery(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
