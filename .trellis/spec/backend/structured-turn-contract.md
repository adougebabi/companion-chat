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
- A committed Schedule Tool refreshes the private Agent's outbound runtime
  context before the next physical model request. If one model response asks
  for `intention.schedule` or `schedule.replan` and `conversation.reply`
  together, the premature reply is rejected as a Tool result; the next model
  decision reads the accepted Schedule before composing user-visible text.
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
  Private conversation messages require a real native `conversation.reply`
  ToolCall and committed receipt. Final text, `visible_text`, response plans and
  reasoning never publish a message. Accepted media alone does not satisfy the
  private reply boundary; final structured output is cognition/settlement data.
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

## Scenario: Recoverable Tool Arguments And Minimal Model Result

### 1. Scope / Trigger

- Trigger: a native ADK ToolCall has schema-invalid or missing model arguments, a Tool returns an authorized business rejection, or a later model decision consumes its result.
- The same rules apply to a first execution, same native-ID replay, and an early same-batch rejection (for example a reply requested alongside a Schedule mutation).

### 2. Signatures

```go
App.ExecuteTool(ctx, ToolExecutionRequest) (ToolExecutionReceipt, error)
classifyToolPrepareError(error) (code string, retryable bool, detail string)
modelFacingToolResult(ToolExecutionReceipt, CapabilityDefinition) map[string]any
```

`CapabilityDefinition.ModelResultOmitFields` is a Core-owned, output-schema-declared list of fields to omit only from the model-facing result. It does not change the persisted receipt or direct Tool output.

### 3. Contracts

- `CapabilityInvocation.Validate` remains the sole model-argument schema gate. An `ErrInvalidArguments` from either the Core or shared capability package becomes `invalid_arguments`, `retryable=false`, and a bounded field/type hint. Unknown model-supplied property names are not copied into this hint.
- A typed, non-retryable business target error retains its domain code. Database, resolver, dependency, authorization and cancellation failures are not converted into a successful or correctable schema error.
- ADK sends the next model decision only `{status,error_code?,retryable?,output?}`. Native ToolCall pairing stays in the actual Tool message ID; operation ID, execution call ID, Provider request ID, authority revisions and the complete receipt remain Core-side.
- `ModelResultOmitFields` must name declared output properties. For `memory_event`, the model receives `target_ref` for a later correction while the raw `memory_id` and `revision` remain only in the canonical receipt. A replay must apply the same projection.
- Queries keep their requested answer in `output`. Mutations retain only business fields required for a later decision. No fixed Agent round/tool-count limit is introduced; the existing request lifetime governs continuation.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| `commit_review.observations` has the wrong JSON type | `invalid_arguments`, non-retryable Tool result with field/type feedback; no domain write |
| An unknown Memory/Active Memory target ref | Non-retryable target error; no guessed ID or revision |
| A unique current profile intention/activity omits its ID | Core binds that one target under the current scope |
| Multiple possible targets omit an ID | `*_selection_required`, non-retryable; no arbitrary first-row choice |
| Tool preparation cannot reach PostgreSQL or another dependency | Retryable failure; ADK run may terminate and retains prior committed receipts |
| Native ToolCall ID is absent or reused inconsistently | Protocol error, never reinterpreted as a missing business target ID |
| First/replayed/same-batch Tool result | Same bounded model-facing projection; full Core receipt remains available for audit |

### 5. Good / Base / Bad Cases

- Good: malformed `observations` comes back as a field/type failure, the model corrects the ToolCall, and exactly one accepted review commits.
- Base: `memory_event` creates a Memory; its result supplies an opaque `target_ref` that the next model decision can use without a raw database ID.
- Bad: mark a schema mistake as retryable infrastructure failure, expose a full receipt in a replay fast path, or select one of several active activities by row order.

### 6. Tests Required

- A direct/ADK schema-invalid Tool test asserts `invalid_arguments`, `retryable=false`, field/type feedback, and no write; a dependency fault retains retryable classification.
- First execution, same native-ID replay and same-batch rejection all assert model input omits receipt identity and configured fields while the internal trace still contains them.
- Isolated PostgreSQL tests assert unique/ambiguous/missing target behavior, profile isolation, Memory create→revise and Active Memory create→complete via returned `target_ref`.
- `TestConversationCapabilityCatalogFitsDefaultPromptBudget` must remain green; concise descriptions and Core target inference are preferred to unbounded `oneOf` expansion.

