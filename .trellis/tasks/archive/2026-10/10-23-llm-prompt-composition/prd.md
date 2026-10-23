# 统一 LLM 提示词组成与最小必要上下文

## 目标

减少所有 LLM 请求中的重复规则、重复事实、混合序列化和无用持久化元数据；统一提示词组成，并交付逐 Agent 的实际输入清单，防止后续开发重复引入同类问题。

## 背景与证据

用户样本位于 `/Users/vinson/.codex/attachments/33d25905-57db-4815-bec7-cf91dcf3bffe/已粘贴的文本.txt`。4 条消息正文共 57,020 字符，其中 5 条历史 catalog evidence 约占 50.1%；另有 stable/runtime 目标重复、日程多种时间表示和固定规则重复。

已验证原因与锚点：

- `apps/core-go/internal/core/goal_evaluation_task.go:250` 将专用评估输入复用通用 Reflection surface，`provider_context.go:662` 又注入目标。
- `goal_evaluation_source_input.go:29` 无 catalog 专用投影；`goal_evaluation_wire.go:432` 递归按字段名改写，已损坏样本中的 tool schema。
- `prompt_context_assembler.go:457、548` 与 `provider_prompt_composer.go:194` 分别输出 JSON、YAML/TOON、带 YAML 的 system。
- `formal_agents.go:74` 注册 28 个 Agent，调用方各自组装；还需覆盖 Kev、媒体子任务和 correction 等入口。
- 2026-10-23 `bea62002` 已移除 token caps，Provider 规范已有覆盖旧条款说明，但其他规范仍残留冲突预算要求。

完整组成、规模、历史策略和回归出处在 `research/current-composition.md`，该文件与本 PRD 一起作为验收追溯材料。

## 要求与验收

| ID | 要求 | 可观察验收 |
| --- | --- | --- |
| R1 | 统一所有正式 Agent、共享 Agent 子任务及辅助模型输入的构造规范，按职责提供材料 | 全部入口有组成矩阵和测试；共同顺序为运行协议、任务动态数据、按需历史和当前输入；业务结构化数据不再混用 TOON/YAML/JSON |
| R2 | 固定规则和每类事实单点拥有，减少重复 | 样本重建中 selected goal/criterion 无 runtime 重复；日程派生时间无多份；catalog 历史结果不再夹带无关完整工具契约；相同证据正文复用但所有事件可独立引用 |
| R3 | 仅发送任务必要字段，Core 持有数据库与执行身份 | 纯 id/version/digest/lease/审计字段不泄漏；必要 opaque refs 可正确解析并维持授权/CAS；正文、用户 JSON、schema 属性名与合法同名业务字段不被误删 |
| R4 | 保留记忆/摘要优先与对话连续性 | 原始消息按实际源覆盖去重；整组覆盖才可省略、半组保留、当前输入一次；同来源不同事实不得丢失；不改成固定 10 句，无关后台任务不自动附带私聊 |
| R5 | 禁止用限制模型输入/输出解决提示词冗长 | 不新增、恢复或调整 prompt/context/response/output token 上限、section token 配额或按估算截断/拒绝；ordinary/stream/native/Writer 实际请求仍无 max_tokens/max_completion_tokens，大证据完整保留 |
| R6 | 规则可被后续 session 找到并自动核验 | AGENTS 与 backend index 可发现专用规范；清理与 R5 冲突的现行 spec；回归覆盖泄漏/去重/格式/源完整性与全 Agent 入口 |
| R7 | 交付真实当前组成及优化结果 | 文档列出每个 Agent 的协议、动态字段、记忆/摘要、历史数量条件、工具/图片；提供可复核的前后字符/条目对比与 Go 测试结果，区分静态/受控传输/live Provider 证据 |

## 已确定边界

- 使用当前分支 `codex/fluctlight-functional-recovery`，不创建分支。前端与其他 session 的任务记录不在本次修改范围。
- 用户于本轮明确确认现有历史覆盖机制是以前的优化成果，并允许有理由移除；本设计保留其价值，修复不严谨的去重，不新增固定聊天条数限制。
- 不修改模型配置、timeout、原生工具授权、事实权威、人格边界、目标完成标准、持久化事实或 DB schema。
- 真实图片、原生 tool transcript、必要输出 schema 和 Kev 外部 API envelope 保留其传输契约；不能为了外观一致将它们转换成普通文本。
- 不进行独立的记忆系统重写、模型切换或前端改造。真实模型输出风格/延迟不凭离线测试宣称通过。

## 规划交付状态

用户已批准方案并开始实施。代码、专用规范与完整 Agent 清单已完成，变更范围回归通过；验证边界和四项基线失败见 `evidence/verification-summary.json`。用户已确认提交，工作提交为 `2d82c94e73e823b8001f6cb9d1f3195cbbae9b66`；本任务随后按流程归档。
