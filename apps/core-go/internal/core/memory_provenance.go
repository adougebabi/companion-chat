package core

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type memorySource struct {
	kind        string
	id          string
	revision    *int
	fingerprint string
	occurredAt  *time.Time
}

func memorySourcesStillValid(ctx context.Context, query currentAuthorityReader, memoryID string, revision int, provenanceStatus string) (bool, error) {
	if provenanceStatus == "pending" || provenanceStatus == "invalid" {
		return false, nil
	}
	if provenanceStatus == "legacy_unknown" {
		return true, nil
	}
	var supported bool
	err := query.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.memory_source_links l
WHERE l.memory_id=$1 AND l.memory_revision=$2 AND l.status='valid'
 AND public.memory_source_is_live(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint))`, memoryID, revision).Scan(&supported)
	return supported, err
}

// Source resolution proves a reference exists in the authorized Fluctlight
// scope. It does not claim that the source semantically supports the derived
// text. An unresolved ref remains unknown instead of becoming a fake fact.
func resolveMemorySourceTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, ref string) (memorySource, error) {
	ref = strings.TrimSpace(ref)
	var source memorySource
	var occurred time.Time
	switch {
	case strings.HasPrefix(ref, "sequence:"):
		sequence, err := strconv.Atoi(strings.TrimPrefix(ref, "sequence:"))
		if err != nil || sequence < 0 {
			return source, nil
		}
		err = tx.QueryRow(ctx, `SELECT id,public.cognition_source_fingerprint(payload),occurred_at FROM public.cognition_inbox WHERE fluctlight_id=$1 AND sequence=$2`, command.OwnerFluctlightID, sequence).Scan(&source.id, &source.fingerprint, &occurred)
		if errors.Is(err, pgx.ErrNoRows) {
			return memorySource{}, nil
		}
		if err != nil {
			return memorySource{}, err
		}
		source.kind = "fact"
	case strings.HasPrefix(ref, "fact:"):
		id := strings.TrimPrefix(ref, "fact:")
		err := tx.QueryRow(ctx, `SELECT id,public.cognition_source_fingerprint(payload),occurred_at FROM public.cognition_inbox WHERE fluctlight_id=$1 AND id=$2`, command.OwnerFluctlightID, id).Scan(&source.id, &source.fingerprint, &occurred)
		if errors.Is(err, pgx.ErrNoRows) {
			return memorySource{}, nil
		}
		if err != nil {
			return memorySource{}, err
		}
		source.kind = "fact"
	case strings.HasPrefix(ref, "message:"):
		id := strings.TrimPrefix(ref, "message:")
		err := tx.QueryRow(ctx, `SELECT msg.id,md5(msg.text || msg.attachment_refs::text),msg.created_at
