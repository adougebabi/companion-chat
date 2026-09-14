# 多重人格接管、Working Persona 与 Prompt 收敛

## 目标

基于当前 Go 代码实现「未执行候选 → 轻量 Judge → 最终发言人格 → 仅执行胜出候选」，并收敛普通 Main 请求的人格与响应职责。完整原始需求保存在 [request.md](request.md)，其 1–20 节均属于本任务约束；本文不替代或缩减原始要求。

## 已确认背景

- 用户已授权创建本任务，要求先盘点代码、形成契约与迁移表，再实施和验证，不因阶段划分重复确认。
- 创建时分支为 master，工作区干净，基线提交 7b26364。
- 项目使用 Trellis；本任务由主会话直接实施与检查。探索代理仅在独立且有收益的只读任务中使用。
- 原始只读报告是待核实线索，不是当前代码证据。

## 要求

| ID | 要求 | 原始需求章节 |
| --- | --- | --- |
| R01 | 盘点用户输入到最终投递、冻结恢复、状态消费者、QUERY continuation，产出带代码位置的调用链表和职责迁移表 | 2、19 |
| R02 | A/B Candidate 包含工具及根级状态提案，仲裁前完成结构、参数、权限及约束校验；拒绝候选不得产生任何业务副作用或事实记忆 | 3–6、15 |
| R03 | 从正式工具契约无副作用地生成真实待发送内容预览；用户私聊保持唯一 reply Capability 管线 | 4–5、9–10 |
| R04 | 单次 takeover_once 分离 persistent active 与 turn reply owner；B 独立重生成，禁止反向仲裁，失败不回退发送 A | 1、3、9 |
| R05 | Runtime 从合法规则选定唯一接管方；Judge 只判断布尔值，使用有限控制投影、数据隔离、明确预算与异常降级 | 6–8、14 |
| R06 | QUERY continuation 与 takeover 互斥；主生成/合成合计最多两次，Judge 单独统计；B pure QUERY 需要第三次合成时受控失败 | 10 |
| R07 | forced_activation 逐条分类为确定性持续切换、语义持续切换或本轮接管；保留不明确原始规则并诊断，稳定 ID 贯穿全链路 | 7、11 |
| R08 | Core 先合并 Shared Identity、当前 profile 和合法 Overlay，再确定性投影唯一 Working Persona，保护身份、关键语义和修订可追溯性 | 11–12 |
| R09 | 收敛 Main Prompt/Schema，保留真实原生工具协议；信任边界、历史发言归属和整个请求预算可验证 | 12–14 |
| R10 | 复用冻结、幂等、短事务、版本控制和投递机制；模型及外部执行在事务外，恢复不得重判或重演已执行动作 | 15 |
| R11 | 复核并小范围修复 traits、relationship.role、默认值来源、共享作用域、时区、TOON 递归和 assistant 自我举证问题 | 16 |
| R12 | 完成确定性边界测试、Persona/Prompt 契约测试、适用真实模型隔离测试、全请求大小与全调用成本性能报告 | 17–18、20 |

## 验收标准

- R01：修改执行链之前具备实际文件位置表、职责迁移表、forced_activation 逐条迁移表。
- R02–R05：Fake Provider/Judge 与 Spy Executor 证明不接管、接管、非法候选、Judge 降级、B 失败及提示注入数据隔离；A 的所有根级状态和工具副作用在被拒时均不提交。
- R04、R07–R08：takeover_once 后 active 不变且消息属于 B；下一轮仍按 persistent profile 构建；持续切换入口仍可更新 active；共享关系和目标跨人格可见；合法规则 ID 可追溯。
- R06：纯查询保留原结果合成；混合候选不续调用；接管后的纯查询不能触发第三次生成；静态守卫体现精确边界。
- R08–R09：A 的 Main Prompt 不含完整 B，反之亦然；唯一权威 Working Persona 保留准确 traits 与行为特征；原生 tool_calls 不重复；预算覆盖 tools、response_format 和输出预留。
- R10：取消、超时、冻结恢复及并发快照失效测试证明无提前发送、无重复执行、无孤立永久切换、无旧状态覆盖；拒绝候选不会进入事实记忆。
- R11：每个复核成立的缺陷具有精确字段测试，不用全对象关键词匹配代替契约断言。
- R12：按项目命令执行 build/test/lint，适用时执行 go test ./... 和相关包 race；区分基线失败、环境阻塞与新增回归。真实模型、Token、可见延迟等未实际测量项目明确标记未验证。
- 交付代码、简洁架构文档、规则迁移表、测试与性能报告；逐项回答原始需求第 20 节的八个问题。

## 范围边界

不引入新 Agent 框架、MCP、向量基础设施或工作流引擎；不重写 Initialization、全部 Memory 生命周期或媒体资产平台；不增加 visible_reply 并行发送路径；不支持递归抢答、多人格竞价或同轮 QUERY continuation 与 takeover 叠加；不破坏角色卡与历史，不虚构真实模型验证或作品资产。

## 当前证据状态

**代码盘点已完成**（2026-09-14，基线 `7b26364`）。证据见 `research/` 下四份盘点报告：

