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

## Scenario: Durable local-command media progress

### 1. Scope / Trigger

- Trigger: a server-owned media provider such as `h3` runs a long local command and the local inspector must reveal the final provider prompt plus real execution activity.
- This crosses the durable SQLite job, a leased worker, child-process stdout/stderr, the debug DTO, and the inspector card UI.

### 2. Signatures

- `recordMediaJobProgress(job, patch) -> {changed, progress?, result?}`
- `createMediaProgressReporter(job)` exposes `stage()`, `output()`, and `flush()`.
- `companion_jobs.result_json.progress` contains `{schemaVersion, attempt, stage, percent, startedAt, updatedAt, elapsedMs, latestOutput, latestStream, outputSeen, outputLineCount}`.
- `GET /api/companion/personas/:personaId/debug-context` includes `mediaJobs[].finalPrompt` and `mediaJobs[].progress`.

### 3. Contracts

- The only prompt presented as the final provider prompt is the compiled prompt persisted immediately before provider submission; raw `mediaIntent` remains secondary diagnostic information.
- h3 receives `progress` through the server-owned provider boundary, reads both stdout and stderr, and stores only the latest normalized line. Explicit `%` values may set `percent`; time elapsed must never synthesize a percentage.
- Progress writes use the same `status = 'leased'`, `lease_owner`, and non-expired lease guard as settlement. They never renew a lease or alter status/attempt/retry scheduling.
- Output is ANSI/control-character cleaned, recursively credential-redacted, path-redacted, and capped at 480 characters. Do not persist whole command lines, argument arrays, model/output paths, or stream history.
- Completion and failure merge existing `result_json` so final prompts and h3 progress survive terminal settlement. A retry starts a new `attempt` snapshot.
- `activity_media_poll` and `chat_media_poll` are internal worker children, not separate user-visible media tasks. Debug projection groups them by their shared `activity_id` or `message_id`, returns the source media job once, and overlays the newest poll child’s status/error/external ID.
- `simplifiedMediaMode` is a persisted client-display setting. It may prevent chat `<img>`/`<video>` creation, but it must not stop queueing, provider execution, attachment persistence, or this local inspector.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Provider outputs `12.5%` | Store/display `percent: 12.5`. |
| Provider outputs text but no percentage | Store stage, elapsed time, and latest output; display no reported percentage. |
| Old/expired lease emits output | Guarded update changes zero rows; it cannot overwrite a later attempt. |
| h3 exits, times out, or lacks output media | Preserve a `failed` terminal progress snapshot and use the normal retry/failure policy. |
| Legacy or ComfyUI job has no progress | Return a stable fallback based on job status; do not require an h3-shaped record. |
| Source job has an active poll child | Return one source-job media DTO with the child’s effective status; do not return a second poll card. |
| Simplified mode is enabled | Chat does not load media URLs, while the durable job and inspector remain available. |

### 5. Good / Base / Bad Cases

- Good: a running h3 job shows “正在生成视频”, its real elapsed time, a safe latest line, and an explicit percentage only when h3 printed one.
- Base: a queued ComfyUI job still displays its final provider prompt and ordinary job status with no fabricated progress.
- Bad: append raw stdout indefinitely to `result_json`, calculate `62%` from elapsed time, show an internal poll as a second media task, or let a stale child process write after its lease expired.

### 6. Tests Required

- Assert ANSI/CR output is normalized, credentials and absolute paths are redacted, and explicit percentages are clamped.
- Assert stdout and stderr both reach the reporter, high-frequency output is throttled while the latest line survives a flush, and the line count remains bounded.
- Assert stale leases cannot write, a failed attempt retains `failed` progress, the next attempt resets percent/output count, and h3 completion preserves the final prompt plus `complete: 100%`.
- Assert debug-context remains persona-scoped and limits displayed output.
- Assert an active poll child produces one aggregated source-job DTO, and simplified mode remains a rendering-only setting.

### 7. Wrong vs Correct

#### Wrong

```js
child.stderr.on('data', chunk => job.result_json += chunk);
```

#### Correct

```js
reporter.output('stderr', chunk); // lease-guarded, redacted, bounded, throttled
```
