# Tool Availability Contract

## R6: scoped discovery must survive narrowed registries

An Owner life request using deferred tool disclosure must install the read-only
capability.catalog in its scoped registry. Discovery reads that registry, never
the global mutation catalogue. Exact-target action wrappers, CAS and refusal
remain unchanged. A registry with no public baseline and an empty loaded window
otherwise silently offers zero Tools, even when its text mentions wardrobe.wear.

Native Provider fixtures must check the actual outgoing tools before returning
a ToolCall. Test discovery→load→execution→durable receipt→readback, plus rejection,
no-effect claims, stale targets and replay. Returning a pre-scripted wear call
when its schema was absent is not end-to-end evidence. See
`docs/design/v62-r6-wear-remediation.md` and `r6_wear_contract_test.go`.

R7 separates protocol reachability from model behavior. A scripted Provider that
chooses catalog/inspect/wear only proves execution of that script. Live acceptance
must start with natural language, use actual physical schemas and returned IDs,
and compare the final worn set with the prior set plus the requested change.
Unknown tool names require an unfiltered directory read before named loading;
listing/loading alone never fulfills an action. Owner accepted/no-change still
requires the real target-scoped receipt. Test unavailable items and discussion
without wearing; review the actual text for false completion, not just keywords.

## 1. Scope / Trigger

Applies to every installed Tool, including the 48 builtin names and private
Writer `reply.image.request`, direct/native input and both capability runtimes.
The 2026-10-14 user policy supersedes older fail-closed recommendation, empirical
capacity and ordinary recoverable-precondition guidance. A Tool call must produce
a usable receipt; program-recoverable input/state conditions are warnings.

## 2. Signatures

- `capability.NormalizeInvocation(ctx, invocation, definition)` validates the
  trusted envelope, repairs fresh model argument representation, and freezes a
  no-effect marker for unresolved semantic input. Prepared invocations retain
  their frozen argument identity.
- `handledCapabilityResult(inv, code, disposition, output)` and
  `withHandledCapabilityPreparation(...)` use canonical `handled` results.
- `recoverToolBusinessResult(inv, result, err)` classifies typed domain sentinels
  and the closed exact-code error tree in `tool_semantic_recovery.go`.
- `reconcileReplyEvidence(inv, live, refs)` returns confirmed optional hints and
  safe counts/reasons. `bindGoalPlannerMetadata` binds earlier consumed snapshot
  receipts, or Core's immutable snapshot in this run's initial request, never
  current DB versions or same-physical-request siblings.

## 3. Contracts

`handled` means the request was processed with no committed requested business
effect. Output requires `handled=true`, `effect_applied=false`,
`mutation_committed=false`, `warning_code` and `disposition`; ErrorCode is empty
and Go error nil. It is distinct from completed/accepted. Do not manufacture
facts, accepted Goal judgments, generated media, delivered messages or success.

Recover only during read-only/Prepare processing or after business savepoint
rollback. `ExecuteTool` isolates linked-attempt admission and domain writes in
that savepoint; handled then commits a technical ledger receipt. An attempt
claimed before the Tool must settle suppressed/requeue without increasing failure
count. Never leave it running, mark it succeeded, or create a new attempt merely
to explain a no-effect receipt. No-effect Persona decisions use Tool ledger
diagnostics, not an audit ID for a persona mutation that rolled back.

Both runtimes check the private prepared marker before business execution and
again after a fresh preparer returns; cancellation still wins. Native model
arguments cannot supply the identity, snapshot, prepared payload or private claim.

Required means key presence; nullable unions decide whether null is valid.
Representation repair uses explicit defaults, unambiguous scalar/enum forms,
optional nonnullable-null omission, and closed-schema extra-field omission.
Missing business meaning is not guessed. Record safe normalization metadata.
For Planner, context_review_required is a state flag rather than a dependency
resource. Ignore that exact misplaced token with a persisted warning, preserve
stored dependencies for all-metadata entries, and require explicit
reviewed_goal_ids to clear a hold. Initial empty criteria remain unchanged.
Handled ADK diagnostics and Planner final tool_warnings retain warning_code and
disposition; do not classify a handled input/state condition as a policy refusal.
The raw argument 64 KiB threshold and ActionOutcome payload/reference thresholds
are advisory; preserve full valid values and warn with counts/bytes.

