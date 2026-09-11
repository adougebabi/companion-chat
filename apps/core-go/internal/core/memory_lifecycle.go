package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	memoryLifecycleSchemaVersion = "memory.lifecycle.v2"
	memoryLifecyclePolicyVersion = "memory.lifecycle.policy.v1"
)

type MemoryOperation string

const (
	MemoryCreate    MemoryOperation = "create"
	MemoryConfirm   MemoryOperation = "confirm"
	MemoryRevise    MemoryOperation = "revise"
	MemoryMerge     MemoryOperation = "merge"
	MemorySupersede MemoryOperation = "supersede"
	MemoryDeprecate MemoryOperation = "deprecate"
	MemoryForget    MemoryOperation = "forget"
	MemoryRollback  MemoryOperation = "rollback"
)

type MemorySemanticInput struct {
	Type                  string  `json:"type"`
	Content               string  `json:"content"`
	Confidence            float64 `json:"confidence"`
	Importance            float64 `json:"importance"`
	EmotionalSignificance float64 `json:"emotional_significance"`
}

type MemoryTarget struct {
	Ref              string `json:"ref,omitempty"`
	MemoryID         string `json:"memory_id"`
	ExpectedRevision int    `json:"expected_revision"`
}

type MemoryRollbackCompensation struct {
	Target           MemoryTarget   `json:"target"`
	RestoreRevision  *int           `json:"restore_revision,omitempty"`
	RestoreSnapshot  map[string]any `json:"restore_snapshot,omitempty"`
	DeprecateCreated bool           `json:"deprecate_created,omitempty"`
}

type PreparedMemoryMutation struct {
	SchemaVersion           string                       `json:"schema_version"`
	Operation               MemoryOperation              `json:"operation"`
	OwnerFluctlightID       string                       `json:"owner_fluctlight_id"`
	OwnerActorID            string                       `json:"owner_actor_id"`
	ActorID                 string                       `json:"actor_id"`
	ActiveProfileID         string                       `json:"active_profile_id,omitempty"`
	ConversationID          string                       `json:"conversation_id,omitempty"`
	ActorRefs               []string                     `json:"actor_refs"`
	EventRefs               []string                     `json:"event_refs"`
	EvidenceRefs            []string                     `json:"evidence_refs"`
	PersonalityPerspectives []any                        `json:"personality_perspectives,omitempty"`
	Target                  *MemoryTarget                `json:"target,omitempty"`
	MergeTargets            []MemoryTarget               `json:"merge_targets,omitempty"`
	Semantic                *MemorySemanticInput         `json:"semantic,omitempty"`
	RollbackRevision        *int                         `json:"rollback_revision,omitempty"`
	RollbackSnapshot        map[string]any               `json:"rollback_snapshot,omitempty"`
	RollbackCompensations   []MemoryRollbackCompensation `json:"rollback_compensations,omitempty"`
	Visibility              string                       `json:"visibility"`
	OccurredAt              time.Time                    `json:"occurred_at"`
	SourceFactID            string                       `json:"source_fact_id"`
	SourceWindow            string                       `json:"source_window,omitempty"`
	ProposalID              string                       `json:"proposal_id,omitempty"`
	CandidateIndex          int                          `json:"candidate_index"`
	SemanticReason          string                       `json:"semantic_reason"`
	IdempotencyKey          string                       `json:"idempotency_key"`
	RequestDigest           string                       `json:"request_digest"`
}

type MemoryApplyResult struct {
	Operation          MemoryOperation `json:"operation"`
	MemoryID           string          `json:"memory_id"`
	Status             string          `json:"status"`
	Revision           int             `json:"revision"`
	Disposition        string          `json:"disposition"`
	ReasonCode         string          `json:"reason_code"`
	RelatedMemoryIDs   []string        `json:"related_memory_ids"`
	ResultingRevisions map[string]int  `json:"resulting_revisions"`
	Replayed           bool            `json:"replayed"`
}

type memoryAuthorityRow struct {
	ID                      string
	OwnerFluctlightID       string
	Type                    string
	Content                 string
	ActorRefs               []any
	ConversationID          string
	EventRefs               []any
	EvidenceRefs            []any
	PersonalityPerspectives []any
	Confidence              float64
	Importance              float64
	EmotionalSignificance   float64
	Visibility              string
	Status                  string
	Revision                int
	CanonicalKey            string
	RequestDigest           string
	SupersededBy            string
	Supersedes              string
	OccurredAt              *time.Time
	CreatedAt               time.Time
	LastConfirmedAt         *time.Time
}

func memoryCanonicalKey(semantic MemorySemanticInput, conversationID, visibility string, actorRefs, eventRefs []string) string {
	return stableDigest(strings.Join([]string{
		semantic.Type, strings.TrimSpace(semantic.Content), strings.TrimSpace(conversationID), strings.TrimSpace(visibility),
		strings.Join(sortedUniqueStrings(actorRefs), "\x1e"), strings.Join(sortedUniqueStrings(eventRefs), "\x1e"),
	}, "\x1f"))
}

