# 实施计划：交互与 LLM 请求调度优化

## 顺序与检查点

1. **先建立合同与迁移基础**
   - 更新 Go migration schema/compatibility SQL：`generic_llm` 兼容绑定、诊断场景/队列时间字段、`llm.queue` 设置允许项。
   - 更新 Core Provider 角色常量、旧角色迁移/兼容读取、Provider 场景 context 和优先级映射。
   - 检查点：Go migration/settings/provider 单元测试，确认旧业务角色数据仍能完成请求。

2. **实现 Provider 队列与诊断生命周期**
   - 新增优先级 FIFO 队列、分别的生成式/Embedding 并发上限、取消/超时/槽位释放。
   - 将 `Structured*`、`StreamText`、`Embed` 的 HTTP 执行包入队列；入队即记录 queued，执行前 running，结束更新终态。
   - 保持现有脱敏、correlation、idempotency header 和结构化解析行为。
   - 检查点：并发上限、优先级顺序、同优先级 FIFO、取消和错误释放、诊断状态转换测试。

3. **同步 Core HTTP、BFF、browser-client 合同**
   - 扩展 model-run 查询字段和 DTO；只返回 generic_llm/embedding 绑定。
   - 增加安全设置读写 `llm.queue`，必要时加入设置测试和 OpenAPI/browser-client 生成校验。
   - 检查点：Core/BFF route/parity/dto 测试，browser-client 类型检查。

4. **修复 Web 聊天交互和流完整性**
   - 修改 `ChatView.vue` 与 `app.css`，桌面/移动 composer 单行布局，提交即清空 draft。
   - 加固 conversations store 的 UTF-8/NDJSON flush、terminal 校验、草稿清理和 retry 状态。
   - 检查点：Web layout/accessibility/skeleton 测试，新增或更新聊天流测试，生产 build。

5. **更新设置和诊断中心**
   - SettingsView 只保留通用 LLM/Embedding 两个 tab，并展示并发设置。
   - DiagnosticsView 展示场景、绑定角色、队列状态、时间字段；定时刷新未完成记录，刷新页面后从服务端恢复。
   - 检查点：浏览器静态测试、响应式断点检查和脱敏字段回归。

6. **唤醒/反思恢复核验**
   - 补充职责注释和状态映射；修复或证明 wake-up terminal retry、stable cycle/idempotency、reflection intent 不重复。
   - 检查点：`go test ./...` 中 wakeup/workflow/reflection 测试，必要时添加回归用例。

7. **全量质量门禁**
   - 运行 `trellis-before-dev` 指定的相关 spec 检查，再运行 Go 测试、BFF 测试、Web lint/type/test/build。
   - 运行 `trellis-check` 做 spec/contract/cross-layer/quality 检查；修复后重新执行失败项。
   - 保留用户现有的三份未提交 `.trellis/spec/backend` 修改，不覆盖、不回滚。

## 预计修改文件/模块

- `apps/core-go/internal/core/provider.go`、新增 `provider_queue.go`、`diagnostics.go`、`settings.go`、调用场景文件（`app.go`、`mutations.go`、`autonomy.go`、`wakeup.go`、`workflow_ops.go`、`media.go`、`schedule_generation.go`、`intelligence.go`）。
- `apps/core-go/internal/migrations/runner.go`、Core diagnostics/settings HTTP handlers。
- `apps/gateway-go/internal/bff/dto.go`、`routes.go` 及其测试。
- `packages/browser-client/openapi.json`、`src/index.ts`、`scripts/generate.mjs`（若合同字段需要更新）。
- `apps/web/src/views/ChatView.vue`、`SettingsView.vue`、`DiagnosticsView.vue`、`stores/conversations.ts`、`stores/control-center.ts`、`styles/app.css` 及 Web 测试。

## 风险与回滚点

- Provider 队列接入是最高风险点：若队列 worker/HTTP context 错配，聊天会出现长期 pending；先以纯内存队列单测锁定生命周期，再接入每个 Provider 方法。
- 新旧角色兼容查询必须避免 embedding 被误当生成式；所有角色映射测试都要断言实际 endpoint/model。
- 诊断 schema 为 additive migration；若新增列导致旧数据库读取失败，回滚到兼容查询只读旧字段仍可运行。
- 前端流修复不得删掉现有 retry/cancel 逻辑；若出现回归，优先回退 store 的 terminal 处理而保留 composer 布局和立即清空行为。
