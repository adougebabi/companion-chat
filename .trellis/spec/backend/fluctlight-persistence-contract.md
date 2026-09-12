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
- The current local migration chain is
  `0026_capability_runtime -> 0027_project_health_evolution -> 0028_affect_canonical -> 0029_memory_lifecycle -> 0030_life_context_revision -> 0031_evolution_authority -> 0032_prompt_context_memory`.
  `0027` and `0028` are digest-frozen; `0028` owns AffectProfile backfill/reconciliation,
  complete PAD/momentum/Drive/Profile constraints, and the unique
  `(fluctlight_id,source_event_id)` state-transition boundary. `0029` adds
  Memory canonical identity, full revisions/governance, audited repair,
  embedding tuple/binding constraints and operation-aware retrieval indexes;
  it never rewrites Memory history. `0030` and `0031` are clean-start cutovers:
  any existing business authority blocks the upgrade and requires explicit
  database rebuild; neither revision backfills or reinterprets active/completed
  business rows. `0032` is additive: it links Raw sources, adds prompt-budget/
  diagnostic fields and creates Active/Summary authorities without converting
  old chat into Memory. A failure in a later revision rolls back its schema
  effects and ledger together.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Composite command supplies `tx` and a module commits | Reject in architecture/review; only the application owner may commit. |
| Foundation request uses a stale revision or reused idempotency key with a different payload | Reject with no state mutation. |
| Proposed/rejected revision is replayed | Materialize optional `accepted_at` as `None`; never fabricate a timestamp. |
| Retirement audit points to a synthetic/nonexistent revision | Reject/rollback; governance must reference a real accepted revision. |
| Intention references a Goal owned by another Fluctlight | Reject before insert. |
| PAD/momentum/normalized JSON value is non-finite or outside its canonical range | Reject in value object and database constraint. |
| Required PAD/Profile JSON key is omitted after `0028` | PostgreSQL constraint rejects the write; `CHECK NULL` must not count as valid. |
| `0028` preflight fails after an `0026` start | Roll back `0027` effects and keep the ledger at `0026`. |
| `0029` sees pre-lifecycle/unverifiably repaired Memory, duplicate identity, malformed embedding/active intent | Roll back all `0029` schema effects and keep the ledger at `0028`. |
| `0030` or `0031` sees any pre-cutover business authority | Reject clean-start cutover; preserve the previous ledger/schema and require explicit rebuild. |
| `0031` Goal/Intention/Reflection/evolution row violates closed replay/CAS authority | Abort the owning transaction; never store `null` where an authority JSON array is required. |
| `0032` sees duplicate conversation/Fluctlight sequence values or an invalid persisted prompt budget | Abort the migration and ledger advance; never repair history heuristically. |
| Ledger head contains surrounding whitespace | Reject the noncanonical ledger; never insert a second head. |

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
- Run isolated PostgreSQL routes for `0026→0027→0028`, `0027→0028`, malformed
  rollback, required-key/typed-Drive constraints, rerun idempotency, and
  noncanonical ledger rejection. Mutation tests create and delete their own
  randomly named database instead of writing to the supplied database.
- Run isolated PostgreSQL routes for empty and explicitly repaired
  `0028→0029`, rerun idempotency, unverifiable repair, duplicate active
  canonical key/revision/embedding tuple, malformed durable `working` Memory,
  malformed active embedding intent, post-cutover constraints and ledger rollback.
- Run isolated PostgreSQL routes for empty `0029→0030→0031`, direct empty
  `0030→0031`, nonempty-business rejection, deferred replay-ready constraints,
  head rerun, transaction rollback, Goal/Intention CAS, Reflection watermark,
  overlay persistence and process-restart replay.
- Run isolated PostgreSQL routes for empty and `0031→0032`, head rerun,
  duplicate message/fact sequence rollback, invalid role budget rollback,
  Raw source links/FTS, Active lifecycle constraints, Summary provenance, and
  diagnostic metric constraints.
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

## Scenario: Prompt Context and Memory Projection Migration

### 1. Scope / Trigger

- Trigger: an empty database, `0031_evolution_authority`, or an already-current
  database applies migration `0032_prompt_context_memory`.
- The migration is additive. It links owning Raw History records, creates
  Active/Summary authorities, persists model input budgets and prompt metrics,
  and never converts conversation history into durable Memory.

### 2. Signatures

