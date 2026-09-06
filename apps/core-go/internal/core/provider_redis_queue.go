package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	providerRedisQueuePrefix = "fluctlight:llm"
	providerRedisLease       = 2 * time.Minute
	providerRedisPendingTTL  = providerRedisLease
	providerRedisJobTTL      = 24 * time.Hour
	providerRedisPoll        = 40 * time.Millisecond
	providerRedisScoreUnit   = int64(1_000_000_000_000)
)

var providerRedisClaimScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local lease = tonumber(ARGV[3])
local owner = ARGV[4]
local job = ARGV[5]

local expired = redis.call('ZRANGE', KEYS[2], '-inf', now, 'BYSCORE')
for _, member in ipairs(expired) do
  local original = redis.call('HGET', 'fluctlight:llm:job:' .. member, 'score')
  if original then
    redis.call('ZADD', KEYS[1], original, member)
    redis.call('HSET', 'fluctlight:llm:job:' .. member, 'status', 'queued', 'pending_owner', 'reconciler', 'pending_until', now + lease)
    redis.call('HDEL', 'fluctlight:llm:job:' .. member, 'owner', 'lease_until')
    redis.call('EXPIRE', 'fluctlight:llm:job:' .. member, 120)
  end
  redis.call('ZREM', KEYS[2], member)
end

if redis.call('ZSCORE', KEYS[2], job) then
  return 0
end
if not redis.call('ZSCORE', KEYS[1], job) then
  return -1
end
if redis.call('ZCARD', KEYS[2]) >= limit then
  return 0
end
local first = redis.call('ZRANGE', KEYS[1], 0, 0)[1]
if first then
  local firstKey = 'fluctlight:llm:job:' .. first
  local firstExists = redis.call('EXISTS', firstKey)
  local pendingUntil = redis.call('HGET', firstKey, 'pending_until')
  -- Pending jobs created by an older build have no lease and cannot be
  -- associated with a live caller. Remove them instead of blocking every
  -- later request until the long job TTL expires.
  if firstExists == 0 or not pendingUntil or tonumber(pendingUntil) <= now then
    redis.call('ZREM', KEYS[1], first)
    redis.call('DEL', firstKey)
    if first == job then
      return -1
    end
    return 0
  end
end
if first ~= job then
  return 0
end
redis.call('ZREM', KEYS[1], job)
redis.call('ZADD', KEYS[2], now + lease, job)
redis.call('HSET', 'fluctlight:llm:job:' .. job, 'status', 'processing', 'owner', owner, 'lease_until', now + lease)
redis.call('HDEL', 'fluctlight:llm:job:' .. job, 'pending_owner', 'pending_until')
redis.call('EXPIRE', 'fluctlight:llm:job:' .. job, 86400)
return 1
`)

var providerRedisReleaseScript = redis.NewScript(`
local jobKey = 'fluctlight:llm:job:' .. ARGV[1]
if redis.call('HGET', jobKey, 'owner') ~= ARGV[2] then
  return 0
end
redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('DEL', jobKey)
return 1
`)

var providerRedisCancelScript = redis.NewScript(`
local job = ARGV[1]
redis.call('ZREM', KEYS[1], job)
redis.call('ZREM', KEYS[2], job)
redis.call('DEL', 'fluctlight:llm:job:' .. job)
return 1
`)

var providerRedisRenewScript = redis.NewScript(`
local jobKey = 'fluctlight:llm:job:' .. ARGV[2]
if redis.call('HGET', jobKey, 'owner') ~= ARGV[3] then
  return 0
end
redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
redis.call('HSET', jobKey, 'lease_until', ARGV[1])
redis.call('EXPIRE', jobKey, ARGV[4])
return 1
`)

var providerRedisRenewPendingScript = redis.NewScript(`
local jobKey = 'fluctlight:llm:job:' .. ARGV[2]
if redis.call('HGET', jobKey, 'status') ~= 'queued' then
  return 0
end
if redis.call('HGET', jobKey, 'pending_owner') ~= ARGV[3] then
  return 0
end
redis.call('HSET', jobKey, 'pending_until', ARGV[1])
redis.call('EXPIRE', jobKey, ARGV[4])
return 1
`)

var providerRedisRequeueExpiredScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local members = redis.call('ZRANGE', KEYS[2], '-inf', now, 'BYSCORE')
for _, member in ipairs(members) do
  local jobKey = 'fluctlight:llm:job:' .. member
  local original = redis.call('HGET', jobKey, 'score')
  if original and redis.call('EXISTS', jobKey) == 1 then
    redis.call('ZADD', KEYS[1], original, member)
    redis.call('HSET', jobKey, 'status', 'queued', 'pending_owner', 'reconciler', 'pending_until', now + 120000)
    redis.call('HDEL', jobKey, 'owner', 'lease_until')
    redis.call('EXPIRE', jobKey, 120)
  else
    redis.call('DEL', jobKey)
  end
  redis.call('ZREM', KEYS[2], member)
end
local pendingMembers = redis.call('ZRANGE', KEYS[1], 0, -1)
for _, member in ipairs(pendingMembers) do
  local jobKey = 'fluctlight:llm:job:' .. member
  local pendingUntil = redis.call('HGET', jobKey, 'pending_until')
  if redis.call('EXISTS', jobKey) == 0 or not pendingUntil or tonumber(pendingUntil) <= now then
    redis.call('ZREM', KEYS[1], member)
    redis.call('DEL', jobKey)
  end
end
return #members
`)

func providerRedisKeys(binding string) (string, string, string) {
	binding = strings.TrimSpace(binding)
	if binding == "" {
		binding = "generic_llm"
	}
	base := providerRedisQueuePrefix + ":" + binding
	return base + ":pending", base + ":processing", base + ":sequence"
}

