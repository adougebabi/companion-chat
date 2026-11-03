# Technical design

## Boundaries

1. `apps/web/src/lib/fluctlight-display.ts` 提供共享的字段标签、枚举文案、值格式化和时区时间格式化函数，详情与治理共同使用。
2. `InstanceDetailsDialog.vue` 扩展为只读详情 read model 的展示入口；新增人格/表达、生活世界时间轴、关系与记忆区块，继续复用已有详情 API 与 store。
3. `GovernanceView.vue` 聚焦可变更操作。保留治理表单和必要的操作历史，移除与详情重复的只读区块，并把剩余动态值改为共享格式化函数。
4. `SettingsView.vue` 通过 section 变化使 Accordion 内容重新初始化（或受控同步），同时不重置用户正在编辑的表单字段。
5. `control-center.ts` 在读取 actor groups 时做兼容归一化；`apps/core-go/internal/core/operations.go` 将正式响应键改为 `actor_ids`。

## Data flow and contracts

- Core/BFF actor group 正式响应：`{ id, name, owner_actor_id, actor_ids: string[], created_at }`。
- 前端归一化接受 `actor_ids` 或历史 `members`，并保证空数组默认值；后续计算属性只接触归一化结果。
- Fluctlight detail 继续读取现有 snake_case 字段：`identity`、`personality`、`behavioral_policy`、`inner_state`、`goals`、`intentions`、`relationships`、`memories`、`schedule`、`context` 等。
- `formatDisplayValue` 对 `null`/`undefined`、原始值、数组和对象分别处理；对象输出紧凑 JSON 或递归的键值文本，绝不调用隐式 `String(object)`。
- `formatZonedTime(value, timezone, fallback)` 使用 `Intl.DateTimeFormat`，先验证时区；无效时区回退到实例时区和 `Asia/Shanghai`，异常时返回可读的未设定文案。

## Compatibility and rollout

- 先修复 Core 响应，再保留 Web 归一化以覆盖旧服务/缓存 payload，避免移动端在滚动发布期间崩溃。
- 详情扩展只依赖已经由 `FluctlightDetail` 返回的数据，不增加新接口；无数据时显示空态。
- 设置修复采用局部组件 remount/key 或受控 `modelValue`，不触发整个 App 路由重载。

## Risks and rollback

- 详情内容增多可能压缩移动端可视区域；保持 body 独立滚动、footer 固定，并为时间轴/长文本加换行。
- 枚举映射覆盖不足时使用中文通用 fallback，不显示原始英文键；新增映射可后续增量补充。
- 若 Core 客户端还有依赖 `members` 的调用，先通过搜索确认并在测试中锁定 `actor_ids`；回滚时可恢复键名并保留前端双字段兼容。
