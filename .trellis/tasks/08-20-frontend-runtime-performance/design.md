# Technical Design: Vue Client Rewrite

## Scope And Final State

Rewrite the active client once in Vue 3 + TypeScript + Vite + Pinia. The current `src/` client is a temporary migration reference only. The final application is `web/` source compiled to `dist/`; Express and Docker serve only `dist/` and API routes. The old `src/` directory and old entry scripts are deleted before completion.

The rewrite preserves API paths, response fields, SSE event names, SQLite-backed behavior, existing information architecture and current visual language. It includes technical layout stability and state feedback, but not a new visual design system.

## Source Layout

```text
web/
  index.html                    static shell and loading skeleton
  src/
    main.ts                     Vue app bootstrap
    app/App.vue                  top-level shell and view router
    app/router.ts                contacts/activity/chat/settings views
    api/
      client.ts                 typed fetch wrapper and errors
      contracts.ts              API/SSE DTO types and guards
      conversations.ts          cursor-page requests
      activities.ts             activity API
    stores/
      app.ts                    bootstrap, settings, instance list, active view
      conversations.ts          per-instance message pages/cursors
      activities.ts             feed pages and mutations
    composables/
      useChatStream.ts          SSE reader and frame-coalesced output
      useMessageHistory.ts      20-item pages, sentinel, anchor restoration
      useComposer.ts            draft, selection, IME composition, submit
      useBootstrap.ts            contacts-first boot and polling guards
      useActivities.ts          activity loading and pagination
      useDialog.ts               focus return and modal lifecycle
    components/
      shell/
      contacts/
      chat/
      activity/
      settings/
      persona/
      media/
      inspector/
      feedback/
    views/                       Contacts, Chat, Activity, Settings
    styles/                      migrated existing tokens/layout only
    types/                       shared UI/domain DTO types
vite.config.ts
tsconfig.json
```

Feature modules own rendering and feature interactions; stores own shared server/app state; composables own asynchronous side effects. No component performs raw fetch, parses SSE, or mutates another feature's state directly.

## State Model

### Pinia app state

```ts
type AppState = {
  boot: 'idle' | 'loading' | 'ready' | 'error';
  personas: PersonaSummary[];
  groups: ContactGroup[];
  settings: PublicSettings;
  currentView: 'contacts' | 'chat' | 'activity' | 'settings';
  activePersonaId: string | null;
};
```

### Per-instance conversation state

```ts
type ConversationState = {
  items: Message[];
  nextCursor: string | null;
  hasMore: boolean;
  loadingInitial: boolean;
  loadingOlder: boolean;
  historyError: string | null;
  stream: {status: 'idle' | 'sending' | 'done' | 'error'; pendingId?: string};
};
```

Draft text, selection, composition state, scroll anchor and focus are owned by `useComposer`/`useMessageHistory`, not persisted in Pinia server state. A background refresh cannot replace a composer DOM node or clear a draft.

## Boot And History

```text
static web/index.html shell
  -> bootstrap
  -> render contacts immediately
  -> user selects 摇光实例
  -> GET latest 20 messages
  -> scroll-top sentinel requests older 20 by cursor
```

The initial page never fetches a conversation. `MessagePage` merges older pages at the head and new messages at the tail, deduplicating by message ID. Before inserting older messages, capture the first visible message ID and offset; after insertion, restore that anchor. Reaching `nextCursor = null` disables the sentinel. A failed page shows a retry affordance without replacing the composer.

## Streaming Chat

`useChatStream` reads the existing application SSE contract:

```text
token -> append to transient assistant text node
done  -> replace transient message with ordered done.messages
error -> remove transient entry, preserve composer text, show error state
```

Token updates are coalesced to one DOM write per animation frame. Streaming displays safe plain text; final reconcile renders Markdown/attachments once. The composer is never recreated during a stream. Auto-follow occurs only when the reader was already near the bottom.

## Composer And Mobile IME

`useComposer` tracks `draft`, selection, `isComposing`, `isSending`, and submit status. `compositionstart` blocks chat redraws; `compositionend` resumes safe updates. Only an explicitly accepted send clears the draft. Failed requests keep the original draft. Polling never runs while the document is hidden, a send is active, the textarea is editing, a draft exists, or IME composition is active. No `window.focus` listener triggers a chat render.

## Media And Activity Performance

- Images use `loading="lazy"`, `decoding="async"`, and dimensions/aspect ratio when available.
- Videos use `preload="none"` until user activation.
- Media containers reserve space to prevent layout shifts.
- Activity pages consume batched DTOs and use cursor pagination.
- Activity and chat rendering updates only changed feature regions; no whole-app `innerHTML` replacement during normal interaction.

## Build And Delivery

- Vite dev server proxies `/api` to Node/Express.
- Production `vite build` emits hashed `dist/assets` and a manifest.
- Express serves `dist/` with short-lived HTML and long-lived hashed assets; API remains `no-store`.
- Docker builds the web app before copying `dist/` into the runtime image.
- CI runs `vue-tsc --noEmit`, `vite build`, backend tests, API smoke tests and browser smoke tests.

## Compatibility And Deletion

Use typed API/SSE fixtures to verify the new client against the existing server. During migration the old client can be opened for comparison, but it is never the final fallback. After the new client passes all flows, remove old `src/`, old scripts, old static references and old checks; run a second full browser/API regression after deletion.
