# Implementation checklist

1. 固定正式接线与基线请求、Provider 可用性；核查初始化 JSON 对稳定偏好的保留。
2. 扩展 Working Persona 数据契约、编译 Agent、校验和持久化版本读取；测试画像保真与失败。
3. 接入初始化、正式更新和存量补编译；确保事务外模型调用、CAS 与发布一致性。
4. 接入 Main、WakeUp、接管后的唯一画像装配，版本检查和状态读取；测试实际发送请求。
5. 注册人格详情只读 Tool，完成独立调用、权限、分段和同一 Eino Loop 续接测试。
6. 运行真实 Provider E2E、预算对比、行为场景测试；缺依赖时明确记录 BLOCKED。
7. 清理替代路径、同步规范、全量质量检查并交付证据。

## Validation

- `cd apps/core-go && go test ./internal/core/... ./internal/ai/...`
- 运行仓库既有真实 Provider E2E 条件与新链路测试，记录环境、实际请求、Trace、SKIP 原因。
- `git diff --check` 与 Trellis backend Quality Check。

## Risk boundaries

- `app.go` 创建事务、`operations.go` revision 发布、`working_persona.go` 发言人格判定及 `tool_execution.go` 授权为关键回滚点。
- 不在远程 LLM 调用中保持数据库事务；不在生产全库运行付费补编译。

## Current execution status

- Completed in code: formal compiler and persisted Working Persona; initialization, Foundation and stable overlay lifecycle; startup readiness and scoped backfill; Main/WakeUp/takeover egress; direct persona.detail and native Eino feedback; prior full-profile egress cleanup.
- Controlled PostgreSQL chain, migration, HTTP replay, direct Tool and Go package checks passed. See `evidence.md` for exact commands, traces, budget figures and baseline comparison.
- BLOCKED acceptance: real Provider compilation/Tool Loop and multi-sample behavior checks require live Provider URL/model/key. The live suite reports SKIP without them, which is **not** acceptance.
- Pre-existing full DB suite failures were reproduced on original commit `c966c31`; keep task open until real acceptance and any separately scoped baseline fixes are resolved.
