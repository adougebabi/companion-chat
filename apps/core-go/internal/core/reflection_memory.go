package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var reflectionMemoryCandidateFields = map[string]struct{}{
	"operation": {}, "target_ref": {}, "merge_refs": {}, "type": {}, "content": {},
	"confidence": {}, "importance": {}, "emotional_significance": {}, "evidence_refs": {},
	"semantic_reason": {},
}

type reflectionMemoryEvidenceScope struct {
	FactID         string
	ConversationID string
	Known          bool
}

type reflectionMemoryCompileRequest struct {
	FluctlightID    string
	OwnerActorID    string
	ActiveProfileID string
	ProposalID      string
	SourceWindow    string
	OccurredAt      time.Time
	ReferenceIndex  ContextReferenceIndex
	AllowedEvidence map[string]struct{}
	EvidenceScopes  map[string]reflectionMemoryEvidenceScope
}

type reflectionActiveMemoryCompileRequest struct {
	FluctlightID    string
	OwnerActorID    string
	ProposalID      string
	SourceWindow    string
	Timezone        string
	OccurredAt      time.Time
	ReferenceIndex  ContextReferenceIndex
	AllowedEvidence map[string]struct{}
	EvidenceScopes  map[string]reflectionMemoryEvidenceScope
}

type reflectionAcceptedActiveMemoryCandidate struct {
	Index     int
	Candidate ReflectionActiveMemoryCandidateV1
}

