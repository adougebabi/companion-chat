# Structured Turn And Independent Agent / Tool Contract

## Scenario: Formal tasks share one native Eino loop

### 1. Scope / Trigger

Applies to every complete model task, direct business Tool command, conversation,
background task and streamed browser turn. The Agent/Tool convergence replaces
the earlier single-Main, two-generation, pure-QUERY continuation, frozen-output
settlement and structured-sidecar execution rules. Those are retired contracts,
not compatibility requirements. Eino remains pinned to the repository version.

### 2. Signatures

```go
RunFormalAgent(ctx, FormalAgentID, FormalAgentRunInput) (ADKStructuredTaskResult, error)
RunFormalAgentStream(ctx, FormalAgentID, FormalAgentRunInput) (ADKStructuredTaskResult, error)
RunConversationCognitionAgent(ctx, ConversationCognitionAgentInput) (ConversationCognitionAgentResult, error)
RunVisualIdentityAgent(ctx, VisualIdentityAgentInput) (VisualIdentityAgentResult, error)
RunADKLoop(ctx, ADKLoopConfig, []*schema.Message) (ADKLoopResult, error)
App.ExecuteTool(ctx, ToolExecutionRequest) (ToolExecutionReceipt, error)
```

`FormalAgentDefinitions()` explicitly registers complete tasks. Shared language,
context-authority fragments, Slots, serializers and embedding calls are not
Agents. Typed task entries own their prompt, input/context assembly, model role,
Tool selection and final decoder; they all use `internal/ai/agent`'s Eino
`ChatModelAgent` / `Runner`, including naturally tool-free tasks.

`ToolExecutionRequest` separates authorized resource/subject, operation ID,
arguments and optional output target from actual model/run correlation.
`NativeToolCallID` and `ProviderRequestID` describe a model call only when it
really occurred. Direct commands use an explicit local execution identity.

### 3. Contracts

- Only Eino native ToolCalls request execution. Body text, reasoning and final
  JSON fields never create a second Tool protocol. Tool results return through
  matching native IDs and are consumed by the next model decision.
- Normal reads, writes, publications and business rejections may be followed by
  more decisions. Never terminate based on a QUERY/ACTION classification,
  number of prior queries, `deferred` result, or a write-tool list.
- Cancellation and request lifetime are protection. Production Agents do not
  impose a model/tool round or Tool-call count limit. Each physical model
  request has its own queue lease, request identity, diagnostic record and usage
  metrics; no hidden retry/failover is introduced around the whole Agent.
- Final schema validation applies only to the final assistant result. An
  intermediate ToolCall/result is not a final DTO. Invalid final output cannot
  fall back to older text. Text-output Agents are not subjected to JSON DTO
  validation merely because their text resembles JSON.
- Keep final content and executed-call audit separate. Preserve all actual
  intermediate ToolCalls/results, including published replies. A repeat native
  ID must not overwrite the original execution's physical model provenance.
- Registry schemas are the sole argument contract. Model-owned arguments cannot
  supply prepared state, ownership, database revision, or idempotency fields.
  A context resolver reads declared dependencies; optional internal preparation
  is not a deferred business outcome and does not require a frozen Main action.
- Catalog surfaces are default assembly groups, not execution-stage permission.
  Explicit Agent installation validates canonical registration and visibility;
  each Tool independently validates resource ownership and business conditions.
- `ExecuteTool` is the direct and Eino-adapter business boundary. Pure queries
  execute fresh reads. Writes prepare outside the transaction, then own a short
  transaction containing domain state, required outbox and `tool_executions`
  receipt. A target-owning Tool uses `DirectToolCapability`; ordinary mutation
  implementations use `TransactionalCapability` behind the same boundary.
- `completed` means the declared synchronous effect committed. `accepted`
  means an actual durable asynchronous task exists and includes its ID. Neither
  `waiting`, queued media nor a prepared candidate claims completed business.
  Business rejection is a normal explicit result; code/dependency/cancellation
  failures preserve their cause and are not silently normalized to success.
- Stable operation identity is independent of model ToolCall identity. Same
  scoped operation and same payload replays the committed receipt; changed
  parameters, evidence, subject or target conflict. Read-only queries are not
  cached in the mutation receipt ledger.
