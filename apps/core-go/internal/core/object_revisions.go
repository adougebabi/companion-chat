package core

import (
	"context"
	"net/url"
	"time"
)

// This is a directory of real owning objects. History and restoration remain
// in the original domain page and preserve its revision/evidence semantics.
func (a *App) OwnerObjectRevisions(ctx context.Context, owner string, q url.Values) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	kind, fid, id, search := q.Get("kind"), q.Get("fluctlightId"), q.Get("objectId"), q.Get("q")
	if len(kind) > 64 || len(fid) > 128 || len(id) > 128 || len([]rune(search)) > 256 {
		return nil, ErrDiagnosticsFilterInvalid
	}
	scope := stableDigest(jsonString([]any{"object-revisions", owner, kind, fid, id, search}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, invalid("对象修订游标不属于当前筛选")
	}
	const source = `SELECT 'memory:'||m.id AS key,m.id,'memory'::text AS kind,m.owner_fluctlight_id AS fid,left(m.content,180) AS title,m.revision,m.updated_at AS at,(SELECT count(*) FROM public.memory_revisions v WHERE v.memory_id=m.id) AS history_count,''::text AS context_id FROM public.memories m JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id WHERE f.created_by_actor_id=$1
 UNION ALL SELECT 'relationship:'||r.id,r.id,'relationship',r.owner_fluctlight_id,left(COALESCE(r.summary,r.target_actor_id),180),r.revision,r.updated_at,(SELECT count(*) FROM public.relationship_revisions v WHERE v.relationship_id=r.id),'' FROM public.relationships r JOIN public.fluctlights f ON f.id=r.owner_fluctlight_id WHERE f.created_by_actor_id=$1
 UNION ALL SELECT 'resource:'||r.id,r.id,CASE WHEN r.payload->>'kind'='npc' THEN 'npc_definition' ELSE 'resource' END,'',r.payload->>'name',r.revision,r.updated_at,(SELECT count(*) FROM public.studio_resource_versions v WHERE v.resource_id=r.id AND v.owner_actor_id=$1),'' FROM public.owner_surface_records r WHERE r.owner_actor_id=$1 AND r.kind='studio_resource'
 UNION ALL SELECT 'self:'||c.id,c.id,'self',c.fluctlight_id,left(c.claim,180),c.revision,c.updated_at,(SELECT count(*) FROM public.fluctlight_developing_self_revisions v WHERE v.claim_id=c.id),'' FROM public.fluctlight_developing_self_claims c JOIN public.fluctlights f ON f.id=c.fluctlight_id WHERE f.created_by_actor_id=$1
 UNION ALL SELECT 'place:'||h.place_id,h.place_id,'place',h.fluctlight_id,left(h.after_value->>'name',180),h.place_revision,h.created_at,(SELECT count(*) FROM public.world_place_default_history v WHERE v.place_id=h.place_id AND v.owner_actor_id=$1),h.world_id FROM (SELECT DISTINCT ON(place_id) * FROM public.world_place_default_history WHERE owner_actor_id=$1 ORDER BY place_id,created_at DESC,id DESC) h`
	rows, err := a.DB.Pool().Query(ctx, `WITH objects AS (`+source+`) SELECT key,id,kind,fid,title,revision,at,history_count,context_id FROM objects WHERE ($2::text='' OR kind=$2) AND ($3::text='' OR fid=$3) AND ($4::text='' OR id=$4) AND ($5::text='' OR strpos(lower(COALESCE(title,'')),lower($5))>0) AND ($6::text='' OR (at,key)<($7,$6)) ORDER BY at DESC,key DESC LIMIT 31`, owner, kind, fid, id, search, cursor.ID, cursor.Time)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	keys := []string{}
	times := []time.Time{}
	for rows.Next() {
		var key, object, typ, scopeID, contextID string
		var title *string
		var revision int
		var at time.Time
		var history int64
		if err := rows.Scan(&key, &object, &typ, &scopeID, &title, &revision, &at, &history, &contextID); err != nil {
			return nil, err
		}
		root := "/c/" + url.PathEscape(scopeID)
		href := ""
		switch typ {
		case "place":
			href = root + "/worlds/" + url.PathEscape(contextID) + "/place/" + url.PathEscape(object) + "/history"
		case "memory":
			href = root + "/growth/memory/" + url.PathEscape(object)
		case "relationship":
			href = root + "/people/relation/" + url.PathEscape(object)
		case "npc_definition":
			href = "/studio/npc/" + url.PathEscape(object)
		case "resource":
			href = "/studio/edit/" + url.PathEscape(object)
		case "self":
			href = root + "/growth/self/claim/" + url.PathEscape(object)
		}
		items = append(items, map[string]any{"id": object, "kind": typ, "fluctlight_id": scopeID, "title": title, "revision": revision, "history_count": history, "updated_at": at.UTC().Format(time.RFC3339Nano), "href": href})
		keys = append(keys, key)
		times = append(times, at)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	next := ""
	if len(items) > 30 {
		next = encodeOwnerReadCursor(ownerReadCursor{Scope: scope, ID: keys[29], Time: times[29]})
		items = items[:30]
	}
	return map[string]any{"items": items, "next_cursor": next, "filters": map[string]any{"kind": kind, "fluctlight_id": fid, "object_id": id, "q": search}, "semantics": "领域对象与保留的地点字段历史按最近修改或记录排序；并发修订后可刷新重新查询。进入原领域读取全部修订和受控恢复，不回滚角色时间线。"}, nil
}
