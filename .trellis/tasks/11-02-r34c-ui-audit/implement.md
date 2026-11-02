1. Inspect audit evidence and owning code.
2. Parallel independent world/wizard, task diagnostics, remaining UI/backend fixes with regressions.
3. Run targeted Node/Go tests only, web lint/typecheck/build.
4. Serve production dist with isolated fixtures; headless Chrome screenshots in /tmp/r34c-shots.
5. Review diff and record per-item roots/files/tests/components and old assertion changes. Leave uncommitted.

Completed: all mandatory audit behaviors have per-item regressions; targeted JS 157/157, isolated PostgreSQL Go 11/11, final web typecheck/lint/build and desktop/mobile production Chrome 28 records pass. Full details and all changed old assertions are in final-report.md. No commits, Git writes, accept/env edits or repository images. Optional prompt change was inspected and deferred to preserve frozen reply contracts.
Final narrow-layout verification: WorldAutomaticEvents uses a full-width description before controls on mobile; r34c-task-audit 6/6, web typecheck/lint/build, and 28 production Chrome records re-passed with a >=200px time-description geometry assertion.
