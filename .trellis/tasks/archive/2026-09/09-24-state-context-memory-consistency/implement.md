# 实施与验证清单

| 现状 | 本次调整 | 受影响调用方 | 验收方式 |
| --- | --- | --- | --- |
| Effective Life 已有当前态，但总投影/结算未校验所有领域版本 | 定义字段权威和 Fluctlight 事务代际，补齐读写约束与当前态输出 | Main、WakeUp、Reflection、persona.detail、media、schedule、Tools | 剪发/库存/穿着/人格切换/迟到结果数据库及多轮测试 |
| 任务投影部分统一，初始化与部分一次性任务直拼；原生工具轮次由 Eino 管理 | 收敛投影入口、预算/来源诊断、工具后刷新，保留原始消息/回执 | Main、WakeUp、Reflection、初始化、native/daily/switch、相关一次性任务 | 请求样本、原生配对、预算、权限与多模态测试 |
| Memory 修订与 summary digest 已有，跨层来源/失效不足 | 来源关系、状态/版本、纠正传播、legacy 迁移 | memory_event、Reflection、summary、检索、governance | 迁移重跑、来源校验、删除/竞态测试 |
| Active/Durable/Summary/Working 已有，缺显式 Episode/Resident 合同 | 增量 Episode → Long-term → Resident、代际发布、工具读取 | 旁路任务、memory.recall、自动投影 | 跨天事项、无变化不重跑、预算/搜索、发布失败测试 |

## 顺序

1. 保存实施前基线：Git、相关测试、真实送模/调用次数与资源可用性；记录未执行项。
2. Current State：字段归属、版本与结算、初始化语义、当前态读取/写入；针对性测试。
3. Context Projection：所有相关正式模型入口、预算/来源映射、工具轮次刷新/只读压缩；针对性测试。
4. Memory Provenance：存储迁移、历史 backfill、合法引用、纠正/失效传播、查询过滤；针对性测试。
5. Memory Hierarchy：Episode、Long-term、Resident 的增量触发/代际发布、现有 `memory.recall` 接入；针对性测试。
6. 清理旧路径；Go 全量测试、race/vet/build，workspace typecheck/test/build；有 disposable PostgreSQL 时执行真实迁移和数据库测试。
7. 用现有 Provider 及实际入口完成自然语言多轮验证，记录诊断 SQL/请求样本、工具结果、持久化重读、成本和延迟。编写独立报告并核对 PASS/FAIL/BLOCKED。

## 安全检查点

- 每次修改前重读即将编辑的源码并确认 Git 状态；保留外部未提交改动。
- 迁移只对 disposable 数据库执行；迁移前后核对数量、版本和引用完整性。
- Tool 回执、历史消息、诊断与秘密不做破坏性清理；报告脱敏。
- 用户已明确要求不自动提交/推送，完成后保留改动供复核。

## 本次执行结果

- 四项代码、`0037_memory_provenance` 迁移、正式调用方迁移及相关隔离数据库测试已完成。最终证据与 PASS/FAIL/BLOCKED 明细见 [`docs/verification/state-context-memory-consistency/report.md`](../../../docs/verification/state-context-memory-consistency/report.md)。
- 最终 `GO_CORE_TEST_DATABASE_URL=... go -C apps/core-go test -mod=readonly -p 1 -parallel 1 -count=1 ./...`、聚焦 race、`go vet`、`go build`、`pnpm typecheck/test/build` 均通过。五类真实场景各有一次成功样本；额外虚拟理发样本的不稳定结果保留为单独限制。
- 未提交、未推送；按仓库 Trellis 工作流，任务在用户自行决定提交前保持 `in_progress`，本轮不归档。
