package core

import (
	"strings"
	"testing"
)

func TestFormatProviderPromptContentUsesCompactYAMLForNestedJSON(t *testing.T) {
	input := `{"context":{"name":"影者","count":2},"items":[{"kind":"user","text":"你好"},{"kind":"assistant","text":"收到"}]}`
	got := formatProviderPromptContent(input)
	for _, fragment := range []string{"\"", ","} {
		if strings.Contains(got, fragment) {
			t.Fatalf("formatted prompt still contains %q: %s", fragment, got)
		}
	}
	for _, fragment := range []string{"context:", "items:", "kind: user", "kind: assistant"} {
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
