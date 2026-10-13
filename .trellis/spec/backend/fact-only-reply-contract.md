# Fact-only Reply Contract

## 1. Scope / Trigger
Opt-in conversation.reply in a confirmed direct conversation; ordinary cognition and authorized WakeUp keep the existing native Eino loop. Other channels keep their existing implementation. Missing settings means legacy.

## 2. Signatures
`withPrivateReplyRun(ctx,scope,input)` fixes settings/assignment and an immutable registry view. `RunConversationReplyTask(ctx,ConversationReplyTaskInput)` owns the tool-free Writer prompt and output decoder. `factOnlyReplyCapability.Prepare` generates outside the mutation transaction; `ExecuteDirectTx` uses existing publication and receipt settlement. Schema head 0061 adds private conversation_reply_drafts and conversation_messages.reply_metadata.

## 3. Contracts
The one run contract accepts refs rather than parent text/direction/tone/constraints. Writer persona/Goals/state/history are authority-owned and budgeted, never a parent transcript or response plan. Required Goal/current input/refusal content cannot be silently trimmed. Clone registry definitions and implementations together; do not mutate App.Capabilities or only change a native wire schema.

Logical reply identity uses inbound source, otherwise the fixed run root, not physical Tool position. Durable generation leases fence concurrent/late candidate writes; source and publication permissions remain authoritative. Published replay consumes the stored message without calling an unavailable model. Actual committed text returns to the parent; completed means visible_output_committed, not external network delivery or Goal acceptance.

Child context isolates parent publication/Kev/refresh/diagnostic state while retaining cancellation and logical/source guards. Each physical request shares generated capacity; Writer adds bounded task capacity through the existing queue/Redis lease. Independent route identities must not split the shared generated bucket.

Default failure is explicit. Only recoverable transport errors retry within configured bounds; truncation/input-too-large/OOM are not retried unchanged. Envelope correction is bounded once. Explicit legacy_fallback selects the existing cognitive model for a tool-free fallback, uses the same publication key, records actual binding/model and mode, and cannot count as fact-only acceptance. It cannot retry the same failed model or send after parent cancellation.

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
