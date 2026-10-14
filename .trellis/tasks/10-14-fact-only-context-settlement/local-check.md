# 最终本地检查

- Go test ./...：通过，JSON 日志记录 PASS 事件 1759、SKIP 事件 531（包含父测试与子测试，不能当独立验收案例数）。无 fail 事件。
- Go test -race ./internal/core ./internal/workflow：通过，条件化数据库/真实服务用例仍 SKIP。
- Go vet ./...、Go build ./...：通过。
- pnpm typecheck/test/build：通过；Browser 21、Core Client 2、Web 105 全通过。构建保留既有 >500 kB chunk 提示。
- 原生 HTTP/SSE 格式/阶段矩阵20个子场景通过；每场景一次模拟 reply effect。新 PG 8个场景 SKIP，不算持久化证明。
- 真实模型、原事故实际输入 A/B、ComfyUI/S3/Temporal/Redis、真实人格长聊：NOT_RUN。

完整分层状态在 acceptance-matrix.md；summary 在 verification/check-summary.json；逻辑上下文估算在 verification/context-metrics.json。构建和测试仅本地，不是部署或真实模型验收。
