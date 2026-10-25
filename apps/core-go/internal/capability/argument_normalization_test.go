package capability

import (
	"encoding/json"
	"testing"
)

func TestModelJSONContainersRecoverWithoutChangingTextOrAuthority(t *testing.T) {
	schema := map[string]any{"type": "object", "additionalProperties": false, "required": []any{"items", "text"}, "properties": map[string]any{
		"items": map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []any{"slot"}, "additionalProperties": false, "properties": map[string]any{"slot": map[string]any{"type": "string"}}}},
		"text":  map[string]any{"type": "string"},
	}}
	raw := json.RawMessage(`{"items":"[{\"slot\":\"outer\"}]","text":"[{\"slot\":\"original user text\"}]"}`)
	result, err := NormalizeModelArguments(raw, schema)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(result, &got)
	if len(got["items"].([]any)) != 1 || got["text"] != `[{"slot":"original user text"}]` {
		t.Fatal(string(result))
	}
	for _, raw := range []string{`{"items":"[bad]","text":"x"}`, `{"items":"{\"slot\":\"outer\"}","text":"x"}`, `{"items":"[{\"wrong\":\"outer\"}]","text":"x"}`, `{"items":[],"text":"x","prepared_payload":"{}"}`} {
		if _, err := NormalizeModelArguments(json.RawMessage(raw), schema); err == nil {
			t.Fatal("invalid container or authority accepted", raw)
		}
	}
}
