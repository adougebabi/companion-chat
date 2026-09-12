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

const (
	activeMemoryLifecycleSchemaVersion = "active-memory.lifecycle.v1"
	activeMemoryLifecyclePolicyVersion = "active-memory.policy.v1"
)

type ActiveMemoryOperation string

const (
	ActiveMemoryCreate    ActiveMemoryOperation = "create"
	ActiveMemoryConfirm   ActiveMemoryOperation = "confirm"
	ActiveMemoryRevise    ActiveMemoryOperation = "revise"
	ActiveMemoryComplete  ActiveMemoryOperation = "complete"
	ActiveMemoryExpire    ActiveMemoryOperation = "expire"
	ActiveMemorySupersede ActiveMemoryOperation = "supersede"
)

var validActiveMemoryKinds = map[string]struct{}{
	"future_event": {}, "commitment": {}, "temporary_context": {},
}

var validActiveMemoryTimePrecisions = map[string]struct{}{
	"exact": {}, "part_of_day": {}, "date": {}, "range": {}, "unknown": {},
}

type ActiveMemorySemanticInput struct {
	Kind                   string     `json:"kind"`
	Content                string     `json:"content"`
	Confidence             float64    `json:"confidence"`
	Importance             float64    `json:"importance"`
	OriginalTimeExpression string     `json:"original_time_expression,omitempty"`
	ValidFrom              *time.Time `json:"valid_from,omitempty"`
	ValidUntil             *time.Time `json:"valid_until,omitempty"`
	TimePrecision          string     `json:"time_precision"`
	Timezone               string     `json:"timezone"`
}

type ActiveMemoryTarget struct {
	Ref              string `json:"ref,omitempty"`
	ActiveMemoryID   string `json:"active_memory_id"`
	ExpectedRevision int    `json:"expected_revision"`
}

type PreparedActiveMemoryMutation struct {
	SchemaVersion     string                     `json:"schema_version"`
	Operation         ActiveMemoryOperation      `json:"operation"`
	OwnerFluctlightID string                     `json:"owner_fluctlight_id"`
	OwnerActorID      string                     `json:"owner_actor_id"`
	ActorID           string                     `json:"actor_id"`
	ActorRefs         []string                   `json:"actor_refs"`
	ConversationID    string                     `json:"conversation_id,omitempty"`
	SourceFactID      string                     `json:"source_fact_id"`
	EvidenceRefs      []string                   `json:"evidence_refs"`
	OccurredAt        time.Time                  `json:"occurred_at"`
	Target            *ActiveMemoryTarget        `json:"target,omitempty"`
	Semantic          *ActiveMemorySemanticInput `json:"semantic,omitempty"`
	SemanticReason    string                     `json:"semantic_reason"`
	IdempotencyKey    string                     `json:"idempotency_key"`
	RequestDigest     string                     `json:"request_digest"`
	CloseReason       string                     `json:"close_reason,omitempty"`
	ProviderTargetRef string                     `json:"provider_target_ref,omitempty"`
}

type ActiveMemoryApplyResult struct {
	Operation          ActiveMemoryOperation `json:"operation"`
	ActiveMemoryID     string                `json:"active_memory_id"`
	Status             string                `json:"status"`
	Revision           int                   `json:"revision"`
	Disposition        string                `json:"disposition"`
	ReasonCode         string                `json:"reason_code"`
	RelatedMemoryIDs   []string              `json:"related_memory_ids"`
	ResultingRevisions map[string]int        `json:"resulting_revisions"`
	Replayed           bool                  `json:"replayed"`
}

type activeMemoryAuthorityRow struct {
	ID                     string
	OwnerFluctlightID      string
	ConversationID         string
	Kind                   string
	Content                string
	Status                 string
	Confidence             float64
	Importance             float64
	ActorRefs              []any
	SourceFactID           string
	EvidenceRefs           []any
	OriginalTimeExpression string
	ValidFrom              *time.Time
	ValidUntil             *time.Time
	TimePrecision          string
	Timezone               string
	LastRelevantAt         time.Time
	Revision               int
	CanonicalKey           string
	RequestDigest          string
	SupersededByID         string
	SupersedesID           string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	ClosedAt               *time.Time
}

