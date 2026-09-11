# Capability Runtime 实现整改说明

状态：implementation remediation required。本文供实现 session 直接接手修复当前未提交实现；它不替代 `prd.md`、`design.md` 或 `implement.md`，只描述本次审查发现的偏移、整改终态、执行顺序和复验门槛。

## 1. 接手位置与权威输入

- Worktree：`/private/tmp/local-ai-companion-llm-capability-runtime`
- Branch：`codex/llm-capability-runtime`
- Fixed point：`master` / `6b915bebf2516d27a89b3af834858ab0325895af`
- 当前实现：全部未提交；不要丢弃或覆盖现有修改，不要在 `master` 工作区修复。
- Trellis task：`.trellis/tasks/09-09-llm-capability-runtime`
- Task status：`in_progress`

实现前必须依次阅读：

1. `prd.md`
2. `design.md`
3. `implement.md`
4. `research/current-tool-inventory.md`
5. `research/prompt-runtime-baseline.md`
6. `research/spec-constraints.md`
7. 本文
8. `.trellis/spec/backend/index.md` 及其中与本任务相关的规范

## 2. 审查结论

当前实现不满足最终设计，不能作为完成版本提交或合并。工程门禁虽然通过，但本质上仍是：

```text
CapabilityDefinition / CapabilityContext facade
                   ↓
          legacyCapabilityAdapter
                   ↓
CapabilityExecutor + CapabilityManifest + ToolCallV1
```

即“新外壳 + 旧执行链”，与任务要求的一次性全量 Cutover 相反。

同时，多个 Thin Schema 已经从 Provider catalog 删除旧参数，但旧 executor 仍把这些参数视为必填，造成“对外合法、执行必败”；`schedule.replan` 更是没有任何 intent 成功路径。迁移 SQL 也没有真正把 v1 payload 转成 v2 Invocation。

整改目标不是给 adapter 增加更多特殊分支，而是完成下面的唯一执行模型：

```text
Provider native/root-sidecar codec
  -> []CapabilityInvocation
  -> CapabilityRegistry.Lookup / Catalog
  -> ContextResolver.Resolve(RequiredContext)
  -> Capability.Execute
  -> CapabilityResult
  -> generic deferred target settlement / persistence
```

## 3. 不允许妥协的终态

完成整改后必须同时成立：

1. 11 个内建能力全部直接实现 `Capability`；不再通过 `CapabilityExecutor` 注册。
2. Registry 只保存 Capability/Definition，不保留 legacy executor map 或双向 adapter。
3. Provider API 直接接收 `[]CapabilityDefinition`，经 `RenderCapabilityTools` 渲染；不经过 Manifest → Definition 往返。
4. Provider native 和唯一 root sidecar 直接归一化为 `[]CapabilityInvocation`。
5. Main flow、Runtime、replay、workflow、deferred settlement 不再把 Invocation 降级成 `ToolCallV1`。
6. 每个内建 Capability 声明真实 `RequiredContext`，并实际消费 `CapabilityContext`；不能 resolve 后丢弃。
7. Image/Schedule 的内部规划在 Capability 内完成；Main conversation/WakeUp/autonomy 不编译具体业务参数。
8. active v1 payload 通过一次性迁移转换为合法 v2；Runtime 不保留 v1-first/v2-fallback 双读取。
9. 必需能力失败时，在对用户发出可见内容前 fail closed；normal 与 crash recovery 使用同一结算语义。
10. 删除旧 registry、old schema、adapter、compat executor、具体名称分发、legacy media/reply 兼容和死测试 fixture 支持代码。
11. 普通 conversation 保持一次 Main LLM cognition，不新增同轮 `role=tool` continuation。
12. 不引入 MCP、Discovery、Event Bus、新 DI 框架或其他超出范围的系统。

只要 `legacyCapabilityAdapter`、`CapabilityExecutor` 生产执行、`CatalogManifests`、v1-first replay 或 schedule planner missing 任一仍存在，就不能宣称 Cutover 完成。

## 4. 修复批次与依赖顺序

以下批次必须按顺序执行。后面的批次依赖前面的结构，不要并行做互相矛盾的补丁。

### R0. 冻结当前证据并添加失败回归

目的：先让当前已确认缺陷成为会失败的测试，避免边改边丢问题。

