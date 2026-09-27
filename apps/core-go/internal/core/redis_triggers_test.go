package core

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisTriggerSchedulingUsesStableKeysAndTTL(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Skipf("Redis integration test requires a local listener: %v", err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	app := &App{Redis: client}
	if err := app.scheduleReflectionTrigger(context.Background(), "fl-1", 30*time.Second); err != nil {
		t.Fatal(err)
	}
	server.FastForward(5 * time.Minute)
	if err := app.scheduleReflectionTrigger(context.Background(), "fl-1", time.Second); err != nil {
		t.Fatal(err)
	}
	if err := app.scheduleWakeUpTrigger(context.Background(), "fl-1", 60); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		key   string
		value string
		min   time.Duration
	}{
		{key: reflectionTriggerPrefix + "fl-1", value: "fl-1", min: 20 * time.Second},
		{key: wakeUpTriggerPrefix + "fl-1", value: "fl-1", min: 50 * time.Second},
	} {
		if got := client.Get(context.Background(), test.key).Val(); got != test.value {
			t.Fatalf("%s value = %q, want %q", test.key, got, test.value)
		}
		if ttl := client.TTL(context.Background(), test.key).Val(); ttl < test.min {
			t.Fatalf("%s TTL = %s, want at least %s", test.key, ttl, test.min)
		}
		if test.key == reflectionTriggerPrefix+"fl-1" {
			if ttl := client.TTL(context.Background(), test.key).Val(); ttl > reflectionQuietPeriod {
				t.Fatalf("%s TTL = %s, must stay within fixed quiet period %s", test.key, ttl, reflectionQuietPeriod)
			}
		}
	}
}

func TestReflectionQuietPeriodIsFixedTenMinutes(t *testing.T) {
	if got, want := (&App{}).reflectionDelay(context.Background()), reflectionQuietPeriod; got != want {
		t.Fatalf("reflection delay = %s, want fixed %s", got, want)
	}
	if got, want := wakeUpAfterCognitionDelay(30*60), 10*time.Minute; got != want {
		t.Fatalf("wake-up delay after cognition = %s, want %s", got, want)
	}
}

func TestWakeUpIdleClockKeepsAbsoluteTenThirtyAndRecurringPhases(t *testing.T) {
	t0 := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	clock := wakeUpIdleClock{Epoch: "message-1", Since: t0, Slot: 0}
	for _, expected := range []time.Duration{10 * time.Minute, 30 * time.Minute, 60 * time.Minute, 90 * time.Minute} {
		if got := wakeUpIdleDue(clock, 1800); !got.Equal(t0.Add(expected)) {
			t.Fatalf("slot %d due=%s, want %s", clock.Slot, got, t0.Add(expected))
		}
		clock = nextWakeUpIdleClock(clock, t0.Add(expected), 1800)
	}
	lateFirst := nextWakeUpIdleClock(wakeUpIdleClock{Epoch: "message-1", Since: t0, Slot: 0}, t0.Add(35*time.Minute), 1800)
	if lateFirst.Slot != 1 || !wakeUpIdleDue(lateFirst, 1800).Equal(t0.Add(30*time.Minute)) {
		t.Fatalf("late first wake-up skipped the required second phase: %#v", lateFirst)
	}
	lateSecond := nextWakeUpIdleClock(lateFirst, t0.Add(95*time.Minute), 1800)
	if lateSecond.Slot != 4 || !wakeUpIdleDue(lateSecond, 1800).Equal(t0.Add(120*time.Minute)) {
		t.Fatalf("late recurring wake-up burst instead of preserving absolute phase: %#v", lateSecond)
	}
}

func TestWakeUpExecutionRejectsChangedIdleEpochOrCycle(t *testing.T) {
	payload := map[string]any{"cycle": 3, "idle_epoch": "message-new"}
	if wakeUpExecutionMatches("running", payload, 3, "message-old") || wakeUpExecutionMatches("running", payload, 2, "message-new") || wakeUpExecutionMatches("superseded", payload, 3, "message-new") {
		t.Fatal("stale Wake-up execution remained executable")
	}
	if !wakeUpExecutionMatches("running", payload, 3, "message-new") {
		t.Fatal("current Wake-up execution was rejected")
	}
}

