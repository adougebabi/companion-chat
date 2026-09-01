package bff

import "testing"

func TestBrowserCapabilityRequestsMapsOwnerSafeFields(t *testing.T) {
	value := browserCapabilityRequests([]any{map[string]any{
		"id": "request-1", "capability_key": "calendar.read", "fluctlight_id": "fl-1",
		"aggregate_count": 2, "evidence_refs": []any{"fact-1"}, "status": "proposed",
	}})
	if len(value) != 1 {
		t.Fatalf("mapped requests = %#v", value)
	}
	item := object(value[0])
	if item["capabilityKey"] != "calendar.read" || item["fluctlightId"] != "fl-1" || item["aggregateCount"] != 2 {
		t.Fatalf("mapped request = %#v", item)
	}
}

func TestValidateCapabilityRequestReviewRequiresVersionForFulfilled(t *testing.T) {
	if validateCapabilityRequestReview(map[string]any{"status": "fulfilled", "note": "done"}) {
		t.Fatal("fulfilled request without capability version should be rejected")
	}
	if !validateCapabilityRequestReview(map[string]any{"status": "fulfilled", "note": "done", "capabilityVersion": "v1"}) {
		t.Fatal("fulfilled request with capability version should be accepted")
	}
}
