package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	activeMemoryCandidateLimit = 256
	activeMemoryResultLimit    = 30
)

// ActiveMemoryCapabilitySource is the Core-owned source contract frozen into
// an Active Memory command. A cognition fact is one valid source, but a direct
// business operation uses its explicit evidence ID and admission time instead
// of manufacturing a cognition inbox row or an action snapshot.
type ActiveMemoryCapabilitySource struct {
	Kind         string    `json:"kind"`
	EvidenceID   string    `json:"evidence_id"`
	OccurredAt   time.Time `json:"occurred_at"`
	ActorID      string    `json:"actor_id"`
	ActorRefs    []string  `json:"actor_refs"`
	OperationID  string    `json:"operation_id"`
	Conversation string    `json:"conversation_id,omitempty"`
}

func activeMemoryEventCapabilityDefinition() CapabilityDefinition {
	properties := map[string]any{
		"operation":                map[string]any{"type": "string", "enum": []any{"create", "confirm", "revise", "complete", "supersede"}},
		"target_ref":               map[string]any{"type": "string", "minLength": 1, "maxLength": maxContextReferenceRunes},
		"kind":                     map[string]any{"type": "string", "enum": []any{"future_event", "commitment", "temporary_context"}},
		"content":                  map[string]any{"type": "string", "minLength": 1, "maxLength": 4000},
		"confidence":               map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
		"importance":               map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
		"original_time_expression": map[string]any{"type": "string", "maxLength": 512, "description": "Copy the source time phrase exactly; use an empty string only when the source has no time expression."},
		"valid_from":               map[string]any{"type": "string", "minLength": 1, "maxLength": 64, "description": "Optional relevance-start boundary. For a future event, omit it or use the current/source time so the event remains visible before it starts."},
		"valid_until":              map[string]any{"type": "string", "minLength": 1, "maxLength": 64, "description": "Expiry boundary after which the fact no longer affects behavior; required for future events."},
		"time_precision":           map[string]any{"type": "string", "enum": []any{"exact", "part_of_day", "date", "range", "unknown"}},
	}
	semanticRequired := []any{"operation", "kind", "content", "confidence", "original_time_expression", "time_precision"}
	futureEventRequired := append(append([]any{}, semanticRequired...), "valid_until")
	return CapabilityDefinition{
		Name: "active_memory_event", Version: "v1", Type: CapabilityTypeAction,
		Description:     "Record or close a short-lived fact that remains behaviorally relevant. For future_event create, preserve the exact source time phrase, set valid_until after the event, and omit valid_from or use current/source time so the fact is visible beforehand. Use opaque target_ref for non-create operations; never invent database identifiers or revisions.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition},
		FailurePolicy:   FailurePolicyOptionalInternal,
		RequiredContext: []ContextSlot{SlotMemoryScope, SlotCurrentLife},
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"operation"}, "properties": properties,
			"oneOf": []any{
				map[string]any{"required": futureEventRequired, "properties": map[string]any{"operation": map[string]any{"enum": []any{"create"}}, "kind": map[string]any{"enum": []any{"future_event"}}, "original_time_expression": map[string]any{"type": "string", "minLength": 1, "maxLength": 512}, "time_precision": map[string]any{"enum": []any{"exact", "part_of_day", "date", "range"}}}},
				map[string]any{"required": semanticRequired, "properties": map[string]any{"operation": map[string]any{"enum": []any{"create"}}, "kind": map[string]any{"enum": []any{"commitment", "temporary_context"}}}},
				map[string]any{"required": []any{"operation", "target_ref"}, "properties": map[string]any{"operation": map[string]any{"enum": []any{"confirm"}}}},
				map[string]any{"required": append(append([]any{}, semanticRequired...), "target_ref"), "properties": map[string]any{"operation": map[string]any{"enum": []any{"revise"}}}},
				map[string]any{"required": []any{"operation", "target_ref"}, "properties": map[string]any{"operation": map[string]any{"enum": []any{"complete"}}}},
				map[string]any{"required": append(append([]any{}, semanticRequired...), "target_ref"), "properties": map[string]any{"operation": map[string]any{"enum": []any{"supersede"}}}},
			},
		},
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"operation", "status", "disposition", "reason_code", "recorded", "replayed"},
			"properties": map[string]any{
				"operation": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"},
				"disposition": map[string]any{"type": "string"}, "reason_code": map[string]any{"type": "string"},
				"recorded": map[string]any{"type": "boolean"}, "replayed": map[string]any{"type": "boolean"},
			},
		},
		SideEffectClass: "native_projection", SuccessBoundary: "active_memory_revision_committed",
		ConcurrencyClass: "transactional", SupportsRetry: true,
	}
}

