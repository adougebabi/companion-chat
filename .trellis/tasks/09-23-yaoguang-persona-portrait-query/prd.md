# 摇光自我画像编译与人格详情查询闭环

## Goal

完整人格仍为唯一权威来源。为每个可发言 profile 保存可追溯、版本匹配的精简 Working Persona，在正式认知请求中使用；需要细节时由模型通过只读 Tool 查询真实完整资料并继续生成。

## Confirmed repository facts

- 初始化 `AnalyzeDescription` → `RunInitializationTask` → `prepareInitializationResponse` → `CreateFluctlight`，完整来源存于 `fluctlight_initialization_sources`，权威人格存于 `fluctlights.core_persona` 和 foundation revisions。
- `working_persona.go:75` 已有确定性运行时投影；`provider_context.go:111` 用于正式 System 人格。它没有编译 Agent 或持久化产物。
- `cognition_agent.go:42` 与 `agent_result_adapter.go:574` 分别为正式 Main、WakeUp；接管通过同一 Main Eino Loop 的 `persona.takeover` 继续。
- `builtin_capabilities.go:76` 的注册表无人格详情查询 Tool；内部 `FluctlightDetail` HTTP 查询不是 Agent Tool。`memory.recall` 查询运行时记忆。

## Requirements

1. 复用 Working Persona 为唯一画像产物，按共享身份、profile、正式稳定覆盖层编译；保留身份、机制、表达、行为边界、稳定偏好及条件例外，排除动态状态，并保存来源引用和诊断。
2. 产物与人格来源 revision/hash、覆盖层 revision、规则版本绑定。新建、正式更新和规则升级触发更新；普通对话与动态状态变化不触发。失败不发布不一致组合。
3. 新建人格的每个可激活 profile 在激活前具有效画像。提供有预览、幂等、失败报告的存量补编译入口。
4. Main、WakeUp 和实际接管后的模型请求使用本轮发言 profile 的画像；完整资料只在受控编译、治理和查询路径读取；缺失或版本冲突显式失败。
5. 实现正式注册的只读人格资料 Tool，提供 `list` 与有界 `read`、继续位置、来源版本和权限隔离。独立调用与 Agent Loop 共用实现；模型查询结果必须回填并续接。
6. 记录真实请求预算，区分运行协议、画像、动态状态、历史、工具、查询结果及续接；字符、字节与实际或标明估算方法的 Token 分开。
7. 清理被替代的旧投影和重复说明，保留完整来源与既有机器契约。

## Acceptance criteria

- 实际调用链完整文本 → JSON → 编译并持久化 → 正式请求 → 真实 Tool 查询 → 同一 Agent 续接；保留可审计 Trace。
- 编译和回放覆盖不同人格、咖啡偏好及例外、动态状态排除、来源更新竞态、失败不发布、多 profile、幂等补编译。
- 正式请求测试断言单份画像、无完整人格旁路、正确发言 profile；独立 Tool 测试验证 list/read、分页、无记录、非法 section、权限与版本。
- Provider 真实 E2E 与确定性协议测试分别报告；缺凭据时标为 BLOCKED，不将 SKIP/Mock 充作真实通过。
- 提交接线表、前后样例、预算对比、测试命令与结果、旧路径清理和剩余阻塞。

## Out of scope

全局 YAML/TOON 迁移、新决策模型、新向量库、每轮筛选 Agent、重写 Agent/Tool/事务/日程/人格切换系统，以及整体运行协议重写。
