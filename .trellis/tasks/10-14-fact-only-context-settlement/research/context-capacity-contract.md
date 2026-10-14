# Exact internal-capacity contract excerpts

Verbatim complete capacity scenarios; original specs remain authoritative. Regenerate when either whole-source hash changes.

# Exact task-scoped contract excerpts

Source: `.trellis/spec/backend/structured-turn-contract.md`

Source SHA-256: `515774a1b41dbfa1caf33cf0aca664e29c10091c3cdee63ff17760066a3b23c0`

Verbatim selected complete scenario sections to prevent native context injection truncation. The original spec remains authoritative. Read further source sections if implementation touches neighboring behavior; regenerate if its source hash changes.

<!-- Original lines 747–820 -->

## Scenario: Internal context capacity is advisory (2026-10-14)

### 1. Scope / Trigger
`ProcessCognitionActivity` failed before a model request with
`context_reference_index_too_large`. Internal optimization thresholds must permit
complete valid context to continue, including freezing, Tool replay and settlement.
This scenario supersedes byte/count rejection for this internal data path; the
configured physical model budget and external model-argument schema remain separate.

### 2. Signatures
`ContextReferenceIndex.add(...)`, `ContextReferenceIndex.Validate() error`,
`ContextSnapshotFromProjection(projection)`, both Core/canonical
`CapabilityContext.Snapshot()` and snapshot resolvers, prepared payload
encode/decode/data helpers, `frozenResultingStateCausalityTx`,
`actionOutcomeContextReferences` and `ActionOutcome.Validate` preserve complete
JSON data. `App.recordContextReferenceCapacity(ctx, projection, stage)` emits
the advisory described in the diagnostics contract.

### 3. Contracts
- Reference count 128, single snapshot 32 KiB and encoded index 128 KiB are
  **warning thresholds**, not validation limits. Values greater than a threshold
  remain valid. Do not raise a new ceiling, discard entries or shorten snapshots.
- `add` remains idempotent for an identical reference at and above 128 entries;
  incompatible collisions still reject. JSON encoding failure is invalid data,
  not `context_reference_index_too_large`.
- Projection, prompt assembly and reflection register every selected Active
  Memory reference. Do not clip selection to the remaining reference count.
  Existing scoped retrieval/selection policies retain their own semantics.
- Both capability implementations preserve slots and complete frozen snapshots
  above 128 KiB. The historical `boundedSnapshotValue`/`BoundedSnapshotValue`
  names mean a JSON copy; they must not return nil because of size.
- Both prepared envelope codecs accept internal data above 128 KiB. Keep the
  closed top-level envelope, schema version and object-shaped data/provenance.
- Core resulting-state references may extend the selected influence set. The
  internal ActionOutcome reference map has no model influence-count or byte cap;
  its ref/snapshot checks still run. The Provider-owned influence list retains
  its 24-item schema and validation contract. Do not apply that limit again to
  Core's merged map.
- Ownership, scope, revision, hash, authorized ref membership, valid JSON and
  existing Tool argument/schema checks remain required. External `Arguments`
  retain their independent 64 KiB boundary. This change creates no authority.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| Valid index >128 entries, >128 KiB total or >32 KiB single snapshot | complete context continues; pre-model capacity warning |
| Identical add at/above old count threshold | same ref and unchanged index |
| Valid internal frozen/prepared data >128 KiB | retain all slots/data through both runtimes and canonical invocation validation |
| 24 selected influences plus one actual resulting-state ref | settlement retains all 25 references |
| Provider sends >24 influences or oversized/invalid external Arguments | original schema/argument rejection |
| Foreign owner/scope, unknown ref, stale revision or hash tampering | original authority/integrity rejection |
| Malformed snapshot/envelope or non-JSON prepared data | original shape/encoding rejection; no fabricated empty context |

### 5. Good / Base / Bad Cases
Good: 180 valid refs and selected Active Memory freeze and replay intact, with a
capacity advisory. Base: smaller valid data follows the same validators without
a warning. Bad: accept the index but silently omit a large required slot, or let
Core's resulting-state ref make an otherwise valid 24-influence decision fail.

### 6. Tests Required
`context_reference_capacity_test.go` exercises the actual builder/replay validator,
180 refs, 64×4 KiB snapshots, a 40 KiB snapshot, duplicate registration, active-ref
retention, large projection freezing, both snapshot resolvers, both prepared
codecs and canonical invocation validation. Settlement regression uses
`freezeDecisionInfluences` → `frozenDecisionCausality` → `buildActionOutcomes` with
24 selected refs plus resulting state, including >128 KiB reference data. Keep
foreign identity/owner, hash tampering, >24 Provider influences and existing
model-smuggling/schema negatives. Pure tests do not prove a live workflow retry.

