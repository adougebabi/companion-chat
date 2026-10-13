# 全站统一化实施证据

## 已完成巡检

`artifacts/ui-audit-2026-10-13/report.md`保存F01–F17、file:line定位与现场截图；`coverage.md`保存实际覆盖边界。原始证据不改写为已修复。用户2026-10-13明确要求第一、第二期全部落地。

## 共享基础

- `apps/web/src/styles/globals.css:55`默认primary深色；`styles/app.css:14`产品accent蓝色，`:40`全局button颜色继承，`:96`产品按钮至少42px。`components/ui/button/index.ts:11`默认bg-primary，`:19`默认h-8。目标规划曾加局部文字补丁，根因应在共享层修复。
- 现有Input/Textarea/Select/Dialog/Accordion/Tabs可复用；FormField、统一反馈/异步状态、ConfirmDialog、JSON查看器尚不存在。Dialog已有portal/overlay/focus底座。不可新造第二套基础组件体系。
- `lib/zoned-local-input.ts:3`处理选定IANA时区的wall clock、DST gap/fold；诊断时间用viewer zone，聊天仍用冻结的sender provenance。

## 背景统一入口契约

- `components/instances/ActorContextEditor.vue:9`缺relationship_distance/meeting_confirmed；`lib/actor-user-background.ts:3`另一列表缺interests/preferences。Browser DTO与Core支持10字段；meeting_confirmed为boolean/null，其余string/null。
- `packages/browser-client/src/index.ts:124`Owner接口使用expectedCurrentFactsRevision；`:130–131`通用接口使用expectedContextVersion。二者并发token不同，不得互换。
- `apps/core-go/internal/core/actor_context.go:43`context_version由facts history+relationships摘要决定，`:125`source=direct；Owner写入也source=direct。correct/change必须保留，来源不可任意选择。
- `GovernanceView.vue:22`把第一个ID标成Owner是位置推断。`BrowserSession`位于browser-client`:30`已有actorId；conversations store initialize/login目前仅保留authenticated（`:358–376`）。可以保留session.actorId为可信Owner标签与adapter选择依据，不添加新后端身份契约。
- Actor编辑epoch、草稿保留和幂等身份需覆盖目标变空、实例切换、保存时改草稿及409；不把旧请求结果赋给新对象。

## 设置往返

- `apps/core-go/internal/core/settings.go:133`按setting key替换整个JSON。自治/media开放对象应合并已加载对象与编辑字段；queue/wakeup则为封闭schema，拒绝未知key，不能机械保留非法字段。
- `internal/ai/model/queue.go:14–15`并发整数范围1–8。
- `wakeup.go:17–21`周期固定600秒，UI只展示10分钟并提供enabled开关。
- `fluctlight-diagnostics-contract.md`明确保留30天/10000行固定清理，diagnostic_retention无runtime consumer。不能把存储值呈现成已生效调整。当前有效规则只读，存储配置可放高级详情，不新增后端消费能力。
- autonomy存在mode/allowed_actions/budget_remaining/quiet_hours/topic_suppression_seconds；修改普通字段需保留未显示字段与现有自定义action，topic suppression保存边界300–604800秒。空actions在服务器被换默认，UI不可把空列表描述为完全禁用。
- `media.go:573–628`支持baseUrl/base_url、media_workflows、visual_identity_workflows和stage分支；当前SettingsView saveMedia重构对象会丢分支。必须无损往返。
- `SettingsView.vue:35–52`已有支持裸占位符的parseWorkflowTemplate；可提取并做可执行测试。无服务器模板目录/校验API。媒体入口提供已保存模板、用户文件/文本导入及JSON结构校验，不伪造可运行模板或把语法通过宣称为ComfyUI运行通过。

## 验证设施

- Web使用node --test，typecheck=vue-tsc，build=Vite。`test/conversation-delivery.test.mjs:596–669`已有真实SFC+Pinia+fixture fetch+SSR模式；goal-panel、goal-planning-panel、diagnostics已有脚本执行harness。
- `apps/web/scripts/serve-preview.mjs`仅提供dist预览，无fixture API；Nginx生产入口同源代理/api/auth/health。需要为浏览器安全交互提供任务自有fixture服务，不能把真实Provider E2E当UI fixture。
- 不创建新的全套测试框架；沿用现有行为harness与CUA浏览器交互。以现有dist静态服务或同源fixture服务验证构建产物，真实API仅读。
