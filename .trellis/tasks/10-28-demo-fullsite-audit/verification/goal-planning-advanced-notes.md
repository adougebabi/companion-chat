# Goal planning advanced state batch

Scope: retained production `renderRealGoalPlanning` reached through DomainPage/DomainPrimitives. No backend, prompt, global CSS, Demo or fullsite metadata changes.

## Reproduced defects and correction

- Manual order included candidate/paused IDs. Core `goal_set.go` requires `order` to cover active goals only (around lines 554–589). New regression run against HEAD renderer fails with candidate-one/candidate-two present. Submit now filters to actual active IDs; dependency editor still retains every current goal.
- Acknowledged policy write followed by failed GET reopened normal saving and had no durable read-only recovery. New regression against HEAD fails; original body/key and accepted checkpoint now persist with the existing Owner/fid session draft, and recovery after acknowledgement executes GET only.
- An unknown mutation response had no original identity recovery. HEAD fails the recovery regression. Retry now uses the same frozen body, baseline, reason and key, separately from newer draft values.
- Candidate/source decisions previously advanced the separate policy draft baseline silently. They now leave it for explicit reviewed adoption. Candidate reason remains available after rejection/conflict; cancelling an unsubmitted decision clears only that draft.
- `立即规划` now preserves its original key and acknowledged state; failed history reads cannot submit another planning request. A matching receipt key is required. UI retains accepted-versus-completed distinction.
- Source JSON syntax errors now have a specific local error, matching the existing policy JSON behavior.

Production changes: `apps/web/src/v3-domain/goal-planning.js`; exact provider helpers at `apps/web/src/v3/providers/life.ts:100–101` accept an optional explicit identity and preserve existing callers.

## Executed checks

- `node --test apps/web/test/goal-planning-recovery.test.mjs`: 8 passed, 0 failed. Tests cover advanced policy/dependencies/active order, reload, acknowledged GET-only recovery after remount, exact unknown replay, CAS draft retention, candidate/source fixture admission, actual provider identity mapping, plan receipt/history recovery, and newer edits during awaited readback.
- Ran the first five regression cases with only the renderer temporarily replaced by HEAD source, then restored the implementation in a finally block. 3 expected failures / 2 passes; raw output: `goal-planning-baseline-test.txt`.
- `pnpm --filter @fluctlight/web typecheck`: passed.
- `node --check` on the production-browser script and fixture server: passed.

These DOM tests exercise the real retained renderer and actual provider in the dedicated provider test. They do not claim browser/Radix geometry, PostgreSQL, Worker, or model execution.

## Browser handoff (not executed in this batch)

Main session must finish concurrent implementation, run the coordinated production build, then execute:

```sh
GOAL_PLANNING_PORT=4451 AUDIT_PHASE=advanced node .trellis/tasks/10-28-demo-fullsite-audit/verification/verify-goal-planning-ui.mjs
```

The script owns a fresh loopback fixture process for each scenario/viewport and kills only that child. Port 4451 must be unused. It uses installed Chrome and playwright-core from `/private/tmp/fluctlight-demo-audit-20261028/browser-tools`, 1348×925 and 390×844, DPR1, zh-CN, Asia/Shanghai, a fresh browser context for every run. It covers normal advanced/manual/dependency save/reload; candidate cancel/accept/reject; source review; plan request; reviewed conflict rebase; accepted readback failure; and lost-response replay. It captures requests, screenshots and document overflow/errors. Outputs go to `evidence/goal-planning-advanced/` (or the supplied AUDIT_PHASE suffix).

Manual server alternative:

```sh
FLUCTLIGHT_UI_FIXTURE_PORT=4451 FLUCTLIGHT_UI_FIXTURE_GOAL_PLANNING=normal node apps/web/test/fixtures/ui-quality-server.mjs
```

Scenario values: `normal`, `conflict`, `readback-failure`, `lost-response`. Restart for a fresh server state; use a fresh context or clear only this fixture origin's session draft when starting a new independent scenario. Reuse the same context and server for recovery tests.

## Boundaries

New fixture is opt-in, test-only, and handles only actual goal-set/goal-planning endpoints before fallback. It uses BrowserGoalSetCommand camel-case fields, Core CAS/facts checks, saved receipts before CAS on replay, direct write commit snapshots, active-only order, candidate resume/cancel, review revisions and full nested state preservation. The plan POST response mirrors Core `{status:'requested',idempotency_key}`; pending history does not simulate completion. It implements the planner operations used by this batch, not the whole Core goal authority engine (e.g. every dependency validity/category/profile rule). No fixture result certifies real persistence or model acceptance. Production build guards and actual browser runs remain main-session checks. Demo has no direct equivalent for these retained formal advanced controls, so this batch claims no same-content Demo visual match.

## 2026-10-29 主线程验收更新

主线程实际验收：定向回归扩展至**12/12通过**；最新evidence/goal-planning-advanced-pass103/report.json **8/8通过**。初次浏览器发现原生checkbox双change造成NotFoundError，DomainPrimitives修复后异常消失；原冲突基线须显式采用。人工复核高级编辑、GET恢复与桌面普通态截图。advanced/pass101/redraw102失败报告保留，不作最新通过依据。

生产build104与全量pass106（694/694、0skip）通过；这些受控HTTP结果不证明真实Core/PG/Worker/模型闭环。任务继续in_progress。
