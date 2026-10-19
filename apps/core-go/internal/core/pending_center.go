package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
)

var pendingKinds = []string{"capability", "persona_proposal", "studio_apply", "failed_task", "owner_request"}

func ownerQueryFluctlight(query url.Values) (string, error) {
	a, b := query.Get("fluctlightId"), query.Get("fluctlight_id")
	if a != "" && b != "" && a != b {
		return "", invalid("conflicting Fluctlight scope")
	}
	if a == "" {
		a = b
	}
	if len(a) > 128 {
		return "", invalid("invalid Fluctlight scope")
	}
	return a, nil
}

// Each source is an authorized read over its owning authority. No pending
// table duplicates the domain status, and marking a notice read is unrelated.
func pendingSourceSQL(kind string) (string, bool) {
	switch kind {
	case "capability":
		return `SELECT r.id AS source_id,r.fluctlight_id,r.title,r.created_at,r.status,'capability'::text AS target_kind,r.id AS target_id FROM public.capability_requests r JOIN public.fluctlights f ON f.id=r.fluctlight_id WHERE f.created_by_actor_id=$1 AND r.status IN ('proposed','reviewing')`, true
	case "persona_proposal":
		return `SELECT r.id AS source_id,r.fluctlight_id,'人格与身份修订 v'||r.revision AS title,r.created_at,r.status,'persona'::text AS target_kind,r.id AS target_id FROM public.fluctlight_foundation_revisions r JOIN public.fluctlights f ON f.id=r.fluctlight_id WHERE f.created_by_actor_id=$1 AND r.status='proposed'`, true
	case "studio_apply":
		return `SELECT r.id AS source_id,r.fluctlight_id,'待应用资源 '||(r.payload->>'resource_id') AS title,r.created_at,r.status,'resource'::text AS target_kind,r.payload->>'resource_id' AS target_id FROM public.owner_surface_records r WHERE r.owner_actor_id=$1 AND r.kind='studio_apply' AND r.status IN ('recorded_not_applied','pending','conflicted')`, true
	case "owner_request":
		return `SELECT r.id AS source_id,r.fluctlight_id,COALESCE(NULLIF(r.payload->'payload'->>'title',''),r.payload->>'kind') AS title,r.created_at,r.status,'request'::text AS target_kind,r.id AS target_id FROM public.owner_surface_records r WHERE r.owner_actor_id=$1 AND r.kind='owner_request' AND r.status IN ('pending','processing','deferred','awaiting_goal')`, true
	case "failed_task":
		return `SELECT i.intent_id AS source_id,COALESCE(i.payload->>'fluctlight_id','') AS fluctlight_id,i.intent_type AS title,i.created_at,i.status,'workflow'::text AS target_kind,i.intent_id AS target_id FROM public.platform_workflow_intents i LEFT JOIN public.fluctlights f ON f.id=i.payload->>'fluctlight_id' WHERE i.status IN ('failed','dead_letter') AND (f.created_by_actor_id=$1 OR COALESCE(i.payload->>'fluctlight_id','')='') AND NOT EXISTS(SELECT 1 FROM public.fluctlight_diary_attempts g JOIN public.fluctlight_diary_days d ON d.id=g.diary_id WHERE i.intent_type='diary.generate' AND g.id=i.payload->>'diary_attempt_id' AND d.latest_attempt>g.attempt) AND NOT EXISTS(SELECT 1 FROM public.owner_surface_records r WHERE r.owner_actor_id=$1 AND r.kind='owner_request' AND r.status='failed' AND (r.id=i.payload->>'request_id' OR r.payload->>'source_fact_id'=i.payload->>'inbox_id'))
 UNION ALL SELECT r.id AS source_id,r.fluctlight_id,r.payload->>'kind' AS title,r.created_at,r.status,'request'::text AS target_kind,r.id AS target_id FROM public.owner_surface_records r WHERE r.owner_actor_id=$1 AND r.kind='owner_request' AND r.status='failed'`, true
	}
	return "", false
}

