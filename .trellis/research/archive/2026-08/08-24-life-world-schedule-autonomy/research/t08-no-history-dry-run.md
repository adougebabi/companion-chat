# T08 No-History Handoff Dry Run

Date: 2026-08-25

T08 consumes T07 public Memory/Relationship boundaries and T04 Goal/Intention
services. It owns only the Life World and Autonomy paths in `implement.md`.

## Execution

1. Define aware Event, full local-day Schedule and Context authority values.
2. Add `0007` Life World/Autonomy tables and migration.
3. Implement immutable acceptance, future-only replan, explicit pending state,
   timezone/DST handling and Event > Schedule > pending resolution.
4. Implement typed autonomy policy snapshots, frozen action IDs, governance and
   reconciliation without rewriting history.
5. Run focused Python checks and hand Temporal/restart/real-PostgreSQL and
   cross-module validation to T12.

## Exclusions / Risks

T08 does not implement Moments/Media, UI, backup or legacy deletion and does
not recreate T04 Goals/Intentions. Semantic planning remains an injected LLM
port; no default routine or natural-language heuristic is added.

Conclusion: T07 handoff plus the assigned contracts resolve the planning
boundary required to start this child.