Kev recommendation/disclosure/config-window changes cannot revoke execution of
installed current-scope Tools. Preserve raw Kev No audit and expose warnings;
discovery deduplicates/batches and reports an installed subset plus unavailable
names. Installed scope and actual actor/resource authority remain independent.

Optional Fact-only evidence failures exclude only those hints. Preserve exact
offered/live matching, current authorization, independent Writer composition,
draft/publication replay and real Provider failures. Unknown optional short aliases
reach reconciliation. Private reply draft hints also reconcile to the current
Core-owned canonical draft; action/target aliases and actual draft/resource
ownership remain strict. Emit
`reply.evidence.reconciled` before the Writer lease, with counts/reasons/surface
and correlation, never raw refs/body/secrets.
For a private draft hint mismatch, emit reply.draft.reconciled with safe fixed
counts/reason and carry reply_draft_hint_ignored warnings through prepared,
published/replayed and accepted-media receipts. Do not read or mutate the hinted
draft. Moments/legacy use the prior evidence-only decoder.

Missing/invalid optional Goal Review cannot undo a valid root assessment.
Validated terminal roots supersede the pending review; incomplete roots use
`evaluation_incomplete/wait` with no opportunity/ineffective streak. Optional
improvement/quote text and skip/abandon fields must not inherit create requirements.
Stale recovery carries `refresh_context` (and `refresh_reply` where applicable)
so the next native request receives current refs. Evaluation remains once per
Fluctlight per 30 minutes; Planner still needs real capacity and an eligible
completion/day/authorized Owner episode.

## 4. Validation & Error Matrix

| Condition | Receipt / effect |
| --- | --- |
| Valid nullable null, schema default or unambiguous representation | Normalize, warn if changed, continue |
| Missing meaning, ambiguous target, no record, already achieved | handled needs_input/no_results/no_change; no writes |
| Domain CAS/source version changed | handled awaiting_state plus refresh; no stale writes |
| Invalid generated schedule/activity business result | handled waiting; no invented schedule/purchase/body change |
| Optional reply hint mismatch | Exclude hint, warning, continue Writer |
| Optional private draft pointer mismatch | Ignore pointer, warn, recover current canonical cache/media or generate current reply |
| Duplicate wear/use/habit or redundant slot removal | Dedupe/no-op; no duplicate history/revision |
| Foreign actor/resource, private task forged/expired claim | Preserve actual ownership/claim refusal; never guess authority |
| Cancellation or actual Provider/DB/network failure | Preserve truthful failure/cancellation and rollback |
| Actual committed failed media/action outcome | Preserve real failure fact; never relabel no-effect |
| Missing renderer/file/vision proof | No generation/finalization claim; actual waiting/unavailable state |
| Same command identity with conflicting payload | Preserve original receipt identity; no overwrite or second effect |

## 5. Good / Base / Bad Cases

Good: valid root + missing review persists the root and technical wait; stale
scene returns handled and triggers a fresh projection; native due + only handled
Tools closes the durable attempt as suppressed. Base: successful mutations still
commit once and replay. Bad: catch arbitrary SQL errors as no_match, accept a
foreign ref, mark queued media ready, or translate handled into Goal progress.

## 6. Tests Required

Keep a fixed 49-name inventory independently of registry enumeration. Test legal
null and rejected union members, complete >64 KiB arguments/outcomes, native
schema repair, canonical fresh-Prepare marker/cancel, exact code-tree recovery,
Kev No/config-window warnings and partial discovery. Controlled PostgreSQL tests
must prove provisional writes roll back, handled ledger replays, preclaimed due
attempts close without progress/failure count, missing Review preserves valid root,
repeated wear/use does not write, and unavailable item.stop can clear existing use.
Keep direct/native HTTP/SSE and independent Writer tests. Label inventory/schema,
pure, controlled Provider, PG and actual external execution separately. Full-suite
baseline failures/SKIP are not positive persistence or live acceptance evidence.

## 7. Wrong vs Correct

Wrong: changing `failed` to `completed` or returning empty success after a DB error.
Correct: recover closed business conditions after rollback as handled, retain
real errors, and record exactly whether the requested effect exists. Wrong: skip
all attempt settlement for handled. Correct: close a previously durable attempt
through the existing suppressed path while leaving rolled-back admissions absent.