func pendingTargetLink(kind, fid, target string) string {
	base := "#/c/" + url.PathEscape(fid)
	switch kind {
	case "capability":
		return base + "/governance/capability/" + url.PathEscape(target)
	case "persona":
		return base + "/governance/foundation/" + url.PathEscape(target)
	case "resource":
		return "#/studio/edit/" + url.PathEscape(target) + "?charId=" + url.QueryEscape(fid)
	case "request":
		return "#/diag/request/" + url.PathEscape(target)
	case "workflow":
		return "#/diag/task/" + url.PathEscape(target)
	}
	return "#/diag/pending"
}

func (a *App) OwnerPending(ctx context.Context, owner string, query url.Values, countOnly bool) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	fid, err := ownerQueryFluctlight(query)
	if err != nil {
		return nil, err
	}
	if fid != "" {
		if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
			return nil, err
		}
	}
	limit, err := reflectionReadPage(query)
	if err != nil {
		return nil, err
	}
	kinds := pendingKinds
	if kind := query.Get("kind"); kind != "" {
		if _, ok := pendingSourceSQL(kind); !ok {
			return nil, invalid("unknown pending source")
		}
		kinds = []string{kind}
	} else if query.Get("cursor") != "" {
		return nil, invalid("a cursor requires its pending source kind")
	}
	groups := []any{}
	total := 0
	partial := false
	for _, kind := range kinds {
		group, count, readErr := a.readPendingSource(ctx, owner, fid, kind, query.Get("cursor"), limit, countOnly)
		if readErr != nil {
			var cursorErr *SurfaceError
			if errors.As(readErr, &cursorErr) && cursorErr.Status == 400 {
				return nil, readErr
			}
			partial = true
			groups = append(groups, map[string]any{"kind": kind, "status": "failed", "error_code": "pending_source_failed", "message": "该来源读取失败，请重试或查看诊断。"})
			continue
		}
		total += count
		groups = append(groups, group)
	}
	if countOnly {
		return map[string]any{"count": total, "partial": partial}, nil
	}
	return map[string]any{"groups": groups, "count": total, "partial": partial}, nil
}

func (a *App) readPendingSource(ctx context.Context, owner, fid, kind, rawCursor string, limit int, countOnly bool) (map[string]any, int, error) {
	source, _ := pendingSourceSQL(kind)
	scope := stableDigest(jsonString([]any{"pending", owner, fid, kind}))
	cursor, err := decodeOwnerReadCursor(rawCursor, scope)
	if err != nil || (rawCursor != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, 0, invalid("pending cursor is invalid")
	}
	var total int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT count(*) FROM (`+source+`)p WHERE ($2='' OR fluctlight_id=$2)`, owner, fid).Scan(&total); err != nil {
		return nil, 0, err
	}
	group := map[string]any{"kind": kind, "status": "ok", "count": total, "items": []any{}, "next_cursor": ""}
	if countOnly {
		return group, total, nil
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT source_id,fluctlight_id,title,created_at,status,target_kind,target_id FROM (`+source+`)p WHERE ($2='' OR fluctlight_id=$2) AND ($3='' OR (created_at,target_kind||':'||source_id)<($4,$3)) ORDER BY created_at DESC,(target_kind||':'||source_id) DESC LIMIT $5`, owner, fid, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []any{}
	last := ownerReadCursor{Scope: scope}
	for rows.Next() {
		var id, role, title, status, targetKind, target string
		var at time.Time
		if err := rows.Scan(&id, &role, &title, &at, &status, &targetKind, &target); err != nil {
			return nil, 0, err
		}
		if len(items) == limit {
			group["next_cursor"] = encodeOwnerReadCursor(last)
			break
		}
		key := targetKind + ":" + id
		items = append(items, map[string]any{"id": key, "source_id": id, "fluctlight_id": role, "title": strings.TrimSpace(title), "status": status, "href": pendingTargetLink(targetKind, role, target), "created_at": at.Format(time.RFC3339Nano)})
		last = ownerReadCursor{Scope: scope, Time: at, ID: key}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	group["items"] = items
	return group, total, nil
}
