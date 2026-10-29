# Command receipts, editable drafts and native events

## 1. Scope / Trigger

Use these contracts in the retained Goal Planning renderer, World event recovery,
MemoryPage self governance and DomainPrimitives. This records existing Core/BrowserClient protocols and
frontend recovery behavior; it does not add backend APIs, statuses or model rules.
A controlled fixture must mirror the actual receipt before certifying UI behavior.

## 2. Signatures

- `lifeClient.updateGoalSet(fid, base, changes, reason, idempotencyKey?)` sends
  `expectedVersion`, `expectedFactsRevision`, `reason`, `idempotencyKey` plus the
  selected changes to `PUT /api/fluctlights/{fid}/goal-set`. Omitted optional key
  preserves the prior helper behavior of generating an identity.
- `lifeClient.plan(fid, idempotencyKey?)` sends the original identity to
  `POST /api/fluctlights/{fid}/goal-planning`.
- World retry posts to
  `/api/fluctlights/{fid}/world/templates/{templateId}/runs/{runId}/retry` with
  `expectedRevision`, `expectedRunRevision`, reason and the original key.
- Self governance uses the existing `growthClient` review/forget/restore commands
  and `selfClaim`, revision and Owner-review readers for the selected fid/claim.
  Command revision/status acknowledgement and secondary ledgers are separate.
- `FieldPort.commit` forwards input and immediate change for checkbox, radio,
  select, date and datetime-local. `FieldPort.blur` forwards focusout and only
  submits change for text-like/number controls whose change is deferred to blur.
  File selection keeps its dedicated existing branch.

## 3. Contracts

### Goal planning

Manual `order` contains every current **active** goal exactly once; candidate,
paused and closed IDs do not belong in that payload. Dependency editing and
candidate/source review remain separate controls with their full actual records.

The Goal set write is atomic. Its result contains `commit_succeeded: true`, the
committed revision and full acknowledged projection. Persist the original key,
body, CAS/facts baseline, reason, acknowledgement and receipt revision separately
from the editable policy/review draft. After acknowledgement, recovery performs
GET only. Latest revision must be at least the receipt revision; at the same
revision, the selected fields must agree with the acknowledged projection.

One mounted editor owns draft fields for each Owner/fid slot. An older editor may
merge an acknowledgement only into a pending record with the same key. It cannot
persist its whole stale draft over the reopened editor. Import that durable
acknowledgement before deciding whether recovery needs another write.

Candidate/source commands must not silently rebase an independent policy draft.
Show the changed baseline and require explicit reviewed adoption. Unknown results
keep the original body/key; accepted results use labels such as
`只读恢复已保存提交结果` or `只读恢复规划任务历史`. A requested planning receipt is
not evidence that the planner completed or a goal was achieved.

### World commands and baselines

Retry returns only `{id, status: "queued", revision}`. Validate original run ID
and exact `expectedRunRevision + 1`; do not demand missing full-run fields from
that receipt. Then GET the original run and validate fid, world, template, run ID
and minimum revision. Trigger's full-run projection checks remain intact.

Current saved fields, exact applied definition and recorded recovery baseline
are independent. Preview/capture does not change current place fields. Explicit
restore may do so. A verified applied definition can be the effective recorded
source at default revision `0`; preserve that zero in commands. Missing exact
publication is unreadable, not an invented default. Keep source/version and
nested extensions during capture/restore.

### Self governance and history

An explicitly checked latest baseline must update both the pending command's
revision and the opened current detail. Refresh list, revisions and Owner reviews;
otherwise cancelling returns to stale UI. Manual self detail refresh must also
refresh those mounted secondary readers. Do not broaden memory/reflection refresh
behavior without a separate requirement.

An accepted forget/restore followed by the **same** revision is consistent only
when status matches the receipt. Preserve GET-only recovery on mismatch. A newer
authoritative revision may supersede that result. Review acknowledges an Owner
read independently of semantic status; it must not require the forget/restore
status condition, append a semantic revision, or raise confidence/evidence.

Fixtures must expose complete immutable snapshots for every seeded and appended
revision. Preserve original evidence/provenance/value and explicit expiry or
supersession fields across forget and restore. A restored accepted snapshot
appends a new revision; it does not rewrite history or erase the independent
review ledger. These controlled checks do not implement Core's source-dependency
or expiry authority engine.

### Native change/blur

An immediate-change control may synchronously redraw its owner in onchange.
Removing its focused visible port causes blur. Emitting a second change from
that blur can reenter `innerHTML` replacement and throw in a real browser. Keep
focusout/relatedTarget and selection forwarding, but do not replay the immediate
change. Text/number/textarea still retain their draft and blur submission.

## 4. Validation & Error Matrix

