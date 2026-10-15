# 页面覆盖表
覆盖程度：**可操作**（主操作会写 store 或打开真实 UI）｜**仅展示**（只读；表单只记录偏好也算这里）｜**占位**｜**未完成**。
由 `js/modules/registry.js` 中实际注册的路由生成（2026-10-15）。共 55 条路由（`life.now` 注册了两条）：可操作 35 · 仅展示 18 · 占位 2 · 未完成 0（页面级）。

所有页面都可用右上角「演示状态」切换 正常/空/加载/失败/未接入/冲突/部分成功；冲突态「保留草稿」可点击（显示“草稿已保留”）。
非小遥角色（阿青 异常 / 木木 已暂停 / 星野 已退休 / 向导新建的 未启动）在角色内页面显示**自己的状态卡**，不再回落到小遥数据。

| 分区 | page key | 页面 | 路由 | 覆盖 | 备注 |
|---|---|---|---|---|---|
| 相处体验 | `chat.thread.view` | 对话 | `#/c/:charId/chat` | 可操作 | 角色库进入默认落地；通知/动态深链。右侧摘要随 store 实时刷新 |
| 相处体验 | `chat.history.view` | 会话历史与搜索 | `#/c/:charId/chat/history` | 可操作 | 对话页顶部「历史」 |
| 相处体验 | `chat.correction.list` | 纠错记录 | `#/c/:charId/chat/corrections` | 可操作 | 对话右栏「纠错记录」；日记/记忆的“可能有误”提示 |
| 相处体验 | `chat.call.view` | 语音与通话（P2 占位） | `#/c/:charId/chat/call` | 占位 | P2：语音输入/实时通话仅占位。 |
| 相处体验 | `chat.media.view` | 聊天中的图片 | `#/c/:charId/chat/media` | 仅展示 | 对话附件按钮（未接入） |
| 相处体验 | `feed.post.list` | 动态 | `#/feed` | 可操作 | 全局导航「动态」。用户只能以自己的 Actor 发帖，或向角色提建议 |
| 相处体验 | `feed.post.detail` | 动态详情 | `#/feed/:postId` | 可操作 |  |
| 相处体验 | `settings.char.autonomy#outreach` | 主动触达设置 | `#/settings/outreach` | 仅展示 | 仅记录偏好，不改变运行时权限 |
| 相处体验 | `inbox.notice.list` | 通知中心 | `#/inbox` | 可操作 |  |
| 生活成长 | `life.now` | 生活 · 此刻 | `#/c/:charId/life/now` | 可操作 | 工作区“生活”默认 tab；对话页状态条；通知深链 |
| 生活成长 | `life.now` | 生活 · 此刻 | `#/c/:charId/life` | 可操作 |  |
| 生活成长 | `items.wardrobe.view` | 衣柜与物品 | `#/c/:charId/life/wardrobe` | 可操作 | 生活 tab；此刻页当前穿着；行动完成卡 |
| 生活成长 | `life.schedule` | 生活 · 日程 | `#/c/:charId/life/schedule` | 可操作 | 生活 tab；通知 nt_1；地点详情「建议她来这里」 |
| 生活成长 | `goals.list.view` | 目标 · 意向 · 约定 | `#/c/:charId/life/goals` | 可操作 |  |
| 生活成长 | `diary.list.view` | 日记 | `#/c/:charId/life/diary` | 仅展示 |  |
| 生活成长 | `diary.entry.detail` | 日记条目 | `#/c/:charId/life/diary/:entryId` | 可操作 |  |
| 生活成长 | `diary.shared.timeline` | 共同经历时间线 | `#/c/:charId/life/shared` | 仅展示 |  |
| 生活成长 | `growth.memory.list` | 记忆库 | `#/c/:charId/growth` | 可操作 |  |
| 生活成长 | `growth.memory.item` | 记忆条目 | `#/c/:charId/growth/memory/:memId` | 可操作 |  |
| 生活成长 | `growth.reflection.timeline` | 反思与成长时间线 | `#/c/:charId/growth/reflection` | 仅展示 |  |
| 生活成长 | `growth.reflection.detail` | 反思详情 | `#/c/:charId/growth/reflection/:refId` | 可操作 |  |
| 生活成长 | `growth.self.view` | 自我认知 | `#/c/:charId/growth/self` | 可操作 |  |
| 生活成长 | `profile.char.view` | 角色资料 | `#/c/:charId/profile` | 可操作 | 每个角色显示自己的资料与状态 |
| 世界人物 | `world.map.view` | 世界总览 | `#/c/:charId/world` | 仅展示 |  |
| 世界人物 | `world.place.detail` | 地点详情 | `#/c/:charId/world/place/:placeId` | 可操作 | 浏览地点不会移动角色；「建议她来这里」走行动请求 |
| 世界人物 | `world.event.detail` | 事件详情 | `#/c/:charId/world/event/:evId` | 可操作 |  |
| 世界人物 | `world.event.list` | 世界变化与事件流 | `#/c/:charId/world/events` | 仅展示 |  |
| 世界人物 | `world.scene.view` | 当前场景 | `#/c/:charId/world/scene` | 可操作 | 默认远程查看，无“进入场景”按钮 |
| 世界人物 | `world.instance.settings` | 运行世界设定 · 随机事件 | `#/c/:charId/world/settings` | 可操作 | 永久伤害类事件系统强制禁用，不可绕过 |
| 世界人物 | `people.graph.view` | 人物关系 | `#/c/:charId/people` | 仅展示 |  |
| 世界人物 | `people.npc.detail` | NPC 详情 | `#/c/:charId/people/npc/:npcId` | 仅展示 |  |
| 世界人物 | `people.list.view` | 人物目录 | `#/c/:charId/people/list` | 可操作 |  |
| 世界人物 | `people.owner.background` | 我的背景 | `#/c/:charId/people/owner` | 可操作 |  |
| 世界人物 | `people.relation.edit` | 关系修正 | `#/c/:charId/people/relation/:relId` | 可操作 |  |
| 世界人物 | `media.gallery.view` | 媒体库 | `#/c/:charId/media` | 仅展示 |  |
| 世界人物 | `media.identity.view` | 视觉身份 | `#/c/:charId/media/identity` | 仅展示 |  |
| 世界人物 | `media.item.generate` | 媒体生成 | `#/c/:charId/media/generate` | 可操作 | 输出均标「生成图·非事实」，不改变事实 |
| 世界人物 | `media.item.detail` | 媒体详情 | `#/c/:charId/media/:mdId` | 仅展示 |  |
| 创作治理 | `auth.signin` | 登录/注册 | `#/auth` | 可操作 | 原型模拟登录，不连接真实账号 |
| 创作治理 | `library.home` | 角色库 | `#/library` | 可操作 |  |
| 创作治理 | `library.char.create` | 新建角色向导 | `#/library/create` | 可操作 |  |
| 创作治理 | `studio.home` | 创作中心 | `#/studio` | 可操作 |  |
| 创作治理 | `studio.resource.edit` | 资源编辑 | `#/studio/edit/:resId` | 可操作 | 创作中心首页；角色资料“有新版本”提示；通知 nt_3 |
| 创作治理 | `studio.preview` | 试生成沙盒 | `#/studio/preview` | 可操作 | 结果只写 studioDrafts，标「创作预览·不进入生活」 |
| 创作治理 | `studio.apply` | 应用到实例 | `#/studio/apply` | 可操作 |  |
| 创作治理 | `studio.event.template` | 事件模板 | `#/studio/template` | 仅展示 |  |
| 创作治理 | `settings.global` | 系统设置 | `#/settings` | 仅展示 | 仅记录偏好，不改变运行时权限 |
| 创作治理 | `settings.char.autonomy` | 角色自治与触达设置 | `#/c/:charId/settings/autonomy` | 仅展示 | 权限只读展示；偏好仅记录 |
| 创作治理 | `diag.home` | 运行诊断 | `#/diag` | 可操作 |  |
| 创作治理 | `diag.task.detail` | 诊断任务详情 | `#/diag/task/:taskId` | 可操作 |  |
| 创作治理 | `diag.audit` | 操作审计日志 | `#/diag/audit` | 仅展示 |  |
| 创作治理 | `diag.pending.list` | 待处理事项 | `#/diag/pending` | 仅展示 | 由 proposals/resources/tasks 派生，不另存 |
| 创作治理 | `data.tools` | 数据工具 | `#/data` | 可操作 | 配置包可导出/导入为草稿；业务备份 P2 未完成；回档 P3 只有说明 |
| 创作治理 | `data.object.revisions` | 对象修订记录 | `#/data/revisions` | 仅展示 |  |
| 创作治理 | `data.rollback` | 回档说明（P3） | `#/data/rollback` | 占位 |  |

