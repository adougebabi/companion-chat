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
	"episodic": {}, "semantic": {}, "relationship": {}, "autobiographical": {},
}

var validMemoryVisibility = map[string]struct{}{"private": {}, "owner": {}, "participants": {}}

func memoryCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "memory_event", Version: "v1", Type: CapabilityTypeAction,
		Description:     "Record an explicit evidence-backed Fluctlight memory candidate.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition},
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
		RequiredContext: []ContextSlot{SlotCorePersona, SlotMemoryScope},
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"content", "type", "confidence", "importance"},
			"properties": map[string]any{
				"content":                map[string]any{"type": "string", "minLength": 1, "maxLength": 32000},
				"type":                   map[string]any{"type": "string", "enum": []any{"episodic", "semantic", "relationship", "autobiographical"}},
				"confidence":             map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				"importance":             map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				"emotional_significance": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			},
		},
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"operation", "memory_id", "status", "revision", "disposition", "replayed"},
			"properties": map[string]any{
				"operation":   map[string]any{"type": "string", "enum": []any{"create"}},
				"memory_id":   map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
				"status":      map[string]any{"type": "string", "enum": []any{"active"}},
				"revision":    map[string]any{"type": "integer", "minimum": 0},
				"disposition": map[string]any{"type": "string", "enum": []any{"applied", "no_change"}},
				"replayed":    map[string]any{"type": "boolean"},
			},
		},
		SideEffectClass: "native_projection", SuccessBoundary: "memory_revision_committed", ConcurrencyClass: "exclusive", SupportsRetry: true,
		ProvenanceFields: []string{"evidence_refs", "idempotency_key"},
	}
}

func (a *App) applyMemoryCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
}

func (a *App) prepareMemoryCapability(_ context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if err := requireCapabilityContext(resolved, SlotCorePersona, SlotMemoryScope); err != nil {
		return invocation, err
	}
	args, err := capabilityExecutionArguments(invocation, memoryCapabilityDefinition())
	if err != nil {
		return invocation, err
	}
	semantic := MemorySemanticInput{
		Type: strings.TrimSpace(stringValue(args["type"])), Content: strings.TrimSpace(stringValue(args["content"])),
	}
	if _, ok := validMemoryTypes[semantic.Type]; !ok || semantic.Content == "" || len([]rune(semantic.Content)) > 32000 {
		return invocation, errors.New("memory_semantic_invalid")
	}
	if semantic.Confidence, err = requiredBoundedNumber(args["confidence"]); err != nil {
		return invocation, errors.New("memory_confidence_invalid")
	}
	if semantic.Importance, err = requiredBoundedNumber(args["importance"]); err != nil {
		return invocation, errors.New("memory_importance_invalid")
	}
	if raw, present := args["emotional_significance"]; present && raw != nil {
		if semantic.EmotionalSignificance, err = requiredBoundedNumber(raw); err != nil {
			return invocation, errors.New("memory_emotional_significance_invalid")
		}
	}
	ownerActorID := strings.TrimSpace(stringValue(resolved.Memory.Data["owner_actor_id"]))
	activeProfileID := strings.TrimSpace(stringValue(resolved.Memory.Data["active_profile_id"]))
	if ownerActorID == "" {
		return invocation, errors.New("memory_owner_scope_invalid")
	}
	evidence := sortedUniqueStrings(decisionServiceRefValues(args["evidence_refs"]))
	if len(evidence) != 1 || evidence[0] != invocation.SourceFactID {
		return invocation, errors.New("memory_runtime_evidence_invalid")
	}
	command := PreparedMemoryMutation{
		SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate,
		OwnerFluctlightID: invocation.Metadata.FluctlightID, OwnerActorID: ownerActorID, ActorID: invocation.Metadata.FluctlightID,
		ActiveProfileID: activeProfileID, ConversationID: invocation.Metadata.ConversationID,
		ActorRefs: []string{}, EventRefs: []string{}, EvidenceRefs: evidence, Semantic: &semantic,
		Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: invocation.SourceFactID,
		CandidateIndex: -1, SemanticReason: "explicit_memory_event",
		IdempotencyKey: "memory:create:" + invocation.Metadata.FluctlightID + ":" + invocation.CallID,
	}
	command.RequestDigest = memoryCommandDigest(command)
	if err := validatePreparedMemoryMutation(command); err != nil {
		return invocation, err
	}
	return withCapabilityPreparedData(invocation, "memory_plan", command)
}

