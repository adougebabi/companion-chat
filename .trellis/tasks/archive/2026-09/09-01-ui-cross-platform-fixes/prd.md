# 修复跨端 UI、状态加载与列表兼容问题

## Goal

让 PC 与手机端在查看 Fluctlight 详情、编辑治理、设置切换和实例分组时都能稳定显示可读内容，并确保生活日程按实例时区呈现，避免用户依赖刷新页面或看到原始英文字段与 `[object Object]`。

## Background and confirmed facts

- 详情弹窗 [`apps/web/src/components/instances/InstanceDetailsDialog.vue:24-29`] 使用 `String(value)` / `Array.join()`，对象及对象数组会渲染为 `[object Object]`；标签映射缺少 `core_values`、`worldview`，未知字段会直接暴露 snake_case 英文键。
- 编辑与治理页 [`apps/web/src/views/GovernanceView.vue:17-22`] 存在相同格式化风险，且页面直接渲染目标、意图、日程、事件、关系、认知动作和修订的英文状态/类型。
- 身份与人格、生活世界、关系与记忆的只读展示当前位于治理页 [`apps/web/src/views/GovernanceView.vue:71-114`]；详情弹窗只有身份、此刻和生活摘要 [`apps/web/src/components/instances/InstanceDetailsDialog.vue:54-81`]。
- 生活日程当前只截取 `start_at` 字符串 [`apps/web/src/views/GovernanceView.vue:88-90`]，没有根据日程 `timezone` 进行格式化。
- 设置页的 `Accordion` 只使用初始化型 `default-value` [`apps/web/src/views/SettingsView.vue:78`]。切换 section 时组件实例复用，新的内容项不会自动展开；刷新后重新挂载才显示。
- Go Core 的 actor group 响应返回 `members` [`apps/core-go/internal/core/operations.go:606-643`]，而 browser-client、store、实例列表和桌面上下文均读取 `actor_ids`，实例筛选在 [`apps/web/src/views/InstancesView.vue:69-78`] 与 [`apps/web/src/views/InstancesView.vue:278`] 调用 `includes`，导致 `undefined.includes`。

## Requirements

### R1. 可读的详情与治理数据

- 详情弹窗与治理页不得渲染 `[object Object]`；对象、数组、空值和嵌套值必须使用稳定、可读的格式。
- 详情弹窗与治理页的身份字段（包括 `education`、`fashion_preference`、`physical_attributes`、`nationality`、`core_values`、`worldview` 等）使用中文标签；未知英文键不得原样泄漏到用户界面。
- 动态状态、趋势、事件类型、动作类型、修订来源/状态等常见后端枚举在详情/治理页显示中文文案，并保留必要的原始值语义。

### R2. 详情页承载只读人格与生活信息

- 详情弹窗在现有身份与“此刻”之外，展示身份与人格、生活世界、关系与记忆的只读内容，包括人格/表达策略、目标与意图、当前上下文、关系摘要和可展示记忆。
- 治理页不再重复承载上述只读摘要；仍保留暂停/恢复、日程提交/取消、事件、Presence、记忆修正/遗忘、关系回滚和修订等操作入口，避免丢失治理能力。
- 详情页在 PC 与手机端均可滚动访问新增内容，不遮挡底部操作按钮。

### R3. 生活世界日程时间轴

- 详情页的“今日日程”使用时间轴样式展示时间、活动、场景和状态。
- 每个时间显示按该日程的 `timezone` 格式化；缺失或非法时区安全回退到实例身份时区，再回退 `Asia/Shanghai`。
- 跨日期/带偏移的 ISO 时间不会被简单字符串截取；开始和结束时间均使用同一时区格式化。

### R4. 设置切换即时生效

- 从设置概览或设置导航切换到任一 section（模型角色、Endpoint、绑定、媒体、运行策略、所有者）后，目标板块立即可见，无需刷新。
- 切换行为不得破坏现有表单交互或 Accordion 手动折叠能力；若已有数据加载失败，用户至少能通过切换再次触发安全重试或看到明确错误。

### R5. Actor group 响应契约兼容

- 统一 actor group 的正式响应字段为 `actor_ids`，覆盖列表和创建响应，并与 browser-client、Web store、实例列表及桌面上下文保持一致。
- Web 端对历史/迁移期间仍可能出现的 `members` payload 提供防御性归一化，确保任何 group 的成员字段始终为字符串数组，`includes` 不再对 `undefined` 调用。
- PC 与手机端分组筛选、默认分组和加入分组操作均能在空成员、缺字段和旧 payload 下安全运行。

## Acceptance criteria

- [ ] 详情弹窗和治理页使用统一格式化/标签逻辑；用包含对象、数组、`null`、未知键的 fixture 渲染时无 `[object Object]`，且不显示原始 `core_values` / `worldview` / snake_case 标签。
- [ ] 详情弹窗可看到人格与表达策略、生活世界（目标/意图/上下文/今日日程时间轴）、关系和记忆；治理页仍可执行原有治理操作且不重复展示同一只读块。
- [ ] 日程时间轴的时间在至少两个时区 fixture（如 `Asia/Shanghai`、`America/Los_Angeles`）下与 `Intl.DateTimeFormat` 结果一致，非法时区会回退而不抛异常。
- [ ] 设置页 section 切换的源码/组件测试证明每个 section 立即展开；不需要整页刷新，Accordion 仍可手动折叠。
- [ ] Core actor group list/create 响应返回 `actor_ids`；Web 兼容层能读取 `members` 并归一化；实例列表在空/旧/缺字段 group 下不抛 `includes` 异常。
- [ ] 现有 Web 静态测试、相关 Go 测试和新增回归测试全部通过；PC 与手机端相关 CSS/布局断言保持通过。

## Constraints and out of scope

- 不改变详情 API 的 snake_case read model，不进行数据库迁移；仅修复响应字段契约和前端展示/归一化。
- 不重新设计治理操作的业务权限、审计或 API 语义；本次只调整展示位置、文案和客户端状态处理。
- 不把截图中的界面文本当作额外产品需求；截图只用于确认英文标签和 `[object Object]` 的视觉问题。

## Verification status

All acceptance criteria are covered by the implementation and regression checks. Web typecheck/build/tests, browser-client typecheck/tests, and Core/BFF Go tests pass; a production-build viewport check found no horizontal overflow at desktop and 390px mobile widths.
