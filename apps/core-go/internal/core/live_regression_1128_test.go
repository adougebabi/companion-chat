package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Opt-in real-model regression for the 2026-11-27/28 changes. Run with
// FLUCTLIGHT_LIVE_PROVIDER_TEST=1, FLUCTLIGHT_LIVE_PROVIDER_URL/MODEL(/API_KEY)
// and GO_CORE_TEST_DATABASE_URL. It logs what the model actually did; the
// assertions cover only the contract this code owns.

func liveWireCalls(rows []map[string]any) []map[string]any {
	calls := []map[string]any{}
	for _, row := range rows {
		for _, choice := range arrayValue(mapValue(row["response"])["choices"]) {
			for _, raw := range arrayValue(mapValue(mapValue(choice)["message"])["tool_calls"]) {
				function := mapValue(mapValue(raw)["function"])
				calls = append(calls, map[string]any{"name": strings.ReplaceAll(stringValue(function["name"]), "_", "."), "arguments": decodeObject([]byte(stringValue(function["arguments"])))})
			}
		}
	}
	return calls
}

func liveRequestsContaining(rows []map[string]any, marker string) []map[string]any {
	found := []map[string]any{}
	for _, row := range rows {
		if strings.Contains(jsonString(mapValue(row["request"])["messages"]), marker) {
			found = append(found, row)
		}
	}
	return found
}

func setupLiveDirectChat(t *testing.T, suffix string) (context.Context, *PostgresRepository, *App, *r7LiveWire, string, string, string) {
	t.Helper()
	baseURL, model := liveProviderConfig(t)
	ctx, repo, _, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, suffix, newFakeProviderRouter())
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	endpoint := "mixed-endpoint-" + suffix
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, endpoint, baseURL); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id=$2,timeout_seconds=180 WHERE provider_endpoint_id=$1`, endpoint, model); err != nil {
		t.Fatal(err)
	}
	wire := &r7LiveWire{}
	return ctx, repo, newTestApp(t, repo, wire), wire, owner, fid, conversation
}

func TestLive1128PhotoDeclaresInFrameAndKeepsExpression(t *testing.T) {
	ctx, repo, app, wire, owner, fid, conversation := setupLiveDirectChat(t, "live-1128-photo")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_visual_identities(id,fluctlight_id,status,identity_snapshot) VALUES('live-1128-identity',$1,'active','{"identity":{"name":"Fixture identity"}}')`, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://comfy.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
		t.Fatal(err)
	}
	liveCtx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	result, err := app.HandleTurn(liveCtx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "今天升职了！拍张你现在开心的自拍给我看看嘛", "idempotency_key": "live-1128-photo", "attachment_refs": []any{}})
	if err != nil {
		t.Fatalf("turn failed: %v", err)
	}
	t.Logf("reply: %s", stringValue(result.Assistant["text"]))
	var photo map[string]any
	for _, call := range liveWireCalls(wire.rows) {
		t.Logf("tool call %s %s", call["name"], jsonString(call["arguments"]))
		if call["name"] == "media.image.generate" {
			photo = mapValue(call["arguments"])
		}
	}
	if photo == nil {
		t.Fatal("the model did not call media.image.generate")
	}
	if stringValue(mapValue(photo["capture"])["in_frame"]) == "" {
		t.Fatalf("photo call lacks capture.in_frame: %v", photo)
	}
	var intent string
	if err := repo.Pool().QueryRow(liveCtx, `SELECT id FROM public.media_intents WHERE conversation_id=$1 AND owner_fluctlight_id=$2 ORDER BY created_at DESC LIMIT 1`, conversation, fid).Scan(&intent); err != nil {
		t.Fatal(err)
	}
	before := len(wire.rows)
	_, mediaErr := app.ProcessMediaIntent(liveCtx, intent) // ComfyUI is unreachable; the prompt pass runs first
	t.Logf("media processing ended with %v (expected: renderer unavailable)", mediaErr)
	media := wire.rows[before:]
	parses := liveRequestsContaining(media, "解析本次原始图像意图")
	prompts := liveRequestsContaining(media, "你负责把本次媒体意图编译")
	for _, row := range prompts {
		for _, choice := range arrayValue(mapValue(row["response"])["choices"]) {
			t.Logf("media prompt output: %s", stringValue(mapValue(mapValue(choice)["message"])["content"]))
		}
	}
	succeeded := 0
	for _, row := range prompts {
		if intValue(row["http_status"]) == 200 {
			succeeded++
		}
	}
	if len(parses) != 0 || succeeded == 0 {
		t.Fatalf("want only the prompt pass: subject parses=%d prompt passes=%d (media model rows=%d)", len(parses), len(prompts), len(media))
	}
}