```text
Head = 0032_prompt_context_memory
PreviousHead = 0031_evolution_authority

conversation_messages += turn_id, source_fact_id, correlation_id,
                         generated search_document
model_roles += context_window_tokens, max_input_tokens,
               prompt_budget_policy_version
diagnostic_model_runs += fluctlight_id, metrics,
                         estimated_input_tokens, actual_prompt_tokens,
                         actual_completion_tokens, latency_ms

active_memories / active_memory_revisions / active_memory_commands
conversation_summaries
```

`Runner.Apply(ctx, pool)` is the sole production migration entry. At head it
reapplies the same additive `promptContextMemorySchemaSQL`; an upgrade first
runs duplicate-sequence preflight, then applies that schema and advances the
single ledger row in the same transaction.

### 3. Contracts

- `conversation_messages(conversation_id,sequence)` and
  `cognition_inbox(fluctlight_id,sequence)` become unique. Existing duplicates
  fail closed; the migration never renumbers, merges, or deletes history.
- New message linkage is nullable for unverifiable historical rows. New turns
  atomically insert the user message, claimed cognition fact, workflow intent,
  and outbox; assistant settlement binds the same turn/source/correlation.
- `conversation_messages.search_document` is a stored `simple` FTS projection.
  Raw History remains a logical reader over owning tables; no `raw_history`
  table or second writer is created.
- Active Memory current/revision/command rows enforce closed kind/status/time
  enums, nonempty evidence, score/time constraints, FKs, active canonical-key
  uniqueness, revision identity, and command idempotency. Runtime DML belongs
  only to `applyActiveMemoryCommandTx`.
- Summary rows require owner/conversation, inclusive source range, ordered
  nonempty source refs/digest, nonblank summary, active/superseded lineage,
  Provider/prompt/schema/policy/request provenance, and one active exact range.
  Runtime DML belongs only to `conversation_summary.go` settlement.
- Model role defaults are context `65536`, max input `49152`, output reserve
  `token_budget=4096`, safety margin `4096`, policy `prompt-budget.v1`. The DB
  requires `max_input + token_budget + 4096 <= context_window`.
- Diagnostic metrics are a JSON object; estimated/actual token and latency
  columns are nullable nonnegative values. These rows are operational data, not
  Memory/Raw History authority.
- No `memory.recall` table, status, intent, or audit row exists; it is a bounded
  read-only Capability over existing authorities.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Ledger contains zero or multiple heads, whitespace, or an unknown head | Abort; do not guess or create a parallel migration path. |
| Existing conversation or cognition sequence duplicates exist | Raise during `0031 -> 0032`; roll back DDL and ledger together. |
| Existing model role violates positive values, policy version, or capacity equation | Abort migration; preserve `0031`. |
| Active/Summary row violates enum, evidence, time/range, JSON, FK, or uniqueness constraints | Reject the write; owning transaction rolls back. |
| Diagnostic metric is not an object or a numeric metric is negative | Reject the diagnostic row/update without weakening domain authority. |
| `Runner.Apply` executes again at `0032` | Idempotently reapply additive schema; keep one canonical head. |

### 5. Good / Base / Bad Cases

- Good: `0031` with valid unique history and role settings upgrades atomically;
  nullable old links remain null, new turns are linked, and no Memory content is
  fabricated.
- Base: an empty database reaches `0032`; a rerun changes neither ledger
  identity nor domain rows.
- Bad: repair duplicate sequences by renumbering, backfill every old message as
  Memory, run Provider I/O in the migration transaction, or add a recall table.

### 6. Tests Required

- Embedded-SQL tests assert all columns, closed constraints, indexes/FKs, one
  schema literal, one head, no Raw History table, and no history rewrite/delete.
- Real PostgreSQL tests run empty→head, `0031`→head, head rerun, duplicate
  message/fact sequences, malformed role budgets, Active/Summary constraints,
  and transaction/ledger rollback in isolated disposable databases.
- Turn-atomicity tests fail each user/fact/intent/outbox insert boundary and
  assert all-or-none visibility; Provider remains outside the transaction.
- Lifecycle tests assert Active/Summary DML comes only from their owning Core
  authorities; durable `memories` remains under `memory_lifecycle.go`.
- PostgreSQL tests may be authored earlier but are acceptance evidence only
  when executed at the final full-verification gate.

### 7. Wrong vs Correct

#### Wrong

```sql
INSERT INTO memories (content, type)
SELECT text, 'semantic' FROM conversation_messages;
```

#### Correct

```text
ALTER owning message rows with nullable source links and FTS
+ create independent Active/Summary projection tables
+ validate sequence and prompt-budget invariants
+ advance the ledger in the same transaction
```