FROM public.conversation_messages msg WHERE msg.id=$1 AND
 (msg.conversation_id=$2 OR EXISTS (SELECT 1 FROM public.conversation_participants p
  WHERE p.conversation_id=msg.conversation_id AND p.actor_id=$3))`, id, command.ConversationID, command.OwnerFluctlightID).Scan(&source.id, &source.fingerprint, &occurred)
		if errors.Is(err, pgx.ErrNoRows) {
			return memorySource{}, nil
		}
		if err != nil {
			return memorySource{}, err
		}
		source.kind = "message"
	case strings.HasPrefix(ref, "outcome:"):
		id := strings.TrimPrefix(ref, "outcome:")
		var revision int
		err := tx.QueryRow(ctx, `SELECT id,revision,md5(request_digest || ':' || revision::text),occurred_at FROM public.cognition_action_outcomes WHERE id=$1 AND fluctlight_id=$2`, id, command.OwnerFluctlightID).Scan(&source.id, &revision, &source.fingerprint, &occurred)
		if errors.Is(err, pgx.ErrNoRows) {
			return memorySource{}, nil
		}
		if err != nil {
			return memorySource{}, err
		}
		source.kind, source.revision = "outcome", &revision
	default:
		if err := tx.QueryRow(ctx, `SELECT id,public.cognition_source_fingerprint(payload),occurred_at FROM public.cognition_inbox WHERE fluctlight_id=$1 AND id=$2`, command.OwnerFluctlightID, ref).Scan(&source.id, &source.fingerprint, &occurred); err == nil {
			source.kind, source.occurredAt = "fact", &occurred
			return source, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return memorySource{}, err
		}
		if command.AuthenticatedDirect && ref == command.SourceFactID {
			return memorySource{kind: "authenticated_command", id: command.IdempotencyKey, fingerprint: command.RequestDigest}, nil
		}
		if command.ActorID == command.OwnerActorID && slices.Contains(command.EvidenceRefs, ref) {
			// The owner-governance command itself is the confirmation source.
			// It is distinct from an invented conversation or action event.
			return memorySource{kind: "owner_confirmation", id: command.IdempotencyKey, fingerprint: command.RequestDigest}, nil
		}
		return memorySource{}, nil
	}
	source.occurredAt = &occurred
	return source, nil
}

func resolveFrozenMemorySourceTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, dependent memoryAuthorityRow, frozen FrozenMemoryEvidenceSource) (memorySource, error) {
	var source memorySource
	switch frozen.Kind {
	case "memory":
		if frozen.ID == dependent.ID {
			return source, errors.New("memory_source_cycle")
		}
		var owner, status, digest, conversationID, provenanceStatus string
		var revision int
		var occurred *time.Time
		err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,revision,status,request_digest,COALESCE(conversation_id,''),provenance_status,occurred_at FROM public.memories WHERE id=$1 FOR SHARE`, frozen.ID).Scan(&owner, &revision, &status, &digest, &conversationID, &provenanceStatus, &occurred)
		if errors.Is(err, pgx.ErrNoRows) {
			return source, errors.New("memory_source_version_stale")
		}
		if err != nil {
			return source, err
		}
		if owner != command.OwnerFluctlightID || revision != frozen.Revision || status != "active" || digest != frozen.Fingerprint || (conversationID != "" && conversationID != command.ConversationID) {
			return source, errors.New("memory_source_version_stale")
		}
		valid, err := memorySourcesStillValid(ctx, tx, frozen.ID, revision, provenanceStatus)
		if err != nil {
			return source, err
		}
		if !valid || provenanceStatus == "legacy_unknown" {
			return source, errors.New("memory_source_unverified")
		}
		var cycle bool
		if err := tx.QueryRow(ctx, `WITH RECURSIVE ancestry(id) AS (
 SELECT source_id FROM public.memory_source_links WHERE memory_id=$1 AND source_kind='memory' AND status='valid'
 UNION SELECT l.source_id FROM public.memory_source_links l JOIN ancestry a ON l.memory_id=a.id
 WHERE l.source_kind='memory' AND l.status='valid'
) SELECT EXISTS(SELECT 1 FROM ancestry WHERE id=$2)`, frozen.ID, dependent.ID).Scan(&cycle); err != nil {
			return source, err
		}
		if cycle {
			return source, errors.New("memory_source_cycle")
		}
		source = memorySource{kind: "memory", id: frozen.ID, revision: &revision, fingerprint: digest, occurredAt: occurred}
	case "outcome":
		var owner, digest string
		var revision int
		var occurred time.Time
		err := tx.QueryRow(ctx, `SELECT fluctlight_id,revision,request_digest,occurred_at FROM public.cognition_action_outcomes WHERE id=$1 FOR SHARE`, frozen.ID).Scan(&owner, &revision, &digest, &occurred)
		if errors.Is(err, pgx.ErrNoRows) {
			return source, errors.New("memory_source_version_stale")
		}
		if err != nil {
			return source, err
		}
		fingerprint := fmt.Sprintf("%x", md5.Sum([]byte(digest+":"+fmt.Sprint(revision))))
		if owner != command.OwnerFluctlightID || revision != frozen.Revision || fingerprint != frozen.Fingerprint {
			return source, errors.New("memory_source_version_stale")
		}
		source = memorySource{kind: "outcome", id: frozen.ID, revision: &revision, fingerprint: fingerprint, occurredAt: &occurred}
	default:
		return source, errors.New("memory_source_kind_invalid")
	}
	return source, nil
}

func writeMemorySourceLinksTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, row memoryAuthorityRow) error {
	for _, ref := range sortedUniqueStrings(decisionServiceRefValues(row.EvidenceRefs)) {
		var source memorySource
		var err error
		if frozen, found := command.FrozenEvidenceSources[ref]; found {
			source, err = resolveFrozenMemorySourceTx(ctx, tx, command, row, frozen)
		} else {
			source, err = resolveMemorySourceTx(ctx, tx, command, ref)
		}
		if err != nil {
			return err
		}
		if expected := command.ExpectedEvidenceFingerprints[ref]; expected != "" && expected != source.fingerprint {
			return errors.New("memory_source_version_stale")
		}
		status := "unknown"
		kind := "legacy"
		if source.id != "" {
			status, kind = "valid", source.kind
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.memory_source_links(memory_id,memory_revision,source_ref,source_kind,source_id,source_revision,source_fingerprint,occurred_at,status)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, row.ID, row.Revision, ref, kind, nullableString(source.id), source.revision, nullableString(source.fingerprint), source.occurredAt, status); err != nil {
			return err
		}
	}
	var validCount, unknownCount int
	hasOutcome := false
	for _, raw := range row.EvidenceRefs {
		if strings.HasPrefix(stringValue(raw), "outcome:") {
			hasOutcome = true
			break
		}
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='valid'),count(*) FILTER (WHERE status='unknown') FROM public.memory_source_links WHERE memory_id=$1 AND memory_revision=$2`, row.ID, row.Revision).Scan(&validCount, &unknownCount); err != nil {
		return err
	}
	provenanceStatus := "pending"
	if row.Status != "active" {
		provenanceStatus = "invalid"
	} else if validCount > 0 && unknownCount == 0 {
		provenanceStatus = "verified"
	} else if validCount > 0 {
		provenanceStatus = "partial"
	}
	epistemicKind := "inference"
	switch {
	case command.ActorID == command.OwnerActorID:
		epistemicKind = "user_statement"
	case hasOutcome && row.Type == "episodic":
		epistemicKind = "action_result"
	case command.AuthenticatedDirect:
		epistemicKind = "self_report"
	case command.ProposalID != "":
		epistemicKind = "summary"
	}
	return setMemoryProvenanceStatusTx(ctx, tx, row.ID, row.Revision, provenanceStatus, epistemicKind)
}
