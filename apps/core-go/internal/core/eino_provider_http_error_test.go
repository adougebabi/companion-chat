package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

type providerErrorTestTransport func(*http.Request) (*http.Response, error)

func TestGatewayTimeoutClassificationSurvivesCancelledErrorBody(t *testing.T) {
	err := &providerHTTPResponseError{StatusCode: 524, Detail: "gateway timeout", readErr: context.Canceled}
	if got := providerRunErrorCode(err); got != "provider_gateway_timeout" {
		t.Fatal("received HTTP timeout relabelled", got)
	}
	if got := replyErrorCode(err); got != "reply_gateway_timeout" {
		t.Fatal("reply lost gateway timeout", got)
	}
}

func (f providerErrorTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEinoProviderHTTPErrorSurvivesNativeBoundary(t *testing.T) {
	for _, body := range []string{`{"error":{}}`, `{"error":{"message":{"detail":"GPU unavailable"}}}`, ``, `{"error":{"message":"ordinary rejection","type":"invalid_request_error"}}`} {
		for _, streaming := range []bool{false, true} {
			t.Run(body+map[bool]string{false: "/generate", true: "/stream"}[streaming], func(t *testing.T) {
				requests := 0
				client := einoHTTPClientWithHeaders(&http.Client{Transport: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) {
					requests++
					return &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
				})}, nil)
				chat, err := NewEinoModelFactory(client).NewChatModel(context.Background(), EinoModelConfig{BaseURL: "http://provider.invalid/v1", Model: "test", HTTPClient: client})
				if err != nil {
					t.Fatal(err)
				}
				queued := &queuedToolCallingChatModel{inner: chat, provider: &ProviderClient{HTTP: client}, role: "cognitive_assessment", assignment: providerAssignment{TokenBudget: 4096, ContextWindowTokens: 131072, MaxInputTokens: 98304, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}}
				result, err := RunADKLoop(context.Background(), ADKLoopConfig{Name: "http-error-test", Description: "preserve provider failure", Model: queued, EnableStreaming: streaming}, []*schema.Message{schema.UserMessage("hello")})
				if err == nil || !errors.Is(err, errProviderRequestFailed) || !strings.Contains(err.Error(), "HTTP 400") {
					t.Fatalf("actual HTTP failure lost through SDK/ADK: %v", err)
				}
				var failure *providerHTTPResponseError
				if !errors.As(err, &failure) || failure.StatusCode != 400 {
					t.Fatalf("typed HTTP cause lost: %v", err)
				}
				if body != "" && !strings.Contains(err.Error(), body) {
					t.Fatalf("error payload lost: %v", err)
				}
				if requests != 1 || result.FinalMessage != nil {
					t.Fatalf("requests=%d final=%v", requests, result.FinalMessage)
				}
			})
		}
	}
}

type providerErrorTestBody struct {
	io.Reader
	closed int
	read   int
}

func (b *providerErrorTestBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err
}
func (b *providerErrorTestBody) Close() error { b.closed++; return nil }

func TestEinoProviderHTTPErrorTransport(t *testing.T) {
	for _, status := range []int{200, 204, 299, 302, 400, 507} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			body := &providerErrorTestBody{Reader: strings.NewReader(strings.Repeat("x", 20000))}
			response := &http.Response{StatusCode: status, Body: body}
			transport := einoHeaderRoundTripper{base: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) { return response, nil })}
			request, _ := http.NewRequest(http.MethodPost, "http://provider.invalid", nil)
			got, err := transport.RoundTrip(request)
			if status < 400 {
				if got != response || err != nil || body.closed != 0 || body.read != 0 {
					t.Fatalf("success changed: response=%p err=%v body=%+v", got, err, body)
				}
				return
			}
			var failure *providerHTTPResponseError
			if got != nil || !errors.As(err, &failure) || failure.StatusCode != status || !failure.Truncated || len(failure.Detail) > providerHTTPErrorDetailLimit || body.read != providerHTTPErrorDetailLimit+1 || body.closed != 1 {
				t.Fatalf("non2xx contract: response=%p err=%v body=%+v", got, err, body)
			}
		})
	}
	for _, original := range []error{errors.New("network failure"), context.Canceled, context.DeadlineExceeded} {
		request, _ := http.NewRequest(http.MethodPost, "http://provider.invalid", nil)
		transport := einoHeaderRoundTripper{base: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) { return nil, original })}
		got, err := transport.RoundTrip(request)
		if got != nil || err != original {
			t.Fatalf("network cause changed: %v", err)
		}
	}
	client := &http.Client{Timeout: 17 * time.Second}
	if wrapped := einoHTTPClientWithHeaders(client, nil); wrapped.Timeout != client.Timeout {
		t.Fatal("client timeout changed")
	}
}

type providerErrorTestReader struct{ err error }

func (r providerErrorTestReader) Read([]byte) (int, error) { return 0, r.err }

func TestEinoProviderHTTPErrorReadFailure(t *testing.T) {
	readErr := errors.New("reader failure")
	body := &providerErrorTestBody{Reader: providerErrorTestReader{readErr}}
	request, _ := http.NewRequest(http.MethodPost, "http://provider.invalid", nil)
	err := preserveProviderHTTPError(request, &http.Response{StatusCode: 503, Body: body})
	if !errors.Is(err, readErr) || body.closed != 1 || !strings.Contains(err.Error(), "HTTP 503") || !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("read cause lost: %v", err)
	}
}

