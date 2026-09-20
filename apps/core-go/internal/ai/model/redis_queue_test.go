package model

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisKeysKeepBindingsSeparate(t *testing.T) {
	pending, processing, sequence := RedisKeys("generic_llm")
	if pending != "fluctlight:llm:generic_llm:pending" || processing != "fluctlight:llm:generic_llm:processing" || sequence != "fluctlight:llm:generic_llm:sequence" {
		t.Fatalf("generic keys = %q %q %q", pending, processing, sequence)
	}
	embeddingPending, _, _ := RedisKeys("embedding")
	if embeddingPending == pending {
		t.Fatal("embedding and generic queues share a pending key")
	}
}

func TestRedisScoreKeepsPriorityBeforeFIFO(t *testing.T) {
	if RedisScore(100, 999) >= RedisScore(90, 1) {
		t.Fatal("higher priority did not sort ahead of lower priority")
	}
	if RedisScore(90, 1) >= RedisScore(90, 2) {
		t.Fatal("equal priority did not preserve FIFO sequence")
	}
}

func TestRedisAgingSelectsOldLifecycleBeforeFreshPriority(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Skipf("Redis integration test requires a local listener: %v", err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	pendingKey, processingKey, _ := RedisKeys("generic_llm")
	now := time.Now().UnixMilli()
	owner := "provider-aging-test"
	lowID, highID := "aged-reflection", "fresh-reply"
	for _, item := range []struct {
		id       string
		priority int
		sequence int64
		queuedAt int64
	}{
		{id: lowID, priority: 70, sequence: 1, queuedAt: now - RedisMaximumWait.Milliseconds() - 1},
		{id: highID, priority: 100, sequence: 2, queuedAt: now},
	} {
		score := RedisScore(item.priority, item.sequence)
		jobKey := RedisQueuePrefix + ":job:" + item.id
		if err := client.HSet(context.Background(), jobKey, map[string]any{
			"score": score, "sequence": item.sequence, "queued_at": item.queuedAt,
			"status": "queued", "pending_owner": owner, "pending_until": now + RedisPendingTTL.Milliseconds(),
		}).Err(); err != nil {
			t.Fatal(err)
		}
		if err := client.ZAdd(context.Background(), pendingKey, redis.Z{Score: float64(score), Member: item.id}).Err(); err != nil {
			t.Fatal(err)
		}
	}
	state, err := RedisClaimScript.Run(context.Background(), client, []string{pendingKey, processingKey},
		strconv.FormatInt(now, 10), "1", strconv.FormatInt(RedisLease.Milliseconds(), 10), owner, lowID,
		strconv.FormatInt(RedisMaximumWait.Milliseconds(), 10), strconv.FormatInt(RedisScoreUnit, 10),
	).Int64()
	if err != nil {
		t.Fatal(err)
	}
	if state != 1 {
		t.Fatalf("expected aged low-priority job to claim first, state=%d", state)
	}
}
