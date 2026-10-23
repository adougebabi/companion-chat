# Approved HTML demo migration (2026-10-23)

Current source is the supplied **v6.2** demo. Its appearance is the reference;
field names/types/ownership follow the current Core schemas, including fields
omitted by the acceptance report. The Owner confirmed readable initialization
preview before activation, grouped identity/behavior/life editing, read-only
personality dimensions, and secondary links to existing governance.

The 2026-10-23 token-limit removal in the Provider contract supersedes all
earlier binding input/output budget controls and validation instructions.

## 1. Scope / Trigger

The user supplied `index.html` and explicitly requested its one-to-one migration,
reusable components, real backend integration, and simulation controls only in
isolated demo builds. This is the current visual and interaction baseline for
the main workspace, superseding the earlier Pen/navigation review gates.
The user explicitly opted out of creating more Trellis tasks.

## 2. Signatures

- `App.vue`: build-time `VITE_PRODUCT_MODE=mock` selects `reference-fixture.mjs`;
  all other builds select `ApprovedApp.mjs`.
- `approvedRoute(hash)` / `approvedHref(page,fid)` retain canonical `#/c/:fid/*`
  routes. Unmigrated detail/auth/governance routes mount `LegacySurface.vue`.
  `#/studio?legacy=1` explicitly opens the existing creation workbench.
- `ActionButton`, `Surface`, `CurrentOutfit`, `ActionFeedback`, `FilterChips`,
  `DetailDrawer`, `GarmentArt` own the repeated native UI patterns.
- `unoptimizedRenderer(factory)` wraps retained compiled render bodies with
  balanced `setBlockTracking(-1)` / `setBlockTracking(1)`.
- `configurationPlan(snapshot)` validates the full immutable snapshot before
  any write; `acknowledgeConfiguration(state,operation,receipt)` updates only
  acknowledged inputs/baselines and `receipt.applied_versions`.
- `stagedCommand({storage,key,live}).stage(name,create,execute)` journals the
  immutable request body and identity of each world-expansion stage.

## 3. Contracts

Use the existing Vue runtime and real domain services. Preserve original CSS,
SVG geometry, native root tags and layout classes. Do not vendor a second Vue
runtime. The isolated reference fixture contains only the supplied behavior and
sample records; it must be absent from production assets.

Compiled block lists describe the old native DOM boundaries. After extracting
native elements into components, do not reuse those optimization lists. Ordinary
Vue patching must preserve input nodes, update native attributes/text/listeners,
and handle repeated route changes. No full-app remount is an update strategy.

Reads check owner, character, generation and read epoch before applying results.
Commands keep their original body/key through unknown outcomes; UI generation
fences late projections without preventing same-owner/character recovery.
Checkpoint accepted owner-request IDs before releasing submission identity, and
resume observation when the document becomes visible. Terminal domain failures
open original request diagnostics instead of creating another mutation.

Configuration readback must not silently adopt unrelated later versions. Advance
only exact receipt versions and update the submitted baseline while retaining
newer input. Credentials stay in memory; persisted configuration recovery stores
only the command key. A missing receipt is not proof of non-admission: use the
existing cancellation fence before releasing an unaccepted identity. Character
quiet policy has its own revision and owner-scoped recovery, independent of the
currently selected character.

Each world-expansion stage replays its original payload/key, including the apply
preview's revision and choices. Persist request inputs, never business results.
A later-stage 422/409 cannot release the outer identity after earlier effects.

Memory supports real content revision/forgetting; source, date, classification,
importance and tags remain read-only under the current API. Outfits have no
server deletion operation: disable deletion instead of mutating the local list.
Actual media uses authorized media URLs/native playback and real durations.
Dates, schedules, online/status labels and current wearing come from real data.

## 4. Validation & Error Matrix

| Trigger | Required behavior |
| --- | --- |
| Production build | No simulation checkbox or reference fixture in assets |
| Mock build | Supplied sample behavior and simulation checkbox are available |
| Same page changes after extraction | Text/props/listeners update; no stale parent/block errors |
| Lost world-apply response | Retry exact original body/key without another preview |
| Configuration input changes in flight | Ack original baseline; retain newer input |
| Account/character changes during read | Discard late projection |
| Accepted request becomes hidden | Retain receipt pointer; resume visible observation |
| Saved outfit deletion | Disabled, with truthful explanation |