func compileReflectionActiveMemoryCommands(candidates []reflectionAcceptedActiveMemoryCandidate, request reflectionActiveMemoryCompileRequest) ([]PreparedActiveMemoryMutation, error) {
	if strings.TrimSpace(request.FluctlightID) == "" || strings.TrimSpace(request.OwnerActorID) == "" || strings.TrimSpace(request.ProposalID) == "" || strings.TrimSpace(request.SourceWindow) == "" || strings.TrimSpace(request.Timezone) == "" || request.OccurredAt.IsZero() {
		return nil, errors.New("reflection_active_memory_compile_identity_invalid")
	}
	if err := request.ReferenceIndex.Validate(); err != nil {
		return nil, err
	}
	if request.ReferenceIndex.FluctlightID != request.FluctlightID || request.ReferenceIndex.OwnerActorID != request.OwnerActorID {
		return nil, errors.New("reflection_active_memory_reference_scope_invalid")
	}
	commands := make([]PreparedActiveMemoryMutation, 0, len(candidates))
	touchedTargets := make(map[string]struct{})
	createdKeys := make(map[string]struct{})
	for _, accepted := range candidates {
		index := accepted.Index
		candidate := accepted.Candidate
		operation := ActiveMemoryOperation(strings.TrimSpace(candidate.Operation))
		switch operation {
		case ActiveMemoryCreate, ActiveMemoryConfirm, ActiveMemoryRevise, ActiveMemoryComplete, ActiveMemoryExpire, ActiveMemorySupersede:
		default:
			return nil, fmt.Errorf("reflection_active_memory_candidate_%d_operation_invalid", index)
		}
		reason := strings.TrimSpace(candidate.SemanticReason)
		if reason == "" || len([]rune(reason)) > 1000 {
			return nil, fmt.Errorf("reflection_active_memory_candidate_%d_reason_invalid", index)
		}
		evidenceRefs := sortedUniqueStrings(candidate.EvidenceRefs)
		if len(evidenceRefs) == 0 || len(evidenceRefs) > 64 || !validateEvidenceRefs(stringSliceAny(evidenceRefs), request.AllowedEvidence) {
			return nil, fmt.Errorf("reflection_active_memory_candidate_%d_evidence_invalid", index)
		}
		for _, ref := range evidenceRefs {
			if len([]rune(ref)) > 256 {
				return nil, fmt.Errorf("reflection_active_memory_candidate_%d_evidence_invalid", index)
			}
		}
		command := PreparedActiveMemoryMutation{
			SchemaVersion: activeMemoryLifecycleSchemaVersion, Operation: operation,
			OwnerFluctlightID: request.FluctlightID, OwnerActorID: request.OwnerActorID, ActorID: request.FluctlightID,
			ActorRefs:    []string{},
			EvidenceRefs: evidenceRefs, OccurredAt: request.OccurredAt.UTC(), SourceFactID: reflectionActiveMemorySourceFactID(evidenceRefs, request.EvidenceScopes),
			SemanticReason: reason, IdempotencyKey: "active-memory:reflection:" + request.ProposalID + ":" + fmt.Sprint(index),
		}
		if command.SourceFactID == "" {
			return nil, fmt.Errorf("reflection_active_memory_candidate_%d_source_fact_invalid", index)
		}
		if operation == ActiveMemoryCreate {
			conversationID, err := reflectionMemoryConversationID(evidenceRefs, request.EvidenceScopes)
			if err != nil {
				return nil, fmt.Errorf("reflection_active_memory_candidate_%d_scope_invalid: %w", index, err)
			}
			command.ConversationID = conversationID
			if strings.TrimSpace(candidate.TargetRef) != "" {
				return nil, fmt.Errorf("reflection_active_memory_candidate_%d_target_forbidden", index)
			}
		} else {
			target, snapshot, err := activeMemoryTargetFromRef(candidate.TargetRef, request.ReferenceIndex)
			if err != nil {
				return nil, fmt.Errorf("reflection_active_memory_candidate_%d_target_invalid: %w", index, err)
			}
			if _, duplicate := touchedTargets[target.ActiveMemoryID]; duplicate {
				return nil, errors.New("reflection_active_memory_target_overlap")
			}
			touchedTargets[target.ActiveMemoryID] = struct{}{}
			command.Target = &target
			command.ProviderTargetRef = target.Ref
			command.ConversationID = strings.TrimSpace(stringValue(snapshot["conversation_id"]))
			if err := validateReflectionMemoryEvidenceScope(evidenceRefs, request.EvidenceScopes, command.ConversationID); err != nil {
				return nil, fmt.Errorf("reflection_active_memory_candidate_%d_scope_invalid: %w", index, err)
			}
		}
		needsSemantic := operation == ActiveMemoryCreate || operation == ActiveMemoryRevise || operation == ActiveMemorySupersede
		if needsSemantic {
			timePrecision := strings.TrimSpace(candidate.TimePrecision)
			if timePrecision == "" {
				timePrecision = "unknown"
			}
			semantic := ActiveMemorySemanticInput{
				Kind: strings.TrimSpace(candidate.Kind), Content: strings.TrimSpace(candidate.Content),
				Confidence: candidate.Confidence, Importance: candidate.Importance,
				OriginalTimeExpression: strings.TrimSpace(candidate.OriginalTimeExpression),
				ValidFrom:              candidate.ValidFrom, ValidUntil: candidate.ValidUntil,
				TimePrecision: timePrecision, Timezone: request.Timezone,
			}
			if err := validateActiveMemorySemantic(semantic, request.OccurredAt); err != nil {
				return nil, fmt.Errorf("reflection_active_memory_candidate_%d_semantic_invalid: %w", index, err)
			}
			command.Semantic = &semantic
			canonicalKey := activeMemoryCanonicalKey(command.OwnerFluctlightID, command.ConversationID, semantic)
			if _, duplicate := createdKeys[canonicalKey]; duplicate {
				return nil, errors.New("reflection_active_memory_semantic_overlap")
			}
			createdKeys[canonicalKey] = struct{}{}
		} else if strings.TrimSpace(candidate.Kind) != "" || strings.TrimSpace(candidate.Content) != "" || candidate.ValidFrom != nil || candidate.ValidUntil != nil || strings.TrimSpace(candidate.OriginalTimeExpression) != "" || strings.TrimSpace(candidate.TimePrecision) != "" {
			return nil, fmt.Errorf("reflection_active_memory_candidate_%d_semantic_forbidden", index)
		}
		if operation == ActiveMemoryComplete || operation == ActiveMemoryExpire {
			command.CloseReason = "reflection_" + string(operation)
		}
		command.RequestDigest = activeMemoryCommandDigest(command)
		if err := validatePreparedActiveMemoryMutation(command); err != nil {
			return nil, fmt.Errorf("reflection_active_memory_candidate_%d_compile_invalid: %w", index, err)
		}
		commands = append(commands, command)
	}
	return commands, nil
}

