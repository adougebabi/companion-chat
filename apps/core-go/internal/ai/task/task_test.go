package task

import (
	"reflect"
	"testing"
)

func TestObjectSchemaBuilder(t *testing.T) {
	schema := ObjectSchema(map[string]any{
		"name": StringSchema(),
		"age":  IntegerSchema(),
	}, []string{"name"}, false)

	if schema["type"] != "object" {
		t.Fatalf("expected object schema, got %v", schema["type"])
	}
	required := schema["required"].([]any)
	if len(required) != 1 || required[0] != "name" {
		t.Fatalf("unexpected required list: %v", required)
	}
}

func TestNormalizeStructuredShape(t *testing.T) {
	schema := ObjectSchema(map[string]any{
		"title":       StringSchema(),
		"count":       IntegerSchema(),
		"tags":        ArraySchema(StringSchema()),
		"is_active":   BooleanSchema(),
		"meta":        ObjectSchema(map[string]any{"source": StringSchema()}, []string{"source"}, false),
	}, []string{"title", "tags", "meta"}, false)

	input := map[string]any{
		"count": 42,
	}

	normalized, fields := NormalizeStructuredShape(input, schema)
	if len(fields) == 0 {
		t.Fatal("expected normalized missing fields")
	}
	if normalized["title"] != "" {
		t.Fatalf("expected empty string for title, got %v", normalized["title"])
	}
	if tags, ok := normalized["tags"].([]any); !ok || len(tags) != 0 {
		t.Fatalf("expected empty array for tags, got %v", normalized["tags"])
	}
}

func TestWithChineseOutputInstruction(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": "hello"},
	}
	result := WithChineseOutputInstruction("conversation", messages)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0]["role"] != "system" {
		t.Fatalf("expected system message first, got %v", result[0]["role"])
	}

	// For media_prompt, system message should not include Chinese instruction
	mediaResult := WithChineseOutputInstruction("media_prompt", messages)
	if !reflect.DeepEqual(mediaResult, messages) {
		t.Fatalf("expected media_prompt messages untouched, got %v", mediaResult)
	}
}
