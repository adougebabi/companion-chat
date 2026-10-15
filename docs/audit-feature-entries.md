# 53 个审计 Feature 入口核验

对照总稿附录 E 的 53 个 ID。入口是当前前端路径。状态与 `docs/access-status.md` 一致。

| ID | 内容 | 入口 | 状态 | 核验 |
|---|---|---|---|---|
| SYS-01 | Owner 认证与会话 | 未登录壳层；`/settings?section=owner` | 已接入 | 登录、退出、改密仍在原组件 |
| INS-01 | 实例列表与分组 | `/library` | 部分接入 | 列表和归入分组已有。移出分组没有按钮 |
| INS-02 | 创建与初始化 | `/library` 创建对话框 | 已接入 | JSON 分析是创建，不是读档。数据工具没有恢复 |
| INS-03 | 实例退休 | 角色库打开 `GovernanceView` | 部分接入 | 退休确认在治理页。角色资料没有单独的高级抽屉 |
| CHAT-01 | 发送与流式 | `/c/:id/chat` | 已接入 | 真实 NDJSON 回合，组件场景除外 |
| CHAT-02 | 历史分页与已读 | `/c/:id/chat` | 已接入 | 更早一页和滚动保持 |
| CHAT-03 | 取消、重试、失败恢复 | `/c/:id/chat`；失败后可到 `/diagnostics` | 已接入 | 取消和重试仍走会话 store |
| CHAT-04 | 附件上传 | `/c/:id/chat` | 尚未接入 | 气泡能显示已有附件 id，没有上传 |
| MOM-01 | 动态列表与可见性 | `/feed` | 已接入 | 全局或当前角色，可含已隐藏 |
| MOM-02 | 回应与媒体 | `/feed` | 部分接入 | 反应、评论、隐藏已接。发布未接，媒体标非事实 |
| DET-01 | Persona 身份 | `/c/:id/profile`；关系在 `/c/:id/people` | 已接入 | 身份只读。NPC 作者工具未接 |
| DET-02 | 视觉身份候选 | `/c/:id/media` | 部分接入 | 时间线可读。候选编辑未接 |
| DET-03 | 内在状态只读 | `/c/:id/life/now` | 部分接入 | 有标签才显示。证据键缺失会标明 |
| DET-04 | 生活脉络 | `/c/:id/life/now`；事件在 `/c/:id/world` | 部分接入 | 事件只读。地点详情未接 |
| DET-05 | 衣柜分页 | `/c/:id/life/wardrobe` | 已接入 | 继续用衣柜 cursor |
| DET-06 | Goal 与 Intention 摘要 | `/c/:id/life/goals` | 已接入 | 原目标面板 |
| DET-07 | 关系与 Memory 摘要 | `/c/:id/people`；`/c/:id/growth` | 已接入 | 原修订命令 |
| GOV-01 | Goal 生命周期 | `/c/:id/life/goals` | 已接入 | pause、resume、cancel、abandon 仍要原因和版本 |
| GOV-02 | GoalSet 规划 | `/c/:id/life/goals` | 已接入 | `GoalPlanningPanel` |
| GOV-03 | 自治 action | `/c/:id/settings/autonomy` | 部分接入 | `governAutonomyAction` 要原因。角色级策略未接 |
| GOV-04 | Schedule、Event、Presence | `/c/:id/life/schedule` | 部分接入 | 整日日程和事件已接。逐条建议未接。浏览世界不写 presence |
| GOV-05 | 关系编辑回滚 | `/c/:id/people` | 已接入 | 只回滚关系字段，要原因和证据 |
| GOV-06 | Memory revise 与 forget | `/c/:id/growth` | 已接入 | expectedRevision 与证据 |
| GOV-07 | Capability 审核 | 治理页；队列在 `/pending` | 部分接入 | 审核仍在治理页。待处理只跳转，不就地批准 |
| GOV-08 | Actor 用户背景 | `/c/:id/people` | 已接入 | 所有者背景不是 NPC，不能代发 |
| GOV-09 | Wardrobe 管理 | `/c/:id/life/wardrobe` | 部分接入 | 增删改已接。穿戴和使用命令未接 |
| GOV-10 | Foundation 与 Persona 提案 | `/c/:id/profile` | 部分接入 | 修订流已接。应用到实例未接 |
| GOV-11 | Developing Self | `/c/:id/growth` | 已接入 | 回滚和标记不准确要原因 |
| GOV-12 | 人格切换 | `/c/:id/profile` | 尚未接入 | 没有切换命令。设置文案写明不会把切换写成运行状态 |
| SET-01 | 模型角色绑定 | `/settings?section=model-role` | 已接入 | 原设置保存 |
| SET-02 | Endpoint 与模型 | `/settings?section=endpoint` | 已接入 | 原 Endpoint 表单 |
| SET-03 | 当前绑定查看 | `/settings?section=binding` | 已接入 | 只读当前绑定 |
| SET-04 | 媒体工作流 | `/settings?section=media` | 已接入 | 全局 ComfyUI，不是角色视觉定义 |
| SET-05 | 运行策略与保留 | `/settings?section=operations`；角色入口 `/c/:id/settings/autonomy` | 部分接入 | 全局策略已接。诊断保留只读。角色级未接 |
| SET-06 | Reply 与 Vision | `/settings?section=reply`；`/settings?section=vision` | 已接入 | 原回复和视觉设置 |
| SET-07 | KEV | `/settings?section=kev`；诊断 `/diagnostics?section=kev-decisions` | 已接入 | 设置与 Kev 诊断都还在 |
| SET-08 | Owner 密码与登出 | `/settings?section=owner`；顶栏退出 | 已接入 | 退出调用 `store.logout` |
| DIA-01 | Lifecycle | `/diagnostics?section=lifecycle` | 已接入 | 筛选面板还在 |
| DIA-02 | Agent 与 model runs | `/diagnostics?section=agent-runs`；`/diagnostics?section=model-runs` | 部分接入 | 分页和关联筛选已接。没有 `/diag/tasks/:taskId` |
| DIA-03 | Media prompt 重试 | `/diagnostics?section=media-prompts` | 已接入 | 失败项可重试。提示词标「生成图·非事实」 |
| DIA-04 | Workflow 控制 | `/diagnostics?section=workflows` | 已接入 | 暂停和恢复直接执行。取消、重启、重置要确认 |
| DIA-05 | KEV 审计 | `/diagnostics?section=kev-decisions` | 已接入 | `KevDiagnostics` |
| DIA-06 | 刷新、导出、清空 | 诊断详情工具栏 | 已接入 | 导出当前结果。清空有确认，且说明不删角色事实 |
| GAP-01 | Reflection 管理 | `/c/:id/growth` | 尚未接入 | 页内写明反思详情尚未接入 |
| GAP-02 | Agent 与 Tool 定义 | `/settings?section=model-role` | 尚未接入 | P3 只读列表没有接口，设置里未单列 |
| GAP-03 | World、Scene、Location | `/c/:id/world` | 部分接入 | 事件只读。地图、地点、场景未接入 |
| GAP-04 | NPC 作者工具 | `/studio` 分类「人格片段」 | 尚未接入 | 只显示未接入，不保存 |
| GAP-05 | 独立 Desire | `/c/:id/life/goals` | 尚未接入 | 目标页不另造愿望列表 |
| GAP-06 | 附件上传 | `/c/:id/chat` | 尚未接入 | 同 CHAT-04 |
| GAP-07 | Moment 发布 | `/feed` | 尚未接入 | 说明不能代角色发布，没有提交 |
| GAP-08 | 分组移出 | `/library` | 部分接入 | 归入已有。移出未做按钮 |
| GAP-09 | createConversation | `/c/:id/chat` | 尚未接入 | 客户端有创建会话方法，页面没有新分段 |
| GAP-10 | item-use 管理 | `/c/:id/life/wardrobe` | 尚未接入 | 使用中的读数和写入命令分开。没有使用按钮 |
