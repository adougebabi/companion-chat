package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func privateChatExpressionFixture(t *testing.T, dirty bool) ReplyContextInput {
	t.Helper()
	data, err := os.ReadFile("testdata/private_chat_expression/wenzhiyi.reconstructed.json")
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := os.ReadFile("testdata/private_chat_expression/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Hashes map[string]string `json:"sha256"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != manifest.Hashes["wenzhiyi.reconstructed.json"] {
		t.Fatal("frozen reconstructed fixture changed")
	}
	var fixture struct {
		WorkingPersona map[string]any    `json:"working_persona"`
		CurrentInput   string            `json:"current_input"`
		Projection     ContextProjection `json:"projection"`
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if !dirty {
		fixture.Projection.RecentMessages = fixture.Projection.RecentMessages[len(fixture.Projection.RecentMessages)-1:]
	}
	return ReplyContextInput{Projection: fixture.Projection, WorkingPersona: fixture.WorkingPersona, CurrentInput: fixture.CurrentInput, CapabilityFacts: map[string]any{"image_generation_registered": true, "vision_understanding": "disabled", "sticker_attachments": false}}
}

// Captures actual serialized SDK requests to a controlled transport. This is
// reconstructed input, not the incident API wire or a real Cydonia response.
func TestPrivateChatExpressionBaseline(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		name := "clean"
		if dirty {
			name = "dirty"
		}
		t.Run(name, func(t *testing.T) {
			input := privateChatExpressionFixture(t, dirty)
			input.ExpressionVersion = factOnlyReplyLegacyPromptVersion
			router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Text: "受控模型输出，仅验证请求装配。"}
			})
			capture := captureProviderWirePayload(router)
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: capture}}}
			temperature, topP := 0.8, 0.9
			assignment := providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-cydonia", Timeout: time.Second, TokenBudget: 1024, MaxInputTokens: 8192, ContextWindowTokens: 16384, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1, Temperature: &temperature, TopP: &topP}
			draft, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: assignment, ConfigVersion: 1, CorrelationID: "reconstructed-" + name, DraftRef: "reconstructed-draft", Concurrency: 1})
			if err != nil {
				t.Fatal(err)
			}
			if len(capture.raw) != 1 {
				t.Fatal("unexpected physical calls", len(capture.raw))
			}
			if directory := os.Getenv("FLUCTLIGHT_PRIVATE_REPLY_BASELINE_OUTPUT"); directory != "" {
				if err := os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				for filename, data := range map[string][]byte{"A-" + name + ".wire.json": capture.raw[0], "A-" + name + ".draft.json": jsonBytes(draft)} {
					path := filepath.Join(directory, filename)
					if _, err := os.Stat(path); err == nil {
						t.Fatal("refuse to overwrite frozen baseline", path)
					}
					if err := os.WriteFile(path, data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