func memorySnapshot(row memoryAuthorityRow) map[string]any {
	result := map[string]any{
		"id": row.ID, "owner_fluctlight_id": row.OwnerFluctlightID, "type": row.Type, "content": row.Content,
		"actor_refs": row.ActorRefs, "event_refs": row.EventRefs, "evidence_refs": row.EvidenceRefs,
		"personality_perspectives": row.PersonalityPerspectives, "confidence": row.Confidence,
		"importance": row.Importance, "emotional_significance": row.EmotionalSignificance,
		"visibility": row.Visibility, "status": row.Status, "revision": row.Revision,
		"canonical_key": row.CanonicalKey, "request_digest": row.RequestDigest,
		"created_at": row.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if row.ConversationID != "" {
		result["conversation_id"] = row.ConversationID
	}
	if row.SupersededBy != "" {
		result["superseded_by_memory_id"] = row.SupersededBy
	}
	if row.Supersedes != "" {
		result["supersedes_memory_id"] = row.Supersedes
	}
	if row.OccurredAt != nil {
		result["occurred_at"] = row.OccurredAt.UTC().Format(time.RFC3339Nano)
	}
	if row.LastConfirmedAt != nil {
		result["last_confirmed_at"] = row.LastConfirmedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func memorySnapshotDigest(row memoryAuthorityRow) string {
	snapshot := memorySnapshot(row)
	delete(snapshot, "request_digest")
	return stableDigest(jsonString(snapshot))
}

func memoryCommandDigest(command PreparedMemoryMutation) string {
	copyValue := command
	copyValue.RequestDigest = ""
	// Wall time is audit metadata, not semantic command identity. Excluding it
	// lets an owner/workflow retry replay an already committed governance result
	// instead of conflicting merely because the retry happened later.
	copyValue.OccurredAt = time.Time{}
	return stableDigest(jsonString(copyValue))
}

func validatePreparedMemoryMutation(command PreparedMemoryMutation) error {
	if command.SchemaVersion != memoryLifecycleSchemaVersion || strings.TrimSpace(command.OwnerFluctlightID) == "" || strings.TrimSpace(command.OwnerActorID) == "" || strings.TrimSpace(command.ActorID) == "" || strings.TrimSpace(command.SourceFactID) == "" {
		return errors.New("memory_command_identity_invalid")
	}
	if command.IdempotencyKey == "" || len([]rune(command.IdempotencyKey)) > 256 || command.RequestDigest == "" || command.RequestDigest != memoryCommandDigest(command) {
		return errors.New("memory_command_idempotency_invalid")
	}
	if command.Visibility != "private" && command.Visibility != "owner" && command.Visibility != "participants" {
		return errors.New("memory_command_visibility_invalid")
	}
	if command.OccurredAt.IsZero() || len(command.EvidenceRefs) == 0 || len(command.EvidenceRefs) > 64 || strings.TrimSpace(command.SemanticReason) == "" || len([]rune(command.SemanticReason)) > 1000 {
		return errors.New("memory_command_evidence_invalid")
	}
	for _, ref := range command.EvidenceRefs {
		if strings.TrimSpace(ref) == "" || len([]rune(ref)) > 256 {
			return errors.New("memory_command_evidence_invalid")
		}
	}
	needsSemantic := command.Operation == MemoryCreate || command.Operation == MemoryRevise || command.Operation == MemoryMerge || command.Operation == MemorySupersede || command.Operation == MemoryRollback
	if needsSemantic {
		if command.Semantic == nil {
			return errors.New("memory_command_semantic_required")
		}
		if _, ok := validMemoryTypes[command.Semantic.Type]; !ok || strings.TrimSpace(command.Semantic.Content) == "" || len([]rune(command.Semantic.Content)) > 32000 || command.Semantic.Confidence < 0 || command.Semantic.Confidence > 1 || command.Semantic.Importance < 0 || command.Semantic.Importance > 1 || command.Semantic.EmotionalSignificance < 0 || command.Semantic.EmotionalSignificance > 1 {
			return errors.New("memory_command_semantic_invalid")
		}
	} else if command.Semantic != nil {
		return errors.New("memory_command_semantic_forbidden")
	}
	switch command.Operation {
	case MemoryCreate:
		if command.Target != nil || len(command.MergeTargets) > 0 {
			return errors.New("memory_create_target_forbidden")
		}
	case MemoryConfirm, MemoryRevise, MemorySupersede, MemoryDeprecate, MemoryForget, MemoryRollback:
		if command.Target == nil || command.Target.MemoryID == "" || command.Target.ExpectedRevision < 0 || len(command.MergeTargets) > 0 || (command.ProposalID != "" && strings.TrimSpace(command.Target.Ref) == "") {
			return errors.New("memory_command_target_invalid")
		}
	case MemoryMerge:
		if command.Target == nil || command.Target.MemoryID == "" || command.Target.ExpectedRevision < 0 || len(command.MergeTargets) == 0 || (command.ProposalID != "" && strings.TrimSpace(command.Target.Ref) == "") {
			return errors.New("memory_merge_targets_invalid")
		}
	default:
		return errors.New("memory_command_operation_invalid")
	}
	if command.Operation == MemoryRollback {
		if command.RollbackRevision == nil || *command.RollbackRevision < 0 || len(command.RollbackSnapshot) == 0 || len(jsonBytes(command.RollbackSnapshot)) > 64*1024 {
			return errors.New("memory_rollback_snapshot_invalid")
		}
		for _, compensation := range command.RollbackCompensations {
			if compensation.Target.MemoryID == "" || compensation.Target.ExpectedRevision < 0 {
				return errors.New("memory_rollback_compensation_invalid")
			}
			hasRestore := compensation.RestoreRevision != nil && *compensation.RestoreRevision >= 0 && len(compensation.RestoreSnapshot) > 0
			if hasRestore == compensation.DeprecateCreated || len(jsonBytes(compensation.RestoreSnapshot)) > 64*1024 {
				return errors.New("memory_rollback_compensation_invalid")
			}
		}
	} else if command.RollbackRevision != nil || len(command.RollbackSnapshot) > 0 || len(command.RollbackCompensations) > 0 {
		return errors.New("memory_rollback_snapshot_forbidden")
	}
	seen := map[string]struct{}{}
	if command.Target != nil {
		seen[command.Target.MemoryID] = struct{}{}
	}
	for _, target := range command.MergeTargets {
		if target.MemoryID == "" || target.ExpectedRevision < 0 || (command.ProposalID != "" && strings.TrimSpace(target.Ref) == "") {
			return errors.New("memory_merge_targets_invalid")
		}
		if _, duplicate := seen[target.MemoryID]; duplicate {
			return errors.New("memory_merge_target_duplicate")
		}
		seen[target.MemoryID] = struct{}{}
	}
	for _, compensation := range command.RollbackCompensations {
		if _, duplicate := seen[compensation.Target.MemoryID]; duplicate {
			return errors.New("memory_rollback_compensation_duplicate")
		}
		seen[compensation.Target.MemoryID] = struct{}{}
	}
	return nil
}

func (a *App) applyMemoryCommandTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation) (MemoryApplyResult, error) {
	if err := validatePreparedMemoryMutation(command); err != nil {
		return MemoryApplyResult{}, err
	}
	var liveOwner, fluctlightStatus string
	if err := tx.QueryRow(ctx, `SELECT created_by_actor_id,status FROM public.fluctlights WHERE id=$1 FOR UPDATE`, command.OwnerFluctlightID).Scan(&liveOwner, &fluctlightStatus); err != nil {
		return MemoryApplyResult{}, err
	}
	if liveOwner != command.OwnerActorID || fluctlightStatus != "active" {
		return MemoryApplyResult{}, errors.New("memory_command_owner_stale")
	}
	var existingDigest string
	var existingResult []byte
	if err := tx.QueryRow(ctx, `SELECT request_digest,result FROM public.memory_governance WHERE idempotency_key=$1`, command.IdempotencyKey).Scan(&existingDigest, &existingResult); err == nil {
		if existingDigest != command.RequestDigest {
			return MemoryApplyResult{}, errors.New("memory_command_idempotency_conflict")
		}
		var result MemoryApplyResult
		if jsonUnmarshal(existingResult, &result) != nil {
			return MemoryApplyResult{}, errors.New("memory_governance_result_invalid")
		}
		result.Replayed = true
		return result, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return MemoryApplyResult{}, err
	}
	if command.Operation == MemoryCreate {
		return a.applyMemoryCreateTx(ctx, tx, command, "", true)
	}
	targets := append([]MemoryTarget{*command.Target}, command.MergeTargets...)
	for _, compensation := range command.RollbackCompensations {
		targets = append(targets, compensation.Target)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].MemoryID < targets[j].MemoryID })
	rows := make(map[string]memoryAuthorityRow, len(targets))
	for _, target := range targets {
		row, err := readMemoryAuthorityRowTx(ctx, tx, target.MemoryID)
		if err != nil {
			return MemoryApplyResult{}, err
		}
		statusValid := row.Status == "active"
		if command.Operation == MemoryRollback {
			_, statusValid = map[string]struct{}{"active": {}, "superseded": {}, "deprecated": {}, "forgotten": {}}[row.Status]
		}
		if row.OwnerFluctlightID != command.OwnerFluctlightID || row.Revision != target.ExpectedRevision || !statusValid {
			return MemoryApplyResult{}, errors.New("memory_target_revision_conflict")
		}
		rows[target.MemoryID] = row
	}
	primary := rows[command.Target.MemoryID]
	if command.Operation == MemoryMerge {
		for _, target := range command.MergeTargets {
			candidate := rows[target.MemoryID]
			if !sameMemoryAuthorityScope(primary, candidate) {
				return MemoryApplyResult{}, errors.New("memory_merge_scope_conflict")
			}
		}
	}
	switch command.Operation {
	case MemoryConfirm:
		return a.applyMemoryConfirmTx(ctx, tx, command, primary)
	case MemoryRevise:
		return a.applyMemoryReviseTx(ctx, tx, command, primary, nil)
	case MemoryMerge:
		return a.applyMemoryMergeTx(ctx, tx, command, primary, rows)
	case MemorySupersede:
		return a.applyMemorySupersedeTx(ctx, tx, command, primary)
	case MemoryDeprecate:
		return a.applyMemoryDeprecateTx(ctx, tx, command, primary)
	case MemoryForget:
		return a.applyMemoryForgetTx(ctx, tx, command, primary)
	case MemoryRollback:
		return a.applyMemoryRollbackTx(ctx, tx, command, primary, rows)
	default:
		return MemoryApplyResult{}, errors.New("memory_command_operation_invalid")
	}
}

