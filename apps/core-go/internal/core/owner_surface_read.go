package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) DispatchOwnerSurface(ctx context.Context, method, path string, query url.Values, owner string, body map[string]any) (map[string]any, error) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return nil, &SurfaceError{Status: 503, Code: "surface_unavailable", Message: "Owner surface is unavailable"}
	}
	return NewOwnerSurface(newPostgresSurface(a.DB.Pool()), a.surfaceHooks()).Dispatch(ctx, method, path, query, owner, body)
}

func (a *App) surfaceHooks() SurfaceHooks {
	return SurfaceHooks{
		Now:              a.now,
		Authorize:        a.authorizeSurface,
		DeleteGroup:      a.DeleteActorGroup,
		PublishMoment:    a.publishOwnerMoment,
		CapabilityItems:  a.surfaceCapabilityItems,
		ListMedia:        a.ListBrowserMedia,
		Sidebar:          a.BrowserSidebar,
		MemoryRevision:   a.surfaceMemoryRevision,
		InstanceRevision: a.surfaceInstanceRevision,
	}
}

func (a *App) authorizeSurface(ctx context.Context, owner, fid string) error {
	_, err := a.DB.GetFluctlight(ctx, fid, owner)
	return err
}

func (a *App) publishOwnerMoment(ctx context.Context, owner string, moment map[string]any) error {
	worldID := stringValue(moment["world_id"])
	if _, err := a.DB.GetFluctlight(ctx, worldID, owner); err != nil {
		return err
	}
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.moments(id, owner_fluctlight_id, author_actor_id, text, visibility, status, media_asset_ids) VALUES($1,$2,$3,$4,'owner','visible',$5)`, stringValue(moment["id"]), worldID, owner, stringValue(moment["text"]), jsonBytes(stringList(moment["media_ids"])))
	return err
}

func (a *App) surfaceCapabilityItems(ctx context.Context, owner, fid string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT r.id, r.title, r.status, r.fluctlight_id, r.created_at FROM public.capability_requests r JOIN public.fluctlights f ON f.id=r.fluctlight_id WHERE f.created_by_actor_id=$1 AND r.status IN ('proposed','reviewing') AND ($2='' OR r.fluctlight_id=$2) ORDER BY r.created_at DESC LIMIT 100`, owner, fid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, title, status, fluctlightID string
		var created time.Time
		if err := rows.Scan(&id, &title, &status, &fluctlightID, &created); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "title": title, "status": status, "fluctlight_id": fluctlightID, "created_at": created.UTC().Format(time.RFC3339)})
	}
	return items, rows.Err()
}

func (a *App) surfaceMemoryRevision(ctx context.Context, owner, memoryID string) (int, bool, error) {
	var revision int
	err := a.DB.Pool().QueryRow(ctx, `SELECT m.revision FROM public.memories m JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id WHERE m.id=$1 AND f.created_by_actor_id=$2`, memoryID, owner).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return revision, true, nil
}

func (a *App) surfaceInstanceRevision(ctx context.Context, owner, fid string) (int, error) {
	item, err := a.DB.GetFluctlight(ctx, fid, owner)
	if err != nil {
		return 0, err
	}
	return item.CurrentRevision, nil
}

