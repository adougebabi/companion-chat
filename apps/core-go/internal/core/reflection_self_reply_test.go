package core

import (
	"strings"
	"testing"
)

func TestReflectionEvidenceCarriesTheCharactersOwnReply(t *testing.T) {
	evidence := []map[string]any{{
		"id": "fact-1", "sequence": 7, "event_type": "conversation.turn",
		"payload": map[string]any{"text": "你去买条裙子吧", reflectionSelfReplyKey: "不用买啦，我衣柜里有一条红色长裙"},
	}}
	compact := compactReflectionEvidenceV2(evidence)
	if len(compact) != 1 || compact[0]["observation"] != "你去买条裙子吧" || compact[0]["self_reply"] != "不用买啦，我衣柜里有一条红色长裙" {
		t.Fatalf("reflection did not receive the character's own words: %#v", compact)
	}
	if !strings.Contains(reflectionSelfConsistencyRule, "current_wardrobe") || !strings.Contains(reflectionSelfConsistencyRule, "behavior_policy_evolution_candidates") {
		t.Fatal("self-consistency rule lost its wardrobe check or correction channel")
	}
}
