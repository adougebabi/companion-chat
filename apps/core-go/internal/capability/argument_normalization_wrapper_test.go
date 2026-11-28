package capability

import (
	"encoding/json"
	"testing"
)

func TestStrictFreeformWrapperIsUnwrapped(t *testing.T) {
	schema := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"action_plan": map[string]any{"type": "object", "additionalProperties": true},
		"label":       map[string]any{"type": "object", "additionalProperties": true, "properties": map[string]any{"json": map[string]any{"type": "string"}}},
	}}
	raw := json.RawMessage(`{"action_plan":{"json":"{\"kind\":\"virtual_shopping\",\"items\":[]}"}}`)
	out, err := NormalizeModelArguments(raw, schema)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	_ = json.Unmarshal(out, &decoded)
	plan, _ := decoded["action_plan"].(map[string]any)
	if plan["kind"] != "virtual_shopping" {
		t.Fatalf("wrapper not unwrapped: %s", out)
	}
	// An object that really declares a json property is left alone.
	out, err = NormalizeModelArguments(json.RawMessage(`{"label":{"json":"{\"a\":1}"}}`), schema)
	if err != nil || string(out) != `{"label":{"json":"{\"a\":1}"}}` {
		t.Fatalf("declared json property changed: %s %v", out, err)
	}
}
