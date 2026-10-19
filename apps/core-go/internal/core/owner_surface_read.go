package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) DispatchOwnerSurface(ctx context.Context, method, path string, query url.Values, owner string, body map[string]any) (map[string]any, error) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return nil, &SurfaceError{Status: 503, Code: "surface_unavailable", Message: "Owner surface is unavailable"}
	}
	if method == "GET" && strings.Join(surfaceParts(path), "/") == "configuration-commands/by-key" {
		return a.OwnerConfigurationCommand(ctx, owner, query.Get("key"))
	}
	if method == "POST" && strings.Join(surfaceParts(path), "/") == "configuration-commands/recover" {
		if !eventKeys(body, "key") {
			return nil, ErrInvalidArguments
		}
		return a.RecoverOwnerConfigurationCommand(ctx, owner, stringValue(body["key"]))
	}
	if method == "GET" && strings.Join(surfaceParts(path), "/") == "data/revisions" {
		return a.OwnerObjectRevisions(ctx, owner, query)
	}
	if matchPlaceDefaults(method, path) {
		return a.dispatchPlaceDefaults(ctx, method, path, query, owner, body)
	}
	if matchBusinessBackups(method, path) {
		return a.dispatchBusinessBackups(ctx, method, path, query, owner, body)
	}
	packParts := surfaceParts(path)
	if len(packParts) >= 2 && packParts[0] == "studio" && packParts[1] == "packs" {
		if method == "POST" && len(packParts) == 6 && packParts[3] == "groups" && packParts[5] == "adopt" {
			return a.AdoptResourcePackGroup(ctx, owner, packParts[2], packParts[4], body)
		}
		if method == "GET" && len(packParts) == 3 && packParts[2] == "export" {
			return a.ExportOwnerResourcePack(ctx, owner, query)
		}
		if method == "GET" && len(packParts) == 3 {
			return a.OwnerResourcePack(ctx, owner, packParts[2])
		}
		if method == "POST" && len(packParts) == 2 {
			return a.ImportOwnerResourcePack(ctx, owner, body)
		}
		if method == "POST" && len(packParts) == 3 && packParts[2] == "validate" {
			return a.ValidateOwnerResourcePack(ctx, owner, body)
		}
	}
	configParts := surfaceParts(path)
	if method == "GET" && len(configParts) == 1 && configParts[0] == "configuration-catalog" {
		return a.OwnerConfigurationCatalog(ctx, owner)
	}
	if method == "GET" && len(configParts) == 2 && configParts[0] == "settings" && configParts[1] == "agents" {
		return a.OwnerRuntimeCatalog(ctx, owner)
	}
	if method == "POST" && len(configParts) == 3 && strings.Join(configParts, "/") == "settings/media/test-connection" {
		return a.TestSavedMediaConnection(ctx, owner)
	}
	if matchMomentFeed(method, path) {
		return a.dispatchMomentFeed(ctx, method, path, query, owner, body)
	}
	if method == "GET" && strings.Join(surfaceParts(path), "/") == "diagnostics/harm-intercepts" {
		return a.OwnerHarmSummary(ctx, owner)
	}
	if matchWorkflowControls(method, path) {
		return a.dispatchWorkflowControls(ctx, method, path, query, owner, body)
	}
	if matchDiagnosticOperations(method, path) {
		return a.dispatchDiagnosticOperations(ctx, method, path, query, owner, body)
	}
	if MatchOwnerMediaSurface(method, path) {
		return a.DispatchOwnerMediaSurface(ctx, method, path, query, owner, body)
	}
	parts := surfaceParts(path)
	if method == "GET" && len(parts) >= 2 && parts[0] == "studio" && parts[1] == "resources" {
		if len(parts) == 2 {
			return a.OwnerStudioResources(ctx, owner, query)
		}
		if len(parts) == 5 && parts[3] == "versions" {
			return a.OwnerStudioPublication(ctx, owner, parts[2], parts[4])
		}
		if len(parts) == 4 && parts[3] == "versions" {
			return a.OwnerStudioVersions(ctx, owner, parts[2], query)
		}
	}
	if len(parts) >= 3 && parts[0] == "studio" && parts[1] == "resources" {
		if len(parts) == 3 && method == "DELETE" {
			return a.studioCatalogCommand(ctx, owner, parts[2], "archive", body)
		}
		if len(parts) == 4 && method == "POST" && (parts[3] == "copy" || parts[3] == "restore") {
			return a.studioCatalogCommand(ctx, owner, parts[2], parts[3], body)
		}
	}
	if op, ok := matchSurface(method, surfaceParts(path)); ok && op.name == "visual_manual_review" {
		return a.dispatchOwnerVisualReview(ctx, method, path, query, owner, body)
	}
	if op, ok := matchSurface(method, surfaceParts(path)); ok && op.name == "persona_switch" {
		return a.dispatchOwnerPersonaSwitch(ctx, method, path, query, owner, body)
	}
	if op, ok := matchSurface(method, surfaceParts(path)); ok && (op.name == "moment_suggestion" || op.name == "wear" || op.name == "photo" || op.name == "media_generate" || op.name == "item_use" || op.name == "schedule_item" || op.name == "promise" || op.name == "inventory_request" || op.name == "promise_command") {
		return a.dispatchOwnerLifeRequest(ctx, method, path, query, owner, body)
	}
	if op, ok := matchSurface(method, surfaceParts(path)); ok && (op.name == "reflection_feedback" || op.name == "diary_reply") {
		return a.dispatchOwnerFeedback(ctx, method, path, query, owner, body)
	}
	result, err := NewOwnerSurface(newPostgresSurface(a.DB.Pool()), a.surfaceHooks()).Dispatch(ctx, method, path, query, owner, body)
	if err == nil && method == "GET" && len(parts) == 3 && parts[0] == "studio" && parts[1] == "resources" {
		var visualRefs []any
		visualRefs, err = a.visualStyleReferences(ctx, owner, parts[2])
		result["referenced_by"] = append(arrayValue(result["referenced_by"]), visualRefs...)
		if err == nil {
			var foundationRefs []any
			foundationRefs, err = readWorldObjects(ctx, a.DB.Pool(), foundationResourceReferencesSQL, parts[2], owner)
			result["referenced_by"] = append(arrayValue(result["referenced_by"]), foundationRefs...)
		}
		if err == nil {
			var itemRefs []any
			itemRefs, err = readWorldObjects(ctx, a.DB.Pool(), itemResourceReferencesSQL, parts[2], owner)
			result["referenced_by"] = append(arrayValue(result["referenced_by"]), itemRefs...)
		}
		if err == nil {
			var outfitRefs []any
			outfitRefs, err = readWorldObjects(ctx, a.DB.Pool(), outfitResourceReferencesSQL, parts[2], owner)
			result["referenced_by"] = append(arrayValue(result["referenced_by"]), outfitRefs...)
		}
	}
	return result, err
}

