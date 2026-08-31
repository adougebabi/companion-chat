# Fluctlight Event Contract

## Scenario: Rebuildable Redis Streams Transport From PostgreSQL Events

### 1. Scope / Trigger

- Trigger: a committed domain event is distributed asynchronously, a consumer applies an integration/projection effect, or Worker progress is pushed to clients.
- PostgreSQL outbox/inbox/event records are the durable authority. Redis Streams is a short-term at-least-once transport.
- Redis Streams does not execute workflows, durable timers, delayed jobs, or domain transactions.

### 2. Signatures

```text
fluctlight:events:v1    durable transport
fluctlight:progress:v1  ephemeral progress

EventEnvelopeV1
  event_id
  event_type
  schema_version
  aggregate_type / aggregate_id / aggregate_sequence
  fluctlight_id?
  causation_id / correlation_id
  occurred_at
  payload
```

Publisher interface:

```python
publish_committed_outbox(batch: OutboxBatch) -> PublishResults
```

Consumer interface:

```python
consume(event: EventEnvelopeV1, group: str, consumer: str) -> ConsumerResult
```

Initial durable consumer groups are `bff-notifications`, `cache-projections`, and `integration-observers`. Future group-conversation fan-out adds a group only when a real consumer exists.

### 3. Contracts

- Durable stream entries originate only from committed PostgreSQL outbox rows.
- `event_id` is the cross-transport idempotency key. Redis stream IDs are transport positions, not domain IDs.
- A publisher may emit the same event more than once after crash recovery. Consumers must replay the persisted inbox result.
- A durable consumer opens a PostgreSQL transaction, checks/records `(consumer_group, event_id)`, applies its owned effect, commits, then calls `XACK`.
- Consumer failure before commit leaves no inbox/effect; the entry remains pending. Failure after commit/before `XACK` replays the inbox result and then acknowledges.
- `XAUTOCLAIM` reassigns entries idle beyond the consumer lease. Delivery attempts beyond policy create a PostgreSQL failure record and a bounded operational signal.
- `aggregate_sequence` detects gaps/ordering anomalies; consumers do not silently invent missing state.
- Trim uses a configured time/ID retention window and must remain behind critical consumer-group pending/progress positions. PostgreSQL outbox/event journal provides longer replay.
- Redis AOF `everysec` and persistence volume are enabled, but loss of the Redis volume is recoverable by rebuilding groups/streams from PostgreSQL.
- Progress entries may use approximate `MAXLEN`; they carry no authoritative final state and require no PostgreSQL outbox/inbox. Clients query Go Core for final status.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Event lacks ID/type/schema/aggregate sequence | Do not publish/consume; record bounded outbox failure. |
| Publisher crashes after `XADD` before marking outbox | Republish same `event_id`; consumers deduplicate. |
| Consumer crashes before PostgreSQL commit | Leave pending; reclaim and process normally. |
| Consumer crashes after commit before `XACK` | Replay inbox result, perform no second effect, then acknowledge. |
| Pending entry exceeds idle threshold | `XAUTOCLAIM` to a live consumer. |
| Processing attempts exceed policy | Record poison/failure in PostgreSQL, acknowledge or quarantine by explicit policy, alert inspector. |
| Aggregate sequence gap | Stop dependent projection/application, record gap, replay from PostgreSQL; do not guess. |
| Trim cutoff intersects critical pending entries | Refuse trim/raise operational warning. |
| Redis stream/AOF volume is lost | Recreate streams/groups and republish retained outbox/event-journal range. |
| Progress entry is lost | UI may miss intermediate progress; final API status remains correct. |

### 5. Good / Base / Bad Cases

- Good: an event is published twice after a publisher crash; each consumer group commits one effect and acknowledges both deliveries through one inbox result.
- Good: a dead consumer's pending event is reclaimed and completed by another Worker.
- Base: media progress is trimmed before a browser sees it; the browser refreshes authoritative status from Go Core.
- Bad: store delayed jobs in Streams, `XACK` before database commit, use the stream ID as a business ID, or trim while critical entries remain pending.

