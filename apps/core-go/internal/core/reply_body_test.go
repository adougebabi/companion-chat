package core

import (
	"encoding/json"
	"os"
	"testing"
)

func TestReplyDraftFormatDriftNeverPublishesPrivateSections(t *testing.T) {
	data, err := os.ReadFile("testdata/reply_output_drift.json")
	if err != nil {
		t.Fatal(err)
	}
	var samples []struct{ Name, Input, Want string }
	if err := json.Unmarshal(data, &samples); err != nil {
		t.Fatal(err)
	}
	for _, sample := range samples {
		t.Run(sample.Name, func(t *testing.T) {
			draft, err := parseReplyDraft(sample.Input)
			if sample.Want == "" {
				if err == nil || draft.Text != "" {
					t.Fatalf("private-only output became a reply: %q, %v", draft.Text, err)
				}
				return
			}
			if err != nil || draft.Text != sample.Want {
				t.Fatalf("body = %q, want %q; error = %v", draft.Text, sample.Want, err)
			}
			if extractReplyBody(draft.Text) != draft.Text {
				t.Fatal("publication filter is not idempotent")
			}
		})
	}
}

func TestReplyDraftMalformedEnvelopeFallsBackToCleanBody(t *testing.T) {
	for _, sample := range []struct{ raw, want string }{
		{`{"text":"partial reply"`, "partial reply"},
		{`{"text":"reply","direction":"private planning"}`, "reply"},
		{"```json\n{\"text\":\"partial\"\n```", "partial"},
	} {
		draft, err := parseReplyDraft(sample.raw)
		if err != nil || draft.Text != sample.want {
			t.Fatalf("malformed envelope body = %q, want %q, error=%v", draft.Text, sample.want, err)
		}
	}
}

func TestReplyDraftUnrecoverableEnvelopeHasNoPublishableText(t *testing.T) {
	for _, raw := range []string{
		`{"text":42}`, `{"text":null}`, `{"direction":"private planning"}`,
		`{"private":{"text":"private planning"}`, `{"text":"# 认知\nprivate reasoning"`,
	} {
		draft, err := parseReplyDraft(raw)
		if err == nil || draft.Text != "" {
			t.Fatalf("bodyless envelope retained publishable text: %q, error=%v", draft.Text, err)
		}
	}
}
