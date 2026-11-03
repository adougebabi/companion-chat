# 实施与验证交付记录

日期：2026-09-22。状态：代码及不依赖真实生成模型的验证已收敛；**完整双真实 E2E 待用户按资源条件串行执行，不计作通过**。任务保持 in_progress，未归档、未提交/推送。

## 最终架构与行为变化

- 17 个正式 Agent 共享锁定 Eino v0.7.37 的 ChatModelAgent/Runner。各自拥有任务提示词、输入/上下文、角色、Tools 和 final 契约；无工具任务也走正式运行边界。生产对话、WakeUp、Native Cognition、Daily Review 和完整视觉身份任务已迁移。
- 18 个业务 Tool 通过 `App.ExecuteTool` 独立执行，Eino adapter 使用同一实现。授权 actor、subject、资源、业务 operation ID 和可选 working profile 显式传入；没有 Agent、surface、source 前缀、冻结轮次门禁。业务需要的证据及资源约束仍保留。
- 查询真实读取；mutation 的业务结果、receipt 和 outbox 在同一短事务提交。异步 `accepted` 是已持久受理，不能冒充资产完成。真实 native ToolCall/物理 Provider request 身份与业务幂等身份分开。
- 后续模型失败、取消或 final 非法不会撤销既有 Tool 提交。`agent_runs` 阻止整轮盲目重放；`tool_executions` 保留实际结果。最终认知通过 before/after authority receipt 链校验本轮写入，不能覆盖无关并发更新。
- Persona Tool 返回包含已接受演化结果的 working persona，后续 Memory/Relationship 使用相应作用域，关系写入归属于实际回复人格；takeover 不改变下一轮持久主导人格，persistent switch 则真实提交。
- 独立视觉 Agent 使用真实对象字节的 `image_url` 输入，通过 generation/review/finalize Tools 保存 canonical 与 character-sheet 状态。待媒体完成时不重跑模型决策。
- 已删除两轮限制、QUERY-only continuation、写工具后强制停止、超限成功、外层重复执行、旧 deferred/fake-completion seam、旧 A/Judge/B 编排和 turn_decision.go。正式 standalone Judge/Reply Agent 契约及历史持久命令的必要校验保留。
- 流式 Provider 由 Eino 原生处理；浏览器继续只显示已提交的回复，不提前泄漏部分 JSON、ToolCall 参数或 reasoning。新增 real `StreamTurn`/SSE/NDJSON 核验行，仍待 live 执行。

## 当前源代码与命令证据

四项最终 Go 检查的逐文件源码 SHA-256 均已与当前工作树核对一致。具体 HEAD 与聚合摘要见 acceptance-matrix.md；各 meta.json 保存完整命令、耗时、退出码与源码哈希。

| 检查 | 实际命令核心 | 结果 | 证据目录 |
|---|---|---|---|
| 全仓隔离 PostgreSQL 回归 | `go -C apps/core-go test -p 1 -parallel 1 -count=1 -timeout 20m -json ./...` | exit 0；1217 passing events，0 fail，25 skip 事件（无测试包和 opt-in live） | `research/runs/final-source-go-db` |
| race | `go -C apps/core-go test -race -p 1 -parallel 1 -count=1 ... ./internal/ai/... ./internal/core -run <记录的固定范围>` | exit 0；93 passing events，0 skip | `research/runs/final-source-race` |
| vet | `go -C apps/core-go vet ./...` | exit 0 | `research/runs/final-source-vet` |
| build | `go -C apps/core-go build ./...` | exit 0 | `research/runs/final-source-build` |
| 18 Tool adapter + 依赖/授权拒绝 | `TestFormalToolEinoAdapterE2E` | 55 passing events；真实 PG + 受控 HTTP Provider | `research/runs/adapters-full-failures-02`，最终全仓同样通过 |
| 严格单 Tool 命令 | `python3 /tmp/lac-run-final-live.py --suite tools --tool memory_event --run-dir .../strict-selected-memory-delivery` | exit 0；inventory + 指定 adapter + 独立业务用例全部通过；未请求真实模型 | `research/runs/strict-selected-memory-delivery` |
| 前端 | `pnpm typecheck`、`pnpm test`、`pnpm build` | exit 0；12 client + 47 web tests | `research/frontend-and-runner-checks.md` |
| runner/verifier | `node --test infra/acceptance/run-go-live-provider-smoke.test.mjs infra/acceptance/verify-go-e2e-events.test.mjs` | exit 0；17 tests；scope-label 变更后再跑指定回归通过 | 工具执行记录及 `research/frontend-and-runner-checks.md` |
| 视觉非生成式预检 | `TestVisualIdentityLiveE2EPreflight` | exit 0；Comfy system_stats 与隔离随机空 S3 bucket；无 LLM/生图 | `research/runs/visual-dependencies-preflight-host.jsonl` |
| 格式/脚本 | `git diff --check`；`bash -n infra/acceptance/run-go-live-provider-smoke.sh` | exit 0 | 本次工具记录 |

旧失败尝试完整保留。两个 test-only 破坏实验的目标子进程在故障存在时失败，恢复生产实现后通过；最终全仓及 race 又执行并通过。故障开关没有进入生产代码。

## 审查与验收脚本修正

独立审查未发现事务/CAS/运行恢复/native identity/profile 方面的具体缺陷。接受并修复两项 runner P1：指定 Tool 必须跑 adapter，完整 Agent 必须跑固定跨场景 gate 和真实生产 StreamTurn 行。Node 反例证明缺失/失败的 adapter 或跨场景行不能通过。

审查提出的“必须在模型 final 前显示原始 token”未采纳为产品回归：HEAD 原本已在提交后调用 onChunk，且提前发布候选/协议内容违反本任务发布约束。证据和精确边界说明保留在 `research/final-architecture-check.md`，没有把最终完整 token frame 当作 Provider 流式证据。

## 待用户真实验收

- 全部 17 个正式 Agent 的最终 real Provider 行。
- 真实随机秘密两次 recall/三次决策闭环和生产 StreamTurn/SSE/NDJSON 行。
- `schedule.replan` 的独立真实模型计划与提交。
- 真实 ComfyUI candidate、看图决策、canonical、character-sheet 与 S3 产物闭环。
- 恢复后的最终完整 tools/agents 两套严格命令。

所有 live 行当前仍为 NOT_RUN/PENDING；旧 GPU OOM 和随机秘密错误答案不算通过。视觉 handler live 行不单独证明 Temporal transport/history/worker recovery。

本机会话已提供 `/tmp/lac-run-final-live.py` 私有环境装载入口；使用方式、环境寿命和测试栈清理见 `research/user-live-verification.md`。现有专用回归栈只读复用配置，本任务的测试数据均在隔离库/bucket。保留测试栈供用户复验，不能对原有 phase8 栈执行清理。
