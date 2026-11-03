# Design — Fluctlight Frontend Audit

## Boundary

This is a read-only product audit followed by documentation authoring. The only planned repository additions are six Markdown files under `docs/product/frontend-audit/` and the task's Trellis planning artifacts. No frontend/backend source, migrations, generated clients, runtime settings, or business behavior changes are in scope.

## Evidence method

- Treat the current `apps/web` Vue application, `packages/browser-client`, the Go Core browser boundary, Core services and migrations as implementation evidence.
- Use existing `.trellis/spec/{frontend,backend}` and architecture documents to explain contracts, but distinguish contract text from executable implementation. Historical task requests and stale specs cannot prove current behavior.
- Trace each important UI operation from rendered control/event handler through Pinia/client to the browser endpoint and backend owner where available. Record UI-only draft/local state separately from persisted server state.
- Assign A–F only to actual feature instances. Use “待核验” and state missing evidence when the available source does not close the path.
- Separate confirmed relationships from proposed product concepts; in particular, do not imply resource entities for World/Scene/Location, NPC, or global item catalogs without schema/API evidence.

## Document architecture

1. `01-current-pages.md` is the navigation and surface inventory, including manual pathname handling, query sections, embedded governance, overlays, data sources, and cross-page transitions.
2. `02-feature-inventory.md` is the feature-level source of truth; stable IDs point to exact frontend and backend evidence. Other documents should refer to these IDs where useful.
3. `03-backend-ui-gaps.md` maps implemented backend owners to browser routes and UI coverage, separating reusable APIs from missing owner-management APIs.
4. `04-domain-inventory.md` records authoritative entities, persistence, relationships, and explicit unknowns.
5. `05-restructure-risks.md` extracts preservation constraints from verified flows and contracts.
6. `06-audit-summary.md` summarizes feature distribution and planning questions without proposing a visual system or starting the next phase.

## Status semantics

- A: frontend and backend are connected with an effective interaction path.
- B: both sides exist, but the user-facing flow or management loop is incomplete.
- C: frontend-only interface or simulated behavior without a real backend effect.
- D: backend capability exists but has no frontend entry.
- E: design/docs exist without effective implementation.
- F: legacy/disabled or insufficient evidence to verify; explain which meaning applies.

The status is about product interaction closure, not implementation quality. Where a backend API exists but the UI action is absent, choose D; where both sides are present but UI covers only part, choose B.

## Compatibility and verification

Use current concrete route, BrowserClient, Pinia, NDJSON, localStorage, polling, media proxy, and server-authoritative projection behavior as evidence. Document current contracts that a future UI rewrite must preserve. Validate documentation completeness, route coverage, cross-document consistency, and links/line anchors by source inspection; no test suite or product code change is planned.