| Condition | Required frontend behavior |
| --- | --- |
| Goal write accepted, GET fails or is older than receipt | Keep accepted checkpoint; recover with GET, no second PUT |
| Same acknowledged revision has inconsistent submitted projection | Keep original receipt and report incomplete readback |
| Response lost before acknowledgement | Preserve frozen original body/key, independent newer inputs |
| Receipt arrives after same Owner/fid editor reopened | Merge same-key acknowledgement only; keep current draft fields |
| Candidate/source review changes policy revision | Require explicit policy baseline review before policy/source write |
| World retry returns minimal valid receipt | Read original run; do not reject for absent full projection fields |
| World run GET belongs to another scope | Reject readback and retain recovery; do not adopt foreign fields |
| Publication missing or expired/paused retry authority | Keep honest unreadable/disabled state; no fake source or execution |
| Checkable onchange moves focus/redraws | One change, no recursion; focusout and FormData remain valid |
| Self latest baseline explicitly adopted then edit cancelled | Display checked current detail and refresh its secondary readers |
| Self forget/restore receipt, same revision but inconsistent status | Retain accepted checkpoint; recover via GET only |
| Owner review or newer authoritative self revision | Preserve independent review semantics; allow newer authority |

## 5. Good / Base / Bad Cases

Good: save policy, edit a new reason while readback is pending, reopen, receive
the old receipt, then recover via GET while retaining the new reason.

Base: original World run retries from v4, returns queued receipt v5, and its own
full GET confirms scope and later result. Capturing a definition changes the
baseline; only explicit restore changes the place.

Bad: retry with a new key after an accepted write, silently adopt a candidate's
new policy baseline, require `world_id` from a receipt that has no such field,
or emit checkable change both on click and blur.

## 6. Tests Required

- `goal-planning-recovery.test.mjs`: active-only order, advanced policy fields,
  dependencies, exact original replay, accepted GET-only recovery, older or
  inconsistent readback, same-slot late policy/plan receipt after reopen and newer
  drafts, explicit reviewed baseline, stable planning identity/history recovery.
- `demo-replica-world.test.mjs`: minimal retry receipt red/green, invalid ID/status/
  revision, foreign scope, unknown and readback failure, exact definition capture,
  effective v0 baseline, explicit restore and authority-disabled conditions.
- `r11-domain-primitives.test.mjs`: checkbox/radio synchronous focus movement,
  exactly one change with callback depth one, checked/FormData, focusout target;
  untouched date blur; text/number/textarea timing and selection.
- `demo-replica-memory.test.mjs` / `self-history-fixture.test.mjs`: latest adoption
  then cancel, manual secondary refresh, accepted mismatched status, independent
  review, newer authoritative revision, original unknown replay and full append-only
  restore history with evidence/provenance retained.
- Production dist, desktop1348x925 and narrow390x844: retain browser exception,
  bounds, draft, request identity and readback assertions. DOM tests do not replace
  the actual browser reentrant-removal regression. Controlled HTTP does not prove
  real Core/PG/worker/model completion.

The 2026-10-29 task's actual production-browser reports record World 14, advanced
Goal Planning 8, linked Studio runtime 2 and Self history 10 passing cases. Self
normal cases use pass106; other Self cases use pass104. The consolidated index
retains each source report and earlier failed runs. Full Web pass106: 694 tests,
0 failed/0 skipped. These counts describe checks, never fullsite completion.

## 7. Wrong / Correct

Required startup failure must be presented before the shell infers a missing
session or missing role. A failed directory read does not establish logged-out
state. Keep the retry surface, and clear only `errors.bootstrap` after a successful
bootstrap (including an authoritative unauthenticated session). Preserve other
same-Owner read errors. The bootstrap error card must align to its own content,
not stretch to the SidebarProvider's full viewport height. Browser entry-state
coverage includes the initial failure and actual retry on both viewports.

Studio JSON parser errors have a local owner. Explicit adoption of corrected
JSON, or withdrawal of that JSON draft, clears the old parser error only if it
still owns the displayed error. Do not erase a later independent request/readback
failure. Retain the working body, extensions and pending original commands.
`r11-studio` verifies adoption, withdrawal and an independent subsequent failure;
seven resource kinds exercise real structured fields and saved refresh on both
viewports. This adds no publication or runtime authority.

Wrong: mark an accepted boolean, accept any revision greater than the pre-write
baseline, and persist an old editor's whole draft when its request returns.
Correct: retain the receipt revision/projection and merge only same-key
acknowledgement into the currently owned draft before GET-only recovery.

Wrong: validate a retry receipt as a full run or convert fixture reads into fake
backend success. Correct: validate the declared small receipt, then read the real
run projection; label controlled execution as controlled evidence.

Wrong: every blur emits change. Correct: the same immediate-change classification
is used by commit and blur; focusout and deferred text submission remain intact.