func activeMemoryCanonicalKey(ownerFluctlightID, conversationID string, semantic ActiveMemorySemanticInput) string {
	validFrom, validUntil := "", ""
	if semantic.ValidFrom != nil {
		validFrom = semantic.ValidFrom.UTC().Format(time.RFC3339Nano)
	}
	if semantic.ValidUntil != nil {
		validUntil = semantic.ValidUntil.UTC().Format(time.RFC3339Nano)
	}
	return stableDigest(strings.Join([]string{
		strings.TrimSpace(ownerFluctlightID), strings.TrimSpace(conversationID), strings.TrimSpace(semantic.Kind),
		strings.TrimSpace(semantic.Content), strings.TrimSpace(semantic.OriginalTimeExpression), validFrom, validUntil,
		strings.TrimSpace(semantic.TimePrecision), strings.TrimSpace(semantic.Timezone),
	}, "\x1f"))
}

func activeMemoryCommandDigest(command PreparedActiveMemoryMutation) string {
	copyCommand := command
	copyCommand.RequestDigest = ""
	return stableDigest(jsonString(copyCommand))
}

func validateActiveMemorySemantic(input ActiveMemorySemanticInput, occurredAt time.Time) error {
	if _, ok := validActiveMemoryKinds[strings.TrimSpace(input.Kind)]; !ok {
		return errors.New("active_memory_kind_invalid")
	}
	input.Content = strings.TrimSpace(input.Content)
	if input.Content == "" || len([]rune(input.Content)) > 4000 || !unitFinite(input.Confidence) || !unitFinite(input.Importance) {
		return errors.New("active_memory_semantic_invalid")
	}
	if _, ok := validActiveMemoryTimePrecisions[strings.TrimSpace(input.TimePrecision)]; !ok {
		return errors.New("active_memory_time_precision_invalid")
	}
	if strings.TrimSpace(input.Timezone) == "" {
		return errors.New("active_memory_timezone_invalid")
	}
	location, err := time.LoadLocation(input.Timezone)
	if err != nil {
		return errors.New("active_memory_timezone_invalid")
	}
	original := strings.TrimSpace(input.OriginalTimeExpression)
	if len([]rune(original)) > 512 {
		return errors.New("active_memory_time_expression_invalid")
	}
	if input.TimePrecision == "unknown" {
		if input.ValidFrom != nil || input.ValidUntil != nil {
			return errors.New("active_memory_unknown_time_must_be_unresolved")
		}
	} else if original == "" {
		return errors.New("active_memory_time_expression_required")
	}
	if input.ValidFrom != nil && input.ValidUntil != nil && !input.ValidUntil.After(*input.ValidFrom) {
		return errors.New("active_memory_time_range_invalid")
	}
	const maxActiveMemoryHorizon = 366 * 24 * time.Hour
	for _, value := range []*time.Time{input.ValidFrom, input.ValidUntil} {
		if value != nil {
			if value.Before(occurredAt.Add(-24*time.Hour)) || value.After(occurredAt.Add(maxActiveMemoryHorizon)) {
				return errors.New("active_memory_time_horizon_invalid")
			}
			_, suppliedOffset := value.Zone()
			_, expectedOffset := value.In(location).Zone()
			if suppliedOffset != expectedOffset {
				return errors.New("active_memory_time_offset_invalid")
			}
		}
	}
	return nil
}

