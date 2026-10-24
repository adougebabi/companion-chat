# Fact-only Reply Contract

2026-10-14 availability amendment: apply [Tool Availability](./tool-availability-contract.md).
Optional evidence hints that cannot be reconciled are excluded with safe
counts/reasons while the independent Writer continues from current authorized
context. Optional private draft pointers likewise cannot stop the current reply:
ignore a mismatch and use only Core's current canonical draft. Keep exact
offered/live evidence matching and actual draft/action ownership; no rejected
pointer authorizes external content. Published/prepared replay remains first,
and real Provider failures remain failures. `reply.evidence.reconciled` must be
visible in worker logs before the generation lease, without raw refs or body.

## 1. Scope / Trigger
conversation.reply in a confirmed direct conversation uses fact_only when the setting is absent or the stored object omits mode. A stored mode of legacy or fact_only is kept; runtime_settings rows are not rewritten. Ordinary cognition and authorized WakeUp keep the existing native Eino loop. Other channels keep their existing implementation.

## 2. Signatures
`withPrivateReplyRun(ctx,scope,input)` fixes settings/assignment and an immutable registry view. `RunConversationReplyTask(ctx,ConversationReplyTaskInput)` owns the isolated Writer prompt and output decoder. `factOnlyReplyCapability.Prepare` generates outside the mutation transaction; `ExecuteDirectTx` uses existing publication and receipt settlement. Schema head 0061 adds private conversation_reply_drafts and conversation_messages.reply_metadata.

## 3. Contracts
The one run contract accepts refs rather than parent text/direction/tone/constraints. Writer persona/Goals/state/history are authority-owned, never a parent transcript or response plan. Required Goal/current input and the two newest whole history groups cannot be silently trimmed; older user and assistant turns are optional candidates, not all required. Clone registry definitions and implementations together; do not mutate App.Capabilities or only change a native wire schema.

Logical reply identity uses inbound source, otherwise the fixed run root, not physical Tool position. Durable generation leases fence concurrent/late candidate writes; source and publication permissions remain authoritative. Published replay consumes the stored message without calling an unavailable model. Actual committed text returns to the parent; completed means visible_output_committed, not external network delivery or Goal acceptance.

Child context isolates parent publication/Kev/refresh/diagnostic state while retaining cancellation and logical/source guards. Each physical request shares generated capacity; Writer adds bounded task capacity through the existing queue/Redis lease. Independent route identities must not split the shared generated bucket.

Default failure is explicit. Upstream HTTP 500/502/503/504 and gateway get_channel_failed retry at most twice around that physical model call, with exponential backoff, jitter, and the same idempotency key. The reply task itself is not repeated, so tools and other side effects do not run again. 4xx, quota, truncation, input-too-large and OOM stay terminal. Envelope correction is bounded once. Explicit legacy_fallback selects the existing cognitive model for an isolated fallback, uses the same publication key, records actual binding/model and mode, and cannot count as fact-only acceptance. It cannot retry the same failed model or send after parent cancellation.

## 4. Validation & Error Matrix
Foreign run scope or actual action/resource ownership: reject. Mismatching
optional draft hint: warning plus current canonical recovery. Large valid input: complete request regardless of legacy token fields. Child inherits required tool policy: contract failure. Model length stop:
incomplete error. Context changed after generation: reject stale candidate.
Cancelled source or lost lease: no publication. Same scoped operation/payload:
replay; changed payload: conflict. Final cognition failure does not erase a
committed Tool message.

## 5. Good / Base / Bad Cases
Good: Qwen native refs → independent Writer → committed message/receipt → actual text feedback → final cognition. Base: legacy skips Writer. Bad: Qwen writes a draft for Cydonia to polish, suppresses an active Goal via a matched inference, puts hidden reasoning in history, or sends a late second reply.

## 6. Tests Required
Local HTTP/Eino Generate/Stream tests cover frozen model/sampling and omitted output caps, parent-context isolation, nested capacity=1, actual text feedback, malformed envelope, truncation, OOM/nonretry, bounded transient retry and explicit fallback. Immutable registry/discovery tests cover both contracts and forbidden arguments. Source/run logical-key tests reject physical-position identity. PostgreSQL sender/receipt/replay/authority integration remains opt-in; mock publication is not production or real-model acceptance.