### 7. Wrong vs Correct

#### Wrong

```go
return jsonString(receipt), fmt.Errorf("tool execution capability_prepare_failed: %w", schemaErr)
```

#### Correct

```go
result := failedCapabilityResultDetail(invocation, "invalid_arguments", false, safeToolArgumentFeedback(schemaErr))
return jsonString(modelFacingToolResult(toolExecutionReceipt(request, callID, result), definition)), nil
```


## Scenario: Recoverable Runtime Errors Without Fabricated Effects (2026-10-06)

### 1. Scope / Trigger

Borrowed wardrobe persistence, WorkingMemory section pressure, an oversized
physical continuation, stale private reply context, unknown decision references,
or cancellation during an Agent run. Preserve committed actions and domain fences.

### 2. Signatures

```text
reserveRequiredWorkingMemory(input, policy, maxInput) -> policy | budget error
queuedToolCallingChatModel.compactPhysicalInput(ctx, messages) -> outbound copy
publicationCapabilityError(ErrLifeContextStale, ...) -> life_context_stale,false
providerContextRefCodec.registerIndex(index) -> current permitted ref set
validateADKFinalContract(..., codec) -> unknown-ref error before settlement
borrowingPersistenceFailure(invocation, code, pgError) -> failed result
```

### 3. Contracts

- PostgreSQL loan Event arithmetic explicitly casts the bound instant to
  timestamptz. Every loan/return Event also carries real id, revision, status,
  expected/resulting Life Context revision and replayed=false before commit.
  The released deferred replay-ready trigger applies even to historical inserts;
  do not bypass it. Event/effect/outbox/Tool receipt remain one transaction.
- SQLSTATE class 42 indicates a SQL/schema/type/access failure that repeating
  business arguments cannot repair. Borrowing returns failed/non-retryable feedback
  saying no change committed. Transient connection/serialization errors retain
  recovery behavior; never fabricate acquired IDs or clothing after a failure.
- Required WorkingMemory fragments rank before optional ones. Assembly may expand
  an individual section quota to its required content, bounded by the configured
  total input limit. Required recent groups reserve the whole group/contiguous
  tail. Final wire checks still include system, Runtime, current input, Tools,
  results, response schema and output/safety reserves; no context-limit increase.
- If the physical request is oversized, compact only its outbound copy: discard
  older plain assistant reasoning first, then complete optional historical
  user/assistant turns when a formal Runtime marker identifies them. Preserve
  system, Runtime, the latest user input including images and every native
  ToolCall/ToolResult unchanged. Original Eino history and receipts remain intact.
  A still-oversized mandatory request fails before HTTP; never truncate a result
  or silently convert it to a successful empty response. Record estimated
  before/after counts and removed message counts in adk.model.input_compacted.
- A private publication failing Life Context CAS returns non-retryable-in-place
  life_context_stale as ordinary Tool feedback and marks projection refresh dirty.
  The next physical decision reads the new context and may compose a new reply;
  it does not resend old prose or re-execute earlier committed Tools automatically.
  A confirmed stale-reply rejection followed by a real projection reread may
  re-anchor settlement to that new decision: server-owned AfterRecoveryCallID
  marks the last trace invocation already observed, and StaleReplyCallID must
  identify an actual failed conversation.reply/life_context_stale result before
  that boundary. Earlier effects stay in the audit/outcomes; only later receipts
  extend the new CAS chain. Ordinary refresh never rebases, missing/forged
  recovery markers are rejected, and later external changes still fail CAS.
- Well-shaped influences.ref must be present in the current permitted projection
  index, after alias decoding. Update the codec's permitted set after refresh
  merges the run-start references allowed by scope; old alias decoding alone is
  not authority. Unknown refs enter the existing single tool-free final repair
  before business settlement. Do not drop refs silently, invent replacements,
  replay Tools or restart the whole run. The unbound repair model also omits Tool
  schema from physical budgeting, matching the actual no-Tool request.
- Cancellation/timeout classification precedes wrapped Tool-stage classification.
  Cancellation remains request_cancelled and never starts a replacement run.
  Storage's existing failed terminal status is not changed by this classification.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| SQL 42804 or another class-42 loan write error | failed borrowing code, retryable=false, no committed inventory effect. |