## 页面内分段标注
| 页面 | 分段 | 覆盖 |
|---|---|---|
| `data.tools` | 配置包导出（JSON + 下载链接）/ 校验 / 导入为草稿 | 可操作 |
| `data.tools` | 业务数据备份（P2） | **未完成**（按钮禁用，如实写明） |
| `data.tools` | 时间线回档（P3） | 占位：只链到说明页 `data.rollback` |
| `settings.global` / `settings.char.autonomy` / `#outreach` | 偏好表单 | 仅展示：「记录偏好」只写 `prefs`，明确不改变运行时权限；权限表只读 |
| `world.instance.settings` | 永久伤害类事件 | 仅展示（复选框禁用，系统强制，导入配置包也会重新锁定） |
| `chat.media.view` | 发送图片/语音 | 未完成（接口待核验，附件按钮禁用） |
| `media.identity.view` | 修改视觉身份 | 未完成（需在角色定义里编辑，未接入） |
| `auth.signin` | 登录/注册 | 可操作，但为模拟（不连真实账号） |

抽屉/弹窗：`chat.correction.edit`、`chat.action.detail`、`items.item.detail`、`media.item.generate`（拍照弹窗）、`studio.apply`（抽屉，另有 `#/studio/apply` 列表页）、建议日程抽屉、约定抽屉、新建资源弹窗，全部可操作，可 Esc/遮罩/× 关闭。
