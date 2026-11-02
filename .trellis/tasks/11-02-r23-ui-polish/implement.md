# Execution

1. Inspect AGENTS, frontend contracts, audit PNGs and immutable demo.
2. Reproduce cancelled chat selection leak, fix read ownership and cover cancellation, failed preflight, success, role switch and obsolete completion.
3. Apply remaining presentation fixes in owning views/shared shadcn compositions; cover every item.
4. Run focused tests, then all workspace build/lint/typecheck/tests; fix failures.
5. Check production desktop/narrow layout with isolated fixture and Chrome; keep images in /tmp.
6. Record per-item files, test names, root cause, command results and limitations. Leave working tree uncommitted.

## Completed

All eleven UI items and 13 new regressions pass. Install/build/web lint/workspace typecheck/full 877 JS tests and full Go suite pass. Seven obsolete Go assertions were synchronized with existing behavior. Chrome passes desktop/narrow and three actual chunk+data skeleton flows, with 28 PNGs exclusively in /tmp/r23-shots. See final-report.md and verification/browser-results.json. No Git operations that write the index/history; no automatic Trellis archive/journal commits.
