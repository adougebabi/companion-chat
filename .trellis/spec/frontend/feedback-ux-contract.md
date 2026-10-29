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

## Good, base and bad cases

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