## 5. Good / Base / Bad Cases

Good: save setting A, type B while waiting, acknowledge A's exact receipt, and
preserve B. Base: select a role, open its wardrobe and read actual wearing.
Bad: update the demo array and announce a persisted deletion, or freeze an
invalid compiler block list and remount the entire app when it breaks.

## 6. Tests Required

`approved-ui.test.mjs` verifies planning/acknowledgement, immutable stage recovery,
canonical/deep routes, SSR native parity, dynamic primitives, computed filter
unwrapping, repeated extracted-app page transitions and quiet policy planning.
`approved-workspace.test.mjs` verifies stale reads, role round-trip retry, later
stage validation retention, receipt checkpoints and visibility resume.

Run generation, workspace typecheck/tests/build, isolated mock build, targeted Go
quiet-hours race tests, and browser checks through a static production build.
Controlled API fixtures and screenshots are visual/contract evidence, not a
substitute for live PostgreSQL/model acceptance. Record baseline failures as such.

## 7. Wrong vs Correct

Wrong: move a compiled native card's children inside a component and retain its
old `dynamicChildren` list, then rely only on a successful Vite build.
Correct: disable obsolete compiler block tracking for the retained render bodies,
keep component native DOM parity, and execute repeated route/data updates.

## Scenario: v6.2 initialization and character details

### 1. Scope / Trigger
New/empty library, `#/library/create`, `#/c/:fid/profile`, and report feedback.

### 2. Signatures
`createCreationController`, `readApprovedCreationProgress`, `profileFields`,
`profileChanges`, `rebaseProfileFields`, `profileSaveCommand`; shared Vue
`CharacterAvatar`, `ProfileForm`, `SemanticTree`, `PreparationSteps` and
`ModelDirectory`. Existing analysis/activation and Foundation APIs remain owners.

### 3. Contracts
Select a published world/version or the default world before description,
JSON, blank or published-template creation. Timezone is required; age is optional.
Analysis stops at preview. Store editable preview UI values separately and clear
them when source/world/mode/analysis changes. Activation freezes the reviewed
typed body and original request ID; unknown outcomes replay exactly that body.
Checkpoint activation before releasing identity; returning restores progress.

Seven preparation rows read independent authorities: analysis, character/world,
wardrobe, canonical reference, character sheet, display avatar, first-day schedule.
Failed reads are unknown, never ready. Wardrobe uses dedicated preparation
recovery. Visual recovery reads identity.current_revision and visualBrief.revision.

Profile forms derive existing nested fields from real records; canonical nullable
identity/behavior controls retain schema types. Patch only changed leaves; array
edits preserve untouched array elements. Personality/Current State/Developing Self
stay read-only. Save means original proposal plus acceptance with baseline CAS.
Known stale proposals permit explicit reread/rebase; unknown writes retain identity.
Readback preserves edits made after submission, including returning to the original
value. A failed readback retains the editor and rereads rather than re-proposing.

### 4. Validation & Error Matrix
Empty description/invalid timezone → Chinese inline error, no write. Owner/role
change → discard late projection. Lost activation/acceptance → same request/body.
CAS/stale proposal → keep edits, reread and confirm new baseline. Missing values
→ hide separator/label; midnight remains valid when explicitly recorded. Explicit
unscheduled placeholders → readable pending row. Duplicate wardrobe failure →
latest attempt per role. Raw model JSON stays in diagnostic details.

### 5. Good/Base/Bad Cases
Good: submit Bob, type Alice while saving, read back committed Bob, retain Alice
as unsaved input. Base: first-use welcome links model setup and new creation.
Bad: replace the role with demo fields, or declare preparation complete from a
task status without reading its real result.

### 6. Tests Required
`approved-v62.test.mjs`, workspace conflict/recovery tests, full workspace checks,
production fixture exclusion, isolated mock build, desktop/390px browser flows.
Disposable PG timeout/retry tests prove SQL behavior; controlled browser fixtures
prove rendering/transport shape only, and do not certify a user's live models.

### 7. Wrong vs Correct
Wrong: resubmit edited live fields after an unknown activation, or compare newer
save-time input only against the original edit-session baseline. Correct: replay
the frozen command; preserve post-submit edits against the submitted snapshot.
