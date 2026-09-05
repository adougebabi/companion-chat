package core

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestProviderRedisKeysKeepBindingsSeparate(t *testing.T) {
	pending, processing, sequence := providerRedisKeys("generic_llm")
	if pending != "fluctlight:llm:generic_llm:pending" || processing != "fluctlight:llm:generic_llm:processing" || sequence != "fluctlight:llm:generic_llm:sequence" {
		t.Fatalf("generic keys = %q %q %q", pending, processing, sequence)
	}
	embeddingPending, _, _ := providerRedisKeys("embedding")
	if embeddingPending == pending {
		t.Fatal("embedding and generic queues share a pending key")
	}
}

func TestProviderRedisScoreKeepsPriorityBeforeFIFO(t *testing.T) {
	if providerRedisScore(100, 999) >= providerRedisScore(90, 1) {
		t.Fatal("higher priority did not sort ahead of lower priority")
	}
	if providerRedisScore(90, 1) >= providerRedisScore(90, 2) {
		t.Fatal("equal priority did not preserve FIFO sequence")
	}
}

func TestProviderRedisSlotHonorsPriorityAndLease(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Skipf("Redis integration test requires a local listener: %v", err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	provider := &ProviderClient{redis: client, redisID: "test-provider"}

	firstRelease, enabled, err := provider.acquireProviderRedisSlot(context.Background(), "reflection", 70, 1, "run-1")
	if err != nil || !enabled {
		t.Fatalf("first acquire = enabled:%v err:%v", enabled, err)
	}
	defer firstRelease()

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if _, enabled, err := provider.acquireProviderRedisSlot(ctx, "reply", 100, 1, "run-2"); !enabled || err == nil {
		t.Fatalf("blocked acquire = enabled:%v err:%v, want context cancellation", enabled, err)
	}

	firstRelease()
	secondRelease, enabled, err := provider.acquireProviderRedisSlot(context.Background(), "reply", 100, 1, "run-3")
	if err != nil || !enabled {
		t.Fatalf("second acquire = enabled:%v err:%v", enabled, err)
	}
	secondRelease()
	if got := client.ZCard(context.Background(), "fluctlight:llm:generic_llm:processing").Val(); got != 0 {
		t.Fatalf("processing members after release = %d", got)
	}
}