## 7. Wrong vs Correct
Wrong: use the final cognition text as a message, overwrite registry per request, or choose a new generated body after delivery. Correct: capability-owned preparation, immutable per-run execution view, stable private draft, short message+receipt transaction and real result feedback.

## Media continuation (0062)
A private draft owns opaque component refs. The scoped image wrapper preserves the Writer's exact intent and existing Capture/Kev pipeline, fixes external operation identity to the component, and binds its durable media intent in the same transaction. Both media ready paths skip legacy publication for draft-bound intents. Pending has no visible prose. Terminal results receive one cached result-aware Writer adjustment; ready bytes never imply visual inspection. Text and real attachment refs commit together through the existing publisher.

Final cognition settles once into `waiting_reply`, releases its inbox lease and creates a continuation in a separate table. Waiting replay cannot reclaim the original cognition. A newer same-conversation turn or cancellation closes its authority; unrelated conversations are not blocked by waiting. The dedicated `reply-continuation-v1` Temporal queue and independent dispatcher are invisible to old platform-intent SQL. Stable-ID start/reconcile, expiry and bounded retries cover missed media callbacks and worker crashes. Resume uses the original source, canonical turn correlation, fixed config/model parameters, restricted native reply Tool and a publication-time durable authority check. WakeUp also rechecks autonomy, awake state, topic and absence of a newer inbound user message. Invalid original final cognition explicitly fails unpublished drafts; it cannot authorize a late reply. Published message and continuation/inbox settlement share the publication transaction.

Draft-owned media has no legacy conversation/message/moment publication target. This protects rollback even if an old MediaWorkflow processes the generation. Before stable cognition workers resume, drain/cancel new continuations and terminalize waiting_reply; an old claimant does not understand that new nonterminal state. Actual fallback assignment is captured once, persists without its secret, and is reused for result-aware adjustment. Native evidence must match the exact offered scope/entity/revision/snapshot as well as live authority. Media error and failed result both become nonretryable so the native loop can hand the failure to Writer; a wrapped error alone is insufficient.

For dynamic PostgreSQL inbox settlement, explicitly cast a reused status parameter consistently in both varchar assignment and CASE comparison. Untyped reuse can fail with 42P08 while an already committed reply masks the failure through degraded recovery. The controlled PG test must assert processed inbox plus empty error_code, not only a returned assistant message.

## Writer context and image requests (2026-10-13)
Writer uses ResolveWorkingMemory then AssemblePromptContext, like cognition.
Follow [LLM Prompt Composition](./prompt-composition-contract.md): no input/output
caps or section token targets. Reuse persisted runtime summaries without another
model call. Only selected source coverage removes whole covered optional turns;
retain protected recent groups, original roles/text, current input once and
uncertainty. Record candidate/selected counts, deduplication and actual wire
metrics; estimates do not prove actual tokenizer usage or latency.

When image generation is installed, first Writer requests offer only private native reply.image.request(intent), max two handoffs. This task-local audited/native receipt records requested_not_generated, not generated bytes or publication. The common formal Runner validates a private scoped registry and accepts a typed task-owned Invoker; no ambient executors/tools/refresh are inherited. Handoffs enter the existing draft-owned generation/source/permission/Kev/attachment pipeline. Media-result adjustment removes the image tool and cannot request new images. Legacy prepared envelopes remain decodable; fresh media handoffs require native receipts, including on a bounded correction. Output correction is a separate required protocol rule and never changes or duplicates CurrentInput. Reject fabricated path data and send-image action markers without actual ready media, with one bounded correction. Normalize output paragraphs outside code fences: collapse excess actual/literal escaped blank lines, preserve ordinary Unicode prose and code escapes. Raw physical diagnostics remain raw; prepared/published text is normalized.

Evidence refs are optional actionable offered ref fields, not historical message/source_refs. Resolve live projection and validate exact offered scope/entity/revision/snapshot before a durable generation lease. Correctable input rejection cannot mark a candidate failed. Prepared/published replay remains first; actual Writer failure remains same-run terminal with source/lease/cancellation fences.

