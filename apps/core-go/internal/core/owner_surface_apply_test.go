package core

import (
	"context"
	"testing"
)

func TestStudioApplyPersistsChoicesAndObservedRevision(t *testing.T) {
	surface := NewMemoryOwnerSurface(SurfaceHooks{InstanceRevision: func(context.Context, string, string) (int, error) { return 7, nil }})
	created, err := surface.Dispatch(context.Background(), "POST", "/internal/studio/resources", nil, "owner-1", map[string]any{
		"idempotency_key": "res-1", "kind": "character_template", "name": "模板", "body": map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	resourceID := stringValue(created["id"])
	choices := []any{map[string]any{"field": "name", "decision": "keep"}, map[string]any{"field": "tone", "decision": "override"}}
	applied, err := surface.Dispatch(context.Background(), "POST", "/internal/studio/resources/"+resourceID+"/apply", nil, "owner-1", map[string]any{
		"idempotency_key": "apply-1", "fluctlight_id": "fl-1", "resource_version": "1", "reason": "记录",
		"expected_instance_revision": 7, "choices": choices,
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied["status"] != "recorded_not_applied" || applied["applied"] != false || applied["instance_revision"] != 7 || applied["instance_revision_unchanged"] != true || applied["conflicts"] != nil {
		t.Fatalf("apply = %#v", applied)
	}
	if _, exists := applied["applied_version"]; exists {
		t.Fatalf("apply marked a version applied: %#v", applied)
	}
	records, err := surface.store.(*memorySurface).ListKind(context.Background(), "owner-1", "studio_apply", "fl-1")
	if err != nil || len(records) != 1 {
		t.Fatalf("stored applies = %#v err=%v", records, err)
	}
	stored := records[0].Payload
	storedChoices := arrayValue(stored["choices"])
	if records[0].Status != "recorded_not_applied" || stored["conflicts_resolved"] != false || stored["instance_revision"] != 7 || len(storedChoices) != 2 {
		t.Fatalf("stored apply = %#v", stored)
	}
	if stringValue(mapValue(storedChoices[0])["decision"]) != "keep" || stringValue(mapValue(storedChoices[1])["decision"]) != "override" {
		t.Fatalf("stored choices = %#v", storedChoices)
	}
	if _, err := surface.Dispatch(context.Background(), "POST", "/internal/studio/resources/"+resourceID+"/apply", nil, "owner-1", map[string]any{
		"idempotency_key": "apply-stale", "fluctlight_id": "fl-1", "resource_version": "1", "reason": "记录",
		"expected_instance_revision": 6, "choices": choices,
	}); err == nil {
		t.Fatal("stale instance revision was accepted")
	}
}
