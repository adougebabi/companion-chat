package core

import (
	"context"
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
	app.scheduleWakeUpTrigger(context.Background(), "fl-1", 60)

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