- `agent_runs` admits a business run before model work. Completed results can
  be replayed; failed/interrupted runs retain committed Tool evidence and cannot
  silently restart the whole decision loop. Cancellation-independent terminal
  recording is bounded. A durable visual checkpoint uses a stable state/attempt
  identity, so an unchanged waiting checkpoint does not resubmit a media task.
- A Tool commit survives later model failure, invalid final output, cancellation
  or another Tool's rejection. No global transaction spans model, database and
  external services. Never rewrite a committed sibling as rolled back because
  a later cognition projection failed.
- Conversation/API/background callers provide input and consume final output
  plus committed receipts. They do not prepare/execute an action list, assemble
  Tool results, select continuation, or publish a reply that a Tool published.
  Natural final text uses the same publication service as `conversation.reply`
  without fabricating a model ToolCall. Empty structured final text must never
  expose raw JSON; a valid Tool-only completion needs no invented assistant text.
- Browser transport remains POST/NDJSON with committed message/media resources,
  bounded errors, cancellation and one terminal event. Production streaming uses
  the same streaming Eino Runner. Prove Provider streaming with actual
  `stream=true` requests and consumed SSE deltas/DONE records; an aggregated
  final text alone is not that evidence. Browser `token` frames deliver only
  committed visible replies, preserving the existing post-commit publication
  contract. They are not speculative raw model-token deltas: structured JSON,
  ToolCall arguments and intermediate candidate text must never be flushed
  before validation/publication. Prompts, reasoning, credentials and private
  locators stay private.
- `persona.switch` and `persona.takeover` are business Tools, not exempt internal
  runtime controls. They validate declared profiles/rules and commit/audit their
  contracts. A takeover returns its working persona without changing persistent
  dominance; a persistent switch commits once and is not applied again outside
  the Agent. Model interpretation remains LLM-owned; Core owns authorization,
  CAS, cooldown and numeric policy.
- Existing Temporal activities may recover already-persisted business commands
  through this same Tool boundary and receipt ledger. They must not restore a
  second dispatcher/loop or interpret unexecuted final JSON as model ToolCalls.

### 4. Validation & Error Matrix

| Condition | Required outcome |
| --- | --- |
| Missing/unknown Tool, invalid schema or foreign target | Explicit error/rejection; no unauthorized write |
| Direct command lacks model ToolCall ID | Valid when business identity/resources are valid; no fabricated Provider request |
| Agent native call lacks real physical model identity | Reject execution; retain accurate diagnostics |
| Same operation with changed payload/target/subject | Conflict; original committed result unchanged |
| Same native ID reused with conflicting name/arguments | Protocol error; never merge incompatible calls |
| Business failure | Actual reason reaches subsequent model input |
| Dependency failure, cancellation, timeout, iteration exhaustion | Error; partial committed results preserved |
| Final structured result malformed | Error, no previous-text fallback; earlier Tool commits remain facts |
| Media is queued but no image exists | `accepted`/waiting with actual task ID, never completed asset |
| Mutation succeeds but receipt/outbox write fails in same transaction | Roll back that local transaction |
| Model fails after Tool local commit | Persist failed run and committed receipt; no whole-run replay |
| Structured final contains no visible text | Do not publish its serialized control JSON |
| A newer accepted conversation supersedes this run | Fence later final settlement; preserve already committed facts accurately |

### 5. Good / Base / Bad Cases

Good: `memory_event` commits, `memory.recall` observes that commit, and another
model decision uses the result. A failed scene change is returned to the model,
which chooses a valid alternative. A committed reply remains visible when a
later model call fails.

Base: a summary Agent has no tools and completes naturally through one Runner
model decision. A direct Tool call has an operation ID and no Agent context.

Bad: publish from final JSON `tool_calls`, cap every Agent at two decisions,
return candidate/deferred as successful business, fabricate a cognition fact to
invoke a Tool, or create a second message after `conversation.reply` committed.

### 6. Tests Required

- Fixed product inventory for all business Tools and all complete-task Agents;
  expected sets never come only from the current registry.
- Each Tool: direct success without Agent state, real business rejection,
  independent database/task/message/object assertion, dependency fault,
  repeat/conflict semantics, and actual Eino adapter execution.