func TestLive1128GoShoppingNowStartsAndMoves(t *testing.T) {
	ctx, repo, app, wire, owner, fid, conversation := setupLiveDirectChat(t, "live-1128-shop")
	if _, err := app.DispatchWorldSurface(ctx, "PUT", "/internal/fluctlights/"+fid+"/world", nil, owner, map[string]any{"name": "日常世界", "creation_preset": "daily", "expected_revision": 0, "reason": "live shopping", "idempotency_key": "live-1128-world"}); err != nil {
		t.Fatal(err)
	}
	liveCtx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	result, err := app.HandleTurn(liveCtx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "天冷了，你现在就去商场买两件厚外套吧，别等了，现在就出发！买不到也没关系，去了告诉我就行。", "idempotency_key": "live-1128-shop", "attachment_refs": []any{}})
	defer func() {
		if drows, derr := repo.Pool().Query(context.Background(), `SELECT event_type,payload::text FROM public.diagnostic_events WHERE event_type LIKE 'life.activity%' ORDER BY created_at LIMIT 5`); derr == nil {
			for drows.Next() {
				var kind, payload string
				_ = drows.Scan(&kind, &payload)
				t.Logf("diagnostic %s %.600s", kind, payload)
			}
			drows.Close()
		} else {
			t.Logf("diagnostic query: %v", derr)
		}
		rows, qerr := repo.Pool().Query(context.Background(), `SELECT i.id,i.status,i.trigger::text,COALESCE(item.start_at::text,''),COALESCE(item.id,'') FROM public.fluctlight_intentions i LEFT JOIN public.life_schedule_items item ON item.intention_id=i.id WHERE i.fluctlight_id=$1`, fid)
		if qerr == nil {
			for rows.Next() {
				var id, status, trigger, start, item string
				_ = rows.Scan(&id, &status, &trigger, &start, &item)
				t.Logf("intention %s status=%s trigger=%s item=%s start=%s", id, status, trigger, item, start)
			}
			rows.Close()
		}
		for i, row := range wire.rows {
			for _, choice := range arrayValue(mapValue(row["response"])["choices"]) {
				t.Logf("round %d content=%.300s", i+1, stringValue(mapValue(mapValue(choice)["message"])["content"]))
			}
		}
	}()
	if err != nil {
		for _, call := range liveWireCalls(wire.rows) {
			t.Logf("tool call %s %s", call["name"], jsonString(call["arguments"]))
		}
		for _, row := range wire.rows {
			for _, raw := range arrayValue(mapValue(row["request"])["messages"]) {
				if message := mapValue(raw); message["role"] == "tool" {
					t.Logf("tool result: %.600s", stringValue(message["content"]))
				}
			}
		}
		t.Fatalf("turn failed: %v", err)
	}
	t.Logf("reply: %s", stringValue(result.Assistant["text"]))
	var shop map[string]any
	for _, call := range liveWireCalls(wire.rows) {
		t.Logf("tool call %s %s", call["name"], jsonString(call["arguments"]))
		if call["name"] == "intention.schedule" {
			shop = mapValue(call["arguments"])
		}
	}
	for _, row := range wire.rows {
		for _, raw := range arrayValue(mapValue(row["request"])["messages"]) {
			if message := mapValue(raw); message["role"] == "tool" {
				t.Logf("tool result: %.600s", stringValue(message["content"]))
			}
		}
	}
	if shop == nil || shop["start_now"] != true {
		t.Fatalf("the model did not schedule shopping now: %v", shop)
	}
	_, life, err := app.readLifeContextSnapshotAt(liveCtx, fid, app.now())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("life after: source=%v kind=%v activity=%v location=%v", life["source"], life["event_kind"], life["activity"], life["location"])
	if life["source"] != "event" {
		t.Fatalf("going now did not move the character: %v", life)
	}
	_ = repo
}