func sameMemoryAuthorityScope(left, right memoryAuthorityRow) bool {
	return left.Visibility == right.Visibility && left.ConversationID == right.ConversationID &&
		jsonString(sortedUniqueStrings(decisionServiceRefValues(left.ActorRefs))) == jsonString(sortedUniqueStrings(decisionServiceRefValues(right.ActorRefs))) &&
		jsonString(sortedUniqueStrings(decisionServiceRefValues(left.EventRefs))) == jsonString(sortedUniqueStrings(decisionServiceRefValues(right.EventRefs)))
}

func readMemoryAuthorityRowTx(ctx context.Context, tx pgx.Tx, memoryID string) (memoryAuthorityRow, error) {
	return scanMemoryAuthorityRow(tx.QueryRow(ctx, `SELECT id,owner_fluctlight_id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,canonical_key,request_digest,superseded_by_memory_id,supersedes_memory_id,occurred_at,created_at,last_confirmed_at FROM public.memories WHERE id=$1 FOR UPDATE`, memoryID))
}

func (a *App) readOwnedMemoryAuthorityRow(ctx context.Context, actorID, memoryID string) (memoryAuthorityRow, error) {
	row, err := scanMemoryAuthorityRow(a.DB.Pool().QueryRow(ctx, `SELECT m.id,m.owner_fluctlight_id,m.type,m.content,m.actor_refs,m.conversation_id,m.event_refs,m.evidence_refs,m.personality_perspectives,m.confidence,m.importance,m.emotional_significance,m.visibility,m.status,m.revision,m.canonical_key,m.request_digest,m.superseded_by_memory_id,m.supersedes_memory_id,m.occurred_at,m.created_at,m.last_confirmed_at FROM public.memories m JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id WHERE m.id=$1 AND f.created_by_actor_id=$2`, memoryID, actorID))
	if errors.Is(err, pgx.ErrNoRows) {
		return memoryAuthorityRow{}, ErrNotFound
	}
	return row, err
}

func (a *App) readOwnedMemoryRevisionSnapshot(ctx context.Context, actorID, memoryID string, revision int) (memoryAuthorityRow, map[string]any, error) {
	var raw []byte
	var schemaVersion string
	err := a.DB.Pool().QueryRow(ctx, `SELECT r.snapshot,r.schema_version FROM public.memory_revisions r JOIN public.memories m ON m.id=r.memory_id JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id WHERE r.memory_id=$1 AND r.revision=$2 AND f.created_by_actor_id=$3`, memoryID, revision, actorID).Scan(&raw, &schemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return memoryAuthorityRow{}, nil, ErrNotFound
	}
	if err != nil {
		return memoryAuthorityRow{}, nil, err
	}
	snapshot := decodeObject(raw)
	if schemaVersion != memoryLifecycleSchemaVersion {
		return memoryAuthorityRow{}, nil, errors.New("memory_revision_snapshot_unavailable")
	}
	row, err := memoryAuthorityRowFromSnapshot(snapshot)
	if err != nil || row.ID != memoryID || row.Revision != revision {
		return memoryAuthorityRow{}, nil, errors.New("memory_revision_snapshot_invalid")
	}
	return row, snapshot, nil
}

func memoryAuthorityRowFromSnapshot(snapshot map[string]any) (memoryAuthorityRow, error) {
	row := memoryAuthorityRow{
		ID: stringValue(snapshot["id"]), OwnerFluctlightID: stringValue(snapshot["owner_fluctlight_id"]),
		Type: stringValue(snapshot["type"]), Content: stringValue(snapshot["content"]),
		ActorRefs: arrayValue(snapshot["actor_refs"]), EventRefs: arrayValue(snapshot["event_refs"]), EvidenceRefs: arrayValue(snapshot["evidence_refs"]),
		PersonalityPerspectives: arrayValue(snapshot["personality_perspectives"]), ConversationID: stringValue(snapshot["conversation_id"]),
		Visibility: stringValue(snapshot["visibility"]), Status: stringValue(snapshot["status"]), Revision: intValue(snapshot["revision"]),
		CanonicalKey: stringValue(snapshot["canonical_key"]), RequestDigest: stringValue(snapshot["request_digest"]),
		SupersededBy: stringValue(snapshot["superseded_by_memory_id"]), Supersedes: stringValue(snapshot["supersedes_memory_id"]),
	}
	var err error
	if row.Confidence, err = requiredBoundedNumber(snapshot["confidence"]); err != nil {
		return memoryAuthorityRow{}, err
	}
	if row.Importance, err = requiredBoundedNumber(snapshot["importance"]); err != nil {
		return memoryAuthorityRow{}, err
	}
	if row.EmotionalSignificance, err = requiredBoundedNumber(snapshot["emotional_significance"]); err != nil {
		return memoryAuthorityRow{}, err
	}
	if row.ID == "" || row.OwnerFluctlightID == "" || row.Content == "" || row.Visibility == "" || row.Status == "" || row.Revision < 0 {
		return memoryAuthorityRow{}, errors.New("memory_revision_snapshot_invalid")
	}
	return row, nil
}

func scanMemoryAuthorityRow(source pgx.Row) (memoryAuthorityRow, error) {
	var row memoryAuthorityRow
	var actorRefs, eventRefs, evidenceRefs, perspectives []byte
	var conversationID, supersededBy, supersedes *string
	err := source.Scan(
		&row.ID, &row.OwnerFluctlightID, &row.Type, &row.Content, &actorRefs, &conversationID, &eventRefs, &evidenceRefs, &perspectives,
		&row.Confidence, &row.Importance, &row.EmotionalSignificance, &row.Visibility, &row.Status, &row.Revision,
		&row.CanonicalKey, &row.RequestDigest, &supersededBy, &supersedes, &row.OccurredAt, &row.CreatedAt, &row.LastConfirmedAt,
	)
	if err != nil {
		return memoryAuthorityRow{}, err
	}
	row.ActorRefs, row.EventRefs, row.EvidenceRefs, row.PersonalityPerspectives = decodeArray(actorRefs), decodeArray(eventRefs), decodeArray(evidenceRefs), decodeArray(perspectives)
	if conversationID != nil {
		row.ConversationID = *conversationID
	}
	if supersededBy != nil {
		row.SupersededBy = *supersededBy
	}
	if supersedes != nil {
		row.Supersedes = *supersedes
	}
	return row, nil
}

func ownerMemoryEvidence(values []any) ([]string, error) {
	refs := sortedUniqueStrings(decisionServiceRefValues(values))
	if len(refs) == 0 || len(refs) > 64 {
		return nil, errors.New("memory_evidence_required")
	}
	for _, ref := range refs {
		if len([]rune(ref)) > 256 {
			return nil, errors.New("memory_evidence_invalid")
		}
	}
	return refs, nil
}

