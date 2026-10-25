# Owner schedule requests and story reads

## 1. Scope / Trigger

User-approved production Pen implementation requires real cross-day schedule reads, non-overwriting day generation and accepted-promise scheduling. The story view additionally requires real complete counts rather than frontend cursor-page estimates. Scope is Owner/character plus the relevant frozen profile/world facts.

## 2. Signatures

- `GET /api/fluctlights/{fid}/schedules?from=YYYY-MM-DD&to=YYYY-MM-DD`
- `POST /api/fluctlights/{fid}/schedules/requests`
- `GET /api/fluctlights/{fid}/schedules/requests/{requestId}`
- Migration `0097_owner_schedules`: durable request rows and independent `life_schedule_items.promise_request_id` association.
- Workflow intent `schedule.owner`; HTTP admission does not call a model.
- `GET /api/fluctlights/{fid}/shared-experiences/overview` → `BrowserStoryOverview`.
- `GET /api/fluctlights/{fid}/memories?aboutOwner=true` → existing managed-memory cursor page with an additional Owner-effective scope.
- `BrowserMomentRecord.like_count` is distinct from all-reaction `reaction_count`.

## 3. Contracts

Schedule dates use the character's authoritative timezone and a 1–31 day range. Requests carry localDate, expectedRevision, expectedLifeContextRevision, idempotencyKey and reason. Promise requests additionally carry promiseId, expectedPromiseRevision, startAt and endAt. Original request replay precedes stale date/timezone/mutability guards. Freeze foundation/profile/world and life/schedule versions; prevent overlapping pending day/promise admissions. Existing accepted schedules are preserved by generation. Promise associations require the actual accepted execution objects and trusted Worker scope; normal/model schedule writes cannot invent them. Association is not execution or fulfillment. Future valid appointments survive replans; cancelled or changed-window associations no longer provide current-life authority. Query the original durable request after uncertain transport results.

`OwnerStoryOverview` uses one read-only repeatable-read transaction, including fid ownership, active profile, character timezone, counts and cited rows. It does not generate narrative or write a story entity. Response includes as_of/timezone/actor_id/profile_id, each metric's value/available/complete and explicitly partial footprints.

- Communication uses the same actual participant time intervals as shared-experiences. First communication is the earliest actual shared message, not character/session creation or the earliest loaded page.
- Completed turns deduplicate `(conversation_id,turn_id)` and require an Owner user input plus a nonempty persisted character assistant reply. Inbox processed/superseded or message count is insufficient.
- Relationship direction is character → current Owner; prefer the active-profile row, then null-profile fallback. Do not average across profiles/directions.
- Effective Owner memories have `actor_refs ? owner`, active status and the existing prompt source-liveness predicate. Pending/invalid is excluded. Preserve the established `legacy_unknown` compatibility rule. Daily memories require every source live; ordinary memories require at least one.
- Diary totals count ready day records with nonempty body from the current active attempt, not attempts or pending/no_content rows.
- Footprints cite original record identity/revision/date. An undated relationship or pending promise remains undated; no fabricated occurrence time or fulfillment.
- `aboutOwner=true` applies that same effective-memory predicate and binds it into cursor scope. Default managed-memory browsing retains its original inclusion of historical/invalid-source records and old cursor scope. Reject malformed boolean values.
- Browser/Core OpenAPI and generated clients change with the boundary contract. No new production environment variables or story migration are needed.

## 4. Validation & Error Matrix

| Condition | Result |
|---|---|
| Other Owner/fid or unknown object | Existing not-found/authorization contract |
| Schedule range too large/invalid local date | Invalid request |
| Accepted schedule already exists | Preserve and report actual result |
| Frozen scope/revision changed | Conflict, no new arrangement |
| Promise cancelled, fulfilled or window changed | Guard new admission/authority; preserve history |
| Cross-midnight replay of admitted request | Return original request before stale-day rejection |
| Story query has filters | Invalid request; no silently ignored filter |
| Any story SQL read fails | Fail the read; frontend may retain its earlier snapshot, never use fake zero |
| About-owner cursor reused in another scope | Invalid cursor |

## 5. Good / Base / Bad Cases

- Good: a promise moves from D1 to D2; obsolete D1 association does not block its valid new admission.
- Base: generation of a day with an accepted schedule reports preservation.
- Bad: HTTP runs generation directly, a promise link becomes an executable action_plan, or the story counts segmented assistant output as multiple completed turns.

## 6. Tests Required

Keep schedule regressions for scope/CAS/idempotency, day preservation, real accepted-promise association, cross-midnight replay and obsolete-window association. Story/read assertion points: cross-Owner rejection, profile fallback/direction, DST-inclusive natural days, split replies and duplicate turn identity, invalid/live/daily/legacy memory sources, ready active-attempt diary counts, correct about-owner cursor isolation, no write/model side effect and synchronized generated boundaries. User unified acceptance owns full runtime and upgrade verification; compile-only checks are not these tests.

## 7. Wrong vs Correct

Wrong: `rounds = messages.length / 2; memoryTotal = page.items.length; promiseAccepted = promiseFulfilled`.

Correct: read counts and sources in the authorized database snapshot, deduplicate durable conversation/turn pairs with actual published replies, use source-liveness authorities and preserve execution/fulfillment as separate domain facts.

## R9: chat proposals and original scheduler recovery

`promise.propose` is a conversation-only Tool. Its title comes verbatim from the
actual current Owner user message bound to the source fact and active target
participant. The model supplies an explicit future window and reason, never an
Owner-authored title or accepted/scheduled/fulfilled state. The existing inbox /
Goal / Commitment / Intention flow decides acceptance. Future candidate/qualified
registration checks authority without requiring window_start to have arrived;
actual due/start execution still requires that window. Only accepted, genuinely
linked chat promises enter the existing Owner schedule-request flow, once per
promise. A stale/unschedulable individual does not abort subsequent candidates;
missed starts are not silently shifted or retroactively fulfilled.

POST `/api/fluctlights/{fid}/schedules/recover` accepts reason/idempotencyKey.
OwnerSurface authenticates, audits and commits in its original transaction;
recovery re-arms only the original failed/dead_letter schedule_intent with its
original payload/workflow identity. Pending/running/completed is an idempotent
readback, never a duplicate scheduler. Temporal allows failed-only reuse. The
browser generated client, route matrix and initialization/schedule page use the
same command, preserving unknown outcomes for retry. Regression includes actual
PG promise-to-calendar associations and strict future execution negatives.
