# T12 Final Integration Evidence Handoff

Status: final acceptance pending; cutover and legacy deletion **not executed**.

## Handoff Completeness

- T03 and T04 handoffs are present; T05-T11 handoffs remain implementation
  evidence with `acceptance_owner=T12` and `acceptance=pending`.
- JSONL context validation and `infra/acceptance/validate-handoffs.sh` pass, but
  `task.py validate` does not prove dependency completion. T02-T11 task
  metadata remains `in_progress`/unmerged, so the parent dependency gate is
  not claimed.

## Validation Matrix

| Gate | Result | Evidence |
| --- | --- | --- |
| T03-T11 handoffs/manifests | evidence present, dependency gate pending | `validate-handoffs.sh` checks every handoff's T12 owner/status, concrete coverage IDs, exclusions and rollback; JSONL validation passes, but task metadata is still in progress |
| Core format/lint | pass | `.venv/bin/ruff format --check`, `.venv/bin/ruff check` |
| Core typecheck | pass | `.venv/bin/mypy --follow-imports=skip apps/core/src apps/core/tests`, 170 files |
| Core tests | pass | `.venv/bin/pytest -q apps/core/tests`: `126 passed, 1 skipped` (the skip is the opt-in loopback test); streaming order/cancellation regression coverage is included |
| Core client generation/typecheck | pass | generate + `tsc --noEmit` |
| BFF typecheck/build/tests | pass | typecheck/build; external `tsx --test`, 12 passed |
| Browser client generation/typecheck | pass | generate + `tsc --noEmit` |
| Web test/typecheck/build | pass | 3 tests passed (keyboard/live-region/form boundary included); `vue-tsc`; `vite build` |
| Live browser DOM smoke | partial | In-app Browser verified the Owner boundary and, after the generated-client URL/bound-fetch fixes, authenticated `New conversation` shell; create-and-chat, successful token stream, disconnect cancellation, a11y/mobile and logout/revocation remain pending for manual follow-up |
| Generated artifact determinism | pass | second generation produced identical SHA-256 values |
| Compose config | pass | `docker compose ... config` |
| Compose readiness/smoke | pass | Disposable private env override; guarded PID-unique project started migration/minio-init/PostgreSQL/Redis/Temporal/Core/Worker/BFF/Web and container-internal BFF health fallback passed |
| Excluded scope guard | pass | no excluded production capability references |
| Legacy deletion guard | fail as expected | exact legacy targets remain (`server/`, legacy `web/`, `test/`, root Docker/Compose, npm lock, `.env.example`, `.nvmrc`); the content scan also reports the frozen README/env references |
| Provider success | pass, disposable configured endpoint | Guarded `run-provider-success-smoke.sh`: real HTTP fixture container, six-role preflight, two independently flushed SSE token frames before terminal, conversation reply, embedding=`ready`, and 3 provenance rows passed |
| Redis recovery | pass, disposable volume loss | Guarded `run-redis-recovery-check.sh`: one committed outbox event consumed by exactly 3 groups, Redis volume removed/recreated, replay stream length exactly 1, inbox count preserved, and Worker publisher/replay ran outside the PostgreSQL read transaction |
| Redis failure policy | pass for bounded unit and disposable runtime behavior | `run-redis-poison-check.sh` injected a failing event with `max_attempts=1`, verified `platform_consumer_failures.status=quarantined`, attempt `1`, and ACK only after quarantine |
| Redis aggregate gap | pass, disposable out-of-order drill | `run-redis-gap-check.sh` delivered sequence `2` before `1`, verified the gap stayed pending, then replayed sequence `2` after the head advanced and observed both ACKs |
| pgvector benchmark | pass, representative disposable table | Guarded `run-pgvector-benchmark.sh`: 2,000 `vector(3)` rows, HNSW index scan and measured execution time `0.102 ms`; cutover deliberately keeps HNSW disabled until a stable production model dimension/workload is approved |
| Real PostgreSQL/MinIO/Temporal recovery | pass for disposable smoke | Guarded Compose active-workflow gate completed `PlatformControlWorkflow` with `COMPLETED`, history length 10, no pending activities and `fluctlight.platform-v1` deployment metadata |
| Backup/restore/upgrade | pass, disposable storage restore checks | Guarded `run-backup-restore-check.sh`: final `0012` app schema (68 public tables) and row counts matched, pgvector `0.8.6/vector` plus FTS GIN, Temporal restore-check databases had 39/3 public tables, MinIO object copy matched and manifest create/verify passed; active-workflow resume is recorded by the separate Temporal gate |
| Auth/domain smoke | pass for explicit failure boundary | Guarded Owner setup/Core login, Fluctlight creation with materialized Actor, exact Conversation participant linkage and unconfigured-Provider error stream passed |
| Media proxy smoke | pass for proxy/authorization boundary | Guarded disposable MinIO object + PostgreSQL ready asset, Owner authorization, BFF Range proxy returned `206`, body `media`, `Content-Range: 0-4/17`; full intent/upload/provider recovery lifecycle remains covered by module tests, not this seed script |