func validatePreparedActiveMemoryMutation(command PreparedActiveMemoryMutation) error {
	if command.SchemaVersion != activeMemoryLifecycleSchemaVersion || strings.TrimSpace(command.OwnerFluctlightID) == "" || strings.TrimSpace(command.OwnerActorID) == "" || strings.TrimSpace(command.ActorID) == "" || strings.TrimSpace(command.SourceFactID) == "" || command.OccurredAt.IsZero() {
		return errors.New("active_memory_command_identity_invalid")
	}
	if len(command.EvidenceRefs) == 0 || len(command.EvidenceRefs) > 64 || strings.TrimSpace(command.SemanticReason) == "" || len([]rune(strings.TrimSpace(command.SemanticReason))) > 1000 || strings.TrimSpace(command.IdempotencyKey) == "" || len([]rune(command.IdempotencyKey)) > 256 {
		return errors.New("active_memory_command_provenance_invalid")
	}
	if err := validateBoundedRefs(command.EvidenceRefs); err != nil {
		return errors.New("active_memory_command_provenance_invalid")
	}
	if len(command.ActorRefs) > 64 || validateBoundedRefs(command.ActorRefs) != nil {
		return errors.New("active_memory_command_actor_refs_invalid")
	}
	if command.RequestDigest == "" || command.RequestDigest != activeMemoryCommandDigest(command) {
		return errors.New("active_memory_command_digest_invalid")
	}
	needsSemantic := command.Operation == ActiveMemoryCreate || command.Operation == ActiveMemoryRevise || command.Operation == ActiveMemorySupersede
	if needsSemantic {
		if command.Semantic == nil || validateActiveMemorySemantic(*command.Semantic, command.OccurredAt) != nil {
			return errors.New("active_memory_command_semantic_invalid")
		}
	} else if command.Semantic != nil {
		return errors.New("active_memory_command_semantic_forbidden")
	}
	switch command.Operation {
	case ActiveMemoryCreate:
		if command.Target != nil {
			return errors.New("active_memory_create_target_forbidden")
		}
	case ActiveMemoryConfirm, ActiveMemoryRevise, ActiveMemoryComplete, ActiveMemoryExpire, ActiveMemorySupersede:
		if command.Target == nil || strings.TrimSpace(command.Target.ActiveMemoryID) == "" || command.Target.ExpectedRevision < 1 || (command.ProviderTargetRef != "" && command.Target.Ref != command.ProviderTargetRef) {
			return errors.New("active_memory_target_invalid")
		}
	default:
		return errors.New("active_memory_operation_invalid")
	}
	return nil
}

func (a *App) applyActiveMemoryCommandTx(ctx context.Context, tx pgx.Tx, command PreparedActiveMemoryMutation) (ActiveMemoryApplyResult, error) {
	if err := validatePreparedActiveMemoryMutation(command); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	var liveOwner, fluctlightStatus string
	if err := tx.QueryRow(ctx, `SELECT created_by_actor_id,status FROM public.fluctlights WHERE id=$1 FOR UPDATE`, command.OwnerFluctlightID).Scan(&liveOwner, &fluctlightStatus); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	if liveOwner != command.OwnerActorID || (fluctlightStatus != "active" && fluctlightStatus != "paused") {
		return ActiveMemoryApplyResult{}, errors.New("active_memory_owner_stale")
	}
	if replay, found, err := readActiveMemoryCommandReplayTx(ctx, tx, command); err != nil {
		return ActiveMemoryApplyResult{}, err
	} else if found {
		return replay, nil
	}
	if command.Operation == ActiveMemoryCreate {
		return a.applyActiveMemoryCreateTx(ctx, tx, command)
	}
	target, err := readActiveMemoryAuthorityRowTx(ctx, tx, command.Target.ActiveMemoryID)
	if err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	if target.OwnerFluctlightID != command.OwnerFluctlightID || target.Revision != command.Target.ExpectedRevision || target.Status != "active" {
		return ActiveMemoryApplyResult{}, errors.New("active_memory_target_revision_conflict")
	}
	switch command.Operation {
	case ActiveMemoryConfirm:
		return a.applyActiveMemoryConfirmTx(ctx, tx, command, target)
	case ActiveMemoryRevise:
		return a.applyActiveMemoryReviseTx(ctx, tx, command, target)
	case ActiveMemoryComplete, ActiveMemoryExpire:
		return a.applyActiveMemoryCloseTx(ctx, tx, command, target)
	case ActiveMemorySupersede:
		return a.applyActiveMemorySupersedeTx(ctx, tx, command, target)
	default:
		return ActiveMemoryApplyResult{}, errors.New("active_memory_operation_invalid")
	}
}

