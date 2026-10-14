# Exact task-scoped contract excerpts

Source: `.trellis/spec/backend/structured-turn-contract.md`

Source SHA-256: `515774a1b41dbfa1caf33cf0aca664e29c10091c3cdee63ff17760066a3b23c0`

Verbatim selected complete scenario sections to prevent native context injection truncation. The original spec remains authoritative. Read further source sections if implementation touches neighboring behavior; regenerate if its source hash changes.

<!-- Original lines 318–430 -->

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



<!-- Original lines 513–745 -->

## Scenario: Private chat requires native publication (2026-10-10)

### 1. Scope / Trigger
Conversation final text previously bypassed the Tool protocol. Private chat must publish only through native conversation.reply.

### 2. Signatures
`conversationPublicationPhysicalRequestPolicy.DecidePhysicalModelRequest(ctx)` reads the current native trace. `conversationHasAuthoritativeOutput(trace)` accepts only committed conversation.reply receipts.

### 3. Contracts
Until a real reply commits, physical Generate/Stream requests use required Tool choice and omit final response_format. Other Tool results, accepted media and prose do not release the boundary. After the reply commits, restore auto with the optional catalog and keep final grammar omitted. Validate the stopping candidate against the cognition schema; the existing isolated tool-free repair retains that schema. Remove visible_text from the turn/response_plan schemas and remove natural-final publication. handleTurn loads committed message resources only. Final JSON describes cognition and settlement; it never creates a message. Keep the existing single Eino Runner and independently committed Tool transactions.

### 4. Validation & Error Matrix
| Condition | Behavior |
| --- | --- |
| Provider returns nonempty final-only without reply despite required | at most one eligible native-publication correction; persistent omission fails conversation_native_output_missing, no text publication/DTO repair |
| Only media accepted | reply still required |
| Reply already committed, later final malformed | existing tool-free final repair may correct DTO; no duplicate send |
| No committed message at caller projection | agent_output_publication_failed with conversation_native_output_missing |

### 5. Good / Base / Bad Cases
Good: actual native reply → committed receipt → final cognition. Base: optional read Tool precedes reply. Bad: publish visible_text or infer message/image from thought.

### 6. Tests Required
`TestConversationPhysicalRequestsForceNativePublicationGenerateAndStream`, `TestConversationFinalOnlyResponseNeverPublishesOrRepairs` and `TestConversationPublicationPolicyRespectsCommittedOutputs` check wire phases, actual native results, accepted-media boundary and no final-only publication. PG caller/receipt behavior remains opt-in.

### 7. Wrong vs Correct
Wrong: decode a final field and call publication service. Correct: the model calls conversation.reply, its receipt proves publication, and the caller projects the stored message.


Physical request policies may set AllowedToolNames to a subset of the already installed catalog. The queued Generate/Stream adapter narrows actual model.WithTools and effective budget/diagnostics together, rejects unknown/empty requested catalogs with physical_request_tool_catalog_invalid, and never grants capabilities. Nil preserves the existing catalog. Planner query phases use this seam; Goal Evaluation, Visual Identity and conversation publication retain their existing policies and single Runner. Tool-free final repair suppresses the policy as before.


## Scenario: Final repair isolation and empty-output authority (2026-10-11)

### 1. Scope / Trigger
A final DTO correction inherited Kev selection and reinstalled business tools despite empty definitions. Reasoning-only empty finals also lost actual physical termination metadata.

### 2. Signatures
`finalRepairContext(ctx)` masks Kev selection/persona, physical business policy and projection refresh without mutating the original context. `finalRepairModelOptions` removes tools/tool_choice/parallel_tool_calls and sets enable_thinking=false on that correction request. `aiagent.EmptyFinalMessageError` retains the native final message and original adk_final_message_empty cause.

### 3. Contracts
Use one existing tool-free correction at most, retaining Generate/Stream mode, schema, refs, identity and cancellation. Ordinary thinking, Kev selection and required-native policies remain unchanged. No correction may replay committed effects, create Tool success from final fields, send reasoning, synthesize no_op or repair a private conversation that has not actually published through its Tool.

The queued native stream has one existing reader/producer, with a bounded diagnostic observer and no additional pump. Preserve available physical finish/usage across WithTools and native graph aggregation; never invent missing details from budgets or text. Empty content with no native ToolCall remains failure, classified model_output/adk_final_message_empty. stop, length, content_filter and unknown remain the reported reason, not an inferred budget explanation.

### 4. Validation & Error Matrix
| Condition | Behavior |
| --- | --- |
| Malformed final with Kev active | repair sends zero Tools, no tool_choice, thinking=false |
| Reasoning-only empty final | fail with actual metadata; no publication/automatic retry |
| Stream terminal-only metadata | preserve earlier content/calls and actual finish/usage |
| Stream error/cancel | preserve available partial diagnostics and actual error; release existing queue lease |
| Diagnostic output exceeds bound | mark truncated; native messages unchanged |

### 5. Good / Base / Bad Cases
Good: committed Tool → malformed final → isolated correction → valid DTO, effect once. Base: valid final requires no repair. Bad: Kev adds tools back during repair, or reasoning becomes user-visible text.