## Shared Writer for Moments (2026-10-14)
In fact_only mode, moment.publish uses the same frozen conversation_reply model/settings/failure policy and isolated Writer as private replies. Its scene is explicitly a feed Moment, not private-chat continuation. Native arguments are optional offered evidence_refs, never parent text/strategy. Install the run-local replacement for Conversation/WakeUp/Autonomy, including no conversation ID; keep legacy text schema when mode=legacy. Shared model binding is frozen once, but reply and moment execution progress/namespaces are separate.
Prepare outside mutation transactions; validate live/offered refs, build the current persona/facts/Goals through the task-owned semantic view, then publish through the existing stable moment operation, receipt ledger and outbox. Recheck facts and life revision at commit. Committed receipt replay precedes Writer I/O. A run-local candidate cache preserves the same operation body and bounded failure within that live run; it is not a durable prepublication generation checkpoint, so a process crash before commit may regenerate. No new migration or private conversation continuation state is used.
Moment Writer only produces text. Private reply.image.request/needs_action cannot be used as Moment media authority. Existing authorized media tools can attach real assets to a committed, owned Moment target. Return actual committed Writer text to cognition. Preserve original Goal conditions and mark history/private chat as data; do not quote private conversation wholesale into a feed.

Moment prepublication cache command identity includes subject actor, source/evidence fact, conversation, working profile and canonical arguments; physical/native call IDs are excluded. A changed command under one operation conflicts before regeneration or publication, matching the durable Tool ledger.

### R5: completed feed action and private acknowledgement

An implicit Moment action in a private Conversation is source/action-scoped,
including the adapter's default `conversation` target. Fresh physical call
coordinates cannot grant another publication. Empty/null/omitted optional
evidence selections are equivalent; real changed commands remain conflicts.
Explicit Owner targets and autonomous scopes keep their own action authority.

A real completed Moment never becomes a conversation_message receipt. It closes
the feed action and narrows the unfinished private acknowledgement to
conversation.reply. A real needs_action reply reopens its required action catalog
while excluding the completed Moment; waiting keeps the existing continuation.
Provider attempts to republish after commit fail before native execution. Normal
private reply commit still ends observation locally. Preserve all actual receipts
on cancellation/failure; do not invent acknowledgment or use a global rate cap.
`r5_moment_loop_test.go` proves Generate/Stream, ignored restricted catalog,
real PG publication/outbox, source replay before Writer and new-source admission.

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
- Added metadata must be tested in the final physical request. Keep provenance compact; passing builder-only assertions does
  not prove a candidate survived WorkingMemory/Composer. Required Goal/history
  values use lossless copies without token-based refusal or truncation.
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
| Required criteria/trigger history exceeds legacy token settings | retain complete content, no local budget error |
| Old/new purpose differs but same scoped Fact-only proactive topic | suppress using existing message metadata; real stored text returned |
| Missing trigger source or unproven heading origin | report gap/preserve text; no invented correction |
| Published reply plus invalid final JSON | one tool-free repair; failed settlement retains committed message and error |

### 5. Good / Base / Bad Cases
Good: scoped original Goals + source-tagged inference + real metadata → autonomous
Writer → one committed reply → separate cognition settlement. Base: no reliable
recipient name remains unnamed. Bad: drop the real relationship by reading id,
discard inference because of metadata size, forward a Goal snapshot's
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

## Scenario: Private chat expression v5 and durable single style repair

### 1. Scope / Trigger
Only the validated private Fact-only Writer and its necessary continuation paths
use v5. Moments, legacy, media intents, visual analysis and cognition JSON keep
their own output contracts. Original Persona and sent history are immutable.

### 2. Signatures
`conversation.reply.expression_version` is optional: new runs default to
`fact-only-reply.v5`; explicit `fact-only-reply.v4` rolls back expression without
closing Fact-only. Unknown/blank/null versions reject the settings patch.
`binding_snapshot.reply_contract` stores version, protocol_hash, view_version,
medium_version. Old durable snapshots without it use v4 compatibility.
`binding_snapshot.style_repair_claimed/style_candidate` owns the lineage quota;
`ReplyDraft.style/candidates` retains first, repair and final text.

