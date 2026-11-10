package capability

import (
	"encoding/json"
	"reflect"
	"testing"
)

func optionalObjectSchemaFixture() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "test.optional", Version: "v1", Type: CapabilityTypeAction,
		Description: "Test optional model inputs", FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false, "required": []any{"text", "decision"},
			"properties": map[string]any{
				"text": map[string]any{"type": "string"},
				"decision": map[string]any{"type": "object", "additionalProperties": false, "required": []any{"action_type", "influences"}, "properties": map[string]any{
					"action_type": map[string]any{"type": "string"},
					"influences":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"thought":     map[string]any{"type": "string"},
					"state": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
						"mood": map[string]any{"type": "string"},
					}},
				}},
			},
		},
	}
}

func TestModelToolSchemaPreservesOnlySelectedObjectOptionalFields(t *testing.T) {
	definition := optionalObjectSchemaFixture()
	before, _ := json.Marshal(definition.InputSchema)
	strict := StrictModelSchema(definition.InputSchema)
	if got := ModelToolSchema(definition); !reflect.DeepEqual(got, strict) {
		t.Fatal("unconfigured tools changed their strict model schema")
	}
	definition.ModelPreserveOptionalObjectPaths = []string{"decision"}
	if err := definition.Validate(); err != nil {
		t.Fatal(err)
	}
	wire := ModelToolSchema(definition)
	decision := mapValue(mapValue(wire["properties"])["decision"])
	if !reflect.DeepEqual(arrayValue(decision["required"]), []any{"action_type", "influences"}) {
		t.Fatalf("model decision required=%#v", decision["required"])
	}
	if len(mapValue(decision["properties"])) != 4 {
		t.Fatal("optional fields were removed from the model schema")
	}
	state := mapValue(mapValue(decision["properties"])["state"])
	branches := arrayValue(state["anyOf"])
	if len(branches) != 2 || !reflect.DeepEqual(arrayValue(mapValue(branches[0])["required"]), []any{"mood"}) || stringValue(mapValue(branches[1])["type"]) != "null" {
		t.Fatalf("nested strict and nullable behavior changed: %#v", state)
	}
	after, _ := json.Marshal(definition.InputSchema)
	if string(before) != string(after) {
		t.Fatal("model schema transformation mutated the business schema")
	}
}

func TestModelToolSchemaRejectsInvalidOptionalObjectPaths(t *testing.T) {
	for _, paths := range [][]string{{""}, {"missing"}, {"text"}, {"decision.state"}, {"decision", "decision"}} {
		definition := optionalObjectSchemaFixture()
		definition.ModelPreserveOptionalObjectPaths = paths
		if err := definition.Validate(); err == nil {
			t.Fatalf("invalid or ambiguous optional object paths accepted: %#v", paths)
		}
	}
}
