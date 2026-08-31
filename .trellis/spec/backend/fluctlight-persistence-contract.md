# Fluctlight Persistence Contract

## Scenario: Cross-Module Atomicity Without External I/O In Transactions

### 1. Scope / Trigger

- Trigger: a clean-start Go application command reads or writes PostgreSQL, spans more than one domain module, or schedules any LLM/Redis/object/media side effect.
- This contract replaces old synchronous SQLite transaction assumptions for the new system.
- It preserves module table ownership inside one application schema while using the modular monolith's single PostgreSQL database for application-level atomic invariants.

### 2. Signatures

```python
async with unit_of_work.begin(command_id=command_id) as tx:
    result_a = module_a.apply(command_a, tx=tx)
    result_b = module_b.apply(command_b, tx=tx)
    tx.outbox.add(event_or_external_intent)
    await tx.commit()
```

Required outbox/intent envelope:

```text
id
kind
aggregate_type
aggregate_id
fluctlight_id
causation_id
correlation_id
idempotency_key
payload
occurred_at
available_at
attempt_policy
published_at / completed_at / failed_at
```

Module interfaces accept application commands and the application-owned transaction context. Internal repositories bind to that context but are not exported. A composite command has one commit owner.

Data-access baseline: pgx/v5, PostgreSQL transactions, and the embedded Go migration bundle. Application tables share the `public` schema and one linear migration graph.

### 3. Contracts

- All application tables use one PostgreSQL schema. Each domain module owns its tables, constraints, migration changes, repository implementation, and row-to-domain mapping.
- SQLAlchemy Core is the default; ORM may be used internally by one module but ORM entities/lazy relationships cannot cross module or transport interfaces.
- One Unit of Work owns one AsyncSession, which cannot be shared across concurrent tasks.
- Production uses the explicit Go migration command and revision verification. API/Worker never call `create_all()` or automatic upgrade.
- One module never queries another module's table or imports its internal repository. Cross-module reads and writes use public module interfaces.
- An application Unit of Work may compose multiple module interfaces in one short PostgreSQL transaction when one business invariant requires atomicity.
- Modules participating in a composite command do not commit, roll back, publish events, or call external systems independently.
- The transaction includes domain state, idempotency records, and outbox/external-intent rows. Commit makes all or none visible.
- LLM, Redis, Redis Streams, object storage, ComfyUI, h3, HTTP callbacks, and long polling never run inside a PostgreSQL transaction.
- A Worker executes committed intents with stable workflow/request IDs. Replay returns the prior result or performs an idempotent Provider/object operation.
- Outbox publication is at-least-once. Consumers use event ID plus a durable inbox/idempotency record before applying effects.
- No distributed transaction is introduced between PostgreSQL and Redis/object/Provider systems.
- Long cognitive work uses short phases: claim/capture revisions, external assessment, CAS apply/freeze, external realization, action-result commit.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Module attempts to query another module's table/repository | Architecture failure; change the owning module interface. |
| Module commits inside an application Unit of Work | Contract failure; only the application transaction owner may commit. |
| Outbox insert fails | Roll back all domain changes in the Unit of Work. |
| Process crashes after commit and before publish/execute | Publisher/Worker resumes the committed outbox intent. |
| Publisher sends the same event more than once | Consumer inbox replays the existing result; no duplicate effect. |
| External Provider succeeds but result commit crashes | Retry with the same Provider/workflow idempotency key and persist/recover the existing result. |
| Expected revision/CAS is stale | Apply no state change; explicitly retry/re-assess or terminate according to the workflow. |
| External call is attempted with an open business transaction | Reject in tests/review; move the call behind a committed intent. |
| Transaction exceeds configured duration/lock budget | Roll back and diagnose; never hold locks while waiting for model/media work. |
| AsyncSession is shared across concurrent tasks | Architecture/runtime contract failure; create one Unit of Work/session per task. |
| API/Worker starts against wrong schema revision | Fail readiness/startup with bounded migration instruction; do not auto-upgrade. |
| ORM entity/table mapping crosses module/HTTP interface | Architecture-test failure; map through owning module interface. |

### 5. Good / Base / Bad Cases

- Good: one cognitive Unit of Work consumes an inbox fact, applies inner-state and relationship transitions, freezes a decision, and writes an outbox action intent atomically.
- Good: media deletion commits reference removal plus a tombstone/outbox intent; physical object deletion happens later and is retryable.
- Base: a single-module settings update still uses the same Unit of Work pattern and commits one module plus optional outbox.
- Bad: write a message, commit, then attempt to insert its workflow row in another transaction without an outbox.
- Bad: call an LLM or ComfyUI while holding row locks, or query another module's table to avoid defining an interface.

### 6. Tests Required