### 6. Tests Required
Actual HTTP Generate/Stream tests assert repair no-tools/no-choice/thinking=false/schema intact, original gates unchanged and one effect. Empty stop/length/unknown tests and queue EOF/error/cancel tests verify native identity, classification and actual terminal metadata. Tool binding shares the response pointer without mutating physical usage.

### 7. Wrong vs Correct
Wrong: clearing model.definitions alone while Kev context can rebind them. Correct: isolate correction context and physical wire options, then validate the final schema without executing capabilities.


## Scenario: Single native-publication omission correction (2026-10-12)

### 1. Scope / Trigger
A conversation Provider may ignore required native choice and return nonempty final prose before conversation.reply commits. This is distinct from correcting the final cognition JSON.

### 2. Signatures
`conversationPublicationOmission(message)` accepts nonempty/no-tool attempted finals with stop or unknown finish. The queued model shares publicationCorrectionUsed across WithTools/model copies. A correction-only physical policy sets AllowedToolNames=[conversation.reply], ToolChoiceForced and DisableThinking=true. The final payload modifier emits string tool_choice="required"; validateConversationPublicationCorrection accepts exactly one conversation.reply call.

### 3. Contracts
Initial requests keep the entire authorized selected catalog and optional read/media/persona order. At most one eligible omission triggers another physical request inside the same existing native model boundary/Runner, with fresh real identity/sequence and original authorized context. The model generates its own native arguments; no prose extraction, DTO conversion, invented ToolCall, second Agent or manual Tool loop. Tentative candidate callbacks are isolated like existing persona proposals. Only the corrected real reply call reaches Eino execution.

Correction uses reply-only native schema, string required choice, no final grammar and thinking=false for that request; normal thinking/catalog return afterward. Actual rejected arguments remain native failure feedback and may be corrected in the ordinary loop. Already committed reply, length/filter stops and empty reasoning-only finals do not trigger this correction. Wrong tool, multiple calls or another prose final fails closed; the shared one-shot budget cannot reset. Tool-free final DTO repair is unchanged and cannot publish.

### 4. Validation & Error Matrix
| Condition | Behavior |
| --- | --- |
| Prose stop without reply | one reply-only native correction with string required choice |
| Ignore correction/wrong Tool | fail, no fake publication |
| Correct Tool arguments rejected | real feedback through ordinary native continuation |
| Reply committed | auto/no final grammar with optional catalog; no native-publication correction |
| length/content_filter/empty output | retain failure, no omission correction |
| Retry has committed mutation | History blocked; unchanged enqueue replay protection |

### 5. Good / Base / Bad Cases
Good: read → premature prose → one real reply call → committed receipt → final cognition. Base: original native reply needs no correction. Bad: force reply before every possible read or publish candidate prose directly.

### 6. Tests Required
Actual HTTP/SSE Generate/Stream verifies one correction, string required choice and reply-only schema, unique physical headers, catalog/thinking restoration without final grammar, single effect, argument rejection continuation and ignored/wrong/multiple-call failure. PG zero-effect failed-turn retry reuses sender identity and creates one fresh attempt; partial mutation history agrees with admission fence.

### 7. Wrong vs Correct
Wrong: convert the stopped text into a backend send. Correct: request one actual native ToolCall from the model and keep publication/receipt authority unchanged.


## Scenario: Native Tool references and final Goal candidates (2026-10-12)

Capabilities may own a `nativeToolContract` distinct from the canonical direct
command schema. Apply its definition after Kev selection on every physical
Generate/Stream request and in run-scoped catalog/discovery. The native bridge
validates model parameters, resolves offered refs, and only then executes or
replays canonical commands; direct Owner APIs retain their existing contracts.

Native `goal.inspect(detail)` and `goal.evaluate` accept `goal_ref`. Core binds
only current Goal-kind refs with a real entity and positive frozen revision;
never resolve arbitrary raw model IDs or replace the offered revision with the
latest database revision. Native inspect queries only the offered set and emits
semantic fields with current refs/revisions, preserving list cursor/include_closed
and has_more/next_cursor. Full internal rows and receipts remain private.
Native Actor fact writes require assertion_type and omit source_message_id;
the existing trusted inbound source binding and author/invalidation checks apply.
Inference remains uncertain; this grants no historical-source write permission.

Record rejected native parameters as an invocation plus nonretryable failed
result with physical request identity and diagnostic rejection/result. Compare
original normalized model arguments, tool name and working profile for native-ID
reuse before canonical binding; changed payload conflicts, identical rejected
payload replays its failure. Accepted payload replay still validates current
binding and canonical identity. Rejected calls never execute domain writes.

Validate final goal_event_candidates count/kind/current membership before final
admission and again in settlement. The existing single tool-free DTO correction
may fix invalid refs using explicit allowed-ref feedback; it cannot replay a
committed reply/evaluation request or publish text. Queued evaluation is not
Goal completion. Scope validation does not prove semantic evidence entailment:
unrelated summaries cannot substantiate current interpretations.


## Scenario: String Tool choice on the physical request (2026-10-12)