func (a *App) prepareActiveMemoryCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if err := requireCapabilityContext(resolved, SlotMemoryScope, SlotCurrentLife); err != nil {
		return invocation, err
	}
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return invocation, errors.New("active_memory_database_unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, activeMemoryEventCapabilityDefinition())
	if err != nil {
		return invocation, err
	}
	operation := ActiveMemoryOperation(strings.TrimSpace(stringValue(args["operation"])))
	if operation == ActiveMemoryExpire {
		return invocation, errors.New("active_memory_expire_provider_forbidden")
	}
	ownerActorID := strings.TrimSpace(stringValue(resolved.Memory.Data["owner_actor_id"]))
	timezone := strings.TrimSpace(stringValue(resolved.Life.Data["timezone"]))
	if ownerActorID == "" || timezone == "" {
		return invocation, errors.New("active_memory_scope_invalid")
	}
	idempotencyKey := "active-memory:direct:" + stableDigest(invocation.Metadata.FluctlightID+"\x1f"+capabilityOperationID(invocation))
	index, hasIndex, err := activeMemoryReferenceIndexFromInvocation(invocation, ownerActorID)
	if err != nil {
		return invocation, err
	}
	source, err := a.resolveActiveMemoryCapabilitySource(ctx, invocation, ownerActorID, idempotencyKey, index, hasIndex)
	if err != nil {
		return invocation, err
	}
	command := PreparedActiveMemoryMutation{
		SchemaVersion: activeMemoryLifecycleSchemaVersion, Operation: operation,
		OwnerFluctlightID: invocation.Metadata.FluctlightID, OwnerActorID: ownerActorID, ActorID: source.ActorID,
		ActorRefs:      append([]string(nil), source.ActorRefs...),
		ConversationID: invocation.Metadata.ConversationID, SourceFactID: invocation.SourceFactID,
		EvidenceRefs: []string{source.EvidenceID}, OccurredAt: source.OccurredAt,
		SemanticReason: "direct_active_memory_" + string(operation),
		IdempotencyKey: idempotencyKey,
	}
	var targetSnapshot map[string]any
	if operation != ActiveMemoryCreate {
		target, snapshot, targetErr := a.resolveActiveMemoryCapabilityTarget(ctx, stringValue(args["target_ref"]), invocation, resolved, source, index, hasIndex)
		if targetErr != nil {
			return invocation, targetErr
		}
		if targetConversationID := strings.TrimSpace(stringValue(snapshot["conversation_id"])); targetConversationID != "" && targetConversationID != command.ConversationID {
			return invocation, errors.New("active_memory_target_conversation_invalid")
		}
		command.Target = &target
		command.ProviderTargetRef = target.Ref
		targetSnapshot = snapshot
	}
	if operation == ActiveMemoryCreate || operation == ActiveMemoryRevise || operation == ActiveMemorySupersede {
		fallbackImportance := 0.5
		if targetSnapshot != nil {
			if value, ok := numberFloat(targetSnapshot["importance"]); ok {
				fallbackImportance = value
			}
		}
		semantic, semanticErr := activeMemorySemanticFromArguments(args, timezone, source.OccurredAt, fallbackImportance)
		if semanticErr != nil {
			return invocation, semanticErr
		}
		command.Semantic = &semantic
	}
	if operation == ActiveMemoryComplete {
		command.CloseReason = "provider_completed"
	}
	command.RequestDigest = activeMemoryCommandDigest(command)
	if err := validatePreparedActiveMemoryMutation(command); err != nil {
		return invocation, err
	}
	return withCapabilityPreparedData(invocation, "active_memory_plan", command)
}

