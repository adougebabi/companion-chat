# Initial read-only code findings

- Runtime probe: no Core/Web/dependency services running. Root .env has retired keys; Compose private env absent. Dedicated new volumes required. Current migration head 0063_goal_agent_cadence.
- Active entry App.vue:212 mounts v3 proto/app.js, old Vue PeopleView is unmounted.
- pages/common.js:16 allows only c1; chat.js:125 allows Chinese 正常 instead of active. Other pages hardcode c1/outfit.c1.
- contract-map.mjs:264 overwrites global user.background per Fluctlight; actual detail schedule/relationships/memory fields differ from fixture shape.
- proto/store.js:18 optional swallows 401/404/501; hydrate only commits after all modules. renderOwner updates unhandled summary and immediately toasts success.
- Actor facts/context and Owner background CAS/source/cache invalidation already exist; use existing authority.
- browser_backend.go:474 generic /history branch precedes workflow history and needs conversation prefix.
- owner_surface.go:283 world/nearby/place placeholders. Studio resources only draft, application recorded_not_applied, no immutable body snapshots.
- Actor table supports type storage but several runtime checks only human/fluctlight; NPC integration must explicitly handle scope without granting arbitrary turn authority.

Findings are static evidence, not runtime acceptance. Original agent reports in current conversation include exact symbols/line anchors; main reads exact code before edits.
