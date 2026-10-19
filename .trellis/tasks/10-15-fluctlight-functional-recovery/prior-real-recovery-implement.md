# Execution plan

User authorized creation and continuous implementation; request already fixes scope/visual/architecture/acceptance. No product decision remains unresolved. Ordinary implementation choices follow minimal compatible extensions.

1. [ ] Read exact active v3 and public Quasar pages/source; finish source baseline and create a single capability ledger.
2. [ ] Start isolated PostgreSQL/Redis/Temporal/MinIO, migrate, run Core/Worker and Vite, authenticate through real session/CSRF.
3. [ ] Fix c1/Chinese status/DTO/author/hydration faults and workflow history dispatch; regression using real IDs and actual Go response shapes.
4. [ ] About-you natural entry + ten-field editing/current Actor/CAS/history + readback/isolation/context-consumer evidence.
5. [ ] Add world/place/NPC definitions and runtime persistence/APIs, management UI and actual controlled encounter production/consumption; test no implicit relationship/movement.
6. [ ] Complete remaining confirmed core operations, especially retained old capabilities, diary/shared/notifications/attachments/Studio apply and real async execution.
7. [ ] Full real browser regression desktop/narrow, failure/conflict/isolation; fresh and prior-head migration checks.
8. [ ] Update same ledger + three status docs; review specs and cross-layer checks. Propose scoped commits only after reviewable slices are ready.

## Checks
`pnpm generate`, `pnpm typecheck`, relevant browser/Web tests, `pnpm build`; Go targeted tests with dedicated GO_CORE_TEST_DATABASE_URL and full relevant packages/vet/build. Browser Network/console and persistent GET readback are required beyond build/Mock tests.

## Risk and rollback points
Old volumes and production objects are untouched. New migrations additive. Keep each vertical slice independently reviewable; retain errors and user drafts. Do not delete existing WIP or overwrite historical schema. Record external dependency failures without marking unrun acceptance as passed.
