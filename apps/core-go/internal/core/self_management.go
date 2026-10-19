package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const selfClaimColumns = `c.id,c.fluctlight_id,c.category,c.claim,c.value,c.confidence,c.evidence_refs,c.provenance,c.status,c.expires_at,c.revision,c.superseded_by,c.created_at,c.updated_at`
const selfClaimProjection = `to_jsonb(c)||jsonb_build_object('source_valid',NOT EXISTS(SELECT 1 FROM public.actor_fact_artifacts d JOIN public.actor_facts f ON f.id=d.fact_id WHERE d.artifact_kind='developing_self' AND d.artifact_id=c.id AND d.artifact_revision=c.revision AND (f.status<>'active' OR f.revision<>d.fact_revision OR f.valid_from>$3 OR (f.valid_until IS NOT NULL AND f.valid_until<=$3))),'expired',c.expires_at IS NOT NULL AND c.expires_at<=$3,'owner_review',(SELECT to_jsonb(r) FROM public.fluctlight_self_owner_reviews r WHERE r.claim_id=c.id AND r.owner_actor_id=$2 AND r.claim_revision=c.revision ORDER BY r.created_at DESC,r.id DESC LIMIT 1))`

func (a *App) OwnerSelfClaims(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	status, category, term := q.Get("status"), q.Get("category"), strings.TrimSpace(q.Get("q"))
	if status == "all" {
		status = ""
	}
	if status != "" && status != "active" && status != "uncertain" && status != "forgotten" && status != "superseded" {
		return nil, invalid("invalid claim status")
	}
	if category != "" && !validateDevelopingSelfCategory(category) {
		return nil, invalid("invalid claim category")
	}
	if len([]rune(term)) > 512 {
		return nil, invalid("claim query too long")
	}
	scope := stableDigest(jsonString([]any{"self-claims", owner, fid, status, category, term}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid claim cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT c.id,c.updated_at,`+selfClaimProjection+` FROM public.fluctlight_developing_self_claims c WHERE c.fluctlight_id=$1 AND ($4='' OR c.status=$4) AND ($5='' OR c.category=$5) AND ($6='' OR strpos(lower(c.claim),lower($6))>0) AND ($7='' OR (c.updated_at,c.id)<($8,$7)) ORDER BY c.updated_at DESC,c.id DESC LIMIT $9`, fid, owner, a.now().UTC(), status, category, term, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	last := ownerReadCursor{Scope: scope}
	next := ""
	for rows.Next() {
		var id string
		var at time.Time
		var raw []byte
		if err := rows.Scan(&id, &at, &raw); err != nil {
			return nil, err
		}
		if len(items) == limit {
			next = encodeOwnerReadCursor(last)
			break
		}
		items = append(items, decodeObject(raw))
		last = ownerReadCursor{Scope: scope, ID: id, Time: at}
	}
	return map[string]any{"items": items, "next_cursor": next}, rows.Err()
}

func (a *App) OwnerSelfClaim(ctx context.Context, owner, fid, id string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT `+selfClaimProjection+` FROM public.fluctlight_developing_self_claims c WHERE c.fluctlight_id=$1 AND c.id=$4`, fid, owner, a.now().UTC(), id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &SurfaceError{Status: 404, Code: "self_claim_not_found", Message: "认识不存在或不属于当前角色"}
	}
	return decodeObject(raw), err
}

func (a *App) OwnerSelfHistory(ctx context.Context, owner, fid, id, kind string, q url.Values) (map[string]any, error) {
	if _, err := a.OwnerSelfClaim(ctx, owner, fid, id); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	if kind != "revisions" && kind != "reviews" {
		return nil, invalid("invalid self history kind")
	}
	scope := stableDigest(jsonString([]any{"self-history", owner, fid, id, kind}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid history cursor")
	}
	table := "public.fluctlight_developing_self_revisions"
	filter := ""
	if kind == "reviews" {
		table = "public.fluctlight_self_owner_reviews"
		filter = " AND r.owner_actor_id=$6"
	}
	query := `SELECT r.id,r.created_at,to_jsonb(r) FROM ` + table + ` r WHERE r.claim_id=$1 AND r.fluctlight_id=$2 AND ($3='' OR (r.created_at,r.id)<($4,$3))` + filter + ` ORDER BY r.created_at DESC,r.id DESC LIMIT $5`
	args := []any{id, fid, cursor.ID, cursor.Time, limit + 1}
	if kind == "reviews" {
		args = append(args, owner)
	}
	rows, err := a.DB.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	next := ""
	last := ownerReadCursor{Scope: scope}
	for rows.Next() {
		var rid string
		var at time.Time
		var raw []byte
		if err := rows.Scan(&rid, &at, &raw); err != nil {
			return nil, err
		}
		if len(items) == limit {
			next = encodeOwnerReadCursor(last)
			break
		}
		items = append(items, decodeObject(raw))
		last = ownerReadCursor{Scope: scope, ID: rid, Time: at}
	}
	return map[string]any{"items": items, "next_cursor": next}, rows.Err()
}

func (a *App) OwnerSelfCommand(ctx context.Context, owner, fid, id, operation string, body map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("self_command_transaction_required")
	}
	expected, ok := parseRevision(body["expected_revision"])
	reason := strings.TrimSpace(stringValue(body["reason"]))
	if !ok || expected < 1 || reason == "" || len([]rune(reason)) > 1024 {
		return nil, invalid("claim revision and reason required")
	}
	if operation != "review" && operation != "forget" && operation != "restore" && operation != "rollback" {
		return nil, invalid("unsupported self command")
	}
	if err := lockLifeContextTx(ctx, tx, fid); err != nil {
		return nil, err
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	current, err := scanDevelopingSelfClaim(tx.QueryRow(ctx, `SELECT `+selfClaimColumns+` FROM public.fluctlight_developing_self_claims c WHERE c.fluctlight_id=$1 AND c.id=$2 FOR UPDATE`, fid, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &SurfaceError{Status: 404, Code: "self_claim_not_found", Message: "认识不存在"}
	}
	if err != nil {
		return nil, err
	}
	if current.Revision != expected {
		return nil, &SurfaceError{Status: 409, Code: "self_claim_revision_conflict", Message: "认识版本已变化，请核对最新版本后提交", Details: map[string]any{"current_revision": current.Revision}}
	}
	if operation == "review" {
		reviewID := randomID("self_review_")
		_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_self_owner_reviews(id,fluctlight_id,claim_id,claim_revision,owner_actor_id,reason,disposition) VALUES($1,$2,$3,$4,$5,$6,'acknowledged')`, reviewID, fid, id, current.Revision, owner, reason)
		if err != nil {
			return nil, err
		}
		audit, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, fid, "self_review", reason, id)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "fluctlight_id": fid, "revision": current.Revision, "status": current.Status, "operation": "review", "review_id": reviewID, "audit_id": audit, "semantics": "Owner acknowledgement; evidence, confidence and effective truth unchanged"}, nil
	}
	before := decodeObject(jsonBytes(current))
	// Explicit null belongs to a complete new snapshot. Absence remains reserved
	// for old semantic snapshots that never recorded these boundaries.
	before["expires_at"], before["superseded_by"] = current.ExpiresAt, current.SupersededBy
	after := cloneMap(before)
	target := 0
	if operation == "forget" {
		after["status"] = "forgotten"
	} else {
		target, ok = parseRevision(body["target_revision"])
		if !ok || target < 1 {
			return nil, invalid("target revision required")
		}
		var snapshot, candidate []byte
		err := tx.QueryRow(ctx, `SELECT after_value,candidate FROM public.fluctlight_developing_self_revisions WHERE fluctlight_id=$1 AND claim_id=$2 AND revision=$3 AND status='accepted'`, fid, id, target).Scan(&snapshot, &candidate)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &SurfaceError{Status: 422, Code: "self_snapshot_unavailable", Message: "这版没有可恢复的有效认识快照"}
		}
		if err != nil {
			return nil, err
		}
		selected := decodeObject(snapshot)
		if stringValue(selected["category"]) == "" {
			selected = decodeObject(candidate)
		}
		for _, field := range []string{"category", "claim", "value", "confidence", "evidence_refs", "provenance", "status"} {
			if _, exists := selected[field]; !exists {
				return nil, &SurfaceError{Status: 422, Code: "self_snapshot_incomplete", Message: "旧修订缺少完整快照，不能猜测恢复字段"}
			}
		}
		normalized, err := normalizeDevelopingSelfClaim(selected, "owner_restore")
		if err != nil {
			return nil, &SurfaceError{Status: 422, Code: "self_snapshot_invalid", Message: "该历史快照不能恢复为有效认识"}
		}
		if err := validateArtifactActorDependenciesTx(ctx, tx, fid, "developing_self", id, target, a.now().UTC()); err != nil {
			return nil, &SurfaceError{Status: 409, Code: "self_snapshot_source_changed", Message: "该历史认识依赖的来源已变化，请核对来源"}
		}
		for key, value := range normalized {
			after[key] = value
		}
		// Older semantic snapshots did not record expiry/supersession. Preserve
		// those current boundaries unless the chosen snapshot explicitly has them.
		for _, key := range []string{"expires_at", "superseded_by"} {
			if value, exists := selected[key]; exists {
				after[key] = value
			}
		}
	}
	next := current.Revision + 1
	after["revision"], after["updated_at"] = next, a.now().UTC().Format(time.RFC3339Nano)
	var expires *time.Time
	if raw := stringValue(after["expires_at"]); raw != "" {
		value, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return nil, invalid("snapshot expiry invalid")
		}
		expires = &value
	}
	_, err = tx.Exec(ctx, `UPDATE public.fluctlight_developing_self_claims SET category=$2,claim=$3,value=$4,confidence=$5,evidence_refs=$6,provenance=$7,status=$8,revision=$9,expires_at=$10,superseded_by=$11,updated_at=$12 WHERE id=$1 AND fluctlight_id=$13`, id, after["category"], after["claim"], jsonBytes(after["value"]), after["confidence"], jsonBytes(after["evidence_refs"]), jsonBytes(after["provenance"]), after["status"], next, expires, after["superseded_by"], a.now().UTC(), fid)
	if err != nil {
		return nil, err
	}
	ledgerStatus := "accepted"
	if operation == "forget" {
		ledgerStatus = "forgotten"
	}
	ledgerCandidate := cloneMap(after)
	ledgerCandidate["reason"], ledgerCandidate["owner_actor_id"] = reason, owner
	if err := a.writeDevelopingSelfRevision(ctx, tx, fid, id, operation, "owner_"+operation, ledgerStatus, "owner:"+owner, next, current.Revision, ledgerCandidate, jsonBytes(before), jsonBytes(after), arrayValue(after["evidence_refs"])); err != nil {
		return nil, err
	}
	dependencyRevision := current.Revision
	if operation == "restore" || operation == "rollback" {
		dependencyRevision = target
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.actor_fact_artifacts(fact_id,fact_revision,artifact_kind,artifact_id,artifact_revision) SELECT fact_id,fact_revision,artifact_kind,artifact_id,$3 FROM public.actor_fact_artifacts WHERE artifact_kind='developing_self' AND artifact_id=$1 AND artifact_revision=$2 ON CONFLICT DO NOTHING`, id, dependencyRevision, next); err != nil {
		return nil, err
	}
	audit, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, fid, "self_"+operation, reason, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "fluctlight_id": fid, "revision": next, "status": after["status"], "operation": operation, "target_revision": target, "audit_id": audit}, nil
}
