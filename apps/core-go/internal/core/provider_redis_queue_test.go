package core

import (
	"context"
	"os"
	"strconv"
	"strings"
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

func TestProviderRedisQueueScriptAgesBoundedWaitJobs(t *testing.T) {
	source, err := os.ReadFile("provider_redis_queue.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{"queued_at", "max_wait", "aged_score", "sequence"} {
		if !strings.Contains(text, required) {
			t.Fatalf("Provider Redis queue aging missing %q", required)
		}
	}
	if providerRedisAgedScore(10) >= providerRedisScore(100, 1) {
		t.Fatal("aged Provider job does not sort ahead of fresh priority jobs")
	}
}

func TestProviderRedisAgingSelectsOldLifecycleBeforeFreshPriority(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Skipf("Redis integration test requires a local listener: %v", err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	pendingKey, processingKey, _ := providerRedisKeys("generic_llm")
	now := time.Now().UnixMilli()
	owner := "provider-aging-test"
	lowID, highID := "aged-reflection", "fresh-reply"
	for _, item := range []struct {
		id       string
		priority int
		sequence int64
		queuedAt int64
	}{
		{id: lowID, priority: 70, sequence: 1, queuedAt: now - providerRedisMaximumWait.Milliseconds() - 1},
		{id: highID, priority: 100, sequence: 2, queuedAt: now},
	} {
		score := providerRedisScore(item.priority, item.sequence)
		jobKey := providerRedisQueuePrefix + ":job:" + item.id
		if err := client.HSet(context.Background(), jobKey, map[string]any{
			"score": score, "sequence": item.sequence, "queued_at": item.queuedAt,
			"status": "queued", "pending_owner": owner, "pending_until": now + providerRedisPendingTTL.Milliseconds(),
		}).Err(); err != nil {
			t.Fatal(err)
		}
		if err := client.ZAdd(context.Background(), pendingKey, redis.Z{Score: float64(score), Member: item.id}).Err(); err != nil {
			t.Fatal(err)
		}
	}
	state, err := providerRedisClaimScript.Run(context.Background(), client, []string{pendingKey, processingKey},
		strconv.FormatInt(now, 10), "1", strconv.FormatInt(providerRedisLease.Milliseconds(), 10),
		owner, lowID, strconv.FormatInt(providerRedisMaximumWait.Milliseconds(), 10),
		strconv.FormatInt(providerRedisScoreUnit, 10),
	).Int64()
	if err != nil {
		t.Fatal(err)
	}
	if state != 1 {
		t.Fatalf("aged lifecycle claim state=%d, want selected before fresh priority", state)
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

func TestProviderRedisSlotDropsLegacyPendingOrphan(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Skipf("Redis integration test requires a local listener: %v", err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	provider := &ProviderClient{redis: client, redisID: "test-provider"}
	pendingKey, _, _ := providerRedisKeys("generic_llm")
	orphanID := "legacy-orphan"
	if err := client.HSet(context.Background(), providerRedisQueuePrefix+":job:"+orphanID, map[string]any{
		"model_run_id": "old-run", "role": "reply", "priority": 1, "score": 1, "status": "queued",
	}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.ZAdd(context.Background(), pendingKey, redis.Z{Score: 1, Member: orphanID}).Err(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	release, enabled, err := provider.acquireProviderRedisSlot(ctx, "reply", 100, 1, "run-new")
	if err != nil || !enabled {
		t.Fatalf("acquire after orphan cleanup = enabled:%v err:%v", enabled, err)
	}
	release()
}