func activeMemoryReferenceIndexFromInvocation(invocation CapabilityInvocation, ownerActorID string) (ContextReferenceIndex, bool, error) {
	raw, ok := invocation.ContextSnapshot["context_reference_index"]
	if !ok || raw == nil {
		return ContextReferenceIndex{}, false, nil
	}
	index, err := contextReferenceIndexFromValue(raw)
	if err != nil {
		return ContextReferenceIndex{}, false, err
	}
	if index.FluctlightID != invocation.Metadata.FluctlightID || index.OwnerActorID != ownerActorID || index.ConversationID != invocation.Metadata.ConversationID {
		return ContextReferenceIndex{}, false, errors.New("active_memory_reference_scope_invalid")
	}
	return index, true, nil
}

func (a *App) resolveActiveMemoryCapabilitySource(ctx context.Context, invocation CapabilityInvocation, ownerActorID, idempotencyKey string, index ContextReferenceIndex, hasIndex bool) (ActiveMemoryCapabilitySource, error) {
	evidenceID := strings.TrimSpace(invocation.SourceFactID)
	operationID := strings.TrimSpace(capabilityOperationID(invocation))
	if evidenceID == "" || operationID == "" || len([]rune(operationID)) > 256 {
		return ActiveMemoryCapabilitySource{}, errors.New("active_memory_source_invalid")
	}
	// A replay reuses the exact source time and actor authority already stored
	// with the command. This keeps a fresh native ToolCall ID from changing the
	// business command digest.
	var priorRaw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT command FROM public.active_memory_commands WHERE owner_fluctlight_id=$1 AND idempotency_key=$2`, invocation.Metadata.FluctlightID, idempotencyKey).Scan(&priorRaw); err == nil {
		var prior PreparedActiveMemoryMutation
		if jsonUnmarshal(priorRaw, &prior) != nil || validatePreparedActiveMemoryMutation(prior) != nil {
			return ActiveMemoryCapabilitySource{}, errors.New("active_memory_source_replay_invalid")
		}
		return ActiveMemoryCapabilitySource{
			Kind: "command_replay", EvidenceID: prior.SourceFactID, OccurredAt: prior.OccurredAt,
			ActorID: prior.ActorID, ActorRefs: append([]string(nil), prior.ActorRefs...), OperationID: operationID,
			Conversation: prior.ConversationID,
		}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ActiveMemoryCapabilitySource{}, err
	}
	source := ActiveMemoryCapabilitySource{
		Kind: "business_operation", EvidenceID: evidenceID, OccurredAt: a.now().UTC(),
		ActorID: ownerActorID, ActorRefs: []string{ownerActorID}, OperationID: operationID,
		Conversation: invocation.Metadata.ConversationID,
	}
	var cognitionOccurredAt time.Time
	if err := a.DB.Pool().QueryRow(ctx, `SELECT occurred_at FROM public.cognition_inbox WHERE id=$1 AND fluctlight_id=$2`, evidenceID, invocation.Metadata.FluctlightID).Scan(&cognitionOccurredAt); err == nil {
		source.Kind = "cognition_fact"
		source.OccurredAt = cognitionOccurredAt.UTC()
		source.ActorID = invocation.Metadata.FluctlightID
		if hasIndex {
			if speaker := strings.TrimSpace(index.SpeakerActorID); speaker != "" {
				source.ActorRefs = []string{speaker}
			}
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ActiveMemoryCapabilitySource{}, err
	}
	return source, nil
}

func (a *App) resolveActiveMemoryCapabilityTarget(ctx context.Context, ref string, invocation CapabilityInvocation, resolved CapabilityContext, source ActiveMemoryCapabilitySource, index ContextReferenceIndex, hasIndex bool) (ActiveMemoryTarget, map[string]any, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ActiveMemoryTarget{}, nil, errors.New("active_memory_context_ref_unknown")
	}
	if hasIndex {
		if target, snapshot, err := activeMemoryTargetFromRef(ref, index); err == nil {
			return target, snapshot, nil
		}
	}
	rows, err := a.DB.Pool().Query(ctx, `
		SELECT id,owner_fluctlight_id,COALESCE(conversation_id,''),kind,content,status,confidence,importance,
		       actor_refs,source_fact_id,evidence_refs,COALESCE(original_time_expression,''),valid_from,valid_until,
		       time_precision,timezone,last_relevant_at,revision,canonical_key,request_digest,
		       COALESCE(superseded_by_active_memory_id,''),COALESCE(supersedes_active_memory_id,''),created_at,updated_at,closed_at
		FROM public.active_memories
		WHERE owner_fluctlight_id=$1 AND status='active' AND (conversation_id IS NULL OR conversation_id=$2)
		ORDER BY last_relevant_at DESC,id LIMIT $3`, invocation.Metadata.FluctlightID, nullableString(strings.TrimSpace(invocation.Metadata.ConversationID)), activeMemoryCandidateLimit)
	if err != nil {
		return ActiveMemoryTarget{}, nil, err
	}
	defer rows.Close()
	liveIndex := ContextReferenceIndex{
		SchemaVersion: contextReferenceIndexVersion, FluctlightID: invocation.Metadata.FluctlightID,
		OwnerActorID: stringValue(resolved.Memory.Data["owner_actor_id"]), ConversationID: invocation.Metadata.ConversationID,
		ActiveProfileID: stringValue(resolved.Memory.Data["active_profile_id"]), ByRef: map[string]ContextReference{},
	}
	if len(source.ActorRefs) > 0 {
		liveIndex.SpeakerActorID = source.ActorRefs[0]
	}
	recallRequest := MemoryRecallRequest{FluctlightID: invocation.Metadata.FluctlightID, ConversationID: invocation.Metadata.ConversationID}
	for rows.Next() {
		row, scanErr := scanActiveMemoryAuthorityRow(rows)
		if scanErr != nil {
			return ActiveMemoryTarget{}, nil, scanErr
		}
		snapshot := activeMemoryRowMap(row)
		if recallOpaqueRef("active_memory", row.ID+":"+fmt.Sprint(row.Revision), recallRequest) == ref {
			return ActiveMemoryTarget{Ref: ref, ActiveMemoryID: row.ID, ExpectedRevision: row.Revision}, snapshot, nil
		}
		candidate := cloneMap(snapshot)
		if err := addActiveMemoryReferences(&liveIndex, []map[string]any{candidate}); err != nil {
			return ActiveMemoryTarget{}, nil, err
		}
		if stringValue(candidate["ref"]) == ref {
			return ActiveMemoryTarget{Ref: ref, ActiveMemoryID: row.ID, ExpectedRevision: row.Revision}, snapshot, nil
		}
	}
	if err := rows.Err(); err != nil {
		return ActiveMemoryTarget{}, nil, err
	}
	return ActiveMemoryTarget{}, nil, errors.New("active_memory_context_ref_unknown")
}

func activeMemorySemanticFromArguments(args map[string]any, timezone string, occurredAt time.Time, fallbackImportance float64) (ActiveMemorySemanticInput, error) {
	confidence, err := requiredBoundedNumber(args["confidence"])
	if err != nil {
		return ActiveMemorySemanticInput{}, errors.New("active_memory_confidence_invalid")
	}
	importance := fallbackImportance
	if raw, present := args["importance"]; present && raw != nil {
		importance, err = requiredBoundedNumber(raw)
		if err != nil {
			return ActiveMemorySemanticInput{}, errors.New("active_memory_importance_invalid")
		}
	}
	semantic := ActiveMemorySemanticInput{
		Kind: strings.TrimSpace(stringValue(args["kind"])), Content: strings.TrimSpace(stringValue(args["content"])),
		Confidence: confidence, Importance: importance,
		OriginalTimeExpression: strings.TrimSpace(stringValue(args["original_time_expression"])),
		TimePrecision:          strings.TrimSpace(stringValue(args["time_precision"])), Timezone: strings.TrimSpace(timezone),
	}
	if semantic.TimePrecision == "" {
		semantic.TimePrecision = "unknown"
	}
	if semantic.ValidFrom, err = parseActiveMemoryTime(args["valid_from"]); err != nil {
		return ActiveMemorySemanticInput{}, err
	}
	if semantic.ValidUntil, err = parseActiveMemoryTime(args["valid_until"]); err != nil {
		return ActiveMemorySemanticInput{}, err
	}
	if err := validateActiveMemorySemantic(semantic, occurredAt); err != nil {
		return ActiveMemorySemanticInput{}, err
	}
	return semantic, nil
}

func parseActiveMemoryTime(value any) (*time.Time, error) {
	raw := strings.TrimSpace(stringValue(value))
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, errors.New("active_memory_time_invalid")
	}
	return &parsed, nil
}

func activeMemoryTargetFromRef(ref string, index ContextReferenceIndex) (ActiveMemoryTarget, map[string]any, error) {
	ref = strings.TrimSpace(ref)
	entry, ok := index.ByRef[ref]
	if !ok || entry.Kind != ContextReferenceActiveMemory || entry.Ref != ref {
		return ActiveMemoryTarget{}, nil, errors.New("active_memory_context_ref_unknown")
	}
	snapshot := decodeObject(entry.Snapshot)
	if stringValue(snapshot["id"]) != entry.EntityID || intValue(snapshot["revision"]) != entry.Revision || stringValue(snapshot["status"]) != "active" {
		return ActiveMemoryTarget{}, nil, errors.New("active_memory_context_ref_snapshot_invalid")
	}
	return ActiveMemoryTarget{Ref: ref, ActiveMemoryID: entry.EntityID, ExpectedRevision: entry.Revision}, snapshot, nil
}

func (a *App) applyActiveMemoryCapability(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
}

func (a *App) applyActiveMemoryCapabilityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if err := requireCapabilityContext(resolved, SlotMemoryScope, SlotCurrentLife); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	rawPlan, found, err := capabilityPreparedData(invocation, "active_memory_plan")
	if err != nil || !found {
		return failedCapabilityResultDetail(invocation, "active_memory_plan_invalid", false, "active memory plan is missing"), errors.New("active_memory_plan_invalid")
	}
	var command PreparedActiveMemoryMutation
	if jsonUnmarshal(jsonBytes(rawPlan), &command) != nil || validatePreparedActiveMemoryMutation(command) != nil {
		return failedCapabilityResultDetail(invocation, "active_memory_plan_invalid", false, "active memory plan is malformed"), errors.New("active_memory_plan_invalid")
	}
	ownerActorID := strings.TrimSpace(stringValue(resolved.Memory.Data["owner_actor_id"]))
	if command.OwnerFluctlightID != invocation.Metadata.FluctlightID || command.OwnerActorID != ownerActorID || command.ConversationID != invocation.Metadata.ConversationID || command.SourceFactID != invocation.SourceFactID {
		return failedCapabilityResultDetail(invocation, "active_memory_plan_stale", false, "active memory plan no longer matches the frozen invocation"), newCapabilityError("active_memory_plan_stale", false, ErrConflict)
	}
	if command.Semantic != nil && command.Semantic.Timezone != strings.TrimSpace(stringValue(resolved.Life.Data["timezone"])) {
		return failedCapabilityResultDetail(invocation, "active_memory_plan_stale", false, "active memory timezone no longer matches the frozen invocation"), newCapabilityError("active_memory_plan_stale", false, ErrConflict)
	}
	result, err := a.applyActiveMemoryCommandTx(ctx, tx, command)
	if err != nil {
		code, retryable := capabilityErrorInfo(err, "active_memory_apply_failed", true)
		return failedCapabilityResultDetail(invocation, code, retryable, err.Error()), err
	}
	output := map[string]any{
		"operation": string(result.Operation), "status": result.Status, "disposition": result.Disposition,
		"reason_code": result.ReasonCode, "recorded": result.Disposition == "applied" || result.Disposition == "no_change", "replayed": result.Replayed,
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "active-memory:" + stableDigest(result.ActiveMemoryID)}, nil
}

type ActiveMemoryQuery struct {
	AuthorizationActorID string
	OwnerFluctlightID    string
	ConversationID       string
	Cue                  string
	At                   time.Time
	Limit                int
}

type ActiveMemoryRankTrace struct {
	ActiveMemoryID string             `json:"active_memory_id"`
	Components     map[string]float64 `json:"components"`
	Score          float64            `json:"score"`
	Disposition    string             `json:"disposition"`
	Reason         string             `json:"reason"`
}

type ActiveMemoryRetrievalTrace struct {
	PolicyVersion   string                  `json:"policy_version"`
	QueryDigest     string                  `json:"query_digest,omitempty"`
	CandidateCount  int                     `json:"candidate_count"`
	SelectedCount   int                     `json:"selected_count"`
	ResultLimit     int                     `json:"result_limit"`
	HardFilters     []string                `json:"hard_filters"`
	Ranking         []ActiveMemoryRankTrace `json:"ranking"`
	TruncatedReason string                  `json:"truncated_reason,omitempty"`
}

type ActiveMemoryRetrievalResult struct {
	Items []map[string]any           `json:"items"`
	Trace ActiveMemoryRetrievalTrace `json:"trace"`
}

func (a *App) retrieveActiveMemories(ctx context.Context, query ActiveMemoryQuery) (ActiveMemoryRetrievalResult, error) {
	if a == nil || a.DB == nil || strings.TrimSpace(query.AuthorizationActorID) == "" || strings.TrimSpace(query.OwnerFluctlightID) == "" {
		return ActiveMemoryRetrievalResult{}, errors.New("active_memory_query_identity_invalid")
	}
	if _, err := a.DB.GetFluctlight(ctx, query.OwnerFluctlightID, query.AuthorizationActorID); err != nil {
		return ActiveMemoryRetrievalResult{}, err
	}
	if query.At.IsZero() {
		query.At = time.Now().UTC()
	}
	query.At = query.At.UTC()
	if query.Limit < 1 {
		query.Limit = 1
	}
	if query.Limit > activeMemoryResultLimit {
		query.Limit = activeMemoryResultLimit
	}
	rows, err := a.DB.Pool().Query(ctx, `
		SELECT id,owner_fluctlight_id,COALESCE(conversation_id,''),kind,content,status,confidence,importance,
		       actor_refs,source_fact_id,evidence_refs,COALESCE(original_time_expression,''),valid_from,valid_until,
		       time_precision,timezone,last_relevant_at,revision,canonical_key,request_digest,
		       COALESCE(superseded_by_active_memory_id,''),COALESCE(supersedes_active_memory_id,''),created_at,updated_at,closed_at
		FROM public.active_memories
		WHERE owner_fluctlight_id=$1
		  AND status='active'
		  AND (conversation_id IS NULL OR conversation_id=$2)
		  AND (valid_from IS NULL OR valid_from <= $3)
		  AND (valid_until IS NULL OR valid_until > $3)
		ORDER BY last_relevant_at DESC,id
		LIMIT $4`, query.OwnerFluctlightID, nullableString(strings.TrimSpace(query.ConversationID)), query.At, activeMemoryCandidateLimit)
	if err != nil {
		return ActiveMemoryRetrievalResult{}, err
	}
	defer rows.Close()
	type scored struct {
		row        activeMemoryAuthorityRow
		components map[string]float64
		score      float64
	}
	candidates := make([]scored, 0)
	for rows.Next() {
		row, scanErr := scanActiveMemoryAuthorityRow(rows)
		if scanErr != nil {
			return ActiveMemoryRetrievalResult{}, scanErr
		}
		components := activeMemoryRankComponents(row, query.Cue, query.At)
		score := 0.0
		for _, value := range components {
			score += value
		}
		candidates = append(candidates, scored{row: row, components: components, score: score})
	}
	if err := rows.Err(); err != nil {
		return ActiveMemoryRetrievalResult{}, err
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if !candidates[i].row.LastRelevantAt.Equal(candidates[j].row.LastRelevantAt) {
			return candidates[i].row.LastRelevantAt.After(candidates[j].row.LastRelevantAt)
		}
		return candidates[i].row.ID < candidates[j].row.ID
	})
	trace := ActiveMemoryRetrievalTrace{
		PolicyVersion:  activeMemoryLifecyclePolicyVersion,
		CandidateCount: len(candidates), ResultLimit: query.Limit,
		HardFilters: []string{"owner", "conversation_or_global", "status_active", "valid_from_reached", "valid_until_future"},
		Ranking:     make([]ActiveMemoryRankTrace, 0, len(candidates)),
	}
	if cue := strings.TrimSpace(query.Cue); cue != "" {
		trace.QueryDigest = stableDigest(cue)
	}
	itemCapacity := query.Limit
	if len(candidates) < itemCapacity {
		itemCapacity = len(candidates)
	}
	items := make([]map[string]any, 0, itemCapacity)
	for position, candidate := range candidates {
		disposition, reason := "selected", "ranked"
		if position >= query.Limit {
			disposition, reason = "dropped", "result_limit"
			trace.TruncatedReason = "result_limit"
		} else {
			items = append(items, activeMemoryRowMap(candidate.row))
		}
		trace.Ranking = append(trace.Ranking, ActiveMemoryRankTrace{ActiveMemoryID: candidate.row.ID, Components: candidate.components, Score: candidate.score, Disposition: disposition, Reason: reason})
	}
	trace.SelectedCount = len(items)
	return ActiveMemoryRetrievalResult{Items: items, Trace: trace}, nil
}

func activeMemoryRankComponents(row activeMemoryAuthorityRow, cue string, at time.Time) map[string]float64 {
	components := map[string]float64{
		"importance": row.Importance * 4,
		"confidence": row.Confidence * 0.5,
	}
	ageHours := math.Max(0, at.Sub(row.LastRelevantAt).Hours())
	components["last_relevant_recency"] = 1 / (1 + ageHours/(7*24))
	lowerContent := strings.ToLower(row.Content)
	lowerCue := strings.ToLower(strings.TrimSpace(cue))
	if lowerCue != "" && strings.Contains(lowerContent, lowerCue) {
		components["lexical_relevance"] = 3
	} else {
		matches := 0
		for _, token := range tokenize(lowerCue) {
			if strings.Contains(lowerContent, token) {
				matches++
			}
		}
		components["lexical_relevance"] = math.Min(3, float64(matches))
	}
	var next *time.Time
	for _, candidate := range []*time.Time{row.ValidFrom, row.ValidUntil} {
		if candidate != nil && candidate.After(at) && (next == nil || candidate.Before(*next)) {
			next = candidate
		}
	}
	if next != nil {
		hours := next.Sub(at).Hours()
		components["temporal_urgency"] = 3 / (1 + hours/24)
		if hours <= 36 {
			components["critical_window"] = 4
		}
	}
	return components
}

func activeMemoryRowMap(row activeMemoryAuthorityRow) map[string]any {
	return map[string]any{
		"id": row.ID, "owner_fluctlight_id": row.OwnerFluctlightID, "conversation_id": row.ConversationID,
		"kind": row.Kind, "content": row.Content, "status": row.Status, "confidence": row.Confidence, "importance": row.Importance,
		"actor_refs": row.ActorRefs, "source_fact_id": row.SourceFactID, "evidence_refs": row.EvidenceRefs,
		"original_time_expression": row.OriginalTimeExpression, "valid_from": row.ValidFrom, "valid_until": row.ValidUntil,
		"time_precision": row.TimePrecision, "timezone": row.Timezone, "last_relevant_at": row.LastRelevantAt,
		"revision": row.Revision, "created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
	}
}

func compactActiveMemories(memories []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(memories))
	for _, memory := range memories {
		compact := make(map[string]any, 10)
		for _, key := range []string{"ref", "kind", "content", "status", "confidence", "importance", "original_time_expression", "valid_from", "valid_until", "time_precision", "timezone"} {
			if value, ok := memory[key]; ok && value != nil && value != "" {
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			result = append(result, compact)
		}
	}
	return result
}

// Compile-time protection for the database row scanner used by pgx.Rows.
var _ activeMemoryRowScanner = pgx.Rows(nil)
