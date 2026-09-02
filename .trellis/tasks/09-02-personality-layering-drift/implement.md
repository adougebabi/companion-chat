# Persona 分层与防漂移执行计划

## 执行原则

- 只修改 Persona 相关的 Go Core、Go BFF、浏览器 client 和现有 Vue 页面；不做整个项目重构。
- 先完成 schema/模型/应用边界，再接 provider、API/BFF 和页面；每一步都保留可运行的测试。
- 新实例是唯一验收对象；不编写旧实例数据迁移、旧 candidate 兼容或污染数据回放逻辑。
- 在 `task.py start` 前审核本计划；启动后由主会话按本清单直接实现和检查。

## 1. 规范与契约准备

- [x] 根据 `prd.md` 和 `design.md` 确认 `core_persona`、`developing_self_claims`、`current_state` 的最终字段名和 JSON shape。
- [x] 更新初始化、Reflection、ContextProjection、Foundation governance、Developing Self rollback/forget 的错误码和权限矩阵。
- [x] 确认 OpenAPI/生成 client 的实际来源，准备 Core/BFF/browser contract 同步变更。
- [x] 明确新实例验证方式：使用空/临时 PostgreSQL 数据库和新建 Fluctlight，不对现有污染实例做 reset。

## 2. 数据库与领域模型

- [x] 在 `apps/core-go/internal/migrations/runner.go` 增加 `core_persona` 列，并建立 `fluctlight_developing_self_claims` 与 `fluctlight_developing_self_revisions`。
- [x] 为 claim 的 confidence、status、revision、evidence/provenance 和 persona ownership 建立约束/索引；不增加对旧 `provenance.self_model` 的读写。
- [x] 扩展 `Fluctlight`、Detail DTO 和 Developing Self domain snapshot；保留现有 identity/personality/behavioral_policy/life_profile 投影字段供当前页面使用。
- [x] 新增 claim normalization/validation helper：category allowlist、证据归属、distinct evidence、confidence bounds、status/revision。
- [ ] 风险点：migration 版本和新表字段会影响所有 Core integration tests；回滚点为只回滚新增 migration/代码，不删除已有数据。

## 3. 新 Fluctlight 初始化

- [x] 修改 `initializationResponseSchema` 与 initialization prompt，使 provider 只返回 `core_persona`、`developing_self.claims`、goals/intentions。
- [x] 修改初始化校验和 `CreateFluctlight`：LLM-defined 写入 canonical Core Persona、owner-defined Developing Self seed 和中性 Current State。
- [x] 保留 blank-slate 分支，拒绝非空 layered foundation 输入，创建最小 Core Persona、空 claims 和默认 inner state。
- [x] 将 Core Persona 同事务投影到现有字段，确保现有详情/编辑组件仍能读取；不得让任何 Reflection 路径写这些投影。
- [x] 更新 `apps/gateway-go/internal/bff/routes.go`、browser client 类型和前端 activation body；保留当前分析→预览→激活交互。
- [x] 增加 initialization tests：分层 schema、seed provenance、blank-slate、缺字段/非法字段、创建事务原子性。

## 4. Reflection 与 Developing Self

- [x] 将 `reflectionResponseSchema` 和 Reflection prompt 改为 `developing_self_candidates`，移除 `personality_candidates`/`self_model_candidates`。
- [x] 新增 `applyDevelopingSelfCandidateTx`，只允许写 claim/revision 表；禁止更新 Core Persona、旧 personality 或 provenance self_model。
- [x] 在 `ProcessReflection` 中验证 evidence window、不同事实去重、category、confidence、重复 claim no-op、stale watermark/base revision。
- [x] 为 rejected、insufficient_evidence 和 accepted candidate 写 bounded revision/diagnostic 记录；watermark 与 candidate apply 共用事务。
- [x] 保留现有 memory/relationship/typed-slot candidate 流程，但将 typed slots 在 ContextProjection 中标记为 Developing Self 支持数据；不合并现有表。
- [x] 增加 reflection tests：新 claim、分类/证据校验、旧 key 不再进入 schema 和无效 candidate 拒绝。
- [x] 全仓搜索确认旧 `applyPersonalityCandidateTx`/`applySelfModelCandidateTx` 已移除且没有自动人格写入调用。

## 5. ContextProjection 与生成 guard