func readActiveMemoryCommandReplayTx(ctx context.Context, tx pgx.Tx, command PreparedActiveMemoryMutation) (ActiveMemoryApplyResult, bool, error) {
	var digest string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_digest,result FROM public.active_memory_commands WHERE owner_fluctlight_id=$1 AND idempotency_key=$2`, command.OwnerFluctlightID, command.IdempotencyKey).Scan(&digest, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return ActiveMemoryApplyResult{}, false, nil
	}
	if err != nil {
		return ActiveMemoryApplyResult{}, false, err
	}
	if digest != command.RequestDigest {
		return ActiveMemoryApplyResult{}, false, errors.New("active_memory_idempotency_conflict")
	}
	var result ActiveMemoryApplyResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return ActiveMemoryApplyResult{}, false, errors.New("active_memory_command_result_invalid")
	}
	result.Replayed = true
	return result, true, nil
}

func (a *App) applyActiveMemoryCreateTx(ctx context.Context, tx pgx.Tx, command PreparedActiveMemoryMutation) (ActiveMemoryApplyResult, error) {
	semantic := *command.Semantic
	canonicalKey := activeMemoryCanonicalKey(command.OwnerFluctlightID, command.ConversationID, semantic)
	var existingID, existingStatus string
	var existingRevision int
	err := tx.QueryRow(ctx, `SELECT id,status,revision FROM public.active_memories WHERE owner_fluctlight_id=$1 AND canonical_key=$2 AND status='active' FOR UPDATE`, command.OwnerFluctlightID, canonicalKey).Scan(&existingID, &existingStatus, &existingRevision)
	if err == nil {
		result := ActiveMemoryApplyResult{Operation: command.Operation, ActiveMemoryID: existingID, Status: existingStatus, Revision: existingRevision, Disposition: "no_change", ReasonCode: "exact_duplicate", RelatedMemoryIDs: []string{}, ResultingRevisions: map[string]int{existingID: existingRevision}}
		return a.finishActiveMemoryCommandTx(ctx, tx, command, result)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ActiveMemoryApplyResult{}, err
	}
	memoryID := "active_memory_" + stableDigest(command.OwnerFluctlightID+":"+command.IdempotencyKey)
	row := activeMemoryAuthorityRow{
		ID: memoryID, OwnerFluctlightID: command.OwnerFluctlightID, ConversationID: command.ConversationID,
		Kind: semantic.Kind, Content: strings.TrimSpace(semantic.Content), Status: "active",
		Confidence: semantic.Confidence, Importance: semantic.Importance, ActorRefs: stringSliceAny(sortedUniqueStrings(command.ActorRefs)),
		SourceFactID: command.SourceFactID, EvidenceRefs: stringSliceAny(command.EvidenceRefs),
		OriginalTimeExpression: strings.TrimSpace(semantic.OriginalTimeExpression), ValidFrom: semantic.ValidFrom, ValidUntil: semantic.ValidUntil,
		TimePrecision: semantic.TimePrecision, Timezone: semantic.Timezone, LastRelevantAt: command.OccurredAt.UTC(),
		Revision: 1, CanonicalKey: canonicalKey, RequestDigest: command.RequestDigest,
		CreatedAt: command.OccurredAt.UTC(), UpdatedAt: command.OccurredAt.UTC(),
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.active_memories(id,owner_fluctlight_id,conversation_id,kind,content,status,confidence,importance,actor_refs,source_fact_id,evidence_refs,original_time_expression,valid_from,valid_until,time_precision,timezone,last_relevant_at,revision,canonical_key,request_digest,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'active',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,1,$17,$18,$19,$19)`, row.ID, row.OwnerFluctlightID, nullableString(row.ConversationID), row.Kind, row.Content, row.Confidence, row.Importance, jsonBytes(row.ActorRefs), row.SourceFactID, jsonBytes(row.EvidenceRefs), nullableString(row.OriginalTimeExpression), row.ValidFrom, row.ValidUntil, row.TimePrecision, row.Timezone, row.LastRelevantAt, row.CanonicalKey, row.RequestDigest, row.CreatedAt); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	if err := writeActiveMemoryRevisionTx(ctx, tx, command, row, 0); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	result := ActiveMemoryApplyResult{Operation: command.Operation, ActiveMemoryID: row.ID, Status: row.Status, Revision: row.Revision, Disposition: "applied", ReasonCode: "created", RelatedMemoryIDs: []string{}, ResultingRevisions: map[string]int{row.ID: row.Revision}}
	return a.finishActiveMemoryCommandTx(ctx, tx, command, result)
}

