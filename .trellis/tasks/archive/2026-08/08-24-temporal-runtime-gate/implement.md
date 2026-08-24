# T01B Temporal Runtime Gate Plan

## Entry

- Parent DBOS FAIL/Temporal switch approved.
- Exact six-entry child manifests loaded.
- This child explicitly started as `in_progress` with exclusive writer.
- Manually read parent `READ_FIRST.md`, `decisions.md`, Workflow Direction and T01/T01B/T02 program sections before editing.
- Use `/usr/bin/python3` for Trellis lifecycle commands when pyenv cannot resolve `.python-version`; use `uv run` for application/tests.

## Owned Paths

- `.python-version`, root `pyproject.toml`, root `uv.lock`, `apps/core/pyproject.toml`.
- `apps/core/src/fluctlight_core/temporal_gate/` including `api_entrypoint.py` and `worker_entrypoint.py`.
- `apps/core/tests/temporal_gate/`.
- `infra/compose/temporal-gate.compose.yml`, `infra/compose/temporal-gate.env.example`.
- `infra/compose/run-temporal-gate.sh`.
- Existing DBOS `workflow_gate/` source/tests and DBOS gate Compose files, solely to remove them from current production/default paths while retaining archive/git evidence.
- Parent `research/temporal-runtime-gate-report.md` from approved template.

## Checklist

- [ ] Pin Temporal SDK/Server/PostgreSQL versions and locked dependencies.
- [ ] Build grouped non-HA PG-visibility topology with optional services disabled.
- [ ] Implement API/Worker split and three task queues.
- [ ] Implement stable intent/Workflow/Provider IDs and one-result recovery fixture.
- [ ] Implement timers, Signals/Queries/Updates, pause/resume/cancel/repair operations.
- [ ] Implement 15-minute fake h3 heartbeat/timeout/cancel and live kill/restart tests.
- [ ] Export v1 histories; replay v2; prove current Worker Deployment Versioning and rollback/drain.
- [ ] Implement continue-as-new and reset/restart/backup/restore fixtures.
- [ ] Remove DBOS dependency/scripts/source/tests/Compose from current production/default paths; preserve archive/report/commits.
- [ ] Add Ruff/mypy/full-suite gates and trap-based clean-volume runner.
- [ ] Run 12-hour target-NAS soak with fixed workload/restarts/leak/backlog/disk criteria.
- [ ] Measure all RSS/CPU/connections/readiness/recovery/disk thresholds three times.
- [ ] Complete report and PASS/FAIL parent handoff.

## Validation Commands

```bash
uv sync --locked
uv run ruff format --check apps/core
uv run ruff check apps/core
uv run mypy apps/core/src apps/core/tests/temporal_gate
uv run pytest apps/core/tests -q
docker compose -f infra/compose/temporal-gate.compose.yml config
./infra/compose/run-temporal-gate.sh --clean --functional
./infra/compose/run-temporal-gate.sh --clean --soak-hours 12 --require-target-nas
```

## Exit

- PASS: report/check/commit/archive T01B, parent records final Temporal runtime and prepares T02 child-specific brief.
- FAIL: report/check/commit/archive failed gate, keep T02-T12 blocked and return workflow-runtime planning.