func (a *App) applyMemoryCapabilityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if err := requireCapabilityContext(resolved, SlotCorePersona, SlotMemoryScope); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	rawPlan, found, err := capabilityPreparedData(invocation, "memory_plan")
	if err != nil || !found {
		if err == nil {
			err = errors.New("memory plan missing")
		}
		return failedCapabilityResultDetail(invocation, "memory_plan_invalid", false, err.Error()), err
	}
	var command PreparedMemoryMutation
	if err := jsonUnmarshal(jsonBytes(rawPlan), &command); err != nil || validatePreparedMemoryMutation(command) != nil {
		return failedCapabilityResultDetail(invocation, "memory_plan_invalid", false, "memory plan is malformed"), errors.New("memory plan invalid")
	}
	ownerActorID := strings.TrimSpace(stringValue(resolved.Memory.Data["owner_actor_id"]))
	activeProfileID := strings.TrimSpace(stringValue(resolved.Memory.Data["active_profile_id"]))
	if command.Operation != MemoryCreate || command.OwnerFluctlightID != invocation.Metadata.FluctlightID || command.OwnerActorID != ownerActorID || command.ActiveProfileID != activeProfileID || command.ConversationID != invocation.Metadata.ConversationID || command.SourceFactID != invocation.SourceFactID {
		return failedCapabilityResultDetail(invocation, "memory_plan_stale", false, "memory plan no longer matches the frozen invocation"), newCapabilityError("memory_plan_stale", false, ErrConflict)
	}
	result, err := a.applyMemoryCommandTx(ctx, tx, command)
	if err != nil {
		code, retryable := capabilityErrorInfo(err, "memory_apply_failed", true)
		return failedCapabilityResultDetail(invocation, code, retryable, err.Error()), err
	}
	output := map[string]any{
		"operation": string(result.Operation), "memory_id": result.MemoryID, "status": result.Status,
		"revision": result.Revision, "disposition": result.Disposition, "replayed": result.Replayed,
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "memory:" + result.MemoryID}, nil
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
	if len(record.PersonalityPerspectives) > 0 && !requirePerspectiveEvidence(record.PersonalityPerspectives) {
		return nil, errors.New("memory_perspective_evidence_required")
	}
	if len(record.PersonalityPerspectives) > 0 && !validatePersonalityPerspectiveProfiles(fluctlight.CorePersona, record.PersonalityPerspectives) {
		return nil, errors.New("memory_perspective_profile_invalid")
	}
	var result map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := validateMemoryActorScopeTx(ctx, tx, record, actorID); err != nil {
			return err
		}
		occurredAt := time.Now().UTC()
		if record.OccurredAt != nil {
			occurredAt = record.OccurredAt.UTC()
		}
		conversationID := ""
		if record.ConversationID != nil {
			conversationID = *record.ConversationID
		}
		semantic := MemorySemanticInput{Type: record.Type, Content: record.Content, Confidence: record.Confidence, Importance: record.Importance, EmotionalSignificance: record.EmotionalSignificance}
		command := PreparedMemoryMutation{
			SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate,
			OwnerFluctlightID: fluctlightID, OwnerActorID: actorID, ActorID: actorID,
			ConversationID: conversationID,
			ActorRefs:      decisionServiceRefValues(record.ActorRefs), EventRefs: decisionServiceRefValues(record.EventRefs),
			EvidenceRefs: decisionServiceRefValues(record.EvidenceRefs), PersonalityPerspectives: record.PersonalityPerspectives,
			Semantic: &semantic, Visibility: record.Visibility, OccurredAt: occurredAt,
			SourceFactID:   firstString(stringValue(record.EvidenceRefs[0]), "owner:"+actorID),
			CandidateIndex: -1, SemanticReason: "owner_memory_create",
			IdempotencyKey: "memory:create:" + fluctlightID + ":" + record.IdempotencyKey,
		}
		command.RequestDigest = memoryCommandDigest(command)
		applied, applyErr := a.applyMemoryCommandTx(ctx, tx, command)
		if applyErr != nil {
			return applyErr
		}
		result = map[string]any{"id": applied.MemoryID, "fluctlight_id": fluctlightID, "status": applied.Status, "revision": applied.Revision, "replayed": applied.Replayed, "disposition": applied.Disposition}
		return nil
	})
	return result, err
}

