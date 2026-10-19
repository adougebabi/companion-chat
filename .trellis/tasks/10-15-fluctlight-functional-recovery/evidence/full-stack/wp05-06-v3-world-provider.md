# WP05/06 current v3 world/NPC adapter slice

## Boundary and registration

This slice keeps the current proto grid/card/form/routine/drawer components and CSS.
It does not mount WorldWorkspace or an older Vue page. No Go, generated client,
shared store, app/main, or media functions were changed by this slice.

`apps/web/src/v3/providers/world.ts` exports
`installWorldProvider({client,refreshProduct})`; the main session owns invoking it
from `providers/real.ts::registerProviders`. The install registers explicit `world`
and `npc` services plus read projections. Production registration must be verified
in the final combined build; an isolated provider test alone does not prove it.

The adapter imports existing BrowserClient types. World/place/NPC/resource fields
are normalized centrally. UI Chinese routine days and position modes map explicitly
to `daily/weekday/weekend` and `routine/fixed/unknown`. Array collections survive a
text-editor round trip. Place parent and stable connection IDs have explicit inputs;
connection prose is never interpreted as an identity.

## Connected actions

- World catalog reads all owned worlds and nullable active binding. Creation does
  not select. Editing and inspecting an unselected world use catalogWorld. Selection
  uses the catalog binding revision; selection and switching back reread authority.
- World/place saves and deletions use real commands/revisions. Reference errors are
  visible. World/place draft conflicts preserve fields until explicit rebase. Parent
  and connection identity checks remain server owned. Creation retry retains its
  idempotency identity until mutation plus readback succeeds.
- NPC directory/detail/edit read real runtime scope. Runtime bio, role, home, enabled,
  archived, position mode and routine are saved with runtime CAS then reread. Deletion
  uses the real reference-protected endpoint. Unconfigured location stays unknown.
- NPC global definition has create/save draft, draft-version CAS, publication CAS,
  explicit re-read preserving local draft, and application drawer. Application uses
  actual diff plus per-field keep/override, and freezes the publication/revisions
  that were previewed. It does not auto-select a world, establish acquaintance or
  move the Fluctlight.
- NPC detail reads actual encounter event/relationship/memory IDs, source evidence,
  relationship context/history, confirmed position and planned position separately.
  Relationship edits and restore use existing real controlled commands, stable Actor
  identity, profile scope, revision and evidence, retaining other metrics.
- The former demonstration encounter action now requires an Owner confirmation of
  an actual historical contact, actual time interval, place, summary and evidence.
  It sends the current Life Context revision and a stable modal idempotency key.
  It is explicitly manual history recording; automatic encounter processing is not
  claimed by this slice.
- Map/place/scene and people pages load real authority before rendering. Nearby
  distinguishes confirmed versus predicted NPCs; current Fluctlight location also
  labels authority. Merely reading these pages issues no command.
- Retired roles may inspect world/people read surfaces; adapter rejects their runtime
  writes. World catalog forms are disabled when retired. Owner changes/logouts clear
  cached scope; late refreshes are fenced. Character and current world must both match
  when displaying an NPC, including after inspecting an inactive world.

## Checks and limits of evidence

- Web full suite: `pnpm --filter @fluctlight/web test`: **156 passed** (at this slice's
  integrated checkpoint). Follow-up focused world/route/fixture checks: **22 passed**.
- Web typecheck and build passed. Build verification reports no fixture store, seed,
  diagnostic samples, debug writers or QA page in production.
- New executable `apps/web/test/v3-world-provider.test.mjs` uses the actual typed
  BrowserClient plus adapter with an explicit fetch fixture. It verifies catalog and
  CAS wire payloads, creation without selection, switch/back, no writes from browsing,
  unknown location, NPC mode/day mapping, deletion rejection, absent default restore,
  application pinned to preview publication/revisions, publication CAS, idempotency
  fields, and Owner logout cache cleanup. This is not a real DB/Worker acceptance run.
- Browser client world/v3 tests: `node --import tsx --test
  test/world-contract.test.ts test/v3-contract.test.ts`: **9 passed**. The normal tsx
  CLI launcher was sandbox-blocked on its IPC pipe; the direct import executes the
  same test files without that IPC launcher.
- Source syntax checks passed for all changed JS renderers and world/people routes.
- Existing T04 PostgreSQL evidence remains in `t04-world-catalog-0065.md`. This slice
  did not modify Go or add database test claims. Main session owns combined real
  HTTP/browser, refresh persistence, and 1440/390/320 visual verification.

## Remaining real gaps / follow-ups

1. No real per-place built-in-default snapshot/restore endpoint exists in the current
   client/server contract. The existing restore action remains and reports this
   precise limitation without clearing local/saved content or resetting the world.
2. The existing world-template enabled command returns `runtime_effect:
   preference_only`. The page submits and rereads that actual preference; it does
   not claim a scheduled event ran. The original frequency/category aggregate
   persistence/automatic scheduler is not provided by this contract and requires
   the relevant backend/event-template slice. No local synthetic setting save remains.
3. Automatic arrival → opportunity → decision → durable encounter consumption remains
   the independently assigned backend slice. Owner historical contact is separate.
4. Current BrowserClient NPC list has no cursor input; adapter refuses a nonempty
   next_cursor rather than silently presenting a partial list. Current Core returns
   `next_cursor:null`; a future paginated server needs a generated cursor argument.
5. NPC create/edit definitions, map images, source links, true error recovery and
   retirement need final real-browser inspection after main registration. This report
   does not mark WP05, WP06 or WP12 wholly complete or claim user acceptance.