func (a *App) surfaceHooks() SurfaceHooks {
	return SurfaceHooks{
		Promises: a.OwnerPromises, Promise: a.OwnerPromise, PromiseHistory: a.OwnerPromiseHistory, PromiseCommand: a.OwnerPromiseCommand,
		WearingHistory:      a.OwnerWearingHistory,
		SaveInventoryOutfit: a.SaveOwnerInventoryOutfit, InventoryOutfitHistory: a.OwnerInventoryOutfitHistory,
		InventoryOutfit:  a.OwnerInventoryOutfit,
		InventoryOutfits: a.OwnerInventoryOutfits,
		InventoryList:    a.OwnerInventoryList, InventoryItem: a.OwnerInventoryItem, InventoryHistory: a.OwnerInventoryHistory, InventoryCommand: a.OwnerInventoryCommand,
		VisualTimeline: a.OwnerVisualTimeline,
		VisualBrief:    a.OwnerVisualBrief, VisualBriefHistory: a.OwnerVisualBriefHistory, SaveVisualBrief: a.SaveOwnerVisualBrief, VisualIdentity: a.OwnerVisualIdentity, GenerateVisualIdentity: a.GenerateOwnerVisualIdentity,
		GovernanceCatalog: a.OwnerGovernanceCatalog, GovernanceObject: a.OwnerGovernanceObject, GovernanceHistory: a.OwnerGovernanceHistory, CapabilityReview: a.OwnerCapabilityReview,
		Notifications:        a.OwnerNotifications,
		UnreadNotifications:  a.OwnerUnreadNotifications,
		ReadNotification:     a.MarkOwnerNotificationRead,
		ReadAllNotifications: a.MarkAllOwnerNotificationsRead,
		Pending:              a.OwnerPending,
		Task:                 a.OwnerTask,
		RequestTasks:         a.OwnerRequestTasks,
		AutonomyPolicy:       a.OwnerAutonomyPolicy,
		UpdateAutonomyPolicy: a.UpdateOwnerAutonomyPolicy,
		DiaryList:            a.OwnerDiaryList,
		DiaryDetail:          a.OwnerDiary,
		DiaryReplies:         a.OwnerDiaryReplies,
		SharedExperiences:    a.OwnerSharedExperiences,
		GenerateDiary:        a.RequestOwnerDiary,
		AnnotateDiary:        a.AnnotateOwnerDiary,
		SelfClaims:           a.OwnerSelfClaims,
		SelfClaim:            a.OwnerSelfClaim,
		SelfHistory:          a.OwnerSelfHistory,
		SelfCommand:          a.OwnerSelfCommand,
		ReflectionCatalog:    a.OwnerReflectionCatalog,
		ReflectionDetail:     a.OwnerReflection,
		ReflectionFeedback:   a.OwnerReflectionFeedback,
		EnqueueFeedback:      a.enqueueOwnerFeedback,
		EnqueueLifeRequest:   a.enqueueOwnerLifeRequest,
		ApplyPersonaSwitch:   a.applyOwnerPersonaSwitch,
		Now:                  a.now,
		Authorize:            a.authorizeSurface,
		DeleteGroup:          a.DeleteActorGroup,
		PublishMoment:        a.publishOwnerMoment,
		CapabilityItems:      a.surfaceCapabilityItems,
		ListMedia:            a.ListBrowserMedia,
		Sidebar:              a.BrowserSidebar,
		MemoryRevision:       a.surfaceMemoryRevision,
		InstanceRevision:     a.surfaceInstanceRevision,
		ApplyWardrobeEdit:    a.ApplyWardrobeOwnerEdit,
	}
}

