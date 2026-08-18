# Logging And Debug Traces

## Two Channels

Operational failures use `console.warn`, such as skipped evolution or generation queue work. User-inspectable model/provider traces use the bounded `debugLog` and `generationLog` arrays, exposed by `GET /api/console` and cleared by `DELETE /api/console`.

`appendDebug(state, event)` adds an ID and ISO timestamp, then keeps the newest 160 entries. Include `type`, `phase`, `personaId`, and `traceId` where applicable. Chat uses `input`, `output`, and `error`; memory review and function calls follow the same vocabulary (`server.js:114-118`, `217-285`, `467-555`).

## What To Record

Record enough context to correlate a model request and its result: persona identity, model selection, trace ID, parsed tool calls, job IDs, and concise errors. Generation records include the injected prompt target and ComfyUI prompt ID.

## What Not To Record

Never add API keys, authorization headers, cookies, or unredacted credentials to logs. Treat conversation text, attachments, and prompts as local user data: only record them where the existing console feature explicitly needs them, and keep bounded retention.

## Verification

When changing a trace shape, update `consoleItem()` in [`src/main.js`](../../../src/main.js) and check `/api/console`; otherwise the backend may persist fields that the UI cannot render.
