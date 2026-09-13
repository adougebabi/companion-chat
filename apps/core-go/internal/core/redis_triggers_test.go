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
	app.scheduleReflectionTrigger(context.Background(), "reflection_intent:turn-1", 30*time.Second)
	if err := app.scheduleWakeUpTrigger(context.Background(), "fl-1", 60); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		key   string
		value string
		min   time.Duration
	}{
		{key: reflectionTriggerPrefix + "reflection_intent:turn-1", value: "reflection_intent:turn-1", min: 20 * time.Second},
		{key: wakeUpTriggerPrefix + "fl-1", value: "fl-1", min: 50 * time.Second},
	} {
		if got := client.Get(context.Background(), test.key).Val(); got != test.value {
			t.Fatalf("%s value = %q, want %q", test.key, got, test.value)
		}
		if ttl := client.TTL(context.Background(), test.key).Val(); ttl < test.min {
			t.Fatalf("%s TTL = %s, want at least %s", test.key, ttl, test.min)
		}
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
