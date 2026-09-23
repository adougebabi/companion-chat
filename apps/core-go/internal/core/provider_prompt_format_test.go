package core

import (
	"strings"
	"testing"
)

func TestFormatProviderPromptContentUsesCompactYAMLForNestedJSON(t *testing.T) {
	input := `{"context":{"name":"影者","count":2},"items":[{"kind":"user","text":"你好"},{"kind":"assistant","text":"收到"}]}`
	got := formatProviderPromptContent(input)
	for _, fragment := range []string{"\""} {
		if strings.Contains(got, fragment) {
			t.Fatalf("formatted prompt still contains %q: %s", fragment, got)
		}
	}
	if strings.Count(got, ",") > 1 {
		t.Fatalf("formatted prompt repeats comma delimiters: %s", got)
	}
	for _, fragment := range []string{"context:", "items[2]{kind,text}:", "user|你好", "assistant|收到"} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("formatted prompt lost %q: %s", fragment, got)
		}
	}
	if len(got) >= len(input) {
		t.Fatalf("formatted YAML did not reduce payload: input=%d output=%d\n%s", len(input), len(got), got)
	}
}

func TestFormatProviderMessagesLeavesInstructionsAndProseUnchanged(t *testing.T) {
	messages := []map[string]any{
		{"role": "system", "content": `Return JSON: {"ok":true}`},
		{"role": "user", "content": `{"text":"hello","enabled":true}`},
		{"role": "assistant", "content": "ordinary prose"},
	}
	formatted := formatProviderMessages(messages)
	if formatted[0]["content"] != messages[0]["content"] || formatted[2]["content"] != messages[2]["content"] {
		t.Fatalf("non-payload messages changed: %#v", formatted)
	}
	if formatted[1]["content"] == messages[1]["content"] || !strings.Contains(formatted[1]["content"].(string), "text: hello") {
		t.Fatalf("JSON user payload was not formatted: %#v", formatted[1]["content"])
	}
	if messages[1]["content"] != `{"text":"hello","enabled":true}` {
		t.Fatal("formatter mutated the input message")
	}
}

func TestFormatProviderPromptContentUsesBlockScalarForMultilineText(t *testing.T) {
	got := formatProviderPromptContent(`{"text":"第一行\n第二行"}`)
	if !strings.Contains(got, "text: |\n") || !strings.Contains(got, "  第一行\n  第二行") {
		t.Fatalf("multiline YAML scalar = %q", got)
	}
	if strings.Contains(got, `\\n`) || strings.Contains(got, `\"`) {
		t.Fatalf("multiline value still has JSON escapes: %q", got)
	}
}

func TestFormatProviderPromptContentFormatsJSONAfterAuthorityPreamble(t *testing.T) {
	got := formatProviderPromptContent("The structured payload below is authoritative.\n\n{\"context_binding\":{\"scene\":\"图书馆\"}}")
	if !strings.Contains(got, "authoritative") || !strings.Contains(got, "context_binding:") || !strings.Contains(got, "scene: 图书馆") {
		t.Fatalf("prefixed JSON was not formatted: %q", got)
	}
	if strings.Contains(got, `\"`) {
		t.Fatalf("prefixed JSON still contains escapes: %q", got)
	}
}

func TestFormatProviderMessagesUsesTOONOnlyForNonMediaContexts(t *testing.T) {
	content := `{"items":[{"id":"a","value":"one"},{"id":"b","value":"two"}]}`
	generic := formatProviderMessagesForRole([]map[string]any{{"role": "user", "content": content}}, "cognitive_assessment")
	if !strings.Contains(generic[0]["content"].(string), "items[2]{id,value}:") {
		t.Fatalf("generic context did not use compact table: %q", generic[0]["content"])
	}
	media := formatProviderMessagesForRole([]map[string]any{{"role": "user", "content": content}}, "media_prompt")
	if strings.Contains(media[0]["content"].(string), "items[2]{id,value}:") || !strings.Contains(media[0]["content"].(string), "- id: a") {
		t.Fatalf("media prompt should remain standard YAML: %q", media[0]["content"])
	}
}

func TestProviderYAMLModePropagatesThroughNestedArrays(t *testing.T) {
	value := map[string]any{
		"facts": []any{
			map[string]any{
				"kind": "current_state",
				"value": map[string]any{
					"drives": []any{
						map[string]any{"key": "rest", "pressure": 0.4},
						map[string]any{"key": "social", "pressure": 0.6},
					},
				},
			},
		},
	}

	yaml := renderProviderYAMLWithMode(value, false)
	if strings.Contains(yaml, "drives[2]{key,pressure}:") {
		t.Fatalf("YAML mode leaked a nested TOON table: %s", yaml)
	}
	if !strings.Contains(yaml, "- key: rest") || !strings.Contains(yaml, "  - key: social") {
		t.Fatalf("YAML mode lost nested array entries: %s", yaml)
	}

	toon := renderProviderYAMLWithMode(value, true)
	if !strings.Contains(toon, "drives[2]{key,pressure}:") {
		t.Fatalf("TOON mode did not render the nested homogeneous array: %s", toon)
	}

	mapSliceValue := map[string]any{
		"developing_self": []map[string]any{
			{"claim": "保持专注", "confidence": 0.8},
			{"claim": "自我保护", "confidence": 0.9},
		},
	}
	mapSliceTOON := renderProviderYAMLWithMode(mapSliceValue, true)
	if !strings.Contains(mapSliceTOON, "developing_self[2]{claim,confidence}:") {
		t.Fatalf("TOON mode failed to render []map[string]any as table: %s", mapSliceTOON)
	}
}

func TestProviderFormatterLeavesRuntimeContextEnvelopeUntouched(t *testing.T) {
	content := "[RUNTIME CONTEXT]\n{\"facts\":[{\"kind\":\"scene\",\"value\":\"雨后窗边\"}]}\n[/RUNTIME CONTEXT]"
	if got := formatProviderPromptContent(content); got != content {
		t.Fatalf("runtime context envelope was implicitly reformatted: %q", got)
	}
}

func TestMediaPromptUserPayloadIsOnlyFormattedYAML(t *testing.T) {
	content := `{"context_binding":{"life_context":{"scene":"图书馆"}},"scene":"图书馆","action":"阅读"}`
	formatted := formatProviderMessagesForRole([]map[string]any{{"role": "user", "content": content}}, "media_prompt")
	got := formatted[0]["content"].(string)
	if strings.Contains(got, "authoritative context_binding") || strings.Contains(got, "The structured JSON payload") {
		t.Fatalf("media prompt authority preamble leaked into user payload: %q", got)
	}
	for _, fragment := range []string{"context_binding:", "scene: 图书馆", "action: 阅读"} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("media prompt YAML missing %q: %q", fragment, got)
		}
	}
}