func (a *App) applyActiveMemoryConfirmTx(ctx context.Context, tx pgx.Tx, command PreparedActiveMemoryMutation, row activeMemoryAuthorityRow) (ActiveMemoryApplyResult, error) {
	row.Revision++
	row.EvidenceRefs = stringSliceAny(sortedUniqueStrings(append(decisionServiceRefValues(row.EvidenceRefs), command.EvidenceRefs...)))
	row.LastRelevantAt = command.OccurredAt.UTC()
	row.UpdatedAt = command.OccurredAt.UTC()
	row.RequestDigest = command.RequestDigest
	commandTag, err := tx.Exec(ctx, `UPDATE public.active_memories SET evidence_refs=$2,last_relevant_at=$3,updated_at=$3,revision=$4,request_digest=$5 WHERE id=$1 AND status='active' AND revision=$6`, row.ID, jsonBytes(row.EvidenceRefs), row.UpdatedAt, row.Revision, row.RequestDigest, row.Revision-1)
	if err != nil || commandTag.RowsAffected() != 1 {
		if err != nil {
			return ActiveMemoryApplyResult{}, err
		}
		return ActiveMemoryApplyResult{}, ErrConflict
	}
	if err := writeActiveMemoryRevisionTx(ctx, tx, command, row, row.Revision-1); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	result := ActiveMemoryApplyResult{Operation: command.Operation, ActiveMemoryID: row.ID, Status: row.Status, Revision: row.Revision, Disposition: "applied", ReasonCode: "confirmed", RelatedMemoryIDs: []string{}, ResultingRevisions: map[string]int{row.ID: row.Revision}}
	return a.finishActiveMemoryCommandTx(ctx, tx, command, result)
}

func (a *App) applyActiveMemoryReviseTx(ctx context.Context, tx pgx.Tx, command PreparedActiveMemoryMutation, row activeMemoryAuthorityRow) (ActiveMemoryApplyResult, error) {
	semantic := *command.Semantic
	row.Kind, row.Content, row.Confidence, row.Importance = semantic.Kind, strings.TrimSpace(semantic.Content), semantic.Confidence, semantic.Importance
	row.OriginalTimeExpression, row.ValidFrom, row.ValidUntil = strings.TrimSpace(semantic.OriginalTimeExpression), semantic.ValidFrom, semantic.ValidUntil
	row.TimePrecision, row.Timezone = semantic.TimePrecision, semantic.Timezone
	row.EvidenceRefs = stringSliceAny(sortedUniqueStrings(append(decisionServiceRefValues(row.EvidenceRefs), command.EvidenceRefs...)))
	row.LastRelevantAt, row.UpdatedAt = command.OccurredAt.UTC(), command.OccurredAt.UTC()
	row.Revision++
	row.CanonicalKey = activeMemoryCanonicalKey(row.OwnerFluctlightID, row.ConversationID, semantic)
	row.RequestDigest = command.RequestDigest
	commandTag, err := tx.Exec(ctx, `UPDATE public.active_memories SET kind=$2,content=$3,confidence=$4,importance=$5,evidence_refs=$6,original_time_expression=$7,valid_from=$8,valid_until=$9,time_precision=$10,timezone=$11,last_relevant_at=$12,updated_at=$12,revision=$13,canonical_key=$14,request_digest=$15 WHERE id=$1 AND status='active' AND revision=$16`, row.ID, row.Kind, row.Content, row.Confidence, row.Importance, jsonBytes(row.EvidenceRefs), nullableString(row.OriginalTimeExpression), row.ValidFrom, row.ValidUntil, row.TimePrecision, row.Timezone, row.UpdatedAt, row.Revision, row.CanonicalKey, row.RequestDigest, row.Revision-1)
	if err != nil || commandTag.RowsAffected() != 1 {
		if err != nil {
			return ActiveMemoryApplyResult{}, err
		}
		return ActiveMemoryApplyResult{}, ErrConflict
	}
	if err := writeActiveMemoryRevisionTx(ctx, tx, command, row, row.Revision-1); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	result := ActiveMemoryApplyResult{Operation: command.Operation, ActiveMemoryID: row.ID, Status: row.Status, Revision: row.Revision, Disposition: "applied", ReasonCode: "revised", RelatedMemoryIDs: []string{}, ResultingRevisions: map[string]int{row.ID: row.Revision}}
	return a.finishActiveMemoryCommandTx(ctx, tx, command, result)
}

