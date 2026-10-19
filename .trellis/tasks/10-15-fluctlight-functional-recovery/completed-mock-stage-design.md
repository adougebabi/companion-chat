# Mock page acceptance design

Active baseline is the exact provided v3 native source, assets and CSS. First restore that baseline in the current app entry; then extract/port reusable presentation into components while comparing resulting DOM and visual output to the baseline. Old business Vue views are not page substitutes.

Mock state has its own localStorage namespace and one store. Page-local drafts stay separate from saved Mock state. Public control, overlay, source, state, chat and resource structures are reusable components. Page modules compose them; adapters and mock services own data mutations. No BrowserClient or real domain stores are imported by this stage entry.

All earlier backend, API, migration and integration changes are frozen, unaccepted work for a later stage. They remain in the working tree; prior PRD/design/plan are preserved in prior-real-recovery-* files. No real backend access is needed to display or operate this acceptance app.

Validation compares original v3 served at 8765 with acceptance app at 5173, both desktop and narrow. Use original selftest where applicable and component/flow coverage. New pages are separately identified so extensions cannot quietly change approved pages. Stop at user page acceptance.
