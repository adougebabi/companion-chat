# D12 世界事件与任务可见性验证

## 根因

1. `world_event_surface.go` 的手动模板查询明确要求 `generation_id IS NULL`，自动生成的模板不会进入该目录，这是现有正确的来源隔离。
2. 世界页此前只读取手动模板和实际生活事件。后者虽包含已应用的世界级执行，但不包含生成中、无事件、跳过、审核拒绝或生成失败的小时记录，也不包含持久世界时钟。`world-event-generations` 诊断数据已经存在，但世界页未消费。
3. 管理中心只读取按 `created_at` 倒序的 20 条 workflow-intents，并在客户端筛选已读类型。长期世界时钟创建时间较早，容易被新的角色任务挤出首屏，筛选选项也不会发现它。

## 修复与文件

- `apps/core-go/internal/core/diagnostic_records.go`：新增精确 `worldId` / `intentType` 筛选，保留 Owner 授权、快照和过滤条件绑定的游标；世界时钟投影增加 world_id/world_name。
- `apps/web/src/react-app/shared/WorldAutomaticEvents.jsx`（新增）：按当前浏览世界读取自动生成记录和世界时钟，分别展示生成审核状态与执行结果，提供任务/记录入口、空态、刷新、分页和原 GET 重试。
- `apps/web/src/react-app/views/WorldPage.jsx`：在“世界事件”页签挂载自动事件面板，包括尚未绑定的所浏览世界；查看不切换或执行世界。
- `apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx`：独立读取世界时钟，与最近任务按 ref 合并去重；类型选项恒有世界时钟，类型筛选在服务端分页前执行；世界名称可见、可搜索；两套分页与失败重试独立。
- `apps/web/src/approved-ui/task-presentation.mjs`：补齐 started/generating/admitted 的中文状态。
- `packages/browser-client/scripts/diagnostic-record-contract.mjs`、生成的 `packages/browser-client/openapi.json`：同步两项精确筛选参数。BrowserClient 既有参数序列化直接复用。
- 新增测试：`apps/core-go/internal/core/world_event_visibility_test.go`、`apps/core-go/internal/httpapi/browser/world_event_diagnostics_test.go`、`apps/web/test/d12-world-tasks.test.mjs`、`packages/browser-client/test/world-event-diagnostics.test.ts`。
- 扩展测试：`apps/web/test/demo-replica-world.test.mjs` 新增 2 项页面回归；`apps/web/test/fixtures/world-fixture.mjs` 支持诊断 GET，记录真实查询参数。
- 文档：shared-world-contract.md 和本任务 PRD/design/implement/验证记录；手工追加开发日志，不调用自动提交脚本。

复用 shadcn/ui：**Card（Surface）、Empty（EmptyState）、Badge、Button、Tabs/TabsList/TabsTrigger、Input、Select（ChoiceSelect）**。继续复用 useWorkspaceRead、WorldReader 和原诊断任务详情入口。

## 测试与结果

使用 Node `/tmp/node-v24.12.0-linux-x64/bin`。以下均通过，无跳过：

```bash
PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH node --test \
  apps/web/test/d12-world-tasks.test.mjs \
  apps/web/test/diagnostic-workspace.test.mjs \
  apps/web/test/r23-ui.test.mjs \
  apps/web/test/demo-replica-world.test.mjs
# 61/61 pass（新增 6 项 D12 回归）

PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH node \
  --import ./packages/browser-client/node_modules/tsx/dist/loader.mjs --test \
  packages/browser-client/test/world-event-diagnostics.test.ts
# 1/1 pass

GO_CORE_TEST_DATABASE_URL='postgresql://box@127.0.0.1:55439/postgres?sslmode=disable' \
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test -v \
  ./internal/core ./internal/httpapi/browser \
  -run 'TestWorldEventDiagnosticsFiltersReachOldClocksAndWorldOnlyGenerations|TestWorldDiagnosticFiltersValidateLength|TestWorldEventDiagnosticFiltersPassThroughBrowserBoundary|TestWorldHourLockDeduplicatesConcurrentCallsRestartAndMultipleMembers|TestAbandonedWorldGenerationTimesOutAndClockTaskShowsTerminalCycle' -count=1
# 4 Core + 1 browser 顶层测试通过，browser 下含 2 个数据集子测试；无 SKIP

PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH npm --prefix apps/web run typecheck
PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH npm --prefix apps/web run lint
git diff --check
# 全部通过；仓库 lint 和 typecheck 均执行 tsc --noEmit
```

本机 pnpm 的 Corepack 尝试写只读 `/usr/local/share/corepack`，因此改用 npm 执行相同 package scripts，没有安装或修改依赖。

覆盖：旧时钟被 21 条新任务覆盖后的精确读取；世界级生成分页与跨世界游标拒绝；浏览未绑定世界只读；没有模板/个人事件仍显示自动记录；审核与已应用结果分开；无事件结果；两来源去重；分页失败后复用原游标；世界/Owner 切换后忽略过期读取；任务与记录链接；BrowserClient→Go browser→Core 筛选透传；既有 D7 时钟等待和小时去重。

## 旧断言变更

**无。没有删除、替换或放宽任何既有断言。** 既有测试文件只新增测试，fixture 只新增诊断读取能力。D1–D11 既有实现保留。

## 范围与限制

不修改生成模型、Kev、事件执行逻辑、提示词或数据库 schema，无迁移需求。只运行相关测试和 web typecheck/lint，未运行全仓测试或构建，未声称线上 Temporal/真实模型业务验收。参考图片只读，没有新增或修改图片文件；没有访问 `/workspace/accept`、修改 *.env、Git staging 或提交。