// ApplyWardrobeOwnerEdit writes a management-view wardrobe correction onto the
// real worn or in-use rows. It compares and increments fluctlight_wardrobe_states.revision
// under the same row lock the character wear and use paths take. It does not
// call wardrobe.wear or item.use, does not emit those events, and does not
// change wearing_state.
func (a *App) ApplyWardrobeOwnerEdit(ctx context.Context, owner, fid string, edit WardrobeOwnerEdit) (WardrobeOwnerEditResult, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok || a == nil || a.DB == nil {
		return WardrobeOwnerEditResult{}, &SurfaceError{Status: 503, Code: "surface_unavailable", Message: "Wardrobe revision is not connected"}
	}
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnauthorized) {
			return WardrobeOwnerEditResult{}, &SurfaceError{Status: 404, Code: "fluctlight_not_found", Message: "Fluctlight is unavailable"}
		}
		return WardrobeOwnerEditResult{}, err
	}
	if err := lockLifeContextTx(ctx, tx, fid); err != nil {
		return WardrobeOwnerEditResult{}, err
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return WardrobeOwnerEditResult{}, err
	}
	beforeWorn, err := readCurrentWornItems(ctx, tx, fid)
	if err != nil {
		return WardrobeOwnerEditResult{}, err
	}
	var revision int
	err = tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return WardrobeOwnerEditResult{}, &SurfaceError{Status: 404, Code: "wardrobe_not_found", Message: "Wardrobe was not found"}
	}
	if err != nil {
		return WardrobeOwnerEditResult{}, err
	}
	if revision != edit.Expected {
		return WardrobeOwnerEditResult{}, conflict("wardrobe", edit.Expected, revision)
	}
	at := a.now().UTC()
	switch edit.Kind {
	case "wear":
		var slot, itemKind string
		err := tx.QueryRow(ctx, `SELECT slot,item_kind FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 AND archived_at IS NULL FOR SHARE`, fid, edit.ItemID).Scan(&slot, &itemKind)
		if errors.Is(err, pgx.ErrNoRows) {
			return WardrobeOwnerEditResult{}, &SurfaceError{Status: 404, Code: "wardrobe_item_not_found", Message: "Wardrobe item was not found"}
		}
		if err != nil {
			return WardrobeOwnerEditResult{}, err
		}
		if slot != edit.Slot || itemKind != "wearable" {
			return WardrobeOwnerEditResult{}, &SurfaceError{Status: 422, Code: "wardrobe_slot_mismatch", Message: "Wardrobe item does not belong to this slot"}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id,changed_at) VALUES($1,$2,$3,$4) ON CONFLICT(fluctlight_id,slot) DO UPDATE SET item_id=EXCLUDED.item_id,changed_at=EXCLUDED.changed_at`, fid, edit.Slot, edit.ItemID, at); err != nil {
			return WardrobeOwnerEditResult{}, err
		}
	case "item_use":
		var found int
		err := tx.QueryRow(ctx, `SELECT 1 FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 AND item_kind='object' AND archived_at IS NULL FOR SHARE`, fid, edit.ItemID).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			return WardrobeOwnerEditResult{}, &SurfaceError{Status: 404, Code: "wardrobe_item_not_found", Message: "Wardrobe item was not found"}
		}
		if err != nil {
			return WardrobeOwnerEditResult{}, err
		}
		if edit.Active {
			if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_item_uses(fluctlight_id,item_id,revision,activity,started_at,source_event_id) VALUES($1,$2,1,'',$3,'owner_edit') ON CONFLICT(fluctlight_id,item_id) DO UPDATE SET revision=fluctlight_item_uses.revision+1,activity='',started_at=EXCLUDED.started_at,source_event_id='owner_edit'`, fid, edit.ItemID, at); err != nil {
				return WardrobeOwnerEditResult{}, err
			}
		} else if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_item_uses WHERE fluctlight_id=$1 AND item_id=$2`, fid, edit.ItemID); err != nil {
			return WardrobeOwnerEditResult{}, err
		}
	default:
		return WardrobeOwnerEditResult{}, invalid("wardrobe edit kind is invalid")
	}
	newRevision := revision + 1
	tag, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=$2,updated_at=$3 WHERE fluctlight_id=$1 AND revision=$4`, fid, newRevision, at, revision)
	if err != nil {
		return WardrobeOwnerEditResult{}, err
	}
	if tag.RowsAffected() != 1 {
		return WardrobeOwnerEditResult{}, errors.New("wardrobe revision changed during owner edit")
	}
	if err := recordWearingTransitionTx(ctx, tx, fid, "owner_correction", "owner:"+owner, newRevision, beforeWorn, at); err != nil {
		return WardrobeOwnerEditResult{}, err
	}
	return WardrobeOwnerEditResult{Revision: newRevision}, nil
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
	rows, err := a.DB.Pool().Query(ctx, `SELECT a.id, a.kind, a.mime_type, a.status, a.created_at FROM public.media_assets a JOIN public.fluctlights f ON f.id=a.owner_fluctlight_id WHERE f.created_by_actor_id=$1 AND a.media_scope='fluctlight' AND a.owner_fluctlight_id=$2 AND a.kind IN ('image','video') AND a.status<>'deleted' AND ($3='' OR a.id > $3) ORDER BY a.id LIMIT $4`, owner, fid, cursor, limit+1)
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
	view, err := a.displayAvatarWith(ctx, a.DB.Pool(), owner, fid)
	if err != nil {
		detail["avatar"] = nil
		detail["avatar_state"] = map[string]any{"status": "failed", "code": "display_avatar_read_failed"}
		return
	}
	detail["avatar"] = view["avatar"]
	detail["avatar_state"] = map[string]any{"status": view["read_state"], "revision": view["revision"], "mode": view["mode"]}

}

