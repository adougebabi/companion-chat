package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Going shopping now books a slot that starts at the planning minute; a later
// booking still has to be in the future.
func TestScheduledShoppingSlotStartsNowOnlyWhenAsked(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 30, 40, 0, time.UTC)
	actionPlan := map[string]any{"kind": "virtual_shopping", "duration_minutes": 60}
	plan := func(start time.Time) map[string]any {
		return map[string]any{"items": []any{map[string]any{"planned_action_slot": true, "start_at": start.Format(instantLayout), "end_at": start.Add(90 * time.Minute).Format(instantLayout)}}}
	}
	minute := now.Truncate(time.Minute)
	if _, err := scheduledAppointmentItem(plan(minute), actionPlan, map[string]any{"start_now": true}, now); err != nil {
		t.Fatalf("a start-now slot at the planning minute was rejected: %v", err)
	}
	if _, err := scheduledAppointmentItem(plan(minute), actionPlan, map[string]any{"start_now": true}, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("a slow planner call must not invalidate the start-now slot: %v", err)
	}
	if _, err := scheduledAppointmentItem(plan(now.Add(10*time.Minute)), actionPlan, map[string]any{"start_now": true}, now); err == nil {
		t.Fatal("a later slot is not going now")
	}
	if _, err := scheduledAppointmentItem(plan(minute), actionPlan, map[string]any{}, now); err == nil {
		t.Fatal("a normal booking must still start in the future")
	}
	if _, err := scheduledAppointmentItem(plan(now.Add(10*time.Minute)), actionPlan, map[string]any{}, now); err != nil {
		t.Fatalf("a future booking was rejected: %v", err)
	}
	going := scheduleReplanPlannerInstruction(SchedulePlanInput{Schedule: map[string]any{"revision": 3}, PlannedAction: map[string]any{"start_now": true}})
	later := scheduleReplanPlannerInstruction(SchedulePlanInput{Schedule: map[string]any{"revision": 3}, PlannedAction: map[string]any{"start_now": false}})
	placeLater := "Place the planned_action into exactly one future item"
	if !strings.Contains(going, "starts exactly at planned_action.preferred_start_at") || strings.Contains(going, placeLater) || !strings.Contains(later, placeLater) {
		t.Fatalf("planner instructions do not follow start_now:\n%s\n%s", going, later)
	}
	if !toolRequestStartsNow(ToolExecutionRequest{Arguments: json.RawMessage(`{"start_now":true}`)}) || toolRequestStartsNow(ToolExecutionRequest{Arguments: json.RawMessage(`{"preferred_start_at":"x"}`)}) {
		t.Fatal("start_now flag parsing")
	}
}

// Chat goes shopping through intention.schedule; the redundant directory tool
// is gone and the raw activity tool is not offered to chat.
func TestChatShoppingUsesTheScheduleTool(t *testing.T) {
	registry, err := NewCapabilityRegistry(builtinCapabilities(&App{})...)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := registry.Definition("capability.discover"); found {
		t.Fatal("capability.discover should not be installed")
	}
	for _, definition := range registry.Catalog(CapabilitySurfaceConversation) {
		if definition.Name == lifeActivityStartCapabilityName {
			t.Fatal("life.activity.start must not be offered to chat")
		}
	}
	native := registry.Catalog(CapabilitySurfaceNativeCognition)
	found := false
	for _, definition := range native {
		found = found || definition.Name == lifeActivityStartCapabilityName
	}
	if !found {
		t.Fatal("the scheduled trigger still needs life.activity.start")
	}
	definition, _ := registry.Definition(scheduleActivityCapabilityName)
	if !strings.Contains(jsonString(definition.InputSchema), "start_now") || !strings.Contains(chatToolDefinition(definition).Description, "start_now") {
		t.Fatal("intention.schedule does not offer start_now")
	}
	if !strings.Contains(directChatProtocol, "start_now=true") {
		t.Fatal("chat rule 5 does not explain going now")
	}
}
