package core

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type liveResponseCapture struct {
	inner  http.RoundTripper
	mu     sync.Mutex
	shapes []map[string]any
}

func (capture *liveResponseCapture) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := capture.inner.RoundTrip(request)
	if err != nil || response == nil || response.Body == nil {
		return response, err
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(body))
	if readErr == nil {
		var envelope map[string]any
		if json.Unmarshal(body, &envelope) == nil {
			choices := arrayValue(envelope["choices"])
			if len(choices) > 0 {
				message := mapValue(mapValue(choices[0])["message"])
				shapes := make([]map[string]any, 0)
				for _, raw := range arrayValue(message["tool_calls"]) {
					call := mapValue(raw)
					function := mapValue(call["function"])
					name := stringValue(call["name"])
					if name == "" {
						name = stringValue(call["capability_name"])
					}
					if name == "" {
						name = stringValue(function["name"])
					}
					args := call["arguments"]
					if args == nil {
						args = function["arguments"]
					}
					argumentKeys := []string{}
					if object := mapValue(args); len(object) > 0 {
						for key := range object {
							argumentKeys = append(argumentKeys, key)
						}
					}
					id := stringValue(call["id"])
					if id == "" {
						id = stringValue(call["call_id"])
					}
					shapes = append(shapes, map[string]any{"name": name, "id_present": strings.TrimSpace(id) != "", "argument_keys": argumentKeys})
				}
				capture.mu.Lock()
				capture.shapes = append(capture.shapes, shapes...)
				capture.mu.Unlock()
			}
		}
	}
	return response, readErr
}

func (capture *liveResponseCapture) snapshot() []map[string]any {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	result := make([]map[string]any, len(capture.shapes))
	for index, shape := range capture.shapes {
		result[index] = cloneMap(shape)
	}
	return result
}

// TestLiveHandleTurnUsesRealProviderForPersonalityDecision is the production
// chain smoke that the scripted Provider tests cannot provide. It uses a
// disposable PostgreSQL database, resolves the model through ProviderClient,
// runs HandleTurn, and verifies the durable active profile plus the assistant
// message. The test intentionally has no takeover_rules so the result isolates
// the persistent-switch path instead of asking the live model to also arbitrate.
func TestLiveHandleTurnUsesRealProviderForPersonalityDecision(t *testing.T) {
	if strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST")) != "1" {
		t.Skip("set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider")
	}
	if strings.TrimSpace(os.Getenv("GO_CORE_TEST_DATABASE_URL")) == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is required for the live HandleTurn chain")
	}
	baseURL, model := liveProviderConfig(t)
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "live-turn-owner", "live-turn-fluctlight", "live-turn-conversation"
	takeoverChainSeedWithTakeoverRules(t, ctx, repository, ownerID, fluctlightID, conversationID, nil)
	endpointID := "takeover-chain-endpoint-" + fluctlightID
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, endpointID, strings.TrimRight(baseURL, "/")); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id=$2,timeout_seconds=600,token_budget=4096 WHERE role='cognitive_assessment' AND provider_endpoint_id=$1`, endpointID, model); err != nil {
		t.Fatal(err)
	}

	responseCapture := &liveResponseCapture{inner: http.DefaultTransport}
	capture := captureProviderWirePayload(responseCapture)
	app := newTestApp(t, repository, capture)
	turnCtx, cancel := context.WithTimeout(context.Background(), liveProviderRequestTimeout()+2*time.Minute)
	defer cancel()
	result, err := app.HandleTurn(turnCtx, ownerID, conversationID, takeoverChainTurnPayload(
		fluctlightID,
		"安全确认已收到。请按 safety 规则把持久主导人格从 spark 切换为 twilight，并用暮光的风格回复我。",
		"live-turn-switch-1",
		"live-turn-switch-1",
	))
	if err != nil {
		t.Fatalf("live HandleTurn failed: %v; provider_tool_shapes=%#v", err, responseCapture.snapshot())
	}
	if strings.TrimSpace(stringValue(result.Assistant["text"])) == "" {
		t.Fatalf("live HandleTurn returned no assistant text: %#v", result.Assistant)
	}
	if got := capture.count(); got < 2 {
		t.Fatalf("persistent switch path must use a recognition request and a post-switch Main request, got %d", got)
	}
	var active string
	if err := repository.Pool().QueryRow(turnCtx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != "twilight" {
		t.Fatalf("live persistent switch did not update active profile: %q; provider_tool_shapes=%#v", active, responseCapture.snapshot())
	}
	var messageCount int
	if err := repository.Pool().QueryRow(turnCtx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("live HandleTurn assistant message count = %d, want 1", messageCount)
	}
}
