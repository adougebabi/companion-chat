# Capacity diagnostics audit

Read-only scout, 2026-10-14. No tests, DB, or network used. Paths below are relative to `apps/core-go/internal/core/`.

## Pre-model boundary

- `intelligence.go:437`, `App.BuildContextProjectionFor`: the final stable snapshot builds references; any builder error immediately returns an empty projection. `:440-451` then computes remaining count capacity, silently slices ActiveMemories, sets `TruncatedReason = "context_reference_limit"`, and adds active memory refs. This is the natural producer seam before returning the complete projection, but error diagnostics must be produced within/beside the builder because it currently retains its local index until success.
- `context_reference.go:245`, `buildContextReferenceIndex`: local `index` accumulates references; only `:446-450` validates and installs it on projection. Count/add errors and aggregate validation errors leave `projection.ReferenceIndex` empty. Capturing only the returned projection will therefore miss useful counts.
- `cognition_agent.go:84-87`: builder errors propagate directly, before prompt assembly and provider calls. Provider correlation fallback is set only at `:121` after builder and assembly. A producer at projection time can use `providerCorrelation(ctx)` (`diagnostics.go:62`), but should provide a deterministic fallback using request.SourceFactID when correlation is empty. Request fields available: FluctlightID, SourceFactID, ConversationID, TriggerSource, WorkingProfileID, MemoryOperation (`memory_retrieval.go:100`). Use SourceFactID as causation key and keep raw user text out.

## Existing best-effort producer and fallback

- `diagnostics.go:357`, `App.recordDiagnosticEvent(ctx, eventType, severity, fluctlightID, causationID, correlationID, payload)` directly creates providerRuntimeSupport from a.DB and delegates. It is available before provider invocation and does not depend on ProviderClient.
- `diagnostics.go:678-687`, `providerRuntimeSupport.RecordDiagnosticEvent`: nonfatal; uses detached context with 2 second bound (`:28`) and calls persistence-failure warning on error. `persistDiagnosticEvent`, `:690-708`, handles absent DB with ErrDiagnosticsUnavailable, redacts and marshals payload, and persists to diagnostic_events. Empty correlation gets a time-derived fallback, so explicit caller correlation is preferable for joins.
- `lifecycle_diagnostics.go:333-356`, `recordDiagnosticPersistenceFailure`: operational slog warning includes component, stage, correlation_id, error_type, boundedOperationalCause and caller attributes. Warnings are throttled once per minute by component:stage:error-type (global, not per correlation). Existing generic event wrapper passes only event_type. Consequently DB-down fallback currently **loses all capacity metadata**. A new capacity producer should ensure bounded metadata reaches a safe operational warning, or explicitly pass it through a persistence seam without duplicating generic fallback warnings. Do not pass raw snapshots as slog attributes.
- Recommended metadata: entry count, total encoded bytes, total snapshot bytes, overhead bytes, per-kind count/bytes and bounded top-N snapshot descriptors (kind/ref or stable identifier, snapshot byte count). Deterministic ordering makes diagnosis/tests stable. Counts and byte sizes can explain count, aggregate, and single-snapshot threshold excess independently. Preserve serialization failure as its own cause.

## Pure tests / capture seam

- `context_reference_capacity_test.go:12` already has uncommitted pure builder/validator regression tests for 180 refs, 64 x 4096 byte aggregate, 40 KiB single snapshot, and JSON round-trip. `:49` covers idempotent duplicate at 128 count. These can be extended with pure metadata assertions, no DB.
- `provider_diagnostic_privacy_test.go:9` demonstrates a collector embedding ProviderRuntimeSupport and overriding one method. That pattern works for new event collector types if a helper accepts the narrow RecordDiagnosticEvent interface. Existing App.recordDiagnosticEvent always binds a.DB, so assigning Provider.runtime will **not** intercept it.
- `diagnostics_test.go:227-228` explicitly calls `(&App{}).recordDiagnosticEvent(nil, ...)` to verify nonfatal no-DB behavior. `:387-389` shows bytes.Buffer + slog.SetDefault capture with cleanup; safe fallback behavior can be checked without DB, accounting for/resetting the throttling map key.

## Cause collapse and adjacent limits

- `context_reference.go:155-157` currently maps both `json.Marshal(index)` failure **and** encoded-byte excess to `context_reference_index_too_large`. This is the exact misleading cause-collapse point. `:187-189` likewise maps marshal failure, empty snapshot, and snapshot-byte excess to `context_reference_snapshot_invalid`.
- `context_reference.go:519-529`, `contextReferenceIndexFromValue`: marshal/unmarshal errors become generic index_invalid; Validate errors propagate unchanged. Important replay seam after removing advisory capacity stops.
- `app.go:195-197`, `jsonBytes`: discards json.Marshal errors. Avoid using this helper for new metrics when serialization error must remain inspectable.
- `provider_ref_codec.go:55-68`, registerIndex: no count/byte cap; simply registers aliases/known refs and propagates alias collision. No extra opaque size wrapper there.
- Adjacent downstream limit: `action_outcome.go:77-78` maps oversized Expected, Observed, or ContextReferences to action_outcome_payload_too_large, but it is settlement-time, not this pre-model failure. Do not broaden this task accidentally.