func (a *App) applyActiveMemoryCloseTx(ctx context.Context, tx pgx.Tx, command PreparedActiveMemoryMutation, row activeMemoryAuthorityRow) (ActiveMemoryApplyResult, error) {
	status := "completed"
	if command.Operation == ActiveMemoryExpire {
		status = "expired"
	}
	row.Status, row.ClosedAt = status, timePointer(command.OccurredAt.UTC())
	row.UpdatedAt, row.LastRelevantAt = command.OccurredAt.UTC(), command.OccurredAt.UTC()
	row.Revision++
	row.RequestDigest = command.RequestDigest
	commandTag, err := tx.Exec(ctx, `UPDATE public.active_memories SET status=$2,closed_at=$3,updated_at=$3,last_relevant_at=$3,revision=$4,request_digest=$5 WHERE id=$1 AND status='active' AND revision=$6`, row.ID, row.Status, *row.ClosedAt, row.Revision, row.RequestDigest, row.Revision-1)
	if err != nil || commandTag.RowsAffected() != 1 {
		if err != nil {
			return ActiveMemoryApplyResult{}, err
		}
		return ActiveMemoryApplyResult{}, ErrConflict
	}
	if err := writeActiveMemoryRevisionTx(ctx, tx, command, row, row.Revision-1); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	result := ActiveMemoryApplyResult{Operation: command.Operation, ActiveMemoryID: row.ID, Status: row.Status, Revision: row.Revision, Disposition: "applied", ReasonCode: status, RelatedMemoryIDs: []string{}, ResultingRevisions: map[string]int{row.ID: row.Revision}}
	return a.finishActiveMemoryCommandTx(ctx, tx, command, result)
}

