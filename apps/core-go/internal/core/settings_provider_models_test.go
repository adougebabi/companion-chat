package core

import "testing"

func TestProviderModelIDsAcceptsCommonModelListShapes(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		want    []string
	}{
		{
			name: "openai data objects",
			payload: map[string]any{
				"data": []any{
					map[string]any{"id": "gpt-4o"},
					map[string]any{"id": "text-embedding-3-small"},
				},
			},
			want: []string{"gpt-4o", "text-embedding-3-small"},
		},
		{
			name: "ollama models",
			payload: map[string]any{
				"models": []any{
					map[string]any{"name": "llama3.1:8b"},
					map[string]any{"model": "qwen2.5:7b"},
				},
			},
			want: []string{"llama3.1:8b", "qwen2.5:7b"},
		},
		{
			name: "mixed duplicate entries",
			payload: map[string]any{
				"data": []any{
					"model-a",
					map[string]any{"id": "model-a"},
					map[string]any{"name": "model-b"},
				},
			},
			want: []string{"model-a", "model-b"},
		},
		{
			name: "top-level array",
			payload: []any{
				map[string]any{"id": "model-c"},
				"model-d",
			},
			want: []string{"model-c", "model-d"},
		},
		{
			name: "nested data envelope",
			payload: map[string]any{
				"data": map[string]any{
					"models": []any{map[string]any{"name": "model-e"}},
				},
			},
			want: []string{"model-e"},
		},
		{
			name:    "unknown envelope",
			payload: map[string]any{"items": []any{map[string]any{"label": "model-f"}}},
			want:    []string{},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := providerModelIDs(testCase.payload); !equalStrings(got, testCase.want) {
				t.Fatalf("providerModelIDs() = %#v, want %#v", got, testCase.want)
			}
		})
	}
}

func TestNormalizeProviderQueueSettingsClampsOnlyAcceptedRange(t *testing.T) {
	valid := normalizeProviderQueueSettings(map[string]any{"generated_concurrency": float64(4), "embedding_concurrency": float64(2)})
	if valid == nil || valid["generated_concurrency"] != 4 || valid["embedding_concurrency"] != 2 {
		t.Fatalf("valid queue settings = %#v", valid)
	}
	for _, value := range []map[string]any{
		{"generated_concurrency": float64(0)},
		{"embedding_concurrency": float64(9)},
		{"generated_concurrency": 1.5},
		{"unknown": float64(2)},
	} {
		if got := normalizeProviderQueueSettings(value); got != nil {
			t.Fatalf("invalid queue settings %#v normalized to %#v", value, got)
		}
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
