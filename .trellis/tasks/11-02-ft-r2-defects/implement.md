# Execution
1. D8 first: explicit world selection + no implicit completion; atomic base scaffold and creation regressions.
2. Diagnose evidence and implement D7/D9/D10 with DB/workflow tests.
3. D11 config + middleware tests.
4. Full required validation with disposable PostgreSQL; fix regressions without weakening assertions.
5. Record D1–D11 Chinese final report, old assertion changes, migration/config requirements and limitations. Leave all work uncommitted.

## D12 completed
- Added exact worldId/intentType diagnostic filters and regenerated Browser OpenAPI.
- Added selected-world automatic event records, execution outcomes and clock task links; inactive-world browsing remains read-only.
- Included permanent clocks in management task reads, with deduplication, server type filtering, world labels and independent cursor/retry state.
- Targeted validation: 61 Web + 1 BrowserClient tests; 4 Core + 1 browser-boundary Go top-level tests (2 boundary subtests), all pass with the supplied isolated PostgreSQL. Web typecheck/lint and git diff --check pass.
- No old assertion changed; no Git staging/commit. See d12-validation.md for evidence and limits.