func (a *App) ListBrowserMedia(ctx context.Context, owner, fid, cursor string, limit int) ([]map[string]any, any, error) {
	if limit < 1 {
		limit = 50
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT a.id, a.kind, a.mime_type, a.status, a.created_at FROM public.media_assets a JOIN public.fluctlights f ON f.id=a.owner_fluctlight_id WHERE f.created_by_actor_id=$1 AND a.owner_fluctlight_id=$2 AND a.kind IN ('image','video') AND a.status<>'deleted' AND ($3='' OR a.id > $3) ORDER BY a.id LIMIT $4`, owner, fid, cursor, limit+1)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, kind, mime, status string
		var created time.Time
		if err := rows.Scan(&id, &kind, &mime, &status, &created); err != nil {
			return nil, nil, err
		}
		var mediaURL any
		if status == "ready" {
			mediaURL = "/api/media/" + id
		}
		items = append(items, map[string]any{"id": id, "kind": kind, "mime_type": mime, "status": status, "created_at": created.UTC().Format(time.RFC3339), "url": mediaURL, "factual": false, "label": "生成图·非事实"})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	var next any
	if len(items) > limit {
		next = items[limit-1]["id"]
		items = items[:limit]
	}
	return items, next, nil
}

func (a *App) BrowserSidebar(ctx context.Context, owner, fid string) (map[string]any, error) {
	detail, err := a.FluctlightDetail(ctx, owner, fid)
	if err != nil {
		return nil, err
	}
	a.DecorateBrowserDetail(ctx, owner, detail)
	contextState := mapValue(detail["context"])
	sidebar := map[string]any{
		"location":      nil,
		"doing":         nil,
		"next_schedule": nil,
		"goal_ids":      idsOf(detail["goals"]),
		"memory_ids":    idsOf(detail["memories"]),
	}
	if value, exists := contextState["location"]; exists {
		sidebar["location"] = value
	}
	if value, exists := contextState["activity"]; exists {
		sidebar["doing"] = value
	}
	if next := nextScheduleItem(detail["schedule"]); next != nil {
		sidebar["next_schedule"] = next
	}
	if worn, ok := a.wornItemIDs(ctx, owner, fid); ok {
		sidebar["worn_item_ids"] = worn
	}
	return sidebar, nil
}

func (a *App) DecorateBrowserDetail(ctx context.Context, owner string, detail map[string]any) {
	if a == nil || detail == nil {
		return
	}
	projectAffect(detail)
	if a.DB == nil || a.DB.Pool() == nil {
		return
	}
	fid := stringValue(detail["id"])
	if scene, ok := a.lookupSceneImage(ctx, fid); ok {
		detail["scene_image"] = scene
	}
	a.projectAvatar(ctx, owner, fid, detail)
	a.mergeOwnerSchedule(ctx, owner, fid, detail)
}

func projectAffect(detail map[string]any) {
	inner := mapValue(detail["inner_state"])
	detail["mood"] = projectLabeledState(mapValue(inner["mood"]), "label")
	detail["momentum"] = projectLabeledState(mapValue(inner["momentum"]), "value")
}

func projectLabeledState(state map[string]any, required string) any {
	if len(state) == 0 {
		return nil
	}
	if required == "label" && stringValue(state["label"]) == "" {
		return nil
	}
	if required == "value" && state["value"] == nil && stringValue(state["label"]) == "" {
		return nil
	}
	out := map[string]any{}
	for _, key := range []string{"label", "source", "intensity", "value", "trend"} {
		if value, exists := state[key]; exists && value != nil && value != "" {
			out[key] = value
		}
	}
	if evidence, exists := state["evidence"]; exists {
		out["evidence"] = evidence
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (a *App) lookupSceneImage(ctx context.Context, fid string) (any, bool) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil || fid == "" {
		return nil, false
	}
	var assetID string
	err := a.DB.Pool().QueryRow(ctx, `SELECT asset_id FROM public.media_references WHERE owner_fluctlight_id=$1 AND target_type IN ('scene','place') ORDER BY created_at DESC LIMIT 1`, fid).Scan(&assetID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && assetID == "") {
		return nil, true
	}
	if err != nil {
		return nil, false
	}
	return map[string]any{"asset_id": assetID, "url": "/api/media/" + assetID, "factual": false, "label": "生成图·非事实"}, true
}

func (a *App) projectAvatar(ctx context.Context, owner, fid string, detail map[string]any) {
	store := newPostgresSurface(a.DB.Pool())
	records, err := store.ListKind(ctx, owner, "visual_identity", fid)
	if err != nil {
		return
	}
	for _, record := range records {
		if stringValue(record.Payload["source"]) != "owner_upload" {
			continue
		}
		avatar := map[string]any{"source": "owner_upload", "kind": "visual_identity", "label": "插画"}
		if assetID := stringValue(record.Payload["asset_id"]); assetID != "" {
			avatar["asset_id"] = assetID
			avatar["url"] = "/api/media/" + assetID
		}
		detail["avatar"] = avatar
		return
	}
	canonical := stringValue(mapValue(detail["visual_identity"])["canonical_asset_id"])
	if canonical == "" {
		detail["avatar"] = nil
		return
	}
	detail["avatar"] = map[string]any{"source": "generated", "kind": "generated", "asset_id": canonical, "url": "/api/media/" + canonical, "factual": false, "label": "生成图·非事实"}
}

func (a *App) mergeOwnerSchedule(ctx context.Context, owner, fid string, detail map[string]any) {
	records, err := newPostgresSurface(a.DB.Pool()).ListKind(ctx, owner, "owner_schedule_item", fid)
	if err != nil || len(records) == 0 {
		return
	}
	extra := make([]any, 0, len(records))
	for _, record := range records {
		if record.Status == "deleted" {
			continue
		}
		item := cloneMap(record.Payload)
		item["source"] = "owner_edit"
		item["recorded_by"] = "owner"
		extra = append(extra, item)
	}
	if len(extra) == 0 {
		return
	}
	schedule := mapValue(detail["schedule"])
	if detail["schedule"] == nil {
		schedule = map[string]any{"items": extra}
		detail["schedule"] = schedule
		return
	}
	items := arrayValue(schedule["items"])
	schedule["items"] = append(items, extra...)
}

func (a *App) DecorateWardrobe(ctx context.Context, owner, fid string, page map[string]any) {
	if a == nil || a.DB == nil || page == nil {
		return
	}
	store := newPostgresSurface(a.DB.Pool())
	worn, wornOK := a.readWornItems(ctx, owner, fid, store)
	used, usedOK := a.readUsedItems(ctx, owner, fid, store)
	if wornOK {
		page["worn_items"] = worn
	}
	if usedOK {
		page["used_items"] = used
	}
	revision, err := store.Revision(ctx, owner, fid, "wardrobe:"+fid)
	if err == nil {
		page["owner_revision"] = revision
	}
}

func (a *App) readWornItems(ctx context.Context, owner, fid string, store SurfaceStore) ([]any, bool) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT slot, item_id, changed_at FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 ORDER BY slot`, fid)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	items := make([]any, 0)
	for rows.Next() {
		var slot, itemID string
		var changed time.Time
		if err := rows.Scan(&slot, &itemID, &changed); err != nil {
			return nil, false
		}
		items = append(items, map[string]any{"slot": slot, "item_id": itemID, "since": changed.UTC().Format(time.RFC3339), "recorded_by": "fluctlight"})
	}
	if err := rows.Err(); err != nil {
		return nil, false
	}
	records, err := store.ListKind(ctx, owner, "owner_worn", fid)
	if err != nil {
		return nil, false
	}
	for _, record := range records {
		if record.Status != "active" {
			continue
		}
		item := cloneMap(record.Payload)
		item["recorded_by"] = "owner"
		items = append(items, item)
	}
	return items, true
}

