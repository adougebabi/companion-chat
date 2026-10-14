# Fact-only Reply Contract

## 1. Scope / Trigger
Opt-in conversation.reply in a confirmed direct conversation; ordinary cognition and authorized WakeUp keep the existing native Eino loop. Other channels keep their existing implementation. Missing settings means legacy.

## 2. Signatures
`withPrivateReplyRun(ctx,scope,input)` fixes settings/assignment and an immutable registry view. `RunConversationReplyTask(ctx,ConversationReplyTaskInput)` owns the isolated Writer prompt and output decoder. `factOnlyReplyCapability.Prepare` generates outside the mutation transaction; `ExecuteDirectTx` uses existing publication and receipt settlement. Schema head 0061 adds private conversation_reply_drafts and conversation_messages.reply_metadata.

## 3. Contracts
The one run contract accepts refs rather than parent text/direction/tone/constraints. Writer persona/Goals/state/history are authority-owned and budgeted, never a parent transcript or response plan. Required Goal/current input and the two newest whole history groups cannot be silently trimmed; older user and assistant turns are optional candidates, not all required. Clone registry definitions and implementations together; do not mutate App.Capabilities or only change a native wire schema.

Logical reply identity uses inbound source, otherwise the fixed run root, not physical Tool position. Durable generation leases fence concurrent/late candidate writes; source and publication permissions remain authoritative. Published replay consumes the stored message without calling an unavailable model. Actual committed text returns to the parent; completed means visible_output_committed, not external network delivery or Goal acceptance.

Child context isolates parent publication/Kev/refresh/diagnostic state while retaining cancellation and logical/source guards. Each physical request shares generated capacity; Writer adds bounded task capacity through the existing queue/Redis lease. Independent route identities must not split the shared generated bucket.

Default failure is explicit. Only recoverable transport errors retry within configured bounds; truncation/input-too-large/OOM are not retried unchanged. Envelope correction is bounded once. Explicit legacy_fallback selects the existing cognitive model for an isolated fallback, uses the same publication key, records actual binding/model and mode, and cannot count as fact-only acceptance. It cannot retry the same failed model or send after parent cancellation.

## 4. Validation & Error Matrix
Foreign run scope/ref/draft: reject. Required overflow: no HTTP. Child inherits required tool policy: contract failure. Model length stop: incomplete error. Context changed after generation: reject stale candidate. Cancelled source or lost lease: no publication. Same scoped operation/payload: replay; changed payload: conflict. Final cognition failure does not erase a committed Tool message.

## 5. Good / Base / Bad Cases
Good: Qwen native refs → independent Writer → committed message/receipt → actual text feedback → final cognition. Base: legacy skips Writer. Bad: Qwen writes a draft for Cydonia to polish, suppresses an active Goal via a matched inference, puts hidden reasoning in history, or sends a late second reply.

## 6. Tests Required
Local HTTP/Eino Generate/Stream tests cover frozen model/sampling/cap, parent-context isolation, nested capacity=1, actual text feedback, malformed envelope, truncation, OOM/nonretry, bounded transient retry and explicit fallback. Immutable registry/discovery tests cover both contracts and forbidden arguments. Source/run logical-key tests reject physical-position identity. PostgreSQL sender/receipt/replay/authority integration remains opt-in; mock publication is not production or real-model acceptance.

## 7. Wrong vs Correct
Wrong: use the final cognition text as a message, overwrite registry per request, or choose a new generated body after delivery. Correct: capability-owned preparation, immutable per-run execution view, stable private draft, short message+receipt transaction and real result feedback.

## Media continuation (0062)
A private draft owns opaque component refs. The scoped image wrapper preserves the Writer's exact intent and existing Capture/Kev pipeline, fixes external operation identity to the component, and binds its durable media intent in the same transaction. Both media ready paths skip legacy publication for draft-bound intents. Pending has no visible prose. Terminal results receive one cached result-aware Writer adjustment; ready bytes never imply visual inspection. Text and real attachment refs commit together through the existing publisher.

Final cognition settles once into `waiting_reply`, releases its inbox lease and creates a continuation in a separate table. Waiting replay cannot reclaim the original cognition. A newer same-conversation turn or cancellation closes its authority; unrelated conversations are not blocked by waiting. The dedicated `reply-continuation-v1` Temporal queue and independent dispatcher are invisible to old platform-intent SQL. Stable-ID start/reconcile, expiry and bounded retries cover missed media callbacks and worker crashes. Resume uses the original source, canonical turn correlation, fixed config/model parameters, restricted native reply Tool and a publication-time durable authority check. WakeUp also rechecks autonomy, awake state, topic and absence of a newer inbound user message. Invalid original final cognition explicitly fails unpublished drafts; it cannot authorize a late reply. Published message and continuation/inbox settlement share the publication transaction.

