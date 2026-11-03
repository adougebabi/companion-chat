# Integration Design

The product is delivered through two independently verifiable child tasks.

- `08-17-lifelike-ai-companion` owns the table-backed companion domain, persona interview, layered context, event engine, job queue, activity contract, and lifecycle inspector.
- `08-17-telegram-chat-interface` owns the bright responsive presentation, chats/activity navigation, activity views, composer ergonomics, and visual QA.

The interface consumes only user-safe domain projections. Stable cross-task contracts are: persona summary/current event, cursor-paged activities, immutable post/media state, scoped comments/reactions, screen state, direct unread count, activity red-dot watermark, and lifecycle-inspector data.

Implementation order is domain persistence and API contracts first, then interface integration. The interface can repair existing mobile defects independently while the companion APIs are built.
