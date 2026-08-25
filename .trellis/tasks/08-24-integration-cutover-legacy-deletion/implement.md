# T12 Final Integration / Cutover / Legacy Deletion Implementation Brief

## Status

Parent-authorized final integration brief. This is the only child allowed to
claim final acceptance or perform the one-time cutover; deletion is conditional
on every Required gate and scope guard passing.

## Dependency

T03-T11 handoffs are present in the current serialized worktree. Their local
results remain evidence only and are re-run here; the current pre-scan records
that legacy production paths still exist, so deletion is initially gated.

## Owned Paths

- `infra/acceptance/**` final gate/scope/deletion-manifest scripts
- `.trellis/tasks/08-24-integration-cutover-legacy-deletion/handoff.md`
- Final cutover/deletion targets only after all pre-cutover gates pass:
  `server/**`, legacy `web/**`, legacy `test/**`, root `Dockerfile`, root
  `compose.yaml`, legacy root package scripts/dependencies, old CI/docs/routes.

## Forbidden Paths Before Gates

- Do not delete or modify legacy targets while a Required gate, generated
  contract check, Compose readiness, backup/restore proof or scope guard fails.
- Do not change `.trellis` historical evidence to hide old terms or failed
  scans. Do not claim Future-only/reserved/placeholder-only capabilities.

## Decisions And Contracts

Implement without changing D002-D006, D010-D012, D017-D020, D023-D040. The
assigned spec union is every `fluctlight-*.md` backend contract plus the
frontend guides and T03-T11 handoffs. Required positive acceptance covers only
Must Rebuild and closed Incomplete Old capabilities. Future-only/reserved and
placeholder-only capabilities get a negative scope guard only.

## Final Gate Checklist

1. Validate all child manifests/handoffs and generated OpenAPI/client no-drift.
2. Run Python format/lint/mypy/tests, BFF tests/typecheck/build, browser-client
   generation/typecheck, Web test/typecheck/build and Compose config/readiness.
3. Run Required contract/error/security/redaction/stream/media/migration/
   backup/restore/upgrade evidence checks and record failures without masking.
4. Run excluded-scope guard and legacy deletion scan; write a deletion
   manifest. Abort before deletion on any failure.
5. If all gates pass, execute one cutover, remove legacy targets, regenerate
   lock/client artifacts, then run post-cutover smoke/readiness checks.

## Final Commands

```bash
export FLUCTLIGHT_ENV_FILE=/absolute/path/to/private/fluctlight.env
.venv/bin/ruff format --check apps/core/src apps/core/tests
.venv/bin/ruff check apps/core/src apps/core/tests
.venv/bin/mypy --follow-imports=skip apps/core/src apps/core/tests
.venv/bin/pytest -q apps/core/tests
infra/acceptance/validate-handoffs.sh
infra/acceptance/excluded-scope-guard.sh
infra/acceptance/legacy-scope-guard.sh  # expected to fail before cutover
pnpm --filter @fluctlight/core-client generate && pnpm --filter @fluctlight/core-client typecheck
pnpm --filter @fluctlight/bff generate:browser-client && pnpm --filter @fluctlight/bff typecheck && pnpm --filter @fluctlight/bff test && pnpm --filter @fluctlight/bff build
pnpm --filter @fluctlight/browser-client generate && pnpm --filter @fluctlight/browser-client typecheck
pnpm --filter @fluctlight/web test && pnpm --filter @fluctlight/web typecheck && pnpm --filter @fluctlight/web build
docker compose --env-file "$FLUCTLIGHT_ENV_FILE" -f infra/compose/fluctlight.compose.yml config
infra/compose/run-platform-smoke.sh --clean
infra/acceptance/run-active-workflow.sh
infra/acceptance/run-auth-domain-smoke.sh
infra/acceptance/run-media-smoke.sh
infra/acceptance/run-provider-success-smoke.sh
infra/acceptance/run-redis-recovery-check.sh
infra/acceptance/run-redis-poison-check.sh
infra/acceptance/run-redis-gap-check.sh
infra/acceptance/run-backup-restore-check.sh
infra/acceptance/run-pgvector-benchmark.sh
```

## T12 Coverage Union

Consume all `T03-*` through `T11-*` IDs from child handoffs plus required
cross-module capability, failure, security/redaction, generated-contract,
Compose, migration, backup/restore/upgrade and deletion IDs in the parent
capability inventory. Record each as pass/fail/evidence-pending; no local child
result substitutes for a missing real-runtime proof.

## Rollback Point

Before deletion, preserve the frozen legacy tree and all backup manifests. If a
gate fails, stop with the deletion manifest and retain the worktree. After a
successful one-time deletion, rollback is restore-from-backup into a clean
checkout, not an in-place resurrection of old runtime files.

## Final Acceptance And Cutover

1. Consume all T03-T11 implementation-evidence handoffs and the complete assigned-spec union.
2. Run full lint/type/test/build, complete Compose, Required capability matrix, cross-module e2e/failure/security/redaction, backup/restore/upgrade, excluded-scope guard and legacy-deletion proof.
3. Abort before deletion if any Required gate, cleanup proof or scope guard fails; retain the frozen old system as development reference.
4. Execute exactly one product cutover after all pre-cutover gates pass.
5. Run post-cutover smoke/readiness checks, complete legacy deletion and publish the final production handoff. No next child follows.

## Final Exit

Repository contains one production implementation, no prohibited compatibility/scaffolding, and one complete Compose deployment passes final acceptance. Future-only/reserved/placeholder-only capabilities are not claimed as delivered.
