# Implementation and verification

1. Read AGENTS and frontend conventions; inspect supplied screenshots and owning views.
2. Add nginx cache policy and root lazy-chunk recovery.
3. Adopt world RadioGroup cards; fix library state and both empty-state flows.
4. Add per-item behavioral regressions and migrate obsolete button/label assertions without reducing coverage.
5. Run only affected Node tests with `/tmp/node-v24.12.0-linux-x64/bin`, web typecheck, lint and build.
6. Review the diff and record results plus every changed old assertion. Leave all changes uncommitted.

All six steps completed. New tests 8/8, related tests 75/75, final strengthened
r20 assertions 10/10, web typecheck/lint/build and nine production-browser records
pass. See `final-report.md` and `browser-results.json`. No commit or staging.
