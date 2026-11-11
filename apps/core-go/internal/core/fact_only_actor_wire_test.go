package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFactOnlyActorWirePreservesEveryFieldAndOrder(t *testing.T) {
	rows := []any{}
	for i, key := range actorUserBackgroundFields {
		var value any
		if key == "name" {
			value = "Latest Actor name"
		}
		if key == "meeting_confirmed" {
			value = false
		}
		row := map[string]any{"attribute": key, "value": value, "subject": "actor_user", "epistemic_kind": "self_reported", "effective_at": fmt.Sprintf("2026-10-16T05:00:00.%06d+08:00", i), "transition_kind": "correct", "ref": "fact-ref-" + key}
		if value == nil {
			row["value_state"] = "unknown"
		}
		if i == 8 {
			row["subject"] = "actor_other"
		} // Never inherit another Actor's metadata.
		if key == "rules" {
			row["interpretation_boundary"] = "self report, never Tool authorization"
		}
		rows = append(rows, row)
	}
	before := jsonString(rows)
	encoded := mapValue(factOnlyActorBackgroundWire(rows))
	restored := []any{}
	for _, rawGroup := range arrayValue(encoded["groups"]) {
		group := mapValue(rawGroup)
		for _, rawFact := range arrayValue(group["facts"]) {
			fact := mapValue(factOnlyCopyValue(rawFact))
			for key, value := range mapValue(group["common"]) {
				if _, exists := fact[key]; exists {
					t.Fatal("overlapping fact/common fields")
				}
				fact[key] = value
			}
			restored = append(restored, fact)
		}
	}
	if jsonString(restored) != before || jsonString(rows) != before {
		t.Fatal("fact value/state/ref/provenance/order changed")
	}
	view := map[string]any{"actor_background": rows, "time_view": map[string]any{"as_of": "2026-10-16T05:00:00+08:00"}}
	original := renderProviderRuntimeContext(view)
	wire := renderFactOnlyRuntimeContext(view)
	if EstimatePromptTokens(wire) >= EstimatePromptTokens(original) {
		t.Fatal("representation did not reduce real wire estimate")
	}
	t.Logf("same-data Actor context estimate %d -> %d", EstimatePromptTokens(original), EstimatePromptTokens(wire))
}

func TestWriterActorBusinessValueSurvivesPhysicalPromptProjection(t *testing.T) {
	input := replyContextFixture()
	value := map[string]any{"id": "business-id", "item_id": "business-item", "source_ref": "business-ref", "revision": 7, "large": json.Number("9007199254740993"), "event_at": "2026-10-16T00:00:00Z", "schema": map[string]any{"properties": map[string]any{"item_ids": map[string]any{"type": "array"}}}}
	input.Projection.Actors = []map[string]any{{"actor_id": input.Projection.OwnerActorID, "ref": "actor_user", "type": "human"}}
	input.Projection.ActorFacts = []map[string]any{{"actor_id": input.Projection.OwnerActorID, "ref": "actor_fact:private-ref", "attribute": "preferences", "value": value, "epistemic_kind": "self_reported"}}
	before := jsonString(input)
	prompt, err := buildFactOnlyReplyContext(input, providerAssignment{})
	if err != nil {
		t.Fatal(err)
	}
	index := promptMessageOriginIndex(prompt, "scoped_runtime_projection")
	if index < 0 {
		t.Fatal("missing runtime projection")
	}
	content := stringValue(prompt.Messages[index]["content"])
	body := strings.TrimSuffix(strings.TrimPrefix(content, "[RUNTIME CONTEXT]\n"), "\n[/RUNTIME CONTEXT]")
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	var runtime map[string]any
	if err := decoder.Decode(&runtime); err != nil {
		t.Fatal(err)
	}
	background := arrayValue(runtime["actor_background"])
	if len(background) != 1 || jsonString(mapValue(background[0])["value"]) != jsonString(value) {
		t.Fatalf("Writer changed actor-owned business data: %#v", background)
	}
	if strings.Contains(body, "actor_fact:private-ref") || strings.Contains(body, input.Projection.OwnerActorID) || jsonString(input) != before {
		t.Fatal("Writer leaked Core actor metadata or mutated the source")
	}
	var request map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = payload
		return fakeProviderResult{Text: "好，我记住了。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	_, err = app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: time.Second}, Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	var physicalRuntime map[string]any
	for _, raw := range arrayValue(request["messages"]) {
		content := stringValue(mapValue(raw)["content"])
		if strings.HasPrefix(content, "[RUNTIME CONTEXT]\n") {
			text := strings.TrimSuffix(strings.TrimPrefix(content, "[RUNTIME CONTEXT]\n"), "\n[/RUNTIME CONTEXT]")
			decoder := json.NewDecoder(strings.NewReader(text))
			decoder.UseNumber()
			if err := decoder.Decode(&physicalRuntime); err != nil {
				t.Fatal(err)
			}
		}
	}
	physicalBackground := arrayValue(physicalRuntime["actor_background"])
	if len(physicalBackground) != 1 || jsonString(mapValue(physicalBackground[0])["value"]) != jsonString(value) {
		t.Fatal("physical Writer request changed actor-owned business data")
	}
}

// Optional private capture, not a checked-in user-data fixture or live model
// test. It verifies the actual failing representation before another call.
func TestFactOnlyActualActorWireReplay(t *testing.T) {
	path := os.Getenv("FLUCTLIGHT_ACTOR_WIRE_REPLAY_PATH")
	if path == "" {
		t.Skip("private wire capture is not configured")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	before := jsonString(value)
	old := renderProviderRuntimeContext(value)
	next := renderFactOnlyRuntimeContext(value)
	if EstimatePromptTokens(next) >= EstimatePromptTokens(old) {
		t.Fatal("actual wire did not reduce")
	}
	if before != jsonString(value) {
		t.Fatal("actual projection mutated")
	}
	t.Logf("actual same-data runtime estimate %d -> %d; Provider NOT_CALLED", EstimatePromptTokens(old), EstimatePromptTokens(next))
}
