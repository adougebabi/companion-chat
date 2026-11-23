package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"go.temporal.io/sdk/temporal"
)

func TestProviderGPUCapacity400Classification(t *testing.T) {
	const body = `{"error":{"message":"Prompt (2340 tokens) requires ~1687MB GPU memory but only ~0MB is available"}}`
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint("stream=", streaming), func(t *testing.T) {
			calls := 0
			client := einoHTTPClientWithHeaders(&http.Client{Transport: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})}, nil)
			chat, err := NewEinoModelFactory(client).NewChatModel(context.Background(), EinoModelConfig{BaseURL: "http://provider.invalid/v1", Model: "test", HTTPClient: client})
			if err != nil {
				t.Fatal(err)
			}
			_, err = RunADKLoop(context.Background(), ADKLoopConfig{Name: "capacity-test", Description: "classify capacity failure", Model: chat, EnableStreaming: streaming}, []*schema.Message{schema.UserMessage("hello")})
			var response *providerHTTPResponseError
			if !errors.As(err, &response) || response.StatusCode != 400 || calls != 1 {
				t.Fatalf("lost physical capacity rejection: calls=%d err=%v", calls, err)
			}
			if got := providerRunErrorCode(err); got != "provider_capacity_transient" {
				t.Fatalf("capacity rejection classified as %q; want provider_capacity_transient", got)
			}
		})
	}
}

func TestVisualIdentityCapacity400IsNotTerminal(t *testing.T) {
	err := &providerHTTPResponseError{StatusCode: 400, Detail: "Prompt (2340 tokens) requires ~1687MB GPU memory but only ~0MB is available"}
	if visualIdentityTerminalProviderError(fmt.Errorf("agent: %w", err)) {
		t.Fatal("temporary capacity rejection permanently fails visual identity session")
	}
}

func TestProviderCapacityClassificationIsNarrow(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		detail string
		want   bool
	}{
		{"gpu shortage", 400, "Prompt (2340 tokens) requires ~1687MB GPU memory but only ~0MB is available", true},
		{"gpu shortage 503", 503, "Prompt (2340 tokens) requires ~1687MB GPU memory but only ~0MB is available", true},
		{"nonzero available", 400, "Prompt requires 1.5 GiB GPU memory but only 0.5 GiB is available", true},
		{"cuda oom", 500, `{"error":{"message":"CUDA out of memory. Tried to allocate 2 GiB"}}`, true},
		{"gpu oom", 503, "GPU error: out of memory", true},
		{"pytorch oom", 507, "torch.cuda.OutOfMemoryError: allocation failed", true},
		{"bare oom", 400, "out of memory", true},
		{"json oom", 400, `{"error":{"message":"Out of memory"}}`, true},
		{"authentication", 401, "CUDA out of memory", false},
		{"authorization", 403, "GPU out of memory", false},
		{"rate limited", 429, "GPU out of memory", false},
		{"success", 200, "CUDA out of memory", false},
		{"billing", 400, "insufficient_user_quota: CUDA out of memory", false},
		{"quota", 500, "insufficient_quota: CUDA out of memory", false},
		{"context limit", 400, "context_length_exceeded: CUDA out of memory", false},
		{"schema", 400, "invalid_schema: CUDA out of memory", false},
		{"bad arguments", 400, "invalid request arguments", false},
		{"cpu oom", 400, "CPU out of memory", false},
		{"allocator", 500, "allocation failed", false},
		{"room substring", 400, "no room for prompt", false},
		{"generic memory", 400, "GPU memory unavailable", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := &providerHTTPResponseError{StatusCode: test.status, Detail: test.detail}
			if got := providerCapacityError(fmt.Errorf("wrapped: %w", err)); got != test.want {
				t.Fatalf("typed capacity=%v want=%v", got, test.want)
			}
			if got := ProviderCapacityFailure("agent: " + err.Error()); got != test.want {
				t.Fatalf("serialized capacity=%v want=%v", got, test.want)
			}
			if test.want && providerUpstreamRetryable(err) {
				t.Fatal("capacity should wait durably, not enter subsecond transport retry")
			}
		})
	}
	if providerCapacityError(errors.New("GPU out of memory")) || ProviderCapacityFailure("GPU out of memory") || providerCapacityError(nil) {
		t.Fatal("non-provider error classified as provider capacity")
	}
}

// Verify the actual HTTP/ADK boundary emits the capacity node and its reader
// cause that workflow reconciliation must distinguish after SDK serialization.
func TestProviderCapacityHTTPReadFailurePreservesTemporalEvidence(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint("stream=", streaming), func(t *testing.T) {
			readErr := errors.New("response body interrupted")
			calls := 0
			client := einoHTTPClientWithHeaders(&http.Client{Transport: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				body := io.MultiReader(strings.NewReader(`{"error":{"message":"Prompt requires ~1687MB GPU memory but only ~0MB is available"}}`), providerErrorTestReader{readErr})
				return &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(body), Request: r}, nil
			})}, nil)
			model, err := NewEinoModelFactory(client).NewChatModel(context.Background(), EinoModelConfig{BaseURL: "http://provider.invalid/v1", Model: "test", HTTPClient: client})
			if err != nil {
				t.Fatal(err)
			}
			_, err = RunADKLoop(context.Background(), ADKLoopConfig{Name: "capacity-test", Description: "capacity regression", Model: model, EnableStreaming: streaming}, []*schema.Message{schema.UserMessage("hello")})
			if calls != 1 || !errors.Is(err, readErr) || !providerCapacityError(err) {
				t.Fatalf("lost capacity/read cause: calls=%d err=%v", calls, err)
			}
			failure := temporal.GetDefaultFailureConverter().ErrorToFailure(err)
			found := false
			leaf := ""
			for current := failure; current != nil; current = current.GetCause() {
				found = found || ProviderCapacityFailure(current.GetMessage())
				leaf = current.GetMessage()
			}
			if !found || leaf != readErr.Error() {
				t.Fatalf("unexpected SDK failure chain: capacity=%v leaf=%q", found, leaf)
			}
		})
	}
}
