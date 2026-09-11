# Capability Runtime R0–R9 复验证据

> **状态说明（2026-09-10）**：第二轮P0/P1/P2整改已经完成源码、确定性测试和disposable
> 真实环境复验。用户已确认该阶段可以收口并继续Project Health/Evolution，因此它现在是
> `accepted_as_prerequisite`。为满足最后统一验收的要求，本任务目录和未提交改动仍保留；
> 不提交、不合并、不推送。权威问题与完成判据见`second-review-remediation.md`。

## 2026-09-10 第三轮整改结果（用户已确认前置门禁）

| 第二轮问题 | 当前代码/确定性测试 | 真实环境状态 |
| --- | --- | --- |
| Scene/Presence schema自相矛盾 | 已修复；Definition自洽、oneOf语义和最小输入测试通过 | 不需要外部环境 |
| strict Input/Output schema | 已实现additionalProperties、nested type、enum、pattern、bounds、anyOf/oneOf；测试通过 | 不需要外部环境 |
| Arguments/PreparedPayload混用 | 已物理分离并增加prepared envelope版本、smuggling/corrupt replay测试 | `0025 -> 0026` active row真实转换通过 |
| Image intent/current context丢失 | thin intent到mediaPromptInput端到端fixture通过 | live Provider未配置；确定性完整数据流已验证 |
| v1->v2 migration/replay | Head=`0026_capability_runtime`；11项thin映射、legacy result、bounded Slot snapshot、running/version/postflight/workflow authority和completed-call skip测试通过 | 空库、active valid、malformed rollback和bounded scene snapshot均在pgvector/PostgreSQL 16通过 |
| Schedule prepare/idempotency | prepare-before-execute、prepared replay、stable key/request digest与schema guard通过 | 真实PostgreSQL相同payload replay及不同payload conflict通过 |
| Memory/Affect分裂事务 | transaction-aware Capability、pgx savepoint与interactive settlement通过 | 真实PostgreSQL事务内可见/outer rollback后无Memory/Affect event通过 |
| ContextResolver边界 | wrapper/Slot mismatch拒绝、Relationship授权集与rows分离/空集拒绝、Memory owner/Visual Persona消费、窄Capability依赖测试通过 | 全部Core DB opt-in测试通过 |
| Memory schema元数据 | profile/evidence/provenance/idempotency等已从Provider schema删除 | 不需要外部环境 |
| Prepare error code | typed `CapabilityError`保留`schedule_replan_planner_failed`并通过batch/runtime测试 | 不需要外部环境 |
| OutputSchema与日志 | 完整Output校验通过；普通日志只保留intent presence/length/digest | 不需要外部环境 |

当前schema统计：all 11=`4,864 B/C`，conversation=`4,326 B/C`，native cognition=
`3,734 B/C`。最终Core/Gateway普通测试、race、vet、build、gofmt、diff-check全部通过。

真实环境使用两个PID/名称隔离的disposable Compose项目：完整平台smoke验证PostgreSQL、
Redis、Temporal、MinIO、migration、cutover、Core、Worker、BFF、Web全部健康；定向环境验证
`0025 -> 0026` valid active转换、malformed P0001回滚、bounded Slot snapshot、Schedule数据库
幂等、Memory/Affect caller-owned rollback，以及Core/platform所有DB opt-in测试。两套项目的
容器、网络、volumes和`/tmp`测试env均已删除。

未运行项：`FLUCTLIGHT_LIVE_PROVIDER_TEST`，因为没有配置真实模型endpoint；它是概率性contract
补充，不替代已经通过的fake Provider确定性测试。没有提供既存生产Temporal history，因此
无法声称重放了用户生产history；仓库的显式legacy fence/cutover、Worker Deployment version和
fresh Temporal启动路径已在disposable stack通过。

本记录对应 `/private/tmp/local-ai-companion-llm-capability-runtime` 的
`codex/llm-capability-runtime` 分支；所有修改仍未提交。它记录本地可重复
的实现证据以及当前环境无法提供的真实基础设施证据，不把跳过的集成测试
伪装成通过。

## 最终 11 项 direct Capability

| Name | Type | Provider thin input | RequiredContext | Direct implementation |
| --- | --- | --- | --- | --- |
| `conversation.reply` | action | `text` | none | `conversationReplyCapability` |
| `moment.publish` | action | `text` | none | `momentPublishCapability` |
| `media.image.generate` | action | `intent` | `visual_identity`, `current_life`, `appearance`, `current_state` | `imageGenerateCapability` |
| `visual_identity.initialize` | internal | `{}` | `core_persona`, `visual_identity` | `visualIdentityInitializeCapability` |
| `scene_event` | action | `operation`, `confidence`; start/switch requires `scene`, `activity` | `current_life`, `schedule` | `sceneEventCapability` |
| `presence_event` | action | at least one of `user_presence`/`current_task`, plus `confidence` | `current_life` | `presenceEventCapability` |
| `schedule.replan` | action | `intent` | `schedule`, `current_life`, `agency` | `scheduleReplanCapability` with capability-local `SchedulePlanner` |
| `memory_event` | action | `content`, `type`, `confidence`, `importance` | `core_persona`, `memory_scope` | `memoryEventCapability` |
| `affect_event` | action | nested semantic `event` | `current_state` | `affectEventCapability` |
| `relationship.lookup` | query | `target_actor_id` | `relationship_scope` | `relationshipLookupCapability` |
| `capability.request` | internal | `capability_key`, `title`, `description`, `rationale` | none | `capabilityRequestCapability` |

