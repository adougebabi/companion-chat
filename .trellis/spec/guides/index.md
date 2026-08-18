# Cross-Cutting Thinking Guides

These guides apply when a change crosses the Node server, SQLite state, streaming API, and vanilla browser client.

| Guide | Use it when |
| --- | --- |
| [Code Reuse Thinking Guide](./code-reuse-thinking-guide.md) | Adding a helper, state field, renderer, or repeated provider logic |
| [Cross-Layer Thinking Guide](./cross-layer-thinking-guide.md) | Changing an API payload, SSE event, persisted state shape, or generation flow |

## Pre-Modification Rule

Before changing a constant, endpoint field, event type, or persisted key, search the whole repository for every producer and consumer. These contracts are intentionally informal and duplicated between `server.js` and `src/main.js`.
