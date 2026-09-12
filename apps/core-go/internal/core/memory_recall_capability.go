package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	memoryRecallResultLimit = 12
	memoryRecallTokenBudget = 3072
)

func memoryRecallCapabilityDefinition() CapabilityDefinition {
	item := map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []any{"ref", "source_kind", "content"},
		"properties": map[string]any{
			"ref":         map[string]any{"type": "string", "minLength": 1, "maxLength": maxContextReferenceRunes},
			"source_kind": map[string]any{"type": "string", "enum": []any{"active_memory", "long_term_memory", "conversation_record", "summary_projection"}},
			"kind":        map[string]any{"type": "string"}, "content": map[string]any{"type": "string", "minLength": 1, "maxLength": 12000},
			"confidence":  map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
			"importance":  map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
			"occurred_at": map[string]any{"type": "string"}, "time_expression": map[string]any{"type": "string"},
		},
	}
	return CapabilityDefinition{
		Name: "memory.recall", Version: "v1", Type: CapabilityTypeQuery,
		Description: "Search deeper authorized Active, long-term, conversation, and summary memory only when the answer depends on information not already present in context.",
		Surfaces:    []CapabilitySurface{CapabilitySurfaceConversation}, FailurePolicy: FailurePolicyOptionalInternal,
		RequiredContext: []ContextSlot{SlotMemoryScope},
		InputSchema:     map[string]any{"type": "object", "additionalProperties": false, "required": []any{"intent"}, "properties": map[string]any{"intent": map[string]any{"type": "string", "minLength": 1, "maxLength": 1000}}},
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false, "required": []any{"items", "count", "truncated"},
			"properties": map[string]any{
				"items":     map[string]any{"type": "array", "maxItems": memoryRecallResultLimit, "items": item},
				"count":     map[string]any{"type": "integer", "minimum": 0, "maximum": memoryRecallResultLimit},
				"truncated": map[string]any{"type": "boolean"},
			},
		},
		SideEffectClass: "read_only", SuccessBoundary: "query_result_available", ConcurrencyClass: "parallel", SupportsRetry: true,
	}
}

type MemoryRecallRequest struct {
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	ViewerActorIDs       []string
	ConversationMode     MemoryConversationScopeMode
	ActiveProfileID      string
	Intent               string
}

type MemoryRecallService interface {
	Recall(context.Context, MemoryRecallRequest) ([]map[string]any, bool, error)
}

type memoryRecallService struct {
	retrieveMemory    func(context.Context, string, string, MemoryQueryPlan) (MemoryRetrievalResult, error)
	retrieveActive    func(context.Context, ActiveMemoryQuery) (ActiveMemoryRetrievalResult, error)
	searchRaw         func(context.Context, RawHistorySearchQuery) ([]RawHistoryEvent, error)
	retrieveSummaries func(context.Context, ConversationSummaryQuery) (ConversationSummaryRetrievalResult, error)
}

func newMemoryRecallService(app *App) MemoryRecallService {
	if app == nil || app.DB == nil {
		return nil
	}
	raw := NewRawHistoryReader(app.DB)
	return &memoryRecallService{
		retrieveMemory: app.retrieveMemoryWithPlan, retrieveActive: app.retrieveActiveMemories,
		searchRaw: raw.Search, retrieveSummaries: app.retrieveConversationSummaries,
	}
}

