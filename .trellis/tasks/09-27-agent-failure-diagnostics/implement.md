# 执行计划

1. 复核 Owner 诊断 DTO、`agent_runs` 与 `model_call_id` 真实写入；先写受控“物理 completed、Tool failed、Agent failed”回归。
2. 加逻辑 Agent run 的安全读投影和必要的稳定关联/迁移；修正真实 Tool callback 的 round 关联；取消/超时诊断用独立 bounded context。
3. 更新 Core/Browser API、OpenAPI 与生成客户端，前端按逻辑 run 分组并显示失败阶段、安全原因；旧行显示未知。
4. 验证 Owner 权限、脱敏、限长、两次物理调用、Tool 失败、取消/超时及旧记录兼容；运行 Go Core/BFF 测试、`pnpm generate`、Web typecheck/test/build，并实际打开生产构建的诊断页面。
5. 更新 `.trellis/spec/backend/fluctlight-diagnostics-contract.md` 的逻辑运行展示契约，并在父任务联合验收时核对 correlation。

回滚点：先让新后端读投影 additive 上线，再切换前端展示；旧 Model Runs 查询与已有终止事件保持可用。

验证命令：`go -C apps/core-go test ./internal/core ./internal/httpapi/...`、`pnpm generate`、`pnpm --filter @fluctlight/web typecheck`、`pnpm --filter @fluctlight/web test`、`pnpm --filter @fluctlight/web build`。涉及数据库/浏览器的集成场景在隔离环境中补跑。
