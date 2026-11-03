# Prompt / Role Baseline

Date: 2026-09-11

## Baseline A: Current Production Shape

At the conversation call site, the current message envelope is:

```text
messages[0] role=system
  capabilityConversationPolicyInstruction

messages[1] role=user
  current_message.sender
  current_message.content = text
  text = text
  context = compactCognitionContext(projection)

tools
  surface-filtered canonical CapabilityDefinition schemas

response_format
  strict conversation_turn_response JSON schema
```

Source: `apps/core-go/internal/core/mutations.go:722-729`.

Before transport, `composeProviderMessages` normalizes all ordinary requests into one leading system message and formatted non-system messages (`apps/core-go/internal/core/provider_prompt_composer.go:42-97`). For conversation, the output remains one system plus one large user document.

## Final System Composition

The current system document contains:

1. fixed `providerRuntimeProtocol`
2. call-site operation rules
3. dynamic actor/relationship context
4. filtered Core Persona extracted from the user context

Anchors:

- system extraction and rebuilding: `provider_prompt_composer.go:42-97`
- render order: `provider_prompt_composer.go:128-159`
- filtered persona groups: `provider_prompt_composer.go:213-256`
- dynamic actor/relationship system injection: `provider_context.go:158-194`, `244-250`
- single-leading-system test: `provider_prompt_composer_test.go:287-307`

This means the current branch does not fully satisfy `System = stable rules`. Stable Core Persona belongs there by contract, but current relationship goals/intentions and other relationship snapshot data are dynamic facts and are currently elevated into system.

## Dynamic User Document

`formatProviderDynamicPromptContent` / `renderProviderDynamicDocument` renders a structured text document with sections broadly ordered as:

```text
# 当前上下文
# Developing Self
# 当前日程
# 当前状态
# 记忆
# 当前目标
# 当前意图
# 近期行动结果
# 最近对话
# 关系
# 假设
# 驱动
# 偏好
# 触发偏好
# 视觉身份
# 在场状态
# 本次 Actor 消息
# 本次 actor_user 输入
```

Sources: `apps/core-go/internal/core/provider_prompt_composer.go:333-405`.

### Current input duplication

The same `text` is emitted from both `current_message.content` and top-level `text`, producing the final two sections above. The newest identical user row is separately removed from recent history by `compactRecentMessagesForActors` (`provider_context.go:409-426`), preventing a third copy but not the existing double copy.

### Recent role behavior

`DB.History(..., 12)` returns oldest-to-newest messages and the compactor preserves each item's semantic `role`, time, content and sender (`intelligence.go:239-248`, `repository.go:163-199`, `provider_context.go:409-451`). However, they are serialized as rows under `# 最近对话` inside the one user message. They are not transport-level `messages[]` entries with real `role=user|assistant|tool`.

No provider-level historical tool message is currently reconstructed.

## Tools and Response Schema

Capabilities are correctly omitted from dynamic context and emitted once through the OpenAI-compatible `tools` field:

- compact omission: `apps/core-go/internal/core/provider_context.go:12-22`
- canonical rendering: `apps/core-go/internal/core/tool_contract.go:156-178`
- payload injection: `apps/core-go/internal/core/provider.go:528-544`

Conversation simultaneously requests strict structured output:

- schema name `conversation_turn_response`
- `strict=true`
- `enable_thinking=true`
- `tool_choice=auto`

Sources: `mutations.go:729`, `provider.go:528-572`, `provider_schemas.go:189-224`.

## Assembly Authority Count

One final formatter exists for ordinary provider calls: `composeProviderMessages`.

Main-cognition request construction is still duplicated across four call sites:

1. conversation: `mutations.go:722-729`
2. native cognition: `cognition_growth.go:323-336`
3. daily review/autonomy: `autonomy.go:63-85`
4. wake-up: `wakeup.go:278-304`

Reflection is a fifth compact-context consumer (`reflection_runtime_v2.go:127-149`) but not ordinary Main conversation.

There is no thin authority that takes prepared fragments, applies a total budget, selects semantic units and emits provider messages.

## Baseline Metrics Available Today

- output `max_tokens` is available from role config
- tool schema bytes/chars can be measured by `CapabilityToolSchemaStats`
- prompt diagnostics store rendered messages
- live test can measure serialized bytes/chars/message count and wall latency

Missing today:

- actual provider input/output token usage capture
- total request estimate including tools and response schema
- per-section estimates and selection trace
- selected/dropped Memory reasons

## Experiment Harness

Use identical system facts, runtime facts, recent conversation, current input, tools and response schema. Only alter message/role organization.

- Live OpenAI-compatible fixture: `apps/core-go/internal/core/provider_live_tool_test.go:16-88`
- Full production payload capture without a live model: `apps/core-go/internal/core/life_context_e2e_test.go:68-128`

Known live test configuration pattern:

```text
FLUCTLIGHT_LIVE_PROVIDER_TEST=1
FLUCTLIGHT_LIVE_PROVIDER_URL=http://127.0.0.1:11234/v1
FLUCTLIGHT_LIVE_PROVIDER_MODEL=<local model>
go test ./internal/core -run <role experiment> -v
```

The task must detect endpoint availability and report an explicit environment block rather than inventing live model conclusions.
