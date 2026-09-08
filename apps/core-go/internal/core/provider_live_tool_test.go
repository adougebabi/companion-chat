package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLiveProviderRecognizesImageGenerationIntent is an opt-in regression
// against a real OpenAI-compatible local Provider. It deliberately does not
// assert prompt text; it asserts the model's actual normalized tool choice.
//
// Run with:
// FLUCTLIGHT_LIVE_PROVIDER_TEST=1 \
// FLUCTLIGHT_LIVE_PROVIDER_URL=http://127.0.0.1:11234/v1 \
// FLUCTLIGHT_LIVE_PROVIDER_MODEL=qwen3.8-27b-abliterated-mtplx-optimized-speed \
// go test ./internal/core -run TestLiveProviderRecognizesImageGenerationIntent -v
func TestLiveProviderRecognizesImageGenerationIntent(t *testing.T) {
	if strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST")) != "1" {
		t.Skip("set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_URL")), "/")
	model := strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_MODEL"))
	if baseURL == "" || model == "" {
		t.Fatal("FLUCTLIGHT_LIVE_PROVIDER_URL and FLUCTLIGHT_LIVE_PROVIDER_MODEL are required")
	}
	manifests := []CapabilityManifest{conversationReplyCapabilityManifest(), imageCapabilityManifest(), affectEventCapabilityManifest()}
	messages := composeProviderMessages("cognitive_assessment", []map[string]any{
		{"role": "system", "content": conversationAssessmentInstruction},
		{"role": "user", "content": "请同时完成两件事：第一，实际把刚才这个雨后窗边、整理好衣服和小道具的场景制作成一份视觉作品；第二，用一句话告诉我你准备采用的构图重点。"},
	})
	payload := providerChatPayloadWithSchema(model, messages, 1800, false, manifests, "cognitive_assessment", "conversation_turn_response", cognitiveTurnResponseSchema(), true)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 2 * time.Minute}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("live Provider status=%d body=%s", response.StatusCode, boundedLiveProviderBody(responseBody))
	}
	var envelope map[string]any
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		t.Fatalf("decode live Provider response: %v; body=%s", err, boundedLiveProviderBody(responseBody))
	}
	choices := arrayValue(envelope["choices"])
	if len(choices) == 0 {
		t.Fatalf("live Provider returned no choices: %s", boundedLiveProviderBody(responseBody))
	}
	message := mapValue(mapValue(choices[0])["message"])
	toolCalls := arrayValue(message["tool_calls"])
	foundImage := false
	foundReply := false
	for _, raw := range toolCalls {
		call := mapValue(raw)
		function := mapValue(call["function"])
		switch stringValue(function["name"]) {
		case "media.image.generate":
			foundImage = true
		case "conversation.reply":
			foundReply = true
		}
	}
	if !foundImage || !foundReply {
		t.Fatalf("live Provider must call both media.image.generate and conversation.reply; image=%t reply=%t tool_calls=%s response=%s", foundImage, foundReply, fmt.Sprint(toolCalls), boundedLiveProviderBody(responseBody))
	}
}

func boundedLiveProviderBody(value []byte) string {
	const limit = 4000
	if len(value) <= limit {
		return string(value)
	}
	return string(value[:limit]) + "…"
}
