# 摇光项目第二阶段 ADK 对话运行接入

## Goal

在第一阶段 Eino 基础层已经通过当前代码/测试门禁的前提下，把现有直接对话运行收敛为唯一的 request-scoped ADK 对话运行模块：ADK 负责模型—工具—结果回填—继续生成，Core 继续负责鉴权、幂等、领域校验、冻结、事务提交和发布。

本阶段不重做第一阶段模型适配器、Composer、独立 Task、后台 Worker 或 BFF/Temporal；只修正对话链路的接线、人格动作能力入口、旧 Handler 控制流和对应证据。

## Phase 1 precondition evidence

- 基线提交：`89764bc`、`d82daf8`；当前分支 `codex/yaoguang-eino-adk`，worktree 干净。
- Core/Gateway 全量 test、race、vet 已通过；生产 Core 源码未发现旧 Chat/Embedding HTTP 入口。
- 第一阶段真实 Provider、Judge、ComfyUI、计费/缓存仍需外部凭据，不以 Fake 结果冒充真实兼容性。

## Requirements

### R1. 唯一对话运行模块

- 新增明确的对话运行模块/接口，统一 Main、query continuation、takeover Judge/B 和 recovery 接线。
- `HandleTurn` 保留鉴权、幂等、业务上下文、冻结/事务/发布职责，不再直接承担通用模型—工具交替控制。
- ADK Runner/ChatModelAgent 是正常对话唯一顶层循环；query continuation 保留 pure-query、次数、权限、response-mode 和 no-tools 约束。
- ADK 内每次模型调用继续使用第一阶段 Eino factory、queue、取消、timeout、request identity、per-call diagnostics 和 usage。

### R2. Capability 与人格动作

- ADK 工具目录唯一来源是现有 CapabilityRegistry；展示 definitions 与服务端允许执行集合一致。
- 只读工具返回真实 bounded result；控制动作和持久/外部动作进入现有 Prepare/freeze/transaction/intent 边界。
- 当轮人格接管和持久 active profile 切换拥有可追踪的明确 Capability/policy 执行入口；不改变 Judge、授权和生效时点。
- 接管后 B 重新使用第一阶段 Composer 的 persona/memory/relationship/tools 作用域；不能复用 A 的运行消息、候选或工具结果。
- 原始 tool-call ID、name、arguments、来源、业务运行关联和 CapabilityResult 保持一致；模型 ToolCall 与确定性 policy 调用不混淆。

### R3. 消息、提交和失败

- ADK tool result 与 assistant tool-call 一一对应；不重复 system、user、tool result 或 current input。
- 中间 event、Judge 输出、工具结果、废弃 A 候选不得直接发布；最终 visible output 与 `conversation.reply` 只走一次正式发布入口。
- 取消、超限、模型/工具失败和提交失败完成清理、审计、幂等/抢占收尾，不伪装为空成功。
- 不在 LLM 调用期间持有数据库事务，不新增治理平台。

### R4. 清理与回归

- 删除被替代的对话 Handler 循环、旧查询续接接线、人格递归重生成、临时 facade/别名/双轨开关和无效代码。
- Reflection、WakeUp、媒体、摘要、Embedding 继续使用第一阶段独立 Task。
- Fake ADK/ChatModel/Tool 覆盖正常对话、工具闭环、query、takeover、persistent switch、非法工具、消息配对、single publish、失败/取消/iteration cap、并发隔离和第一阶段回归。

## Out of scope

- 不重写 Eino factory、Prompt Slot/Composer、通用模型 Task、Embedding、后台业务规则或 BFF/Temporal。
- 不修改记忆/情绪/日程/人格演化规则，不新增 token 级展示、多 Agent 平台或新的事务/审计/调度平台。

## Acceptance Criteria

- [ ] 正式对话 runtime 负责 ADK Runner 事件迭代，`HandleTurn` 不再拥有通用循环。
- [ ] Fake ADK 测试从真实业务入口证明 ToolCall → Capability → 下一次输入匹配 tool result → final business result。
- [ ] 原始 tool-call ID 在 Eino/ADK、CapabilityInvocation、CapabilityResult、工具结果和 frozen payload 中一致；每个模型调用有独立 queue/diagnostic/request identity。
- [ ] query 次数/权限/no-tools/终止边界保持通过。
- [ ] takeover、持久切换和人格动作经过明确能力入口/记录，A/B 权限和废弃候选边界保持通过。
- [ ] 每阶段 Composer 重建正确作用域，消息无重复/孤立 tool result。
- [ ] 失败、取消、超限、非法工具和提交失败不产生空成功或重复发布；并发运行不串状态。
- [ ] 所有真实对话入口迁移，旧控制流/临时接线清除；全量 test/race/vet/静态扫描通过。