| Loan Event missing replay fields | Database rejects the transaction; fix the writer, not the trigger. |
| Required section exceeds its default cap but fits total budget | Reserve sufficient section capacity, displace optional fragments. |
| Required content itself exceeds total budget | prompt_required_budget_exceeded before model HTTP. |
| Continuation exceeds budget due optional reasoning/history | Compact outbound copy, re-count full request and continue only if it fits. |
| Latest mandatory Tool result still too large | prompt_tool_result_budget_exceeded; full result remains in audit. |
| Reply context changes between decision and publication | life_context_stale feedback -> refresh -> new decision; stale prose not published. |
| Unknown full ref or alias in final influences | Bounded tool-free final correction; a failed correction stays a failure. |
| Wrapped context.Canceled | cancellation/request_cancelled, no automatic replay. |

### 5. Good / Base / Bad Cases

- Good: a stale home reply is rejected, fresh shop context is read, and the next
  native reply publishes once without rerunning the scene change.
- Good: SQL-safe borrowed registration -> actual ID -> wear -> return -> replay.
- Base: a large required Tool result cannot fit even after optional compaction;
  refuse the model request with explicit budget diagnostics.
- Bad: fix a SQL defect by retrying indefinitely, remove domain/version guards,
  raise the context ceiling, truncate Tool outputs, or discard unknown provenance.

### 6. Tests Required

runtime_recovery_test.go covers section borrowing and strict total refusal,
reasoning/history compaction without changing raw Eino or Tool protocol, actual
native final-ref correction with a script HTTP model, cancellation classification,
and real-PG prior wearing effect -> stale reply rejection -> refresh -> exactly
one new publication -> valid settlement. Negative checks preserve ordinary
external-change rejection and reject forged recovery markers.
wardrobe_borrow_capability_test.go covers actual timestamp writes, deferred commit
constraints, ownership, replay, atomic return failure and unusable returned items.
Existing required-result overflow tests remain negative cases. Script model
transports validate the real loop, not actual vendor-model behavior or image pixels.

### 7. Wrong vs Correct

Wrong: use an untyped $4 - interval expression, create a replay-incomplete Event,
retry stale prose, or allow syntactically valid but nonexistent refs to reach commit.
Correct: typed time and replay-ready Events; consume refreshed context; validate
actual scoped refs before the existing no-Tool final repair; respect the wire limit.


Task-private capabilities may implement Core's `toolExecutionAuthorizer`. The generic ExecuteTool boundary invokes that interface before any mutation receipt replay; business-name dispatch remains forbidden. Private Goal Evaluation sessions must validate actual native/model/task identity and active claim before replay as well as during their mutation transaction. Owner-only governance also uses capability-owned authorization. Catalog visibility does not substitute for this guard.


## Scenario: Goal Evaluation native execution and final output phases

### 1. Scope / Trigger
The typed Goal Evaluation task enters the existing Eino Runner with frozen root targets. A valid final summary must not terminate a run before native submissions; a summary grammar on every physical request can suppress native calls on an OpenAI-compatible server.

### 2. Signatures
`withPhysicalModelRequestPolicy(ctx, physicalModelRequestPolicy)` installs a private task callback. `DecidePhysicalModelRequest(ctx) -> {ToolChoice, OmitResponseFormat}, error` is evaluated by queued Generate/Stream for each physical request. `goalEvaluationPhysicalRequestPolicy` reads `(status, claim_revision, result)` from the owning `goal_evaluation_requests` row using frozen request/Fluctlight/profile identity. No public arguments or settings are added.

### 3. Contracts
- While durable `goal.evaluation.submit` records do not cover all frozen roots, send canonical native Tools with `tool_choice="required"` and omit `response_format` from the serialized body. The original registry schemas, thinking flag, headers and physical call IDs remain intact. Object/plan receipts and summary prose cannot cover a root.
- When root submissions are accepted, restore `tool_choice="auto"` and the original summary schema. Optional object/plan calls remain allowed; no second Runner or manual continuation is introduced.
- A real frozen source/authority conflict permits an error summary so the existing finalizer can queue unresolved/deferred targets with a fresh snapshot. This escape never counts as root coverage or success. Ordinary correctable judgment/argument rejections keep required native execution.
- Apply options using locked Eino APIs: `model.WithToolChoice` and `openaiext.WithRequestPayloadModifier`. Do not replace ExtraFields to remove one format field. Budget and diagnostics use the effective format of this physical request.
- Final DTO repair is tool-free and explicitly suppresses the task request policy; it cannot issue required ToolCalls without a catalog or replay mutations. Conversation and Visual Identity install their own request policies; WakeUp and Planner retain their original request behavior.
- DB/claim/cancellation failures remain errors. Native state, evidence/CAS, ownership, paused settlement and independent Tool transactions remain the sole business authority.

