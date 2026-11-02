# Journal - codex (Part 1)

## 2026-10-30 — r22 P1 UI/UX regression repair

Task: `.trellis/tasks/10-30-r22-p1-uiux`. Fixed only r22 P1 reading, task, default settings, backup and empty-state findings. Reused shadcn primitives; preserved raw sources, original recovery requests, Owner scopes and saved KEV choices. No prompt/transport/token-limit changes.

Web build/lint/workspace typecheck and 821 Web + 43 client tests pass. Headless Chrome checks cover desktop/mobile with 16 product shots and two supplied Demo references in `/tmp/r22-shots`; no images in Git. Decision package race/vet/build and Core Kev/MediaBranch slice pass. Complete Core reveals five HEAD baseline failures plus one transient timeout that passes alone; unrelated behavior left unchanged. Final evidence in task final-report.md.

Environment: original `.git` and requested sibling screenshot directory are read-only. Current checkout retains changes; a hook-checked commit and bundle are prepared in an isolated writable Git directory on the same named branch. No push. Do not archive the task until the original checkout branch is updated.

## 2026-11-02 — r23 UI audit repair

Task: `.trellis/tasks/11-02-r23-ui-polish` (completed, uncommitted by explicit user instruction). Fixed all eleven requested items using existing shadcn primitives. Chat root cause was selection loading left behind by request invalidation; loading now belongs to its read epoch, with actual controller/provider/page and delayed-browser regressions. Added 13 UI tests. Preserved diary search feedback beside a selected entry and existing people-directory leaf; completed missing Studio/relationship leaves.

Install/build/web lint/workspace typecheck and all 877 JS/TS tests pass. Full Go suite passes after correcting seven obsolete assertions in six test files to current normalization/catalog/Agent/CI/React/queue contracts; no backend production or CI configuration changes. Chrome production desktop/mobile checks and three chunk+data skeleton flows pass; 28 PNGs only in `/tmp/r23-shots`. Exact per-item files, test names, check results and limits are in task final-report.md. No accept-tree/env edits, Git commit, staging or image changes. Trellis archive/journal auto-commit scripts were not used.