func buildOwnerMemoryCommand(row memoryAuthorityRow, actorID string, operation MemoryOperation, expectedRevision int, evidenceRefs []string, semantic *MemorySemanticInput, semanticReason string, rollbackRevision *int, rollbackSnapshot map[string]any) PreparedMemoryMutation {
	commandIdentity := map[string]any{
		"operation": operation, "memory_id": row.ID, "expected_revision": expectedRevision,
		"evidence_refs": evidenceRefs, "semantic": semantic, "rollback_revision": rollbackRevision,
	}
	command := PreparedMemoryMutation{
		SchemaVersion: memoryLifecycleSchemaVersion, Operation: operation,
		OwnerFluctlightID: row.OwnerFluctlightID, OwnerActorID: actorID, ActorID: actorID,
		ConversationID: row.ConversationID, ActorRefs: decisionServiceRefValues(row.ActorRefs), EventRefs: decisionServiceRefValues(row.EventRefs),
		EvidenceRefs: evidenceRefs, PersonalityPerspectives: append([]any(nil), row.PersonalityPerspectives...),
		Target: &MemoryTarget{MemoryID: row.ID, ExpectedRevision: expectedRevision}, Semantic: semantic,
		RollbackRevision: rollbackRevision, RollbackSnapshot: rollbackSnapshot,
		Visibility: row.Visibility, OccurredAt: time.Now().UTC(), SourceFactID: evidenceRefs[0],
		CandidateIndex: -1, SemanticReason: semanticReason,
		IdempotencyKey: "memory:owner:" + stableDigest(jsonString(commandIdentity)),
	}
	if operation == MemoryRollback {
		command.ConversationID = stringValue(rollbackSnapshot["conversation_id"])
		command.ActorRefs = decisionServiceRefValues(rollbackSnapshot["actor_refs"])
		command.EventRefs = decisionServiceRefValues(rollbackSnapshot["event_refs"])
		command.PersonalityPerspectives = arrayValue(rollbackSnapshot["personality_perspectives"])
		command.Visibility = stringValue(rollbackSnapshot["visibility"])
	}
	command.RequestDigest = memoryCommandDigest(command)
	return command
}

func (a *App) applyOwnerMemoryCommand(ctx context.Context, command PreparedMemoryMutation) (MemoryApplyResult, error) {
	var result MemoryApplyResult
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var err error
		result, err = a.applyMemoryCommandTx(ctx, tx, command)
		return err
	})
	return result, err
}

func (a *App) readMemoryGovernanceReplay(ctx context.Context, idempotencyKey string) (MemoryApplyResult, bool, error) {
	var raw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT result FROM public.memory_governance WHERE idempotency_key=$1`, idempotencyKey).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MemoryApplyResult{}, false, nil
		}
		return MemoryApplyResult{}, false, err
	}
	var result MemoryApplyResult
	if err := jsonUnmarshal(raw, &result); err != nil {
		return MemoryApplyResult{}, false, errors.New("memory_governance_result_invalid")
	}
	result.Replayed = true
	return result, true, nil
}

func (a *App) buildOwnerRollbackCompensations(ctx context.Context, actorID, memoryID string, targetRevision, expectedRevision int) ([]MemoryRollbackCompensation, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT proposal_id,related_memory_ids FROM public.memory_revisions WHERE memory_id=$1 AND revision>$2 AND revision<=$3 AND operation IN ('merge','supersede') ORDER BY revision`, memoryID, targetRevision, expectedRevision)
	if err != nil {
		return nil, err
	}
	type relatedOperation struct {
		memoryID   string
		proposalID string
	}
	operations := make([]relatedOperation, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var proposalID *string
		var rawRelated []byte
		if err := rows.Scan(&proposalID, &rawRelated); err != nil {
			rows.Close()
			return nil, err
		}
		if proposalID == nil || strings.TrimSpace(*proposalID) == "" {
			rows.Close()
			return nil, errors.New("memory_rollback_lineage_unavailable")
		}
		for _, relatedID := range decisionServiceRefValues(decodeArray(rawRelated)) {
			if relatedID == "" || relatedID == memoryID {
				continue
			}
			if _, duplicate := seen[relatedID]; duplicate {
				rows.Close()
				return nil, errors.New("memory_rollback_lineage_complex")
			}
			seen[relatedID] = struct{}{}
			operations = append(operations, relatedOperation{memoryID: relatedID, proposalID: *proposalID})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := make([]MemoryRollbackCompensation, 0, len(operations))
	for _, operation := range operations {
		current, err := a.readOwnedMemoryAuthorityRow(ctx, actorID, operation.memoryID)
		if err != nil {
			return nil, err
		}
		var revision, baseRevision int
		var revisionOperation, schemaVersion string
		var rawSnapshot []byte
		err = a.DB.Pool().QueryRow(ctx, `SELECT revision,base_revision,operation,snapshot,schema_version FROM public.memory_revisions WHERE memory_id=$1 AND proposal_id=$2 ORDER BY revision LIMIT 1`, operation.memoryID, operation.proposalID).Scan(&revision, &baseRevision, &revisionOperation, &rawSnapshot, &schemaVersion)
		if err != nil {
			return nil, err
		}
		if current.Revision != revision || schemaVersion != memoryLifecycleSchemaVersion {
			return nil, errors.New("memory_rollback_related_revision_conflict")
		}
		compensation := MemoryRollbackCompensation{Target: MemoryTarget{MemoryID: current.ID, ExpectedRevision: current.Revision}}
		if revisionOperation == string(MemorySupersede) && revision == 0 && current.Supersedes == memoryID {
			compensation.DeprecateCreated = true
		} else {
			_, snapshot, err := a.readOwnedMemoryRevisionSnapshot(ctx, actorID, current.ID, baseRevision)
			if err != nil {
				return nil, err
			}
			compensation.RestoreRevision = &baseRevision
			compensation.RestoreSnapshot = snapshot
		}
		result = append(result, compensation)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Target.MemoryID < result[j].Target.MemoryID })
	return result, nil
}

func memoryApplyResultMap(result MemoryApplyResult) map[string]any {
	return map[string]any{
		"id": result.MemoryID, "operation": string(result.Operation), "status": result.Status,
		"revision": result.Revision, "disposition": result.Disposition, "reason_code": result.ReasonCode,
		"replayed": result.Replayed,
	}
}

func memoryRowFromCommand(command PreparedMemoryMutation, memoryID string, semantic MemorySemanticInput, status string, revision int) memoryAuthorityRow {
	createdAt := command.OccurredAt.UTC()
	perspectives := make([]any, len(command.PersonalityPerspectives))
	copy(perspectives, command.PersonalityPerspectives)
	row := memoryAuthorityRow{
		ID: memoryID, OwnerFluctlightID: command.OwnerFluctlightID, Type: semantic.Type,
		Content: strings.TrimSpace(semantic.Content), ActorRefs: stringSliceAny(command.ActorRefs), ConversationID: command.ConversationID,
		EventRefs: stringSliceAny(command.EventRefs), EvidenceRefs: stringSliceAny(sortedUniqueStrings(command.EvidenceRefs)),
		PersonalityPerspectives: perspectives, Confidence: semantic.Confidence, Importance: semantic.Importance,
		EmotionalSignificance: semantic.EmotionalSignificance, Visibility: command.Visibility, Status: status,
		Revision: revision, OccurredAt: &createdAt, CreatedAt: createdAt,
	}
	row.CanonicalKey = memoryCanonicalKey(semantic, row.ConversationID, row.Visibility, decisionServiceRefValues(row.ActorRefs), decisionServiceRefValues(row.EventRefs))
	row.RequestDigest = memorySnapshotDigest(row)
	return row
}

