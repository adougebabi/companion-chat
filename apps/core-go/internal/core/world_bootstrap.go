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
			map[string]any{"key": "home", "name": "居所", "places": []any{
				map[string]any{"key": "living", "name": "起居室", "description": "明亮的起居室设有沙发、茶几和书架，窗边留着适合阅读与闲聊的角落。"},
				map[string]any{"key": "bedroom", "name": "卧室", "description": "安静的卧室里摆着床、衣柜和床头灯，柔和的窗帘隔开外面的光线。"},
				map[string]any{"key": "kitchen", "name": "厨房", "description": "整洁的厨房配有灶台、水槽和料理台，餐具与常用厨具各有收纳的位置。"},
				map[string]any{"key": "bathroom", "name": "浴室", "description": "干净的浴室设有洗手台、镜子和淋浴区，毛巾与洗护用品收在置物架上。"},
			}},
			map[string]any{"key": "shops", "name": "商店", "places": []any{
				map[string]any{"key": "grocer", "name": "杂货店", "description": "街角的小店陈列着食品与日用品，货架之间留着窄窄的过道，收银台靠近门口。"},
				map[string]any{"key": "food", "name": "小吃店", "description": "临街的小吃店摆着几张木桌，开放的料理台前挂着菜单，店内弥漫着热食的香气。"},
			}},
			map[string]any{"key": "public", "name": "公共空间", "places": []any{
				map[string]any{"key": "square", "name": "广场", "description": "开阔的步行广场围着树荫和长椅，几条小路从这里通向附近的街区。"},
				map[string]any{"key": "meeting", "name": "会客处", "description": "温暖的公共会客空间里摆着圆桌和座椅，入口旁设有饮水处，适合坐下交谈。"},
			}},
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
	for _, name := range []string{"周予安", "林晚晴"} {
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
