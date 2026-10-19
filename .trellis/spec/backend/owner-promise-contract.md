# Owner Promise Lifecycle

## 1. Scope / Trigger

An Owner proposes, reschedules, withdraws from, or retries an agreement. The
character may accept, decline or defer. Schema bundle: `0075_promise_lifecycle`.

## 2. Signatures

- `GET/POST /api/fluctlights/{fid}/promises`; GET individual `/{requestId}` and
  `/{requestId}/history`; POST `/{requestId}/{reschedule|cancel|retry}`.
- Proposal: `title`, `dueAt`, optional `startsAt`, `reason`, `idempotencyKey`.
  Changes also require `expectedRevision`; time strings contain an explicit offset.
- `owner_promise_links` links the original Owner request to actual Goal,
  `goal_commitments`, and `fluctlight_intentions`. `owner_promise_history` records
  changes. It does not store an independent fulfilled flag.
- Private Planner `promise_bindings[{request_id,goal_id,expected_revision}]` can
  resolve a newly created local Goal ID through the existing GoalSet command.

## 3. Contracts

Use the existing Owner request/NativeCognition/Tool path. API acceptance performs
no Provider work. The Tool binds the exact requested action/title and window;
only a real Commitment plus Intention is accepted. Missing Goal emits a durable
Owner Planner event and `awaiting_goal`, not accepted/failed. Planner binding,
original-request result, execution links and its durable receipt commit together.

Keep the existing single active tactical `step` Commitment. Independently scoped
`promise` Commitments may coexist on the same Goal; they do not belong to a
temporary Stage. Admission respects the existing evaluator's capacity of eight
unfinished Commitments per Goal. Trusted `OwnerPromiseID` is `json:"-"`; generic
model plans cannot claim that identity or directly adjust/abandon a promise.

Rescheduling is a new character-assessed request. It updates the original window,
Intention deadline/preferred time and, when necessary, its typed trigger; it emits
a schedule-review fact. Withdrawal stops only its associated objects, preserving
the shared Goal, actual effects and history. Clock repair emits elapsed-window
facts. Fulfillment is projected from original Commitment evidence evaluation;
Intention completion alone cannot prove it. Late ingestion of evidence that
actually occurred inside the window remains eligible under the existing evaluator.

Browser submission recovery stores only immutable input/idempotency identity.
After an unknown response or refresh, replay that original command before starting
another proposal. Preserve edits made during submission. Existing unlinked legacy
proposals remain visible as `legacy_unlinked` without fabricated completion history.

## 4. Validation & Error Matrix

| Condition | Behavior |
| --- | --- |
| Missing/ambiguous offset or nonfuture/invalid window | 400; no request or links |
| Old link/Commitment/Intention version or changed profile | 409; keep draft |
| Owner attempts Planner binding directly | Deny; no character acceptance |
| Goal already has a tactical step or other promises | Keep each independently |
| Commitment capacity exhausted | `goal_commitment_capacity_full`; no partial accept |
| Fulfilled/cancelled/in-progress execution is rescheduled | Reject, preserve facts |
| Cancel races an old cognition/Planner response | Life lock, terminal root wins; settle old inbox |
| Response transport outcome unknown | Keep original input/key; do not generate a new proposal |

## 5. Good / Base / Bad Cases

Good: propose → real decision → explicit Planner binding when needed → original
Commitment/Intention → actual evidence judgment. Base: honest rejection, deferral
or capacity wait. Bad: accepted from a task ID, shared-Goal cancellation, or
fulfilled from an executed Intention without Commitment proof.

## 6. Tests Required

`promise_lifecycle_test.go` covers provisional planning, private Tool binding,
replay/recovery, existing tactical step plus independent promises, reschedule CAS,
cancellation isolation, and an inbox cancelled before assessment. Browser window
validation and `v3-persistent-command.test.mjs` cover offsets and unknown response
recovery. These cases are authored; execution status is in the task evidence.
Real model cases are user-owned in `docs/v3-llm-acceptance.md` L12.

The Owner request wrapper advertises `CapabilityPreparer`. If its inner Tool
has no preparation stage, freeze an explicit nonempty `not_required` request
payload. Returning the original empty invocation fails before the transaction
and strands otherwise valid Promise, wardrobe and item-use requests. Preserve
the inner preparer's own payload and errors when it does have a stage.

## 7. Wrong vs Correct

Wrong: `planning_requested => accepted`; retry edited input with a new key after
losing the old response. Correct: persist the original proposal, expose pending
planning, bind actual execution objects transactionally, and replay the frozen
submission identity until its admission result is known.