- 保留当前 schema bytes 基线：pre 7,857 / post prototype 4,252 bytes。
- 将审查探针转成正式测试：逐项验证 Provider-facing 最小合法输入能进入 Capability 的成功/明确业务边界。
- 新增当前应失败的测试：
  - 11 个生产项必须是 direct `Capability`，Registry 不含 legacy executor。
  - `schedule.replan({intent})` 在 configured fake planner 下成功。
  - v1 SQL fixture 转换后能反序列化成非空 `call_id/capability_name`。
  - `CapabilityInvocation` replay 保留 Surface、ActionID、ContextSnapshot。
  - required deferred failure 发生时不调用 visible callback、不留下 assistant row。
  - production MainAgent/Runtime source 不含具体内建 capability-name dispatch。

不要保留 `TestScheduleIntentWithoutPlannerFailsClosed` 作为最终正向验收；planner 未配置的失败只能是负面用例，不能替代成功路径。

### R1. 校正 11 项公开契约

Thin 的含义是“不暴露实现细节”，不是“删掉 executor 需要的所有语义字段”。Runtime 只能补充机械可确定的 provenance，不能猜模型置信度、记忆类型、场景操作或日程内容。

目标 Provider-facing 输入：

| Capability | Required model-owned input | Optional model-owned input | Runtime / Context owned |
|---|---|---|---|
| `conversation.reply` | `text` | — | output target、call/provider IDs |
| `moment.publish` | `text` | — | Moment target、call/provider IDs |
| `media.image.generate` | `intent` | — | visual identity、life context、appearance、state、concept、renderer/workflow/provider IDs |
| `visual_identity.initialize` | — | — | core persona、current visual identity、WakeUp source |
| `scene_event` | `operation`, `confidence`; start/switch 时 `scene`, `activity` | `location` | evidence、source fact、clock、idempotency、current scene/schedule |
| `presence_event` | 至少一个 `user_presence/current_task`，以及 `confidence` | `expires_at` | actor/source/evidence/idempotency、current life |
| `schedule.replan` | `intent` | — | full current schedule/life/agency、timezone、revision、completed boundary、replacement DTO、evidence/idempotency |
| `memory_event` | `content`, `type`, `confidence`, `importance` | `emotional_significance` | owner/visibility、source/evidence、conversation/actor/event refs、profile scope、idempotency/embedding |
| `affect_event` | `event.type`, `event.confidence` | — | evidence/idempotency、current state、numeric reducer/CAS |
| `relationship.lookup` | `target_actor_id` | — | owner/conversation authorization、active profile、live relationship |
| `capability.request` | `capability_key`, `title`, `description`, `rationale` | `desired_contract`, `priority` | source/evidence/idempotency、review persistence；缺少 `desired_contract` 时允许 `{}` |

如果实现 session 认为某个 required semantic field 也应下沉 Planner，必须先同步修改 `design.md` 并说明 Planner 的输入、输出和失败语义；不得静默加默认值。

必须删除或停止维护每个能力的第二份 `Parameters` thick schema。每项只有一个 `CapabilityDefinition.InputSchema` 是 Provider 权威定义。

### R2. 完成 direct Capability Cutover

建议结构：

```go
type Capability interface {
    Definition() CapabilityDefinition
    RequiredContext() []ContextSlot
    Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}
```

可选接口只用于真实差异：

```go
type CapabilityPreflighter interface {
    Preflight(context.Context, CapabilityContext) error
}

type DeferredCapability interface {
    Capability
    ExecuteDeferredTx(
        context.Context,
        pgx.Tx,
        CapabilityInvocation,
        CapabilityContext,
        OutputBindingV1,
    ) (CapabilityResult, error)
}
```

实施要求：

- 将 `conversationReplyCapabilityExecutor` 等 11 项逐个改为 direct Capability。
- Capability 自己持有必要依赖，例如 domain service/provider/preflight dependency；不要持有整个 App 只为继续 service-locator 模式。
- Registry constructor 返回确定错误；nil、invalid、duplicate 不能静默注册。
- 删除：
  - `CapabilityExecutor`
  - `legacyCapabilityAdapter`
  - `registryCapabilityAdapter`
  - executor map
  - `CatalogManifests`
  - `CapabilityManifest.Parameters`
  - `ToolCallPayload`
  - `ExternalCapabilityManifests`
  - `capabilityManifestsExcept`
  - `optionalToolFailureNonFatal`
- `ProviderClient.StructuredWithTools*` 参数改为 `[]CapabilityDefinition`。
- Runtime 直接处理 `CapabilityInvocation/CapabilityResult`，不再调用 legacy Tool executor。

### R3. 让 ContextResolver 成为真实边界

当前实现的问题是 resolver 会加载数据，但 legacy adapter 立即丢弃它；同时 nil/unknown slot 会伪造空 Context。

