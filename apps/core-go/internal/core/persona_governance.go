package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/url"
	"strings"
	"time"
)

// Management reads are paged independently of the bounded runtime projection.
// Table selection is internal; request input is always passed as SQL parameters.
func governanceTable(kind string) (string, error) {
	switch kind {
	case "foundation":
		return "public.fluctlight_foundation_revisions", nil
	case "capability":
		return "public.capability_requests", nil
	}
	return "", invalid("unknown governance kind")
}
func (a *App) OwnerGovernanceCatalog(ctx context.Context, owner, fid, kind string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	table, err := governanceTable(kind)
	if err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	status, term := q.Get("status"), strings.TrimSpace(q.Get("q"))
	if status == "all" {
		status = ""
	}
	valid := map[string]bool{"": true, "proposed": true, "accepted": true, "rejected": true}
	if kind == "capability" {
		valid["reviewing"], valid["fulfilled"], valid["cancelled"] = true, true, true
	}
	if !valid[status] || len([]rune(term)) > 512 {
		return nil, invalid("invalid governance filter")
	}
	scope := stableDigest(jsonString([]any{"governance", owner, fid, kind, status, term}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid governance cursor")
	}
	search := "r.changes::text"
	if kind == "capability" {
		search = "r.title||' '||r.description||' '||r.capability_key"
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT r.id,r.created_at,to_jsonb(r)-'idempotency_key' FROM `+table+` r WHERE r.fluctlight_id=$1 AND ($2='' OR r.status=$2) AND ($3='' OR strpos(lower(`+search+`),lower($3))>0) AND ($4='' OR (r.created_at,r.id)<($5,$4)) ORDER BY r.created_at DESC,r.id DESC LIMIT $6`, fid, status, term, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}
func readGovernanceRows(rows pgx.Rows, scope string, limit int) (map[string]any, error) {
	items := []any{}
	next := ""
	last := ownerReadCursor{Scope: scope}
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
func (a *App) OwnerGovernanceObject(ctx context.Context, owner, fid, kind, id string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	table, err := governanceTable(kind)
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = a.DB.Pool().QueryRow(ctx, `SELECT to_jsonb(r)-'idempotency_key' FROM `+table+` r WHERE r.id=$1 AND r.fluctlight_id=$2`, id, fid).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := decodeObject(raw)
	if kind == "capability" {
		registered := map[string]any{"available": false}
		if capability, ok := a.capabilityRegistry().Lookup(stringValue(result["capability_key"])); ok {
			registered = map[string]any{"available": true, "version": capability.Definition().Version}
		}
		result["registered_capability"] = registered
		result["semantics"] = "Review records a decision; authorization is enforced separately at Tool admission. Fulfilled requires a matching registered implementation."
	}
	return result, nil
}
func (a *App) OwnerGovernanceHistory(ctx context.Context, owner, fid, kind, id string, q url.Values) (map[string]any, error) {
	if _, err := a.OwnerGovernanceObject(ctx, owner, fid, kind, id); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"governance-history", owner, fid, kind, id}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid governance history cursor")
	}
	table, column := "public.fluctlight_foundation_governance", "revision_id"
	if kind == "capability" {
		table, column = "public.capability_request_reviews", "request_id"
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT r.id,r.created_at,to_jsonb(r) FROM `+table+` r WHERE r.fluctlight_id=$1 AND r.`+column+`=$2 AND ($3='' OR (r.created_at,r.id)<($4,$3)) ORDER BY r.created_at DESC,r.id DESC LIMIT $5`, fid, id, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}
func capabilityReviewTransition(previous, next string) bool {
	switch previous {
	case "proposed":
		return next == "reviewing" || next == "accepted" || next == "rejected" || next == "cancelled"
	case "reviewing":
		return next == "accepted" || next == "rejected" || next == "cancelled"
	case "accepted":
		return next == "fulfilled" || next == "cancelled"
	}
	return false
}
func (a *App) OwnerCapabilityReview(ctx context.Context, owner, fid, id string, body map[string]any) (map[string]any, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("capability_review_transaction_required")
	}
	expected, ok := parseRevision(body["expected_revision"])
	status, note, version := stringValue(body["status"]), strings.TrimSpace(stringValue(body["note"])), strings.TrimSpace(stringValue(body["capability_version"]))
	if !ok || expected < 1 || note == "" || len([]rune(note)) > 2000 || len(version) > 128 {
		return nil, invalid("invalid capability review")
	}
	if status != "fulfilled" && version != "" {
		return nil, invalid("only fulfillment can bind an implementation version")
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT to_jsonb(r)-'idempotency_key' FROM public.capability_requests r WHERE r.id=$1 AND r.fluctlight_id=$2 FOR UPDATE`, id, fid).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	before := decodeObject(raw)
	previous := stringValue(before["status"])
	if intValue(before["revision"]) != expected {
		return nil, &SurfaceError{Status: 409, Code: "capability_review_conflict", Message: "申请已被其他操作更新，请重新读取后审核", Details: map[string]any{"current_revision": before["revision"]}}
	}
	if !capabilityReviewTransition(previous, status) {
		return nil, &SurfaceError{Status: 409, Code: "capability_review_transition_invalid", Message: "当前申请状态不允许这项操作", Details: map[string]any{"current_status": previous, "requested_status": status}}
	}
	if status == "fulfilled" {
		capability, registered := a.capabilityRegistry().Lookup(stringValue(before["capability_key"]))
		if !registered {
			return nil, &SurfaceError{Status: 422, Code: "capability_unavailable", Message: "该能力尚无实际注册实现，不能标为已交付"}
		}
		if version == "" || version != capability.Definition().Version {
			return nil, &SurfaceError{Status: 422, Code: "capability_version_mismatch", Message: "填写的版本与实际注册实现不一致"}
		}
	}
	next := expected + 1
	reviewID := randomID("capability_review_")
	var afterRaw []byte
	err = tx.QueryRow(ctx, `UPDATE public.capability_requests r SET status=$3,review_note=$4,reviewer_actor_id=$5,capability_version=CASE WHEN $6='' THEN capability_version ELSE $6 END,revision=$7,updated_at=now() WHERE id=$1 AND fluctlight_id=$2 RETURNING to_jsonb(r)-'idempotency_key'`, id, fid, status, note, owner, version, next).Scan(&afterRaw)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.capability_request_reviews(id,request_id,fluctlight_id,revision,previous_status,status,note,capability_version,reviewer_actor_id,before_value,after_value) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, reviewID, id, fid, next, previous, status, note, nullableString(version), owner, raw, afterRaw); err != nil {
		return nil, err
	}
	if err := appendOutboxTx(ctx, tx, "capability.request.reviewed", "capability_request", id, fid, owner, reviewID, reviewID, map[string]any{"request_id": id, "from_status": previous, "to_status": status, "capability_version": version, "revision": next}); err != nil {
		return nil, err
	}
	audit, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, fid, "capability_review", note, id)
	if err != nil {
		return nil, err
	}
	result := decodeObject(afterRaw)
	result["review_id"], result["audit_id"] = reviewID, audit
	return result, nil
}
