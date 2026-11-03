---
{"id":"legacy-204c6cd38456a0d0490dcbaa","title":"entry-map.md","kind":"research","status":"historical","paths":[],"source":".trellis/research/archive/2026-10/10-27-demo-workspace-replica/research/entry-map.md#sha256=ff491e7b7b283b210faa158eab3667f3bc68ab4de5a1a809e77e6556f127d3cf","superseded_by":"","tasks":null,"critical":false,"path":"","hash":"","body":"","reason":""}
---
# 已核实入口
- App.tsx:72 / :129：createWorkspace / canonical route 与 workspace.load。
- LibraryPage.jsx:22 / state/workspace.mjs:985：现有目录、搜索、创建恢复；library 读取补充 detail。
- ChatPage.tsx:52 / :145：conversation、scope 隔离和 store.send。conversation.ts:1377 保留原 turn/idempotency。
- 测试服务器 node apps/web/test/fixtures/ui-quality-server.mjs 在 4175 提供真实 dist + 同源隔离 API；无角色库空白/延迟/失败开关，按本任务补充。
- 浏览器工具优先 CUA；bundled Node modules 有 playwright（仅用于辅助产物脚本时按工具规则）。
- 原 Demo 的 setTimeout、useDemoData、固定用户、假发布不能迁入生产。