func (a *App) readUsedItems(ctx context.Context, owner, fid string, store SurfaceStore) ([]any, bool) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT item_id, started_at FROM public.fluctlight_item_uses WHERE fluctlight_id=$1 ORDER BY item_id`, fid)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	items := make([]any, 0)
	for rows.Next() {
		var itemID string
		var started time.Time
		if err := rows.Scan(&itemID, &started); err != nil {
			return nil, false
		}
		items = append(items, map[string]any{"item_id": itemID, "since": started.UTC().Format(time.RFC3339), "recorded_by": "fluctlight"})
	}
	if err := rows.Err(); err != nil {
		return nil, false
	}
	records, err := store.ListKind(ctx, owner, "owner_used", fid)
	if err != nil {
		return nil, false
	}
	for _, record := range records {
		if record.Status != "active" {
			continue
		}
		item := cloneMap(record.Payload)
		item["recorded_by"] = "owner"
		items = append(items, item)
	}
	return items, true
}

func (a *App) wornItemIDs(ctx context.Context, owner, fid string) ([]any, bool) {
	items, ok := a.readWornItems(ctx, owner, fid, newPostgresSurface(a.DB.Pool()))
	if !ok {
		return nil, false
	}
	ids := make([]any, 0, len(items))
	for _, item := range items {
		if id := stringValue(mapValue(item)["item_id"]); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, true
}

func (a *App) DecorateFluctlightList(ctx context.Context, owner string, items []Fluctlight) any {
	encoded, err := json.Marshal(items)
	if err != nil {
		return items
	}
	var rows []map[string]any
	if json.Unmarshal(encoded, &rows) != nil {
		return items
	}
	groups, ok := a.fluctlightGroups(ctx, owner)
	if !ok {
		return rows
	}
	for _, row := range rows {
		if groupID, exists := groups[stringValue(row["id"])]; exists {
			row["group_id"] = groupID
			continue
		}
		row["group_id"] = nil
	}
	return rows
}

func (a *App) fluctlightGroups(ctx context.Context, owner string) (map[string]string, bool) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return nil, false
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT m.actor_id, m.group_id FROM public.actor_group_members m JOIN public.actor_groups g ON g.id=m.group_id WHERE g.owner_actor_id=$1 ORDER BY m.created_at, m.group_id`, owner)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	groups := map[string]string{}
	for rows.Next() {
		var actorID, groupID string
		if err := rows.Scan(&actorID, &groupID); err != nil {
			return nil, false
		}
		if _, exists := groups[actorID]; !exists {
			groups[actorID] = groupID
		}
	}
	return groups, rows.Err() == nil
}