整改要求：

- `NewCapabilityRuntime` 要求非 nil resolver，并返回错误或由 composition root 保证；生产不得默认 `EmptyContextResolver`。
- `EmptyContextResolver` 仅允许放在 `_test.go` fixture；生产代码删除。
- App/DB 缺失、未知 Slot、loader 缺失或类型错误都返回 `ErrContextResolve`。
- `slots == []` 时不查 owner/DB。
- 单次 Resolve 内复用已读取的 Fluctlight、schedule、inner state、life context 等，防止 `SlotSchedule + SlotCurrentLife` 重复读。
- 每个 Slot 使用明确的 value type；允许 value object 内继续封装现有领域 map，但不能把公开 API 退化成 `map[ContextSlot]any`。
- Capability 必须读取 resolved context；若只为 authorization/CAS 需要 live value，也要明确区分 frozen decision context 和 live execution guard。
- Image Definition 补齐 `visual_identity/current_life/appearance/current_state`。
- 所有 context tests 必须覆盖：only requested、dedupe、missing、unknown、type mismatch、cancel、snapshot round-trip。

### R4. 把 Image prepare/compilation 移入 Capability

当前 `bindMediaContextToToolCalls` 在 conversation/WakeUp/autonomy 中把 `{intent}` 改成内部 `concept`，仍然是 Main flow 了解图片实现。

目标：Main flow 只看 Invocation 和 Definition metadata。

可选实现方式：

- 在通用 Runtime 中增加一个 metadata-driven prepare 阶段；只有实现 prepare 接口的 Capability 会被调用。
- Image prepare 在事务外完成：
  - validate intent；
  - consume four required context slots；
  - 构造并冻结 internal visual concept/context binding；
  - 返回 prepared invocation/result。
- prepared payload 与 canonical Invocation 一起持久化。
- deferred transaction 只绑定 conversation/Moment/WakeUp target 并创建 durable media intent；不得在事务中做 Provider/ComfyUI/MinIO I/O。
- Worker 后续继续使用 frozen concept 和现有 stable provider/workflow IDs。

完成后删除 Main flow 中所有 `bindMediaContextToToolCalls` 调用和对应的 capability-detection 逻辑。

### R5. 实现 Schedule capability-local planner

这是当前功能性 P0，不能留作 follow-up。

Planner 输入：

- thin `intent`
- resolved current schedule
- current life context
- agency/goals/intentions
- source fact、当前时间与 canonical timezone

Planner 输出为内部严格 DTO：

- `local_date`
- `timezone`
- `expected_revision`
- `completed_before`
- full replacement `items`
- `reschedule_policy`
- bounded reason/evidence

要求：

- 使用当前已配置的合适 Provider role，不引入新 Provider 系统或 MainAgent continuation。
- planner schema 只在 Capability 内部存在，不进入 Main LLM catalog。
- Provider 调用发生在 PostgreSQL transaction 外。
- planner output 必须通过现有 item、全天连续性、completed-history、timezone、revision/CAS 和 idempotency 校验。
- planner error 返回 `schedule_replan_planner_failed`，不回退旧 thick args、不生成 heuristic schedule。
- fake planner 单元/集成测试至少有 success、invalid output、provider error、CAS conflict、completed history preservation。

### R6. 建立唯一 canonical persistence/replay

当前同时写/读 `decision.tool_calls`、root `capability_invocations` 和旧 ToolResults；v2 又会降级为 ToolCall，ContextSnapshot 因此丢失。

整改要求：

- Provider codec 之后立刻得到 `[]CapabilityInvocation`。
- freeze 前填充：CallID、CapabilityName、SourceFactID、ProviderRequestID、Sequence、Surface、Fluctlight/Conversation、ActionID、ContextSnapshot、prepared payload。
- 持久化只保留一份权威 `capability_invocations` 和 `capability_results`。
- `capabilityInvocationsFromValue` 返回 `[]CapabilityInvocation`，不能返回 `[]ToolCallV1`。
- Runtime/replay/workflow/deferred settle 直接接收 Invocation。
- 删除 v1-first/v2-fallback 读取。
- 不允许先写 root Invocation、之后只更新旧 `decision.tool_calls` 的 ActionID；canonical object 必须原子更新。

一次性数据迁移：

- 对 active `cognition_frozen_actions` 和 `autonomy_actions` 逐元素映射：
  - `id -> call_id`
  - `name -> capability_name`
  - 保留 arguments/source/action/provider/sequence
  - 构建合法 metadata/surface
  - 将旧 projection 转成 bounded Slot snapshot