### 3. Contracts
- Private operation rules replace the old protocol once. Persona portrait_text
  remains verbatim with interpretation_boundary; source/compiler/canonical
  Persona/raw-view/reply-view hashes stay in Owner diagnostics, not extra traits.
  For old structured cards, derive the existing body before adding the boundary;
  otherwise an almost-empty body shadows personality/behavioral_policy.
- Validated private entry projects text_chat to cognition and Writer. Current
  co_located facts can provide same/different place with epistemic/source/time
  metadata; online, meeting plans and relationship background cannot. Doorway
  location statements stay source-qualified, not verified co-presence. Current
  Actor fact provenance is enriched locally without changing the public query.
- Ordinary Writer omits unrelated appearance only when the actual scoped
  persona.detail query is installed. Reuse its canonical owner/profile/version
  checks and the fresh child trace. When tools are closed for adjustment/repair,
  keep necessary authority appearance; image execution always keeps full state.
- History remains original role/text/order. Metadata says past_message_verbatim
  is not current event proof and past style is not a format rule. Unmarked mixed
  narrative/dialogue is not split or rewritten. Goal intent/criteria stay intact.
- High-confidence multi-signal non-requested narration gets at most one repair
  by the same frozen Cydonia, before publication, with zero tools. A name/pronoun,
  quote, action word, long answer or refusal alone cannot reject. Ambiguous style
  is observation only; explicit works and their immediate continuations are
  allowed. This detector is not a semantic truth/personality judge.
- Claim quota by one SQL conditional update with draft/owner/active lease/status
  and unclaimed flag. Do not use resume_attempts or an in-memory bool. Claim DB
  errors and repair failures are style errors: no whole-generation retry or Qwen
  fallback. Reject OOM before HTTP transient classification. Explicit fallback
  generation remains separately labeled and cannot do Cydonia style repair.
- Initial/failed lease recovery and media restore keep stored configuration and
  assignment, not current settings. JSONB binding updates merge only effective
  assignment fields and never reset contract/quota. Media fallback ready CAS
  must store its actual no-secret binding, just like initial preparation.
- Production and replay share decodeReplyCompletion finish/native media/secret
  validation. Preserve failed repair raw text, finish reason and underlying error
  code; candidates never become sent history or state evidence. The publisher
  remains unique and Qwen settles actual committed text, never the first draft.

### 4. Validation & Error Matrix
| Condition | Outcome |
| --- | --- |
| Frozen protocol hash/known view differs | reply_expression_snapshot_mismatch; no silent current-version substitution |
| Quota already claimed, lease lost or claim DB error | reply_style_repair_unavailable; no Qwen fallback |
| Repair 503/OOM/timeout, invalid output or remaining high narration | explicit failure; no second repair or regenerated first draft |
| finish_reason is not stop | reply_generation_incomplete, raw/finish preserved; no replay success |
| JSON requests without native receipt | reply_media_request_invalid; no synthetic media |
| Detail reader unavailable or tool-free phase | authority facts retained, not an unusable on-demand placeholder |
| published replay / malformed final cognition | read actual message; schema repair cannot generate or publish again |

### 5. Good / Base / Bad Cases
Good: unchanged literary portrait + unchanged dirty history + sourced text-chat
view -> Cydonia message -> optional one repair -> real media/result -> one commit.
Base: normal/uncertain style adds no model call. Bad: create a new narrator draft
after a repair 503, lose old structured traits, rebind waiting media to new model,
or label an old cached body with the current protocol constant.

### 6. Tests Required
`TestPrivateChat*` covers final wire protocol-only comparison, sources, original
Persona/history hashes, actual installed tools, closed repair, 503/OOM/claim DB
failure, raw incomplete output and effective binding merge. Durable quota and
committed repair + malformed settlement tests require disposable PostgreSQL.
Controlled A/B/C/D replay is reconstructed transport evidence; live ablation
and multiround/voice tests are opt-in and retain failed samples. SKIP/NOT_RUN
cannot certify persistent CAS, real model style or personality/Goal behavior.

