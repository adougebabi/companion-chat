package core

import (
	"context"
	"testing"
)

type dummySchedulePlanner struct{}

func (d dummySchedulePlanner) Plan(ctx context.Context, input SchedulePlanInput) (map[string]any, error) {
	return map[string]any{"items": []any{}}, nil
}

func TestScheduleServiceInitializationAndAccessors(t *testing.T) {
	planner := dummySchedulePlanner{}
	service := NewScheduleService(nil, planner, nil)

	if service == nil {
		t.Fatal("expected non-nil ScheduleService")
	}
	if service.Planner() == nil {
		t.Fatal("expected non-nil planner from ScheduleService")
	}

	service.SetPlanner(nil)
	if service.Planner() != nil {
		t.Fatal("expected nil planner after SetPlanner(nil)")
	}
}