func validateMemoryActorScopeTx(ctx context.Context, tx pgx.Tx, record memoryRecordInput, ownerActorID string) error {
	for _, raw := range record.ActorRefs {
		actorID := strings.TrimSpace(stringValue(raw))
		if actorID == "actor_user" {
			actorID = ownerActorID
		}
		if actorID == "actor_self" {
			actorID = record.FluctlightID
		}
		if actorID == "" {
			return errors.New("memory_actor_ref_invalid")
		}
		var actorType, status string
		if err := tx.QueryRow(ctx, `SELECT actor_type,status FROM public.actors WHERE id=$1`, actorID).Scan(&actorType, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("memory_actor_ref_not_found")
			}
			return err
		}
		if status != "active" || (actorType != "human" && actorType != "fluctlight") {
			return errors.New("memory_actor_ref_invalid")
		}
		if actorType == "human" && actorID != ownerActorID {
			return errors.New("memory_actor_ref_forbidden")
		}
		if actorType == "fluctlight" {
			var createdBy string
			if err := tx.QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, actorID).Scan(&createdBy); err != nil || createdBy != ownerActorID {
				return errors.New("memory_actor_ref_forbidden")
			}
		}
		if record.ConversationID != nil {
			var participant bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id=$2 AND status='active')`, *record.ConversationID, actorID).Scan(&participant); err != nil {
				return err
			}
			if !participant {
				return errors.New("memory_actor_ref_conversation_forbidden")
			}
		}
	}
	return nil
}

func (a *App) RollbackMemory(ctx context.Context, actorID, memoryID string, targetRevision, expectedRevision int, evidenceRefs []any) (map[string]any, error) {
	if targetRevision < 0 || expectedRevision < 0 || targetRevision >= expectedRevision || len(evidenceRefs) == 0 {
		return nil, errors.New("memory_rollback_invalid")
	}
	refs, err := ownerMemoryEvidence(evidenceRefs)
	if err != nil {
		return nil, err
	}
	row, _, err := a.readOwnedMemoryRevisionSnapshot(ctx, actorID, memoryID, expectedRevision)
	if err != nil {
		return nil, err
	}
	_, snapshot, err := a.readOwnedMemoryRevisionSnapshot(ctx, actorID, memoryID, targetRevision)
	if err != nil {
		return nil, err
	}
	if len(snapshot) == 0 || stringValue(snapshot["id"]) != memoryID || stringValue(snapshot["status"]) != "active" {
		return nil, errors.New("memory_rollback_snapshot_invalid")
	}
	confidence, confidenceErr := requiredBoundedNumber(snapshot["confidence"])
	importance, importanceErr := requiredBoundedNumber(snapshot["importance"])
	emotional, emotionalErr := requiredBoundedNumber(snapshot["emotional_significance"])
	semantic := &MemorySemanticInput{Type: stringValue(snapshot["type"]), Content: stringValue(snapshot["content"]), Confidence: confidence, Importance: importance, EmotionalSignificance: emotional}
	if confidenceErr != nil || importanceErr != nil || emotionalErr != nil {
		return nil, errors.New("memory_rollback_snapshot_invalid")
	}
	command := buildOwnerMemoryCommand(row, actorID, MemoryRollback, expectedRevision, refs, semantic, "owner_memory_rollback", &targetRevision, snapshot)
	if replayed, found, err := a.readMemoryGovernanceReplay(ctx, command.IdempotencyKey); err != nil {
		return nil, err
	} else if found {
		value := memoryApplyResultMap(replayed)
		value["target_revision"] = targetRevision
		return value, nil
	}
	compensations, err := a.buildOwnerRollbackCompensations(ctx, actorID, memoryID, targetRevision, expectedRevision)
	if err != nil {
		return nil, err
	}
	command.RollbackCompensations = compensations
	command.RequestDigest = memoryCommandDigest(command)
	result, err := a.applyOwnerMemoryCommand(ctx, command)
	if err != nil {
		return nil, err
	}
	value := memoryApplyResultMap(result)
	value["target_revision"] = targetRevision
	return value, nil
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

func jsonUnmarshal(data []byte, target any) error {
	return json.Unmarshal(data, target)
}
