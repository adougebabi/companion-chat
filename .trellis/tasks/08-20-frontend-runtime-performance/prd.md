# 前端启动与流式性能优化

## Goal

在同一个前端现代化任务中完成活跃客户端的一次性 Vue 重写、交互稳定性修复和加载/流式性能优化。当前任务保持既有视觉语言和主要信息架构；视觉系统调整另立后续任务，避免扩大本次技术重构范围。

## Scope

- 迁移期间新客户端源码放在独立的 `web/` Vite 项目中；旧 `src/` 只作为迁移参考和回滚材料。
- 最终 Express 只服务 Vite `dist/` 构建产物；旧 `src/` 目录和旧客户端入口必须在任务结束前删除，不允许新旧入口并存交付。
- 前端迁移方向确定为 Vue 3 + TypeScript + Vite，并采用一次性整体重写活跃客户端，不保留新旧 UI 长期并行路径。
- Pinia 管理跨页面 server/app state；composables 管理聊天、SSE、历史分页、composer 和输入法副作用；组件本地状态管理短暂 UI 状态。
- 父任务/后端架构讨论负责稳定 API/DTO 边界；本任务负责浏览器请求、状态、渲染、交互和技术迁移。

## Requirements

- contacts shell 在 bootstrap 完成后立即可交互；联系人视图不加载不可见会话。
- `web/index.html` 提供最小静态应用壳、联系人 skeleton 和稳定布局；Vue 应用加载后接管，不以空 `#app` 作为首屏。
- 页面启动始终先显示联系人/摇光实例列表，不为恢复上次聊天阻塞首屏。
- 用户进入聊天后只加载最后 20 条消息；滚动到历史顶部时按 cursor 继续加载更早的 20 条。
- 历史分页插入消息时必须恢复滚动锚点，不能覆盖草稿、正在发送的消息或新到达的尾部消息。
- 历史分页由顶部 sentinel/滚动边界自动触发；请求失败显示可重试状态，没有更多历史时显示明确边界，不要求正常路径点击“加载更多”。
- composer DOM 是独立且稳定的交互区域；轮询、SSE token、历史分页和普通消息 reconcile 不得替换 textarea 或主动关闭移动输入法。
- 只有用户明确提交消息后才允许清空草稿；请求失败、后台刷新、焦点变化和 provider 延迟都必须保留草稿。
- 监听 `compositionstart` / `compositionend`，输入法组合期间禁止重建聊天视图；不得用 `window.focus` 作为刷新触发器。
- SSE token 更新使用单个 transient assistant 节点和 animation-frame coalescing，done 后完整 reconcile。
- 保留 draft、selection、scroll、active persona、polling guard 和 `done.messages=[]` 语义。
- 图片支持 lazy/async 解码和尺寸占位；视频默认 `preload=none`，用户激活后再加载。
- activity 页不因重复 DOM/render 或完整媒体预加载造成移动端卡顿。
- 生产静态资源支持压缩和版本化缓存，API/HTML 缓存语义正确。
- 保持联系人、聊天、动态、设置、创建流程、媒体状态、检查器和移动布局的现有视觉语言与主要信息架构；只修复技术迁移所必需的布局稳定性和状态反馈问题。
- 当前技术重构不能破坏联系人优先、20 条历史分页、SSE 增量渲染、草稿/IME 保留、无障碍语义和现有 API 行为。

## Acceptance Criteria

- [ ] contacts 首屏不等待 conversation，也不会因为 localStorage 中的上次实例而预加载聊天记录。
- [ ] 直接打开 HTML 时先看到静态壳和 loading 状态，JS 接管后不会重复创建导致布局跳动。
- [ ] 聊天首屏只请求最后 20 条；向上滚动可以继续按 20 条分页，直到历史边界。
- [ ] 历史分页后滚动位置、草稿、SSE 新消息和后台刷新状态保持正确。
- [ ] 历史自动加载失败可重试，已到历史边界时不会重复请求。
- [ ] 手机输入法打开时经过多次轮询、SSE 流式输出和历史分页，输入框、组合文本、焦点和草稿不丢失。
- [ ] 发送失败会保留用户原文，只有明确接受发送后才清空输入框。
- [ ] 50/100 条消息的长回复流式期间不会替换整个消息列表，且输入草稿/滚动位置不丢失。
- [ ] 移动端媒体布局无明显跳动，首屏不会自动拉取所有视频。
- [ ] activity 首屏资源请求和渲染次数有前后基线，未引入新的后台轮询循环。
- [ ] 静态资源实际 transfer size 可证明被压缩，重复加载遵循 cache policy。
- [ ] `node --check src/companion-main.js` 及 Express-served desktop/mobile 手工验收通过。
- [ ] Vite production build 生成可缓存的 hashed assets，并且 TypeScript 类型检查成为 CI gate。
- [ ] 一次性新客户端覆盖联系人、聊天、20 条历史分页、SSE、动态、设置、人格创建、媒体、检查器和移动端交互，没有遗漏旧活跃入口的用户流程。
- [ ] 新客户端覆盖所有旧活跃入口流程，并保持现有视觉语言；不引入未规划的视觉系统重设计。
- [ ] Express、Docker 和本地启动路径只引用新 `dist/` 产物，旧 `src/` 目录及旧入口已删除。
- [ ] 新旧客户端迁移期间通过页面/交互 contract fixtures 和脱敏 replay 对照；删除旧 `src/` 后再次完成桌面/移动端回归。
- [ ] 开发 Vite server、生产 Express/Docker、CI typecheck/build/test/smoke 全部验证同一个新 `web/` -> `dist/` 入口。
- [ ] 前端 boot、首屏、history pagination、SSE stream 和资源加载指标可按 correlation/flow 查询，且错误状态不泄露敏感信息。

## Dependencies And Out Of Scope

- 依赖父任务的 cross-layer SSE/API contract review；依赖 backend child 提供批量 activity DTO 后再优化消费端。
- 在父任务确定实体、关系、生活流和当前在场状态的顶层模型前，本子任务只做启动/渲染架构讨论，不启动页面重组或状态迁移实现。
- 不修改聊天 prompt、模型 tool schema 或 SQLite schema；一次性重写活跃客户端，旧 UI 不作为新入口继续运行。视觉语言和设计系统重做明确不属于本任务，后续另立任务。