func TestEinoProviderHTTPErrorRedaction(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "http://provider.invalid", nil)
	request.Header.Set("Authorization", "Bearer actual-private-key")
	request.Header.Set("X-API-Key", "second-private-key")
	for _, input := range []string{
		`{"error":{"message":"rejected actual-private-key second-private-key","api_key":"other-secret","password":{"nested":"hidden-value"}}}`,
		`bad JSON actual-private-key second-private-key "api_key":"hidden-value`,
		`{"error":{"message":"authorization: Bearer hidden-value"}}`,
		`"Bearer hidden-value"`,
		`{"error":{"message":"prefix","credentials":["hidden-value"]}}`,
		`{"error":{"message":"prefix"},"api_key":"` + strings.Repeat("hidden-value", 1000),
	} {
		body := &providerErrorTestBody{Reader: strings.NewReader(input)}
		err := preserveProviderHTTPError(request, &http.Response{StatusCode: 400, Body: body})
		for _, secret := range []string{"actual-private-key", "second-private-key", "other-secret", "hidden-value"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("secret %s leaked: %v", secret, err)
			}
		}
		if !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), "[REDACTED]") {
			t.Fatalf("missing diagnostic: %v", err)
		}
	}
	body := &providerErrorTestBody{Reader: strings.NewReader(strings.Repeat(".", providerHTTPErrorDetailLimit-5) + "actual-private-key")}
	err := preserveProviderHTTPError(request, &http.Response{StatusCode: 400, Body: body})
	if strings.Contains(err.Error(), "actual") {
		t.Fatalf("partial echoed key leaked at read bound: %v", err)
	}
}

func TestEinoProviderHTTPErrorSuccessfulSDKResponses(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(map[bool]string{false: "generate", true: "stream"}[streaming], func(t *testing.T) {
			wire := `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"hello"}}]}`
			contentType := "application/json"
			if streaming {
				contentType = "text/event-stream"
				wire = "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"
			}
			client := einoHTTPClientWithHeaders(&http.Client{Transport: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(wire)), Request: r}, nil
			})}, nil)
			chat, err := NewEinoModelFactory(client).NewChatModel(context.Background(), EinoModelConfig{BaseURL: "http://provider.invalid/v1", Model: "test", HTTPClient: client})
			if err != nil {
				t.Fatal(err)
			}
			messages := []*schema.Message{schema.UserMessage("hello")}
			if !streaming {
				message, err := chat.Generate(context.Background(), messages)
				if err != nil || message.Content != "hello" {
					t.Fatalf("success JSON: %v %v", message, err)
				}
			} else {
				stream, err := chat.Stream(context.Background(), messages)
				if err != nil {
					t.Fatal(err)
				}
				defer stream.Close()
				message, err := stream.Recv()
				if err != nil || message.Content != "hello" {
					t.Fatalf("success SSE: %v %v", message, err)
				}
				if _, err := stream.Recv(); err != io.EOF {
					t.Fatalf("SSE terminal=%v", err)
				}
			}
		})
	}
}

func TestEinoProviderHTTPErrorPreservesRedirectPolicy(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			for _, stop := range []bool{false, true} {
				calls, redirects := 0, 0
				body := &providerErrorTestBody{Reader: strings.NewReader("redirect")}
				base := &http.Client{Transport: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) {
					calls++
					if calls == 1 {
						return &http.Response{StatusCode: status, Header: http.Header{"Location": {"/target"}}, Body: body, Request: r}, nil
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Request: r}, nil
				}), CheckRedirect: func(r *http.Request, via []*http.Request) error {
					redirects++
					if stop {
						return http.ErrUseLastResponse
					}
					return nil
				}}
				client := einoHTTPClientWithHeaders(base, nil)
				response, err := client.Get("http://provider.invalid/start")
				if err != nil {
					t.Fatal(err)
				}
				if redirects != 1 {
					t.Fatalf("redirect policy not called: %d", redirects)
				}
				if stop {
					if response.StatusCode != status || calls != 1 || body.closed != 0 {
						t.Fatalf("custom redirect policy changed: %+v calls=%d body=%+v", response, calls, body)
					}
				} else if response.StatusCode != 200 || calls != 2 || body.closed != 1 {
					t.Fatalf("redirect follow changed: %+v calls=%d body=%+v", response, calls, body)
				}
				response.Body.Close()
			}
		})
	}
}

func TestEinoProviderHTTPErrorRedactsTruncatedEscapedCredential(t *testing.T) {
	for _, credential := range []string{`actual"private-key`, `actual\private-key`} {
		request, _ := http.NewRequest(http.MethodPost, "http://provider.invalid", nil)
		request.Header.Set("Authorization", "Bearer "+credential)
		encoded, _ := json.Marshal(credential)
		escaped := string(encoded[1 : len(encoded)-1])
		prefix := escaped[:10]
		input := `{"error":{"message":"` + strings.Repeat(".", providerHTTPErrorDetailLimit+1-len(`{"error":{"message":"`)-len(prefix)) + escaped + `"}}`
		err := preserveProviderHTTPError(request, &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(input))})
		if strings.Contains(err.Error(), "actual") || !strings.Contains(err.Error(), "[REDACTED") {
			t.Fatalf("escaped credential excerpt leaked: %v", err)
		}
	}
}