## Implemented Scope

T03-T11 logic and contracts are present through Alembic head
`0012_t12_consumer_effects`, with generated Core/BFF/browser clients, responsive
Web Control Center, authenticated login boundary, CSRF/CORS transport,
Provider/embedding workflows, visible NDJSON token drafts, Redis outbox/replay
consumers with split read/publish/mark phases, backup manifest tooling and
acceptance/scope scripts.
Child-local evidence remains non-authoritative by design.

## Cutover Decision

The one-time cutover is still gated. Disposable Compose, configured Provider
success, Redis volume-loss replay, auth/domain/media, CSRF and strengthened
backup checks now pass. Remaining required evidence is full browser/a11y
security acceptance, task dependency completion/merge review, and the approved
legacy deletion step. HNSW is intentionally disabled for this cutover because
embedding model dimensions are not yet a single stable production contract;
the exact pgvector operator path and FTS GIN index remain authoritative.

`infra/acceptance/deletion-manifest.txt` records the exact legacy targets still
present. `infra/acceptance/legacy-scope-guard.sh` must pass only after the
approved final deletion step and the post-cutover content scan; it is
independent of the private Compose environment.

## Remaining Risks

- T02 shared inbox constraint/aggregate-sequence issue is corrected for the
  current event producers; transactional inbox reservation, reclaim and
  published-outbox replay are covered by unit and disposable recovery checks.
- T05/T06 production composition wires configured Provider runtime, cognition
  responder/activity and explicit failure handling; successful configured
  Provider execution is now proven against the disposable HTTP fixture.
- T07 creates the real pgvector extension, vector column, FTS GIN index and
  embedding workflow; JSONB remains a rebuildable projection. Embedding work
  now records pending/failed/stale states, validates target revisions and
  model-wide dimensions. The configured Provider smoke now creates a real
  Memory and observes embedding=`ready`; a representative HNSW benchmark passes,
  but HNSW is intentionally disabled until a stable production model
  dimension/workload contract exists.
- T10 local HTTP secure-cookie behavior follows the Web trusted-origin scheme
  (`13001`) and double-submit CSRF is tested; the login boundary and
  unauthenticated live DOM smoke passed, but full authenticated browser/a11y/
  security acceptance remains.
- T11 manifest CLI requires operator-supplied PostgreSQL/object snapshot
  inventory; it does not claim an empty manifest is restorable.
- Disposable active-workflow evidence: `PlatformControlWorkflow` completed on
  the interaction queue with history length 10, no pending activities and
  `fluctlight.platform-v1` version metadata; the script cleans its named
  volumes/network on exit.
- Disposable auth/domain evidence: `run-auth-domain-smoke.sh` created an Owner,
  logged in through Core, created a `fluctlight_...` plus matching `actors` row,
  submitted a Conversation with the Fluctlight participant and observed the
  bounded Provider error without fabricating a reply.
- Disposable media evidence: `run-media-smoke.sh` wrote a versioned private
  MinIO object, inserted a ready authoritative asset row, and read it through
  the Core/BFF Range path with the resolved Owner session.
- Media Core/BFF proxy and real object authorization passed the disposable
  Range smoke; current-session revocation, provider configuration UI and local
  HTTP cookie policy are wired. The Provider success gate uses a configured
  HTTP fixture; a user-owned external Provider endpoint remains an operational
  deployment check rather than a fabricated acceptance claim.
- Redis publication/rebuild now reads PostgreSQL pages, closes the read
  transaction, publishes outside PostgreSQL, and CAS-marks `published_at` in a
  second short transaction. Durable failure attempts now persist through the
  `0010_t12_event_failures` table and quarantine after the bounded policy;
  aggregate head/gap handling and three durable group effect records are now
  persisted and tested, with Redis recovery proving exactly 3 effects after
  replay; the poison and out-of-order drills prove bounded quarantine and gap
  replay. Provider realization now forwards flushed chunks through the Core
  NDJSON stream and the fixture smoke proves incremental token ordering; the
  browser/BFF disconnect and no-later-write acceptance remains pending.
- Worker long-running task supervision now detects unexpected Temporal task
  exit and logs bounded error classes; deployment build IDs can be supplied
  through `FLUCTLIGHT_BUILD_ID` while preserving the `platform-v1` default.

Acceptance owner remains T12. No production readiness or cutover is claimed.
