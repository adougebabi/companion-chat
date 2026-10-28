# 正式代码与 Demo 的对应关系

参考仓库：https://github.com/adougebabi/companion-chat

参考分支：`codex/fluctlight-functional-recovery`

固定提交：`a74f19a4a9990055dc1fd9db3c8ce8086086dd4a`

下面的路径相对于参考仓库。顶层导航依据 `apps/web/src/approved-ui/routing.mjs`，对象级操作依据 `feature-routes.mjs`，优先使用正式 `react-app` 页面，旧 `v3-domain` 只用于确认尚由域页面承载的字段和操作。

## 页面映射

| Demo 主页面  | 原页面 / 关键依据                                                                                               | Demo 内部细分                                              |
| ------------ | --------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| 全站壳层     | `react-app/App.tsx`、`shared/WorkspaceNavigation.tsx`                                                           | 角色切换、分组导航、面包屑、折叠与移动抽屉                 |
| 角色库       | `react-app/pages/LibraryPage.jsx`                                                                               | 关键词/标签、角色卡、聊天与资料入口、创建草稿              |
| 聊天         | `react-app/pages/ChatPage.tsx`、`components/ChatTurnFeedback.tsx`                                               | 消息、输入、等待、原回应重试、历史、附件、此刻预览         |
| 此刻         | `react-app/pages/LifePage.jsx`、`views/LifePage.jsx`                                                            | 活动、心情、地点、穿着、后续日程、时间线                   |
| 日程         | 同上；`v3-domain/life-schedule.js`                                                                              | 日期、时间线、建议提交、采纳与拒绝                         |
| 目标         | `v3-domain/goal-editor.js`、`goal-planning.js`                                                                  | 目标、重要度、紧迫度、意图、核对进展                       |
| 衣柜         | `react-app/pages/WardrobePage.jsx`、`views/WardrobePage.jsx`                                                    | 搭配、方案库、全部衣服、分类季节、服装详情与拥有状态       |
| 日记         | `v3-domain/diary.js`、正式 LifePage                                                                             | 日记阅读、生成、批注、共同经历                             |
| 记忆         | `v3-domain/memory.js`、`reflection.js`、`react-app/pages/SelfPage.tsx`                                          | 正文纠正、来源、修订、遗忘、反思、自我认知                 |
| 角色档案     | `react-app/pages/CharacterDetailPage.jsx`、`shared/ProfileForm.jsx`、`approved-ui/profile-fields.mjs`           | 身份、性格、表达、生活、人格、切换、历史、治理             |
| 世界         | `react-app/pages/WorldPage.jsx`、`v3-domain/world-management.js`、`world-items.js`                              | 世界目录、地点树、分类、地点编辑、物品目录、事件           |
| 动态         | `react-app/pages/BrowsePage.jsx`、`v3-domain/moments.js`                                                        | 角色范围、动态、评论、详情                                 |
| 人物关系     | `react-app/pages/BrowsePage.jsx`、`v3-domain/about-you.js`、`npc-management.js`                                 | 人物卡、详情、接触记录、关于你、自填与角色认识             |
| 媒体         | `react-app/pages/MediaPage.tsx`、`AvatarPage.tsx`、`v3-domain/visual.js`                                        | 媒体库、素材、视觉定义、候选审阅、生成、设定图、头像       |
| 创作中心     | `react-app/pages/StudioPage.tsx`、`StudioEditorPage.tsx`、`StudioPreviewPage.tsx`                               | 九类资源、类型字段、JSON、草稿、发布、历史、应用、沙盒     |
| 通知与待处理 | `react-app/pages/BrowsePage.jsx`、`v3-domain/operations.js`                                                     | 已读、未读、待处理、任务恢复、来源上下文                   |
| 管理中心     | `react-app/pages/ManagementPage.jsx`、`views/ManagementPage.jsx`、`v3/proto/components/configuration-fields.js` | 服务、用途、聊天视觉、图片、自主、后台通知、诊断数据、高级 |
| 任务详情     | `react-app/pages/TaskDetailPage.tsx`、`shared/WorkflowRecovery.tsx`                                             | 状态、阶段、失败、恢复、原请求和执行记录                   |
| 数据管理     | `v3-domain/data-tools.js`、`backups.js`                                                                         | JSON 预览/导入导出、本地演示备份                           |
| 初始化       | `react-app/pages/CharacterWizardPage.jsx`、`state/creation-controller.mjs`                                      | 四步向导、四种来源、分组完整预览、七阶段准备               |
| 账户界面     | `react-app/pages/AuthPage.tsx`                                                                                  | 登录/注册表单演示，不执行认证                              |

以上 `react-app` / `v3-domain` 路径均位于 `apps/web/src/` 下。

## 初始化重点依据

- JSON 输入大小、禁止字段与嵌套结构：`apps/web/src/v3/creation-json.mjs`。
- 初始化模板：`apps/web/public/templates/yaoguang-character-init-template.v2.json`。
- 世界复用和修订确认：`apps/web/src/v3/reuse-world.mjs`。
- 七项进度与恢复：`apps/web/src/approved-ui/creation-progress.mjs`、`creation-recovery.mjs`。
- 空白/定义模式约束：`apps/core-go/internal/core/app.go`、`provider_schemas.go`。
- 人格/衣柜/日程/视觉次序：`apps/core-go/internal/workflow/creation_order.go`。
- 世界底稿结构：`apps/web/src/v3/proto/components/world-definition.js`。
- 草稿与发布生命周期：`apps/web/src/state/studio-editor.mjs`。

## 信息架构上的重新设计

- 保留原有顶层业务模块，使用统一工作区承载。
- 原来的对象级独立路由，部分收进同页详情/编辑弹窗；返回时保留筛选和上下文。
- 角色详情改为按主题阅读与编辑，长 JSON 置于完整设定入口。
- 设置按二级目录拆分，不把所有参数放在一个长页面。
- 创建世界保留保存和发布两步，发布后可返回原初始化草稿。
- 为纯前端状态增加明确的演示场景入口，方便观察失败与恢复。

这些是界面和交互重组，不声称正式后端增加了新能力。
