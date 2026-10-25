package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestR9LiveShoppingDurableResolutionAndSelectedWear(t *testing.T) {
	base, model := liveProviderConfig(t)
	f := seedWardrobeToolFixture(t)
	_, _, worldItem := seedR8Shop(t, f)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "r9-live-shop")
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url=$1 WHERE id='r9-live-shop'`, base); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id=$1,timeout_seconds=180 WHERE provider_endpoint_id='r9-live-shop'`, model); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, f.ownerID, f.fluctlightID, f.conversationID); err != nil {
		t.Fatal(err)
	}
	wire := &r7LiveWire{}
	if dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		wire.path = filepath.Join(dir, "r9-shopping-wire.json")
	}
	app := newTestApp(t, f.repository, wire)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: base, ModelID: model, Timeout: 180 * time.Second})
	f.ctx, f.app = ctx, app
	result, err := app.handleTurn(ctx, f.ownerID, f.conversationID, takeoverChainTurnPayload(f.fluctlightID, "去湖滨银泰衣橱店买那件燕麦色针织开衫吧，买到以后再换上，其他衣服保留。先看看店里目录，开始后告诉我。", "r9-shopping", "r9-shopping"), turnCallbacks{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("start reply:", stringValue(result.Assistant["text"]))
	var activity string
	var selected bool
	var timers, acquired int
	if err := f.repository.Pool().QueryRow(ctx, `SELECT id,result_json #>> '{request,wear_after_acquisition}'='true' FROM public.fluctlight_life_activity_runs WHERE fluctlight_id=$1 AND kind='virtual_shopping' AND status='in_progress'`, f.fluctlightID).Scan(&activity, &selected); err != nil || !selected {
		t.Fatal("model did not choose start and followup", err, selected)
	}
	if err := f.repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='life.activity.resolve'`, "activity_resolution:"+activity).Scan(&timers); err != nil || timers != 1 {
		t.Fatal(timers, err)
	}
	if err := f.repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE source_world_item_id=$1`, worldItem).Scan(&acquired); err != nil || acquired != 0 {
		t.Fatal("premature acquisition", acquired, err)
	}
	// Only elapsed time is controlled; the production resolution entrypoint calls
	// the real local model and performs actual SQL acquisition/selected wearing.
	forceVirtualActivityDue(t, f, activity)
	outcome, err := app.ResolveScheduledLifeActivity(ctx, activity)
	if err != nil || outcome["status"] != "completed" {
		t.Fatal(outcome, err)
	}
	var item string
	if err := f.repository.Pool().QueryRow(ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_world_item_id=$2`, f.fluctlightID, worldItem).Scan(&item); err != nil {
		t.Fatal(err)
	}
	worn := r6WornIDs(t, f)
	if !containsString(worn, item) || len(worn) != 3 {
		t.Fatal("actual acquired item not worn, or other clothes removed", item, worn)
	}
	if _, err := app.ResolveScheduledLifeActivity(ctx, activity); err != nil {
		t.Fatal(err)
	}
	t.Log("real acquisition and selected wearing committed; replay preserved", item)
}

func TestR9LiveSingleInitializationAndInitialSchedule(t *testing.T) {
	base, model := liveProviderConfig(t)
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "r9-live-structured")
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url=$1 WHERE id='r9-live-structured'`, base); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id=$1,timeout_seconds=180 WHERE provider_endpoint_id='r9-live-structured'`, model); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('initialization','r9-live-structured',$1,'structured_output',4096,180,'{}')`, model); err != nil {
		t.Fatal(err)
	}
	wire := &r7LiveWire{}
	if dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		wire.path = filepath.Join(dir, "r9-structured-wire.json")
	}
	app := newTestApp(t, f.repository, wire)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	value, err := app.RunInitializationTask(ctx, InitializationTaskInput{Description: "周屿，28岁男性，生活在杭州，时区Asia/Shanghai。设计师，沉静，喜欢阅读，日常穿白衬衫、深色长裤和帆布鞋。保持单一人格，不设定与用户的既有关系。"})
	if err != nil {
		t.Fatal("single initialization", err)
	}
	prepared, err := prepareInitializationResponse(value)
	if err != nil {
		t.Fatal("initialization contract", err)
	}
	if len(wire.rows) != 1 {
		t.Fatal("initialization was split or silently regenerated", len(wire.rows))
	}
	persona := mapValue(prepared["core_persona"])
	result, err := app.RunScheduleGenerationTask(ctx, ScheduleGenerationTaskInput{LocalDate: "2026-10-25", Timezone: "Asia/Shanghai", Identity: mapValue(persona["identity"]), LifeProfile: mapValue(persona["life_profile"])})
	if err != nil {
		t.Fatal("schedule generation", err)
	}
	schedule, err := normalizeScheduleResponse(result, "2026-10-25", "Asia/Shanghai")
	if err != nil || len(arrayValue(schedule["items"])) == 0 {
		t.Fatal("schedule contract", err, schedule)
	}
	for _, row := range wire.rows {
		request := mapValue(row["request"])
		if request["enable_thinking"] != false || request["max_tokens"] != nil || request["max_completion_tokens"] != nil {
			t.Fatal("incorrect physical transport")
		}
	}
	t.Log("one complete initialization and a contiguous full-day schedule validated", len(arrayValue(schedule["items"])))
}