func (a *App) applyMemoryCreateTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, supersedes string, govern bool) (MemoryApplyResult, error) {
	semantic := *command.Semantic
	canonicalKey := memoryCanonicalKey(semantic, command.ConversationID, command.Visibility, command.ActorRefs, command.EventRefs)
	var existingID, existingStatus string
	var existingRevision int
	if err := tx.QueryRow(ctx, `SELECT id,status,revision FROM public.memories WHERE owner_fluctlight_id=$1 AND canonical_key=$2 AND status='active' FOR UPDATE`, command.OwnerFluctlightID, canonicalKey).Scan(&existingID, &existingStatus, &existingRevision); err == nil {
		result := MemoryApplyResult{Operation: command.Operation, MemoryID: existingID, Status: existingStatus, Revision: existingRevision, Disposition: "no_change", ReasonCode: "exact_duplicate", ResultingRevisions: map[string]int{existingID: existingRevision}}
		if govern {
			if err := persistMemoryGovernanceTx(ctx, tx, command, result, nil); err != nil {
				return MemoryApplyResult{}, err
			}
		}
		return result, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return MemoryApplyResult{}, err
	}
	memoryID := "memory_" + stableDigest(command.OwnerFluctlightID+":"+command.IdempotencyKey)
	row := memoryRowFromCommand(command, memoryID, semantic, "active", 0)
	row.Supersedes = supersedes
	if _, err := tx.Exec(ctx, `INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,canonical_key,request_digest,supersedes_memory_id,occurred_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'active',0,$14,$15,$16,$17,$17,$17)`, row.ID, row.OwnerFluctlightID, row.Type, row.Content, jsonBytes(row.ActorRefs), nullableString(row.ConversationID), jsonBytes(row.EventRefs), jsonBytes(row.EvidenceRefs), jsonBytes(row.PersonalityPerspectives), row.Confidence, row.Importance, row.EmotionalSignificance, row.Visibility, row.CanonicalKey, row.RequestDigest, nullableString(row.Supersedes), row.CreatedAt); err != nil {
		return MemoryApplyResult{}, fmt.Errorf("insert lifecycle Memory: %w", err)
	}
	if err := refreshMemorySearchDocumentTx(ctx, tx, row.ID); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := writeMemoryRevisionTx(ctx, tx, command, row, 0, nil); err != nil {
		return MemoryApplyResult{}, err
	}
	result := MemoryApplyResult{Operation: command.Operation, MemoryID: row.ID, Status: row.Status, Revision: 0, Disposition: "applied", ReasonCode: "created", ResultingRevisions: map[string]int{row.ID: 0}}
	if govern {
		if err := persistMemoryGovernanceTx(ctx, tx, command, result, nil); err != nil {
			return MemoryApplyResult{}, err
		}
	}
	if err := appendOutboxTx(ctx, tx, "memory.created", "memory", row.ID, row.OwnerFluctlightID, command.SourceFactID, "memory:"+row.ID, "memory-created:"+row.ID, map[string]any{"memory_id": row.ID, "revision": 0, "aggregate_sequence": 1}); err != nil {
		return MemoryApplyResult{}, err
	}
	return result, a.enqueueMemoryEmbeddingTx(ctx, tx, row, command)
}

func (a *App) applyMemoryConfirmTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, row memoryAuthorityRow) (MemoryApplyResult, error) {
	row.Revision++
	row.EvidenceRefs = mergeStringAny(row.EvidenceRefs, command.EvidenceRefs)
	now := command.OccurredAt.UTC()
	row.LastConfirmedAt = &now
	row.RequestDigest = memorySnapshotDigest(row)
	updated, err := tx.Exec(ctx, `UPDATE public.memories SET evidence_refs=$2,revision=$3,request_digest=$4,last_confirmed_at=$5,updated_at=$5 WHERE id=$1 AND revision=$6 AND status='active'`, row.ID, jsonBytes(row.EvidenceRefs), row.Revision, row.RequestDigest, now, row.Revision-1)
	if err != nil {
		return MemoryApplyResult{}, err
	}
	if updated.RowsAffected() != 1 {
		return MemoryApplyResult{}, errors.New("memory_target_revision_conflict")
	}
	if err := writeMemoryRevisionTx(ctx, tx, command, row, row.Revision-1, nil); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := staleMemoryEmbeddingsTx(ctx, tx, row.ID); err != nil {
		return MemoryApplyResult{}, err
	}
	result := MemoryApplyResult{Operation: command.Operation, MemoryID: row.ID, Status: row.Status, Revision: row.Revision, Disposition: "applied", ReasonCode: "confirmed", ResultingRevisions: map[string]int{row.ID: row.Revision}}
	if err := persistMemoryGovernanceTx(ctx, tx, command, result, map[string]int{row.ID: row.Revision - 1}); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := appendMemoryLifecycleOutboxTx(ctx, tx, command, result); err != nil {
		return MemoryApplyResult{}, err
	}
	return result, a.enqueueMemoryEmbeddingTx(ctx, tx, row, command)
}

func (a *App) applyMemoryReviseTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, row memoryAuthorityRow, related []string) (MemoryApplyResult, error) {
	base := row.Revision
	semantic := *command.Semantic
	row.Type, row.Content = semantic.Type, strings.TrimSpace(semantic.Content)
	row.Confidence, row.Importance, row.EmotionalSignificance = semantic.Confidence, semantic.Importance, semantic.EmotionalSignificance
	row.EvidenceRefs = mergeStringAny(row.EvidenceRefs, command.EvidenceRefs)
	row.Revision++
	row.CanonicalKey = memoryCanonicalKey(semantic, row.ConversationID, row.Visibility, decisionServiceRefValues(row.ActorRefs), decisionServiceRefValues(row.EventRefs))
	row.RequestDigest = memorySnapshotDigest(row)
	updated, err := tx.Exec(ctx, `UPDATE public.memories SET type=$2,content=$3,confidence=$4,importance=$5,emotional_significance=$6,evidence_refs=$7,revision=$8,canonical_key=$9,request_digest=$10,updated_at=$11 WHERE id=$1 AND revision=$12 AND status='active'`, row.ID, row.Type, row.Content, row.Confidence, row.Importance, row.EmotionalSignificance, jsonBytes(row.EvidenceRefs), row.Revision, row.CanonicalKey, row.RequestDigest, command.OccurredAt.UTC(), base)
	if err != nil {
		return MemoryApplyResult{}, err
	}
	if updated.RowsAffected() != 1 {
		return MemoryApplyResult{}, errors.New("memory_target_revision_conflict")
	}
	if err := refreshMemorySearchDocumentTx(ctx, tx, row.ID); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := writeMemoryRevisionTx(ctx, tx, command, row, base, related); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := staleMemoryEmbeddingsTx(ctx, tx, row.ID); err != nil {
		return MemoryApplyResult{}, err
	}
	result := MemoryApplyResult{Operation: command.Operation, MemoryID: row.ID, Status: row.Status, Revision: row.Revision, Disposition: "applied", ReasonCode: "revised", RelatedMemoryIDs: related, ResultingRevisions: map[string]int{row.ID: row.Revision}}
	if err := persistMemoryGovernanceTx(ctx, tx, command, result, map[string]int{row.ID: base}); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := appendMemoryLifecycleOutboxTx(ctx, tx, command, result); err != nil {
		return MemoryApplyResult{}, err
	}
	return result, a.enqueueMemoryEmbeddingTx(ctx, tx, row, command)
}

