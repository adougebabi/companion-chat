# 最终本地检查

- Go test ./...：通过，JSON 日志记录 PASS 事件 1759、SKIP 事件 531（包含父测试与子测试，不能当独立验收案例数）。无 fail 事件。
- Go test -race ./internal/core ./internal/workflow：通过，条件化数据库/真实服务用例仍 SKIP。
- Go vet ./...、Go build ./...：通过。
- pnpm typecheck/test/build：通过；Browser 21、Core Client 2、Web 105 全通过。构建保留既有 >500 kB chunk 提示。
- 原生 HTTP/SSE 格式/阶段矩阵20个子场景通过；每场景一次模拟 reply effect。新 PG 8个场景 SKIP，不算持久化证明。
- 真实模型、原事故实际输入 A/B、ComfyUI/S3/Temporal/Redis、真实人格长聊：NOT_RUN。

完整分层状态在 acceptance-matrix.md；summary 在 verification/check-summary.json；逻辑上下文估算在 verification/context-metrics.json。构建和测试仅本地，不是部署或真实模型验收。

## F14容量可用性追加最终检查

- 八项新增顶层容量/传播/结算/日志回归通过，含原实现RED→GREEN。24条模型影响与Core新增实际状态引用可完整结算，模型原24项合同保留。
- 最新Go test ./...：PASS事件1772、SKIP事件531（含父/子事件），16包通过、14包无测试文件，无FAIL。条件化DB/真实服务缺失并未变为通过。
- 最新Go race ./internal/core ./internal/workflow、vet ./...、build ./...：通过。
- 最新pnpm typecheck/test/build：通过，Browser21、Core Client2、Web105；保留既有>500 kB chunk提示。首次pnpm test因沙箱IPC listen EPERM未启动，获本地监听权限后同命令通过。
- 不涉及公共API/schema变更，未重新生成客户端；未连接真实DB/模型或操作日志中的Temporal workflow。
- F14结果和日志摘要见verification/capacity-check-summary.json，说明见docs/context-capacity-availability.md。本追加修复当前未提交。