### 7. Wrong vs Correct
Wrong: publish metadata reads the current constant, or generic transient handling
unwraps a style-503 error and retries the entire Writer. Correct: read the draft
contract, atomically spend one quota, classify style/OOM before retries, and merge
actual binding into the same durable draft without touching original history.

## Scenario: Optional draft pointer recovery (F20)

### 1. Scope / Trigger
Private Fact-only conversation.reply previously failed reply_draft_scope_invalid
before querying its own draft whenever the model supplied a stale/foreign hint.
The optional hint must not revoke the current reply's independent authorization.

### 2. Signatures
`reconcileReplyDraftHint(canonical, hint)` returns safe warning metadata.
`decodePrivateReplyArguments` allows unresolved optional draft/evidence aliases
only at the private native reply boundary. `Prepare` freezes optional warnings;
`ExecuteDirectTx` carries them to actual success/replay/media receipts.

### 3. Contracts
Canonical identity remains privateReplyLogicalKey(source or fixed run root,
Fluctlight, conversation, authorization Actor). Never query a draft using the
model hint. On mismatch emit reply.draft.reconciled and receipt warnings:
warning_code=reply_draft_hint_ignored, disposition=continue_canonical_draft,
reason=not_current_draft, requested_count=ignored_count=1. No raw pointer/body,
asset, configuration or secrets appear in this reconciliation metadata.

Prepared/published cache precedes model availability; cached recovery and
publication replay do not call Writer again. needs_action/waiting_media/ready
continue only canonical draft actions. The program does not infer that a hint
is stale versus foreign by reading it. Action/target aliases and actual action
ownership retain strict checks; Moments and legacy keep their prior decoder.
Source closure, cancellation, Provider/DB failure, profile/lease/facts and v5
style quota/frozen binding remain authoritative. A warning is not a committed
effect; report the actual committed or pending reply status.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| absent/current draft hint | normal path, no recovery warning |
| stale/foreign/arbitrary hint | ignore hint, warn, use current authorized draft |
| unknown private draft alias | reach reconciliation, never decode into authority |
| published current draft with unavailable Writer | stored message replay, no generation |
| foreign media action / unknown required alias | actual resource refusal; no foreign media use |
| closed source / cancellation / actual Provider or DB error | truthful failure, no invented publication |

### 5. Good/Base/Bad Cases
Good: wrong hint + current source -> one independent Writer -> one message;
wrong hint again -> cached/published replay. Base: matching pointer adds nothing.
Bad: select the hinted foreign row, regenerate a published reply, or return a
handled receipt instead of executing a valid current reply.

### 6. Tests Required
`TestReplyDraft*` covers native codec separation, safe persisted diagnostics,
actual Prepare/publication/schema warning, foreign row isolation, one Writer and
one message, replay despite unavailable Writer, current pending/waiting/ready
media, closed source, cancellation and real closed-pool DB failure. Run the six
PG cases against a task-owned disposable instance; SKIP cannot certify effects.

### 7. Wrong vs Correct
Wrong: draft_ref mismatch -> ErrUnauthorized -> no current reply.
Correct: ignore the optional hint with a safe warning; resolve only the Core-owned
canonical draft, then keep the existing generation/publication/effect checks.

## Output quality: isolated unexpected script tail (2026-10-23)

The observed short Gujarati suffix on otherwise Chinese prose is checked before
publication. Names/quotes present in authoritative input, explicit language
requests, emoji, and genuinely multilingual output remain intact. A detected
isolated suffix requests one same-model, same-facts, tool-free correction through
the existing output-correction channel. Repair does not publish the candidate,
run tools again, change speaker/settings, or silently strip characters. A repeated
invalid suffix or failed correction returns `reply_quality_repair_failed` with
the original draft/media requests retained. Failure policy must return this
typed error before format correction or legacy fallback; physical transport
retries remain inside the same tool-free call. It cannot reopen Writer tools
or replace the original media request. `reply_output_quality_test.go` verifies successful
and exhausted repair plus valid contextual multilingual text. This narrow guard
is not a blanket language ban or evidence of a specific Provider's root cause.
