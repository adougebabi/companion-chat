package core

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestRuntimeContextRefreshPreservesRawADKHistoryAndMultimodalInput(t *testing.T) {
	initial, err := providerMessagesToEino([]map[string]any{
		{"role": "system", "content": "trusted old persona and task"},
		{"role": "user", "content": "[RUNTIME CONTEXT]\nold hair\n[/RUNTIME CONTEXT]"},
		{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": "describe this picture"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,abc", "detail": "high"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	refreshes := 0
	refresh := &runtimeContextRefresh{refresh: func(context.Context) (modelContextRefreshContent, error) {
		refreshes++
		return modelContextRefreshContent{System: "trusted new persona and task", Runtime: "[RUNTIME CONTEXT]\nshort hair\n[/RUNTIME CONTEXT]"}, nil
	}}
	first, err := refresh.prepare(context.Background(), initial)
	if err != nil || first[0].Content != "trusted old persona and task" || refreshes != 0 {
		t.Fatalf("first request content=%#v refreshes=%d err=%v", first, refreshes, err)
	}
	toolCall := &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "call-1"}}}
	toolResult := &schema.Message{Role: schema.Tool, ToolCallID: "call-1", ToolName: "appearance.style", Content: `{"status":"completed"}`}
	continuation := append(append([]*schema.Message(nil), initial...), toolCall, toolResult)
	refresh.markDirty()
	second, err := refresh.prepare(context.Background(), continuation)
	if err != nil {
		t.Fatal(err)
	}
	if refreshes != 1 || second[0].Content != "trusted new persona and task" || second[1].Content != "[RUNTIME CONTEXT]\nshort hair\n[/RUNTIME CONTEXT]" {
		t.Fatalf("continuation did not refresh: %#v refreshes=%d", second, refreshes)
	}
	if initial[0].Content != "trusted old persona and task" || initial[1].Content != "[RUNTIME CONTEXT]\nold hair\n[/RUNTIME CONTEXT]" || second[0] == initial[0] || second[1] == initial[1] {
		t.Fatal("refresh modified Eino-owned original messages")
	}
	if len(second[2].UserInputMultiContent) != 2 || second[2].Content != "" || second[3].ToolCalls[0].ID != second[4].ToolCallID || second[4].Content != toolResult.Content {
		t.Fatalf("multimodal input or Tool protocol changed: %#v", second)
	}
	mapping := physicalModelSourceMap(second)
	if stringValue(mapping[0]["source"]) != "trusted_task_configuration" || stringValue(mapping[1]["source"]) != "scoped_runtime_projection" || stringValue(mapping[3]["source"]) != "agent_tool_call" || stringValue(mapping[4]["source"]) != "tool_result" || stringValue(mapping[4]["tool_call_id"]) != "call-1" {
		t.Fatalf("physical source map lost Tool pairing: %#v", mapping)
	}
	third, err := refresh.prepare(context.Background(), continuation)
	if err != nil || refreshes != 1 || third[1].Content != second[1].Content {
		t.Fatalf("unchanged continuation should not rebuild: refreshes=%d err=%v", refreshes, err)
	}
}

func TestRuntimeContextRefreshFailsBeforePhysicalRequest(t *testing.T) {
	input := []*schema.Message{
		{Role: schema.System, Content: "system"},
		{Role: schema.User, Content: "[RUNTIME CONTEXT]\nold\n[/RUNTIME CONTEXT]"},
	}
	refresh := &runtimeContextRefresh{refresh: func(context.Context) (modelContextRefreshContent, error) {
		return modelContextRefreshContent{}, errors.New("source_revision_stale")
	}}
	if _, err := refresh.prepare(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	refresh.markDirty()
	if _, err := refresh.prepare(context.Background(), input); err == nil || err.Error() != "source_revision_stale" {
		t.Fatalf("stale context sent: %v", err)
	}
	if input[1].Content != "[RUNTIME CONTEXT]\nold\n[/RUNTIME CONTEXT]" {
		t.Fatal("failed refresh changed original context")
	}
}
