# 第五阶段执行计划：联合验收与证据交付

## Phase A — Freeze and matrix

- [x] 冻结提交、分支、工作区、工具版本、Go/Node/PNPM/Eino/ADK/Temporal 版本和隔离环境；保存脱敏 baseline。
- [x] 阅读阶段一至四最终要求、交付文档、当前 specs 和源码；创建 S1/S2/S3/S4/CROSS 稳定验收 ID 矩阵。
- [x] 盘点真实模型入口、ADK/Capability/Task/WakeUp/Browser API 调用图和所有旧路径。

## Phase B — Deterministic L0/L1 verification

- [x] 第一阶段：Eino factory、queue/cancel/audit、Composer/Slot budget/isolation、Task wrappers、旧 Provider/Prompt 扫描。
- [x] 第二阶段：ADK Runner、正式 ToolCall、结果回填、query、takeover A/B、persistent switch 权限、single publish、失败/取消/iteration cap；DB settlement 缺口标 BLOCKED。
- [x] 第三阶段：WakeUp shared ADK、Daily Review/Native Cognition 分类、自治/权限/freeze/intent/outbox、取消/重试/无动作；DB/Temporal 缺口标 BLOCKED。
- [x] 第四阶段：真实 API Router、Session/CSRF/CORS/DTO/error/NDJSON/media、无 HTTP forwarding、前端/Compose/CI 删除扫描；浏览器/Compose 缺口标 BLOCKED。
- [x] 跨阶段：工具副作用、事务/CAS、幂等、Outbox、队列许可、Worker/Temporal 接线和前台/后台隔离；不可运行项逐项标 BLOCKED/NOT_RUN。

## Phase C — L2/L3/L4 when safe

- [x] 检查可用的隔离 PostgreSQL/Redis/Temporal/MinIO 和浏览器环境；缺失条件逐项标 BLOCKED/NOT_RUN，不改变生产配置。
- [x] 评估已有真实浏览器链路：本机无可用浏览器/完整服务环境，明确标记未验证。
- [x] 评估 disposable Compose/API+Worker/Temporal/媒体 smoke：私有 env/外部服务未提供，明确标记 BLOCKED；未调用真实 Provider/Embedding/视觉。
- [x] 记录有限调用次数/工具次数/取消资源释放和 usage 观察；无真实 latency baseline，不宣称性能回退结论。

## Phase D — Report and delivery

- [x] 发现缺陷时保存原始失败，最小修复并重测；本轮没有确认源码 FAIL，保留环境 Skip/阻塞证据并更新源码快照和行号。
- [x] 生成 report、CSV、manifest、相对路径 evidence 和脱敏 review-bundle.zip。
- [x] 通过 `trellis-check`，更新 specs，提交/归档 task 和 journal，确认工作区干净（源码快照保持 clean；报告/任务 artifacts 待本阶段归档）。

## Required checks

```bash
go -C apps/core-go test -mod=readonly ./...
go -C apps/core-go test -mod=readonly -race ./internal/httpapi ./internal/core
go -C apps/core-go vet ./...
go -C apps/core-go mod tidy -diff
pnpm --filter @fluctlight/web lint
pnpm --filter @fluctlight/web typecheck
pnpm --filter @fluctlight/web test
pnpm --filter @fluctlight/web build
pnpm --filter @fluctlight/browser-client typecheck
pnpm --filter @fluctlight/browser-client test
pnpm generate
./infra/acceptance/go-core-reference-guard.sh
./infra/acceptance/check-core-openapi.sh
docker compose --env-file infra/compose/fluctlight.env.example -f infra/compose/fluctlight.compose.yml config --quiet
git diff --check
```