### 4. Validation & Error Matrix
| Condition | Required behavior |
| --- | --- |
| Uncovered roots, fresh run or partial retry | required native Tools; no summary grammar |
| Root rejected for wrong judgment/arguments | required remains; actual failure feedback reaches next decision |
| Frozen source/authority stale | allow error summary; missing roots remain unresolved; existing fresh-request path runs |
| All durable roots accepted | auto + summary schema; optional Tools allowed |
| Claim no longer processing or revision changed | goal_evaluation_claim_stale; no new physical request |
| Policy DB read/cancellation fails | propagate cause; no fabricated coverage |
| Final schema repair | no inherited request policy, no native Tool catalog |

### 5. Good / Base / Bad Cases
Good: rejected native root -> feedback -> corrected real root -> other roots -> final summary. Base: a retry starts with one accepted root and submits the remainder. Bad: return “正在评估4个目标” under summary grammar while all root submissions remain absent, or force ToolCalls against an irreparably stale snapshot until timeout.

### 6. Tests Required
`TestGoalEvaluationPhysicalRequestPolicyRequiresNativeRootsBeforeSummary` uses the actual OpenAI adapter and Eino Runner with HTTP/SSE, checking Generate/Stream × empty/partial coverage, three closed canonical Tool schemas, actual stream=true, rejection feedback, required/no-format execution requests, auto/schema final request and consistent nonempty physical identity headers. The controlled service must return premature summary on auto/summary grammar so the old implementation demonstrably fails. `TestGoalEvaluationFrozenConflictCanExitButJudgmentRejectionCannot` distinguishes stale exit from correctable evidence rejection. `TestPhysicalModelRequestPolicyIsSuppressedForFinalRepair` preserves the tool-free repair boundary. Production DB policy/Task cases require the isolated PG fixture and are explicitly SKIP without it. The test router identifies the private native catalog when the final schema is absent; production has no fixture or DTO fallback.

### 7. Wrong vs Correct
Wrong: install the summary JSON schema for every physical decision and rely only on prompt wording to require submissions.
Correct: before each physical request, read durable root coverage, select required native execution without final grammar, and restore the final contract only after accepted coverage or an explicit stale-snapshot error exit.


## Scenario: Background Agent logical ownership and generation provenance (0059)

### 1. Scope / Trigger
Runtime summary and daily-memory consolidation previously ran in independent Activities without the same Fluctlight lease held by chat. Cursor-only Intention processing also advanced the facts fence. Apply this contract to these background entries and every context-generation trigger, preserving genuine source/state changes.

### 2. Signatures
- `ProcessConversationSummaryIntent` / `ProcessConversationDailyMemoryIntent` enter `runBackgroundLogicalWork` with kinds `conversation_summary` / `conversation_daily_memory`.
- Migration head: `0059_fact_generation_provenance`, previous `0058_semantic_fact_generation`.
- `bump_fluctlight_context_generation_with_source(owner, table, operation, entity_id?)` increments and journals in the same transaction; legacy one-argument bump remains supported with unknown source.
- `fluctlight_context_generation_journal`: scoped generation PK, source table/operation, optional entity ID, transaction ID and timestamp; latest 512 generations per Fluctlight.
- Existing Owner `agent.run.termination` payload adds `current_facts_generation_sources` with available/complete, bounded interval, observed count and table/operation/count groups.

