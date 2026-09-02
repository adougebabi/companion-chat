package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var developingSelfCategories = map[string]struct{}{
	"preference": {}, "habit": {}, "sensitivity": {}, "emotion_pattern": {},
	"self_perception": {}, "capability": {}, "interest": {},
}

func validateEvidenceRefs(refs []any, allowed map[string]struct{}) bool {
	if len(refs) == 0 {
		return false
	}
	for _, raw := range refs {
		value := strings.TrimSpace(stringValue(raw))
		if value == "" {
			return false
		}
		if len(allowed) > 0 {
			if _, ok := allowed[value]; !ok {
				return false
			}
		}
	}
	return true
}

func validateDevelopingSelfCategory(category string) bool {
	_, ok := developingSelfCategories[strings.TrimSpace(category)]
	return ok
}

func normalizeDevelopingSelfClaim(raw map[string]any, defaultSource string) (map[string]any, error) {
	category := strings.TrimSpace(stringValue(raw["category"]))
	claim := strings.TrimSpace(stringValue(raw["claim"]))
	if category == "" || !validateDevelopingSelfCategory(category) {
		return nil, errors.New("developing_self_category_invalid")
	}
	if claim == "" || len([]rune(claim)) > 2000 {
		return nil, errors.New("developing_self_claim_invalid")
	}
	confidence, ok := numberFloat(raw["confidence"])
	if !ok || confidence < 0 || confidence > 1 {
		return nil, errors.New("developing_self_confidence_invalid")
	}
	provenance := mapValue(raw["provenance"])
	if len(provenance) == 0 {
		provenance = map[string]any{"source": defaultSource}
	} else if stringValue(provenance["source"]) == "" {
		provenance["source"] = defaultSource
	}
	refs := arrayValue(raw["evidence_refs"])
	for _, ref := range refs {
		if strings.TrimSpace(stringValue(ref)) == "" {
			return nil, errors.New("developing_self_evidence_invalid")
		}
	}
	status := firstString(raw["status"], "active")
	if status != "active" && status != "uncertain" {
		return nil, errors.New("developing_self_status_invalid")
	}
	return map[string]any{"category": category, "claim": claim, "value": raw["value"], "confidence": confidence, "evidence_refs": refs, "provenance": provenance, "status": status}, nil
}

func (a *App) insertDevelopingSelfSeeds(ctx context.Context, tx pgx.Tx, fluctlightID string, claims []any) error {
	for index, raw := range claims {
		normalized, err := normalizeDevelopingSelfClaim(mapValue(raw), "owner_defined")
		if err != nil {
			return fmt.Errorf("developing_self_seed_invalid: %w", err)
		}
		refs := arrayValue(normalized["evidence_refs"])
		if len(refs) == 0 {
			refs = []any{"initialization:" + fluctlightID}
			normalized["evidence_refs"] = refs
		}
		claimID := "self_claim_initial_" + stableDigest(fluctlightID+":"+fmt.Sprint(index)+":"+stringValue(normalized["claim"]))
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_developing_self_claims(id,fluctlight_id,category,claim,value,confidence,evidence_refs,provenance,status,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,1,$10,$10) ON CONFLICT(id) DO NOTHING`, claimID, fluctlightID, normalized["category"], normalized["claim"], jsonBytes(normalized["value"]), normalized["confidence"], jsonBytes(refs), jsonBytes(normalized["provenance"]), normalized["status"], now); err != nil {
			return err
		}
		revisionID := "self_revision_initial_" + stableDigest(claimID)
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_developing_self_revisions(id,fluctlight_id,claim_id,revision,base_revision,change_type,candidate,before_value,after_value,confidence,evidence_refs,provenance,source_window,reason_code,status,created_at) VALUES($1,$2,$3,1,0,'initialization',$4,'{}',$4,$5,$6,$7,$8,'initialization_seed','accepted',$9) ON CONFLICT(id) DO NOTHING`, revisionID, fluctlightID, claimID, jsonBytes(normalized), normalized["confidence"], jsonBytes(refs), jsonBytes(normalized["provenance"]), "initialization:"+fluctlightID, now); err != nil {
			return err
		}
	}
	return nil
}

