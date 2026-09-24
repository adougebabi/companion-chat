# 实施与验收清单

单一任务，按依赖顺序执行；每组完成后核对正式入口，不以新增表、Tool 或 Prompt 字符串单独结项。主线程负责领域取舍、跨组整合、spec 更新和最终证据；实施/检查阶段遵循 Trellis 的 Agent 接线与全量质量门禁。

## 0. 基线与证据

- [ ] 逐个点验 `research/baseline.md` 的高风险 file:line 锚点及最新正式路径；记录 git base、迁移 head、现有测试退出码、Provider/DB/ComfyUI 环境是否可用。
- [ ] 取 Main/WakeUp 初始请求与工具续接的脱敏最终 Eino/Provider wire，记录各 section 的字符、字节、Token、工具和 schema 开销及调用次数。
- [ ] 把能力接线表、初始化语义去向表的“是否生效”用受控和真实请求分别核验，不能用模型没说到某词证明未接入。

## 1. 当前有效自我

- [ ] 修复初始化 schema/normalization/coverage：`life_habits`，可变外观/当前穿着/持有物品和来源语义，数组/对象及 clear/unknown。
- [ ] 改 portrait source allowlist、编译校验/repair/fallback、规则版本与 CAS；保留稳定偏好/例外，排除当前外观/穿着，画像失败保留一致版并显式报错。
- [ ] 加入 profile-scoped habit revision 与受控决定操作，更新有效详情与画像；单次状态变化不得重编译。测试短习惯、profile/shared、清空与跨人格切换。
- [ ] 修复 `SlotAppearance` 指向当前 shared appearance authority，删除身份 JSON 旧读路径。

## 2. 衣柜与身体领域

- [ ] 新 migration + clean-start schema：shared appearance/revision、wardrobe items/sources、outfit references、current worn slots、必要的 activity run；唯一约束、FK、CAS、历史/事件 provenance。
- [ ] 一次性初始化映射、幂等 key、未知所有权、当前穿着引用；重复 activation/backfill 不增副本。
- [ ] 实现 bounded query、完整性标记、可用性、outfit CRUD、partial/full wear、失去/不可用清理；业务操作和 Tool 共用一套实现。
- [ ] 实现临时发型与有效身体事件结果；发长/发色、伤势由有来源的持续结果更新，伤势进入活动 preflight；场景/日程/午夜不重置。

## 3. 意愿与虚拟活动

- [ ] 复用 Goal/Intention 当前表及 revision：管理 Tool、同目标复用/新建、修改/暂停/取消/读取、关联计划/活动/结果；不靠类别机械去重。
- [ ] 贯通 `intention.due` refs → settlement → ActionOutcome → attempt；补正式 Agent E2E，证明失败/延后不完成。
- [ ] 实现 shopping/haircut activity start、持续推进、结构化虚拟结果任务、校验及原子结果应用；completed shopping 入柜而不穿上，completed haircut 更新 shared 身体，失败保留意愿。
- [ ] 复查授权、operation ID、不同主体/重复事件/并发/CAS/事务；外部真实消息或媒体仍按原授权结算。

## 4. 日程、WakeUp、Main

- [ ] 初始 Schedule 输入有效约束、未完成事项、当前状态和近期结果；完善任务说明，允许空闲、合理重复，避免身份模板。重排只动 current/future。
- [ ] 让活动结果、due intention 和相关有效状态事件进入已有 WakeUp/native cognition；无新事实可 no-op，内部写回不自触发无限循环。
- [ ] 注册并按 surface 授权全部新/改 Tool；在 `ExecuteTool` 加 surface 再校验；最小合法上下文下独立调用成功/拒绝/冲突/延后/失败/重试。
- [ ] 用正式 `handleTurn`/`ProcessWakeUp` 验证 Eino ToolCall→真实 ExecuteTool→ToolResult→模型续接与 receipt，不用测试旁路 Main。

## 5. Prompt 与下游消费者

- [ ] 更新 ContextProjection、surface compactor、Prompt Composer/Assembler；当前身体/穿着/必要持续事项在 Runtime，习惯在有效画像，当前输入一次，旧值不重新注入。
- [ ] 完整最终请求预算：required 超限明确失败，optional 整片/整轮裁剪，Tool/schema/续接累计计费与 Trace 来源/排除原因。
- [ ] `persona.detail` 默认当前有效且历史标时；会话摘要保持覆盖范围和失败回退，加入意愿/计划/结果/身体时间语义回归。
- [ ] 媒体 concept 使用 frozen shared appearance、worn item、canonical identity reference 和版本/拍摄时间；生成后状态变动时不误标最新。验证参数，再单独验证视觉效果。

## 6. 存量处理与冲突路径清理

- [ ] 新 migration 的 0035 upgrade 和 empty→head 测试；preview/apply 命令限定 Owner/ID、幂等重跑、失败恢复与 unknown 诊断，付费编译显式授权。
- [ ] 在可丢弃 PostgreSQL 执行 preview→apply→rerun，并检查旧画像、已有最新外观/生活状态/习惯、初始衣物、未完成意愿/活动。
- [ ] 删除或替换已定位的旧回填、场景自动换装、日程覆盖身体、原始全文重复注入、旧查询默认当前和重复意愿写入路径；更新相互矛盾的 backend specs。

## 7. 验收证据与质量门禁

- [ ] 确定性领域/正式入口/独立 Tool PostgreSQL E2E 全部通过，逐项勾选 PRD A1–A11；零匹配与意外 SKIP 算未通过。
- [ ] 脚本化 Provider 验协议与结算；真实 Provider 多人格多样本评估靴子、剪发、习惯、情绪、生活连续性和身体事件，留模型/输入/调用/失败例。缺环境记 BLOCKED。
- [ ] ComfyUI 有条件时检验图像模型真的遵循新发型/穿着；参数正确与视觉正确分别记录。
- [ ] 比较改造前后最终 Provider 首轮与 Tool 续接累计字符/字节/Token/调用数；一次性迁移/初始化成本单列。
- [ ] 执行 `go -C apps/core-go test -race ./...`、`vet ./...`、`build ./...`、`pnpm generate`、`pnpm typecheck`、`pnpm test`、`pnpm build` 及受影响 Compose/迁移 gate，记录每条真实退出码。
- [ ] Trellis check 全范围复核、更新相关 spec、按项目 commit 流程提交，完成证据文件和任务收尾。

## 高风险文件与回滚点

- `internal/core/persona_compilation.go`、`working_persona_persistence.go`：编译发布必须保留旧一致版本；新规则启用前 backfill/readiness 全通过。
- `internal/migrations/runner.go` + 新 migration：released-head 升级与 clean start 同步；回滚应用不删除已写历史。
- `internal/core/tool_execution.go`、`agent_result_adapter.go`、`action_outcome.go`：权限、receipt、Tool 副作用和 Intention 结算在短事务内，不整轮重放。
- `internal/core/provider_context.go`、`prompt_context_assembler.go`、`media.go`：最终 wire/预算/图像冻结结果需直接检查；诊断不能泄露私密档案。
- 环境不足时只执行可丢弃本地验证，不对生产库自动 apply 或批量付费调用。