func (a *App) DecorateMoments(ctx context.Context, owner string, items []map[string]any) []map[string]any {
	if items == nil {
		items = []map[string]any{}
	}
	seen := map[string]struct{}{}
	for _, item := range items {
		annotateMoment(owner, item)
		seen[stringValue(item["id"])] = struct{}{}
	}
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return items
	}
	records, err := newPostgresSurface(a.DB.Pool()).ListKind(ctx, owner, "user_moment", "")
	if err != nil {
		return items
	}
	for _, record := range records {
		if _, exists := seen[record.ID]; exists {
			continue
		}
		item := cloneMap(record.Payload)
		annotateMoment(owner, item)
		items = append(items, item)
	}
	return items
}

func annotateMoment(owner string, item map[string]any) {
	author := stringValue(item["author_actor_id"])
	switch {
	case author != "" && author == stringValue(item["owner_fluctlight_id"]):
		item["author_type"] = "fluctlight"
	case author != "" && author == owner:
		item["author_type"] = "user"
	default:
		item["author_type"] = "npc"
		item["full_fluctlight"] = false
	}
	if _, exists := item["author_name"]; !exists {
		item["author_name"] = nil
	}
	if _, exists := item["avatar"]; !exists {
		item["avatar"] = nil
	}
	if _, exists := item["source_action_id"]; !exists {
		item["source_action_id"] = nil
	}
}

func idsOf(value any) []any {
	ids := make([]any, 0)
	for _, item := range arrayValue(value) {
		if id := stringValue(mapValue(item)["id"]); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func nextScheduleItem(value any) map[string]any {
	schedule := mapValue(value)
	if value == nil {
		return nil
	}
	now := time.Now().UTC()
	for _, item := range arrayValue(schedule["items"]) {
		row := mapValue(item)
		if end := stringValue(row["end_at"]); end != "" {
			if parsed, err := time.Parse(time.RFC3339, end); err == nil && !parsed.After(now) {
				continue
			}
		}
		return row
	}
	return nil
}
