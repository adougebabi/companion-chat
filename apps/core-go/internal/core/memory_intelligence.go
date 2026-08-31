package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var validMemoryTypes = map[string]struct{}{
	"working": {}, "episodic": {}, "semantic": {}, "relationship": {}, "autobiographical": {},
}

var validMemoryVisibility = map[string]struct{}{"private": {}, "owner": {}, "participants": {}}

func memoryCapabilityManifest() CapabilityManifest {
	return CapabilityManifest{
		Name: "memory_event", Version: "v1",
		Description: "Record an explicit evidence-backed Fluctlight memory candidate.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"operation", "content", "type", "confidence", "importance", "evidence_refs", "idempotency_key"},
			"properties": map[string]any{
				"operation":              map[string]any{"type": "string", "enum": []any{"record"}},
				"type":                   map[string]any{"type": "string", "enum": []any{"working", "episodic", "semantic", "relationship", "autobiographical"}},
				"content":                map[string]any{"type": "string", "minLength": 1, "maxLength": 32000},
				"confidence":             map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				"importance":             map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				"emotional_significance": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				"visibility":             map[string]any{"type": "string", "enum": []any{"private", "owner", "participants"}},
				"actor_refs":             map[string]any{"type": "array"}, "event_refs": map[string]any{"type": "array"},
				"evidence_refs":   map[string]any{"type": "array", "minItems": 1},
				"source_fact_id":  map[string]any{"type": "string", "minLength": 1},
				"idempotency_key": map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
			},
		},
		SideEffectClass: "native_projection", ConcurrencyClass: "exclusive", SupportsRetry: true,
	}
}

func (a *App) applyMemoryCapability(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	var args map[string]any
	if err := jsonUnmarshal(call.Arguments, &args); err != nil {
		return failedToolResult(call, "memory_arguments_invalid", false, err.Error()), err
	}
	if source := stringValue(args["source_fact_id"]); source != "" && source != sourceFactID {
		return failedToolResult(call, "memory_source_invalid", false, "source fact does not match current cognition fact"), errors.New("memory source fact invalid")
	}
	if firstString(args["operation"], "record") != "record" {
		return failedToolResult(call, "memory_operation_invalid", false, "only record is supported by this capability"), errors.New("memory operation invalid")
	}
	args["source_fact_id"] = sourceFactID
	args["conversation_id"] = conversationID
	if len(arrayValue(args["evidence_refs"])) == 0 {
		args["evidence_refs"] = []any{sourceFactID}
	}
	if !containsStringValue(arrayValue(args["evidence_refs"]), sourceFactID) {
		return failedToolResult(call, "memory_evidence_invalid", false, "evidence must include the current source fact"), errors.New("memory evidence invalid")
	}
	if stringValue(args["idempotency_key"]) == "" {
		args["idempotency_key"] = "tool:" + call.ID
	}
	var ownerActorID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerActorID); err != nil {
		return failedToolResult(call, "memory_owner_not_found", true, err.Error()), err
	}
	result, err := a.RecordMemory(ctx, fluctlightID, ownerActorID, args)
	if err != nil {
		return failedToolResult(call, "memory_record_failed", true, err.Error()), err
	}
	return ToolResultV1{ToolCallID: call.ID, Name: call.Name, Status: "completed", Output: result, ProviderRequestID: call.ProviderRequestID, CorrelationID: "memory:" + stringValue(result["id"]), SchemaVersion: ToolResultSchemaVersion}, nil
}

// RecordMemory is the only ordinary-chat authority path for a new long-lived
// memory. It creates the initial revision, embedding intent, and outbox event
// atomically so indexing failure cannot erase the fact.
func (a *App) RecordMemory(ctx context.Context, fluctlightID, actorID string, payload map[string]any) (map[string]any, error) {
	if fluctlightID == "" || actorID == "" {
		return nil, errors.New("memory_owner_required")
	}
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	record, err := normalizeMemoryRecord(fluctlightID, payload)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		result, err = recordMemoryTx(ctx, tx, record, actorID)
		return err
	})
	return result, err
}

