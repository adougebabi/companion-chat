package core

import "testing"

func TestLegacyTokenSettingsDoNotInvalidateReplyConfiguration(t *testing.T) {
	for _, raw := range []string{`{"max_tokens":0,"max_input_tokens":0}`, `{"max_tokens":9999999,"max_input_tokens":1}`} {
		if _, err := decodeConversationReplySettings([]byte(raw)); err != nil {
			t.Fatal(err)
		}
	}
}