- migration 开始前检测 malformed active payload；发现一条即失败并阻止 cutover，不能 `WHERE` 静默跳过。
- completed/failed audit rows不重写、不重新执行。
- active Temporal histories 必须通过 saved-history replay 或部署前 drain gate；不能靠永久 runtime compat。
- migration 完成并验证后删除 `MigrateCapabilityPayload` 运行时 helper 与所有 v1 codec fallback。线性 migration SQL 可以保留为历史，但不属于运行时双轨。

### R7. 统一 fail-closed 可见结算

当前正常 transaction 内有 required failure check，但可见 callback 在它之前执行；crash recovery 又遗漏 check。

整改顺序必须是：

```text
execute/prepare immediate calls
  -> persist provisional results
  -> begin output transaction
  -> create/bind output
  -> settle deferred calls
  -> apply generic required failure policy
  -> commit
  -> emit visible callback/frame
  -> complete cognition
```

要求：

- `callbacks.onChunk(visible)` 移到 transaction 成功之后。
- normal 与 recovery 共用同一个 settle + required-policy helper。
- required failure 回滚新 assistant/Moment，不向客户端输出成功文本。
- optional internal failure保留结构化失败结果，但不伪造成功。
- 对旧数据中“assistant 已存在、required result 失败”的 crash window 定义显式 compensation/quarantine；不得无条件 `CompleteTurnCognition`。
- Image 的成功边界是 durable media intent 创建成功，不是最终 asset ready；回复不能声称最终图片已完成。

### R8. 删除 compat/name branches 和更新文档

必须删除或迁移：

- `normalizeConversationReplyCalls`
- `moment_media_request -> media.image.generate`
- `legacy-media` intent/workflow/request 创建路径
- WakeUp 的 concrete-name fallback switch
- output preference 的 image-name fallback
- 仅为了旧测试保留在生产 package 的 fixture compatibility helpers
- 旧 `conversationAssessmentInstruction` / `wakeUpAssessmentInstruction` / `dailyReviewInstruction` 及其厚 prompt tests

Reply/action 判断使用 Definition metadata：

- output target kind
- deferred/output classification
- result status
- failure policy

不能按具体名字判断，也不能把任何带 `text` 参数的 Capability 都误当 conversation reply。

文档同步：

- `docs/capability-architecture.md` 删除“schedule planner remaining follow-up”和“old fields retained until drain”之类未完成表述；文档只描述最终真实代码。
- `README.md` 将新增能力步骤从 `CapabilityExecutor/CapabilityManifest` 更新为 direct `Capability/Definition/RequiredContext/registration`。
- `.trellis/spec/backend/structured-turn-contract.md` 必须与代码一致；不能先写“唯一 Runtime 已完成”再用规范掩盖实际双轨。
- `docs/capability-schema-report.md` 重新记录最终 renderer 的 exact bytes/chars。

### R9. 最终测试与架构证明

必须补齐：

#### Registry / Definition

- direct Capability register/lookup/duplicate/nil/invalid/stable order/surface
- Registry 内不存在 legacy executor state
- Definition schema 与 Capability argument validator 对齐

#### End-to-end dummy

一个测试完整贯穿：

```text
implement dummy Capability
  -> register
  -> Catalog(surface)
  -> RenderCapabilityTools
  -> normalize fake native/root-sidecar response
  -> resolve declared context
  -> Runtime.Execute
  -> CapabilityResult
```

测试期间不得修改 MainAgent、Provider schema switch 或 Runtime name switch。

#### Thin schemas

- 11 项逐一最小合法输入测试
- executor/runtime 接受最小合法输入或进入配置好的 planner success path
- forbidden fields 不出现在 Provider catalog
- schema bytes/chars 与重构前比较

#### Context / replay

- only-requested/dedupe/missing/unknown/cancel/type mismatch
- frozen snapshot round-trip
- active v1 → v2 real PostgreSQL migration
- malformed active fail closed
- completed audit untouched
- stable call/provider/workflow/idempotency IDs

#### Failure / recovery

- required immediate failure
- required deferred failure
- optional internal failure
- crash after external success/before local result
- crash after assistant transaction/before cognition completion
- no duplicate media/provider job
- no visible callback before required success

#### Behavior

- production Prompt 下普通聊天不强制 Capability
- 图片请求产生 `{intent}` image invocation
- 场景改变产生并真实执行 scene Capability
- Moment publish 使用 output Capability
- 普通 conversation 恰好一次 Main LLM cognition
- internal planner 不生成第二份 MainAgent 回复

