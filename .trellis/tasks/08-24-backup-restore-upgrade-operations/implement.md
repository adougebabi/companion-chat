# T11 Backup / Restore / Upgrade Operations Implementation Brief

## Status

Parent-authorized implementation brief for the seventh executable child in the
T05-T12 chain. This child produces implementation evidence only; T12 remains
the acceptance owner.

## Dependency

T10 browser/Control Center handoff is available. T11 consumes public storage,
configuration and workflow boundaries without adding a second runtime.

## Owned Paths

- `apps/core/src/fluctlight_core/operations/**`
- `apps/core/src/fluctlight_core/entrypoints/backup.py`
- `apps/core/tests/operations/**`, `apps/core/tests/contract/test_t11_*.py`
- `infra/backup/**`, `infra/minio/README.md`
- `apps/core/pyproject.toml` script registration only if required

## Forbidden Paths

- Frozen legacy `server/**`, `web/**`, `test/**`, old SQLite backup/import and
  root legacy runtime files.
- Automatic API/Worker migration, plaintext secret archives, second workflow/
  queue runtime, or fixed-duration/resource soak gates.
- T12 cutover/deletion and final acceptance artifacts.

Carry-forward from T01B: T12 final acceptance must boot restored Temporal default+visibility and resume an active workflow; this child only prepares implementation seams and evidence. Do not add fixed-duration/resource gates.

## Decisions And Contracts

Implement without changing D001-D003, D018-D020, D024-D025, D029-D033,
D035-D036 and D039. The assigned contracts are
`fluctlight-persistence-contract.md`, `fluctlight-media-contract.md`,
`fluctlight-configuration-contract.md`, `fluctlight-workflow-contract.md`,
and `fluctlight-temporal-gate-contract.md`. Manifest verification is explicit;
API/Worker only verify the deployed migration revision and never auto-upgrade.

## Implementation Checklist

1. Add typed backup manifest, component checksums/counts, env/key presence
   boundary and verify/restore plan contracts.
2. Add operator CLI/library for manifest creation/verification, cleanup plan,
   previous-release migration command handoff and Temporal default/visibility
   restore/resume seam.
3. Add NAS backup/recovery/secret re-entry documentation and a disposable
   recovery-drill script with no destructive default target.
4. Add focused unit/contract checks; T12 performs real dumps, object restore,
   migration upgrade and active workflow resume.

## Implementation Checks

```bash
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline ruff check apps/core/src/fluctlight_core/operations apps/core/src/fluctlight_core/entrypoints/backup.py apps/core/tests/operations apps/core/tests/contract/test_t11_*.py
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline pytest -q apps/core/tests/operations apps/core/tests/contract/test_t11_*.py
```

## T12 Coverage IDs

`T11-BKP-01` PostgreSQL/object/.env manifest and integrity; `T11-BKP-02`
empty-deployment restore and sampled checksum/count; `T11-UPG-01` previous
release migration/explicit head; `T11-UPG-02` restored Temporal default/
visibility and active workflow resume; `T11-OPS-01` cleanup/retention and
secret re-entry procedure; `T11-SEC-01` no plaintext credentials in manifest.

## Rollback Point

Before T12 starts, remove only T11-owned operations/docs/scripts if the
manifest/recovery seam cannot be satisfied. Preserve T05-T10 and all prior
unrelated worktree edits.

## Implementation Evidence Handoff

Record changed paths, operator/contract artifacts, implementation-check
commands/results, unresolved recovery risks, excluded scope, T12 coverage IDs
and rollback point. State `acceptance_owner=T12` and `acceptance=pending`; no
child PASS, production readiness or cutover is established here.
