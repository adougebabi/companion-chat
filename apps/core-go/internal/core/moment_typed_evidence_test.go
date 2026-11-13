package core

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMomentTypedEvidencePreservesBusinessDataThroughPhysicalWriter(t *testing.T) {
	business := map[string]any{"id": "business-id", "source_ref": "business-ref", "source": "business-source", "revision": 7, "valid": false, "count": 0, "unknown": nil}
	world := map[string]any{"name": "测试世界", "source": "owner_world_setting", "background": business, "rules": business, "flavor": business, "revision": 9}
	world["current_place"] = map[string]any{"id": "private-place", "name": "客厅", "source": "event", "atmosphere": business}
	world["known_npcs"] = []any{map[string]any{"actor_id": "private-npc", "name": "邻居", "relationship_summary": "已认识", "source": "recorded_relationship"}}
	world["active_world_effects"] = []any{map[string]any{"run_id": "private-run", "event_id": "private-world-event", "source": "virtual_world_event", "description": "环境变化", "effects": []any{map[string]any{"kind": "weather", "scope": "world", "value": business, "place_id": "private-place"}}}}
	rows := map[ContextReferenceKind]map[string]any{
		ContextReferenceScene:            {"source": "event", "authority_status": "confirmed", "scene": "客厅", "world_environment": world, "event_id": "private-event"},
		ContextReferenceLifeContext:      {"source": "schedule", "authority_status": "planned", "activity": "阅读", "world_environment": world, "schedule_id": "private-schedule"},
		ContextReferenceSchedule:         {"local_date": "2026-11-13", "timezone": "Asia/Shanghai", "status": "accepted", "reschedule_policy": business, "current_item_id": "private-item"},
		ContextReferenceScheduleItem:     {"activity": "阅读", "status": "pending", "priority": 0, "promise_schedule_valid": false, "promise_state": "cancelled", "action_plan": map[string]any{"private_plan": true}, "intention_id": "private-intention"},
		ContextReferencePresence:         {"current_task": "阅读", "user_presence": "remote", "actor_id": "private-actor"},
		ContextReferenceState:            {"mood": map[string]any{"label": "calm", "source": "affect_event", "intensity": 0.2}, "revision": 9},
		ContextReferenceOutcome:          {"capability_name": "wardrobe.wear", "status": "failed", "error_code": "item_unavailable", "request_digest": "private-digest", "goal_refs": []any{"private-goal"}, "intention_refs": []any{"private-intention"}, "expected": map[string]any{"private_expected": true}, "observed": map[string]any{"status": "failed", "private_observed": true}},
		ContextReferenceEvolutionOverlay: {"kind": "habit", "field_path": "behavioral_policy.response_style", "semantic_direction": "更直接", "confidence": 0.8, "policy_version": "private-policy"},
	}
	input := replyContextFixture()
	input.Scene = replySceneMoment
	for kind, row := range rows {
		input.ToolFacts = append(input.ToolFacts, momentReplyEvidenceFact(ContextReference{Kind: kind, Snapshot: jsonBytes(row)}, input.Projection))
	}
	if fact := momentReplyEvidenceFact(ContextReference{Kind: ContextReferenceAffectProfile, Snapshot: jsonBytes(map[string]any{"decay_policy": map[string]any{"private_reducer": true}, "policy_version": "private-policy"})}, input.Projection); fact != nil {
		t.Error("Core numeric affect policy became additional Writer evidence")
		input.ToolFacts = append(input.ToolFacts, fact)
	}
	var request map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = payload
		return fakeProviderResult{Text: "今天留一点时间读书。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	if _, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: time.Second}, Concurrency: 1}); err != nil {
		t.Fatal(err)
	}
	var runtime map[string]any
	for _, raw := range arrayValue(request["messages"]) {
		content := stringValue(mapValue(raw)["content"])
		if strings.HasPrefix(content, "[RUNTIME CONTEXT]\n") {
			text := strings.TrimSuffix(strings.TrimPrefix(content, "[RUNTIME CONTEXT]\n"), "\n[/RUNTIME CONTEXT]")
			if err := json.Unmarshal([]byte(text), &runtime); err != nil {
				t.Fatal(err)
			}
		}
	}
	facts := map[ContextReferenceKind]map[string]any{}
	for _, raw := range arrayValue(runtime["tool_facts"]) {
		fact := mapValue(raw)
		facts[ContextReferenceKind(stringValue(fact["kind"]))] = mapValue(fact["value"])
	}
	for _, kind := range []ContextReferenceKind{ContextReferenceScene, ContextReferenceLifeContext} {
		if facts[kind]["source"] != rows[kind]["source"] || jsonString(mapValue(facts[kind]["world_environment"])["background"]) != jsonString(business) || jsonString(mapValue(facts[kind]["world_environment"])["rules"]) != jsonString(business) {
			t.Errorf("%s source or world business data changed: %s", kind, jsonString(facts[kind]))
		}
		if jsonString(mapValue(mapValue(facts[kind]["world_environment"])["current_place"])["atmosphere"]) != jsonString(business) {
			t.Errorf("%s place atmosphere business JSON changed", kind)
		}
	}
	if jsonString(facts[ContextReferenceSchedule]["reschedule_policy"]) != jsonString(business) {
		t.Error("schedule policy business JSON changed")
	}
	if mapValue(facts[ContextReferenceState]["mood"])["source"] != "affect_event" {
		t.Error("mood lost its semantic source")
	}
	if item := facts[ContextReferenceScheduleItem]; item["priority"] != float64(0) || item["promise_schedule_valid"] != false || item["promise_state"] != "cancelled" {
		t.Errorf("schedule item semantic values changed: %s", jsonString(item))
	}
	if facts[ContextReferenceOutcome]["status"] != "failed" || mapValue(facts[ContextReferenceOutcome]["observed"])["status"] != "failed" {
		t.Error("real outcome status was lost")
	}
	for _, forbidden := range []string{"private-event", "private-schedule", "private-item", "private-intention", "private-actor", "private-digest", "private-goal", "private_plan", "private_expected", "private_observed", "private-policy", "private_reducer", "private-place", "private-npc", "private-run", "private-world-event"} {
		if strings.Contains(jsonString(runtime["tool_facts"]), forbidden) {
			t.Errorf("typed evidence exposed Core data %q", forbidden)
		}
	}
}
