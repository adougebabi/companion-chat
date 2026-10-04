package core

import (
	"strings"
	"testing"
	"time"
)

func TestInstantTransportAndReferenceTimezonePreserveSameMoment(t *testing.T) {
	at, err := time.Parse(time.RFC3339Nano, "2026-10-03T19:26:18.123456789+08:00")
	if err != nil {
		t.Fatal(err)
	}
	shanghai, _ := time.LoadLocation("Asia/Shanghai")
	if got := formatInstant(at); got != "2026-10-03T11:26:18.123+00:00" {
		t.Fatal(got)
	}
	if got := formatLocalInstant(at, shanghai); got != "2026-10-03T19:26:18.123+08:00" {
		t.Fatal(got)
	}
	view := projectionTimeView(ContextProjection{AsOf: formatInstant(at), ReferenceTimezone: "Asia/Shanghai"})
	if view["actor_user_timezone"] != nil || view["actor_user_local_time"] != nil {
		t.Fatal("unknown user location was inferred", view)
	}
	if view["actor_self_local_time"] != "2026-10-03T19:26:18.123+08:00" {
		t.Fatal(view)
	}
	// Serialization must not modify the original high precision value.
	if at.Nanosecond() != 123456789 {
		t.Fatal(at)
	}
}

func TestRuntimeHistoryUsesOneReferenceZoneAndKeepsOriginalSpeech(t *testing.T) {
	projection := ContextProjection{ReferenceTimezone: "Asia/Shanghai", LifeContext: map[string]any{"timezone": "Asia/Shanghai"}, RecentMessages: []map[string]any{
		{"id": "a", "kind": "user", "text": "我在国外", "created_at": "2026-10-03T11:26:18Z", "sender_timezone": "America/Los_Angeles", "sequence": 1},
		{"id": "b", "kind": "assistant", "text": "知道了", "created_at": "2026-10-03T19:26:19+08:00", "sender_timezone": "Asia/Shanghai", "sequence": 2},
	}}
	fragments := recentPromptFragments(projection)
	for i, stamp := range []string{"2026-10-03T19:26:18.000+08:00", "2026-10-03T19:26:19.000+08:00"} {
		if !strings.Contains(stringValue(mapValue(fragments[i].Content)["content"]), stamp) {
			t.Fatal(fragments[i])
		}
	}
	if projection.RecentMessages[0]["text"] != "我在国外" {
		t.Fatal("raw speech changed")
	}
	zone, _ := time.LoadLocation("America/Los_Angeles")
	for _, tc := range []struct{ at, want string }{
		{"2026-03-08T09:59:00Z", "2026-03-08T01:59:00.000-08:00"},
		{"2026-03-08T10:01:00Z", "2026-03-08T03:01:00.000-07:00"},
		{"2026-11-01T08:30:00Z", "2026-11-01T01:30:00.000-07:00"},
		{"2026-11-01T09:30:00Z", "2026-11-01T01:30:00.000-08:00"},
	} {
		at, _ := time.Parse(time.RFC3339, tc.at)
		if got := formatLocalInstant(at, zone); got != tc.want {
			t.Fatalf("%s: %s", tc.at, got)
		}
	}
}

func TestShoppingUsesInjectedBusinessClockWithoutWaitingOrChangingAuditTime(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	// Advancing business time beyond the host clock exposes accidental time.Now
	// or SQL now() in selection, permission and result settlement.
	at := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Millisecond)
	fixture.app.Clock = func() time.Time { return at }
	started, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "clock-start", map[string]any{
		"kind": "virtual_shopping", "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "黑色短靴", "reason": "购买缺少的靴子",
	}))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatalf("start: %#v %v", started, err)
	}
	activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
	var startedAt, createdAt time.Time
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT started_at,created_at FROM public.fluctlight_life_activity_runs WHERE id=$1`, activityID).Scan(&startedAt, &createdAt); err != nil {
		t.Fatal(err)
	}
	if !startedAt.Equal(at) || !createdAt.Before(at.Add(-23*time.Hour)) {
		t.Fatalf("business=%s audit=%s", startedAt, createdAt)
	}
	router := setupVirtualActivityTestProvider(t, fixture, map[string]any{"status": "completed", "reason": "已完成购买", "acquired_item": map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"}})
	at = at.Add(14 * time.Minute)
	early, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "clock-early", map[string]any{}))
	if err != nil || early.Result.Status != "accepted" || router.requestCount("virtual_activity_result") != 0 {
		t.Fatalf("early: %#v %v", early, err)
	}
	at = at.Add(time.Minute)
	done, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "clock-finish", map[string]any{}))
	if err != nil || done.Result.Status != "completed" || stringValue(mapValue(done.Result.Output)["item_id"]) == "" {
		t.Fatalf("finish: %#v %v", done, err)
	}
	var itemCount, wornCount int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT (SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'),(SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2)`, fixture.fluctlightID, stringValue(mapValue(done.Result.Output)["item_id"])).Scan(&itemCount, &wornCount); err != nil || itemCount != 1 || wornCount != 0 {
		t.Fatalf("items=%d worn=%d err=%v", itemCount, wornCount, err)
	}
}

func TestScheduleWallTimeRejectsDSTGapAndSelectsEarlierFold(t *testing.T) {
	for _, tc := range []struct{ zone, day, gap, fold, want string }{
		{"America/Los_Angeles", "2026-03-08", "2026-03-08 02:30", "2026-11-01 01:30", "2026-11-01T08:30:00.000+00:00"},
		{"Europe/Berlin", "2026-03-29", "2026-03-29 02:30", "2026-10-25 02:30", "2026-10-25T00:30:00.000+00:00"},
	} {
		zone, _ := time.LoadLocation(tc.zone)
		day, _ := time.ParseInLocation("2006-01-02", tc.day, zone)
		if _, err := parseScheduleTimeInLocation(tc.gap, day, zone); err == nil {
			t.Fatal("DST gap normalized", tc)
		}
		fold, err := parseScheduleTimeInLocation(tc.fold, day, zone)
		if err != nil || formatInstant(fold) != tc.want {
			t.Fatalf("fold %s %v", fold, err)
		}
		again, _ := parseScheduleTimeInLocation(tc.fold, day, zone)
		if !again.Equal(fold) {
			t.Fatal("fold replay drifted")
		}
	}
}
