# Frontend Guidelines

The active client is **React + TypeScript/JSX, shadcn/ui (Radix base), Tailwind
CSS 4 and lucide-react** under `apps/web/`. `index.html` loads `src/main.tsx`;
`react-app/App.tsx` owns the shell and explicit hash routes. Vite builds to
`apps/web/dist/`, served by Nginx. Local preview is not another implementation.

The Owner explicitly required removal of Vue and old UI entry points on
2026-10-26. Do not restore Vue/Pinia/Reka, SFCs, compiled Vue render bodies,
`LegacySurface`, `mountProduct`, the old proto page/module router or QA HTML.
Every `legacy=1` route is rejected before page dispatch, including duplicate
query values. It is not redirected to another old interface.

The supplied 2026-10-27 workspace Demo is the visual baseline for migrated pages;
read [Workspace Replica Contract](./workspace-replica-contract.md). Existing domain
behavior remains authoritative.
Read [Approved Demo Contract](./approved-demo-contract.md) together with the
current [React shadcn Contract](./react-shadcn-contract.md).

## Guides

| Guide | Use it for |
| --- | --- |
| [Workspace Replica Contract](./workspace-replica-contract.md) | Current Demo source, page sequence, layout and state verification |
| [React shadcn Contract](./react-shadcn-contract.md) | Required stack, real adoption, retired route gate, controller and primitive contracts |
| [Directory Structure](./directory-structure.md) | Page, view, state, service and UI ownership |
| [Component Guidelines](./component-guidelines.md) | React events, composition, forms and accessibility |
| [State Management](./state-management.md) | Domain state, request identity and readback fences |
| [Product UI Contracts](./product-ui-contract.md) | Fields, errors, guided configuration and diagnostic drafts |
| [Confirmed Interactions](./pen-interaction-contract.md) | Reading state, drafts and independent configuration slots |
| [Goal / Actor Governance](./goal-planner-governance-contract.md) | Versions, dependencies, Actor CAS and source review |
| [Command Receipt / Draft Recovery](./command-receipt-draft-recovery.md) | Goal/World acknowledgements, late receipts, GET-only recovery and native event timing |
| [Feedback UX](./feedback-ux-contract.md) | Six semantic kinds, field errors, diagnostics, event lifetime and real recovery actions |
| [Quality Guidelines](./quality-guidelines.md) | Production/browser and contract verification |

## Pre-Development Checklist

- Find the owning React page/view and domain controller before editing.
- Reuse the local `components/ui/*.tsx` primitive; add from the official shadcn
  **Radix** registry when absent. Installing a package alone is not adoption.
- Follow generated BrowserClient payloads and existing providers; no component
  may invent an API or persist a second authoritative business snapshot.
- Separate immutable render snapshots, editable drafts and frozen commands.
- Render external text through JSX. Keep raw records in explicit disclosures.
- Read the specific contracts relevant to the page, not only this index.

## Quality Check

Run Web typecheck, tests and build. Build runs both the production fixture guard
and `verify-react-stack.mjs`, which verifies dependency/import/HTML entry rules.
Use the actual production dist with the isolated UI fixture for browser checks.
Exercise relevant desktop/narrow layouts, input/IME, focus, route changes,
role switching, errors and original-request recovery. Fixture evidence cannot
certify live PostgreSQL, Temporal, model or NAS deployment behavior.
