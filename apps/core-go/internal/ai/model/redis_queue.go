package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	RedisQueuePrefix      = "fluctlight:llm"
	CognitionCancelPrefix = "fluctlight:cognition:cancel:"
	CancellationTTL       = 15 * time.Minute
	RedisLease            = 2 * time.Minute
	RedisPendingTTL       = RedisLease
	RedisJobTTL           = 24 * time.Hour
	RedisPoll             = 40 * time.Millisecond
	RedisMaximumWait      = 2 * time.Minute
	RedisScoreUnit        = int64(1_000_000_000_000)
)

type CancellationKey struct{}

func WithCancellationKey(ctx context.Context, sourceFactID string) context.Context {
	return context.WithValue(ctx, CancellationKey{}, strings.TrimSpace(sourceFactID))
}

func CancellationMarker(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(CancellationKey{}).(string)
	return strings.TrimSpace(value)
}

func WakeUpCancellationMarker(fluctlightID string, cycle int) string {
	return fmt.Sprintf("wake_up:%s:cycle:%d", strings.TrimSpace(fluctlightID), cycle)
}

func ReflectionCancellationMarker(intentID string) string {
	return "reflection:" + strings.TrimSpace(intentID)
}

func RedisKeys(binding string) (string, string, string) {
	binding = strings.TrimSpace(binding)
	if binding == "" {
		binding = "generic_llm"
	}
	base := RedisQueuePrefix + ":" + binding
	return base + ":pending", base + ":processing", base + ":sequence"
}

func RedisScore(priority int, sequence int64) int64 {
	if priority < 0 {
		priority = 0
	}
	if priority > 100 {
		priority = 100
	}
	return int64(100-priority)*RedisScoreUnit + sequence
}

func RedisAgedScore(sequence int64) int64 {
	return -RedisScoreUnit + sequence
}

// Redis Lua scripts for distributed priority queue
var (
	RedisClaimScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local lease = tonumber(ARGV[3])
local owner = ARGV[4]
local job = ARGV[5]
local max_wait = tonumber(ARGV[6])
local aged_score_unit = tonumber(ARGV[7])

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
local pending = redis.call('ZRANGE', KEYS[1], 0, 127)
for _, member in ipairs(pending) do
  local jobKey = 'fluctlight:llm:job:' .. member
  local queued_at = redis.call('HGET', jobKey, 'queued_at')
  local sequence = redis.call('HGET', jobKey, 'sequence')
  if queued_at and sequence and now - tonumber(queued_at) >= max_wait then
    local aged_score = -aged_score_unit + tonumber(sequence)
    redis.call('ZADD', KEYS[1], aged_score, member)
    redis.call('HSET', jobKey, 'aged_score', aged_score)
  end
end
local first = redis.call('ZRANGE', KEYS[1], 0, 0)[1]
if first then
  local firstKey = 'fluctlight:llm:job:' .. first
  local firstExists = redis.call('EXISTS', firstKey)
  local pendingUntil = redis.call('HGET', firstKey, 'pending_until')
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

	RedisReleaseScript = redis.NewScript(`
local jobKey = 'fluctlight:llm:job:' .. ARGV[1]
if redis.call('HGET', jobKey, 'owner') ~= ARGV[2] then
  return 0
end
redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('DEL', jobKey)
return 1
`)

	RedisCancelScript = redis.NewScript(`
local job = ARGV[1]
redis.call('ZREM', KEYS[1], job)
redis.call('ZREM', KEYS[2], job)
redis.call('DEL', 'fluctlight:llm:job:' .. job)
return 1
`)

	RedisRenewScript = redis.NewScript(`
local jobKey = 'fluctlight:llm:job:' .. ARGV[2]
if redis.call('HGET', jobKey, 'owner') ~= ARGV[3] then
  return 0
end
redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
redis.call('HSET', jobKey, 'lease_until', ARGV[1])
redis.call('EXPIRE', jobKey, ARGV[4])
return 1
`)

	RedisRenewPendingScript = redis.NewScript(`
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

	RedisRequeueExpiredScript = redis.NewScript(`
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
)
