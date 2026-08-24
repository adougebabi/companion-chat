# T01 Implementation Plan

## Entry

- Parent planning explicitly approved.
- This child reviewed, curated and started as `in_progress`.
- No other writing child/session.
- Read exact five manifest entries and D001/D003/D005-D007/D016/D020-D021/D028-D029/D031-D033.

## Owned Paths

- `.python-version`, root `pyproject.toml`, root `uv.lock`.
- `apps/core/pyproject.toml`.
- `apps/core/src/fluctlight_core/workflow_gate/`, including `api_entrypoint.py` and `worker_entrypoint.py`.
- `apps/core/tests/workflow_gate/`.
- `infra/compose/dbos-gate.compose.yml`, `infra/compose/dbos-gate.env.example`.
- Parent `research/dbos-runtime-gate-report.md` from its approved template.

## Checklist

- [ ] Pin Python/uv/DBOS/PostgreSQL dependencies and commit lockfile.
- [ ] Implement minimal committed gate intent and stable IDs.
- [ ] Implement API/Worker entrypoints and three queues.
- [ ] Implement durable sleep and fake h3/provider steps.
- [ ] Add heartbeat/timeout/cancel/crash/recovery failure injection.
- [ ] Add canonical management operation wrapper and audit record.
- [ ] Add active-history upgrade/replay and backup/restore fixtures.
- [ ] Add structured correlation and quantified resource measurement.
- [ ] Run exact commands and complete PASS/FAIL report.

## Validation Commands

```bash
uv sync --locked
uv run pytest apps/core/tests/workflow_gate -q
docker compose -f infra/compose/dbos-gate.compose.yml up -d --build
docker compose -f infra/compose/dbos-gate.compose.yml ps
uv run pytest apps/core/tests/workflow_gate -m compose -q
docker compose -f infra/compose/dbos-gate.compose.yml down
```

## Exit

- PASS: parent/check reviews report and may prepare T02 child brief.
- FAIL: stop T02+, return parent to planning and evaluate Temporal.