### 7. Wrong vs Correct
Wrong: `len(index.ByRef) >= 128` returns an error before deduplication, or a
snapshot copy returns nil above `maxToolArgumentsBytes*2`.
Correct: validate identity/scope/JSON/hash, preserve the complete data, emit
metadata-only capacity diagnostics and continue through the original pipeline.

# Exact task-scoped contract excerpts

Source: `.trellis/spec/backend/fluctlight-diagnostics-contract.md`

Source SHA-256: `56b812cd23735da6c7decbbcc363f742894ce114c9a3cf0516cfb967311556ac`

Verbatim selected complete scenario sections to prevent native context injection truncation. The original spec remains authoritative. Read further source sections if implementation touches neighboring behavior; regenerate if its source hash changes.

<!-- Original lines 732–793 -->

## Scenario: Pre-model context capacity advisories (2026-10-14)

### 1. Scope / Trigger
Oversized valid internal reference data must remain usable and inspectable even
when projection fails before any ModelRun could otherwise be recorded. Capacity
is optimization metadata, not business authority or a reason to reject context.

### 2. Signatures
`ContextReferenceIndex.capacityReport() contextReferenceCapacityReport` measures
the complete index. `App.recordContextReferenceCapacity(ctx, projection, stage)`
emits `context.reference.capacity_warning` with level `warning` after successful
reference registration at `context_projection`, `prompt_assembly` and
`reflection_context`, before their model request.

### 3. Contracts
- Warning thresholds: reference_count=128, encoded_bytes=131072 and single
  snapshot_bytes=32768. Emit only when a value is greater than its threshold.
- Owner event payload includes stage, decision=`continue_complete_context`,
  as_of, advisory_thresholds and capacity. Capacity contains reference_count,
  encoded_bytes, snapshot_bytes (sum), largest_snapshot_bytes, by_kind
  (count/snapshot_bytes), largest_snapshots and advisory_thresholds_exceeded.
- `largest_snapshots` contains at most five ref/kind/entity_id/bytes descriptors,
  sorted by bytes descending then ref. No snapshot bodies, user prose, Prompt,
  response, headers or credentials enter this event or its worker log.
- Always emit the worker `slog` warning first, with event_type, stage,
  correlation_id, fluctlight_id, source_fact_id, as_of, decision, thresholds and
  measured capacity metadata. Then attempt the existing best-effort Owner sink;
  missing diagnostic DB must not hide the operational evidence.
- Correlation uses providerCorrelation, then SourceFactID, then a stable
  context identity. The warning neither modifies the index/hash nor returns a
  business error. Existing sink writes use an independent context with a two
  second timeout; this is not a zero-latency or asynchronous delivery guarantee.
- Warning means complete internal data continues. It does not prove Provider
  success, authorize wider scope or claim that a production retry recovered.

### 4. Validation & Error Matrix
| Condition | Diagnostic / behavior |
| --- | --- |
| Valid data exceeds one or more thresholds | worker warning plus attempted Owner event; continue complete context |
| Values below/equal thresholds | no capacity warning |
| DB missing/unavailable | worker metadata still visible; sink error does not replace business result |
| Invalid identity/hash/JSON | original validation fails; no capacity bypass |
| Large private snapshot body | only descriptor and measured bytes, never its body |

### 5. Good / Base / Bad Cases
Good: an Owner can find the stage, source, oversized kind and largest entity
from worker logs even without a ModelRun. Base: small valid indexes are quiet.
Bad: only return `context_reference_index_too_large`, log every snapshot body,
or label this warning as truncation while silently discarding required context.

### 6. Tests Required
`TestContextReferenceCapacityWarningIsInspectableWithoutDatabase` captures the
real helper's JSON worker log with no DB and asserts event identity, correlation,
decision, thresholds, measurements, kind/largest-entry descriptors and absence
of private snapshot text. Size/count/rehydration and integrity regressions live
beside it. Missing DB is tested; a live DB outage and production workflow retry
remain separate environment verification.

### 7. Wrong vs Correct
Wrong: wait for a ModelRun that never started, or rely exclusively on a failed
diagnostic insert. Correct: emit safe structured worker metadata before the
best-effort sink and continue the valid internal context without data loss.