func (service *memoryRecallService) Recall(ctx context.Context, request MemoryRecallRequest) ([]map[string]any, bool, error) {
	request.Intent = strings.TrimSpace(request.Intent)
	if service == nil || service.retrieveMemory == nil || service.retrieveActive == nil || service.searchRaw == nil || service.retrieveSummaries == nil || request.AuthorizationActorID == "" || request.FluctlightID == "" || request.Intent == "" {
		return nil, false, errors.New("memory_recall_request_invalid")
	}
	plan, err := buildMemoryQueryPlan(MemoryForCapabilityPlanner, request.ViewerActorIDs, request.ConversationMode, request.ConversationID, nil, request.ActiveProfileID, []MemoryQueryCue{{Kind: "recall_intent", Text: request.Intent}}, 50, maxMemoryRetrievalBudget)
	if err != nil {
		return nil, false, err
	}
	longTerm, err := service.retrieveMemory(ctx, request.AuthorizationActorID, request.FluctlightID, plan)
	if err != nil {
		return nil, false, err
	}
	active, err := service.retrieveActive(ctx, ActiveMemoryQuery{AuthorizationActorID: request.AuthorizationActorID, OwnerFluctlightID: request.FluctlightID, ConversationID: request.ConversationID, Cue: request.Intent, At: time.Now().UTC(), Limit: activeMemoryResultLimit})
	if err != nil {
		return nil, false, err
	}
	var raw []RawHistoryEvent
	var summaries ConversationSummaryRetrievalResult
	if request.ConversationID != "" {
		raw, err = service.searchRaw(ctx, RawHistorySearchQuery{AuthorizationActorID: request.AuthorizationActorID, FluctlightID: request.FluctlightID, ConversationID: request.ConversationID, Query: request.Intent, Limit: 50})
		if err != nil {
			return nil, false, err
		}
		summaries, err = service.retrieveSummaries(ctx, ConversationSummaryQuery{AuthorizationActorID: request.AuthorizationActorID, FluctlightID: request.FluctlightID, ConversationID: request.ConversationID, Limit: conversationSummaryMaxResults, MaxRunes: conversationSummaryDefaultBudget})
		if err != nil {
			return nil, false, err
		}
	}
	type candidate struct {
		item  map[string]any
		score float64
	}
	candidates := make([]candidate, 0, len(active.Items)+len(longTerm.Items)+len(raw)+len(summaries.Items))
	for _, item := range active.Items {
		score := numberOrZero(item["importance"])*4 + numberOrZero(item["confidence"])
		candidates = append(candidates, candidate{item: recallActiveItem(item, request), score: score + 4})
	}
	for index, item := range longTerm.Items {
		score := numberOrZero(item["importance"])*3 + numberOrZero(item["confidence"])
		candidates = append(candidates, candidate{item: recallLongTermItem(item, request), score: score + 3 - float64(index)/100})
	}
	for _, event := range raw {
		if item := recallRawItem(event, request); item != nil {
			candidates = append(candidates, candidate{item: item, score: event.Relevance + 2})
		}
	}
	for index, item := range summaries.Items {
		candidates = append(candidates, candidate{item: recallSummaryItem(item), score: 1 - float64(index)/100})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return stringValue(candidates[i].item["ref"]) < stringValue(candidates[j].item["ref"])
	})
	items := make([]map[string]any, 0, memoryRecallResultLimit)
	used := 0
	truncated := false
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		ref := stringValue(candidate.item["ref"])
		if _, duplicate := seen[ref]; duplicate {
			continue
		}
		cost := EstimatePromptTokens(candidate.item)
		if len(items) >= memoryRecallResultLimit || used+cost > memoryRecallTokenBudget {
			truncated = true
			continue
		}
		seen[ref] = struct{}{}
		used += cost
		items = append(items, candidate.item)
	}
	return items, truncated, nil
}

func recallOpaqueRef(kind, identity string, request MemoryRecallRequest) string {
	return kind + ":ctx_" + stableDigest(strings.Join([]string{kind, identity, request.FluctlightID, request.ConversationID}, "\x1f"))
}

func recallActiveItem(item map[string]any, request MemoryRecallRequest) map[string]any {
	result := map[string]any{"ref": recallOpaqueRef("active_memory", stringValue(item["id"])+":"+fmt.Sprint(item["revision"]), request), "source_kind": "active_memory", "kind": item["kind"], "content": item["content"], "confidence": item["confidence"], "importance": item["importance"]}
	if value := stringValue(item["original_time_expression"]); value != "" {
		result["time_expression"] = value
	}
	return compactRecallItem(result)
}

func recallLongTermItem(item map[string]any, request MemoryRecallRequest) map[string]any {
	return compactRecallItem(map[string]any{"ref": recallOpaqueRef("memory", stringValue(item["id"])+":"+fmt.Sprint(item["revision"]), request), "source_kind": "long_term_memory", "kind": item["type"], "content": item["content"], "confidence": item["confidence"], "importance": item["importance"], "occurred_at": item["created_at"]})
}

func recallRawItem(event RawHistoryEvent, request MemoryRecallRequest) map[string]any {
	content := strings.TrimSpace(stringValue(event.Content["text"]))
	if content == "" {
		return nil
	}
	return compactRecallItem(map[string]any{"ref": recallOpaqueRef("conversation_record", event.SourceRef, request), "source_kind": "conversation_record", "kind": string(event.Kind), "content": content, "occurred_at": event.OccurredAt.UTC().Format(time.RFC3339Nano)})
}

func recallSummaryItem(item map[string]any) map[string]any {
	return compactRecallItem(map[string]any{"ref": item["ref"], "source_kind": "summary_projection", "kind": "conversation_summary", "content": item["summary"], "occurred_at": item["completed_at"]})
}

func compactRecallItem(item map[string]any) map[string]any {
	result := make(map[string]any, len(item))
	for _, key := range []string{"ref", "source_kind", "kind", "content", "confidence", "importance", "occurred_at", "time_expression"} {
		if value, ok := item[key]; ok && value != nil && value != "" {
			result[key] = value
		}
	}
	return result
}
