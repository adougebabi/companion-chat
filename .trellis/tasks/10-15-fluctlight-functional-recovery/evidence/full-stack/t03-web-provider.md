# T03 Web provider / WP01—03 component wiring

该记录是实现证据，不替代 docs/v3-page-acceptance.md 功能台账，不表示真实浏览器/数据库验收通过。

## 实施边界

保留现有 proto 页面和共享 DOM/CSS；真实 provider + generated BrowserClient + 现有 Pinia 会话协议层。未挂载旧 Vue 业务整页。真实 server snapshot facade 不接受任意 mutator；未接动作明确报错，不写本地业务事实。Mock 仅显式 VITE_PRODUCT_MODE=mock，失败不回退，query 无切换开关。

## 现已绑定的真实动作

- Owner session / setup-status / login / setup / logout，复用 generated client cookie / CSRF。
- 角色列表、分组读取/创建/成员移动/删除；暂停/恢复/退休使用已读取 current_revision + 原因 + 状态读回。
- 创建向导：完整描述→模型分析→展示生成结果→稳定 requestId 激活；固定设备时区。分组失败报告已激活的部分成功，不假称创建失败后重复创建。
- 关于你：全部当前 actorUserBackgroundFields，三页签，Owner current_facts_revision 与独立 Actor context_version 分开；保留 draft/key、409 冲突重读、修订/来源；非 human Actor 拒绝。Owner reason 500 code points / 其他 Actor reason 1000 UTF-8 bytes。
- 聊天：直接复用原 Pinia 的 NDJSON、冻结发送时钟、持久 turn/idempotency、排队、取消、重试、部分提交阻断、pending 恢复；输入框保留挂载，消息区域增量重画、保留滚动位置。导航只终止 observer。
- 历史：conversationDirectory 分页、selectDirectConversation、searchConversationMessages 全会话搜索分页、newDirectConversation 稳定幂等；消息深链有界读取更早页。
- 媒体 attachmentRefs 走 /api/media/:assetId；不恢复图片发送入口。原消息来源显示 sourceRef/conversation/turn，纠错交流真实发送；仅已有明确 memoryRef + revision 时可所有者 CAS reviseMemory 并读回，未猜测关联。
- ProfileProvider 注册真实 detail 映射、简介 Foundation 修订/采纳/CAS/读回；persona-switch pending → ownerRequest<BrowserPersonaSwitchTerminal> 有界观察 → 终态目标一致 → detail 再读。导航取消观察不取消任务。
- 未接业务服务通过统一 services facade 报错，未恢复基于审计文案猜命令。

## 已执行检查

- Web typecheck: passed。
- Web test: 155 passed / 0 failed（同目录 t03-web-test.log）。包括既有真实会话 identity/queue/abort regressions 与新真实仓库拒绝假写、accepted人格任务观察/取消观察不发取消命令测试。
- Web production build: passed；构建后 verify-product-build.mjs 检查 fixture namespace/seed/Mock 诊断样本/调试 writers，删除 dist/v3-qa。
- 显式 VITE_PRODUCT_MODE=mock 另输出 /tmp/fluctlight-web-mock-build，构建 passed；不是生产验证证据。
- git diff --check（apps/web）passed。

## 验收入口与限制

真实 dev http://127.0.0.1:5175/（PID 26748，exec session 24724），VITE_PRODUCT_MODE=real，proxy http://127.0.0.1:8081，Origin http://localhost:5175。
本切片未操作真实浏览器或真实模型/DB链路；主线程执行统一真实验证。关于你/创建/生命周期/人格切换的新后台契约仍需对应 real HTTP/Worker 回归与视觉对照。其余 WP04后业务页面未逐动作接通，此记录不声明全量完成。