func (a *App) RollbackMemory(ctx context.Context, actorID, memoryID string, targetRevision, expectedRevision int, evidenceRefs []any) (map[string]any, error) {
	if targetRevision < 0 || expectedRevision < 0 || len(evidenceRefs) == 0 {
		return nil, errors.New("memory_rollback_invalid")
	}
	var content string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT r.content FROM public.memory_revisions r JOIN public.memories m ON m.id=r.memory_id JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id WHERE r.memory_id=$1 AND r.revision=$2 AND f.created_by_actor_id=$3`, memoryID, targetRevision, actorID).Scan(&content); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	result, err := a.ReviseMemory(ctx, actorID, memoryID, content, &expectedRevision, append(evidenceRefs, "rollback:"+fmt.Sprint(targetRevision)))
	if err != nil {
		return nil, err
	}
	result["status"] = "rolled_back"
	result["target_revision"] = targetRevision
	return result, nil
}

type memoryRecordInput struct {
	ID                    string
	FluctlightID          string
	Type                  string
	Content               string
	ActorRefs             []any
	ConversationID        *string
	EventRefs             []any
	EvidenceRefs          []any
	Confidence            float64
	Importance            float64
	EmotionalSignificance float64
	Visibility            string
	IdempotencyKey        string
	OccurredAt            *time.Time
}

func normalizeMemoryRecord(fluctlightID string, payload map[string]any) (memoryRecordInput, error) {
	typeName := firstString(payload["type"], "")
	if _, ok := validMemoryTypes[typeName]; !ok {
		return memoryRecordInput{}, errors.New("memory_type_invalid")
	}
	content := strings.TrimSpace(stringValue(payload["content"]))
	if content == "" || len([]rune(content)) > 32000 {
		return memoryRecordInput{}, errors.New("memory_content_invalid")
	}
	evidence := arrayValue(payload["evidence_refs"])
	if len(evidence) == 0 {
		return memoryRecordInput{}, errors.New("memory_evidence_required")
	}
	confidence, err := requiredBoundedNumber(payload["confidence"])
	if err != nil {
		return memoryRecordInput{}, errors.New("memory_confidence_invalid")
	}
	importance, err := requiredBoundedNumber(payload["importance"])
	if err != nil {
		return memoryRecordInput{}, errors.New("memory_importance_invalid")
	}
	emotional, err := requiredBoundedNumber(payload["emotional_significance"])
	if err != nil {
		return memoryRecordInput{}, errors.New("memory_emotional_significance_invalid")
	}
	visibility := firstString(payload["visibility"], "private")
	if _, ok := validMemoryVisibility[visibility]; !ok {
		return memoryRecordInput{}, errors.New("memory_visibility_invalid")
	}
	idempotency := stringValue(payload["idempotency_key"])
	if idempotency == "" || len(idempotency) > 256 {
		return memoryRecordInput{}, errors.New("memory_idempotency_required")
	}
	var conversationID *string
	if value := stringValue(payload["conversation_id"]); value != "" {
		conversationID = &value
	}
	return memoryRecordInput{ID: "memory_" + stableDigest(fluctlightID+":"+idempotency), FluctlightID: fluctlightID, Type: typeName, Content: content, ActorRefs: arrayValue(payload["actor_refs"]), ConversationID: conversationID, EventRefs: arrayValue(payload["event_refs"]), EvidenceRefs: evidence, Confidence: confidence, Importance: importance, EmotionalSignificance: emotional, Visibility: visibility, IdempotencyKey: idempotency}, nil
}

func requiredBoundedNumber(value any) (float64, error) {
	if value == nil {
		return 0, errors.New("number_required")
	}
	parsed, ok := numberFloat(value)
	if !ok || parsed < 0 || parsed > 1 {
		return 0, errors.New("number_invalid")
	}
	return parsed, nil
}

func containsStringValue(values []any, expected string) bool {
	for _, value := range values {
		if stringValue(value) == expected {
			return true
		}
	}
	return false
}

func recordMemoryTx(ctx context.Context, tx pgx.Tx, record memoryRecordInput, actorID string) (map[string]any, error) {
	var existingOwner, existingType, existingContent string
	var existingRevision int
	err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,type,content,revision FROM public.memories WHERE id=$1`, record.ID).Scan(&existingOwner, &existingType, &existingContent, &existingRevision)
	if err == nil {
		if existingOwner != record.FluctlightID || existingType != record.Type || existingContent != record.Content {
			return nil, ErrConflict
		}
		return map[string]any{"id": record.ID, "fluctlight_id": record.FluctlightID, "status": "active", "revision": existingRevision, "replayed": true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,confidence,importance,emotional_significance,visibility,status,revision,search_document) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'active',0,to_tsvector('simple',$4))`, record.ID, record.FluctlightID, record.Type, record.Content, jsonBytes(record.ActorRefs), record.ConversationID, jsonBytes(record.EventRefs), jsonBytes(record.EvidenceRefs), record.Confidence, record.Importance, record.EmotionalSignificance, record.Visibility); err != nil {
		return nil, err
	}
	revisionID := "memory_revision_" + stableDigest(record.ID+":0")
	if _, err := tx.Exec(ctx, `INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,content,status,actor_id,evidence_refs,idempotency_key) VALUES($1,$2,0,0,$3,'active',$4,$5,$6) ON CONFLICT(idempotency_key) DO NOTHING`, revisionID, record.ID, record.Content, actorID, jsonBytes(record.EvidenceRefs), "memory-record:"+record.IdempotencyKey); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','memory.embedding',$3) ON CONFLICT DO NOTHING`, "memory_embedding_intent:"+record.ID+":0", "memory_embedding:"+record.ID+":0", jsonBytes(map[string]any{"memory_id": record.ID, "revision": 0})); err != nil {
		return nil, err
	}
	if err := appendOutboxTx(ctx, tx, "memory.created", "memory", record.ID, record.FluctlightID, record.ID, "memory:"+record.ID, "memory:"+record.IdempotencyKey, map[string]any{"memory_id": record.ID, "revision": 0, "aggregate_sequence": 1}); err != nil {
		return nil, err
	}
	if err := appendOutboxTx(ctx, tx, "memory.embedding.requested", "memory", record.ID, record.FluctlightID, record.ID, "memory:"+record.ID, "memory-embedding:"+record.ID+":0", map[string]any{"memory_id": record.ID, "revision": 0, "aggregate_sequence": 2}); err != nil {
		return nil, err
	}
	return map[string]any{"id": record.ID, "fluctlight_id": record.FluctlightID, "status": "active", "revision": 0, "replayed": false}, nil
}

func jsonUnmarshal(data []byte, target any) error {
	return json.Unmarshal(data, target)
}
