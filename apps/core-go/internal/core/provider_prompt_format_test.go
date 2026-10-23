package core

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProviderJSONAndMultimodalTransportRemainLossless(t *testing.T) {
	payload := map[string]any{"facts": []any{map[string]any{"goal_id": "quoted-domain-field", "text": "a|b\n第二行"}, map[string]any{"value": false}}, "large": json.Number("9007199254740993")}
	text := renderProviderJSON(payload)
	image := map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,bytes"}}
	messages := []map[string]any{{"role": "system", "content": "rules"}, {"role": "user", "content": []any{map[string]any{"type": "text", "text": text}, image}}, {"role": "tool", "content": text, "tool_call_id": "call-1"}}
	for _, role := range []string{"media_prompt", "cognitive_assessment"} {
		if got := formatProviderMessagesForRole(messages, role); !reflect.DeepEqual(got, messages) {
			t.Fatalf("transport drift: %#v", got)
		}
	}
	if !json.Valid([]byte(text)) {
		t.Fatal("not JSON")
	}
	_, decoded, ok := decodeProviderJSONPayload(text)
	if !ok || renderProviderJSON(decoded) != text {
		t.Fatal("number/body changed")
	}
}

func TestProviderFormatterLeavesRuntimeAndUserTextUntouched(t *testing.T) {
	for _, content := range []string{"[RUNTIME CONTEXT]\n{\"facts\":[]}\n[/RUNTIME CONTEXT]", " {\"id\": 7,\"text\":\"第一行\\n第二行\"} ", "说明\n{}"} {
		if got := formatProviderPromptContent(content); got != content {
			t.Fatalf("text changed: %q", got)
		}
	}
}
