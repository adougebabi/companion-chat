# Approved HTML demo migration (2026-10-23)

> Current implementation (2026-10-26): React shadcn/ui + Tailwind CSS + Radix UI
> + Lucide React. [React shadcn Contract](./react-shadcn-contract.md) owns the
> technology boundary. Vue and old page/mount code have been removed;
> legacy=1 is rejected. Appearance, route intent and domain contracts below
> remain applicable; historical implementation mechanisms are superseded.


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

R6 wearing follow-up: cold entry/refresh of 此刻 loads actual wardrobe state
independently of other life modules. Missing/malformed worn_items is a failed
read, not an empty outfit; keep the last confirmed projection. Valid [] remains
authoritative empty. Resolve a worn description from its actual read row when
its ID is absent from the current catalogue. Outfit request receipts/feedback
bind to the original operation and target card, retain original request links,
and distinguish business failure from a post-commit readback failure.
An accepted request sets requestUnresolved until a domain terminal status is
observed. A status transport error retains its receipt identity, blocks another
wear submission and offers recovery of the same request. Restored legacy
checkpoints also remain unresolved; recover their operation from the actual
inventory_outfit payload. Unknown-target errors stay visible in page feedback.
Terminal readback errors use readbackError/refreshResult separately from the
domain status, reason and original diagnostic link. Re-reading results must
never rerun the mutation. r6-wear-readback.test.mjs executes these transitions,
including a genuinely recreated workspace and rendered production WardrobePage.

R7 retains the latest request identity per Owner/character/outfit in sessionStorage
after settlement, then re-reads that original request after refresh. Store only
identity/routing/terminal marker, never cached business outcomes. Known pointers
and legacy unbound pointers converge by original submission order, not GET finish
order. Reattach an existing observation after an Owner round trip and avoid
duplicate operation records. In-flight outfit submission also keeps one command
identity before 202. Only completed+disposition=no_change means already wearing;
accepted without a mutation remains accepted. Use the character/outfit names in
wardrobe copy and accessible button names instead of demo numeric IDs/gender.

The r4 follow-up in `docs/design/v62-r4-remediation.md` records production-shaped
coverage: saved-setting status stays beside the button through receipt readback;
unused timeout defaults to 120 seconds. Failed preparation successors explicitly
name the blocker while strict serial admission remains unchanged. Notification
groups start collapsed, preserve all notice identities, and normalize only a
single leading hash for internal navigation. Blank portrait scalar lines are
display-only translations; common/profile and flat/carrier exact aliases appear
once, while distinct facts and unknown values remain available.

## 2. Signatures

- `main.tsx` mounts `react-app/App.tsx`; current routes resolve to React pages.
- `approvedRoute` / `approvedHref` retain canonical routes and explicitly reject
  `legacy=1`, including duplicate and encoded query keys. No old app can mount.
- `feature-routes.mjs` owns explicit object-level selectors and named domain
  loaders. React `DomainPage` owns mounting, Owner/generation fences, validation
  and cleanup. Retained domain modules never install an app, router or shell.

- `ActionButton`, `Surface`, `CurrentOutfit`, `ActionFeedback`, `FilterChips`,
  `DetailDrawer`, `GarmentArt` are React business compositions of library primitives.
- `configurationPlan(snapshot)` validates the full immutable snapshot before
  any write; `acknowledgeConfiguration(state,operation,receipt)` updates only
  acknowledged inputs/baselines and `receipt.applied_versions`.
- `stagedCommand({storage,key,live}).stage(name,create,execute)` journals the
  immutable request body and identity of each world-expansion stage.

## 3. Contracts

Use React and the existing real domain services. Preserve approved appearance
and native form semantics while Radix owns compound interactions. Production
assets exclude fixture data, old QA pages and old renderers. React reconciles
stable input nodes; server updates must not remount the entire app or composer.

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
| Isolated fixture | The same React production UI reads controlled test APIs |
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
canonical/deep routes, domain configuration and routing invariants. React DOM regressions cover
primitive semantics, filtering, input continuity and repeated page transitions and quiet policy planning.
`approved-workspace.test.mjs` verifies stale reads, role round-trip retry, later
stage validation retention, receipt checkpoints and visibility resume.

Run generation, workspace typecheck/tests/build, isolated production-shaped fixture, targeted Go
quiet-hours race tests, and browser checks through a static production build.
Controlled API fixtures and screenshots are visual/contract evidence, not a
substitute for live PostgreSQL/model acceptance. Record baseline failures as such.

## 7. Wrong vs Correct

Wrong: install shadcn while the default entry still uses old renderers.
Correct: connect React primitives to real pages and execute route/data/input
updates against the production build.

## Scenario: v6.2 initialization and character details

