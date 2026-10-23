package core

import (
	"errors"
	"strings"
	"unicode"
)

// A quality repair already consumed the one tool-free correction. Higher-level
// format/fallback policy must not reopen tools or replace its frozen requests.
var errReplyQualityRepair = errors.New("reply_quality_repair_failed")

// Detect the concrete isolated script-tail corruption without erasing authored
// multilingual content. Known names/quotes and explicit language requests pass.
func unexpectedReplyScriptTail(text, source string) bool {
	runes := []rune(strings.TrimSpace(text))
	end := len(runes)
	start := end
	for start > 0 && unicode.Is(unicode.Gujarati, runes[start-1]) {
		start--
	}
	if start == end || end-start > 12 {
		return false
	}
	tail := string(runes[start:])
	if strings.Contains(source, tail) || strings.Contains(strings.ToLower(source), "gujarati") || strings.Contains(source, "古吉拉特") {
		return false
	}
	for _, r := range runes[:start] {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
func replyQualitySource(input ReplyContextInput) string {
	facts := []map[string]any{}
	for _, fact := range input.ToolFacts {
		if fact["kind"] != "private_candidate_text" {
			facts = append(facts, fact)
		}
	}
	return jsonString(map[string]any{"input": input.CurrentInput, "persona": input.WorkingPersona, "projection": input.Projection, "facts": facts})
}
