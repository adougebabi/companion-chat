# r22 P1 修复与验证

仅处理 `/workspace/ux/摇光-UIUX回归-r22.md` 的 P1；P2、`/workspace/accept/` 未修改。参考源为 `/workspace/ux/demo/fluctlight-demo.html`。使用现有 React + shadcn/Radix；没有提示词、模型请求或 token 限制改动。

## 按问题映射

下表文件均相对仓库根；测试完整名称见下一节。

| P1 | 文件 | 回归测试 |
| --- | --- | --- |
| A3 补充设定字段名无法区分 | `apps/web/src/react-app/shared/CreationPreviewValue.jsx` | reading: A3 preview names style, taste… |
| A3 人格大片重复空项、默认关系被当作输入 | `apps/web/src/react-app/views/CharacterWizard.jsx`, `CreationPreviewValue.jsx` | reading: A3 single persona…；description/template 无来源默认值两项 |
| A3 default/friend/weekly 原始英文枚举 | `CreationPreviewValue.jsx` | reading: A3 preview names…；A3 single persona… |
| A4 执行记录缺汇总、每轮状态与耗时、输入/输出/错误；原始 JSON | `apps/web/src/react-app/shared/TaskExecutionResults.tsx`, `apps/web/src/react-app/pages/TaskDetailPage.tsx`, `apps/web/src/approved-ui/task-presentation.mjs`, `apps/web/src/styles/replica-diagnostics.css` | task: execution results…；failed pre-model…；pagination… |
| A4 向导和管理中心详情应为右侧 Sheet | `apps/web/src/react-app/shared/TaskRecordDialog.tsx`, `CharacterWizard.jsx` | task: wizard opens task…；management task detail… |
| A4 缺少跳过入口 | `apps/web/src/react-app/shared/PreparationSteps.jsx`, `apps/web/src/state/creation-controller.mjs`, `CharacterWizard.jsx` | task: skip explicitly defers waiting… |
| B 关系称呼和主文案 unknown | `apps/web/src/approved-ui/people-presentation.mjs`, `apps/web/src/react-app/shared/AboutYouPanel.jsx` | reading: B unknown relationship titles… |
| C1 默认策略与阈值 | `apps/core-go/internal/ai/decision/decision.go`, `decision_test.go` | `TestR22KevDefaultsUseConfidenceGateAndDisableMedia`；settings: advanced settings… |
| C1 媒体决策默认开启 | 同上；`apps/web/src/v3/proto/components/configuration-fields.js`；媒体测试夹具 `apps/core-go/internal/core/media_capture_plan_test.go` 显式选择启用 | 同上；`TestMediaBranchKevTwoStagesAndMixedPointAudit` 及 Kev/MediaBranch 回归 |
| C3 缺少本地备份区块 | `apps/web/src/react-app/shared/LocalBackupsPanel.tsx`, `apps/web/src/react-app/pages/DomainPage.tsx`, `apps/web/src/v3-domain/data-tools.js` | settings/backups: create persistent…；production data route…；错误/Owner/删除确认等共 7 项备份测试 |
| D 目标/人物关系/角色库空态白框无叶子 | `apps/web/src/react-app/pages/GoalsPage.jsx`, `LibraryPage.jsx`, `apps/web/src/react-app/views/PeoplePage.jsx` | reading: D goals…；D character library…；D relationships… |
| E 世界完整记录英文键与内部 ID | `apps/web/src/react-app/shared/WorldRecord.jsx`, `apps/web/src/react-app/views/WorldPage.jsx` | reading: E current world record… |

既有测试同步：`demo-replica-creation-ui.test.mjs`、`diagnostic-task-detail.test.mjs`、`r20-creation.test.mjs` 中仅更新被本次要求覆盖的旧断言。工作规范同步至 frontend/workspace-replica-contract.md 和 backend/kev-decision-contract.md。

## 完整新增测试名称

### apps/web/test/r22-reading-ui.test.mjs

- 'r22 A3 preview names style, taste, commitments, relationship seeds and media fields in Chinese without changing data'
- 'r22 A3 single persona omits empty default blocks and inferred relationship rows while retaining explicit source and edits'
- `r22 A3 description preview keeps unproven relationship defaults out until explicit source adoption`
- `r22 A3 template preview keeps unproven relationship defaults out until explicit source adoption`
- 'r22 B unknown relationship titles are Chinese in main copy, basis Sheet and revision Sheet with originals downloadable'
- 'r22 D goals confirmed empty uses borderless Leaf and read failure retains recovery instead of empty'
- 'r22 D character library true empty uses borderless Leaf and keeps working creation/configuration links'
- 'r22 D relationships confirmed empty uses borderless Leaf'
- 'r22 E current world record displays Chinese labels, hides internal IDs and downloads the complete unchanged record'

### apps/web/test/r22-task-ui.test.mjs