func TestCognitionFollowupsArmReflectionAndWakeUpWithIndependentTTLs(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Skipf("Redis integration test requires a local listener: %v", err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	app := &App{Redis: client}
	if err := app.scheduleCognitionFollowups(context.Background(), "fl-followup"); err != nil {
		t.Fatal(err)
	}
	reflectionTTL := client.TTL(context.Background(), reflectionTriggerPrefix+"fl-followup").Val()
	wakeTTL := client.TTL(context.Background(), wakeUpTriggerPrefix+"fl-followup").Val()
	if reflectionTTL < 9*time.Minute || reflectionTTL > reflectionQuietPeriod {
		t.Fatalf("Reflection TTL = %s, want approximately %s", reflectionTTL, reflectionQuietPeriod)
	}
	if wakeTTL < 9*time.Minute || wakeTTL > 10*time.Minute {
		t.Fatalf("WakeUp TTL = %s, want approximately 10m after cognition without a stored user epoch", wakeTTL)
	}
	server.FastForward(5 * time.Minute)
	if err := app.scheduleCognitionFollowups(context.Background(), "fl-followup"); err != nil {
		t.Fatal(err)
	}
	if refreshed := client.TTL(context.Background(), reflectionTriggerPrefix+"fl-followup").Val(); refreshed < 9*time.Minute {
		t.Fatalf("repeated cognition did not refresh Reflection TTL: %s", refreshed)
	}
	if refreshed := client.TTL(context.Background(), wakeUpTriggerPrefix+"fl-followup").Val(); refreshed < 9*time.Minute {
		t.Fatalf("repeated cognition did not refresh WakeUp TTL: %s", refreshed)
	}
}

func TestLifecycleProviderCancellationStopsWakeUpAndReflectionBeforeDatabaseWork(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Skipf("Redis integration test requires a local listener: %v", err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	app := &App{Redis: client}
	ctx := context.Background()

	wakeMarker := WakeUpProviderCancellationMarker("fl-cancel", 4)
	if err := app.RequestProviderCancellation(ctx, wakeMarker); err != nil {
		t.Fatal(err)
	}
	wakeResult, err := app.ProcessWakeUp(WithProviderCancellationKey(ctx, wakeMarker), "fl-cancel", 4)
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(wakeResult["status"]) != "cancelled" || stringValue(wakeResult["reason"]) != "superseded_by_cognition" {
		t.Fatalf("WakeUp cancellation result = %#v", wakeResult)
	}

	reflectionMarker := ReflectionProviderCancellationMarker("reflection-intent-cancel")
	if err := app.RequestProviderCancellation(ctx, reflectionMarker); err != nil {
		t.Fatal(err)
	}
	reflectionResult, err := app.ProcessReflection(WithProviderCancellationKey(ctx, reflectionMarker), "fl-cancel", "reflection-correlation")
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(reflectionResult["status"]) != "cancelled" || stringValue(reflectionResult["reason"]) != "superseded_by_cognition" {
		t.Fatalf("Reflection cancellation result = %#v", reflectionResult)
	}
}

func TestWakeUpRedisHintReturnsTransportFailure(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Skipf("Redis integration test requires a local listener: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: 0})
	server.Close()
	defer client.Close()
	app := &App{Redis: client}
	if err := app.scheduleWakeUpTrigger(context.Background(), "fl-transport", 60); err == nil {
		t.Fatal("WakeUp Redis hint transport failure was silently ignored")
	}
}

func TestNextWakeUpDuePreservesFixedCadence(t *testing.T) {
	previousDue := time.Date(2026, 9, 13, 1, 0, 0, 0, time.UTC)
	now := previousDue.Add(35 * time.Minute)
	if got, want := nextWakeUpDue(previousDue, now, 30*60), previousDue.Add(time.Hour); !got.Equal(want) {
		t.Fatalf("nextWakeUpDue() = %s, want fixed slot %s", got, want)
	}
	alreadyScheduled := now.Add(10 * time.Minute)
	if got := nextWakeUpDue(alreadyScheduled, now, 30*60); !got.Equal(alreadyScheduled) {
		t.Fatalf("future due changed from %s to %s", alreadyScheduled, got)
	}
}

func TestWorkerRunsPostgresWakeUpReleaseAndClockAudit(t *testing.T) {
	source, err := os.ReadFile("../../cmd/worker/main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Count(text, "ReleaseDueWakeUpIntents") < 2 || !strings.Contains(text, "AuditWakeUpClocks") {
		t.Fatal("Worker does not run startup/periodic WakeUp release and clock audit")
	}
}
