# Technical Design

## Scope and Boundaries

The active client remains the vanilla browser module in `src/companion-main.js`; the server remains the single-module Express application in `server.js`. The existing interview endpoints stay available for older clients, but the active create-persona wizard will use only the new natural-language analysis endpoint.

The new flow has two explicit phases:

```text
textarea description
  -> POST /api/companion/interviews/analyze
  -> server-side JSON-only persona extraction
  -> normalized answers + provenance
  -> ready interview session + preview
  -> POST /api/companion/interviews/:id/activate
  -> existing life-model generation and createPersona transaction
```

The original description is request-scoped only. It is sent to the provider and is never stored in SQLite, returned in the preview, or injected into later chat context.

## API Contract

### `POST /api/companion/interviews/analyze`

Request:

```json
{"description":"一段自然语言人格描述，最多 6000 个字符"}
```

Success (`201`): the same interview view shape used by the existing preview flow, with `status: "ready"`, `source: "natural-language"`, normalized `answers`, `preview`, and `inferredFields`. The response contains no raw description.

Failure:

- `400 {"error":"人格描述不能为空"}` for a missing, non-string, or blank description.
- `400 {"error":"人格描述不能超过 6000 个字符"}` for an oversized description.
- `502 {"error":"人格分析失败：..."}` for provider failure, timeout, empty output, invalid JSON, or an invalid model shape. No interview row or persona is written for these failures.

### Existing activation contract

`POST /api/companion/interviews/:interviewId/activate` remains the only persistence boundary for the new wizard. Its `overrides` object may edit `name`, `role`, `foundation`, `interests`, `visualBaseline`, and `supportingCast` before activation. Activation continues through `activateInterviewWithLifeModel()` and `createPersona()`.

## Extraction Contract

The extraction prompt is versioned (`persona-description-v1`) and requires one strict JSON object:

```json
{
  "answers": {"name":"...", "role":"...", "foundation":"..."},
  "inferredFields": ["name", "role", "foundation"]
}
```

`answers` is filtered through the existing `interviewQuestions` allowlist and each field is bounded by its existing `maxLength`. `inferredFields` is filtered to the same allowlist. The prompt instructs the model to keep only facts relevant to the existing persona blueprint, discard unrelated prose, and supply conservative defaults for missing `name`, `role`, and `foundation`. The server also applies deterministic defaults (`新朋友`, `陪伴者`, and a one-sentence foundation) if the model omits them, marking those fields as inferred.

`previewInterviewAnswers()` gains a provenance option. Parsed fields not listed as inferred are treated as user-provided; inferred top-level fields and their corresponding `characterCard` paths are marked `inferred`. Preview overrides are treated as user-provided so edits replace generated defaults in the persisted blueprint.

## Session Metadata and Compatibility

Migration 10 adds two additive columns to `companion_interview_sessions`:

- `source TEXT NOT NULL DEFAULT 'interview'`
- `inferred_fields_json TEXT NOT NULL DEFAULT '[]'`

Existing rows and callers keep the old defaults. Natural-language sessions set `source` to `natural-language` and store only the bounded field-name list, never the source paragraph. `interviewView()` reads this metadata to rebuild provenance after a refresh. Both activation functions consume the same metadata so the final life blueprint retains its source distinctions.

## Error and State Semantics

- Validate body shape and description bounds before provider work.
- Keep provider credentials and network calls server-side through `lmCompletion()`.
- Use a bounded timeout and `modelJson()` for Markdown-fenced JSON; unknown keys and invalid shapes are rejected before session creation.
- Provider/model errors are converted to bounded `502` messages by the analysis helper and the existing route wrapper returns `{error}`.
- A failed analysis leaves no new session. A failed activation changes an `activating` session back to `ready`; no persona is partially committed.

## Frontend Behavior

`openPersonaWizard()` immediately renders one multi-line description form. Submit calls the new analysis endpoint. The same dialog then renders a preview with editable `name`, `role`, `foundation`, `interests`, `visualBaseline`, and `supportingCast`, plus a concise list of inferred fields. “返回修改” returns to the original description in transient browser memory; it does not expose the legacy question-by-question form. Analysis errors remain in the dialog and keep the entered text for retry.

All interpolated preview values pass through `esc()`. Styling uses existing wizard classes plus a small `.wizard-error` rule in `companion-style.css`; no framework or new client state store is introduced.

## Verification and Rollback

- Unit-test extraction normalization, defaults, provenance, JSON/unknown-key failures, and no-row-on-failure behavior using `companionTestHooks` and a mocked `fetch`.
- Keep existing adaptive-interview tests unchanged and verify the migration on a clean temporary database.
- Run `npm test`, `node --check server.js`, `node --check src/companion-main.js`, then exercise the new route and dialog through the local server with a temporary `DATA_DIR`.
- Rollback is additive: revert the frontend to the existing interview wizard and leave migration 10 columns unused; no existing persona or interview data needs transformation.
