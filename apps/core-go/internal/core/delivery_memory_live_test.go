package core

import (
	"context"
	"os"
	"testing"
	"time"
)

// Opt-in browser fixture produced by the real Owner command, not by SQL or a
// simulated model. It is deliberately labelled and forbidden in any other DB.
// This is management integration evidence, never cognition-production evidence.
func TestDeliveryMemoryOwnerCommandFixture(t *testing.T) {
	if os.Getenv("FLUCTLIGHT_DELIVERY_MEMORY_FIXTURE") != "1" {
		t.Skip("explicit isolated delivery fixture only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo, err := NewPostgresRepository(ctx, os.Getenv("CORE_GO_DATABASE_URL"))
	if err != nil {
		t.Fatal("delivery database unavailable")
	}
	defer repo.Close()
	var database string
	if err = repo.Pool().QueryRow(ctx, `SELECT current_database()`).Scan(&database); err != nil || database != "fluctlight_delivery_1015" {
		t.Fatal("fixture requires named isolated delivery database")
	}
	owner, fid := "human_fd44c910e182f4ec295e1228aa06cd34", "fluctlight_1cc52b2d06be229916c79600caec9b17"
	var message, conversation string
	var at time.Time
	if err = repo.Pool().QueryRow(ctx, `SELECT id,conversation_id,created_at FROM public.conversation_messages WHERE author_actor_id=$1 AND text LIKE '这是隔离环境的功能验证。%' ORDER BY created_at DESC LIMIT 1`, owner).Scan(&message, &conversation, &at); err != nil {
		t.Fatal("actual accepted test message is required")
	}
	app := &App{DB: repo}
	result, err := app.RecordMemory(ctx, fid, owner, map[string]any{"type": "semantic", "content": "MEMORY-LIVE-1016：所有者已发起一次隔离环境功能验证。这是管理联调测试记录，不代表虚构的共同生活经历。", "confidence": 1.0, "importance": 0.1, "emotional_significance": 0.0, "actor_refs": []any{owner}, "conversation_id": conversation, "event_refs": []any{}, "evidence_refs": []any{"message:" + message}, "visibility": "owner", "occurred_at": at.Format(time.RFC3339Nano), "idempotency_key": "MEMORY-LIVE-1016"})
	if err != nil {
		t.Fatal(err)
	}
	read, err := NewMemoryService(repo.Pool(), app).ReadManaged(ctx, owner, stringValue(result["id"]))
	if err != nil || read["provenance_status"] != "verified" {
		t.Fatalf("fixture readback %#v %v", read, err)
	}
	t.Logf("owner_command_fixture memory_id=%s revision=%v; source is actual test message; no model production claimed", result["id"], result["revision"])
}