func (a *App) applyMemoryMergeTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, primary memoryAuthorityRow, rows map[string]memoryAuthorityRow) (MemoryApplyResult, error) {
	related := make([]string, 0, len(command.MergeTargets))
	baseRevisions := map[string]int{primary.ID: primary.Revision}
	for _, target := range command.MergeTargets {
		related = append(related, target.MemoryID)
		baseRevisions[target.MemoryID] = rows[target.MemoryID].Revision
	}
	sort.Strings(related)
	base := primary.Revision
	semantic := *command.Semantic
	primary.Type, primary.Content = semantic.Type, strings.TrimSpace(semantic.Content)
	primary.Confidence, primary.Importance, primary.EmotionalSignificance = semantic.Confidence, semantic.Importance, semantic.EmotionalSignificance
	primary.EvidenceRefs = mergeStringAny(primary.EvidenceRefs, command.EvidenceRefs)
	primary.Revision++
	primary.CanonicalKey = memoryCanonicalKey(semantic, primary.ConversationID, primary.Visibility, decisionServiceRefValues(primary.ActorRefs), decisionServiceRefValues(primary.EventRefs))
	primary.RequestDigest = memorySnapshotDigest(primary)
	updated, err := tx.Exec(ctx, `UPDATE public.memories SET type=$2,content=$3,confidence=$4,importance=$5,emotional_significance=$6,evidence_refs=$7,revision=$8,canonical_key=$9,request_digest=$10,updated_at=$11 WHERE id=$1 AND revision=$12 AND status='active'`, primary.ID, primary.Type, primary.Content, primary.Confidence, primary.Importance, primary.EmotionalSignificance, jsonBytes(primary.EvidenceRefs), primary.Revision, primary.CanonicalKey, primary.RequestDigest, command.OccurredAt.UTC(), base)
	if err != nil {
		return MemoryApplyResult{}, err
	}
	if updated.RowsAffected() != 1 {
		return MemoryApplyResult{}, errors.New("memory_target_revision_conflict")
	}
	if err := refreshMemorySearchDocumentTx(ctx, tx, primary.ID); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := writeMemoryRevisionTx(ctx, tx, command, primary, base, related); err != nil {
		return MemoryApplyResult{}, err
	}
	resulting := map[string]int{primary.ID: primary.Revision}
	for _, memoryID := range related {
		row := rows[memoryID]
		baseRevision := row.Revision
		row.Revision++
		row.Status = "superseded"
		row.SupersededBy = primary.ID
		row.RequestDigest = memorySnapshotDigest(row)
		updated, err := tx.Exec(ctx, `UPDATE public.memories SET status='superseded',revision=$2,superseded_by_memory_id=$3,request_digest=$4,updated_at=$5 WHERE id=$1 AND revision=$6 AND status='active'`, row.ID, row.Revision, primary.ID, row.RequestDigest, command.OccurredAt.UTC(), baseRevision)
		if err != nil {
			return MemoryApplyResult{}, err
		}
		if updated.RowsAffected() != 1 {
			return MemoryApplyResult{}, errors.New("memory_target_revision_conflict")
		}
		if err := writeMemoryRevisionTx(ctx, tx, command, row, baseRevision, []string{primary.ID}); err != nil {
			return MemoryApplyResult{}, err
		}
		if err := staleMemoryEmbeddingsTx(ctx, tx, row.ID); err != nil {
			return MemoryApplyResult{}, err
		}
		resulting[row.ID] = row.Revision
	}
	if err := staleMemoryEmbeddingsTx(ctx, tx, primary.ID); err != nil {
		return MemoryApplyResult{}, err
	}
	result := MemoryApplyResult{Operation: command.Operation, MemoryID: primary.ID, Status: "active", Revision: primary.Revision, Disposition: "applied", ReasonCode: "merged", RelatedMemoryIDs: related, ResultingRevisions: resulting}
	if err := persistMemoryGovernanceTx(ctx, tx, command, result, baseRevisions); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := appendMemoryLifecycleOutboxTx(ctx, tx, command, result); err != nil {
		return MemoryApplyResult{}, err
	}
	return result, a.enqueueMemoryEmbeddingTx(ctx, tx, primary, command)
}

func (a *App) applyMemorySupersedeTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, old memoryAuthorityRow) (MemoryApplyResult, error) {
	base := old.Revision
	replacementCommand := command
	replacementCommand.Operation = MemorySupersede
	replacement, err := a.applyMemoryCreateTx(ctx, tx, replacementCommand, old.ID, false)
	if err != nil {
		return MemoryApplyResult{}, err
	}
	if replacement.Disposition != "applied" {
		return MemoryApplyResult{}, errors.New("memory_supersede_replacement_conflict")
	}
	old.Revision++
	old.Status = "superseded"
	old.SupersededBy = replacement.MemoryID
	old.RequestDigest = memorySnapshotDigest(old)
	updated, err := tx.Exec(ctx, `UPDATE public.memories SET status='superseded',revision=$2,superseded_by_memory_id=$3,request_digest=$4,updated_at=$5 WHERE id=$1 AND revision=$6 AND status='active'`, old.ID, old.Revision, replacement.MemoryID, old.RequestDigest, command.OccurredAt.UTC(), base)
	if err != nil {
		return MemoryApplyResult{}, err
	}
	if updated.RowsAffected() != 1 {
		return MemoryApplyResult{}, errors.New("memory_target_revision_conflict")
	}
	if err := writeMemoryRevisionTx(ctx, tx, command, old, base, []string{replacement.MemoryID}); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := staleMemoryEmbeddingsTx(ctx, tx, old.ID); err != nil {
		return MemoryApplyResult{}, err
	}
	replacement.RelatedMemoryIDs = []string{old.ID}
	replacement.ResultingRevisions[old.ID] = old.Revision
	replacement.ReasonCode = "superseded"
	if err := persistMemoryGovernanceTx(ctx, tx, command, replacement, map[string]int{old.ID: base}); err != nil {
		return MemoryApplyResult{}, err
	}
	return replacement, appendMemoryLifecycleOutboxTx(ctx, tx, command, replacement)
}

func (a *App) applyMemoryDeprecateTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, row memoryAuthorityRow) (MemoryApplyResult, error) {
	base := row.Revision
	row.Revision++
	row.Status = "deprecated"
	row.EvidenceRefs = mergeStringAny(row.EvidenceRefs, command.EvidenceRefs)
	row.RequestDigest = memorySnapshotDigest(row)
	updated, err := tx.Exec(ctx, `UPDATE public.memories SET status='deprecated',revision=$2,evidence_refs=$3,request_digest=$4,deprecated_at=$5,updated_at=$5 WHERE id=$1 AND revision=$6 AND status='active'`, row.ID, row.Revision, jsonBytes(row.EvidenceRefs), row.RequestDigest, command.OccurredAt.UTC(), base)
	if err != nil {
		return MemoryApplyResult{}, err
	}
	if updated.RowsAffected() != 1 {
		return MemoryApplyResult{}, errors.New("memory_target_revision_conflict")
	}
	if err := writeMemoryRevisionTx(ctx, tx, command, row, base, nil); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := staleMemoryEmbeddingsTx(ctx, tx, row.ID); err != nil {
		return MemoryApplyResult{}, err
	}
	result := MemoryApplyResult{Operation: command.Operation, MemoryID: row.ID, Status: row.Status, Revision: row.Revision, Disposition: "applied", ReasonCode: "deprecated", ResultingRevisions: map[string]int{row.ID: row.Revision}}
	if err := persistMemoryGovernanceTx(ctx, tx, command, result, map[string]int{row.ID: base}); err != nil {
		return MemoryApplyResult{}, err
	}
	return result, appendMemoryLifecycleOutboxTx(ctx, tx, command, result)
}

