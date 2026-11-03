# Shell / Companion 源码清单

生成 `inventory-shell-companion.json`，共 59 个子视图/操作族，涵盖 shell、角色库、认证、聊天、角色档案、人物关系、动态、通知，并列出正式版额外能力。所有视觉、交互和业务状态均为「待检查」。本轮只做现行源码定位，没有运行浏览器或声称视觉通过。

## 已核实的关键差异与后续检查抓手

- **头像来源存在明确差异。** Demo `src/components/shared.tsx:247–276` 的顺序为 data:image URL、世界插画模式、portrait 或隐式 `id === "linwan"` 人物插画、名字首字。正式 `apps/web/src/react-app/shared/CharacterAvatar.tsx:14–24` 只用 URL 与 initial/名字末字；`apps/web/src/v3/proto/data/profile.js:26–34` 优先 detail.avatar 再 directory.avatar。正式角色库和侧栏直接用 `_directory.avatar`。本轮对现行 react-app、approved-ui、state 检索没有发现 `linwan`；不能把 Demo 的人物插画当真实头像资产。
- **世界/此刻插画已简化。** Demo `App.tsx:252` 为 yuejian 选 forest，其余 city；正式 `LibraryPage.jsx:134` 固定 city。Demo `CompanionPages.tsx:616–623` 按角色选择此刻画面；正式 `ChatPage.tsx:217` 默认 `<WorkspaceMomentArt/>`，其默认 books 与装饰声明见 `shared/WorkspaceMomentArt.tsx:1–10`。这是源码差异，尺寸与视觉仍待实测。
- **聊天外来文本预填缺少对应实现。** Demo `CompanionPages.tsx:349–359, 545–564` 消费 `route.params.say`，已有不同草稿时显示替换/放下建议。正式 `ChatPage.tsx:73–86` 仅恢复 sessionStorage 草稿，当前文件无 say/promptOffer 对应逻辑。应从跨页入口联测；不要直接推断所有正式入口都会传此参数。
- **快速前往的页面集合变小。** Demo `App.tsx:637–645` 从 PAGE_LABELS 搜索，包含 create；正式 `WorkspaceQuickJump.tsx:11` 只取 workspaceNavigationGroups，因此创建页没有对应搜索结果。正式补了无结果与无角色过滤。
- **角色库入口丢失明确类型参数。** Demo `App.tsx:330` 创作入口附 `kind: character_template`；正式 `LibraryPage.jsx:145` 仅 `w.navigate("创作中心")`。实际创作默认页是否仍为人物模板需浏览器验证。角色卡最近相处时间在正式版改为固定说明文字。
- **新增人物流程不是一步保存。** Demo `CompanionPages.tsx:2676–2688` 直接保存本地人物目录；正式 `shared/PersonDefinition.jsx:169, 276, 326` 创建定义、发布、核对差异再应用到世界。正式编辑已有真实人物走 `PersonDetail.jsx:93–140`。验收必须区分「定义已保存」「已发布」「已进入世界」三个结果。
- **动态操作改为持久命令确认。** Demo `CompanionPages.tsx:3289–3319, 3430–3447` 即时点赞、评论、隐藏；正式 `pages/FeedPage.jsx:205–288` 先确认回应/隐藏/恢复，并使用版本与幂等键回读。正式支持关心、庆祝与撤销回应；这些是额外能力。Demo 分享建议一定同步发动态和通知 (`3343–3400`)，正式请求、角色判断与实际发布结果分开 (`FeedPage.jsx:350–411`)。
- **通知里的恢复按钮迁移到正式来源。** Demo `CompanionPages.tsx:4176–4258` 展示失败任务轨迹并直接恢复，2.2 秒后写本地媒体结果 (`3901–3942`)。正式 `InboxPage.jsx:43` 详情只展示真实正文与原始来源链接，任务恢复交给正式来源页；需要实际走链接验证原任务身份、结果与返回通知。正式 pending 从真实多业务分组汇总，另分进度与服务区，不能以已读代替完成。
- **认证差异是业务契约。** Demo `App.tsx:338–419` 邮箱+任意6位密码、注册切换与直接体验。正式 `AuthPage.tsx:28–160` 是部署所有者密码≥12位，首次初始化另需≥16位令牌和确认密码，有真实状态读取、401与重试。Demo说明/重置也是Demo专属，无需在正式版恢复。

## 正式版额外能力，Demo不适用

JSON 已单列：真实初始化与失败恢复；未知角色/非法路由/启动失败；历史全会话搜索、分页、来源消息定位、继续所选会话；资料原提交与回读恢复、版本冲突重整、人格准备恢复、身份修订回滚；参与者 actor 范围与额外自填字段；真实接触记录、事件/记忆回读、人物移除、在场/接触/机会记录与重判；动态真实媒体多选、建议拒绝/失败/恢复、NPC互动重试；通知真实 attention/progress/service 分组与来源分页。

## 审计限制

源文件行号为现行文件锚点；JSON fields 是待核对字段组，不代表这些字段已经在真实数据/真实 DOM 中完整出现。特别是角色档案正式编辑器使用 `approved-ui/profile-fields.mjs:77` 动态遍历，再由 `shared/ProfileForm.jsx:31` 分类，必须用真实档案逐组打开核对，不能只看四个页签就判字段完整。没有修改生产、Demo或测试代码。
