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
				"personality_perspectives": map[string]any{"type": "array", "maxItems": 16, "items": objectSchema(map[string]any{
					"profile_id": stringSchema(), "interpretation": stringSchema(), "emotion": stringSchema(),
					"evidence_refs": arraySchema(stringSchema()), "provenance": openObjectSchema(),
				}, []string{"profile_id", "interpretation"}, false)},
				"actor_refs": map[string]any{"type": "array"}, "event_refs": map[string]any{"type": "array"},
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
	refs := arrayValue(args["evidence_refs"])
	if !containsStringValue(refs, sourceFactID) {
		// The Runtime owns the current fact boundary. Provider-supplied message
		// refs remain useful evidence, but the authoritative source fact is
		// appended rather than making an otherwise valid memory call fail.
		refs = append(refs, sourceFactID)
	}
	args["evidence_refs"] = refs
	if perspectives, err := normalizePersonalityPerspectives(args["personality_perspectives"]); err != nil {
		return failedToolResult(call, "memory_perspective_invalid", false, err.Error()), err
	} else {
		args["personality_perspectives"] = bindPerspectiveEvidence(perspectives, sourceFactID)
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
	fluctlight, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID)
	if err != nil {
		return nil, err
	}
	record, err := normalizeMemoryRecord(fluctlightID, payload)
	if err != nil {
		return nil, err
	}
	if !requirePerspectiveEvidence(record.PersonalityPerspectives) {
		return nil, errors.New("memory_perspective_evidence_required")
	}
	if !validatePersonalityPerspectiveProfiles(fluctlight.CorePersona, record.PersonalityPerspectives) {
		return nil, errors.New("memory_perspective_profile_invalid")
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
	var perspectives []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT r.content,r.personality_perspectives FROM public.memory_revisions r JOIN public.memories m ON m.id=r.memory_id JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id WHERE r.memory_id=$1 AND r.revision=$2 AND f.created_by_actor_id=$3`, memoryID, targetRevision, actorID).Scan(&content, &perspectives); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	result, err := a.reviseMemory(ctx, actorID, memoryID, content, &expectedRevision, append(evidenceRefs, "rollback:"+fmt.Sprint(targetRevision)), perspectives)
	if err != nil {
		return nil, err
	}
	result["status"] = "rolled_back"
	result["target_revision"] = targetRevision
	return result, nil
}

type memoryRecordInput struct {
	ID                      string
	FluctlightID            string
	Type                    string
	Content                 string
	ActorRefs               []any
	ConversationID          *string
	EventRefs               []any
	EvidenceRefs            []any
	PersonalityPerspectives []any
	Confidence              float64
	Importance              float64
	EmotionalSignificance   float64
	Visibility              string
	IdempotencyKey          string
	OccurredAt              *time.Time
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
	emotional := 0.0
	if rawEmotional, present := payload["emotional_significance"]; present && rawEmotional != nil {
		emotional, err = requiredBoundedNumber(rawEmotional)
		if err != nil {
			return memoryRecordInput{}, errors.New("memory_emotional_significance_invalid")
		}
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
	perspectives, err := normalizePersonalityPerspectives(payload["personality_perspectives"])
	if err != nil {
		return memoryRecordInput{}, err
	}
	return memoryRecordInput{ID: "memory_" + stableDigest(fluctlightID+":"+idempotency), FluctlightID: fluctlightID, Type: typeName, Content: content, ActorRefs: arrayValue(payload["actor_refs"]), ConversationID: conversationID, EventRefs: arrayValue(payload["event_refs"]), EvidenceRefs: evidence, PersonalityPerspectives: bindPerspectiveEvidence(perspectives, firstString(stringValue(payload["source_fact_id"]), "")), Confidence: confidence, Importance: importance, EmotionalSignificance: emotional, Visibility: visibility, IdempotencyKey: idempotency}, nil
}

func normalizePersonalityPerspectives(value any) ([]any, error) {
	if value != nil {
		switch value.(type) {
		case []any, []map[string]any, []string:
		default:
			return nil, errors.New("memory_perspectives_invalid")
		}
	}
	raw := arrayValue(value)
	if len(raw) > 16 {
		return nil, errors.New("memory_perspectives_too_many")
	}
	result := make([]any, 0, len(raw))
	seen := map[string]struct{}{}
	for index, item := range raw {
		perspective := mapValue(item)
		if len(perspective) == 0 {
			return nil, fmt.Errorf("memory_perspective_%d_invalid", index)
		}
		profileID := strings.TrimSpace(stringValue(perspective["profile_id"]))
		interpretation := strings.TrimSpace(stringValue(perspective["interpretation"]))
		if profileID == "" || len([]rune(profileID)) > 128 || interpretation == "" || len([]rune(interpretation)) > 4000 {
			return nil, fmt.Errorf("memory_perspective_%d_invalid", index)
		}
		if _, exists := seen[profileID]; exists {
			return nil, errors.New("memory_perspective_duplicate_profile")
		}
		seen[profileID] = struct{}{}
		clean := map[string]any{"profile_id": profileID, "interpretation": interpretation}
		if emotion := strings.TrimSpace(stringValue(perspective["emotion"])); emotion != "" {
			if len([]rune(emotion)) > 512 {
				return nil, fmt.Errorf("memory_perspective_%d_invalid", index)
			}
			clean["emotion"] = emotion
		}
		refs := arrayValue(perspective["evidence_refs"])
		cleanRefs := make([]any, 0, len(refs))
		for _, ref := range refs {
			value := strings.TrimSpace(stringValue(ref))
			if value == "" || len([]rune(value)) > 256 {
				return nil, fmt.Errorf("memory_perspective_%d_evidence_invalid", index)
			}
			cleanRefs = append(cleanRefs, value)
		}
		if len(cleanRefs) > 0 {
			clean["evidence_refs"] = cleanRefs
		}
		if provenance := mapValue(perspective["provenance"]); len(provenance) > 0 {
			clean["provenance"] = cloneMap(provenance)
		}
		result = append(result, clean)
	}
	return result, nil
}

func bindPerspectiveEvidence(values []any, sourceFactID string) []any {
	if strings.TrimSpace(sourceFactID) == "" {
		return values
	}
	result := make([]any, 0, len(values))
	for _, raw := range values {
		item := cloneMap(mapValue(raw))
		refs := arrayValue(item["evidence_refs"])
		if !containsStringValue(refs, sourceFactID) {
			refs = append(refs, sourceFactID)
		}
		item["evidence_refs"] = refs
		result = append(result, item)
	}
	return result
}

func validatePerspectiveEvidence(values []any, allowed map[string]struct{}) bool {
	for _, raw := range values {
		for _, ref := range arrayValue(mapValue(raw)["evidence_refs"]) {
			if _, ok := allowed[strings.TrimSpace(stringValue(ref))]; !ok {
				return false
			}
		}
	}
	return true
}

func requirePerspectiveEvidence(values []any) bool {
	for _, raw := range values {
		if len(arrayValue(mapValue(raw)["evidence_refs"])) == 0 {
			return false
		}
	}
	return true
}

func validatePersonalityPerspectiveProfiles(corePersona map[string]any, values []any) bool {
	profiles := personalityProfileIDs(corePersona)
	for _, raw := range values {
		if _, ok := profiles[strings.TrimSpace(stringValue(mapValue(raw)["profile_id"]))]; !ok {
			return false
		}
	}
	return true
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
	var existingPerspectives []byte
	var existingRevision int
	err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,type,content,personality_perspectives,revision FROM public.memories WHERE id=$1`, record.ID).Scan(&existingOwner, &existingType, &existingContent, &existingPerspectives, &existingRevision)
	if err == nil {
		if existingOwner != record.FluctlightID || existingType != record.Type || existingContent != record.Content || jsonString(decodeArray(existingPerspectives)) != jsonString(record.PersonalityPerspectives) {
			return nil, ErrConflict
		}
		return map[string]any{"id": record.ID, "fluctlight_id": record.FluctlightID, "status": "active", "revision": existingRevision, "replayed": true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var generated string
	if err := tx.QueryRow(ctx, `SELECT is_generated FROM information_schema.columns WHERE table_schema='public' AND table_name='memories' AND column_name='search_document'`).Scan(&generated); err != nil {
		return nil, err
	}
	if generated == "ALWAYS" {
		if _, err := tx.Exec(ctx, `INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'active',0)`, record.ID, record.FluctlightID, record.Type, record.Content, jsonBytes(record.ActorRefs), record.ConversationID, jsonBytes(record.EventRefs), jsonBytes(record.EvidenceRefs), jsonBytes(record.PersonalityPerspectives), record.Confidence, record.Importance, record.EmotionalSignificance, record.Visibility); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.Exec(ctx, `INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,search_document) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'active',0,to_tsvector('simple',$4))`, record.ID, record.FluctlightID, record.Type, record.Content, jsonBytes(record.ActorRefs), record.ConversationID, jsonBytes(record.EventRefs), jsonBytes(record.EvidenceRefs), jsonBytes(record.PersonalityPerspectives), record.Confidence, record.Importance, record.EmotionalSignificance, record.Visibility); err != nil {
			return nil, err
		}
	}
	revisionID := "memory_revision_" + stableDigest(record.ID+":0")
	if _, err := tx.Exec(ctx, `INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,content,personality_perspectives,status,actor_id,evidence_refs,idempotency_key) VALUES($1,$2,0,0,$3,$4,'active',$5,$6,$7) ON CONFLICT(idempotency_key) DO NOTHING`, revisionID, record.ID, record.Content, jsonBytes(record.PersonalityPerspectives), actorID, jsonBytes(record.EvidenceRefs), "memory-record:"+record.IdempotencyKey); err != nil {
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
