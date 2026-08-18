# Cross-Layer Contract Guide

Use this guide for anything that moves through Express, SQLite, an external provider, and the browser.

## Map The Flow

```text
browser event
  -> fetch /api route
  -> readState / provider call
  -> saveState or SSE event
  -> main.js state update
  -> render()
```

For each boundary, write down the exact JSON keys, ownership, and failure behavior. The chat path additionally has upstream MTPLX SSE -> server SSE translation; generation has tool call -> queued job -> ComfyUI history -> attachment message.

## Contract Checklist

- Does a new persisted field have a `defaultState` fallback?
- Does the route validate it and return the documented status/error shape?
- Does an awaited worker re-read state before merging its result?
- Are SSE `token`, `done`, and `error` events still parseable by `streamChat()`?
- Does the frontend render the field safely and recover it after `refreshState()`?
- Are logs bounded and free of credentials?

## High-Risk Boundaries

1. [`server.js:441-569`](../../../server.js) and [`src/main.js:434-510`](../../../src/main.js): chat request, streaming, and optimistic messages.
2. [`server.js:517-555`](../../../server.js) and [`src/main.js:512-531`](../../../src/main.js): tool calls and generation job creation.
3. [`server.js:622-710`](../../../server.js) and conversation rendering: serialized ComfyUI work and eventual attachment replacement.
4. [`server.js:66-89`](../../../server.js): additive state compatibility and legacy migration.

When a change touches one of these paths, verify success, provider failure, refresh recovery, and an empty/malformed input case.
