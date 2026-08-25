# T11 Implementation Evidence Handoff

Status: implementation evidence complete; `acceptance_owner=T12`; `acceptance=pending`.

## Changed Paths

- `apps/core/src/fluctlight_core/operations/backup.py` typed JSON manifest,
  object entries, env/key presence boundary, verification and cleanup/restore
  plan values.
- `apps/core/src/fluctlight_core/entrypoints/backup.py` safe manifest/verify
  operator command and `fluctlight-backup` script registration.
- `infra/backup/README.md`, `infra/backup/recovery-drill.sh` and MinIO operator
  notes for PostgreSQL/object/.env/Temporal restore and secret re-entry.
- T11 operations and contract tests.

## Implementation Evidence

```text
.venv/bin/pytest -q apps/core/tests
108 passed
.venv/bin/ruff check apps/core/src apps/core/tests
All checks passed
.venv/bin/mypy --follow-imports=skip <T11 sources>
Success: no issues found in 3 source files
```

## Produced Contracts / Schema

- Manifest records schema revision, PostgreSQL snapshot ID, private bucket
  object key/version/size/SHA-256, env field presence, settings-key presence,
  and Temporal default/visibility/namespace/active workflow IDs.
- Verification reports revision/count/env/key issues; it never serializes
  secret values. Cleanup is represented as an explicit dry-run plan.
- Recovery drill refuses non-temporary targets and documents explicit Alembic
  upgrade plus restored Temporal active-workflow resume.

## Remaining Risks / Excluded Scope

- Real `pg_dump`/object transfer, empty-deployment restore, previous-release
  migration, restored Temporal Server/Worker resume, and backup security remain
  T12-only. No automatic migration or fixed-duration resource gate was added.
- Manifest CLI deliberately requires operator-supplied snapshot/object review;
  it does not claim a generated empty manifest is restorable.

## T12 Coverage

Re-run `T11-BKP-01`, `T11-BKP-02`, `T11-UPG-01`, `T11-UPG-02`, `T11-OPS-01`,
and `T11-SEC-01` from the child brief.

Rollback point: remove only T11-owned operations/docs/scripts before T12 if the
manifest/recovery seam cannot be satisfied.
