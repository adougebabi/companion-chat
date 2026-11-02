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

func TestReplyDraftInvalidEnvelopeHasNoPublishableText(t *testing.T) {
	for _, raw := range []string{
		`{"text":"partial reply"`,
		`{"text":"reply","direction":"private planning"}`,
		"```json\n{\"text\":\"partial\"\n```",
	} {
		draft, err := parseReplyDraft(raw)
		if err == nil || draft.Text != "" {
			t.Fatalf("invalid envelope retained publishable text: %q, error=%v", draft.Text, err)
		}
	}
}