func reflectionActiveMemorySourceFactID(refs []string, scopes map[string]reflectionMemoryEvidenceScope) string {
	for _, ref := range refs {
		if factID := strings.TrimSpace(scopes[ref].FactID); factID != "" {
			return factID
		}
	}
	return ""
}

func (a *App) applyReflectionActiveMemoryCommandsTx(ctx context.Context, tx pgx.Tx, commands []PreparedActiveMemoryMutation) ([]ActiveMemoryApplyResult, error) {
	results := make([]ActiveMemoryApplyResult, 0, len(commands))
	for _, command := range commands {
		result, err := a.applyActiveMemoryCommandTx(ctx, tx, command)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func reflectionActiveMemoryResultSummary(results []ActiveMemoryApplyResult) map[string]any {
	counts := map[string]any{"applied": 0, "no_change": 0, "rejected": 0, "deferred": 0}
	items := make([]any, 0, len(results))
	for _, result := range results {
		if _, ok := counts[result.Disposition]; ok {
			counts[result.Disposition] = intValue(counts[result.Disposition]) + 1
		}
		items = append(items, map[string]any{
			"operation": string(result.Operation), "status": result.Status, "revision": result.Revision,
			"disposition": result.Disposition, "reason_code": result.ReasonCode, "replayed": result.Replayed,
		})
	}
	return map[string]any{"counts": counts, "results": items}
}

func normalizeReflectionMemoryCandidate(item map[string]any) map[string]any {
	result := make(map[string]any, len(reflectionMemoryCandidateFields)+1)
	for key, value := range item {
		if _, ok := reflectionMemoryCandidateFields[key]; !ok {
			result["__invalid_candidate"] = true
			continue
		}
		result[key] = value
	}
	return result
}

func validateReflectionMemoryCandidate(item map[string]any, allowedEvidence map[string]struct{}) error {
	if invalid, _ := item["__invalid_candidate"].(bool); invalid {
		return errors.New("reflection_memory_candidate_closed_schema_invalid")
	}
	operation := MemoryOperation(strings.TrimSpace(stringValue(item["operation"])))
	switch operation {
	case MemoryCreate, MemoryConfirm, MemoryRevise, MemoryMerge, MemorySupersede, MemoryDeprecate:
	default:
		return errors.New("reflection_memory_operation_invalid")
	}
	reason := strings.TrimSpace(stringValue(item["semantic_reason"]))
	if reason == "" || len([]rune(reason)) > 1000 {
		return errors.New("reflection_memory_reason_invalid")
	}
	refs := arrayValue(item["evidence_refs"])
	if len(refs) == 0 || len(refs) > 64 || !validateEvidenceRefs(refs, allowedEvidence) {
		return errors.New("reflection_memory_evidence_invalid")
	}
	seenEvidence := make(map[string]struct{}, len(refs))
	for _, raw := range refs {
		ref := strings.TrimSpace(stringValue(raw))
		if ref == "" || len([]rune(ref)) > 256 {
			return errors.New("reflection_memory_evidence_invalid")
		}
		if _, duplicate := seenEvidence[ref]; duplicate {
			return errors.New("reflection_memory_evidence_duplicate")
		}
		seenEvidence[ref] = struct{}{}
	}
	targetRef := strings.TrimSpace(stringValue(item["target_ref"]))
	if targetRef != "" && (len([]rune(targetRef)) > maxContextReferenceRunes || !contextReferencePattern.MatchString(targetRef)) {
		return errors.New("reflection_memory_target_ref_invalid")
	}
	mergeRefs := arrayValue(item["merge_refs"])
	if len(mergeRefs) > 16 {
		return errors.New("reflection_memory_merge_refs_invalid")
	}
	seenTargets := map[string]struct{}{}
	if targetRef != "" {
		seenTargets[targetRef] = struct{}{}
	}
	for _, raw := range mergeRefs {
		ref := strings.TrimSpace(stringValue(raw))
		if ref == "" || len([]rune(ref)) > maxContextReferenceRunes || !contextReferencePattern.MatchString(ref) {
			return errors.New("reflection_memory_merge_refs_invalid")
		}
		if _, duplicate := seenTargets[ref]; duplicate {
			return errors.New("reflection_memory_target_duplicate")
		}
		seenTargets[ref] = struct{}{}
	}
	semanticFields := []string{"type", "content", "confidence", "importance", "emotional_significance"}
	hasSemantic := false
	for _, field := range semanticFields {
		if _, present := item[field]; present {
			hasSemantic = true
			break
		}
	}
	needsSemantic := operation == MemoryCreate || operation == MemoryRevise || operation == MemoryMerge || operation == MemorySupersede
	if needsSemantic {
		if _, ok := validMemoryTypes[strings.TrimSpace(stringValue(item["type"]))]; !ok {
			return errors.New("reflection_memory_type_invalid")
		}
		content := strings.TrimSpace(stringValue(item["content"]))
		if content == "" || len([]rune(content)) > 32000 {
			return errors.New("reflection_memory_content_invalid")
		}
		for _, field := range []string{"confidence", "importance", "emotional_significance"} {
			if _, err := requiredBoundedNumber(item[field]); err != nil {
				return errors.New("reflection_memory_numeric_invalid")
			}
		}
	} else if hasSemantic {
		return errors.New("reflection_memory_semantic_forbidden")
	}
	switch operation {
	case MemoryCreate:
		if targetRef != "" || len(mergeRefs) > 0 {
			return errors.New("reflection_memory_create_target_forbidden")
		}
	case MemoryConfirm, MemoryRevise, MemorySupersede, MemoryDeprecate:
		if targetRef == "" || len(mergeRefs) > 0 {
			return errors.New("reflection_memory_target_invalid")
		}
	case MemoryMerge:
		if targetRef == "" || len(mergeRefs) == 0 {
			return errors.New("reflection_memory_merge_targets_invalid")
		}
	}
	return nil
}

func compileReflectionMemoryCommands(proposal map[string]any, request reflectionMemoryCompileRequest) ([]PreparedMemoryMutation, error) {
	if strings.TrimSpace(request.FluctlightID) == "" || strings.TrimSpace(request.OwnerActorID) == "" || strings.TrimSpace(request.ProposalID) == "" || strings.TrimSpace(request.SourceWindow) == "" || request.OccurredAt.IsZero() {
		return nil, errors.New("reflection_memory_compile_identity_invalid")
	}
	if err := request.ReferenceIndex.Validate(); err != nil {
		return nil, err
	}
	if request.ReferenceIndex.FluctlightID != request.FluctlightID || request.ReferenceIndex.OwnerActorID != request.OwnerActorID || request.ReferenceIndex.ActiveProfileID != request.ActiveProfileID {
		return nil, errors.New("reflection_memory_reference_scope_invalid")
	}
	items := arrayValue(proposal["memory_candidates"])
	commands := make([]PreparedMemoryMutation, 0, len(items))
	touchedTargets := make(map[string]struct{})
	createdKeys := make(map[string]struct{})
	for index, raw := range items {
		item := mapValue(raw)
		if err := validateReflectionMemoryCandidate(item, request.AllowedEvidence); err != nil {
			return nil, fmt.Errorf("reflection_memory_candidate_%d_invalid: %w", index, err)
		}
		operation := MemoryOperation(stringValue(item["operation"]))
		evidenceRefs := sortedUniqueStrings(decisionServiceRefValues(item["evidence_refs"]))
		command := PreparedMemoryMutation{
			SchemaVersion: memoryLifecycleSchemaVersion, Operation: operation,
			OwnerFluctlightID: request.FluctlightID, OwnerActorID: request.OwnerActorID, ActorID: request.FluctlightID,
			ActiveProfileID: request.ActiveProfileID, EvidenceRefs: evidenceRefs,
			Visibility: "private", OccurredAt: request.OccurredAt.UTC(),
			SourceFactID: reflectionMemorySourceFactID(evidenceRefs, request.EvidenceScopes, request.ProposalID),
			SourceWindow: request.SourceWindow, ProposalID: request.ProposalID, CandidateIndex: index,
			SemanticReason: strings.TrimSpace(stringValue(item["semantic_reason"])),
			IdempotencyKey: "memory:reflection:" + request.ProposalID + ":" + fmt.Sprint(index),
			ActorRefs:      []string{}, EventRefs: []string{}, PersonalityPerspectives: []any{},
		}
		if operation == MemoryCreate {
			conversationID, err := reflectionMemoryConversationID(evidenceRefs, request.EvidenceScopes)
			if err != nil {
				return nil, fmt.Errorf("reflection_memory_candidate_%d_scope_invalid: %w", index, err)
			}
			command.ConversationID = conversationID
		} else {
			target, snapshot, err := reflectionMemoryTargetFromRef(stringValue(item["target_ref"]), request.ReferenceIndex)
			if err != nil {
				return nil, fmt.Errorf("reflection_memory_candidate_%d_target_invalid: %w", index, err)
			}
			command.Target = &target
			command.ConversationID = strings.TrimSpace(stringValue(snapshot["conversation_id"]))
			command.Visibility = strings.TrimSpace(stringValue(snapshot["visibility"]))
			command.ActorRefs = decisionServiceRefValues(snapshot["actor_refs"])
			command.EventRefs = decisionServiceRefValues(snapshot["event_refs"])
			command.PersonalityPerspectives = arrayValue(snapshot["personality_perspectives"])
			if err := validateReflectionMemoryEvidenceScope(evidenceRefs, request.EvidenceScopes, command.ConversationID); err != nil {
				return nil, fmt.Errorf("reflection_memory_candidate_%d_scope_invalid: %w", index, err)
			}
			if _, duplicate := touchedTargets[target.MemoryID]; duplicate {
				return nil, errors.New("reflection_memory_target_overlap")
			}
			touchedTargets[target.MemoryID] = struct{}{}
			for _, rawRef := range arrayValue(item["merge_refs"]) {
				mergeTarget, mergeSnapshot, err := reflectionMemoryTargetFromRef(stringValue(rawRef), request.ReferenceIndex)
				if err != nil {
					return nil, fmt.Errorf("reflection_memory_candidate_%d_merge_target_invalid: %w", index, err)
				}
				if _, duplicate := touchedTargets[mergeTarget.MemoryID]; duplicate {
					return nil, errors.New("reflection_memory_target_overlap")
				}
				if !sameReflectionMemoryScope(snapshot, mergeSnapshot) {
					return nil, errors.New("reflection_memory_merge_scope_conflict")
				}
				touchedTargets[mergeTarget.MemoryID] = struct{}{}
				command.MergeTargets = append(command.MergeTargets, mergeTarget)
			}
		}
		if operation == MemoryCreate || operation == MemoryRevise || operation == MemoryMerge || operation == MemorySupersede {
			confidence, _ := requiredBoundedNumber(item["confidence"])
			importance, _ := requiredBoundedNumber(item["importance"])
			emotional, _ := requiredBoundedNumber(item["emotional_significance"])
			command.Semantic = &MemorySemanticInput{
				Type: stringValue(item["type"]), Content: stringValue(item["content"]),
				Confidence: confidence, Importance: importance, EmotionalSignificance: emotional,
			}
			canonicalKey := memoryCanonicalKey(*command.Semantic, command.ConversationID, command.Visibility, command.ActorRefs, command.EventRefs)
			if _, duplicate := createdKeys[canonicalKey]; duplicate {
				return nil, errors.New("reflection_memory_semantic_overlap")
			}
			createdKeys[canonicalKey] = struct{}{}
		}
		command.RequestDigest = memoryCommandDigest(command)
		if err := validatePreparedMemoryMutation(command); err != nil {
			return nil, fmt.Errorf("reflection_memory_candidate_%d_compile_invalid: %w", index, err)
		}
		commands = append(commands, command)
	}
	return commands, nil
}

func (a *App) applyReflectionMemoryCommandsTx(ctx context.Context, tx pgx.Tx, commands []PreparedMemoryMutation) ([]MemoryApplyResult, error) {
	results := make([]MemoryApplyResult, 0, len(commands))
	for _, command := range commands {
		result, err := a.applyMemoryCommandTx(ctx, tx, command)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func reflectionMemoryResultSummary(results []MemoryApplyResult) map[string]any {
	counts := map[string]any{"applied": 0, "no_change": 0, "rejected": 0, "deferred": 0}
	items := make([]any, 0, len(results))
	for _, result := range results {
		if _, ok := counts[result.Disposition]; ok {
			counts[result.Disposition] = intValue(counts[result.Disposition]) + 1
		}
		items = append(items, map[string]any{
			"operation": string(result.Operation), "status": result.Status, "revision": result.Revision,
			"disposition": result.Disposition, "reason_code": result.ReasonCode, "replayed": result.Replayed,
		})
	}
	return map[string]any{"counts": counts, "results": items}
}

func reflectionMemoryTargetFromRef(ref string, index ContextReferenceIndex) (MemoryTarget, map[string]any, error) {
	ref = strings.TrimSpace(ref)
	entry, ok := index.ByRef[ref]
	if !ok || entry.Kind != ContextReferenceMemory || entry.Ref != ref {
		return MemoryTarget{}, nil, errors.New("memory_context_ref_unknown")
	}
	snapshot := decodeObject(entry.Snapshot)
	if stringValue(snapshot["id"]) != entry.EntityID || intValue(snapshot["revision"]) != entry.Revision || stringValue(snapshot["status"]) != "active" {
		return MemoryTarget{}, nil, errors.New("memory_context_ref_snapshot_invalid")
	}
	return MemoryTarget{Ref: ref, MemoryID: entry.EntityID, ExpectedRevision: entry.Revision}, snapshot, nil
}

func reflectionMemoryConversationID(refs []string, scopes map[string]reflectionMemoryEvidenceScope) (string, error) {
	conversationID := ""
	for _, ref := range refs {
		scope, ok := scopes[ref]
		if !ok || !scope.Known {
			return "", errors.New("memory_evidence_scope_unknown")
		}
		if strings.TrimSpace(scope.ConversationID) == "" {
			continue
		}
		if conversationID != "" && conversationID != strings.TrimSpace(scope.ConversationID) {
			return "", errors.New("memory_evidence_conversation_conflict")
		}
		conversationID = strings.TrimSpace(scope.ConversationID)
	}
	return conversationID, nil
}

func validateReflectionMemoryEvidenceScope(refs []string, scopes map[string]reflectionMemoryEvidenceScope, targetConversationID string) error {
	targetConversationID = strings.TrimSpace(targetConversationID)
	for _, ref := range refs {
		scope, ok := scopes[ref]
		if !ok || !scope.Known {
			return errors.New("memory_evidence_scope_unknown")
		}
		conversationID := strings.TrimSpace(scope.ConversationID)
		if conversationID == "" {
			continue
		}
		if targetConversationID == "" || conversationID != targetConversationID {
			return errors.New("memory_evidence_conversation_conflict")
		}
	}
	return nil
}

func reflectionMemorySourceFactID(refs []string, scopes map[string]reflectionMemoryEvidenceScope, proposalID string) string {
	for _, ref := range refs {
		if factID := strings.TrimSpace(scopes[ref].FactID); factID != "" {
			return factID
		}
	}
	return "reflection:" + proposalID
}

func (a *App) reflectionOutcomeEvidenceScope(ctx context.Context, fluctlightID string, entry ContextReference) (reflectionMemoryEvidenceScope, error) {
	if entry.Kind != ContextReferenceOutcome {
		return reflectionMemoryEvidenceScope{}, errors.New("reflection_outcome_ref_kind_invalid")
	}
	snapshot := decodeObject(entry.Snapshot)
	conversationID := ""
	for _, factID := range decisionServiceRefValues(snapshot["evidence_refs"]) {
		var candidate string
		err := a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(payload->>'conversation_id','') FROM public.cognition_inbox WHERE id=$1 AND fluctlight_id=$2`, factID, fluctlightID).Scan(&candidate)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return reflectionMemoryEvidenceScope{}, err
		}
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if conversationID != "" && conversationID != candidate {
			return reflectionMemoryEvidenceScope{}, errors.New("reflection_outcome_conversation_conflict")
		}
		conversationID = candidate
	}
	return reflectionMemoryEvidenceScope{ConversationID: conversationID, Known: true}, nil
}

func sameReflectionMemoryScope(left, right map[string]any) bool {
	return stringValue(left["conversation_id"]) == stringValue(right["conversation_id"]) &&
		stringValue(left["visibility"]) == stringValue(right["visibility"]) &&
		jsonString(sortedUniqueStrings(decisionServiceRefValues(left["actor_refs"]))) == jsonString(sortedUniqueStrings(decisionServiceRefValues(right["actor_refs"]))) &&
		jsonString(sortedUniqueStrings(decisionServiceRefValues(left["event_refs"]))) == jsonString(sortedUniqueStrings(decisionServiceRefValues(right["event_refs"])))
}
