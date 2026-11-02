# Feedback semantics and original recovery

## Scope and trigger

All production pages, retained domain tools and command dialogs follow the
2026-10-29 feedback UX addition to `10-28-demo-fullsite-audit`. The supplied
workspace Demo owns visual tokens; a missing Demo state does not remove a real
business failure, pending task or recovery constraint.

## Signatures and ownership

`react-app/shared/Presentation.tsx` exposes:

```tsx
type FeedbackKind = 'information' | 'action-required' | 'warning'
  | 'error' | 'success' | 'progress';
<Feedback kind={kind} title={title} actions={realActions}
  details={<pre>{JSON.stringify(diagnostic, null, 2)}</pre>}>
  {humanReadableReason}
</Feedback>
<ReaderFeedback source={reader} label="资料" />
<ResultFeedback notice={notice} onDismiss={original =>
  setNotice(current => current === original ? null : current)} />
```

`Feedback.kind` is required. Its component owns appearance and announcement;
the business owner chooses the kind, message, action and lifetime. `InlineFeedback`
is a compatibility wrapper: the existing implicit `destructive` still means
`error`; `variant="default"` means information. Audit a caller before changing
its semantics. Never neutralize all legacy errors with a default change.

`approved-ui/creation-recovery-state.mjs` exposes
`inspectCreationRecovery({owner, storage, resume})` and
`requireCreationRecovery(...)`. Inspection is read-only. The latter throws a
typed error carrying `recoveryState` when mounting a controller would be unsafe.

## Contracts

| Kind | Meaning | Default announcement |
| --- | --- | --- |
| information | Explanation or optional guidance | note |
| action-required | A concrete business state with a real next step | note |
| warning | Limited or incomplete data, retained confirmed content | status / polite |
| error | Current operation actually failed | alert / assertive |
| success | Confirmed completion or persisted result | status / polite |
| progress | Accepted or running work / actual ongoing read | status / polite |

Use the normalized specific reason as primary copy. `feedbackDiagnostic(cause)`
preserves code/status/message/request correlation/stack in an explicit collapsed
disclosure. Do not put raw diagnostics in page copy or replace every cause with
one generic summary. A custom field error stays adjacent to its field, with
`aria-invalid` and `aria-describedby`; clear it on that field's correction.

Initial-read failure differs from refresh failure with confirmed data. Keep the
confirmed data and show a warning for the latter. Independent datasets retain
independent retries: current and historical goals, list and unread count, detail
and revisions are not repaired by one unrelated GET.

`ResultFeedback` expires ordinary success/information after 6000 ms and supports
closing that event. The callback receives the original notice identity; an older
timer must never clear a newer notice. Warning, progress and action-required do
not expire as ordinary confirmations. Clear them from authoritative state.

For retained DOM ports, owners declare `data-feedback-kind`; the bridge applies
the corresponding semantics. Do not infer business severity from text or a CSS
class. Preserve source node/listener/FormData identity and hidden placeholders.

## Validation and recovery matrix

| Source state | Required UI and action |
| --- | --- |
| Valid unsubmitted creation draft | Compact action-required card, one Continue action |
| Creation draft/command is corrupt or incompatible | Warning; preserve original bytes, actual local reread/export, no fabricated Continue |
| Storage access or getter throws | Error with real reread; block fresh admission rather than treating missing access as an empty record |
| Activation result unknown | Action-required; open the original command, keep body/key/requestId |
| Draft and command requestId mismatch / orphan command | Block new creation; retain both records and diagnostics |
| Valid archived creation with independent corrupt draft | Open the requested history only; never substitute a different current draft |
| Preparation accepted/running/awaiting-review/failed | Real per-stage state and allowed action; accepted is not completed |
| Persona preparation failed/pending | Chat keeps the draft, disables send, and exposes scoped recovery/readback; explicit admission rejection creates no pending turn |
| Visual session `last_error=vision_disabled` | Explain the saved candidate is unverified; link to 看图理解 settings and retain explicit manual review |
| Original configuration unresolved | Disable all new save admission, including form submit; recover original receipt first |
| Configuration receipt recovered or stale connection draft restored | Explicit latest-baseline read preserves current input before new save |
| Structured configuration field invalid | Atomic pre-write validation, attached settingKey/key, field error and focus |
| Related-page read fails after confirmed write | Confirmed write remains; actual readback retry sends GET only |
| Task control accepted | Progress until authoritative revision/run advancement; no premature completion |

Local reread, navigation, dismissal, expiry and refresh never delete a draft,
reset a task, replace a frozen request, or change a backend message status. Keep
Owner/fid/world fences, CAS, allowed_actions and accepted-readback GET-only rules
from `command-receipt-draft-recovery.md`. Buttons require a real handler at the
owning layer; omitted capabilities do not get a no-op recovery control.

### Diagnostic task status and chat recovery (2026-11-02)

`taskStatusPresentation(task, now)` owns the six mutually exclusive groups:
attention/running/waiting/completed/ended/unknown. Summary, task-tab number,
filters, row badges and task detail consume this projection and the same filtered
loaded rows; summary counts must add up to the visible list count. `started`
counts as running; after 15 minutes it remains running with a verification
warning. Unknown states stay separately counted. World event clocks use actual
cycle evidence for “生成中” / “等待下一小时”; lifetime started alone is insufficient.
Ordinary waiting tasks use “等待中”. `d7-task-taxonomy.test.mjs` renders the audit's
20-row distribution and verifies filtering/count agreement and clock consumers.

