# Implement — Fluctlight Frontend Audit

## Ordered checklist

1. Complete source inventory for the five manually selected top-level views, nested settings/diagnostics sections, dialogs, embedded GovernanceView, route fallbacks, auth, Pinia stores, BrowserClient and media/realtime paths.
2. Trace the important user operations into BrowserClient calls and Go Core browser route/service implementations; separately mark local-only behavior, partial paths, absent entries, and missing evidence.
3. Build the feature matrix with stable IDs and required columns, grouped by domain; reconcile its page names and route paths with the page inventory.
4. Document backend/UI gaps and core entity/persistence relationships, explicitly separating implemented fields/relations from resource models that do not exist and from proposals.
5. Extract restructure preservation risks from verified interaction flows and current API/state/stream/media contracts.
6. Write the six requested Markdown deliverables under `docs/product/frontend-audit/`.
7. Perform a documentation-only review: verify all current routes and important controls have coverage; confirm every A–F assignment has evidence; check backend mapping, entity claims, line anchors, unknown markers, cross-document terminology and adherence to scope.

## Validation

- [x] Six deliverable files present; 53 unique Feature IDs across the matrix with all 12 required columns.
- [x] 272 cited source anchors resolve to existing files and in-range line numbers.
- [x] Summary status totals match the matrix: A=34, B=8, C=0, D=4, E=1, F=6.
- [x] Git status shows only this Trellis task record and `docs/product/frontend-audit/`; no frontend/backend business files changed.

- Use `rg`/source reads and targeted line inspection for every material claim.
- Check all route registrations/dispatch branches against `01-current-pages.md` and all API clients/store operations against the feature matrix and gap map.
- Check matrix columns, feature ID uniqueness, and that every backend/UI status matches the linked UI and backend evidence.
- Check no business code or generated artifact changed; do not run tests or builds because the deliverable is an evidence report and no code implementation is in scope.

## Risk / rollback points

- Main risk is undercounting a route, modal, query section, or nested action. Mitigate with whole-tree route/operation inventories and cross-document reconciliation.
- Main risk is conflating documentation/contracts with current runtime. Label docs-only contracts and prefer current Go/API implementation where they differ.
- Main risk is treating text fields, UI group names, or loosely typed JSON fields as domain entities. Require schema/service evidence for confirmed models and mark alternatives as proposals.
- If evidence conflicts or an operation cannot be followed to a persisted source, downgrade the claim to B/F or “待核验” with the exact missing link; do not fill gaps by assumption.
