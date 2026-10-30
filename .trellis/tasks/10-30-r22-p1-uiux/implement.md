# Execution

1. Load specs and locate owning components/controllers.
2. Fix independent preview/people/empties/world, task detail/skip, defaults/local backup slices with tests.
3. Run affected regressions; update obsolete expectations only for explicitly superseded P1 behavior.
4. Run pnpm install, build/lint/typecheck/test and affected Go package checks.
5. Check production desktop/narrow screenshots outside Git if practical.
6. Review staged diff/images, document per-issue files/tests/results and remaining deviations; conventional local commit; no push.

## Final status

- P1 implementation complete, 23 new DOM tests plus Core default/media regression coverage.
- pnpm install/build/lint/typecheck pass; all 864 JS/TS tests pass. Decision race/vet/build and Core Kev/MediaBranch slice pass.
- Complete Core suite reveals five failures reproduced at original HEAD; the transient retry timeout passes alone. These are outside P1 scope.
- Headless Chrome production desktop/mobile verification passes (16 product + two immutable Demo screenshots, /tmp/r22-shots).
- Original .git is read-only, so the exact changes are committed with the original image hook in a writable Git directory on the same named branch; original checkout promotion remains blocked. No push.
