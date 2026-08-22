# 迁移收尾与回归修复

## Goal

修复 2026-08-21 后端模块化和 Vue 前端切换后发现的功能遗漏、交互回归和架构边界偏差，使已归档任务的完成状态与实际验收结果一致。

## Confirmed Gaps

- 前端设置页丢失模型/provider/ComfyUI/h3 配置。
- 隐藏动态管理入口只发请求、不展示/恢复结果。
- Debug inspector 缺少 h3 preflight、事件模拟和测试媒体作业动作。
- 聊天顶部 sentinel 未接通，短于一屏的 20 条历史无法自动加载更早页。
- 切换摇光实例会清空未发送草稿。
- Vue 接管静态壳时 `boot=idle` 会先显示空态再显示 loading。
- settings/inspector 等非核心模块没有动态分包；生产没有压缩传输。
- 前端性能、browser/mobile smoke、真实服务 smoke 尚未完成。
- 后端 generic flow registry 主要覆盖 chat；life/relationship/media 等 flow 仍自行持有 transaction/job side effects。
- ContextFragment 仍是 source/priority/text + 字符数预算，capability layer 在预算外拼接，未达到设计中的结构化预算边界。

## Requirements

- 保持 `/api/companion/...`、SSE `token/done/error`、SQLite 数据和部署契约。
- 新前端必须覆盖旧活跃入口：联系人、聊天、历史分页、动态、设置、人格创建/详情、媒体、隐藏动态、检查器和移动交互。
- 发送失败、轮询、历史分页、切换页面不得丢失草稿或关闭 IME；切换实例应按实例保存草稿，除非用户明确清空。
- 顶部 sentinel 在消息不足一屏时也能触发历史分页；分页必须恢复锚点且去重。
- 设置和检查器动作必须有完整 API/UI/错误反馈。
- 后端所有可执行 flow 必须通过统一 registry/runner 或明确的通用 flow/effect adapter；不得由领域 flow 私自复制 enqueue/retry/settlement。
- ContextFragment 必须包含 section、priority、required、budget/provenance 等结构化元数据；预算器负责必需片段和总预算，serializer 只负责序列化。
- 生产静态资源支持压缩和 hashed cache；非核心前端模块动态分包。
- 完成真实 Express `dist` 服务 smoke、浏览器 desktop/mobile smoke、后端 API/worker/restart smoke，并记录结果。

## Acceptance Criteria

- [x] 前端旧流程全部可用，设置/隐藏动态/检查器不再是死入口。
- [x] 联系人首屏稳定，20 条聊天历史和顶部自动分页在短/长消息列表都可用。
- [x] 草稿、IME、流式输出、滚动锚点和错误恢复通过浏览器/代码回归验证。
- [x] 后端 flow/effect/prompt 边界符合设计，新增回归测试证明没有重复 job/provider 路径。
- [x] `npm test`、`npm run typecheck`、`npm run build`、临时数据库 smoke、Express dist smoke 和浏览器 smoke 全部通过。
- [x] 归档任务中的 deferred 项已记录明确结果；真实外部 provider 执行保留为环境依赖，不伪造通过。

## Out Of Scope

- 摇光视觉系统重设计；只修复当前功能所需的布局/状态反馈。
- `/api/companion`、`companion_*` 表和部署标识的 major rename。