### 6. Tests Required

- Publisher tests for committed-only rows, batch publish, success-before-mark crash, duplicate event IDs, and bounded serialization failures.
- Consumer tests for inbox atomicity, before/after-commit crashes, duplicate delivery, `XACK` ordering, and group isolation.
- Reclaim tests for idle PEL entries, delivery attempts, poison/failure records, and consumer replacement.
- Sequence tests for ordered events, gaps, replay, and no guessed projection.
- Retention tests for safe trim cutoffs, pending protection, approximate progress trim, and PostgreSQL replay beyond Redis retention.
- Disaster test deleting the Redis volume, restoring from PostgreSQL outbox/event journal, and proving one effective consumer outcome.
- Progress tests prove intermediate loss does not alter authoritative workflow/media status.

### 7. Wrong vs Correct

#### Wrong

```python
event = await redis.xreadgroup(...)
await redis.xack(stream, group, event.stream_id)
await apply_business_effect(event)
```

#### Correct

```python
event = await redis.xreadgroup(...)
async with unit_of_work.begin(command_id=event.event_id) as tx:
    result = consumer_inbox.apply_once(group, event, tx=tx)
    await tx.commit()
await redis.xack(stream, group, event.stream_id)
```

## Scenario: Go Outbox/Redis Worker Pipeline

### 1. Scope / Trigger

- Trigger: Go Core commits an outbox event or the Worker consumes a durable
  Redis Stream delivery after a restart.

### 2. Signatures

- `OutboxPublisher.PublishOnce(ctx, limit)` claims PostgreSQL rows, publishes
  `EventEnvelope` to `fluctlight:events:v1`, and settles published/retry/failed
  state without holding a transaction across Redis I/O.
- `EventConsumer.ConsumeOnce(ctx, count)` performs inbox/effect/head writes in
  one PostgreSQL transaction and acknowledges Redis only after commit.

### 3. Contracts

- Event IDs remain the cross-transport idempotency key; Redis stream IDs are
  delivery positions only.
- Claim leases, bounded attempts and `failed_at` prevent a crashed publisher
  or poison consumer from spinning indefinitely.
- Duplicate group/event deliveries reuse the existing inbox result and create
  no second effect. Aggregate sequence gaps are rejected rather than guessed.
- Worker owns publisher and all configured durable groups; API never polls
  Redis or Temporal queues.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| PostgreSQL row is claimed but Redis publish fails | clear claim, back off, or mark terminal after max attempts |
| Worker crashes after `XADD` before `published_at` | republish same event ID; consumers deduplicate |
| duplicate Redis delivery | inbox conflict, effect unchanged, then `XACK` |
| malformed event payload | durable poison failure record and acknowledgement |
| aggregate sequence gap | leave delivery pending and record retryable processing failure |

### 5. Good/Base/Bad Cases

- Good: a cognition fact is published once, consumed by all three groups, and
  each group has one inbox/effect record with zero pending lag.
- Base: Redis is rebuilt from PostgreSQL outbox; already published events are
  not duplicated in PostgreSQL effects.
- Bad: acknowledge before PostgreSQL commit, use stream IDs as business IDs,
  or leave an outbox row permanently claimed after a crash.

### 6. Tests Required

- Miniredis + PostgreSQL integration tests for publish, duplicate delivery,
  poison handling, claim retry and consumer group isolation.
- Restart/crash-window tests for `XADD`/published marker and transaction/
  `XACK` ordering, plus aggregate gap tests.

### 7. Wrong vs Correct

#### Wrong

```go
redis.XAck(ctx, stream, group, messageID)
applyConsumerEffect(ctx, message)
```

#### Correct

```go
commitInboxEffect(ctx, message)
redis.XAck(ctx, stream, group, messageID)
```