func (a *App) applyActiveMemorySupersedeTx(ctx context.Context, tx pgx.Tx, command PreparedActiveMemoryMutation, prior activeMemoryAuthorityRow) (ActiveMemoryApplyResult, error) {
	semantic := *command.Semantic
	replacementID := "active_memory_" + stableDigest(command.OwnerFluctlightID+":"+command.IdempotencyKey+":replacement")
	prior.Revision++
	prior.Status, prior.SupersededByID = "superseded", replacementID
	prior.ClosedAt, prior.UpdatedAt, prior.RequestDigest = timePointer(command.OccurredAt.UTC()), command.OccurredAt.UTC(), command.RequestDigest
	commandTag, err := tx.Exec(ctx, `UPDATE public.active_memories SET status='superseded',superseded_by_active_memory_id=$2,closed_at=$3,updated_at=$3,revision=$4,request_digest=$5 WHERE id=$1 AND status='active' AND revision=$6`, prior.ID, replacementID, *prior.ClosedAt, prior.Revision, prior.RequestDigest, prior.Revision-1)
	if err != nil || commandTag.RowsAffected() != 1 {
		if err != nil {
			return ActiveMemoryApplyResult{}, err
		}
		return ActiveMemoryApplyResult{}, ErrConflict
	}
	replacement := activeMemoryAuthorityRow{
		ID: replacementID, OwnerFluctlightID: command.OwnerFluctlightID, ConversationID: prior.ConversationID,
		Kind: semantic.Kind, Content: strings.TrimSpace(semantic.Content), Status: "active",
		Confidence: semantic.Confidence, Importance: semantic.Importance, ActorRefs: append([]any(nil), prior.ActorRefs...),
		SourceFactID: command.SourceFactID, EvidenceRefs: stringSliceAny(command.EvidenceRefs),
		OriginalTimeExpression: strings.TrimSpace(semantic.OriginalTimeExpression), ValidFrom: semantic.ValidFrom, ValidUntil: semantic.ValidUntil,
		TimePrecision: semantic.TimePrecision, Timezone: semantic.Timezone, LastRelevantAt: command.OccurredAt.UTC(),
		Revision: 1, CanonicalKey: activeMemoryCanonicalKey(command.OwnerFluctlightID, prior.ConversationID, semantic), RequestDigest: command.RequestDigest,
		SupersedesID: prior.ID, CreatedAt: command.OccurredAt.UTC(), UpdatedAt: command.OccurredAt.UTC(),
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.active_memories(id,owner_fluctlight_id,conversation_id,kind,content,status,confidence,importance,actor_refs,source_fact_id,evidence_refs,original_time_expression,valid_from,valid_until,time_precision,timezone,last_relevant_at,revision,canonical_key,request_digest,supersedes_active_memory_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'active',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,1,$17,$18,$19,$20,$20)`, replacement.ID, replacement.OwnerFluctlightID, nullableString(replacement.ConversationID), replacement.Kind, replacement.Content, replacement.Confidence, replacement.Importance, jsonBytes(replacement.ActorRefs), replacement.SourceFactID, jsonBytes(replacement.EvidenceRefs), nullableString(replacement.OriginalTimeExpression), replacement.ValidFrom, replacement.ValidUntil, replacement.TimePrecision, replacement.Timezone, replacement.LastRelevantAt, replacement.CanonicalKey, replacement.RequestDigest, replacement.SupersedesID, replacement.CreatedAt); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	if err := writeActiveMemoryRevisionTx(ctx, tx, command, prior, prior.Revision-1); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	if err := writeActiveMemoryRevisionTx(ctx, tx, command, replacement, 0); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	result := ActiveMemoryApplyResult{Operation: command.Operation, ActiveMemoryID: replacement.ID, Status: replacement.Status, Revision: replacement.Revision, Disposition: "applied", ReasonCode: "superseded", RelatedMemoryIDs: []string{prior.ID}, ResultingRevisions: map[string]int{prior.ID: prior.Revision, replacement.ID: replacement.Revision}}
	return a.finishActiveMemoryCommandTx(ctx, tx, command, result)
}

func (a *App) finishActiveMemoryCommandTx(ctx context.Context, tx pgx.Tx, command PreparedActiveMemoryMutation, result ActiveMemoryApplyResult) (ActiveMemoryApplyResult, error) {
	if err := persistActiveMemoryCommandTx(ctx, tx, command, result); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	if err := appendOutboxTx(ctx, tx, "active_memory.lifecycle.applied", "active_memory", result.ActiveMemoryID, command.OwnerFluctlightID, command.SourceFactID, "active-memory:"+result.ActiveMemoryID, "active-memory:"+command.IdempotencyKey, map[string]any{
		"active_memory_id": result.ActiveMemoryID, "operation": result.Operation, "status": result.Status,
		"revision": result.Revision, "disposition": result.Disposition, "reason_code": result.ReasonCode,
		"related_memory_ids": result.RelatedMemoryIDs, "resulting_revisions": result.ResultingRevisions,
	}); err != nil {
		return ActiveMemoryApplyResult{}, err
	}
	return result, nil
}

func persistActiveMemoryCommandTx(ctx context.Context, tx pgx.Tx, command PreparedActiveMemoryMutation, result ActiveMemoryApplyResult) error {
	if result.RelatedMemoryIDs == nil {
		result.RelatedMemoryIDs = []string{}
	}
	if result.ResultingRevisions == nil {
		result.ResultingRevisions = map[string]int{}
	}
	targetID := ""
	if command.Target != nil {
		targetID = command.Target.ActiveMemoryID
	}
	commandID := "active_memory_command_" + stableDigest(command.OwnerFluctlightID+":"+command.IdempotencyKey)
	inserted, err := tx.Exec(ctx, `INSERT INTO public.active_memory_commands(id,owner_fluctlight_id,conversation_id,source_fact_id,operation,target_active_memory_id,actor_id,evidence_refs,semantic_reason,request_digest,command,result,schema_version,policy_version,idempotency_key,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) ON CONFLICT(owner_fluctlight_id,idempotency_key) DO NOTHING`, commandID, command.OwnerFluctlightID, nullableString(command.ConversationID), command.SourceFactID, command.Operation, nullableString(targetID), command.ActorID, jsonBytes(command.EvidenceRefs), command.SemanticReason, command.RequestDigest, jsonBytes(command), jsonBytes(result), activeMemoryLifecycleSchemaVersion, activeMemoryLifecyclePolicyVersion, command.IdempotencyKey, command.OccurredAt.UTC())
	if err != nil {
		return err
	}
	if inserted.RowsAffected() != 1 {
		return errors.New("active_memory_command_identity_conflict")
	}
	return nil
}

func writeActiveMemoryRevisionTx(ctx context.Context, tx pgx.Tx, command PreparedActiveMemoryMutation, row activeMemoryAuthorityRow, baseRevision int) error {
	revisionID := "active_memory_revision_" + stableDigest(row.ID+":"+fmt.Sprint(row.Revision))
	idempotency := "active-memory-revision:" + row.ID + ":" + fmt.Sprint(row.Revision)
	inserted, err := tx.Exec(ctx, `INSERT INTO public.active_memory_revisions(id,active_memory_id,revision,base_revision,operation,snapshot,status,actor_id,evidence_refs,semantic_reason,request_digest,schema_version,policy_version,idempotency_key,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT(active_memory_id,revision) DO NOTHING`, revisionID, row.ID, row.Revision, baseRevision, command.Operation, jsonBytes(activeMemorySnapshot(row)), row.Status, command.ActorID, jsonBytes(row.EvidenceRefs), command.SemanticReason, command.RequestDigest, activeMemoryLifecycleSchemaVersion, activeMemoryLifecyclePolicyVersion, idempotency, command.OccurredAt.UTC())
	if err != nil {
		return err
	}
	if inserted.RowsAffected() != 1 {
		return errors.New("active_memory_revision_identity_conflict")
	}
	return nil
}

func activeMemorySnapshot(row activeMemoryAuthorityRow) map[string]any {
	return map[string]any{
		"id": row.ID, "owner_fluctlight_id": row.OwnerFluctlightID, "conversation_id": row.ConversationID,
		"kind": row.Kind, "content": row.Content, "status": row.Status, "confidence": row.Confidence, "importance": row.Importance,
		"actor_refs": row.ActorRefs, "source_fact_id": row.SourceFactID, "evidence_refs": row.EvidenceRefs,
		"original_time_expression": row.OriginalTimeExpression, "valid_from": row.ValidFrom, "valid_until": row.ValidUntil,
		"time_precision": row.TimePrecision, "timezone": row.Timezone, "last_relevant_at": row.LastRelevantAt,
		"revision": row.Revision, "canonical_key": row.CanonicalKey, "request_digest": row.RequestDigest,
		"superseded_by_active_memory_id": row.SupersededByID, "supersedes_active_memory_id": row.SupersedesID,
		"created_at": row.CreatedAt, "updated_at": row.UpdatedAt, "closed_at": row.ClosedAt,
	}
}

func readActiveMemoryAuthorityRowTx(ctx context.Context, tx pgx.Tx, activeMemoryID string) (activeMemoryAuthorityRow, error) {
	return scanActiveMemoryAuthorityRow(tx.QueryRow(ctx, `SELECT id,owner_fluctlight_id,COALESCE(conversation_id,''),kind,content,status,confidence,importance,actor_refs,source_fact_id,evidence_refs,COALESCE(original_time_expression,''),valid_from,valid_until,time_precision,timezone,last_relevant_at,revision,canonical_key,request_digest,COALESCE(superseded_by_active_memory_id,''),COALESCE(supersedes_active_memory_id,''),created_at,updated_at,closed_at FROM public.active_memories WHERE id=$1 FOR UPDATE`, activeMemoryID))
}

type activeMemoryRowScanner interface {
	Scan(...any) error
}

func scanActiveMemoryAuthorityRow(scanner activeMemoryRowScanner) (activeMemoryAuthorityRow, error) {
	var row activeMemoryAuthorityRow
	var actorRefs, evidenceRefs []byte
	if err := scanner.Scan(&row.ID, &row.OwnerFluctlightID, &row.ConversationID, &row.Kind, &row.Content, &row.Status, &row.Confidence, &row.Importance, &actorRefs, &row.SourceFactID, &evidenceRefs, &row.OriginalTimeExpression, &row.ValidFrom, &row.ValidUntil, &row.TimePrecision, &row.Timezone, &row.LastRelevantAt, &row.Revision, &row.CanonicalKey, &row.RequestDigest, &row.SupersededByID, &row.SupersedesID, &row.CreatedAt, &row.UpdatedAt, &row.ClosedAt); err != nil {
		return activeMemoryAuthorityRow{}, err
	}
	if err := json.Unmarshal(actorRefs, &row.ActorRefs); err != nil {
		return activeMemoryAuthorityRow{}, err
	}
	if err := json.Unmarshal(evidenceRefs, &row.EvidenceRefs); err != nil {
		return activeMemoryAuthorityRow{}, err
	}
	return row, nil
}

func timePointer(value time.Time) *time.Time {
	value = value.UTC()
	return &value
}
