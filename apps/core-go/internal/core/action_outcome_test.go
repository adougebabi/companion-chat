package core

import (
	"strings"
	"testing"
)

func TestBuiltInCapabilitiesDeclareSuccessBoundaries(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	for _, definition := range registry.Definitions() {
		if definition.SuccessBoundary == "" {
			t.Errorf("capability %s does not declare a success boundary", definition.Name)
		}
	}
}

func TestBuildActionOutcomesPreservesCallIdentityAndDecisionCausality(t *testing.T) {
	goalRef := "goal:ctx_0123456789abcdef0123456789abcdef"
	intentionRef := "intention:ctx_0123456789abcdef0123456789abcdef"
	registry := mustCapabilityRegistry(
		&canonicalTestCapability{definition: CapabilityDefinition{
			Name: "query.one", Version: "v1", Type: CapabilityTypeQuery, Description: "Read one test value.", InputSchema: map[string]any{"type": "object"},
			SideEffectClass: "read_only", SuccessBoundary: "query_result_available", FailurePolicy: FailurePolicyOptionalInternal,
		}},
		&canonicalTestCapability{definition: CapabilityDefinition{
			Name: "mutation.two", Version: "v1", Type: CapabilityTypeAction, Description: "Apply one test mutation.", InputSchema: map[string]any{"type": "object"},
			SideEffectClass: "native_projection", SuccessBoundary: "state_projection_committed", FailurePolicy: FailurePolicyOptionalInternal,
		}},
	)
	causality := map[string]any{
		"goal_refs":      []any{goalRef},
		"intention_refs": []any{intentionRef},
		"context_references": map[string]any{
			goalRef:      map[string]any{"ref": goalRef, "kind": "goal", "entity_id": "goal_internal", "revision": 2, "scope": "internal", "snapshot": map[string]any{}},
			intentionRef: map[string]any{"ref": intentionRef, "kind": "intention", "entity_id": "intention_internal", "revision": 3, "scope": "internal", "snapshot": map[string]any{}},
		},
	}
	results := []CapabilityResult{
		{CallID: "call-1", CapabilityName: "query.one", Status: "completed", Output: map[string]any{"found": true}},
		{CallID: "call-2", CapabilityName: "mutation.two", Status: "failed", ErrorCode: "mutation_failed", Retryable: true},
	}
	outcomes, err := buildActionOutcomes("action-1", "fluctlight-1", "fact-1", "capability", results, causality, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 3 {
		t.Fatalf("outcomes=%d want 3", len(outcomes))
	}
	if outcomes[0].CallID != actionPrimaryCallID || outcomes[1].ID == outcomes[2].ID || outcomes[1].CallID != "call-1" || outcomes[2].CallID != "call-2" {
		t.Fatalf("call identity lost: %#v", outcomes)
	}
	if outcomes[1].SuccessBoundary != "query_result_available" || outcomes[2].SuccessBoundary != "state_projection_committed" {
		t.Fatalf("success boundaries=%q/%q", outcomes[1].SuccessBoundary, outcomes[2].SuccessBoundary)
	}
	if len(outcomes[1].GoalRefs) != 1 || outcomes[1].GoalRefs[0] != goalRef || len(outcomes[1].IntentionRefs) != 1 || outcomes[1].IntentionRefs[0] != intentionRef {
		t.Fatalf("decision service refs lost: %#v", outcomes[1])
	}
	if outcomes[2].Status != ActionOutcomeFailed || outcomes[2].ErrorCode != "mutation_failed" {
		t.Fatalf("failure semantics lost: %#v", outcomes[2])
	}
	replay, err := buildActionOutcomes("action-1", "fluctlight-1", "fact-1", "capability", results, causality, registry)
	if err != nil || replay[0].ID != outcomes[0].ID || replay[1].ID != outcomes[1].ID || replay[2].ID != outcomes[2].ID {
		t.Fatalf("outcome replay identity changed: %#v err=%v", replay, err)
	}
}

func TestBuildActionOutcomeWithoutCapabilityUsesPrimaryIdentity(t *testing.T) {
	outcomes, err := buildActionOutcomes("action-2", "fluctlight-2", "fact-2", "no_op", nil, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || outcomes[0].CallID != actionPrimaryCallID || outcomes[0].Status != ActionOutcomeCompleted || outcomes[0].SuccessBoundary != "action_settled" {
		t.Fatalf("unexpected primary outcome: %#v", outcomes)
	}
}

func TestExternalAsyncOutcomeSeparatesDurableIntentFromFinalCompletion(t *testing.T) {
	registry := mustCapabilityRegistry(imageGenerateCapability{service: nil})
	results := []CapabilityResult{{
		CallID: "image-call", CapabilityName: "media.image.generate", Status: "completed",
		Output: map[string]any{"media_intent_id": "media-intent-1", "target_kind": "conversation_message", "target_ref": "message-1"},
	}}
	outcomes, err := buildActionOutcomes("action-image", "fluctlight-image", "fact-image", "reply", results, map[string]any{"status": "completed"}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("outcomes=%#v", outcomes)
	}
	call := outcomes[1]
	if call.Status != ActionOutcomePending || call.SuccessBoundary != "durable_media_intent_created" || call.CompletionBoundary != "final_media_asset_ready" || call.ExternalRef != "media-intent-1" {
		t.Fatalf("async outcome collapsed intent/final boundary: %#v", call)
	}
}

func TestCancelledActionCancelsUnsettledCapabilityOutcome(t *testing.T) {
	registry := mustCapabilityRegistry(&canonicalTestCapability{definition: CapabilityDefinition{
		Name: "mutation.cancel", Version: "v1", Type: CapabilityTypeAction, Description: "Test cancelled mutation.",
		InputSchema: map[string]any{"type": "object"}, SideEffectClass: "native_projection",
		SuccessBoundary: "mutation_committed", FailurePolicy: FailurePolicyOptionalInternal,
	}})
	outcomes, err := buildActionOutcomes("cancel-action", "cancel-fluctlight", "cancel-fact", "capability", []CapabilityResult{{
		CallID: "cancel-call", CapabilityName: "mutation.cancel", Status: "deferred",
	}}, map[string]any{"status": "cancelled"}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 || outcomes[0].Status != ActionOutcomeCancelled || outcomes[1].Status != ActionOutcomeCancelled {
		t.Fatalf("cancelled outcomes=%#v", outcomes)
	}
}

func TestCompactRecentActionOutcomesUsesStrictProviderAllowlist(t *testing.T) {
	ref := "outcome:ctx_0123456789abcdef0123456789abcdef"
	goalRef := "goal:ctx_0123456789abcdef0123456789abcdef"
	stateRef := "state:ctx_0123456789abcdef0123456789abcdef"
	compact := compactRecentActionOutcomes([]map[string]any{{
		"ref": ref, "id": "outcome_internal", "action_id": "action_internal", "call_id": "call_internal",
		"capability_name": "media.image.generate", "status": "completed", "success_boundary": "durable_media_intent_created",
		"occurred_at": "2026-09-10T12:00:00Z", "error_code": "", "goal_refs": []any{goalRef},
		"expected": map[string]any{"action_type": "capability", "text": "private expected text"},
		"observed": map[string]any{
			"status": "completed", "target_kind": "conversation_message", "delivery_status": "queued",
			"text": "private assistant text", "target_ref": "message_internal", "media_intent_id": "media_internal",
			"resulting_state_ref": stateRef, "resulting_state_revision": 8,
		},
		"context_references": map[string]any{ref: map[string]any{"entity_id": "internal_entity"}},
		"evidence_refs":      []any{"fact_internal"}, "provenance": map[string]any{"provider_request_id": "provider_internal"},
		"revision": 4,
	}})
	if len(compact) != 1 {
		t.Fatalf("compact outcomes=%#v", compact)
	}
	encoded := string(jsonBytes(compact))
	for _, allowed := range []string{ref, goalRef, stateRef, "media.image.generate", "durable_media_intent_created", "target_kind", "delivery_status"} {
		if !strings.Contains(encoded, allowed) {
			t.Fatalf("allowlisted field %q missing from %s", allowed, encoded)
		}
	}
	for _, forbidden := range []string{
		"outcome_internal", "action_internal", "call_internal", "private expected text", "private assistant text",
		"message_internal", "media_internal", "context_references", "internal_entity", "evidence_refs", "fact_internal",
		"provenance", "provider_internal", "expected", "revision",
	} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("provider outcome leaked %q: %s", forbidden, encoded)
		}
	}
}
