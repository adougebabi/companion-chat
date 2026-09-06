package core

import (
	"strings"
	"testing"
)

func TestNormalizeVisibleReplyExtractsNestedActionContent(t *testing.T) {
	got := normalizeVisibleReply(`{"action":{"action_type":"send_message","content":"你好，今天过得怎么样？"}}`)
	if got != "你好，今天过得怎么样？" {
		t.Fatalf("normalized visible reply = %q", got)
	}
}

func TestNormalizeVisibleReplyExtractsFencedNestedActionContent(t *testing.T) {
	got := normalizeVisibleReply("```json\n{\n  \"action\": {\n    \"action_type\": \"send_message\",\n    \"content\": \"围栏里的消息\"\n  }\n}\n```")
	if got != "围栏里的消息" {
		t.Fatalf("normalized fenced visible reply = %q", got)
	}
}

func TestNormalizeVisibleReplyLeavesPlainTextUnchanged(t *testing.T) {
	plain := "我在图书馆看书。"
	if got := normalizeVisibleReply(plain); got != plain {
		t.Fatalf("plain visible reply = %q, want %q", got, plain)
	}
}

func TestVisibleReplyStreamFiltersActionWrapperBeforeEmission(t *testing.T) {
	var emitted []string
	push, wasEmitted := newVisibleReplyStream(func(chunk string) error {
		emitted = append(emitted, chunk)
		return nil
	})
	for _, chunk := range []string{`{"action":{"action_type":"send_`, `message","content":"只保留这句话"}}`} {
		if err := push(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if !wasEmitted() || len(emitted) != 1 || emitted[0] != "只保留这句话" {
		t.Fatalf("emitted visible chunks = %#v", emitted)
	}
	if strings.Contains(strings.Join(emitted, ""), "action_type") {
		t.Fatalf("action protocol leaked: %#v", emitted)
	}
}

func TestVisibleReplyStreamFiltersFencedActionWrapperBeforeEmission(t *testing.T) {
	var emitted []string
	push, wasEmitted := newVisibleReplyStream(func(chunk string) error {
		emitted = append(emitted, chunk)
		return nil
	})
	for _, chunk := range []string{"```json\n{\n  \"action\": {\n    \"action_type\": \"send_message\",\n    \"content\": \"围栏流式消息\"\n  }\n}\n```"} {
		if err := push(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if !wasEmitted() || len(emitted) != 1 || emitted[0] != "围栏流式消息" {
		t.Fatalf("emitted fenced visible chunks = %#v", emitted)
	}
}
