# Persona 与 Active Memory 独立 Tool 实施记录

日期：2026-09-22

## 实现边界

- `persona.takeover` 与 `persona.switch` 改为 `TransactionalCapability`，不再返回 `deferred/awaiting_domain_commit`。
- `persona.switch` 复用 `preparePersonalityDecision` 与 `applyPersonalityDecisionPlanTx` 的既有 profile、trigger、cooldown、revision/CAS 规则。
- `persona.takeover` 复用既有 typed takeover decision、声明 profile 与 `takeover_rules`；它提交当前轮接管结论的审计事件，不改写持久 dominant profile。
- 两项 persona action 都以 `fluctlight + capability + operation_id` 为稳定业务身份；相同 payload replay 返回既有结果，不同 payload 返回 `persona_action_idempotency_conflict`。
- persona 业务拒绝以 canonical `CapabilityResult{status:"rejected"}` 返回，同时在同一事务写 `platform_outbox_events` 审计；数据库/事务异常仍返回 Go error。
- `active_memory_event` 不再要求 `ActionID`、冻结 action 或 cognition inbox 行。来源冻结为 `ActiveMemoryCapabilitySource`：显式 evidence ID、operation ID、Conversation、actor 与业务受理时间；真实 cognition fact 存在时仍保留其权威 `occurred_at`。
- direct Active Memory target 可由 `memory.recall` 返回的 opaque ref 或冻结 ContextReference ref 解析到 live owner/conversation/revision；生命周期写入仍只走 `applyActiveMemoryCommandTx`。
- direct Active Memory replay 从 `active_memory_commands.command` 恢复最初的来源时间与 actor，native ToolCall ID 改变不会改变业务幂等身份。

## 修改文件

- `apps/core-go/internal/core/persona_action_capabilities.go`
- `apps/core-go/internal/core/persona_action_service.go`（新增）
- `apps/core-go/internal/core/persona_action_capabilities_test.go`
- `apps/core-go/internal/core/active_memory.go`
- `apps/core-go/internal/core/active_memory_tool_test.go`（新增）

## 必需装配（由主线程在受保护文件完成）

`builtin_capabilities.go` 需要创建并注入同一服务：

```go
personaActions := newPersonaActionService(app)

personaActionCapability{name: personaTakeoverCapabilityName, service: personaActions},
personaActionCapability{name: personaSwitchCapabilityName, service: personaActions},
```

并增加编译期声明：

```go
_ TransactionalCapability = personaActionCapability{}
```

旧外层 settlement 仍会再次调用 `applyPersistentSwitchIfAuthorizedTx`；在 persona Tool 已提交后，该重复应用必须由拥有 `mutations.go` 的主线程切片移除或改为消费已提交结果，避免第二次 CAS。`turn_takeover.go` 的 stage 写仍是接管结果的 owning turn state，本切片没有改动其判定、模型或 stage machine。

## 正式 PostgreSQL 覆盖

- `TestPostgresDirectPersonaToolsCommitRejectReplayConflictAndAudit`
  - persona.switch 成功与 runtime revision 持久化
  - 同 operation、不同 native ToolCall replay
  - 同 operation、不同 payload 冲突
  - unknown profile 业务拒绝及审计
  - persona.takeover 审计提交且不改写 persistent profile
  - unknown rule 业务拒绝及审计
- `TestPostgresDirectActiveMemoryToolCommitsRejectsReplaysAndPersistsAudit`
  - 无 Main、无 cognition fact 的 create
  - 明确 evidence、owner actor、受理时间持久化
  - 同 operation、不同 native ToolCall replay
  - 同 operation、不同 payload 冲突
  - unknown target 拒绝
  - `memory.recall` opaque ref 驱动 complete
  - current row、revision、command ledger 与 outbox 独立查库

## 验证记录

- `gofmt`：通过。
- `git diff --check`（本切片文件）：通过。
- `go -C apps/core-go test -count=1 -run '^Test(PersonaPolicyCapabilitiesStayOutOfConversationCatalog|PersonaPolicyInvocationUsesStablePolicyIdentity|ActiveMemoryCapabilityDefinitionIsThinOptionalTransactionalAction|ActiveMemorySemanticValidationPreservesUnknownTimeAndChecksOffset|ActiveMemoryPreparedCommandDigestFreezesSourceTime|ActiveMemoryOpaqueReferenceAndProviderCompaction)$' ./internal/core`：exit 0。
- `go -C apps/core-go vet ./internal/core`：exit 0。
- `python3 /tmp/lac-run-tool-test.py persona-tools 'TestPostgresDirectPersonaToolsCommitRejectReplayConflictAndAudit$'`：exit 0，真实一次性 PostgreSQL 通过；脱敏证据为 `research/runs/persona-tools.jsonl` 与 `.meta.json`。
- 首次组合运行 `python3 /tmp/lac-run-tool-test.py persona-active-tools 'TestPostgresDirect(PersonaToolsCommitRejectReplayConflictAndAudit|ActiveMemoryToolCommitsRejectsReplaysAndPersistsAudit)$'`：exit 1。persona 用例通过；Active Memory 精确暴露已有数据库约束 `fk_active_memories_source_fact -> cognition_inbox(id)`，direct evidence 在写 `active_memories` 时被拒绝。脱敏失败证据保留于 `research/runs/persona-active-tools.jsonl`，未把它计为通过。

## 待主线程完成的 schema 接线

`active_memories.source_fact_id` 当前仍由 migration 0032 强制外键到 `cognition_inbox`。若不移除此约束，任何“不伪造 cognition fact”的 direct Active Memory 实现都无法提交。这不是 Runtime 门禁，而是 PostgreSQL authority。schema owner 需在新的迁移/兼容步骤中解除 `fk_active_memories_source_fact`，并明确该 legacy 列保存 evidence resource ID；更完整的后续形态可增加 `source_kind/source_resource_id`，但本任务不应为满足旧 FK 制造 inbox 行。完成后重跑 `TestPostgresDirectActiveMemoryToolCommitsRejectsReplaysAndPersistsAudit`。
