# Prompt, Runtime, and Test Baseline

## Prompt coupling

- Global `providerRuntimeProtocol` is injected by `provider_prompt_composer.go:9-21,35-86,118-149` and already says external capabilities must use a standard Tool Call rather than claiming completion in prose.
- `conversationAssessmentInstruction` (`provider_prompts.go:19`) is approximately 1,818 characters and repeats per-tool triggers, fields, no-op behavior, media intent, scene/schedule coordination, and claims rules. `wakeUpAssessmentInstruction` is approximately 1,101 characters; daily review is approximately 781 characters. `provider_prompts_test.go:8-46` asserts many tool names/rules remain present.
- `scene_event` description is approximately 323 characters and `schedule.replan` approximately 221 characters; both contain business trigger/coordination rules that should move to the global policy or implementation.
- `provider_prompt_composer.go:203-394` also hardcodes persona field filtering and 14 dynamic context sections. `provider.go:305-325` embeds Visual Identity media instructions in the provider boundary. This task should only extract capability-related policy and avoid broad response-format redesign.

## Runtime and persistence baseline

- Native and sidecar calls normalize into `ToolCallV1` (`tool_contract.go:306-390`) and are frozen with the decision (`cognition.go:280-353`).
- `ExecuteToolCalls` validates/looks up/executes in input order, returns per-call results and currently always returns a nil aggregate error (`capability_runtime.go:50-154`). Per-call failures become `ToolResultV1` and continue.
- Deferred output calls first record `deferred`, then settle after the assistant/Moment target has a durable ID (`capability_runtime.go:334-410`; `mutations.go:791-825`).
- Main chat does one structured Provider call and does not send ToolResult as a second `role=tool` message. This single-cognition behavior is a hard compatibility constraint.
- The Provider queue/HTTP path propagates request context through timeout and cancellation (`server.go:656`; `mutations.go:1075-1082`; `provider.go:181-198`). Exclusive lock cleanup uses `context.Background()` only for post-cancellation unlock (`capability_runtime.go:462-464`).
- Frozen actions store projection/tool calls/results for replay. The projection is not a universal database snapshot: most capabilities re-read live domain state; image media is the only capability with an explicit frozen context binding (`media_context.go:10-99`; `intelligence.go:118-245`).

## Test baseline

Existing focused tests:

- Normalize/schema/registry: `tool_contract_test.go:20-199,252-330,339-409`.
- Composite/deferred/no-op: `composite_actions_test.go:65-127`, `conversation_tool_response_fixture_test.go:8-69`, `intelligence_test.go:65-150`, `wakeup_test.go:49-115`.
- Image context: `media_context_test.go:9-109`.
- Prompt size/required fragments: `provider_prompts_test.go:8-46`.

Missing tests:

- No direct `ExecuteToolCalls` or deferred settlement tests.
- No Context Resolver/Slot/CapabilityContext tests.
- No thin schema forbidden-field or before/after size tests.
- No deterministic Provider -> Runtime -> domain behavior tests for image, scene, or Moment publish.
- No guard preventing new concrete capability-name branches.
- Existing `NewCapabilityRegistry` constructor discards registration errors (`tool_contract.go:124-129`).

## Baseline validation run

Read-only baseline from this worktree's HEAD:

Provider tool definitions serialized through the current `ToolCallPayload`:

| Catalog / Tool | Count | JSON bytes |
|---|---:|---:|
| All production capabilities | 11 | 7,857 |
| Ordinary conversation | 10 | 7,564 |
| Native cognition | 8 | 6,500 |
| `schedule.replan` | 1 | 1,514 |
| `memory_event` | 1 | 1,375 |
| `scene_event` | 1 | 1,099 |
| `capability.request` | 1 | 920 |
| `affect_event` | 1 | 765 |
| `presence_event` | 1 | 594 |
| `media.image.generate` | 1 | 385 |
| `relationship.lookup` | 1 | 332 |
| `conversation.reply` | 1 | 301 |
| `moment.publish` | 1 | 294 |
| `visual_identity.initialize` | 1 | 288 |

The exact numbers were produced by a temporary Go test overlay against unmodified HEAD; no repository source file was added. They are the pre-refactor comparison baseline.

```text
go -C apps/core-go test ./... -count=1             PASS outside restricted socket sandbox
go -C apps/core-go test -race ./... -count=1      PASS outside restricted socket sandbox
go -C apps/core-go vet ./...                      PASS
go -C apps/core-go build ./...                    PASS
go -C apps/gateway-go test ./... -count=1         PASS outside restricted socket sandbox
go -C apps/gateway-go vet ./...                   PASS
go -C apps/gateway-go build ./...                 PASS
gofmt -l apps/core-go apps/gateway-go             PASS (no output)
```

The restricted sandbox can fail tests that bind `127.0.0.1:0`/`[::1]:0`; this is an environment limitation, not a repository baseline failure. DB integration tests skip without `GO_CORE_TEST_DATABASE_URL`; live provider behavior skips without `FLUCTLIGHT_LIVE_PROVIDER_TEST=1`. Browser generation gates were not run because `pnpm generate` writes generated artifacts.
