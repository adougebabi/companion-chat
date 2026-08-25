# T05 No-History Handoff Dry Run

Date: 2026-08-25

This dry run reconstructs the T05 implementation boundary without relying on
conversation history. The child owns cognition and diagnostics code plus the
`0004` migration and explicit registration seams listed in `implement.md`.

## Inputs

- Parent `READ_FIRST.md`, `decisions.md`, `design.md`, and `implement.md`.
- T05 `prd.md`, `design.md`, and the T05 implementation brief.
- The cognitive runtime, diagnostics, provider, workflow, persistence, and
  event contracts in `.trellis/spec/backend/`.
- Existing T02/T03/T04 public modules and the current uncommitted T03/T04
  integration worktree.

## Reconstructed Execution

1. Define framework-free assessment, decision, action, realization, reflection
   and provider port contracts.
2. Add T05-owned inbox/cognition/diagnostics tables and the linear `0004`
   migration, registering metadata without changing prior revisions.
3. Implement short transaction phases with stable IDs, CAS and idempotency;
   external provider calls happen only through injected ports after commit.
4. Implement typed-redacted diagnostics persistence/query/export/retention and
   failure-isolated sink behavior.
5. Register a cognition worker seam and add focused implementation checks.

## Ownership / Exclusions

The brief's owned and forbidden path lists are sufficient to prevent edits to
the frozen legacy system and future child modules. T06 owns conversation and
stream transport; later children own memory, life world, media, UI, operations
and cutover. T05 does not claim final acceptance.

## Gates

The local checks in `implement.md` are executable with the repository's pinned
Python environment. T12 must re-run real PostgreSQL, provider failure,
cross-Fluctlight ordering, Redis replay, Temporal restart, security/redaction,
and cross-module scenarios under the listed coverage IDs.

## Remaining Carry-Forward Risks

- T02's platform inbox constraint name and aggregate sequence implementation
  require later integration correction; T05 does not silently patch them.
- T03/T04 shared migration/API readiness changes remain uncommitted and must
  be included in the eventual T12 migration and generated-contract gate.
- Actual provider/network and Temporal runtime behavior is injected/evidenced
  locally; no child-level production acceptance is claimed.

Conclusion: no unresolved planning ambiguity prevents starting T05 under the
user-authorized one-child/one-writer execution order.
