# Current stage execution

1. [x] Restore exact v3 pages/source/assets and Mock data in the active entry. Remove route replacements with old business views.
2. [x] Isolate Mock state and confirm zero real business network requests.
3. [x] Componentize shared controls, overlays, sources/status, shell and repeated domain structures; keep original layout/interactions during the port.
4. [x] Add missing about-you and NPC management/detail/edit flows in the same component system.
5. [x] Compare all routes and key flows against reference v3 at desktop and 390px; typecheck/build and appropriate UI tests.
6. [x] Update one page acceptance ledger with coverage, source comparison and screenshots. Deliver reviewable UI and wait for explicit user acceptance.

Do not resume real interface wiring before explicit page approval. Backend WIP is parked, not delivered as accepted work.

7. [ ] 等待用户明确确认页面通过，再开始真实接口填充。当前禁止自行进入下一阶段。

本阶段交付记录：docs/v3-page-acceptance.md。142 项 Web 测试通过；原型自测桌面和窄屏各 113 项通过。任务仍保持 in_progress 等待页面验收，不将后端 WIP 标为完成。