func scanDevelopingSelfClaim(row interface{ Scan(...any) error }) (DevelopingSelfClaim, error) {
	var result DevelopingSelfClaim
	var value, refs, provenance []byte
	if err := row.Scan(&result.ID, &result.FluctlightID, &result.Category, &result.Claim, &value, &result.Confidence, &refs, &provenance, &result.Status, &result.ExpiresAt, &result.Revision, &result.SupersededBy, &result.CreatedAt, &result.UpdatedAt); err != nil {
		return DevelopingSelfClaim{}, err
	}
	result.Value = decodeJSONValue(value)
	result.EvidenceRefs = decodeStringArray(refs)
	result.Provenance = decodeObject(provenance)
	return result, nil
}

func decodeStringArray(value []byte) []string {
	decoded := decodeArray(value)
	result := make([]string, 0, len(decoded))
	for _, item := range decoded {
		if text := stringValue(item); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func (a *App) listDevelopingSelfClaims(ctx context.Context, fluctlightID string) ([]DevelopingSelfClaim, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,fluctlight_id,category,claim,value,confidence,evidence_refs,provenance,status,expires_at,revision,superseded_by,created_at,updated_at FROM public.fluctlight_developing_self_claims WHERE fluctlight_id=$1 AND status IN ('active','uncertain') AND (expires_at IS NULL OR expires_at > now()) ORDER BY updated_at DESC,id LIMIT 200`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]DevelopingSelfClaim, 0)
	for rows.Next() {
		claim, err := scanDevelopingSelfClaim(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, claim)
	}
	return result, rows.Err()
}

func (a *App) ListDevelopingSelfClaims(ctx context.Context, fluctlightID string) ([]DevelopingSelfClaim, error) {
	return a.listDevelopingSelfClaims(ctx, fluctlightID)
}

func (a *App) rollbackDevelopingSelfClaim(ctx context.Context, actorID, claimID string, expectedRevision int, reason string) (map[string]any, error) {
	if strings.TrimSpace(claimID) == "" || expectedRevision < 1 || strings.TrimSpace(reason) == "" {
		return nil, errors.New("developing_self_rollback_invalid")
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var fluctlightID, category, claim string
		var currentRevision int
		var before []byte
		if err := tx.QueryRow(ctx, `SELECT c.fluctlight_id,c.category,c.claim,c.revision,c.value FROM public.fluctlight_developing_self_claims c JOIN public.fluctlights f ON f.id=c.fluctlight_id WHERE c.id=$1 AND f.created_by_actor_id=$2 FOR UPDATE`, claimID, actorID).Scan(&fluctlightID, &category, &claim, &currentRevision, &before); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if currentRevision != expectedRevision {
			return ErrConflict
		}
		var target []byte
		if err := tx.QueryRow(ctx, `SELECT candidate FROM public.fluctlight_developing_self_revisions WHERE claim_id=$1 AND revision=$2 AND status='accepted'`, claimID, expectedRevision-1).Scan(&target); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("developing_self_rollback_unavailable")
			}
			return err
		}
		previous := decodeObject(target)
		if len(previous) == 0 {
			return errors.New("developing_self_rollback_unavailable")
		}
		newRevision := currentRevision + 1
		command, err := tx.Exec(ctx, `UPDATE public.fluctlight_developing_self_claims SET category=$2,claim=$3,value=$4,confidence=$5,evidence_refs=$6,provenance=$7,status=$8,revision=$9,updated_at=now() WHERE id=$1 AND revision=$10`, claimID, firstString(previous["category"], category), firstString(previous["claim"], claim), jsonBytes(previous["value"]), previous["confidence"], jsonBytes(arrayValue(previous["evidence_refs"])), jsonBytes(mapValue(previous["provenance"])), firstString(previous["status"], "active"), newRevision, expectedRevision)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		rollbackID := "self_revision_rollback_" + stableDigest(claimID+":"+fmt.Sprint(newRevision))
		rollbackCandidate := map[string]any{"category": firstString(previous["category"], category), "claim": firstString(previous["claim"], claim), "reason": reason}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_developing_self_revisions(id,fluctlight_id,claim_id,revision,base_revision,change_type,candidate,before_value,after_value,evidence_refs,reason_code,status) VALUES($1,$2,$3,$4,$5,'rollback',$6,$7,$8,'[]',$9,'accepted') ON CONFLICT(id) DO NOTHING`, rollbackID, fluctlightID, claimID, newRevision, expectedRevision, jsonBytes(rollbackCandidate), before, target, "owner_rollback"); err != nil {
			return err
		}
		result = map[string]any{"id": claimID, "fluctlight_id": fluctlightID, "revision": newRevision, "status": "rolled_back", "category": firstString(previous["category"], category), "claim": firstString(previous["claim"], claim)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func canonicalEvidenceKey(ctx context.Context, tx pgx.Tx, fluctlightID, ref string) string {
	if strings.HasPrefix(ref, "sequence:") {
		return ref
	}
	var sequence int
	if err := tx.QueryRow(ctx, `SELECT sequence FROM public.cognition_inbox WHERE id=$1 AND fluctlight_id=$2`, ref, fluctlightID).Scan(&sequence); err == nil {
		return fmt.Sprintf("sequence:%d", sequence)
	}
	return ref
}

func (a *App) distinctDevelopingSelfEvidence(ctx context.Context, tx pgx.Tx, fluctlightID string, refs []any, allowed map[string]struct{}) ([]any, error) {
	if !validateEvidenceRefs(refs, allowed) {
		return nil, errors.New("developing_self_evidence_invalid")
	}
	seen := make(map[string]struct{}, len(refs))
	result := make([]any, 0, len(refs))
	for _, raw := range refs {
		ref := strings.TrimSpace(stringValue(raw))
		key := canonicalEvidenceKey(ctx, tx, fluctlightID, ref)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, ref)
	}
	return result, nil
}

func (a *App) writeDevelopingSelfRevision(ctx context.Context, tx pgx.Tx, fluctlightID, claimID, changeType, reasonCode, status, sourceWindow string, revision, baseRevision int, candidate map[string]any, before, after []byte, refs []any) error {
	revisionID := "self_revision_" + stableDigest(fluctlightID+":"+claimID+":"+changeType+":"+sourceWindow+":"+reasonCode+":"+string(jsonBytes(candidate)))
	var confidence any
	if value, ok := numberFloat(candidate["confidence"]); ok {
		confidence = value
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_developing_self_revisions(id,fluctlight_id,claim_id,revision,base_revision,change_type,candidate,before_value,after_value,confidence,evidence_refs,provenance,source_window,reason_code,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT(id) DO NOTHING`, revisionID, fluctlightID, nullableString(claimID), revision, baseRevision, changeType, jsonBytes(candidate), before, after, confidence, jsonBytes(refs), jsonBytes(mapValue(candidate["provenance"])), nullableString(sourceWindow), reasonCode, status)
	return err
}

func (a *App) applyDevelopingSelfCandidateTx(ctx context.Context, tx pgx.Tx, fluctlightID string, candidate map[string]any, evidenceRefs []any, allowedEvidence map[string]struct{}, sourceWindow string) error {
	normalized, err := normalizeDevelopingSelfClaim(candidate, "reflection")
	if err != nil {
		return err
	}
	distinctRefs, err := a.distinctDevelopingSelfEvidence(ctx, tx, fluctlightID, evidenceRefs, allowedEvidence)
	if err != nil {
		return err
	}
	if len(distinctRefs) < 2 {
		return a.writeDevelopingSelfRevision(ctx, tx, fluctlightID, "", "rejected", "insufficient_evidence", "rejected", sourceWindow, 0, 0, normalized, []byte(`{}`), []byte(`{}`), distinctRefs)
	}
	normalized["evidence_refs"] = distinctRefs
	category := stringValue(normalized["category"])
	claimText := stringValue(normalized["claim"])
	var claimID string
	var revision int
	var beforeValue, beforeRefs, beforeProvenance []byte
	var beforeConfidence float64
	err = tx.QueryRow(ctx, `SELECT id,revision,value,evidence_refs,provenance,confidence FROM public.fluctlight_developing_self_claims WHERE fluctlight_id=$1 AND category=$2 AND claim=$3 AND status IN ('active','uncertain') ORDER BY revision DESC LIMIT 1 FOR UPDATE`, fluctlightID, category, claimText).Scan(&claimID, &revision, &beforeValue, &beforeRefs, &beforeProvenance, &beforeConfidence)
	if errors.Is(err, pgx.ErrNoRows) {
		claimID = "self_claim_" + stableDigest(fluctlightID+":"+category+":"+claimText)
		revision = 1
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_developing_self_claims(id,fluctlight_id,category,claim,value,confidence,evidence_refs,provenance,status,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(id) DO NOTHING`, claimID, fluctlightID, category, claimText, jsonBytes(normalized["value"]), normalized["confidence"], jsonBytes(distinctRefs), jsonBytes(normalized["provenance"]), normalized["status"], revision); err != nil {
			return err
		}
		return a.writeDevelopingSelfRevision(ctx, tx, fluctlightID, claimID, "create", "accepted", "accepted", sourceWindow, 1, 0, normalized, []byte(`{}`), jsonBytes(normalized), distinctRefs)
	}
	if err != nil {
		return err
	}
	previousRefs := decodeArray(beforeRefs)
	canonicalPrevious := make([]any, 0, len(previousRefs))
	for _, ref := range previousRefs {
		canonicalPrevious = append(canonicalPrevious, canonicalEvidenceKey(ctx, tx, fluctlightID, stringValue(ref)))
	}
	canonicalCurrent := make([]any, 0, len(distinctRefs))
	for _, ref := range distinctRefs {
		canonicalCurrent = append(canonicalCurrent, canonicalEvidenceKey(ctx, tx, fluctlightID, stringValue(ref)))
	}
	normalizedConfidence, _ := numberFloat(normalized["confidence"])
	if sameEvidence(canonicalPrevious, canonicalCurrent) && string(jsonBytes(decodeJSONValue(beforeValue))) == string(jsonBytes(normalized["value"])) && beforeConfidence == normalizedConfidence {
		return nil
	}
	newRevision := revision + 1
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_developing_self_claims SET value=$2,confidence=$3,evidence_refs=$4,provenance=$5,status=$6,revision=$7,updated_at=now() WHERE id=$1 AND revision=$8`, claimID, jsonBytes(normalized["value"]), normalized["confidence"], jsonBytes(distinctRefs), jsonBytes(normalized["provenance"]), normalized["status"], newRevision, revision)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	previous := map[string]any{"category": category, "claim": claimText, "value": decodeJSONValue(beforeValue), "confidence": beforeConfidence, "evidence_refs": previousRefs, "provenance": decodeObject(beforeProvenance), "status": "active"}
	return a.writeDevelopingSelfRevision(ctx, tx, fluctlightID, claimID, "update", "accepted", "accepted", sourceWindow, newRevision, revision, normalized, jsonBytes(previous), jsonBytes(normalized), distinctRefs)
}

func (a *App) listDevelopingSelfRevisions(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,claim_id,revision,base_revision,change_type,candidate,before_value,after_value,confidence,evidence_refs,provenance,source_window,reason_code,status,created_at FROM public.fluctlight_developing_self_revisions WHERE fluctlight_id=$1 ORDER BY created_at DESC,id DESC LIMIT 200`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, changeType, reasonCode, status string
		var claimID, sourceWindow *string
		var revision, baseRevision int
		var candidate, before, after, refs, provenance []byte
		var confidence *float64
		var created time.Time
		if err := rows.Scan(&id, &claimID, &revision, &baseRevision, &changeType, &candidate, &before, &after, &confidence, &refs, &provenance, &sourceWindow, &reasonCode, &status, &created); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"id": id, "claim_id": claimID, "revision": revision, "base_revision": baseRevision,
			"change_type": changeType, "candidate": decodeJSONValue(candidate), "before": decodeJSONValue(before), "after": decodeJSONValue(after),
			"confidence": confidence, "evidence_refs": decodeArray(refs), "provenance": decodeObject(provenance), "source_window": sourceWindow,
			"reason_code": reasonCode, "status": status, "created_at": created.Format(time.RFC3339Nano),
		})
	}
	return result, rows.Err()
}