func (a *App) applyMemoryForgetTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, row memoryAuthorityRow) (MemoryApplyResult, error) {
	base := row.Revision
	row.Revision++
	row.Status = "forgotten"
	row.EvidenceRefs = mergeStringAny(row.EvidenceRefs, command.EvidenceRefs)
	row.RequestDigest = memorySnapshotDigest(row)
	updated, err := tx.Exec(ctx, `UPDATE public.memories SET status='forgotten',revision=$2,evidence_refs=$3,request_digest=$4,updated_at=$5 WHERE id=$1 AND revision=$6 AND status='active'`, row.ID, row.Revision, jsonBytes(row.EvidenceRefs), row.RequestDigest, command.OccurredAt.UTC(), base)
	if err != nil {
		return MemoryApplyResult{}, err
	}
	if updated.RowsAffected() != 1 {
		return MemoryApplyResult{}, errors.New("memory_target_revision_conflict")
	}
	if err := writeMemoryRevisionTx(ctx, tx, command, row, base, nil); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := staleMemoryEmbeddingsTx(ctx, tx, row.ID); err != nil {
		return MemoryApplyResult{}, err
	}
	result := MemoryApplyResult{Operation: command.Operation, MemoryID: row.ID, Status: row.Status, Revision: row.Revision, Disposition: "applied", ReasonCode: "forgotten", ResultingRevisions: map[string]int{row.ID: row.Revision}}
	if err := persistMemoryGovernanceTx(ctx, tx, command, result, map[string]int{row.ID: base}); err != nil {
		return MemoryApplyResult{}, err
	}
	return result, appendMemoryLifecycleOutboxTx(ctx, tx, command, result)
}

func (a *App) applyMemoryRollbackTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, current memoryAuthorityRow, rows map[string]memoryAuthorityRow) (MemoryApplyResult, error) {
	targetRevision := *command.RollbackRevision
	baseRevisions := map[string]int{current.ID: current.Revision}
	resulting := make(map[string]int, len(command.RollbackCompensations)+1)
	restoredRows := make(map[string]memoryAuthorityRow, len(command.RollbackCompensations)+1)
	related := make([]string, 0, len(command.RollbackCompensations))
	restored, err := restoreMemorySnapshotTx(ctx, tx, command, current, targetRevision, command.RollbackSnapshot)
	if err != nil {
		return MemoryApplyResult{}, err
	}
	resulting[restored.ID] = restored.Revision
	restoredRows[restored.ID] = restored
	for _, compensation := range command.RollbackCompensations {
		row := rows[compensation.Target.MemoryID]
		baseRevisions[row.ID] = row.Revision
		related = append(related, row.ID)
		if compensation.DeprecateCreated {
			row, err = deprecateRollbackCreatedMemoryTx(ctx, tx, command, row, current.ID)
		} else {
			row, err = restoreMemorySnapshotTx(ctx, tx, command, row, *compensation.RestoreRevision, compensation.RestoreSnapshot)
		}
		if err != nil {
			return MemoryApplyResult{}, err
		}
		resulting[row.ID] = row.Revision
		restoredRows[row.ID] = row
	}
	sort.Strings(related)
	result := MemoryApplyResult{Operation: command.Operation, MemoryID: restored.ID, Status: restored.Status, Revision: restored.Revision, Disposition: "applied", ReasonCode: "rolled_back", RelatedMemoryIDs: related, ResultingRevisions: resulting}
	if err := persistMemoryGovernanceTx(ctx, tx, command, result, baseRevisions); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := appendMemoryLifecycleOutboxTx(ctx, tx, command, result); err != nil {
		return MemoryApplyResult{}, err
	}
	if err := a.enqueueMemoryEmbeddingTx(ctx, tx, restored, command); err != nil {
		return MemoryApplyResult{}, err
	}
	for _, compensation := range command.RollbackCompensations {
		row := restoredRows[compensation.Target.MemoryID]
		if compensation.DeprecateCreated {
			continue
		}
		if err := a.enqueueMemoryEmbeddingTx(ctx, tx, row, command); err != nil {
			return MemoryApplyResult{}, err
		}
	}
	return result, nil
}

func restoreMemorySnapshotTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, current memoryAuthorityRow, targetRevision int, snapshot map[string]any) (memoryAuthorityRow, error) {
	target, err := memoryAuthorityRowFromSnapshot(snapshot)
	if err != nil || target.ID != current.ID || target.OwnerFluctlightID != current.OwnerFluctlightID || target.Revision != targetRevision || target.Status != "active" {
		return memoryAuthorityRow{}, errors.New("memory_rollback_snapshot_invalid")
	}
	base := current.Revision
	liveStatus := current.Status
	current.Type, current.Content = target.Type, target.Content
	current.Confidence, current.Importance, current.EmotionalSignificance = target.Confidence, target.Importance, target.EmotionalSignificance
	current.ActorRefs = stringSliceAny(sortedUniqueStrings(decisionServiceRefValues(target.ActorRefs)))
	current.EventRefs = stringSliceAny(sortedUniqueStrings(decisionServiceRefValues(target.EventRefs)))
	current.EvidenceRefs = mergeStringAny(target.EvidenceRefs, command.EvidenceRefs)
	current.PersonalityPerspectives = make([]any, len(target.PersonalityPerspectives))
	copy(current.PersonalityPerspectives, target.PersonalityPerspectives)
	current.ConversationID = target.ConversationID
	current.Visibility = target.Visibility
	current.Status = "active"
	current.SupersededBy = ""
	current.Supersedes = target.Supersedes
	current.Revision++
	now := command.OccurredAt.UTC()
	current.LastConfirmedAt = &now
	semantic := MemorySemanticInput{Type: current.Type, Content: current.Content, Confidence: current.Confidence, Importance: current.Importance, EmotionalSignificance: current.EmotionalSignificance}
	current.CanonicalKey = memoryCanonicalKey(semantic, current.ConversationID, current.Visibility, decisionServiceRefValues(current.ActorRefs), decisionServiceRefValues(current.EventRefs))
	current.RequestDigest = memorySnapshotDigest(current)
	updated, err := tx.Exec(ctx, `UPDATE public.memories SET type=$2,content=$3,actor_refs=$4,conversation_id=$5,event_refs=$6,evidence_refs=$7,personality_perspectives=$8,confidence=$9,importance=$10,emotional_significance=$11,visibility=$12,status='active',revision=$13,canonical_key=$14,request_digest=$15,superseded_by_memory_id=NULL,supersedes_memory_id=$16,last_confirmed_at=$17,deprecated_at=NULL,updated_at=$17 WHERE id=$1 AND revision=$18 AND status=$19`, current.ID, current.Type, current.Content, jsonBytes(current.ActorRefs), nullableString(current.ConversationID), jsonBytes(current.EventRefs), jsonBytes(current.EvidenceRefs), jsonBytes(current.PersonalityPerspectives), current.Confidence, current.Importance, current.EmotionalSignificance, current.Visibility, current.Revision, current.CanonicalKey, current.RequestDigest, nullableString(current.Supersedes), now, base, liveStatus)
	if err != nil {
		return memoryAuthorityRow{}, err
	}
	if updated.RowsAffected() != 1 {
		return memoryAuthorityRow{}, errors.New("memory_target_revision_conflict")
	}
	if err := refreshMemorySearchDocumentTx(ctx, tx, current.ID); err != nil {
		return memoryAuthorityRow{}, err
	}
	if err := writeMemoryRevisionTx(ctx, tx, command, current, base, nil); err != nil {
		return memoryAuthorityRow{}, err
	}
	if err := staleMemoryEmbeddingsTx(ctx, tx, current.ID); err != nil {
		return memoryAuthorityRow{}, err
	}
	return current, nil
}

func deprecateRollbackCreatedMemoryTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, row memoryAuthorityRow, restoredMemoryID string) (memoryAuthorityRow, error) {
	base := row.Revision
	liveStatus := row.Status
	row.Revision++
	row.Status = "deprecated"
	row.EvidenceRefs = mergeStringAny(row.EvidenceRefs, command.EvidenceRefs)
	row.RequestDigest = memorySnapshotDigest(row)
	updated, err := tx.Exec(ctx, `UPDATE public.memories SET status='deprecated',revision=$2,evidence_refs=$3,request_digest=$4,deprecated_at=$5,updated_at=$5 WHERE id=$1 AND revision=$6 AND status=$7`, row.ID, row.Revision, jsonBytes(row.EvidenceRefs), row.RequestDigest, command.OccurredAt.UTC(), base, liveStatus)
	if err != nil {
		return memoryAuthorityRow{}, err
	}
	if updated.RowsAffected() != 1 {
		return memoryAuthorityRow{}, errors.New("memory_target_revision_conflict")
	}
	if err := writeMemoryRevisionTx(ctx, tx, command, row, base, []string{restoredMemoryID}); err != nil {
		return memoryAuthorityRow{}, err
	}
	if err := staleMemoryEmbeddingsTx(ctx, tx, row.ID); err != nil {
		return memoryAuthorityRow{}, err
	}
	return row, nil
}

func writeMemoryRevisionTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, row memoryAuthorityRow, baseRevision int, related []string) error {
	if related == nil {
		related = []string{}
	}
	revisionID := "memory_revision_" + stableDigest(row.ID+":"+fmt.Sprint(row.Revision))
	idempotency := "memory-revision:" + row.ID + ":" + fmt.Sprint(row.Revision)
	inserted, err := tx.Exec(ctx, `INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,operation,snapshot,content,personality_perspectives,status,actor_id,evidence_refs,source_window,proposal_id,candidate_index,semantic_reason,related_memory_ids,request_digest,reason_code,schema_version,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20) ON CONFLICT(memory_id,revision) DO NOTHING`, revisionID, row.ID, row.Revision, baseRevision, command.Operation, jsonBytes(memorySnapshot(row)), row.Content, jsonBytes(row.PersonalityPerspectives), row.Status, command.ActorID, jsonBytes(row.EvidenceRefs), nullableString(command.SourceWindow), nullableString(command.ProposalID), nullableInt(command.CandidateIndex, command.ProposalID != ""), command.SemanticReason, jsonBytes(related), row.RequestDigest, "memory_"+string(command.Operation), memoryLifecycleSchemaVersion, idempotency)
	if err != nil {
		return err
	}
	if inserted.RowsAffected() != 1 {
		return errors.New("memory_revision_identity_conflict")
	}
	return nil
}

func persistMemoryGovernanceTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, result MemoryApplyResult, baseRevisions map[string]int) error {
	if baseRevisions == nil {
		baseRevisions = map[string]int{}
	}
	if result.RelatedMemoryIDs == nil {
		result.RelatedMemoryIDs = []string{}
	}
	if result.ResultingRevisions == nil {
		result.ResultingRevisions = map[string]int{}
	}
	governanceID := "memory_governance_" + stableDigest(command.IdempotencyKey)
	inserted, err := tx.Exec(ctx, `INSERT INTO public.memory_governance(id,fluctlight_id,proposal_id,source_window,candidate_index,operation,target_memory_id,related_memory_ids,base_revisions,resulting_revisions,actor_id,evidence_refs,semantic_reason,disposition,reason_code,policy_version,request_digest,result,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19) ON CONFLICT(idempotency_key) DO NOTHING`, governanceID, command.OwnerFluctlightID, nullableString(command.ProposalID), nullableString(command.SourceWindow), nullableInt(command.CandidateIndex, command.ProposalID != ""), command.Operation, nullableString(result.MemoryID), jsonBytes(result.RelatedMemoryIDs), jsonBytes(baseRevisions), jsonBytes(result.ResultingRevisions), command.ActorID, jsonBytes(command.EvidenceRefs), command.SemanticReason, result.Disposition, result.ReasonCode, memoryLifecyclePolicyVersion, command.RequestDigest, jsonBytes(result), command.IdempotencyKey)
	if err != nil {
		return err
	}
	if inserted.RowsAffected() != 1 {
		return errors.New("memory_governance_identity_conflict")
	}
	return nil
}

func staleMemoryEmbeddingsTx(ctx context.Context, tx pgx.Tx, memoryID string) error {
	_, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET status='stale',error_code=COALESCE(error_code,'memory_revision_changed') WHERE memory_id=$1 AND status<>'stale'`, memoryID)
	return err
}

func refreshMemorySearchDocumentTx(ctx context.Context, tx pgx.Tx, memoryID string) error {
	var generated string
	if err := tx.QueryRow(ctx, `SELECT is_generated FROM information_schema.columns WHERE table_schema='public' AND table_name='memories' AND column_name='search_document'`).Scan(&generated); err != nil {
		return err
	}
	if generated == "ALWAYS" {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE public.memories SET search_document=to_tsvector('simple',content) WHERE id=$1`, memoryID)
	return err
}

func (a *App) enqueueMemoryEmbeddingTx(ctx context.Context, tx pgx.Tx, row memoryAuthorityRow, command PreparedMemoryMutation) error {
	intentID := "memory_embedding_intent:" + row.ID + ":" + fmt.Sprint(row.Revision)
	workflowID := "memory_embedding:" + row.ID + ":" + fmt.Sprint(row.Revision)
	payload := map[string]any{"memory_id": row.ID, "revision": row.Revision, "request_digest": row.RequestDigest}
	var providerEndpointID, modelID string
	assignmentErr := tx.QueryRow(ctx, `SELECT r.provider_endpoint_id,r.model_id FROM public.model_roles r JOIN public.provider_endpoints e ON e.id=r.provider_endpoint_id WHERE r.role='embedding' AND e.capability_status<>'failed' LIMIT 1`).Scan(&providerEndpointID, &modelID)
	if assignmentErr == nil {
		payload["provider_endpoint_id"] = providerEndpointID
		payload["model_id"] = modelID
	} else if !errors.Is(assignmentErr, pgx.ErrNoRows) {
		return assignmentErr
	}
	inserted, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','memory.embedding',$3) ON CONFLICT(intent_id) DO NOTHING`, intentID, workflowID, jsonBytes(payload))
	if err != nil {
		return err
	}
	if inserted.RowsAffected() != 1 {
		var existing []byte
		if err := tx.QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&existing); err != nil || jsonString(decodeObject(existing)) != jsonString(payload) {
			return errors.New("memory_embedding_intent_conflict")
		}
	}
	return appendOutboxTx(ctx, tx, "memory.embedding.requested", "memory", row.ID, row.OwnerFluctlightID, command.SourceFactID, "memory:"+row.ID, "memory-embedding:"+row.ID+":"+fmt.Sprint(row.Revision), map[string]any{"memory_id": row.ID, "revision": row.Revision, "aggregate_sequence": 2*row.Revision + 2})
}

func appendMemoryLifecycleOutboxTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, result MemoryApplyResult) error {
	return appendOutboxTx(ctx, tx, "memory.lifecycle.applied", "memory", result.MemoryID, command.OwnerFluctlightID, command.SourceFactID, "memory:"+result.MemoryID, "memory-lifecycle:"+command.IdempotencyKey, map[string]any{
		"memory_id": result.MemoryID, "operation": result.Operation, "status": result.Status,
		"revision": result.Revision, "disposition": result.Disposition, "reason_code": result.ReasonCode,
		"related_memory_ids": result.RelatedMemoryIDs, "resulting_revisions": result.ResultingRevisions,
		"aggregate_sequence": 2*result.Revision + 1,
	})
}

func mergeStringAny(existing []any, additional []string) []any {
	values := make([]string, 0, len(existing)+len(additional))
	for _, raw := range existing {
		values = append(values, stringValue(raw))
	}
	values = append(values, additional...)
	return stringSliceAny(sortedUniqueStrings(values))
}

func nullableInt(value int, present bool) any {
	if !present {
		return nil
	}
	return value
}
