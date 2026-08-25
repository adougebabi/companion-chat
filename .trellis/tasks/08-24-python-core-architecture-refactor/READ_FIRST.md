# Read First: Fluctlight Clean-Start Rebuild

## Purpose

This is the mandatory entrypoint for any new planning, implementation, checking, or integration session. Do not infer current architecture from the frozen old code or prior chat history.

## Current Phase

Parent task status: `planning`. No implementation is authorized until the final planning artifacts are reviewed and an implementation child task is explicitly created/started.

T01 DBOS and T01B Temporal are archived evaluations. Parent D020/D036 accept Temporal core and remove resource-duration/soak blockers. T02 is the next child, but it still requires its own approved brief/dry run and explicit start. T03 remains `in_progress` with implementation evidence and no child acceptance; T04 is active under the explicit D038 implementation exception with no child acceptance; T05-T12 remain planning outlines. D038 does not establish T03/T04 PASS or production readiness and defers Docker/full-stack runtime gates for the current session. T12 owns the final acceptance of T03-T11 scope.

## Read In This Order

1. `prd.md` - product scope, hard constraints, acceptance criteria, unresolved decisions.
2. `decisions.md` - numbered decisions that child tasks cannot silently change.
3. `design.md` - target topology, module boundaries, data flow, consistency, runtime and deployment.
4. `research/capability-inventory.md` - everything to rebuild, close, delete, or leave future-only.
5. `implement.md` - ordered child tasks, dependencies, ownership, gates, validation and rollback.
6. Root `CONTEXT.md` - canonical Fluctlight domain language.
7. Only the `.trellis/spec/backend/fluctlight-*.md` files listed by the assigned child task/manifests.

## Hard Prohibitions

- Do not implement in the parent planning task.
- Do not modify frozen old `server/`, old `web/`, or old `test/` to keep old behavior/tests green.
- Do not add compatibility routes, DTOs, state import, dual read/write, old job recovery, old terminology aliases, or old media locators.
- Do not use code regex/keywords/default semantics for perception, appraisal, relationship meaning, Memory significance, Goals/Intentions, decision, or reflection.
- Do not let Node BFF access PostgreSQL, Redis domain state, Temporal/workflow runtime, or domain repositories.
- Do not introduce Celery, another task queue, a second workflow engine, a vector database, or an external telemetry stack.
- Do not change a numbered decision inside a child task. Return to parent planning with evidence and a proposed replacement.

## Implementation Shape

- New code lives under `apps/`, `packages/`, `infra/`, and new-system `tests/`.
- Product delivery is one final cutover. T03-T11 are internal construction/evidence tasks only; T12 owns final verification and cutover.
- T01B Temporal runtime gate must pass before domain implementation proceeds.
- Every child task declares owned modules/paths, allowed shared files, required decisions/specs, entry criteria, implementation-check commands, T12 coverage IDs, and handoff artifacts. T03-T11 do not own acceptance tests.
- Shared Alembic graph, OpenAPI artifacts/generated clients, Compose, root workspace files, specs, and final integration are edited only by their assigned integration owner.
- Default execution is one active child + one writing implementation session. Additional sessions are read-only research/check sessions unless the parent explicitly authorizes a non-overlapping worktree split.
- A program-level T02-T12 outline is not executable. The parent must first create a child-specific brief/manifests/paths/commands, T12 coverage IDs/exclusions, and pass a no-history handoff dry run, following T01's model. T04 satisfies this under D038 for implementation evidence only, with its deferred runtime gates recorded for T12.

## Before Editing

1. Run Trellis session/bootstrap context and identify the exact child task. Because the application `.python-version` may not exist in local pyenv, invoke Trellis scripts with `/usr/bin/python3`; use `uv run` only for application/test commands.
2. Confirm every dependency child is completed and merged.
3. Confirm the child task status is `in_progress`.
4. Read the exact code to be changed plus the assigned spec/research context.
5. Check current worktree status and preserve unrelated changes.

## Before Reporting Completion

- T02 keeps its platform implementation/readiness checks. T03-T11 may run only minimal implementation checks needed to develop safely; those results are evidence, not child acceptance, and must not be reported as PASS or production readiness.
- T12 alone reruns the complete new-system validation matrix: lint/type/test/build, Compose, Required capability scenarios, cross-module e2e/failure/security/backup/restore and legacy-deletion proof. Future-only/reserved/placeholder-only capabilities are excluded from positive validation.
- Update every T03-T11 handoff with changed paths, implementation-check commands/results, remaining risks, produced contract/schema artifacts, T12 coverage IDs, excluded scope, and `acceptance_owner=T12` / `acceptance=pending`.
- Do not mark product delivered or cut over until the final integration/deletion task passes the full capability matrix.
