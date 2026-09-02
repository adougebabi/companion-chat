package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAlignMediaConceptWithContextKeepsVisualFieldsAndRepairsState(t *testing.T) {
	projection := ContextProjection{
		FluctlightID:    "fl-1",
		SourceFactID:    "fact-1",
		ContextRevision: 7,
		LifeContext:     map[string]any{"scene": "教室/图书馆", "activity": "大学课程/自习", "location": "校内"},
		InnerState:      map[string]any{"mood": map[string]any{"label": "专注"}},
		Identity:        map[string]any{"appearance": map[string]any{"hair": "长发"}},
	}
	concept, fields := alignMediaConceptWithContext(map[string]any{
		"subject":  "苏星洛",
		"pose":     "举起手机自拍",
		"scene":    "卧室",
		"activity": "睡觉",
		"outfit":   "JK制服",
	}, projection)
	if concept["subject"] != "苏星洛" || concept["pose"] != "举起手机自拍" || concept["outfit"] != "JK制服" {
		t.Fatalf("normal visual fields changed: %#v", concept)
	}
	if concept["scene"] != "教室/图书馆" || concept["activity"] != "大学课程/自习" || concept["location"] != "校内" {
		t.Fatalf("context fields were not aligned: %#v", concept)
	}
	binding := mapValue(concept["context_binding"])
	if binding["source"] != "cognition.life_context" || binding["context_revision"] != 7 {
		t.Fatalf("context binding = %#v", binding)
	}
	if len(fields) != 5 || fields[0] != "activity" || fields[1] != "hair" || fields[2] != "location" || fields[3] != "mood" || fields[4] != "scene" {
		t.Fatalf("changed fields = %#v", fields)
	}
}

func TestAlignMediaConceptHonorsExplicitContextOverride(t *testing.T) {
	projection := ContextProjection{LifeContext: map[string]any{"scene": "图书馆"}}
	concept, fields := alignMediaConceptWithContext(map[string]any{
		"scene":            "卧室",
		"context_override": map[string]any{"explicit": true},
	}, projection)
	if concept["scene"] != "卧室" || len(fields) != 0 {
		t.Fatalf("explicit override was changed: concept=%#v fields=%#v", concept, fields)
	}
}

func TestBindMediaContextToToolCallsAddsFrozenSnapshot(t *testing.T) {
	projection := ContextProjection{FluctlightID: "fl-1", SourceFactID: "fact-1", ContextRevision: 3, LifeContext: map[string]any{"scene": "图书馆", "activity": "自习"}}
	calls := bindMediaContextToToolCalls([]ToolCallV1{{ID: "call-1", Name: "media.image.generate", Arguments: json.RawMessage(`{"concept":{"scene":"卧室","subject":"苏星洛"}}`)}}, projection)
	if len(calls) != 1 {
		t.Fatalf("calls = %#v", calls)
	}
	var args map[string]any
	if err := json.Unmarshal(calls[0].Arguments, &args); err != nil {
		t.Fatal(err)
	}
	concept := mapValue(args["concept"])
	if concept["scene"] != "图书馆" || concept["subject"] != "苏星洛" {
		t.Fatalf("bound concept = %#v", concept)
	}
	if binding := mapValue(concept["context_binding"]); binding["context_revision"] != float64(3) {
		t.Fatalf("bound context = %#v", binding)
	}
}

func TestBindMediaPromptContextAddsAuthorityOnlyWhenSnapshotExists(t *testing.T) {
	if got := bindMediaPromptContext(`{"subject":"a cat"}`); got != `{"subject":"a cat"}` {
		t.Fatalf("prompt without snapshot changed: %q", got)
	}
	got := bindMediaPromptContext(`{"context_binding":{"life_context":{"scene":"图书馆"}}}`)
	if len(got) <= len(`{"context_binding":{"life_context":{"scene":"图书馆"}}}`) || !strings.Contains(got, "authoritative context_binding") {
		t.Fatalf("prompt with snapshot was not annotated: %q", got)
	}
}

func TestWithContextAuthorityInstructionKeepsUserMessageLast(t *testing.T) {
	messages := withContextAuthorityInstruction([]map[string]any{
		{"role": "system", "content": "decide"},
		{"role": "user", "content": "current request"},
	})
	if len(messages) != 3 || stringValue(messages[1]["role"]) != "system" || stringValue(messages[2]["role"]) != "user" {
		t.Fatalf("messages = %#v", messages)
	}
	if !strings.Contains(stringValue(messages[1]["content"]), "authoritative cognition-time snapshot") {
		t.Fatalf("authority instruction = %#v", messages[1])
	}
}