### 1. Scope / Trigger
LM Studio rejects object-valued tool_choice. The pinned ACL OpenAI component
also automatically converts forced choice with one installed tool to an object,
even without a named function option.

### 2. Signatures
physicalModelRequestDecision owns ToolChoice/AllowedToolNames/format/thinking.
applyPhysicalModelRequestPolicy uses model.WithToolChoice plus the existing
final payload modifier. NamedTool is removed.

### 3. Contracts
Map Eino Forced/Allowed/Forbidden to wire strings required/auto/none explicitly;
the internal enum literals forced/allowed/forbidden are not wire values.
Task-owned catalog narrowing still limits installation, never grants authority.
Do not add endpoint heuristics, capability flags or HTTP400 retries. The existing
reply correction checks exactly one authorized reply; text cannot publish.
Final tool-free repair still deletes all tools/tool_choice/parallel hints.

### 4. Validation & Error Matrix
| Condition | Behavior |
| --- | --- |
| Forced + one tool | wire string required, not SDK-generated named object |
| Allowed | string auto |
| Forbidden | string none |
| Wrong tool/multiple calls in reply correction | reject before execution |
| Final DTO repair | no tools or tool_choice |

### 5. Good / Base / Bad Cases
Good: sole reply schema + required + exactly-one native validation.
Base: normal selected catalog uses task-owned choice. Bad: remove NamedTool and
assume the SDK will preserve a string without checking actual serialized wire.

### 6. Tests Required
Actual HTTP/SSE fixture rejects nonstring or unsupported choice values;
Generate/Stream read→prose→correction→reply→final verifies required-only schema,
identity, one effect, receipt rejection continuation, ignored/wrong/multiple calls.
Planner single-tool query tests accept required only. Final repair remains isolated.

### 7. Wrong vs Correct
Wrong: tool_choice={type:function,function:{name:conversation.reply}}.
Correct: tool_choice="required" with exactly one installed reply schema and
independent native response validation.

## Scenario: Separate native Tool selection from final response grammar (2026-10-13)

### 1. Scope / Trigger
LM Studio rejected a post-publication request with HTTP400 `Cannot combine
structured output constraints with lazy grammar`. Required Tool choice worked,
but optional auto choice plus response_format did not. Apply this shared rule
to all native Tool-bearing requests, including surfaces without task policies.

### 2. Signatures
`queuedToolCallingChatModel.physicalRequestOptions(ctx, opts)` reads effective
Tools through `model.GetCommonOptions`, after native/Kev disclosure and task-owned
catalog narrowing. It sets `physicalModelRequestDecision.OmitResponseFormat`
when that actual catalog is nonempty. No endpoint switch or public API is added.

### 3. Contracts
Every native Tool-bearing physical request omits response_format and uses the
same effective format for input budgeting. Required/auto, authorized catalogs,
thinking and optional continuation retain their task-owned rules. A committed
reply or accepted root does not itself end the run or remove optional Tools.
When the native Runner stops successfully, a valid final DTO passes strict Core
validation directly. An invalid DTO may use the existing single tool-free repair:
original schema, zero tools/tool_choice/parallel hints, thinking=false, retained
real Tool history/IDs, refs and cancellation; no business admission or refresh.
Failed/empty/cancelled native runs retain their error, and private publication
must already be proved by a committed native reply before DTO repair is allowed.

### 4. Validation & Error Matrix
| Condition | Behavior |
| --- | --- |
| Required or auto with effective Tools | no final response_format; optional native calls still execute |
| Tools installed only through model options | same omission; task policy presence is irrelevant |
| Explicit empty effective Tool override | retain ordinary tool-free final format |
| Successful stopping candidate satisfies schema | accept without another model call |
| Malformed final candidate | at most one isolated schema repair; strict validation still applies |
| Invalid repair, provider failure or cancellation | actual error; prior Tool commits preserved |
| Reasoning-only empty/length/filter outcome | existing failure semantics, no added retry |

### 5. Good / Base / Bad Cases
Good: required reply → committed receipt → optional read under auto/no-format →
valid final DTO, or one no-Tool schema correction. Base: a naturally tool-free
Agent retains its original structured request. Bad: retry the rejected mixed
request, drop optional Tools immediately after the reply, or publish final prose.

### 6. Tests Required
`TestToolFinalSchemaSeparationPhysicalRequests` uses the actual SDK/Eino HTTP/SSE
path with a service rejecting auto+schema. Cover Generate/Stream, policy/no-policy,
valid/repair/invalid repair, exactly one reply effect, optional read after reply,
paired native results, unique physical identities and repair isolation.
`TestToolFinalSchemaSeparationEffectiveFormat` covers bound/option/empty catalogs,
format immutability and policy suppression. Planner optional commits/snapshot
rereads/native-ID replay, Goal coverage, Visual Identity and omission-correction
tests must retain their effect checks. Controlled fixtures recognize canonical
catalogs in both required and auto phases; they never create calls from content.

### 7. Wrong vs Correct
Wrong: restore auto and final schema in the same request after required effects.
Correct: keep native decisions free of final grammar, validate the eventual DTO,
and request schema-constrained correction only through the isolated no-Tool path.
