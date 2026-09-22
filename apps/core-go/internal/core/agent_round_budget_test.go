package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestNativeRoundBudgetIncludesToolHistoryBeyondDiagnosticLimit(t *testing.T) {
	m := &queuedToolCallingChatModel{role: "cognitive_assessment", assignment: providerAssignment{MaxInputTokens: 4096}}
	input := []*schema.Message{schema.UserMessage("small initial input")}
	if err := m.validateInputBudget(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 64; n++ {
		input = append(input, schema.AssistantMessage("ok", nil))
	}
	input = append(input, &schema.Message{Role: schema.Tool, ToolCallID: "real-call", Content: strings.Repeat("new committed tool output ", 1000)})
	// The oversized payload occurs after the diagnostics' 64-message bound.
	// Generate and Stream must both reject it before touching the nil transport.
	if _, err := m.Generate(context.Background(), input); !errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("Generate budget: %v", err)
	}
	if _, err := m.Stream(context.Background(), input); !errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("Stream budget: %v", err)
	}
}

func TestNativeRoundBudgetIncludesMultimodalTextAndImageAllowance(t *testing.T) {
	m := &queuedToolCallingChatModel{role: "visual_identity_vision", assignment: providerAssignment{MaxInputTokens: 400}}
	url := "data:image/png;base64," + strings.Repeat("A", 100000)
	input := []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &url}, Detail: schema.ImageURLDetail("high")}},
	}}}
	if err := m.validateInputBudget(context.Background(), input); !errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("image allowance was omitted: %v", err)
	}
	m.assignment.MaxInputTokens = 4096
	if err := m.validateInputBudget(context.Background(), input); err != nil {
		t.Fatalf("base64 bytes were incorrectly charged as text: %v", err)
	}
	input[0].UserInputMultiContent = append(input[0].UserInputMultiContent, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: strings.Repeat("multimodal text ", 1000)})
	if err := m.validateInputBudget(context.Background(), input); !errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		t.Fatalf("multimodal text was omitted: %v", err)
	}
}