The [r3 recovery follow-up](../backend/r3-recovery-contract.md) adds optional unused
model bindings, credential-free connection drafts, saved-only directory feedback,
explicit diary dates/terminal receipts, exact-source notification previews and
cancelled-analysis fencing. It retains single initialization generation and the
existing no-token-cap policy.

The follow-up [durable preparation contract](../backend/initialization-preparation-contract.md)
adds async saved-source compilation, serial bootstrap dependencies and editable
error recovery. Creation drafts and original commands now use Owner-scoped
localStorage; archived per-character progress has an independent command key.
The seven preparation rows include persona compilation in the character/world
row. Profile detail shows background progress and recovery; observation pauses
while editing/saving or hidden. Known validation rejection unlocks source
editing; unknown outcomes retain immutable activation identity.

### 1. Scope / Trigger
New/empty library, `#/library/create`, `#/c/:fid/profile`, and report feedback.

### 2. Signatures
`createCreationController`, `readApprovedCreationProgress`, `profileFields`,
`profileChanges`, `rebaseProfileFields`, `profileSaveCommand`; shared React
`CharacterAvatar`, `ProfileForm`, `SemanticTree`, `PreparationSteps` and
`ModelDirectory`. Existing analysis/activation and Foundation APIs remain owners.

### 3. Contracts
Explicitly select an existing world's configuration, a published world/version,
or the default world before description, JSON, blank or published-template
creation. New drafts do not silently choose the default world. Timezone is
required; age is optional.
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
production fixture exclusion, isolated production-shaped fixture, desktop/390px browser flows.
Disposable PG timeout/retry tests prove SQL behavior; controlled browser fixtures
prove rendering/transport shape only, and do not certify a user's live models.

### 7. Wrong vs Correct
Wrong: resubmit edited live fields after an unknown activation, or compare newer
save-time input only against the original edit-session baseline. Correct: replay
the frozen command; preserve post-submit edits against the submitted snapshot.

## R9 recurring regression contracts

Both retained creation renderers use gender-neutral headings and name labels.
An empty schedule exposes original-task recovery; failed creation schedule
stages are retryable while blocked later image stages remain blocked. Recovery
uses the same persisted command body/key and never activates another character.

A new Studio definition can have no update diff and still need creation. Consume
`operation=create` / `create_required=true`, show a creation explanation and keep
Confirm enabled. Do not display "already consistent" or infer existence from an
empty diff. Place definitions have ordinary Chinese fields and optional advanced
JSON. Blank floor remains unknown; blank furniture/layout labels stay hidden.
Saved item kind/category/slot reflect submitted/server-read values; a newer local
draft remains separate. Studio's main page lists real resource records with edit
and creation entries. Memory copy distinguishes idle conversation segments from
formal daily memories; chat alone does not fabricate an immediate memory record.

Node tests cover recovery identity, fields and saved selections. Isolated
production-browser fixtures cover visible controls/readback, not real database
or model success; retain separate disposable-PG and opt-in model evidence.

## R10 single-workspace and configuration reuse

The old app, shell, registries, LegacySurface and proto/pages directory are
removed. Do not restore a second production application for missing features.
Unique editing/history/recovery functions live under v3-domain and inherit the
React workspace's tokens; their imperative forms remain an explicitly tracked
custom-control migration batch. Native React main lists are the only production
lists. Tests exercise actual React pages and shared controllers, without Vue or
SFC harnesses. A dependency-graph test rejects imports of retired apps, proto
pages/modules and old shells; React view components are allowed.

At 390px the sidebar is a closed drawer and the chat preview starts collapsed;
the chat composer must fit in the first viewport. Settings switches are 36×21px,
checkbox labels use adjacent text, and the save bar stays in document flow.
Media card images fill their 108px preview using cover; the preview uses block
layout so an intrinsic grid row cannot stretch a tall image. Detail images use
contain in a substantially taller frame. Media defaults to all characters,
including entry from a character bookmark; explicit fid filters remain valid.
Zero-character navigation still permits world definitions, feed and media.

Existing-world reuse copies only saved world attributes and places into fresh
runtime IDs. It remaps place parents/connections, establishes independent place
baselines, and copies no NPCs, inventory, event templates, runtime position or
life history. Core verifies common Owner and source revision and creates the
copy atomically through the existing world command ledger. Copy, select and
readback are serial persisted stages: unknown outcomes replay the exact original
body/key. Only a first definite admission rejection permits explicit review and
confirmation of a new source baseline; a conflict following an unknown outcome
does not release the original command. No schema migration is required.

`single-workspace`, `r10-active-*`, `r10-native-profile-equivalence`,
`r10-media-default`, `r10-account` and `r10-world-reuse` cover these boundaries.
`TestR10WorldConfigurationCopyIsIndependentAtomicAndReplayable` uses disposable
PostgreSQL. Browser evidence at 1280×800 and 390×844 is separate from model or
live-business-data acceptance; see docs/design/v62-r10-remediation.md.
