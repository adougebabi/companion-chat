# Implementation Plan: Life-like AI Companion

## Order of Work

1. Add versioned same-file SQLite schema initialization and repository helpers. Keep legacy data untouched and do not read it.
2. Replace fresh-start persona/conversation persistence with table-backed CRUD, strict persona ID validation, and settings responses that omit secrets.
3. Build the persona interview session and blueprint preview/activation flow, including immutable foundation versions and visual-profile reservation.
4. Implement context composition, persona-private memories, evidence records, evolution history, rollback, and guarded automatic evolution patches.
5. Implement routine/schedule/state/event tables, deterministic eligibility evaluation, recovery reconciliation, and the development lifecycle inspector/simulation controls.
6. Add durable job leasing/retry/backoff and move media/evolution work onto it. Preserve `maxConcurrency=1` initially.
7. Implement activities, comments, likes, supporting-character rules, hide/restore, screening, read state, and cursor endpoints.
8. Compose state into chat and image prompts; add proactive direct message decisions without changing the existing chat SSE event contract.
9. Implement first-release event-driven image attachment, original-item placeholder replacement, and job failure states.
10. Add focused API/data tests and manual lifecycle scenarios; then update backend specs, README persistence notes, and task artifacts.

## Validation Matrix

- Fresh database creates no demo personas and enters the interview flow.
- Foundation revision is user-only, versioned, reviewable, and rollbackable; automatic evolution cannot change it.
- Two personas never share memories, comments, events, supporting characters, or chat context.
- Current situation always has a schedule/event source and affects both chat context and image prompt composition.
- Eligible event generation respects local time, routines, cooldowns, negative-event guardrails, screening, and focus tier.
- Offline recovery produces a bounded summary, not a burst of routine records.
- A chat plan persists only after explicit time-bounded acceptance; cancellation/reschedule is audited.
- Activity paging has no duplicate/missing records across a cursor boundary; hidden posts disappear only from display reads.
- Supporting-character comments prioritize event participants and obey a per-post cap.
- Pending/failed/successful image jobs retain original text, show stable placeholder state, and update the same activity/message on refresh.
- Provider outage queues retryable work while deterministic routine state advances.
- Concurrent chat, job, and event writes do not lose unrelated rows.

## Commands

Use a temporary fresh database for all persistence tests. At minimum run:

```sh
node --check server.js
DATA_DIR=/private/tmp/local-ai-companion-life-test npm start
curl -fsS http://localhost:4178/api/health
```

Add a project test command before claiming automated domain coverage. Exercise browser flows at the frontend task's required viewports and inspect browser-console errors.

## Risk and Rollback

- Do not point new code at a user's legacy database for destructive development tests.
- Schema DDL is additive and ledgered; no legacy conversion is attempted.
- Provider calls must never run inside a SQLite transaction.
- A release rollback is the old binary with existing untouched legacy storage. New companion records are not compatible with the old application by design.