| 报告 | 覆盖范围 |
| --- | --- |
| `research/inventory-turn-chain.md` | 从用户输入到最终投递的 21 步调用链表、候选冻结生命周期、副作用落点清单、`conversation.reply` 与 QUERY continuation 真实实现、事务与恢复、执行资格校验 |
| `research/inventory-prompt-schema.md` | Prompt 组装链、消息布局与裁剪顺序、Main response schema 字段消费者表、工具协议、预算设施、重复序列化审计、Provider 能力边界 |
| `research/inventory-persona.md` | 人格数据模型与持久化、切换入口清单、`forced_activation` 全链路、Evolution Overlay 合并顺序、三处人格序列化出口、作用域审计、第 16 节线索逐条核实 |
| `research/inventory-tests-specs.md` | Fake/Spy/Clock 测试基础设施现状、现有相关测试清单、静态守卫清单、相关规范硬约束、构建测试命令、环境阻塞 |

### 关键盘点结论（影响需求可行性）

**有利**

- 候选冻结（`cognition.go:398`）与结算事务（`mutations.go:1122`）之间存在真实「未执行」窗口，且结算以 frozen payload 为唯一事实源 → 拒绝 A 的候选副作用可通过覆盖 payload 实现，无需在五个副作用点分别打补丁。
- `conversation.reply` 无二次模型改写，私聊发送路径唯一，不存在 `visible_reply` 旁路。**但「唯一发送管线」≠「唯一文本权威」**：可信文本由 `response_plan.visible_text` / `decision.visible_text` 优先决定（`mutations.go:830`），本次须收敛为唯一 canonical 值。
- Judge 可作为独立 Provider 角色复用既有 `generic_llm` 绑定与 `model_run` / `provider_provenance` 记账，无需新表或新服务；**复用当前模型绑定已获接受**，但须显式配置并如实上报超时/输入预算/最大输出/采样参数与成本。
- 已有请求预算设施可复用（token 估算 + 三层 cap + 确定性守卫：`prompt_context_assembler.go:14-19/38-46/255-259`、`provider_live_tool_test.go:34-47`），本次补分节字节数与 Judge 覆盖即可。
- A 的审计行 `cognition_assessments` / `cognition_decision_proposals` **只有写入、全仓零读取方**（`cognition.go:455/458`）→ 被拒候选不可能经它们进入反思或记忆。这是实测证据而非承诺。

**需注意（R1 修订，2026-09-14）**

- `cognition_frozen_actions` 只有 `frozen/completed/failed`，**不存在 rejected/superseded 状态**；被拒候选改为记录在同一 payload 内，并用 payload 内的 `turn_stage` 表达执行资格。
- **`RowsAffected` 不是 CAS**（`cognition.go:474/488/502` 条件只有 `id` + `status`）；覆盖写必须叠加 `turn_stage` 预期值与既有版本门控。
- **候选校验不在仲裁点之前**：`prepareCapabilityInvocations` 由 `mutations.go:925` 调用（仲裁点之后）；须在仲裁前增加一步复用既有 `validateCapabilityInvocationsForPersistence`（`capability_runtime.go:205-220`）的无副作用校验。
- **`forced_activation` 不是「0 条规则」**（R1 更正）：仓库自带多头卡 `testdata/initialization/dense_multi_card.txt` 明确声明「遇到强烈的公开质疑时星火可强制接管」，且 `dense_multi_expectations.json` 的断言 `id=forced` 要求产出的 `forced_activation` 含「公开质疑」。但它无结构、无 ID、无校验、无消费者，且唯一校验路径是 live provider（未设 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1` 时 skip）→ **本地未验证**。因此 R07 必须让语义明确的旧规则产出**可执行**接管规则，不能整体归入诊断。
- **持久切换入口的真实情况**：`applyPersonalityDecisionPlanTx` 调用方只有三处 turn 结算路径；WakeUp / Reflection 只读不切换；`applyPersonalityDecision` 是死代码；无服务端时间引擎。当前唯一可用机制是 **Main 驱动**（`switching` 规则 + `personality_decision`）。`request.md` 中 3 处「已有切换入口」的表述已按代码证据更正。
- 不存在 Fake Judge、可控时钟、统一测试 App 构造器 → `implement.md` 阶段 1 必须先补测试基座。
- **时区缺陷成立**（R1 更正）：`provider_context.go:626-629` 对历史时间戳强制 UTC 且**无时区标识**；此前以「TOON 扁平渲染」推翻它不成立。TOON 递归问题**尚未充分复核**。

### 规划产物

- `design.md` — 技术设计（**已修订至 R1**：权限模型与执行脊 E1–E5、权限迁移表、R07 迁移表、唯一文本权威、阶段机与恢复枚举、静态守卫与运行时预算、A/B 作用域读写表、已决议事项）
- `implement.md` — 执行计划（12 个阶段 + G0/G1/G2 检查点 + R1–R5/R2.5 回滚点）
- `implement.jsonl` / `check.jsonl` — 已填入真实 spec 与 research 条目（占位行已清理）
- `review-response.md` — 对评审报告 F01–F14 的逐条代码核实 + 用户决议采纳结果
- `yaoguang_persona_design_review.md` — 外部评审报告（存档）

### 实施前需评审确认（已关闭 — 决议见 `design.md` §11）

1. ~~`persistent_deterministic` 仅分类还是实现求值器~~ → **只分类 + 稳定 ID + 诊断**，不实现确定性时间引擎。
2. ~~接管规则的正式声明入口~~ → **`takeover_rules[]` 为唯一执行语义**；`forced_activation` 作为受控迁移输入，语义明确者必须产出可执行规则。
3. ~~是否接受「无新表、接管状态全部存于 frozen payload」~~ → **接受**，但须叠加 `turn_stage` / `winner` / `expected` / `persistent_switch` 与执行资格守卫。
4. 新增：**持久切换职责不离开 Main**，改为逐调用场景明确授权，并在候选校验/结算/恢复三处执行权限。