- Each Agent: its real prompt, context, model adapter, tools and output contract.
  Across appropriate tasks cover no tools, one tool, two tool rounds/three
  decisions, failure adjustment, write/read, multiple calls, cancellation,
  isolation and commit-before-model-failure. Controlled regressions are labeled
  separately from real Provider evidence.
- A random database-only secret must be retrieved through the real memory Tool,
  observed in the next request and used in the final result; initial input and
  prompt cannot already contain the answer.
- Break result feedback and skip a write while returning success in isolated
  test-only experiments. The original success checks must fail, then pass again
  against final restored production code. No production fault flags remain.
- Existing runner: `infra/acceptance/run-go-live-provider-smoke.sh --suite
  tools|agents|all`, with `--tool`/`--agent` selectors. Full suites fail on missing
  coverage, zero matches, SKIP, BLOCKED or failure. LLM requests run one at a time
  with cross-process locking. Deferred user-run live acceptance stays NOT_RUN or
  BLOCKED; ordinary Go tests are not dual E2E acceptance.

### 7. Wrong vs Correct

Wrong:

```go
calls := parseActionsFromFinalJSON(completion)
freeze(calls)
settleInMainTransaction(calls)
```

Correct:

```go
// Inside the native Tool adapter, with a real Eino call ID and physical request.
receipt, err := app.ExecuteTool(ctx, request)
// Eino consumes the same serialized receipt before the next model decision.
// Outside the Agent, only inspect committed receipts and publish no duplicate.
```

## Scenario: Bounded context, semantic evidence and publication ownership

### 1. Scope / Trigger

Applies to context projections, claims, state proposals, persona attribution,
Memory/Relationship writes and reflection after an Agent run.

### 2. Signatures

`BuildContextProjectionFor`, `normalizeResponsePlan`, `freezeDecisionInfluences`,
`applyFrozenCognitiveStagesTx`, `MemoryRecallService.Recall`, and
`ProcessReflection` retain their owning domain authority. Function names with
`Frozen` do not reintroduce a Tool execution stage or global transaction.

### 3. Contracts

- Initial context is an authorized bounded snapshot. Subsequent committed Tool
  results and fresh queries are newer facts; the model must consume them.
- Authority remains confirmed Event > inferred Event > accepted Schedule >
  pending. Presence overlays only subject presence/current task. Core Persona
  identity is a hard constraint; Developing Self remains evidence-backed.
- Actor identity is explicit; transport role=user is not an authorization actor.
  Owner, speaking/subject Actor and Fluctlight are distinct when the product
  involves Actor-to-Actor conversations. Profile-scoped effects remain attributed
  to the actual acting profile; a Tool cannot widen ownership or visibility.
- Claims use bounded kind/content/confidence/evidence references. Unsupported or
  repeated self-claims do not become durable Memory or increase confidence.
  Never infer semantic state from assistant prose, regex or keywords.
- No appraisal means no fabricated state transition. A present malformed
  appraisal fails validation. Core owns bounded numeric deltas and CAS.
- Reflection validates its complete closed proposal and evidence window before
  applying candidates. Its domain mutations and watermark CAS remain atomic.
  Raw assistant realization is not new learning evidence. Reflection's current
  default tool set may be empty, but it uses the same formal Agent runtime.
- Publication uses authoritative message/Moment rows and existing outbox kinds;
  private storage URLs and internal control envelopes are never chat content.

### 4. Validation & Error Matrix

Foreign evidence, stale revision, invalid numeric input and unauthorized scope
fail before their domain mutation. These failures never erase prior independent
Tool commits. Invalid reflection candidates leave the watermark unchanged.

### 5. Good / Base / Bad Cases

Good: a fresh Tool query observes a just-committed scene revision. Base: no claims
means no invented Memory. Bad: retain an old initial state after a Tool changed
it, or attribute another Actor's statement to the human Owner.

### 6. Tests Required

Retain authorization-before-ranking, whole-fragment prompt budgeting, Actor and
profile attribution, reflection watermark rollback, claim non-promotion,
post-commit publication, supersession and real streamed request coverage.

### 7. Wrong vs Correct

Wrong: treat the last assistant sentence as evidence of a scene/Memory write.
Correct: read the owned resource or committed Tool receipt and retain its source,
revision and operation identity.
