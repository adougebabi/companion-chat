# Development Debug Observability

## Scenario: Persona-scoped local inspector

### 1. Scope / Trigger

- Trigger: a browser inspector needs prompt/job diagnostics that normally contain provider-adjacent data.
- The boundary is cross-layer: SQLite records flow through Express to a vanilla DOM renderer. Hiding a button alone is not a security control.

### 2. Signatures

- Environment gate: `COMPANION_DEBUG_INSPECTOR=1`
- `GET /api/companion/personas/:personaId/debug-context`
  - Success: `{state, layers, recentRequests, mediaJobs}`. `state` is the small
    resolver read model used by the inspector: `{situation, scene, outfit,
    special, mood}`. It must be derived from the persona-scoped life-state
    resolver, not from a stale state projection alone.
- `GET /api/companion/personas/:personaId/lifecycle`
  - Success includes normalized `jobs[]` records with `jobType`, `status`,
    attempt/time fields, optional message/activity links, and bounded
    `payloadSummary`/`resultSummary`; raw SQLite `payload_json` and
    `result_json` are never part of the browser contract.
- `POST /api/companion/personas/:personaId/debug-media`
  - Request: `{kind: "image" | "video", request?: string, prompt?: string}`.
  - Success: existing `createChatMediaRequest()` response containing the queued placeholder message.

### 3. Contracts

- Do not register either debug route unless `COMPANION_DEBUG_INSPECTOR === '1'`. Bootstrap exposes `debugInspector: true` only with the same condition, so ordinary UI never requests or renders diagnostics.
- Resolve the requested persona first and use `persona_id = ?` in every debug query. Do not parse/summarize another persona's rows and filter afterward.
- Return at most 10 recent request records and 10 media jobs. Limit every rendered summary to 2,000 characters.
- The browser debug workspace treats prompt-run records as the authoritative
  request/response pair. `layers` and `recentRequests` remain compatibility
  fields in the bounded context DTO, but are not rendered as a second prompt
  summary. Flow runs are not persisted as a separate read model; the UI must
  label the durable job list accordingly instead of presenting jobs as flows.
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

- Good: an explicitly enabled local inspector fetches the selected persona's last ten jobs and shows redacted envelope → persona concept → prompt-template → final-prompt/workflow summaries.
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

## Scenario: h3 configuration summary and no-generation preflight

### 1. Scope / Trigger

- Trigger: an operator needs to confirm the runtime h3 configuration and test whether the local executable can start, without submitting a prompt or creating media work.
- This is a cross-layer settings → bootstrap → settings UI → explicitly gated debug API contract. Full h3 paths remain server-only.

### 2. Signatures

- Bootstrap-safe derived field: `publicSettings().h3ConfigSummary`.
- Debug-only route: `POST /api/companion/h3-preflight`.
- Server helper: `h3Preflight(config = settings()) -> {ok, stage, checks, process?}`.

### 3. Contracts

- `h3ConfigSummary` returns only `{executable, modelDir, outputDir}` checks, each with `configured`, `valid`, an optional safe `displayName` such as `…/h3`, and a safe error. It never returns full paths, command arguments, or model profile values.
- Saving an h3 field or selecting `videoProvider: 'h3'` validates an absolute executable regular file with execute permission, a real model directory, and an output directory inside the allowed root that can be created and written. Invalid settings fail before the SQLite settings row changes.
- Register the preflight route only when `COMPANION_DEBUG_INSPECTOR === '1'`; an unset or any other value is disabled.
- Preflight uses current server settings, does file-system validation, then runs only `["--help"]` using the configured executable with `shell: false` and a short timeout. It does not read a prompt, load a model, enqueue a job, or create a media asset.
- Redact and truncate process output before returning it. Keep at most four 480-character output records in a rolling buffer, including on process failure.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Executable is absent, relative, non-file, or non-executable | Save/preflight returns a safe executable validation error; settings write is rejected. |
| Model directory is absent or not a directory | Save/preflight returns a model-directory error; settings write is rejected. |
| Output directory is outside the allowed root, cannot be created, or is not writable | Save/preflight returns an output-directory error; settings write is rejected. |
| Filesystem checks pass but `h3 --help` cannot start or exits unsuccessfully | Preflight returns `stage: "process"` with a safe environment-compatibility error and bounded output. |
| Debug flag is absent or not `1` | The preflight route is absent (`404`); bootstrap does not enable the inspector. |

### 5. Good / Base / Bad Cases

- Good: settings show `…/h3`, `…/MiniMax-H3`, and `…/outputs` as valid without exposing their absolute paths; a local inspector test confirms `h3 --help` and leaves job/asset counts unchanged.
- Base: an unconfigured h3 provider reports each missing field safely and existing ComfyUI flows remain unchanged.
- Bad: returning `settings()` directly, allowing `spawn('h3.c')`/relative executables, registering a debug route by default, or buffering unlimited subprocess output.