真实 Provider 测试只作为 opt-in 补充；确定性 fake Provider 测试必须存在。

#### Architecture guard

生产 Go 文件中禁止：

- `legacyCapabilityAdapter`
- `CapabilityExecutor`
- `CapabilityManifest.Parameters`
- `ToolCallPayload`
- `capabilityManifestsExcept`
- concrete built-in capability-name dispatch
- v1-first replay fallback
- old prompt constants / legacy media conversion

允许具体稳定名称出现的位置仅限各 Capability Definition、自身业务测试和一次性迁移映射；架构 guard 应使用明确 allowlist。

## 5. 已验证基线与环境缺口

审查时以下检查通过：

```text
Core go test ./...                         PASS
Core go test -race ./...                   PASS
Core go vet ./...                          PASS
Core go build ./...                        PASS
Gateway go test ./...                      PASS
Gateway go test -race ./...                PASS
Gateway go vet ./...                       PASS
Gateway go build ./...                     PASS
gofmt -l                                   PASS
git diff --check master                    PASS
```

当前测试绿灯不能视为验收通过，因为：

- migration tests 只检查 SQL 字符串；
- schedule test 把 planner 缺失当成预期；
- production Thin Schema 与 executor 的错位未测试；
- live provider test 使用旧 prompt；
- dummy tests 没有贯穿 provider renderer/runtime；
- required deferred failure/recovery 缺少集成覆盖。

审查环境未配置：

- `GO_CORE_TEST_DATABASE_URL`
- `FLUCTLIGHT_LIVE_PROVIDER_TEST`
- `TEMPORAL_ADDRESS`

因此最终完成前还需要真实 PostgreSQL migration/replay 和 Temporal history/drain 证明。没有环境时必须在报告中标为未验证，不能把 unit test 当替代证据。

## 6. 完成判定

只有以下命令与证明全部成立，才可以请求复验：

```bash
GOCACHE=/tmp/lac-capability-remediation-test go -C apps/core-go test ./... -count=1
GOCACHE=/tmp/lac-capability-remediation-race go -C apps/core-go test -race ./... -count=1
GOCACHE=/tmp/lac-capability-remediation-vet go -C apps/core-go vet ./...
GOCACHE=/tmp/lac-capability-remediation-build go -C apps/core-go build ./...
GOCACHE=/tmp/lac-capability-remediation-gateway-test go -C apps/gateway-go test -race ./... -count=1
GOCACHE=/tmp/lac-capability-remediation-gateway-vet go -C apps/gateway-go vet ./...
GOCACHE=/tmp/lac-capability-remediation-gateway-build go -C apps/gateway-go build ./...
test -z "$(find apps/core-go apps/gateway-go -name '*.go' -print0 | xargs -0 gofmt -l)"
git diff --check master
```

并提供：

- 11 项最终 Capability 表：Name / Type / Thin Args / Required Context / Direct Implementation。
- legacy/compat/old registry/old schema/old prompt 的删除清单。
- 架构 guard 结果。
- pre/post schema bytes/chars。
- real PostgreSQL migration/replay 结果。
- normal + crash recovery 的 fail-closed 结果。
- 未运行或跳过的环境测试清单。

## 7. 建议技能

实现 session 建议按以下顺序使用：

1. `trellis-before-dev`：重新加载 task artifacts 与 backend specs 后再编辑。
2. `codebase-design`：收敛 Registry/Runtime/Context/Deferred prepare 深模块接口，避免继续叠 adapter。
3. `tdd`：先加入 R0 中的失败回归，再逐批修到通过。
4. `trellis-check`：每个大批次后局部检查，最终做一次全 scope 质量门禁。
5. `trellis-update-spec`：仅在最终代码真实满足单一 Runtime 后更新规范；不要用规范文字美化未完成实现。
6. `trellis-break-loop`：修复结束后复盘这次“为了渐进迁移留下双轨、测试固化未完成功能”的漂移原因。

## 8. 给实现 session 的直接指令

不要在现有 facade 上继续补更多兼容。先写失败测试，然后从 Registry/Provider/Invocation 主干开始把 11 个内建能力迁成 direct Capability；再完成 Context、Image prepare、Schedule planner、canonical persistence、fail-closed 和 cleanup。每完成一批都应删除对应旧链，而不是等最后统一清理。

若修复过程中发现必须改变已评审的产品语义，停止修改并更新 task artifact 请求用户评审；不要自行扩大范围，也不要通过默认值、关键词推断或旧 thick schema 绕过问题。
