# 页面与旧功能迁移对应表

阶段 1–5 的页面现在由原型 v3 的壳层渲染（`#v3-root`）。旧 URL 仍会收成 hash 路由。`GovernanceView` 文件还在，供原来的审核实现和测试阅读，但产品界面不再挂载它，待处理页也不再打开那张旧对话框。

| 新页面 | 规范路径 | 旧入口 | 旧实现 | 迁移结果 |
|---|---|---|---|---|
| 登录 | 未登录时任意页 | 同一 `AuthPanel` | `AuthPanel.vue` | 保留。会话、CSRF、设置密码仍走原 store |
| 角色库 | `/library` | `/instances` | `InstancesView.vue` | 保留创建、分组归入、治理入口。地址重定向到 `/library` |
| 对话 | `/chat`、`/c/:id/chat` | 同一聊天页 | `ChatView.vue` 内联气泡 | 拆成 `ChatPanel` 等组件。流式、取消、重试、历史分页仍走会话 store |
| 聊天组件场景 | `/dev/chat-components` | 无 | 无 | 本地夹具，标「不是业务数据」，不进生活 |
| 动态 | `/feed` | `/moments` | `MomentsView.vue` | 保留列表、反应、评论、隐藏。地址重定向到 `/feed`。发布标尚未接入 |
| 生活此刻 | `/c/:id/life/now` | 详情对话框里的只读片段 | `InstanceDetailsDialog` 只读区 | 独立页读 detail。缺字段标尚未接入 |
| 日程 | `/c/:id/life/schedule` | 治理里的日程表单 | control-center 日程命令 | 独立页复用 `acceptSchedule`、`cancelSchedule`、`createLifeEvent` |
| 目标 | `/c/:id/life/goals` | 治理目标面板 | `GoalPanel`、`GoalPlanningPanel` | 正式页嵌入原面板。组件场景不挂会打接口的面板 |
| 衣柜 | `/c/:id/life/wardrobe` | 详情对话框衣柜分页 | 衣柜 client | 独立页。拥有、穿着、使用、来源分列 |
| 日记 | `/c/:id/life/diary` | 无完整页 | 无 | 仍是 `UnwiredView` |
| 世界 | `/c/:id/world` | 阶段 3 的未接入占位 | `UnwiredView` | 换成 `WorldView`。事件只读，地图等仍未接入 |
| 人物关系 | `/c/:id/people` | 治理关系编辑 | `editRelationship`、`rollbackRelationship` | 独立页复用原命令。所有者背景不是 NPC |
| 记忆成长 | `/c/:id/growth` | 治理记忆与自我认知 | revise / forget / rollback | 独立页复用原命令。反思详情未接入 |
| 媒体 | `/c/:id/media` | 详情视觉身份只读 | `visual_identity` 与诊断提示词 | 独立页。生成图标「生成图·非事实」 |
| 角色资料 | `/c/:id/profile` | 治理基础修订 | foundation revision 命令 | 独立页复用提交、接受、回滚。可跳到自治 |
| 生活组件场景 | `/dev/life-components` | 无 | 无 | 本地夹具。不把样本写进接口 |
| 创作中心 | `/studio`、`/studio/apply` | 阶段 3 未接入占位 | `UnwiredView` | 换成 `StudioView`。设定助手复用 `ChatPanel`，会话只在本页 |
| 自治权限 | `/c/:id/settings/autonomy` | 设置「运行策略」 | `OperationalSettings.vue` | 角色页嵌入全局策略，并列出自治动作治理。角色级字段未接入 |
| 系统设置 | `/settings` | 同一设置页 | `SettingsView.vue` | 保留模型、Endpoint、媒体、Kev、所有者 |
| 运行诊断 | `/diagnostics` | `/settings/diagnostics` | `DiagnosticsView.vue` | 保留筛选、分页、导出、清空确认。工作流取消、重启、重置增加确认。地址重定向 |
| 待处理 | `/pending` | 铃铛原先进通知占位 | 无聚合页 | 新页。能力审核跳 `GovernanceView`，不在本页调用审核 |
| 通知 | `/inbox` | 无列表 | `UnwiredView` | 仍未接入。铃铛改去待处理，不显示假未读 |
| 数据工具 | `/data` | 无 | 无 | 三栏说明。导入和备份都未接入 |
| 时间线回档 | `/data/rollback` | 无 | 无 | 说明页，标「未提供」。不进主导航 |
| 治理 | 角色库内打开 | 同一对话框 | `GovernanceView.vue` | 保留。能力审核、退休仍在这里 |

旧地址：

| 旧路径 | 规范路径 | 方式 |
|---|---|---|
| `/instances` | `/library` | `replaceState`，视图仍是角色库 |
| `/moments` | `/feed` | `replaceState`，视图仍是动态 |
| `/settings/diagnostics` | `/diagnostics` | `replaceState`，查询串保留 |

被换掉的只有创作中心和世界的 `UnwiredView` 占位。日记、通知的占位还在，因为没有可复用的接口。`GovernanceView`、`DiagnosticsView`、`MomentsView`、`SettingsView`、`InstancesView`、`ChatView` 都还在调用原来的 store 方法。
