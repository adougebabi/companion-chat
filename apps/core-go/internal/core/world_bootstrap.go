package core

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func requireWorldCreationChoiceTx(ctx context.Context, tx pgx.Tx, fid string, body map[string]any) error {
	if preset, present := body["creation_preset"]; present {
		if preset != "daily" {
			return invalid("creationPreset must be daily")
		}
		var hasWorld bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_worlds WHERE owner_actor_id=(SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1))`, fid).Scan(&hasWorld); err != nil {
			return err
		}
		if hasWorld {
			return &SurfaceError{Status: 409, Code: "world_choice_required", Message: "已有生活世界，请明确选择加入"}
		}
	}
	return nil
}

// New worlds are usable configurations, never evidence of arrival or acquaintance.
// This runs only in the explicit creation transaction, never from character init,
// GETs, clock repair or a background data-cleanup pass.
func (a *App) bootstrapWorldTx(ctx context.Context, tx pgx.Tx, fid, world string, body map[string]any) error {
	if _, err := tx.Exec(ctx, `SELECT public.seed_world_event_starters($1)`, world); err != nil {
		return err
	}
	frame := mapValue(body["framework"])
	if len(frame) == 0 {
		frame = map[string]any{"schema": "world-framework.v1", "venues": []any{
			map[string]any{"key": "home", "name": "居所", "places": []any{map[string]any{"key": "living", "name": "起居室"}, map[string]any{"key": "bedroom", "name": "卧室"}, map[string]any{"key": "kitchen", "name": "厨房"}, map[string]any{"key": "bathroom", "name": "浴室"}}},
			map[string]any{"key": "shops", "name": "商店", "places": []any{map[string]any{"key": "grocer", "name": "杂货店"}, map[string]any{"key": "food", "name": "小吃店"}}},
			map[string]any{"key": "public", "name": "公共空间", "places": []any{map[string]any{"key": "square", "name": "广场"}, map[string]any{"key": "meeting", "name": "会客处"}}},
		}}
	}
	var places int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.world_places WHERE world_id=$1 AND status='active'`, world).Scan(&places); err != nil {
		return err
	}
	if places == 0 {
		if _, err := a.materializeWorldFrameworkTx(ctx, tx, fid, world, worldPublication{Body: map[string]any{"framework": frame}}); err != nil {
			return err
		}
	}
	var npcs int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.world_npcs WHERE world_id=$1 AND status='active'`, world).Scan(&npcs); err != nil {
		return err
	}
	if npcs > 0 {
		return nil
	}
	var owner, home string
	if err := tx.QueryRow(ctx, `SELECT owner_actor_id FROM public.life_worlds WHERE id=$1`, world).Scan(&owner); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM public.world_places WHERE world_id=$1 AND status='active' ORDER BY id LIMIT 1`, world).Scan(&home); err != nil {
		return err
	}
	// NPC sources keep the existing immutable Studio provenance contract. No schema
	// exception and no synthetic encounter/relationship are needed for base NPCs.
	for _, name := range []string{"当地居民", "常住居民"} {
		source, npc, actor := randomID("resource_"), randomID("npc_"), randomID("actor_npc_")
		definition := map[string]any{"role": "居民", "background": "这个世界的独立居民；尚无与角色相识或实际在场的记录。"}
		rec := surfaceRecord{ID: source, OwnerID: owner, Kind: "studio_resource", Status: "published", Revision: 1, Payload: map[string]any{"kind": "npc", "name": name, "body": definition, "draft_version": "v1", "published_version": "v1", "versions": []any{map[string]any{"version": "v1", "body": definition}}}}
		if err := (postgresSurfaceTx{tx}).Save(ctx, rec); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.studio_resource_versions(resource_id,version,owner_actor_id,kind,name,body) VALUES($1,'v1',$2,'npc',$3,$4)`, source, owner, name, jsonBytes(definition)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'npc','active')`, actor); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.world_npcs(id,world_id,actor_id,source_resource_id,source_version,name,role,background,home_place_id,position_mode) VALUES($1,$2,$3,$4,'v1',$5,'居民',$6,$7,'unknown')`, npc, world, actor, source, name, definition["background"], home); err != nil {
			return err
		}
	}
	return nil
}