### 6. Tests Required

- Assert the public summary has safe display names and contains no configured absolute path.
- Assert relative/non-executable paths, missing model directories, invalid output directories, and allowed-root escapes fail without changing `companion_settings`.
- Assert the debug route is absent for both `COMPANION_DEBUG_INSPECTOR=0` and an unset flag, and present only for `=1`.
- Use a harmless executable fixture (or `process.execPath --help`) to assert preflight succeeds without adding durable jobs/assets; assert output is redacted and at most four records.

### 7. Wrong vs Correct

#### Wrong

```js
const debugInspectorEnabled = process.env.COMPANION_DEBUG_INSPECTOR !== '0';
const output = [];
runH3(executable, ['--help'], 8_000, {onOutput: item => output.push(item)});
```

#### Correct

```js
const debugInspectorEnabled = process.env.COMPANION_DEBUG_INSPECTOR === '1';
const output = [];
const record = item => {
  output.push(redactAndBound(item));
  if (output.length > 4) output.shift();
};
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
- Media asset URLs are served through the provider-owned `readAsset({asset, settings, res})` boundary. The HTTP response object must remain inside that input so provider adapters can stream the stored asset to the browser.

### 3. Contracts

- The only prompt presented as the final provider prompt is the compiled prompt persisted immediately before provider submission; raw `mediaIntent` remains secondary diagnostic information.
- h3 receives `progress` through the server-owned provider boundary, reads both stdout and stderr, and stores only the latest normalized line. Explicit `%` values may set `percent`; time elapsed must never synthesize a percentage.
- Progress writes use the same `status = 'leased'`, `lease_owner`, and non-expired lease guard as settlement. They never renew a lease or alter status/attempt/retry scheduling.
- Output is ANSI/control-character cleaned, recursively credential-redacted, path-redacted, and capped at 480 characters. Do not persist whole command lines, argument arrays, model/output paths, or stream history.
- Completion and failure merge existing `result_json` so final prompts and h3 progress survive terminal settlement. A retry starts a new `attempt` snapshot.
- Media diagnostics additionally preserve the bounded capability call/frozen persona concept, prompt-master template, and C-stage acceptance history. C `retry` is a separate one-time quality retry and must not be confused with provider transport/poll attempts; C infrastructure `skipped` is a successful delivery with a safe diagnostic.
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

## Scenario: Unified LLM prompt runs

### 1. Scope / Trigger

- Trigger: any server-owned `lmCompletion()` call needs one local place to inspect the exact bounded request sent to the model, regardless of persona, chat path, or background job.

### 2. Signatures

- Migrations 12-13 own `companion_prompt_runs` with `persona_id`, optional `job_id`/`message_id`, `operation`, `status`, `model`, bounded `request_json`/`response_json`, error, and timestamps.
- `GET /api/companion/prompt-runs?limit=50&personaId=...` returns the newest prompt runs across personas (or one persona when filtered).
- `lmCompletion(payload)` accepts a transport-only `trace` object; the trace is removed before the provider request.

### 3. Contracts

- Every LLM request is recorded at the shared `lmCompletion()` boundary; streaming calls record the full request before token consumption, and continuation calls are separate rows. A clone of the provider response is drained independently so ordinary callers can still consume it; JSON responses are stored as objects and streaming responses as bounded SSE text.
- The prompt run table is bounded to the newest 5,000 rows. Request text is recursively redacted, path-scrubbed, string-bounded, and binary data URLs are omitted before persistence.
- The global prompt-run route is registered only when `COMPANION_DEBUG_INSPECTOR=1`; it returns redacted request JSON and remains persona-filterable with `persona_id = ?`.
- Prompt runs are local diagnostics, not user-visible conversation messages. Media provider prompts continue to be read from their durable media jobs.

### 4. Validation / Failure Behavior

| Condition | Result |
| --- | --- |
| Provider accepts and the response body is captured | Run status is `completed`; resolved model/request/response are stored. |
| Provider accepts but response cloning/reading is unavailable | Run remains `submitted`; request and model are still stored. |
| Model resolution or provider HTTP call fails | Run status is `failed` with a bounded error; the original provider error behavior is unchanged. |
| Debug flag is absent or not `1` | The prompt-run route is absent; the SQLite rows remain available for local inspection. |
| Persona is deleted | Prompt rows cascade with the persona; job/message references are nullable. |

### 5. Tests Required

- Assert migration 12, one row per direct/continuation LLM call, bounded retention, redaction, and binary omission.
- Assert the route is absent without the explicit debug flag and present with `=1`.
- Assert deleting a persona removes its prompt rows without affecting another persona.
