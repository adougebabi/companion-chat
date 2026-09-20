package task

// ObjectSchema builds a JSON Schema object specification.
func ObjectSchema(properties map[string]any, required []string, additionalProperties bool) map[string]any {
	result := map[string]any{"type": "object", "properties": properties, "additionalProperties": additionalProperties}
	if len(required) > 0 {
		values := make([]any, len(required))
		for index, value := range required {
			values[index] = value
		}
		result["required"] = values
	}
	return result
}

func StringSchema() map[string]any  { return map[string]any{"type": "string"} }
func NumberSchema() map[string]any  { return map[string]any{"type": "number"} }
func IntegerSchema() map[string]any { return map[string]any{"type": "integer", "minimum": 0} }
func BooleanSchema() map[string]any { return map[string]any{"type": "boolean"} }

func EnumStringSchema(values ...string) map[string]any {
	enumValues := make([]any, len(values))
	for index, value := range values {
		enumValues[index] = value
	}
	return map[string]any{"type": "string", "enum": enumValues}
}

func UnitNumberSchema() map[string]any {
	return map[string]any{"type": "number", "minimum": 0, "maximum": 1}
}

func ArraySchema(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}