- [x] 扩展 `ContextProjection` 为 `CorePersona`、`DevelopingSelf`、`CurrentState` canonical 字段，并保留从 Core Persona 派生的旧 identity/personality/policy 视图。
- [x] `readInnerState` 补齐 `momentum`、`regulation`、`conflicts`；组合 `current_state` 时只使用当前快照和 resolved context，不复制长期人格。
- [x] 更新 chat、wake-up、daily review、media context、Reflection 和 realization 的分层 prompt/authority instruction。
- [x] 在 response plan/assessment 中增加 `core_alignment`、`state_expression` 的 bounded structured fields；不得允许 realization 返回 semantic side effects。
- [x] 修复 frozen action realization：重试时使用 assessment 冻结的 projection，不重新读取可能已经变化的 Persona。
- [x] 增加 projection/prompt tests：Core > Developing Self > Current State、当前状态可变、Core 不可被候选覆盖、frozen snapshot 重试。

## 6. API、BFF 与治理

- [x] Detail API 返回 `core_persona`、`developing_self`（claims + bounded revisions）和 `current_state`；敏感 provider/raw prompt/hidden reasoning 不跨 BFF。
- [x] Foundation routes 继续承载 Core Persona owner governance，并将 patch 集中映射到 canonical Core Persona；保留 CAS/revision 语义。
- [x] 新增 Developing Self list/detail、claim rollback、claim forget Core/BFF/browser client 方法和稳定错误码。
- [x] 同步 browser OpenAPI artifact/generated browser client。
- [x] 增加 HTTP/BFF tests：schema、ownership、CAS、error mapping、rollback/forget route coverage。

## 7. Web 页面最小增量

- [x] `InstancesView.vue` 保持两个创建入口，更新分析预览和 activation mapping 到新 `core_persona`/`developing_self` contract。
- [x] `InstanceDetailsDialog.vue` 保留现有字段形式，新增只读 Core Persona、Developing Self、Current State JSON 区块。
- [x] `GovernanceView.vue` 保留 Foundation 治理，新增 Developing Self revision/rollback/forget 的最小操作和 JSON 审计展示；Current State 不增加编辑控件。
- [x] 使用统一 JSON display helper，显式处理 null、array、object，避免 `[object Object]`；所有外部文本使用 Vue text binding。
- [x] 更新 Pinia/control-center 的 detail refresh 和 mutation reconciliation；不在浏览器推断 confidence/evidence/authority。
- [x] 增加 Vue/source tests，并按前端规范检查 dialog scroll、320/390/640px 和桌面布局。

## 8. 全局验证与质量门

- [x] 全仓搜索确认没有 Reflection 到 `fluctlights.personality` 或 `provenance.self_model` 的自动写入路径。
- [x] `gofmt`、Go unit tests、Core HTTP tests、migration/integration tests（无 PostgreSQL 时完成可运行的单元/HTTP覆盖）通过。
- [x] `pnpm --filter @fluctlight/web typecheck` 通过。
- [x] `pnpm --filter @fluctlight/web build` 通过。
- [x] 运行 workspace Web/client 测试并覆盖新建 contract、详情、治理和 JSON 展示 smoke（真实 provider/PostgreSQL 端到端待环境可用时执行）。
- [x] 静态检查确认 Core Persona 不因 Reflection candidate 变化；Developing Self 记录 evidence/confidence/provenance；Current State 读取完整字段。
- [x] 检查 Core/BFF 不泄露完整 prompt、hidden reasoning、provider credentials 或不受限原始数据库 JSON。
- [x] 已运行 Trellis 检查所需的 diff、spec 同步、lint/type/test 和跨层数据流检查。

## 回滚点

- 数据库迁移前：删除新代码即可，旧 schema 不受影响。
- 初始化 contract 完成后：可暂时关闭新建分层流程，但不要写入半成品 Core/claim 数据。
- Reflection 切换后：通过 feature/config gate 停止新 claim apply，保留已写 revision；禁止恢复旧 personality 自动写入。
- UI/API 完成后：可隐藏新增 JSON/rollback 区块，Core contract 和审计数据保持可读。

## `task.py start` 前检查

- [x] 用户已审核 `prd.md`、`design.md`、`implement.md`。
- [x] 所有产品 open questions 已解决；剩余内容仅为实现细节和代码内验证。
- [x] 已读取 backend/frontend 相关规范和 cross-layer thinking guide。
- [x] 未创建旧数据迁移或无关项目重构工作项。