`PersonaPreparationFeedback` renders directly before chat messages. Recovery
keeps the draft and original request, and the persona intent link opens its actual
task with returnTo pointing back to chat. Keep `ft-r1-defects.test.mjs` DOM/order,
task-reference, draft-retention and recovery assertions.

Assistant bubble/history and reply-notification rendering use `replyBodyText`.
Malformed JSON and incomplete headings fall back to recovered plain body text,
with the same private-section/metadata filter; multiple reply sections become
plain paragraphs. Do not hide recoverable text or display the raw envelope.
An empty cleaned body never falls back to private content. User messages remain
unchanged. Shared Go/JS drift fixtures verify the server/UI boundary;
real chat, history and inbox DOM tests verify all presentation consumers.

## Good, base and bad cases

### R34C same-source counts and effective selections (2026-11-02)

1. **Scope / trigger:** diagnostics, Inbox service issues, world/wizard previews,
   Studio counts, wardrobe and memory empty states.
2. **Signatures:** `readAttentionTasks(operationsClient)` consumes the existing
   `pending('', 'failed_task', cursor)` pages; `worldEventClockLabel(startedAt,
   slotAt, includeDate)` formats existing timestamps. `OwnerTask.source` preserves
   `cycle_started_at` unchanged for running world-cycle age checks.
3. **Contracts:** diagnostics and Inbox share unresolved pending membership and
   attention states. Read all original pending cursors; preserve a pending row's
   status when its separate detail fails or changes concurrently. Late Owner
   responses cannot publish. Counts and filters use the same visible projection.
   Running-task age prefers valid actual starts, then creation evidence, ahead of
   mutable updates; every running kind gets the 15-minute hint. World time says
   `20:00 整点（20:39 开始）` without replacing source timestamps. Wizard previews
   use the effective selectable source; historical daily/published draft choices
   cannot imply selection when existing worlds require another choice. Step
   navigation resets `.workspace-page` to top and focuses its heading.
4. **Validation / errors:** unread pending source means unknown count, never a
   fabricated zero or empty state. Failed task detail retains membership and a
   real retry. Studio counts include the cards shown, label published/draft/archive
   subtotals, and remain visible on narrow screens. Registered garments and
   unfinished preparation are described separately; confirmed wear is a list.
   Unfiltered empty memory offers chat guidance, while filters offer reset.
5. **Good / base / bad:** good: an older failed task appears in both Inbox and
   diagnostics. Base: all visible counts sum to the visible rows. Bad: deriving
   the global unresolved count from only the 20 newest workflows, or showing a
   retained default-world draft as a current selection.
6. **Tests:** `r34c-task-audit`, `r34c-world-wizard`, `r34c-sidebar` and the
   memory/wardrobe/studio/inbox DOM suites cover counts, source failures, Owner
   fences, selection, scroll and accessible labels. Go
   `TestR34CPendingAttentionSharesAllActionRequiredStates` and
   `TestR34CWorldTaskRetainsActualCycleStartForStalePresentation` verify the real
   source in isolated PostgreSQL. Production browser evidence belongs outside Git.
7. **Wrong / correct:** wrong: `started_at || updated_at || created_at` and
   counting a separate recent page. Correct: pick the first valid execution or
   creation timestamp and classify the same pending/visible set used by rows.

Good: unreadable storage blocks a new send and offers local reread; after access
returns, the original retry identity is restored without a replacement command.
Base: an empty successful directory has ordinary empty guidance and a real create
entry. Bad: JSON corruption becomes a red “needs review” box linking a new wizard
which silently overwrites the original record.

## Required tests and browser evidence

Use `feedback-semantics`, `creation-recovery-state`,
`conversation-storage-feedback`, `demo-replica-management`, `r16-regressions` and
the owning page regressions. Assert real callbacks, original bodies/identities,
scope fences, retained inputs, semantic kind and stale-event clearing.

`ft-r1-defects` exercises the actual chat recovery control through
`BrowserClient.recoverPersonaPreparation(fid, {expectedRevision, idempotencyKey})`
and `workspace.command`. A recovery receipt followed by a failed detail read
retains GET-only readback; never enqueue a second recovery request. Clear the
blocked state only after `detail.persona_preparation.status` reads `ready`.
Remounting chat restores its unresolved workspace command. If the original
command belongs to an unmounted page whose readback callback no longer reads,
chat reads its current scope itself and retains GET-only recovery on failure.
`persona_preparation_failed` and `persona_preparation_pending` are explicit
pre-admission rejections even when HTTP uses 409; retain the draft without a
phantom optimistic message or an unknown-outcome retry. Do not translate these
states into model timeout or connection failure.

Browser checks use production dist, desktop 1348×925 and mobile 390×844. Record
before/after with the same content and fault. Check location, duplication, real
actions, clearing after recovery, diagnostic expand/collapse, accessible roles,
long text, button visibility and scroll containment. DOM/screenshot collection
alone does not certify visual acceptance of an unopened page or live Core/model
behavior. Keep eight-scenario applicability/evidence per original matrix row.

## Wrong versus correct

Wrong: classify every catch as `action-required`, hide its failure details and
offer “continue” regardless of readable source records.

Correct: inspect the actual source state at its owner; show a compact classified
summary with its specific reason, supported actions and retained diagnostics.
Derive clearing from confirmed recovery, not from closing the card.
