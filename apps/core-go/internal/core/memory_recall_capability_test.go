package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeMemoryRecallService struct {
	request MemoryRecallRequest
	items   []map[string]any
}

func (service *fakeMemoryRecallService) Recall(_ context.Context, request MemoryRecallRequest) ([]map[string]any, bool, error) {
	service.request = request
	return service.items, false, nil
}

func TestMemoryRecallDefinitionIsConversationOnlyPureQuery(t *testing.T) {
	definition := memoryRecallCapabilityDefinition()
	if err := definition.Validate(); err != nil {
		t.Fatal(err)
	}
	if definition.Name != "memory.recall" || definition.Type != CapabilityTypeQuery || definition.SideEffectClass != "read_only" || definition.FailurePolicy != FailurePolicyOptionalInternal || len(definition.Surfaces) != 1 || definition.Surfaces[0] != CapabilitySurfaceConversation || len(definition.RequiredContext) != 1 || definition.RequiredContext[0] != SlotMemoryScope {
		t.Fatalf("definition = %#v", definition)
	}
	if class, err := classifyCapabilityExecution(memoryRecallCapability{}, definition); err != nil || class != CapabilityExecutionPureQuery {
		t.Fatalf("execution class=%q err=%v", class, err)
	}
	properties := mapValue(definition.InputSchema["properties"])
	if len(properties) != 1 || properties["intent"] == nil {
		t.Fatalf("recall input is not intent-only: %#v", properties)
	}
}

func TestMemoryRecallCapabilityUsesFrozenAuthorizationScope(t *testing.T) {
	service := &fakeMemoryRecallService{items: []map[string]any{{"ref": "memory:ctx_0123456789abcdef0123456789abcdef", "source_kind": "long_term_memory", "content": "旧事实"}}}
	capability := memoryRecallCapability{service: service}
	invocation := CapabilityInvocation{CallID: "recall-call", CapabilityName: "memory.recall", Arguments: jsonBytes(map[string]any{"intent": "旧事实"}), ProviderRequestID: "provider", Metadata: InvocationMetadata{FluctlightID: "fl-1", ConversationID: "conversation-1"}}
	resolved := CapabilityContext{Memory: &MemoryScope{Data: map[string]any{"owner_actor_id": "owner-1", "viewer_actor_ids": []any{"viewer-1"}, "conversation_mode": "exact", "active_profile_id": "default"}}}
	result, err := capability.Execute(context.Background(), invocation, resolved)
	if err != nil || result.Status != "completed" || intValue(mapValue(result.Output)["count"]) != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if service.request.AuthorizationActorID != "owner-1" || service.request.FluctlightID != "fl-1" || service.request.ConversationID != "conversation-1" || len(service.request.ViewerActorIDs) != 1 || service.request.ViewerActorIDs[0] != "viewer-1" {
		t.Fatalf("frozen request = %#v", service.request)
	}
}

func TestMemoryRecallServiceCombinesDeepAuthoritiesWithOpaqueBoundedOutput(t *testing.T) {
	var capturedPlan MemoryQueryPlan
	service := &memoryRecallService{
		retrieveMemory: func(_ context.Context, _, _ string, plan MemoryQueryPlan) (MemoryRetrievalResult, error) {
			capturedPlan = plan
			return MemoryRetrievalResult{Items: []map[string]any{{"id": "memory_internal", "revision": 7, "type": "semantic", "content": "长期事实", "confidence": 0.9, "importance": 0.8, "visibility": "private", "evidence_refs": []any{"fact_internal"}, "created_at": "2026-01-01T00:00:00Z"}}}, nil
		},
		retrieveActive: func(context.Context, ActiveMemoryQuery) (ActiveMemoryRetrievalResult, error) {
			return ActiveMemoryRetrievalResult{Items: []map[string]any{{"id": "active_internal", "revision": 2, "kind": "future_event", "content": "明早航班", "confidence": 0.9, "importance": 1.0, "original_time_expression": "明早"}}}, nil
		},
		searchRaw: func(context.Context, RawHistorySearchQuery) ([]RawHistoryEvent, error) {
			return []RawHistoryEvent{{SourceRef: "message:internal", Kind: RawHistoryConversationMessage, Content: map[string]any{"text": "更早的原始对话"}, OccurredAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Relevance: 1}}, nil
		},
		retrieveSummaries: func(context.Context, ConversationSummaryQuery) (ConversationSummaryRetrievalResult, error) {
			return ConversationSummaryRetrievalResult{Items: []map[string]any{{"ref": "summary:ctx_0123456789abcdef0123456789abcdef", "summary": "历史摘要", "completed_at": "2026-01-02T00:00:00Z"}}}, nil
		},
	}
	items, _, err := service.Recall(context.Background(), MemoryRecallRequest{AuthorizationActorID: "owner", FluctlightID: "fl", ConversationID: "conversation", ViewerActorIDs: []string{"viewer"}, ConversationMode: MemoryConversationExact, ActiveProfileID: "default", Intent: "航班旧事实"})
	if err != nil {
		t.Fatal(err)
	}
	if capturedPlan.ResultLimit != 50 || capturedPlan.Budget != maxMemoryRetrievalBudget || len(items) != 4 {
		t.Fatalf("deep recall plan=%#v items=%#v", capturedPlan, items)
	}
	encoded := jsonString(items)
	for _, forbidden := range []string{"memory_internal", "active_internal", "message:internal", "visibility", "revision", "evidence_refs", "fact_internal", "plan_id", "score"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("recall output leaked %q: %s", forbidden, encoded)
		}
	}
	for _, sourceKind := range []string{"active_memory", "long_term_memory", "conversation_record", "summary_projection"} {
		if !strings.Contains(encoded, sourceKind) {
			t.Fatalf("recall output missing %q: %s", sourceKind, encoded)
		}
	}
}

func TestMemoryRecallCatalogSchemaStats(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	allBytes, allChars := CapabilityToolSchemaStats(registry.Definitions())
	conversationBytes, conversationChars := CapabilityToolSchemaStats(registry.Catalog(CapabilitySurfaceConversation))
	nativeBytes, nativeChars := CapabilityToolSchemaStats(registry.Catalog(CapabilitySurfaceNativeCognition))
	t.Logf("all=%d/%d conversation=%d/%d native=%d/%d", allBytes, allChars, conversationBytes, conversationChars, nativeBytes, nativeChars)
}
