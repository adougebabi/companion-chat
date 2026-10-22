package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Temporary task fixture: only the disposable task database may be populated.
func TestSeedPenRuntime(t *testing.T) {
	if os.Getenv("FLUCTLIGHT_PEN_SEED") != "1" {
		t.Skip("explicit task fixture only")
	}
	connection := os.Getenv("CORE_GO_DATABASE_URL")
	if !strings.Contains(connection, "127.0.0.1:15432/pen_test?") {
		t.Fatal("requires disposable task database")
	}
	ctx := context.Background()
	repo, err := NewPostgresRepository(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	exec := func(sql string, args ...any) (pgconn.CommandTag, error) {
		for i, value := range args {
			if b, ok := value.([]byte); ok {
				args[i] = string(b)
			}
		}
		return repo.Pool().Exec(ctx, sql, append([]any{pgx.QueryExecModeSimpleProtocol}, args...)...)
	}
	app, err := NewApp(repo, os.Getenv("FLUCTLIGHT_SETTINGS_KEY"), os.Getenv("FLUCTLIGHT_CORE_SERVICE_KEY"), os.Getenv("S3_ENDPOINT"), "us-east-1", os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), os.Getenv("S3_BUCKET"), false)
	if err != nil {
		t.Fatal(err)
	}
	owner := ""
	available, err := app.SetupAvailable(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if available {
		token := "setup_pen_runtime_only"
		digest := sha256.Sum256([]byte(token))
		if _, err = exec(`INSERT INTO public.owner_setup_tokens(id,token_hash,expires_at) VALUES('pen_setup',$1,now()+interval '30 minutes')`, hex.EncodeToString(digest[:])); err != nil {
			t.Fatal(err)
		}
		owner, _, err = app.Setup(ctx, token, "pen-test-only-password")
		if err != nil {
			t.Fatal(err)
		}
	} else {
		if err = repo.Pool().QueryRow(ctx, `SELECT human_actor_id FROM public.owner_accounts LIMIT 1`).Scan(&owner); err != nil {
			t.Fatal(err)
		}
	}
	const fid = "pen_runtime_lan"
	foundation := map[string]any{"identity": map[string]any{"name": "隔离验收 · 林岚", "timezone": "Asia/Shanghai", "occupation": "插画师", "background": "居住在沿海小城，喜欢阅读与绘画。"}, "personality_system": map[string]any{"active_profile_id": "default", "profiles": []any{map[string]any{"id": "default", "name": "林岚", "description": "表达温和，喜欢认真听完对方的话。", "habits": "闲暇时喜欢在海边读书，工作后会整理画具。"}}}, "behavioral_policy": map[string]any{"tone": "自然温和"}, "life_profile": map[string]any{"appearance": map[string]any{"physical_features": map[string]any{"hair_length": "及肩", "hair_color": "深棕"}, "wardrobe_items": []any{map[string]any{"category": "top", "slot": "upper_body", "clothing_type": "upper_body", "description": "浅灰色针织衫", "ownership": "owned", "available": true, "currently_worn": true}, map[string]any{"category": "bottom", "slot": "lower_body", "clothing_type": "lower_body", "description": "深蓝直筒牛仔裤", "ownership": "owned", "available": true, "currently_worn": true}, map[string]any{"category": "shoes", "slot": "shoes", "clothing_type": "shoes", "description": "白色帆布鞋", "ownership": "owned", "available": true, "currently_worn": true}, map[string]any{"category": "accessory", "slot": "accessory", "clothing_type": "accessory", "description": "米白围巾", "ownership": "owned", "available": true, "currently_worn": true}, map[string]any{"category": "accessory", "slot": "accessory", "clothing_type": "accessory", "description": "银色耳钉", "ownership": "owned", "available": true, "currently_worn": true}, map[string]any{"category": "dress", "slot": "full_body", "clothing_type": "lower_body", "description": "深绿连衣裙", "ownership": "owned", "available": true, "currently_worn": false}, map[string]any{"category": "top", "slot": "upper_body", "clothing_type": "upper_body", "description": "白色衬衫", "ownership": "owned", "available": true, "currently_worn": false}}}, "life_habits": []any{}, "preferences": map[string]any{"likes": "海风、旧书与安静的下午"}}}
	identity := mapValue(foundation["identity"])
	if _, err = exec(`INSERT INTO public.actors(id,actor_type,status) VALUES($1,'fluctlight','active') ON CONFLICT DO NOTHING; INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active',$3,$4,'{}',$5,$6,'{"source":"task-owned-controlled-fixture"}') ON CONFLICT DO NOTHING`, fid, owner, jsonBytes(foundation), jsonBytes(identity), jsonBytes(foundation["behavioral_policy"]), jsonBytes(foundation["life_profile"])); err != nil {
		t.Fatal(err)
	}
	if _, err = exec(`INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'default',0) ON CONFLICT DO NOTHING; INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"平静","intensity":0.2}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now()) ON CONFLICT DO NOTHING; INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1) ON CONFLICT DO NOTHING`, fid); err != nil {
		t.Fatal(err)
	}
	if err = withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error { return initializeEffectiveLifeTx(ctx, tx, fid, foundation) }); err != nil {
		t.Fatal(err)
	}
	seedLegacyTestWorkingPersonas(t, app)
	if _, err = exec(`UPDATE public.fluctlight_wardrobe_states SET outfit_tags=ARRAY['日常','工作','外出'] WHERE fluctlight_id=$1; UPDATE public.fluctlight_wardrobe_items SET seasons=ARRAY['spring','autumn'] WHERE fluctlight_id=$1`, fid); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err = repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_schedules WHERE fluctlight_id=$1 AND local_date=(now() AT TIME ZONE 'Asia/Shanghai')::date AND status='accepted')`, fid).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		life := currentLifeForTest(t, ctx, app, fid, app.now())
		location, _ := time.LoadLocation("Asia/Shanghai")
		local := app.now().In(location)
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		items := []any{}
		for _, part := range []struct {
			from, to        int
			activity, scene string
		}{{0, 9, "休息与晨间整理", "住处"}, {9, 13, "绘画工作", "工作室"}, {13, 17, "读书与自由安排", "海边广场"}, {17, 24, "晚餐与休息", "住处"}} {
			items = append(items, map[string]any{"start_at": day.Add(time.Duration(part.from) * time.Hour).Format(time.RFC3339), "end_at": day.Add(time.Duration(part.to) * time.Hour).Format(time.RFC3339), "activity": part.activity, "scene": part.scene, "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5})
		}
		if _, err = app.AcceptSchedule(ctx, owner, fid, map[string]any{"local_date": day.Format("2006-01-02"), "timezone": "Asia/Shanghai", "items": items, "reschedule_policy": map[string]any{}, "expected_revision": 0, "expected_life_context_revision": life["context_revision"], "idempotency_key": "task-fixture-today", "evidence_refs": []any{"task-owned-controlled-fixture"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = exec(`INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES('pen_runtime_conversation',$1,'隔离验收的交流') ON CONFLICT DO NOTHING; INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES('pen_runtime_conversation',1) ON CONFLICT DO NOTHING; INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES('pen_runtime_conversation',$1,'owner','active'),('pen_runtime_conversation',$2,'member','active') ON CONFLICT DO NOTHING`, owner, fid); err != nil {
		t.Fatal(err)
	}
	fmt.Println("Task fixture prepared:", fid)
}