### 3. Contracts
- Identity/replay reads may occur before lease acquisition; no authority snapshot may escape that boundary. Summary revalidates identity within the lease; daily-memory re-reads payload/owner after admission. Source reads, LLM and final commits share the child context and existing transaction fence. Same-owner reentry retains the parent lease; different Fluctlights remain independent.
- Keep chat input acceptance/supersede outside the waiting lease so new user input immediately cancels an older turn. Do not hold an SQL transaction or physical model queue slot across an entire Agent.
- Intention UPDATE skips generation only when all fields excluding trigger_cursor_sequence/updated_at are unchanged. Real status, trigger/action/body, revision, scope and ownership changes still advance. Identical consolidated summary status/memory ID performs no UPDATE; active→consolidated and actual changes remain fenced.
- Preserve inner-state, real Memory/source/message/Actor/Goal/Life changes and all existing CAS. Preserve assistant INSERT exemption, message/fact invalidation and resident-memory refresh side effects in rewritten trigger functions.
- 0059 is applied after historical installers and 0058; head rerun keeps both embedding-cache exclusion and cursor filtering. Journal stores no business payload or credentials and prunes in the same commit, so rollback restores counter and journal together.
- Diagnostics query only `(min(expected,actual), max(expected,actual)]` for the affected Fluctlight, at most 512 rows. Failure recording's later writes are outside that interval. Missing historical rows or retention truncation means complete=false. Query failure cannot replace the original current_facts_stale cause or turn it into success.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| Background summary/daily starts while same Fluctlight chat owns lease | wait before semantic snapshot, Provider and commit |
| Same-owner nested work | reenter, no second lease or parent release |
| Other Fluctlight | independent work |
| Cancellation or lost lease | real cause/fence error, no stale commit |
| Cursor-only or identical summary consolidation | no facts generation change |
| True source/body/status change | generation + source journal; old CAS rejects |
| Conflict before 0059 or beyond retained window | diagnostic complete=false |
| Journal query fails | available=false; original CAS error retained |

### 5. Good / Base / Bad Cases
Good: summary waits for chat settlement, then takes its own fresh snapshot. Base: a cursor consumes an irrelevant processed fact without changing factual authority. Bad: a running independent Activity commits Memory during another Agent's protected snapshot, or a counter difference is attributed to a writer without evidence.

### 6. Tests Required
Background entry-work seam tests check snapshot/Provider/commit waiting, release, same-owner reentry, separate owners, cancellation and lost-lease fence under race. Bypassing the real work wrapper must produce the behavior RED “background entry reached snapshot while chat held the logical lease”; restore for GREEN. Public entry placement is separately reviewed and real cross-process/DB paths require the isolated PG fixture.
0059 real-PG tests cover empty/0058/head-rerun, unchanged cursor versus true status/trigger, source corrections, journal owner isolation/rollback/512 retention and bounded diagnostics. Pure SQL contract and interval parser tests are not PostgreSQL execution. Unconfigured DB cases are explicitly SKIP.

### 7. Wrong vs Correct
Wrong: serialize only chat/Wake/Goal entries, assume dispatcher priority prevents an already running summary Activity, or count cursor progress as a changed world fact.
Correct: every protected background work entry uses the same durable Fluctlight lease across snapshot→LLM→commit; generation changes reflect semantic authority and carry bounded source provenance.


## Scenario: Private chat requires native publication (2026-10-10)

### 1. Scope / Trigger
Conversation final text previously bypassed the Tool protocol. Private chat must publish only through native conversation.reply.

### 2. Signatures
`conversationPublicationPhysicalRequestPolicy.DecidePhysicalModelRequest(ctx)` reads the current native trace. `conversationHasAuthoritativeOutput(trace)` accepts only committed conversation.reply receipts.

### 3. Contracts
Until a real reply commits, physical Generate/Stream requests use required Tool choice and omit final response_format. Other Tool results, accepted media and prose do not release the boundary. After the reply commits, restore auto and the cognition final schema. Remove visible_text from the turn/response_plan schemas and remove natural-final publication. handleTurn loads committed message resources only. Final JSON describes cognition and settlement; it never creates a message. Keep the existing single Eino Runner and independently committed Tool transactions.

### 4. Validation & Error Matrix
| Condition | Behavior |
| --- | --- |
| Provider returns final-only without reply despite required | conversation_native_output_missing under final-contract failure; no text publication/DTO repair |
| Only media accepted | reply still required |
| Reply already committed, later final malformed | existing tool-free final repair may correct DTO; no duplicate send |
| No committed message at caller projection | agent_output_publication_failed with conversation_native_output_missing |

### 5. Good / Base / Bad Cases
Good: actual native reply → committed receipt → final cognition. Base: optional read Tool precedes reply. Bad: publish visible_text or infer message/image from thought.

### 6. Tests Required
`TestConversationPhysicalRequestsForceNativePublicationGenerateAndStream`, `TestConversationFinalOnlyResponseNeverPublishesOrRepairs` and `TestConversationPublicationPolicyRespectsCommittedOutputs` check wire phases, actual native results, accepted-media boundary and no final-only publication. PG caller/receipt behavior remains opt-in.

### 7. Wrong vs Correct
Wrong: decode a final field and call publication service. Correct: the model calls conversation.reply, its receipt proves publication, and the caller projects the stored message.
