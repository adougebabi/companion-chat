# 实施计划：提示词构成重构

## 开始前门禁

- [x] 用户确认本任务规划中的 system/dynamic/TOON/YAML 边界。
- [x] 读取 `trellis-before-dev` 并按 backend spec 加载 provider/persona/structured-turn/media 规范。
- [x] 复核当前 prompt 相关调用方、formatter 和测试基线；提示词构成以外的生产改动保持冻结。
- [x] 明确 `media_prompt`、media quality、Visual Identity media path 不进入普通 composer。

## 1. 建立 prompt composition contracts

- [x] 新增固定 protocol 常量和 system composer，包含运行协议、operation rules、Core Persona。
- [x] 新增动态 section document 类型/渲染入口，确保 heading、字段 ownership、当前时间和当前 user input 一致。
- [x] 让 Provider boundary 继续保证单一首位 system，并移除依赖 prepend 调用顺序的隐式重复规则。

验证：pure composer tests；所有普通 role 的 system message 数量、顺序和内容断言。

## 2. Core Persona 与 metadata filtering

- [x] 从 canonical Core Persona 提取 identity/personality/behavioral_policy/life_profile 的语义字段。
- [x] 过滤 schema_version、实例/claim/message/goal IDs、revision、DB timestamps、FK、state-machine status、transport metadata。
- [x] 保留必要的 semantic fields；不把 update policy 的内部 decay controls 当成人格 prompt。
- [x] 将 metadata stripping 从全局 key denylist 调整为 scoped semantic allowlist/安全 fallback。
- [x] 证明 memory `created_at`、evidence_refs、Developing Self evidence refs 在模型输入中保留且不泄露 storage metadata。

验证：nested field matrix；legacy Core Persona reconstruction；metadata positive/negative tests；redaction tests。

## 3. 动态 context 与当前时间

- [x] 将 `context`、`life_context.current_time`、`timezone` 固定放入 `# 当前上下文`。
- [x] 将 Developing Self、Current State、memory、goals、intentions、recent messages、current user input 按 heading 分隔。
- [x] 保留 recent message 顺序并避免当前 user input 重复。
- [x] 保留 frozen decision 的原始 ContextProjection/replay 内容，只改变 Provider-facing serialization。

验证：Asia/Shanghai/UTC 时区 fixture；malformed instant fallback；current-time semantic test；frozen realization no reread regression。

## 4. Section-aware TOON/YAML

- [x] 为 memory、Developing Self、goals、intentions、recent messages 定义稳定 TOON field order。
- [x] 实现 header/cell delimiter escaping（comma、pipe、brackets、newline、quotes）和 YAML fallback。
- [x] nested objects/arrays、single-row、empty-row、heterogeneous-row、multimodal content 使用 YAML/native representation。
- [x] 避免 map iteration 顺序影响 prompt 输出；保持 deterministic rendering。
- [x] 将 media_prompt 的 YAML/no-Chinese 特例和 quality multimodal bypass 锁定为 negative tests。

验证：golden prompt fixtures；round-trip/parse safety tests；TOON delimiter adversarial fixtures；普通 role vs media role tests。

## 5. 迁移调用方

- [x] conversation cognitive assessment 使用新 composer。
- [x] action realization/streaming 使用新 system/dynamic boundary，不重复传输 context metadata。
- [x] wake-up、daily review、native cognition、reflection、schedule generation、initialization 逐一迁移或通过兼容适配层接入。
- [x] 保留 operation-specific rules、schema、tools、thinking、streaming 和 error behavior。
- [x] 保证 `media_prompt`、quality acceptance、Visual Identity 不被普通 composer 处理。

验证：每个 role 的 provider payload snapshots；schema/tools unchanged tests；stream terminal/error tests；media negative regression。

## 6. 最终质量门禁

- [x] Core `go test ./...`、`go vet ./...`、相关 targeted tests。
- [x] Browser OpenAPI/generated client 无无关变化；Browser Client typecheck/test。
- [x] Web typecheck/build/test 和 disposable Compose/public-boundary smoke。
- [x] 对照 Provider/Persona/Structured Turn spec 做跨层审查：system、dynamic context、tools、schema、diagnostic prompt 均无协议漂移。
- [x] evidence/time 与 metadata 的边界已通过正反向测试固定，不通过删除字段来规避问题。
- [x] 已明确下一步可单独评估 `media_prompt` 是否也需要新的 section/format 设计；本任务默认不改。
