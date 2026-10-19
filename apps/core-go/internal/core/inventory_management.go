package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/url"
	"strings"
	"time"
)

// All Owner inventory entry points join one authority/transaction. Legacy CRUD
// and versioned browser commands share these writes and the existing records.
func (a *App) withOwnerInventoryTransaction(ctx context.Context, owner, fid string, write func(pgx.Tx) error) error {
	run := func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, fid); err != nil {
			return err
		}
		if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
			return err
		}
		var revision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&revision); err != nil {
			return err
		}
		before, err := readCurrentWornItems(ctx, tx, fid)
		if err != nil {
			return err
		}
		if err := write(tx); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fid).Scan(&revision); err != nil {
			return err
		}
		var audit string
		if err := tx.QueryRow(ctx, `SELECT COALESCE(current_setting('fluctlight.inventory.audit_id',true),'')`).Scan(&audit); err != nil {
			return err
		}
		return recordWearingTransitionTx(ctx, tx, fid, "owner_correction", firstString(audit, "owner:"+owner), revision, before, a.now())
	}
	if tx, ok := SurfaceDBTx(ctx); ok {
		return run(tx)
	}
	return withTransaction(ctx, a.DB.Pool(), run)
}
func clearUnavailableItemUseTx(ctx context.Context, tx pgx.Tx, fid, id, source string, at time.Time) error {
	var activity string
	err := tx.QueryRow(ctx, `DELETE FROM public.fluctlight_item_uses WHERE fluctlight_id=$1 AND item_id=$2 RETURNING activity`, fid, id).Scan(&activity)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_item_use_events(id,fluctlight_id,item_id,operation,activity,occurred_at,source_ref,item_revision) SELECT $1::varchar,$2::varchar,$3::varchar,'stop',$4::text,$5::timestamptz,$6::text,revision FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$2 AND id=$3`, randomID("item_use_invalidation_"), fid, id, activity, at, source)
	return err
}

const inventoryItemReadProjection = `to_jsonb(i)||jsonb_build_object('source_world_item_snapshot',(SELECT r.after_value FROM public.world_item_revisions r WHERE r.item_id=i.source_world_item_id AND r.revision=i.source_world_item_revision),'source_verified',` + inventorySourceVerifiedSQL + `,'source_event',(SELECT to_jsonb(e) FROM public.life_events e WHERE e.id=i.source_ref AND e.fluctlight_id=i.fluctlight_id),'worn',(SELECT to_jsonb(w) FROM public.fluctlight_worn_items w WHERE w.fluctlight_id=i.fluctlight_id AND w.item_id=i.id),'use',(SELECT to_jsonb(u) FROM public.fluctlight_item_uses u WHERE u.fluctlight_id=i.fluctlight_id AND u.item_id=i.id),'media',COALESCE((SELECT jsonb_agg(jsonb_build_object('asset_id',m.asset_id,'created_at',m.created_at)) FROM public.media_references m JOIN public.media_assets a ON a.id=m.asset_id AND a.status='ready' AND a.owner_fluctlight_id=i.fluctlight_id WHERE m.owner_fluctlight_id=i.fluctlight_id AND m.target_type='item' AND m.target_id=i.id),'[]'::jsonb))`

func readInventoryItemWith(ctx context.Context, q lifeContextQuerier, fid, id string) (map[string]any, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT `+inventoryItemReadProjection+` FROM public.fluctlight_wardrobe_items i WHERE i.fluctlight_id=$1 AND i.id=$2`, fid, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}
func (a *App) OwnerInventoryItem(ctx context.Context, owner, fid, id string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	return readInventoryItemWith(ctx, a.DB.Pool(), fid, id)
}
func (a *App) OwnerInventoryList(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	term, kind, ownership, availability, state := strings.TrimSpace(q.Get("q")), q.Get("kind"), q.Get("ownership"), q.Get("availability"), q.Get("state")
	if state == "" {
		state = "current"
	}
	if len([]rune(term)) > 512 || (kind != "" && kind != "wearable" && kind != "object") || (ownership != "" && ownership != "owned" && ownership != "borrowed" && ownership != "unknown") || (availability != "" && availability != "available" && availability != "unavailable" && availability != "lost") || (state != "current" && state != "archived" && state != "worn" && state != "used") {
		return nil, invalid("invalid inventory filter")
	}
	scope := stableDigest(jsonString([]any{"inventory", owner, fid, term, kind, ownership, availability, state}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid inventory cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT i.id,i.acquired_at,`+inventoryItemReadProjection+` FROM public.fluctlight_wardrobe_items i WHERE i.fluctlight_id=$1 AND ($2='' OR strpos(lower(i.description),lower($2))>0) AND ($3='' OR i.item_kind=$3) AND ($4='' OR i.ownership=$4) AND ($5='' OR i.availability=$5) AND (($6='archived' AND i.archived_at IS NOT NULL) OR ($6<>'archived' AND i.archived_at IS NULL)) AND ($6<>'worn' OR EXISTS(SELECT 1 FROM public.fluctlight_worn_items w WHERE w.fluctlight_id=i.fluctlight_id AND w.item_id=i.id)) AND ($6<>'used' OR EXISTS(SELECT 1 FROM public.fluctlight_item_uses u WHERE u.fluctlight_id=i.fluctlight_id AND u.item_id=i.id)) AND ($7='' OR (i.acquired_at,i.id)<($8,$7)) ORDER BY i.acquired_at DESC,i.id DESC LIMIT $9`, fid, term, kind, ownership, availability, state, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	page, err := readGovernanceRows(rows, scope, limit)
	rows.Close()
	if err != nil {
		return nil, err
	}
	var revision int
	var complete bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT revision,inventory_complete FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fid).Scan(&revision, &complete); err != nil {
		return nil, err
	}
	page["revision"], page["inventory_complete"] = revision, complete
	return page, nil
}
func (a *App) OwnerInventoryHistory(ctx context.Context, owner, fid, id string, q url.Values) (map[string]any, error) {
	if _, err := a.OwnerInventoryItem(ctx, owner, fid, id); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"inventory-history", owner, fid, id}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid inventory history cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `WITH records AS (
 SELECT h.id,h.occurred_at,to_jsonb(h)||jsonb_build_object('kind','state_revision','owner_audit',(SELECT to_jsonb(a) FROM public.owner_surface_audits a WHERE a.id=h.audit_id AND a.owner_actor_id=$5)) AS value FROM public.fluctlight_inventory_history h WHERE h.fluctlight_id=$1 AND h.item_id=$2
 UNION ALL SELECT e.id,e.occurred_at,jsonb_build_object('id',e.id,'kind','use_event','operation',e.operation,'occurred_at',e.occurred_at,'source_ref',e.source_ref,'payload',to_jsonb(e)) FROM public.fluctlight_item_use_events e WHERE e.fluctlight_id=$1 AND e.item_id=$2
 ) SELECT id,occurred_at,value FROM records WHERE ($3='' OR (occurred_at,id)<($4,$3)) ORDER BY occurred_at DESC,id DESC LIMIT $6`, fid, id, cursor.ID, cursor.Time, owner, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}
func (a *App) OwnerInventoryCommand(ctx context.Context, owner, fid, id, operation string, body map[string]any) (map[string]any, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("inventory_transaction_required")
	}
	expected, ok := parseRevision(body["expected_revision"])
	reason := strings.TrimSpace(stringValue(body["reason"]))
	if !ok || expected < 0 || reason == "" || len([]rune(reason)) > 1024 {
		return nil, invalid("inventory revision and reason required")
	}
	if err := lockLifeContextTx(ctx, tx, fid); err != nil {
		return nil, err
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	var current int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&current); err != nil {
		return nil, err
	}
	if expected != current {
		return nil, conflict("wardrobe", expected, current)
	}
	if operation != "add" {
		expectedItem, ok := parseRevision(body["expected_item_revision"])
		if !ok || expectedItem < 1 {
			return nil, invalid("item revision required")
		}
		var revision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 FOR UPDATE`, fid, id).Scan(&revision); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		if revision != expectedItem {
			return nil, conflict("inventory_item", expectedItem, revision)
		}
	}
	item := mapValue(body["item"])
	if operation == "add" || operation == "update" {
		if err := validateOwnerInventoryItem(item); err != nil {
			return nil, err
		}
	}
	if operation == "add" {
		id = randomID("owner_item_")
		item = cloneMap(item)
		item["id"] = id
	}
	audit, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, fid, "inventory_"+operation, reason, id)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('fluctlight.inventory.audit_id',$1,true)`, audit); err != nil {
		return nil, err
	}
	switch operation {
	case "add":
		_, err = a.AddWardrobeItems(ctx, owner, fid, item)
	case "update":
		_, err = a.UpdateWardrobeItem(ctx, owner, fid, id, item)
	case "archive":
		_, err = a.DeleteWardrobeItem(ctx, owner, fid, id)
	default:
		return nil, invalid("invalid inventory command")
	}
	if err != nil {
		return nil, err
	}
	updated, err := readInventoryItemWith(ctx, tx, fid, id)
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fid).Scan(&current); err != nil {
		return nil, err
	}
	return map[string]any{"item": updated, "revision": current, "audit_id": audit, "operation": operation}, nil
}
func validateOwnerInventoryItem(item map[string]any) error {
	for _, key := range []string{"item_kind", "category", "slot", "description", "ownership", "availability"} {
		if _, ok := item[key].(string); !ok {
			return invalid("all inventory fields must be explicit text")
		}
	}
	for key := range item {
		if key != "item_kind" && key != "category" && key != "slot" && key != "description" && key != "ownership" && key != "availability" {
			return invalid("unsupported inventory field")
		}
	}
	kind, slot, category, description := stringValue(item["item_kind"]), strings.TrimSpace(stringValue(item["slot"])), strings.TrimSpace(stringValue(item["category"])), strings.TrimSpace(stringValue(item["description"]))
	if category == "" || len([]rune(category)) > 64 || description == "" || len([]rune(description)) > 512 || len([]rune(slot)) > 64 || (kind != "object" && kind != "wearable") || (kind == "object" && slot != "") || (kind == "wearable" && slot == "") {
		return invalid("invalid inventory item fields")
	}
	ownership, availability := stringValue(item["ownership"]), stringValue(item["availability"])
	if (ownership != "owned" && ownership != "borrowed" && ownership != "unknown") || (availability != "available" && availability != "unavailable" && availability != "lost") {
		return invalid("inventory ownership and availability required")
	}
	return nil
}
