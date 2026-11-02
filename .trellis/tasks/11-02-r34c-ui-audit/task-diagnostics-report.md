# 诊断、任务与通知计数修复

## 逐项根因与修复

1. **“需要处理”与列表、通知“服务异常”不一致**
   - 根因：通知从 `OwnerPending/failed_task` 读取全部尚未恢复的事项；诊断只分类最近 20 条 workflow，遗漏旧失败，且未复用 pending 对替代日记尝试、重复 Owner 请求的排除规则。两端对 paused/blocked/error/timeout 状态的定义也不同。
   - 修复：诊断读取同一个 `failed_task` 来源的全部原游标分页，把旧事项补入列表，以该来源权威集合排除过期失败；六类摘要、tab 数字、状态/世界/类型筛选与可见行消费同一集合。pending SQL 精确补齐既有 attention 分类的八种状态。单项任务详情失败时保留来源行、状态与计数，显示真实重试；并发恢复的详情不会替换同次 pending 快照的状态。来源未读取完时不显示虚假 0/空态。
   - 文件：`apps/core-go/internal/core/pending_center.go`、`apps/web/src/approved-ui/task-presentation.mjs`、`apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx`。
   - 新回归：`TestR34CPendingAttentionSharesAllActionRequiredStates` 检查八种 action 状态、真实游标分页、恢复移除和无关角色权限；JS 检查同源跨页合并、排除旧失败、单项失败/并发恢复保留数量、Owner 迟到响应隔离。

2. **人物形象准备、今日日程准备缺少 15 分钟提示**
   - 根因：没有有效 `started_at` 时优先采用不断更新的 `updated_at`，从而把状态刷新当成执行重新开始；无效非空时间也阻止回退。
   - 修复：对所有 running 类型依次查找真实开始、原创建证据等有效时间，最后才用更新时间；世界任务优先保留当前周期开始时间。诊断页每 30 秒更新纯展示时钟，页面隐藏时不执行更新。Core 任务详情把已有 `cycle_started_at` 原值透出，避免以长寿命 workflow 开始时间判断当前周期。
   - 文件：`apps/web/src/approved-ui/task-presentation.mjs`、`apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx`、`apps/core-go/internal/core/task_read.go`。
   - 新回归：JS 覆盖 avatar/schedule/persona/wardrobe/cognition/未知新类型/world 的全部 running 状态、无效开始时间与近期状态刷新，并实际渲染人物形象与日程警示；`TestR34CWorldTaskRetainsActualCycleStartForStalePresentation` 保证真实周期开始时间未丢失。

3. **世界事件时间的整点与实际开始不清楚**
   - 根因：诊断展示时钟创建时间，世界详情展示整点槽，两者缺少含义说明。
   - 修复：两处复用 `worldEventClockLabel`，保留日期上下文并展示 `20:00 整点（20:39 开始）`；时钟继续使用原 occurred_at/created_at，生成记录继续使用 slot_at + created_at，不改调度来源或数据。
   - 文件：`apps/web/src/approved-ui/task-presentation.mjs`、`apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx`、`apps/web/src/react-app/shared/WorldAutomaticEvents.jsx`。
   - 新回归：精确检查格式与原始时刻，并同时渲染诊断、世界时钟和世界生成记录验证一致显示。

4. **诊断缺少按世界筛选**
   - 修复：新增“任务世界” shadcn Select；普通任务和独立世界任务读取都传递 `worldId`，显示选项保留已读世界名称，pending 同源集合按原任务 world_id 精确筛选，摘要仍与可见行一致。
   - 文件：`apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx`。
   - 新回归：实际操作 Radix 选项，检查 server `worldId`、世界范围、attention 数字和列表一致。

## 验证命令与结果

```bash
PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH node --test apps/web/test/r34c-task-audit.test.mjs apps/web/test/d7-task-taxonomy.test.mjs apps/web/test/d12-world-tasks.test.mjs apps/web/test/diagnostic-workspace.test.mjs apps/web/test/r23-ui.test.mjs
```

最终 **39/39 通过**；其中新增 R34C JS 回归 6 项。

```bash
cd apps/core-go
GOCACHE=/tmp/fluctlight-go-cache GO_CORE_TEST_DATABASE_URL='postgresql://box@127.0.0.1:55439/postgres?sslmode=disable' go test ./internal/core -run '^TestR34C(PendingAttention|WorldTask)' -count=1
```

**2/2 通过**，用现成 `isolatedCoreTestRepository` 创建随机临时数据库、迁移并仅删除该临时数据库，没有清理共用库行。

全局 web typecheck、lint、build 与生产 Chrome 截图由主代理统一执行，避免重复构建覆盖。

## 复用 shadcn 组件

复用现有 `Button`、`Input`、`Tabs/TabsList/TabsTrigger`、`Badge`；新增世界筛选通过已有 `ChoiceSelect` 组合本地 shadcn/Radix `Select`，没有安装重复组件或手写替代选择器。

## 修改的旧断言及理由

1. `apps/web/test/diagnostic-workspace.test.mjs`：工具栏 combobox 数量 `2` 改为精确的 aria-label 数组 `[任务状态, 任务类型, 任务世界]`。原因是新增真实世界筛选；断言更严格且同时验证可访问名称。
2. `apps/web/test/r23-ui.test.mjs`：时间 `/10-31 14:15/` 改为 `/10-31 14:00 整点（14:15 开始）/`，保留原日期与实际开始时间覆盖，同时检查新增整点含义；无周期证据的 world workflow 原 `/进行中/` 改为精确 `状态待确认`。后者是旧测试未跟随 D7 已有合同：只有 workflow running 不能证明当前小时正在生成；没有修改实现恢复错误状态。

没有放宽其他已有断言；没有提交、写 Git 元数据、修改 `/workspace/accept` 或任何 `*.env`，没有添加图片。