Draft-owned media has no legacy conversation/message/moment publication target. This protects rollback even if an old MediaWorkflow processes the generation. Before stable cognition workers resume, drain/cancel new continuations and terminalize waiting_reply; an old claimant does not understand that new nonterminal state. Actual fallback assignment is captured once, persists without its secret, and is reused for result-aware adjustment. Native evidence must match the exact offered scope/entity/revision/snapshot as well as live authority. Media error and failed result both become nonretryable so the native loop can hand the failure to Writer; a wrapped error alone is insufficient.

For dynamic PostgreSQL inbox settlement, explicitly cast a reused status parameter consistently in both varchar assignment and CASE comparison. Untyped reuse can fail with 42P08 while an already committed reply masks the failure through degraded recovery. The controlled PG test must assert processed inbox plus empty error_code, not only a returned assistant message.

## Writer context and image requests (2026-10-13)
Writer uses ResolveWorkingMemory then AssemblePromptContext, like cognition. Own soft input target: min(8000, assignment hard max), recent section 2400 estimated tokens, retrieved memory 512. Protected recent groups and authoritative persona/Goal/current state may exceed the soft target but never the hard wire limit; required overflow is explicit. Reuse persisted runtime summaries by authorized read only, not an extra model call. Summary source refs deduplicate covered optional whole turns. Preserve original roles/text, current input once, uncertainty on summaries/memories. Record candidate/selected history chars/counts, working-memory/assembly drops and final wire metrics. Global estimator remains conservative; provider prompt usage is measured independently and synthetic estimates cannot prove real latency.

When image generation is installed, first Writer requests offer only private native reply.image.request(intent), max two handoffs. This task-local audited/native receipt records requested_not_generated, not generated bytes or publication. The common formal Runner validates a private scoped registry and accepts a typed task-owned Invoker; no ambient executors/tools/refresh are inherited. Handoffs enter the existing draft-owned generation/source/permission/Kev/attachment pipeline. Media-result adjustment removes the image tool and cannot request new images. Legacy prepared envelopes remain decodable; fresh media handoffs require native receipts, including on a bounded correction. Output correction is a separate required protocol rule and never changes or duplicates CurrentInput. Reject fabricated path data and send-image action markers without actual ready media, with one bounded correction. Normalize output paragraphs outside code fences: collapse excess actual/literal escaped blank lines, preserve ordinary Unicode prose and code escapes. Raw physical diagnostics remain raw; prepared/published text is normalized.

Evidence refs are optional actionable offered ref fields, not historical message/source_refs. Resolve live projection and validate exact offered scope/entity/revision/snapshot before a durable generation lease. Correctable input rejection cannot mark a candidate failed. Prepared/published replay remains first; actual Writer failure remains same-run terminal with source/lease/cancellation fences.

## Shared Writer for Moments (2026-10-14)
In fact_only mode, moment.publish uses the same frozen conversation_reply model/settings/failure policy and isolated Writer as private replies. Its scene is explicitly a feed Moment, not private-chat continuation. Native arguments are optional offered evidence_refs, never parent text/strategy. Install the run-local replacement for Conversation/WakeUp/Autonomy, including no conversation ID; keep legacy text schema when mode=legacy. Shared model binding is frozen once, but reply and moment execution progress/namespaces are separate.
Prepare outside mutation transactions; validate live/offered refs, build the current persona/facts/Goals with the existing budget, then publish through the existing stable moment operation, receipt ledger and outbox. Recheck facts and life revision at commit. Committed receipt replay precedes Writer I/O. A run-local candidate cache preserves the same operation body and bounded failure within that live run; it is not a durable prepublication generation checkpoint, so a process crash before commit may regenerate. No new migration or private conversation continuation state is used.
Moment Writer only produces text. Private reply.image.request/needs_action cannot be used as Moment media authority. Existing authorized media tools can attach real assets to a committed, owned Moment target. Return actual committed Writer text to cognition. Preserve original Goal conditions and mark history/private chat as data; do not quote private conversation wholesale into a feed.

Moment prepublication cache command identity includes subject actor, source/evidence fact, conversation, working profile and canonical arguments; physical/native call IDs are excluded. A changed command under one operation conflicts before regeneration or publication, matching the durable Tool ledger.

## Scenario: Local fact views and native reply v2 (2026-10-14)

### 1. Scope / Trigger
Private Fact-only cognition and Writer must preserve persona, Goals and facts
without forwarding historical execution advice. This supersedes the earlier
private Writer's use of a common cognition compact followed by recursive key
removal. Moments retains its existing scene-owned context selection.

### 2. Signatures
`factOnlyContextFacts`, `factOnlyCognitionMemoryInput`, `factOnlyHistory` and
`factOnlyMemoryCandidates` build call-local read models from ContextProjection.
`backfillFactOnlyHistory` reads a leading assistant's actual stored source fact.
Native conversation.reply version `fact_only.v2` has closed optional fields
evidence_refs, draft_ref, action_result_refs and topic_key. Model purpose is
absent; publication uses Core-owned independent_persona_reply. Prompt version is
fact-only-reply.v4. No persistence migration or new product setting is required.