func (a *App) ListDevelopingSelfRevisions(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	return a.listDevelopingSelfRevisions(ctx, fluctlightID)
}

func (a *App) forgetDevelopingSelfClaim(ctx context.Context, actorID, claimID string, expectedRevision int, reason string) (map[string]any, error) {
	if strings.TrimSpace(claimID) == "" || expectedRevision < 1 || strings.TrimSpace(reason) == "" {
		return nil, errors.New("developing_self_forget_invalid")
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var fluctlightID, category, claim string
		var revision int
		var value, refs, provenance []byte
		var confidence float64
		if err := tx.QueryRow(ctx, `SELECT c.fluctlight_id,c.category,c.claim,c.revision,c.value,c.confidence,c.evidence_refs,c.provenance FROM public.fluctlight_developing_self_claims c JOIN public.fluctlights f ON f.id=c.fluctlight_id WHERE c.id=$1 AND f.created_by_actor_id=$2 FOR UPDATE`, claimID, actorID).Scan(&fluctlightID, &category, &claim, &revision, &value, &confidence, &refs, &provenance); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if revision != expectedRevision {
			return ErrConflict
		}
		newRevision := revision + 1
		command, err := tx.Exec(ctx, `UPDATE public.fluctlight_developing_self_claims SET status='forgotten',revision=$2,updated_at=now() WHERE id=$1 AND revision=$3`, claimID, newRevision, expectedRevision)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		candidate := map[string]any{"category": category, "claim": claim, "value": decodeJSONValue(value), "confidence": confidence, "evidence_refs": decodeArray(refs), "provenance": decodeObject(provenance), "status": "forgotten", "reason": reason}
		if err := a.writeDevelopingSelfRevision(ctx, tx, fluctlightID, claimID, "forget", "owner_forget", "forgotten", "owner:"+actorID, newRevision, expectedRevision, candidate, jsonBytes(candidate), jsonBytes(map[string]any{}), decodeArray(refs)); err != nil {
			return err
		}
		result = map[string]any{"id": claimID, "fluctlight_id": fluctlightID, "revision": newRevision, "status": "forgotten", "reason": reason}
		return nil
	})
	return result, err
}

func (a *App) RollbackDevelopingSelfClaim(ctx context.Context, actorID, claimID string, expectedRevision int, reason string) (map[string]any, error) {
	return a.rollbackDevelopingSelfClaim(ctx, actorID, claimID, expectedRevision, reason)
}

func (a *App) ForgetDevelopingSelfClaim(ctx context.Context, actorID, claimID string, expectedRevision int, reason string) (map[string]any, error) {
	return a.forgetDevelopingSelfClaim(ctx, actorID, claimID, expectedRevision, reason)
}
