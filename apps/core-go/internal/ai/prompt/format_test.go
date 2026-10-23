package prompt

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestJSONPromptRepresentationPreservesValuesAndShapes(t *testing.T) {
	for _, rows := range []any{[]any{map[string]any{"value": "a|b,c\n第二行"}}, []any{map[string]any{"value": "a"}, map[string]any{"value": "b", "unknown": nil}}} {
		value := map[string]any{"items": rows, "enabled": false, "number": json.Number("9007199254740993"), "schema": map[string]any{"properties": map[string]any{"item_ids": map[string]any{"type": "array"}}, "required": []any{"item_ids"}}}
		wire := RenderJSON(value)
		_, decoded, ok := DecodeJSONPayload(wire)
		if !ok || RenderJSON(decoded) != wire || !strings.Contains(wire, "9007199254740993") {
			t.Fatalf("JSON semantics changed: %s", wire)
		}
		if strings.Contains(wire, "items[2]") {
			t.Fatalf("shape-dependent encoding: %s", wire)
		}
	}
}

func TestTransportPreservesUserJSONNativeToolsAndImages(t *testing.T) {
	user := "  {\"evidence_refs\": [\"user data\"], \"text\": \"第一行\\n第二行\"}  "
	messages := []map[string]any{
		{"role": "system", "content": `Return {"ok":true}`},
		{"role": "user", "content": user},
		{"role": "assistant", "content": "ordinary prose", "tool_calls": []any{map[string]any{"id": "native-id"}}},
		{"role": "tool", "tool_call_id": "native-id", "content": `{"id":"db-tool-contract","ok":true}`},
		{"role": "user", "content": []any{map[string]any{"type": "text", "text": user}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,bytes"}}}},
	}
	for _, role := range []string{"media_prompt", "cognitive_assessment", "reflection", ""} {
		got := FormatMessagesForRole(messages, role)
		if !reflect.DeepEqual(got, messages) {
			t.Fatalf("transport changed content for %s: %#v", role, got)
		}
		got[0]["role"] = "changed"
		if messages[0]["role"] != "system" {
			t.Fatal("envelope aliases source")
		}
	}
	if FormatPromptContent(user) != user {
		t.Fatal("user JSON was parsed as internal data")
	}
}

func TestDecodeTaskJSONPreservesPrefixAndRejectsTrailingData(t *testing.T) {
	prefix, value, ok := DecodeJSONPayload("authoritative data\n{\"x\":false}")
	if !ok || prefix != "authoritative data" || RenderJSON(value) != `{"x":false}` {
		t.Fatal("task JSON decode changed")
	}
	for _, input := range []string{"", "  ", "prose", `{} trailing`, `{}` + `{}`} {
		if _, _, ok := DecodeJSONPayload(input); ok {
			t.Fatalf("invalid task accepted: %q", input)
		}
	}
}