Registry 只保存 `Capability` 与 `CapabilityDefinition` 两张 map；构造和注册
会拒绝 nil、重复、非法 Definition 以及 Definition/RequiredContext 不一致。
Provider 只从 `Registry.Catalog(surface)` 取得 Definition，并由
`RenderCapabilityTools` 统一渲染。

## 删除与清理证明

生产 Go（排除测试 fixture 和一次性 migration）扫描通过，不再包含：

```text
CapabilityManifest
CapabilityExecutor
CapabilityManifest.Parameters
ToolCallPayload
legacyCapabilityAdapter
registryCapabilityAdapter
CatalogManifests
ExternalCapabilityManifests
capabilityManifestsExcept
optionalToolFailureNonFatal
normalizeConversationReplyCalls
bindMediaContextToToolCalls
ExecuteToolCalls
settleDeferredToolCallsTx
wakeUpAssessmentInstruction
conversationAssessmentInstruction
dailyReviewInstruction
```

同时删除了生产 `media_context.go` 旧 binder、旧 image executor、旧
`media_request`/`moment_media_request` conversion、WakeUp concrete-name
fallback 和 v1-first replay 读取。`ToolCallV1` 仅保留在 provider-shaped
测试 fixture；`tool_calls` 仅保留在 Provider root sidecar codec、response
schema 和线性 migration 边界。

## R0–R9 对应测试

- `TestBuiltinRegistryContainsExactlyElevenDirectCapabilities`：11 项均为
  direct Capability，且 Registry 没有 executor 字段。
- `TestAllBuiltinDefinitionsExposeExpectedThinRequiredInputs` 与
  `TestAllBuiltinDefinitionsAcceptMinimalProviderInput`：逐项最小输入和
  forbidden implementation fields。
- `TestDummyCapabilityTraversesCatalogCodecResolverRuntime`：dummy 实现只需
  注册即可贯穿 catalog → renderer → codec → resolver → Runtime。
- Context tests 覆盖 only-requested、duplicate slot dedupe、unknown/missing
  slot、typed mismatch、loader error、取消、empty App resolver 和 snapshot
  identity round-trip。
- Image tests 覆盖四个 context slot 的 Prepare、`prepared_concept` 和
  context binding；deferred path 再校验冻结 context binding。
- Schedule tests 覆盖 intent-only schema、fake planner success、invalid
  output、provider error、revision stale rejection、completed-history
  validator、planner schema 不进入 Provider catalog，以及 Prepare 后只调用
  planner 一次。
- Canonical replay tests 覆盖 invocation/result identity、duplicate call ID
  拒绝、snapshot/provenance 保留、CompositeAction 只持有 call IDs，以及
  active v1 payload 的纯函数迁移和 dual-authority/malformed fail-closed。
- Deferred/failure tests 覆盖 reply/Moment target binding、稳定 media
  intent/workflow/provider IDs、`ExecuteDeferred` error → failed、settlement
  rollback 后 deferred → bounded failed、required missing/deferred result
  fail-closed、single cognition/no `role=tool` continuation 和 generic
  output-role classification。
- migration runner tests 检查 active-row 预检、`RAISE EXCEPTION`、真实
  canonical `call_id`/`capability_name`、source/provider fallback、duplicate
  与 dual-authority guard、旧 nested `decision.tool_calls` 删除。

## Schema bytes/chars

研究基线（旧 renderer，全部 11 项）：`7,857 B / 7,857 C`；普通 conversation
`7,564 B`；native cognition `6,500 B`。

第二版阶段记录的 `CapabilityToolSchemaStats`（已被上方第三轮数值取代）：

- all 11 definitions：`4,994 B / 4,994 C`
- conversation surface（9 definitions）：`4,456 B / 4,456 C`
- native-cognition surface（8 definitions）：`3,864 B / 3,864 C`

当前 Definition JSON 为 ASCII，故 bytes 与 rune chars 相同；Provider envelope
没有 usage/token 字段，本次没有增加 tokenizer 或伪造 token 数。

## 本地门禁结果

以下命令均在当前 worktree 通过：

- Core `go test ./... -count=1`：PASS
- Core `go test -race ./... -count=1`：PASS
- Core `go vet ./...`：PASS
- Core `go build ./...`：PASS
- Gateway `go test ./... -count=1`：PASS
- Gateway `go test -race ./... -count=1`：PASS
- Gateway `go vet ./...`：PASS
- Gateway `go build ./...`：PASS
- `find apps/core-go apps/gateway-go -name '*.go' | xargs gofmt -l`：PASS
- `git diff --check master`：PASS

生产架构 guard 使用明确排除测试 fixture 的 `rg` 扫描，结果为 `PASS`；
concrete capability-name switch guard 结果为 `PASS`。

## 尚未运行的真实环境证据

当前环境变量均未配置：

```text
GO_CORE_TEST_DATABASE_URL=unset
TEMPORAL_ADDRESS=unset
FLUCTLIGHT_LIVE_PROVIDER_TEST=unset
REDIS_URL=unset
```

因此以下项目不能在本 session 宣称已验证：

1. 真实 PostgreSQL 上的 active frozen/autonomy migration、replay、CAS 和
   crash-window 集成；migration SQL 目前只有纯字符串/纯 payload 测试证据。
2. Temporal active history replay/drain gate、workflow versioning 和真实
   Worker recovery。
3. Live Provider 对图片 intent、native/sidecar precedence 的 opt-in 行为。
4. Redis/PostgreSQL outbox 的真实 reclaim/poison/duplicate 集成。

这些缺口是外部环境缺失，不是本地单元测试失败；配置相应服务后应重新运行
任务 `implement.md` 的 S08 集成门禁。
