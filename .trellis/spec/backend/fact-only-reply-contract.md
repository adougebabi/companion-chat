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