### 3. Contracts
- Production Actor maps use actor_id, not id. Keep self Actor, effective profile
  and recipient separate; semantic actor aliases identify roles, never nicknames.
  Apply existing profile/shared and recipient visibility before projecting Goals
  and relationships. Preserve original Goal intent, motivation, criteria, status
  and real deadline. next_step/strategy/wait/blocker prose is not an objective
  dependency and cannot enter Writer, including through a valid Goal ref snapshot.
- Goal dependency_snapshot is a read-only status snapshot, not new admission
  authority. Prerequisite completion and context-review state do not replace the
  actual execution guard. Old shared compaction omits the added snapshot field.
- Now comes from as_of/Clock; history comes from stored metadata. Render selected
  history metadata separately from unchanged role/content and preserve original
  sender timestamps plus frozen zone/offset. An unproven Markdown time heading
  stays text. Expiry is field/source-local; unknown, cleared and false differ.
- Group consecutive real replies with their triggering user input. A bounded
  window starting with assistant may backfill only the real same-conversation,
  same-source_fact_id earlier user; mark possible missing intermediate messages.
  Missing source means unanchored, never a fabricated user.
- verified/source_validity prove source liveness, not semantic entailment.
  Preserve epistemic kind, associated Actors, time/scope and uncertainty; exact
  duplicates keep every internal record/source pointer. Same source with distinct
  claims is unresolved, not independent support. Do not use fuzzy semantic merges,
  generic unescaping or regex deletion of eN-like prose/model names.
- Added metadata must be tested at final wire selection under the existing
  section budget. Keep provenance compact; passing builder-only assertions does
  not prove a candidate survived WorkingMemory/Composer. Required Goal/history
  values use lossless copies and explicit budget refusal, not Tool-snapshot caps.
- Referenced entity snapshots do not bypass dedicated views. Private Writer
  accepts only typed statement/outcome additions plus draft-owned media/candidate
  facts; parent reasoning, final text and arbitrary extra fact kinds stay out.
- Old drafts keep stored purpose/topic_key Core-side for continuation. Trusted
  FactOnlyTopicSuppression bridges same-topic/window/inbound-sequence deliveries
  across purposes only when the prior message's reply_metadata says fact_only or
  legacy_fallback. Unrelated legacy callers retain exact-purpose matching.
  Suppressed publication returns the actual stored message text, never the new
  uncommitted candidate.
- Post-commit cognition replaces the sending protocol with settlement guidance
  while preserving authorized optional Tools. All Tool-bearing physical calls
  still omit response_format. Only the existing single tool-free final repair
  binds the complete schema and masks business/Kev/refresh; no second publication.

### 4. Validation & Error Matrix
| Condition | Outcome |
| --- | --- |
| New native purpose/text/tone/unknown or wrongly typed nested input | closed-schema rejection, no Writer I/O |
| Valid Goal ref contains execution advice | validate ref, use the independent visible-Goal view; no snapshot prose forwarding |
| Required criteria/trigger history cannot fit | explicit budget error, no silent truncation |
| Old/new purpose differs but same scoped Fact-only proactive topic | suppress using existing message metadata; real stored text returned |
| Missing trigger source or unproven heading origin | report gap/preserve text; no invented correction |
| Published reply plus invalid final JSON | one tool-free repair; failed settlement retains committed message and error |

### 5. Good / Base / Bad Cases
Good: scoped original Goals + source-tagged inference + real metadata → autonomous
Writer → one committed reply → separate cognition settlement. Base: no reliable
recipient name remains unnamed. Bad: drop the real relationship by reading id,
discard inference because metadata exceeded its cap, forward a Goal snapshot's
strategy, or use changed purpose to bypass proactive suppression.

### 6. Tests Required
TestFactOnlyMorningContextRegression and related view tests use the production
Actor shape and assert final selected input, source immutability, false/unknown,
scope, exact dedup, sender zone and whole-trigger preservation. Actual HTTP/SSE
TestToolFinalSchemaSeparationPhysicalRequests includes syntactically broken JSON,
phase/persona preservation, effective config equality and one reply effect.
TestFactOnlyCommittedReplyMalformedFinalSettlement and
TestFactOnlyProactiveSuppressionAcrossPurposeVersions require disposable PG;
their SKIP is not persistent acceptance. Baselines are explicitly synthetic,
hash-frozen and reconstructed where needed; they are not original incident wire.

### 7. Wrong vs Correct
Wrong: CurrentSpeaker["id"], Writer = common runtime minus a few keys, or verified
inference becomes confirmed fact. Correct: actor_id-scoped dedicated views,
independent source/epistemic semantics, final selected-input assertions and actual
receipt/message authority after bounded repair.
