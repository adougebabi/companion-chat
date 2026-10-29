# Studio world application runtime linkage — controlled evidence

2026-10-29. Scope: world definition application, source receipt and actual target directory/detail. No production defect has been reproduced: the existing result says the original receipt was checked and asks the Owner to inspect the target. Its lack of automatic target GET is not a false completion claim.

The previous fixture returned an application ID and a source reference without writing runtime state. That fixture could certify receipt presentation only. The opt-in linkage now calls `worldReplay.studioWorld` and uses its existing per-fid catalogue, places and active binding. The product bundle does not import these files. Other resource kinds retain their existing fixture behavior; Foundation remains a proposal, not an effective identity.

Core source contracts consulted: `world_resources.go` DispatchWorldApply/worldApplyCurrent/worldApplyDiff, `world_catalog.go` createCatalogWorldTx, and `world_framework.go` materializeWorldFrameworkTx. First world application has create_required=true, diff=[], runtime revision zero and creates an **inactive directory entry**. It does not activate a world or change Life context. Updates locate the original source resource in the original fid, require runtime CAS and explicit keep/override for each actual difference. Framework materialization appends missing local category/place keys without replacing existing place identity. Source references use the runtime world ID, not an invented application ID.

The fixture is intentionally bounded to these paths, not a Core implementation or persistence substitute. It does not model DB transactions, full validation, authorization or other resource kinds. `instance_revision=1` is a controlled baseline. Core's worldApplyCurrent does not project framework, so subsequent previews with a framework remain `update` even when scalar fields match. The browser check permits update/unchanged and verifies the same runtime ID.

Executed checks:

- `node --test apps/web/test/studio-runtime-fixture.test.mjs apps/web/test/demo-replica-studio.test.mjs`: 19 passed before the added delayed-transport case.
- `node --test apps/web/test/studio-runtime-fixture.test.mjs`: 3 passed including delayed transport + duplicate in-flight original key; rejects do not mutate; unknown acknowledgement commits once and replays before stale CAS.
- `node --test apps/web/test/demo-replica-world.test.mjs apps/web/test/v3-studio-foundation.test.mjs`: 31 passed, including all current expansion scenarios and Foundation non-effective semantics.
- Node syntax checks passed for the three fixture files and new browser verification script.

The real Studio editor renderer test uses the production controller/provider and real StudioApply component. It selects a role, reads exact publication, confirms application, checks runtime state/source version, preserves active binding, follows the result action and asserts `/c/A/worlds` with one apply mutation. Transport tests additionally cover immutable old publication after unpublished edits, explicit keep/override, source runtime ref, place identity and other-role isolation.

## Coordinated browser run (not executed by implementer)

Wait for the main thread's coordinated production build, then start a fresh task-owned server on a verified unused port. Never kill/reuse an unrelated listener. Do not use the real stack18480. Default script expects one fresh process for desktop then mobile: desktop creates; mobile updates the same source runtime.

```sh
FLUCTLIGHT_UI_FIXTURE_PORT=4453 FLUCTLIGHT_UI_FIXTURE_WORLD=normal FLUCTLIGHT_UI_FIXTURE_STUDIO=runtime FLUCTLIGHT_UI_FIXTURE_STUDIO_RUNTIME=1 node apps/web/test/fixtures/ui-quality-server.mjs
```

In another process:

```sh
STUDIO_RUNTIME_BASE=http://127.0.0.1:4453 node .trellis/tasks/10-28-demo-fullsite-audit/verification/verify-studio-runtime-ui.mjs
```

Chrome headless is loaded from `/private/tmp/fluctlight-demo-audit-20261028/browser-tools`. Viewports 1348×925 and 390×844, DPR1, zh-CN, Asia/Shanghai, reduced motion. The script follows the real result button into the same-fid world directory, opens the actual new card, reads its background and nested source disclosure, reloads and reopens it, reads catalogue/detail APIs, verifies active binding/other-role unchanged and re-previews the same runtime. Screenshots/report go to `evidence/studio-runtime-application/`. It does not modify the reference Demo or compare this formal-only state to fabricated Demo data.

Optional isolated server scenarios: STUDIO=runtime-rejected (422 before runtime mutation), runtime-unknown (commit once, first acknowledgement lost), runtime-pending (delayed transport; no invented pending-success status). The standalone browser script targets the normal runtime scenario; the failure/unknown/pending paths are currently unit/transport evidence only.

Pending: main thread production build and actual browser execution, screenshot review and any selector fixes from that run. No visual or live DB acceptance is claimed by these unit checks. No backend/schema/model/prompt changes, service restart, live mutation, commit, archive or fullsite completion.

## 2026-10-29 主线程验收更新

主线程实际验收：evidence/studio-runtime-application/report.json **2/2通过**（桌面新建、手机更新，每组17项断言），含结果入口→同fid目标世界目录→详情/来源→刷新→重开→原runtime再预览；当前绑定与另一个角色未改变。已人工复核桌面应用预览与手机目标设定来源。没有生产Studio缺陷，不把fixture联动补全写成产品修复。runtime-rejected/unknown/pending仍为组件/transport证据，未宣称其实际浏览器通过。

生产build104与全量pass106（694/694、0skip）通过；这些受控HTTP结果不证明真实Core/PG/Worker/模型闭环。任务继续in_progress。
