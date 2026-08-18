# Development Debug Observability

## Scenario: Persona-scoped local inspector

### 1. Scope / Trigger

- Trigger: a browser inspector needs prompt/job diagnostics that normally contain provider-adjacent data.
- The boundary is cross-layer: SQLite records flow through Express to a vanilla DOM renderer. Hiding a button alone is not a security control.

### 2. Signatures

- Environment gate: `COMPANION_DEBUG_INSPECTOR=1`
- `GET /api/companion/personas/:personaId/debug-context`
  - Success: `{layers, recentRequests, mediaJobs}`.
- `POST /api/companion/personas/:personaId/debug-media`
  - Request: `{kind: "image" | "video", prompt?: string}`.
  - Success: existing `createChatMediaRequest()` response containing the queued placeholder message.

### 3. Contracts

- Do not register either debug route unless `COMPANION_DEBUG_INSPECTOR === '1'`. Bootstrap exposes `debugInspector: true` only with the same condition, so ordinary UI never requests or renders diagnostics.
- Resolve the requested persona first and use `persona_id = ?` in every debug query. Do not parse/summarize another persona's rows and filter afterward.
- Return at most 10 recent request records and 10 media jobs. Limit every rendered summary to 2,000 characters.
- Apply recursive redaction before truncating. Redact values stored under keys matching `apiKey`, `authorization`, `token`, `secret`, `password`, `credential`, or `cookie`, and redact bearer/key-like values embedded in strings.
- Never expose `settings()` or raw provider headers. Provider state is represented only by safe configuration booleans or model identifiers.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Debug flag is absent or not `1` | Route is not registered; Express returns `404`. |
| Persona does not exist | Existing `requirePersona()` error contract (`404`, `{error}`) applies. |
| Test media kind is not `image` or `video` | Existing media request validation returns `{error}`; no browser-to-provider call occurs. |
| Sensitive key/value appears in an inspected record | Return `[redacted]`, never the original string. |

### 5. Good / Base / Bad Cases

- Good: an explicitly enabled local inspector fetches the selected persona's last ten jobs and shows a redacted prompt/workflow summary.
- Base: no messages or media jobs produces empty arrays; the inspector still renders normally.
- Bad: a production-like process with no debug flag cannot call an unregistered debug URL even if a user guesses it.

### 6. Tests Required

- Import the server with `COMPANION_DEBUG_INSPECTOR=0` and assert neither debug route is registered.
- Import it with `COMPANION_DEBUG_INSPECTOR=1` and assert both routes are registered plus the debug DTO renders the selected persona.
- Seed two personas; assert the first persona's DTO never contains the second persona's request/job data.
- Seed nested secret-bearing JSON and bearer-style text; assert recursive redaction removes secrets and the 2,000-character cap holds.
- Assert test media dispatch returns the existing queued message contract rather than directly invoking ComfyUI from the browser.

### 7. Wrong vs Correct

#### Wrong

```js
// A hidden client button is not a server-side access boundary.
app.get('/api/companion/personas/:personaId/debug-context', route(showEverything));
```

#### Correct

```js
if (process.env.COMPANION_DEBUG_INSPECTOR === '1') {
  app.get('/api/companion/personas/:personaId/debug-context', route((req, res) => {
    res.json(debugContextFor(req.params.personaId));
  }));
}
```