- Unit-of-Work integration tests assert multi-module commit and rollback, one commit owner, and module interface composition.
- Architecture tests prevent cross-module repository/table imports and external SDK imports from domain modules.
- Outbox atomicity tests fail the outbox insert and assert no domain rows commit.
- Crash-window tests stop after commit/before publish and after external success/before result commit, then assert deterministic recovery.
- Duplicate delivery tests assert consumer inbox/idempotency prevents repeated state changes and Provider effects.
- CAS/concurrency tests assert stale cognitive/reflection updates cannot overwrite newer state.
- Transaction-duration tests use fake slow adapters and assert external calls occur only after commit with no open business transaction.
- Migration tests run empty→head and previous-release→head on real PostgreSQL; startup tests assert API/Worker only verify revision.
- Architecture tests reject ORM/table mapping leakage, concurrent AsyncSession sharing patterns, and SQLite substitutes for PostgreSQL-specific integration tests.

### 7. Wrong vs Correct

#### Wrong

```python
async with db.transaction():
    await conversations.append(message)
    completion = await llm.complete(prompt)
    await redis.xadd("events", completion)
```

#### Correct

```python
async with unit_of_work.begin(command_id=command_id) as tx:
    conversations.append(message, tx=tx)
    tx.outbox.add(AssessmentRequested(idempotency_key=command_id))
    await tx.commit()

await workflow_runtime.execute_committed_intent(command_id)
```

## Scenario: T04 Foundation And Inner-State Persistence

### 1. Scope / Trigger

- Trigger: the `fluctlights` or `inner_state` module creates a foundation,
  proposes/accepts a revision, applies a numeric assessment, or governs a Goal
  or Intention in PostgreSQL.
- This scenario makes the T04 migration and module/application transaction
  boundary executable for later cognitive and life-world children.

### 2. Signatures

```python
await fluctlights.create(command, tx=tx)              # no commit when tx is supplied
await fluctlights.submit_revision(request, tx=tx)
await inner_state.apply_assessment(fluctlight_id, assessment,
                                   expected_revision=revision, tx=tx)
await inner_state.govern_intention(command, tx=tx)
```

The released schema owns one linear revision chain, ending at `0020_media_provider_job`, and these public
tables: `fluctlights`, `fluctlight_foundation_revisions`,
`fluctlight_foundation_governance`, `fluctlight_inner_states`,
`fluctlight_inner_state_events`, `fluctlight_goals`,
`fluctlight_goal_governance`, `fluctlight_intentions`, and
`fluctlight_intention_governance`.

### 3. Contracts

- Every T04 service accepts an application-owned `UnitOfWork` when a command
  composes more than one module. A supplied transaction is never committed or
  rolled back by the module; a standalone convenience call may create and own
  one transaction.
- Foundation revisions are append-only. Revision `0` is the accepted
  initialization baseline; later revisions have a strict base/current CAS and
  preserve initialization mode, lifecycle status, foundation creation time,
  evidence, confidence, and audit identity.
- Inner-state assessment, lazy wall-time decay, and its numeric delta audit are
  one state revision. Idempotency replay returns the stored transition and
  rejects reuse for another Fluctlight/source event.
- Goal/Intention ownership is `(fluctlight_id, goal_id)`, not a bare goal ID.
  Every lifecycle transition appends immutable governance history with actor
  and reason.
- Actor audit fields reference `public.actors`; PAD/momentum ranges are
  bounded in code and PostgreSQL constraints. Domain contracts do not expose
  SQLAlchemy rows.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Composite command supplies `tx` and a module commits | Reject in architecture/review; only the application owner may commit. |
| Foundation request uses a stale revision or reused idempotency key with a different payload | Reject with no state mutation. |
| Proposed/rejected revision is replayed | Materialize optional `accepted_at` as `None`; never fabricate a timestamp. |
| Retirement audit points to a synthetic/nonexistent revision | Reject/rollback; governance must reference a real accepted revision. |
| Intention references a Goal owned by another Fluctlight | Reject before insert. |
| PAD/momentum/normalized JSON value is non-finite or outside its canonical range | Reject in value object and database constraint. |

### 5. Good / Base / Bad Cases

- Good: an application UoW creates a Fluctlight, initializes inner state, and
  writes an outbox row before one commit.
- Base: a standalone read or single-module command uses the service's owned
  short transaction and returns a domain snapshot.
- Bad: a module commits its row and then a later module/outbox insert fails, or
  a revision replay reconstructs a blank-slate lifecycle from a paused record.

### 6. Tests Required

- Assert every T04 service accepts an injected UoW and does not commit it.
- Assert baseline/proposed/rejected/accepted/rollback materialization,
  optional timestamps, lifecycle metadata, stale CAS, and idempotency replay.
- Run migration SQL/real-PostgreSQL checks for actor FKs, composite Goal
  ownership, JSON numeric checks, empty-to-head, and `0002`-to-head upgrade.
- Assert assessment revision increments once even when elapsed wall-time decay
  is applied, and requested/applied audit includes mood and drive fields.

### 7. Wrong vs Correct

#### Wrong

```python
async with unit_of_work.begin(command_id=command_id) as tx:
    await fluctlights.create(command)  # opens/commits another transaction
    await inner_state.initialize(fluctlight_id)
    await tx.commit()
```

#### Correct

```python
async with unit_of_work.begin(command_id=command_id) as tx:
    await fluctlights.create(command, tx=tx)
    await inner_state.initialize(command.id, tx=tx)
    await tx.commit()
```
