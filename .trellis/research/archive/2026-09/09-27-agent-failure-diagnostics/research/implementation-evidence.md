# 实施与验证记录（2026-09-28）

## 已实现

- `agent_runs` additive保存 parent correlation和结构化失败阶段/代码；Owner诊断查询将正式Agent循环、物理模型调用、Tool摘要分层呈现。正式循环后发布/结算失败通过 `agent.run.termination`独立显示；入库前失败也有事件回退。旧行关联未知，不按时间伪造。
- Tool诊断的 `model_call_id`来自 trace记录的实际物理请求，而非假设回调继承Generate子context。
- Core/BFF/Browser OpenAPI与生成客户端添加 Agent Runs查询；页面按 correlation展示安全失败原因。Owner-only与无效过滤错误分别映射403/422。
- `safe_cause`对历史自由文本做限长和敏感词/URL凭据保守脱敏；Tool失败码按稳定token语法和128字符上限校验。取消后的生命周期诊断使用独立短时context。
- `.trellis/spec/backend/fluctlight-diagnostics-contract.md`新增可执行场景合同。

## 已执行的验证

- 隔离 `pgvector/pgvector:pg16` 临时库运行仓库 `cmd/migrate` 到 `0041_agent_run_diagnostics`。
- 完整 `go -C apps/core-go test ./internal/core ./internal/httpapi/... ./internal/migrations/... -count=1` 在迁移后的临时库通过：Core 236s、migrations 35s、两个HTTP包通过。首次运行因临时库 `public` 尚未迁移导致旧测试“表不存在”，迁移后重跑全绿。
- 定向数据库回归覆盖 Agent逻辑失败/物理完成、正式Agent后续结算失败、旧行/入库前事件、真实Tool→物理轮次、取消后诊断、0040→0041迁移；真实两轮Eino Tool adapter的 `conversation.reply`子测试通过。
- `go -C apps/core-go vet ./internal/core ./internal/httpapi/... ./internal/migrations/...`、`pnpm generate`、Web/Client typecheck、Web测试56项、Web生产构建和 `git diff --check`通过。

## 联合验收时仍需做

- 在有Owner会话的生产方式Web服务中打开诊断页，核对桌面/窄屏失败组可读性、刷新和关联过滤；不要使用真实个人数据做破坏性测试。
- 父任务最终验证后再做批次提交；本子任务尚未单独提交或归档。