func providerRedisScore(priority int, sequence int64) int64 {
	if priority < 0 {
		priority = 0
	}
	if priority > 100 {
		priority = 100
	}
	return int64(100-priority)*providerRedisScoreUnit + sequence
}

// acquireProviderRedisSlot coordinates the existing synchronous provider call
// across API/Worker processes. Redis stores only a short-lived job reference;
// the closure and provider result remain local to the caller. A false enabled
// result means Redis is unavailable and the caller should use its local queue.
func (p *ProviderClient) acquireProviderRedisSlot(ctx context.Context, role string, priority, limit int, diagnosticID string) (release func(), enabled bool, err error) {
	if p == nil || p.redis == nil || strings.TrimSpace(diagnosticID) == "" {
		return func() {}, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	binding := providerBindingRole(role)
	pendingKey, processingKey, sequenceKey := providerRedisKeys(binding)
	sequence, err := p.redis.Incr(ctx, sequenceKey).Result()
	if err != nil {
		return func() {}, false, nil
	}
	if limit <= 0 {
		limit = providerQueueDefaultConcurrency
		if binding == "embedding" {
			limit = providerQueueDefaultEmbedding
		}
	}
	if priority < 0 {
		priority = 0
	}
	if priority > 100 {
		priority = 100
	}
	score := providerRedisScore(priority, sequence)
	jobID := diagnosticID + ":" + randomID("queue_")
	jobKey := providerRedisQueuePrefix + ":job:" + jobID
	owner := p.redisOwner()
	pendingUntil := time.Now().Add(providerRedisPendingTTL).UnixMilli()
	if err := p.redis.HSet(ctx, jobKey, map[string]any{"model_run_id": diagnosticID, "role": role, "priority": priority, "score": score, "status": "queued", "pending_owner": owner, "pending_until": pendingUntil}).Err(); err != nil {
		return func() {}, false, nil
	}
	if err := p.redis.Expire(ctx, jobKey, providerRedisPendingTTL).Err(); err != nil {
		_ = p.redis.Del(context.Background(), jobKey).Err()
		return func() {}, false, nil
	}
	if err := p.redis.ZAdd(ctx, pendingKey, redis.Z{Score: float64(score), Member: jobID}).Err(); err != nil {
		_ = p.redis.Del(context.Background(), jobKey).Err()
		return func() {}, false, nil
	}
	claim := func() (int64, error) {
		return providerRedisClaimScript.Run(ctx, p.redis, []string{pendingKey, processingKey}, strconv.FormatInt(time.Now().UnixMilli(), 10), strconv.Itoa(limit), strconv.FormatInt(providerRedisLease.Milliseconds(), 10), owner, jobID).Int64()
	}
	nextPendingHeartbeat := time.Now().Add(providerRedisPendingTTL / 3)
	for {
		if time.Now().After(nextPendingHeartbeat) {
			_ = providerRedisRenewPendingScript.Run(ctx, p.redis, []string{pendingKey}, strconv.FormatInt(time.Now().Add(providerRedisPendingTTL).UnixMilli(), 10), jobID, owner, strconv.FormatInt(int64(providerRedisPendingTTL/time.Second), 10)).Err()
			nextPendingHeartbeat = time.Now().Add(providerRedisPendingTTL / 3)
		}
		state, claimErr := claim()
		if claimErr != nil {
			_ = p.cancelProviderRedisJob(context.Background(), pendingKey, processingKey, jobID)
			return func() {}, false, nil
		}
		switch state {
		case 1:
			releaseDone := make(chan struct{})
			var releaseOnce sync.Once
			go func() {
				ticker := time.NewTicker(providerRedisLease / 3)
				defer ticker.Stop()
				for {
					select {
					case <-releaseDone:
						return
					case <-ticker.C:
						leaseUntil := time.Now().Add(providerRedisLease).UnixMilli()
						_ = providerRedisRenewScript.Run(context.Background(), p.redis, []string{processingKey}, strconv.FormatInt(leaseUntil, 10), jobID, owner, strconv.FormatInt(int64(providerRedisJobTTL/time.Second), 10)).Err()
					}
				}
			}()
			return func() {
				releaseOnce.Do(func() {
					close(releaseDone)
					_ = providerRedisReleaseScript.Run(context.Background(), p.redis, []string{processingKey}, jobID, owner).Err()
				})
			}, true, nil
		case -1:
			return func() {}, true, fmt.Errorf("provider redis queue job disappeared")
		}
		timer := time.NewTimer(providerRedisPoll)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = p.cancelProviderRedisJob(context.Background(), pendingKey, processingKey, jobID)
			return func() {}, true, ctx.Err()
		case <-timer.C:
		}
	}
}

func (p *ProviderClient) cancelProviderRedisJob(ctx context.Context, pendingKey, processingKey, jobID string) error {
	return providerRedisCancelScript.Run(ctx, p.redis, []string{pendingKey, processingKey}, jobID).Err()
}

func (p *ProviderClient) redisOwner() string {
	if p.redisID != "" {
		return p.redisID
	}
	return "provider-" + randomID("process_")
}

// ReconcileRedisQueue requeues expired leases and removes orphaned jobs. It is
// intentionally best-effort so a Redis outage cannot stop the Worker; the
// PostgreSQL diagnostic stale-run recovery remains authoritative.
func (p *ProviderClient) ReconcileRedisQueue(ctx context.Context, role string) (int64, error) {
	if p == nil || p.redis == nil {
		return 0, nil
	}
	pendingKey, processingKey, _ := providerRedisKeys(providerBindingRole(role))
	return providerRedisRequeueExpiredScript.Run(ctx, p.redis, []string{pendingKey, processingKey}, strconv.FormatInt(time.Now().UnixMilli(), 10)).Int64()
}