- 'r22 execution results count physical calls, disclose each input/output/error and keep raw lifecycle closed'
- 'r22 failed pre-model task reports zero physical rounds without turning nine lifecycle events into attempts'
- 'r22 task round pagination retries original correlation and discards responses after task changes'
- 'r22 wizard opens task as a right Sheet and restores its original trigger and hash'
- 'r22 management task detail uses the same right Sheet and returns to its task row'
- 'r22 skip explicitly defers waiting, persists progress and preserves failed task and original recovery request'

### apps/web/test/r22-settings-backups.test.mjs

- 'r22 local backups create persistent Owner-scoped copies and fill an import draft without business writes'
- 'r22 backup failures preserve existing copies and unread storage cannot be overwritten'
- 'r22 deleting a local backup requires confirmation and cancellation preserves the copy'
- 'r22 backup result from a previous Owner is discarded after switching Owner'
- 'r22 returning to an Owner cannot admit an export from its previous lifecycle'
- 'r22 blocked browser storage reports an error without crashing the backup panel'
- 'r22 production data route mounts shadcn local backups and restores into the existing validation draft'
- 'r22 advanced settings render authoritative guarded defaults and retain saved zero and media choices'

## 检查结果

- `pnpm install --store-dir /tmp/r22-pnpm-store`：通过，使用项目指定 pnpm 11.0.7 与 Node 24.12.0；系统 Node 20/Corepack 默认缓存路径不可写，工具安装在 /tmp。锁文件无改动。
- `pnpm build`：通过，包含生产夹具排除和 React 图依赖守卫。保留现有大 chunk 提示。
- `pnpm --filter @fluctlight/web lint`：通过。
- `pnpm typecheck`：Web 与两个客户端包全部通过。
- `pnpm test`：Web **821/821**、browser-client **40/40**、core-client **3/3**，共 **864/864**。
- `go test -race ./internal/ai/decision`、该包 `go vet`/`go build`：全部通过。
- `go test ./internal/core -run 'Kev|ConfigurationCatalog|MediaBranch'`：通过。受影响的媒体集成测试已显式选择启用媒体决策。
- 完整 `go test ./internal/core`：未全绿。五项已有失败已用原始 HEAD 独立验证；另一个瞬态重试测试在高并发验证时超时，单独复跑通过。详见下方基线记录，未扩大 P1 范围修改这些行为。
- `pnpm check:commit-images`（最终可写 Git 副本的实际暂存区）和原 pre-commit 钩子、图片钩子测试 **4/4**、`git diff --check`：通过。
- Chrome 生产构建验证：1440×1000、390×844；**16 张产品截图 + 2 张原始 demo 参考截图**，检查右侧 Sheet 几何、原始数据默认折叠、无页面横向溢出、默认字段、空态与真实导出草稿交接。浏览器无未捕获异常。结果在 `verification/browser-results.json`；脚本在 `verification/verify-r22-p1.mjs`。这只验证隔离 HTTP 夹具，不证明真实模型/数据库运行。

## 已有 Core 基线失败

以下为本次改动前 HEAD 可复现的问题，未跨出用户要求的 P1 范围修复：

- `TestActorUserInitializationJSONPreservesOnlyQualifiedHumanBackground`
- `TestInitializationOwnerBackgroundKeepsStrictScalarTypes`
- `TestNoArgCatalogSchemaValidationAndCatalogParity`
- `TestPhase8FormalAgentRegistryCoversCompleteTaskInventory`
- `TestPhase8CIUsesTheRealGateAndVectorEnabledDatabase`

`TestReplyTransientRetryIsBoundedButOOMAndTruncationDoNotRetry` 单独复跑通过。
本地原始日志：`/tmp/r22-core-all.log`、`/tmp/r22-core-baseline.log`、`/tmp/r22-core-baseline-actor.log`、`/tmp/r22-core-transient.log`。

## 偏差与环境限制

- P1 代码修复均已实现并有行为测试。任意未知扩展字段无法可靠推断语义时使用可区分的中文编号；已有中文字段名保持原样，报告中已知字段均有中文名。完整原文保留在编辑器/下载中。
- 后端没有跳过命令。「跳过此环节」明确跳过本次等待，保存偏好并前往聊天/角色库；原任务真实失败/阻塞状态及原请求身份保留，可返回恢复。
- 没有实际模型调用时显示 0 轮，不把 9 条生命周期事件伪装成 9 个执行轮次；原始生命周期默认折叠。
- 本地备份保存真实可导出的作品设定、依赖、分组名及通知偏好，导入先进入现有校验草稿；不同于 demo 的整个演示工作区快照。聊天/媒体完整备份继续通过真实业务备份维护任务。
- 原仓库 `.git` 为只读挂载，`pnpm hooks:install` 无法写 `.git/config`，无法在该检出直接更新当前分支。采用 `/tmp/r22-local-commit/.git` 的可写本地副本，分支名保持 `codex/demo-fullsite-audit-20261028`，启用原 `.githooks/pre-commit`，生成 conventional commit 与 `/tmp/r22-p1.bundle`、`/tmp/r22-p1.patch`。原检出保留相同已验证改动；未推送。
- 请求的 `/workspace/work/r22-shots` 同样只读，截图实际保存在 `/tmp/r22-shots`。图片未进入 Git。