// Historical owner_schedule_item records are drafts, not accepted Schedule
// authority. They must never be appended to the executable daily plan.
func (a *App) mergeOwnerSchedule(ctx context.Context, owner, fid string, detail map[string]any) {}

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
	revision, err := store.Revision(ctx, owner, fid, "wardrobe_owner_count")
	if err == nil {
		page["owner_revision"] = revision
	}
}

func (a *App) readWornItems(ctx context.Context, owner, fid string, store SurfaceStore) ([]any, bool) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT w.slot,w.item_id,w.changed_at,i.description FROM public.fluctlight_worn_items w JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=w.fluctlight_id AND i.id=w.item_id WHERE w.fluctlight_id=$1 AND i.archived_at IS NULL ORDER BY w.slot`, fid)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	type wornRow struct {
		slot, itemID, description string
		changed                   time.Time
	}
	scanned := make([]wornRow, 0)
	for rows.Next() {
		var row wornRow
		if err := rows.Scan(&row.slot, &row.itemID, &row.changed, &row.description); err != nil {
			return nil, false
		}
		scanned = append(scanned, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false
	}
	rows.Close()
	records, err := store.ListKind(ctx, owner, "owner_worn", fid)
	if err != nil {
		return nil, false
	}
	owners := map[string]string{}
	for _, record := range records {
		if record.Status != "active" {
			continue
		}
		owners[record.ResourceKey] = stringValue(record.Payload["item_id"])
	}
	items := make([]any, 0, len(scanned))
	for _, row := range scanned {
		recordedBy := "unknown"
		if owners[row.slot] != "" && owners[row.slot] == row.itemID {
			recordedBy = "owner_annotation"
		}
		items = append(items, map[string]any{"slot": row.slot, "item_id": row.itemID, "description": row.description, "since": row.changed.UTC().Format(time.RFC3339), "recorded_by": recordedBy})
	}
	return items, true
}

func (a *App) readUsedItems(ctx context.Context, owner, fid string, store SurfaceStore) ([]any, bool) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT u.item_id,u.started_at,u.activity,i.description FROM public.fluctlight_item_uses u JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=u.fluctlight_id AND i.id=u.item_id WHERE u.fluctlight_id=$1 AND i.item_kind='object' AND i.availability='available' AND i.ownership IN ('owned','borrowed') AND i.archived_at IS NULL ORDER BY u.item_id`, fid)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	type usedRow struct {
		itemID, activity, description string
		started                       time.Time
	}
	scanned := make([]usedRow, 0)
	for rows.Next() {
		var row usedRow
		if err := rows.Scan(&row.itemID, &row.started, &row.activity, &row.description); err != nil {
			return nil, false
		}
		scanned = append(scanned, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false
	}
	rows.Close()
	records, err := store.ListKind(ctx, owner, "owner_used", fid)
	if err != nil {
		return nil, false
	}
	owners := map[string]struct{}{}
	for _, record := range records {
		if record.Status != "active" {
			continue
		}
		owners[record.ResourceKey] = struct{}{}
	}
	items := make([]any, 0, len(scanned))
	for _, row := range scanned {
		recordedBy := "unknown"
		if _, owned := owners[row.itemID]; owned {
			recordedBy = "owner_annotation"
		}
		items = append(items, map[string]any{"item_id": row.itemID, "activity": row.activity, "description": row.description, "since": row.started.UTC().Format(time.RFC3339), "recorded_by": recordedBy})
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
	for _, item := range items {
		annotateMoment(owner, item)
	}
	// 0089 migrates historical Owner posts into the same authoritative feed.
	// Re-appending stale surface copies would resurrect hidden publications.

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
