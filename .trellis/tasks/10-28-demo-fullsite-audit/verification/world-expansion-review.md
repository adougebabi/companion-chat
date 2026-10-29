# World failed-run retry and effective definition batch

This bounded batch covers `resources.world.events`, `resources.world.place.history` and the previously unverified note under `resources.world.venues`. No shared coverage/progress files were changed. Browser verification remains pending the main session's coordinated production build.

## Reproduced production defect and fix

`apps/core-go/internal/core/world_event_surface.go:124` returns a retry command receipt with only `id`, `status=queued`, and `revision=expectedRunRevision+1`. The generated `BrowserClient.retryWorldEvent` also declares `BrowserWorldEventCommandResult`, not the full run projection. `WorldRuntimePanels.jsx` applied its full run scope check to that receipt and rejected the successful command as “记录不属于当前角色与世界”. The older world fixture returned the full run and hid this mismatch.

Changed only the retry receipt branch: validate original run ID, queued status and exact next revision. The existing `onSaved` still GETs the full original run, validates role/world/run/template and minimum revision, and then refreshes the actual run list. Trigger receipts retain their full projection checks. No API, backend, request body/key/CAS, retry permission, or prompt changes.

Red evidence: `evidence/world-expansion-states/red-minimal-retry-receipt.log`. After making the fixture match Core, the unknown and readback recovery tests reproduced the failure before the frontend fix. Green evidence: `evidence/world-expansion-states/focused-tests.log` (29/29 passed).

## Fixture fidelity and boundaries

`worldFixture` accepts opt-in `expansion-normal`, `expansion-unknown` and `expansion-readback-failure` scenarios. Each has one original failed run (template v6, run v4), never a replacement trigger. Successful retry returns the exact minimal receipt and clears its old error code. Unknown saves commit once and lose the first response, with original body/key replay. Readback failure rejects the first original run GET or world catalogue read; recovery must send GETs only.

The shop has an immutable published `place-resource` / `v1` definition deliberately different from the current saved fields and recorded initial baseline. `current_definition` reads that exact definition, capture freezes it with `source_kind=published_definition`, and capture does not mutate the current place. Only explicit recorded-baseline restoration changes current fields. Nested definition extensions survive. The room has no publication and still rejects the definition preview rather than inventing a baseline. `expansion-effective` removes the shop’s recorded baseline: the existing verified publication is the effective source at revision 0 until explicit restore records revision 1. Existing non-opt-in fixture scenarios remain available.

These are controlled HTTP/React component tests. Queued-to-applied observation in the browser fixture is simulated; this batch does not establish real Core/PostgreSQL/worker/model execution or real publication persistence. The separate AI world expansion Studio pipeline was not altered or certified.

The read-only Demo `src/pages/ResourcePages.tsx:1452` uses a simulated event trigger and has no formal failed-run recovery or effective-definition baseline flow. These retained production capabilities require interaction/protocol evidence and desktop/mobile geometry checks, not invented matching Demo success screenshots. No Demo/reference files were changed.

## Checks executed

- `node --test apps/web/test/demo-replica-world.test.mjs` — 29 passed, 0 failed. Includes original 15 regressions plus source distinction/capture/restore, missing definition, unknown/readback recovery for both commands, invalid retry receipt ID/status/revision, foreign role readback, and disabled recovery for expired/invalid windows, changed template revision and paused template.
- `node --check apps/web/test/fixtures/world-fixture.mjs` — passed.
- `node --check .trellis/tasks/10-28-demo-fullsite-audit/verification/world-expansion-states.mjs` — passed.
- No full Web build, browser execution, real stack request or Core test was run by this implementer. Main session owns combined typecheck/build and browser verification.

## Browser handoff

After the coordinated current-source production build, from repository root:

```sh
WORLD_EXPANSION_PORT=4451 node .trellis/tasks/10-28-demo-fullsite-audit/verification/world-expansion-states.mjs
```

The script uses Playwright from `/private/tmp/fluctlight-demo-audit-20261028/browser-tools`, Chrome headless, DPR1, zh-CN, Asia/Shanghai, desktop 1348×925 and mobile 390×844. It starts a fresh loopback-only `ui-quality-server.mjs` child per scenario with `FLUCTLIGHT_UI_FIXTURE_WORLD=expansion-{normal,unknown,readback-failure}`, unsets the parity overlay for those worlds and refuses an occupied port before launch. It stops only the child it spawned. It does not use/restart 4381, 4396 or 18480. Optional override: `WORLD_EXPANSION_PORT=<another-free-port>`.

Fourteen browser cases cover failed run and definition baseline × three scenarios × two sizes, plus verified published-definition fallback when a legacy place has no recorded baseline. The script records requests, body/key equality, GET-only recovery counts, current/captured distinction, explicit restore and refresh, uncaught exceptions, viewport/dialog bounds and state screenshots under `evidence/world-expansion-states/`; output `report.json` is written only when actually executed. Browser assertions and visual review have not yet been executed and must not be marked passed from syntax/component results.

## 2026-10-29 主线程验收更新

主线程实际验收：生产build104；evidence/world-expansion-states/report.json **14/14通过**，含failed-run最小回执、未知原命令、GET-only恢复、current_definition捕获/显式恢复和有效v0基准，两个视口均无浏览器异常。已人工复核关键恢复与有效定义截图。上面的handoff记录保留为执行方法，不再表示浏览器待执行。

生产build104与全量pass106（694/694、0skip）通过；这些受控HTTP结果不证明真实Core/PG/Worker/模型闭环。任务继续in_progress。
