# Technical Design: Chat interaction, mobile usability, and debug observability

## Scope

This child changes browser interaction and additive debug contracts. It does not replace the existing SSE event names, normalize a new attachment-upload domain, or expose provider secrets.

## Composer and Pending-State Flow

```text
keydown Enter (without Shift)
  -> form.requestSubmit()
  -> existing isSending guard
  -> transient user + assistant typing placeholder
  -> POST /api/companion/chat SSE
  -> token/done replaces the same transient assistant item
  -> refresh restores server messages only
```

The typing item is a client-only record such as `{transient: 'typing'}`. `messageHtml()` renders natural typing copy only for that record; it is never persisted, sent to an API, counted unread, or retained after stream failure/refresh.

## Responsive Composer

Use `100dvh`, the existing mobile-navigation height, `env(safe-area-inset-bottom)`, and a single authoritative mobile layout. The composer uses a shrink-safe flex layout: textarea `min-width: 0`, fixed 44px send control, and bounded content padding. Verify with reduced viewport height to represent an on-screen keyboard.

## Comment Disclosure

Track one transient `commentingActivityId`. `activityHtml()` renders the input only for that ID. Clicking comment sets the ID and re-renders/focuses that field; submitting, escaping, switching activity, hiding the post, or a refresh clears it. The persisted activity/comment API is unchanged.

## Debug Observability

Add a development-only API/read model that resolves exactly one persona and returns bounded redacted records:

```text
GET /api/companion/personas/:personaId/debug-context
  -> { layers, recentRequests, mediaJobs }
```

- `layers` names authority and summarizes rendered current values; it excludes API keys.
- `recentRequests` contains bounded prompt/request summaries, timestamps, job IDs, result state, and provider error text after credential redaction.
- `mediaJobs` contains intent/enriched-prompt summary/workflow summary only for the selected persona.

The response is deliberately a bounded read model rather than a dump of database rows:

```text
layers: { identity, conversation, life, provider } // label + max-2,000-char rendered summary each
recentRequests: max 10 { id, createdAt, status, promptSummary, responseSummary, error }
mediaJobs: max 10 { id, kind, status, createdAt, intentSummary, promptSummary, workflowSummary, error }
```

Every string is recursively redacted before truncation (key names such as `apiKey`, `authorization`, `token`, `secret`, and matching bearer/key values become `[redacted]`). Error and missing-persona behavior follow existing JSON routes. Records are filtered by the requested persona ID before their JSON payload is parsed or summarized.

The inspector fetches it only after the user chooses the development inspector. No bootstrap, feed, or ordinary detail endpoint includes it.

## Manual Media Controls

Consumer composer controls are removed. A development inspector action may create a test media intent/job via the existing server-owned endpoint, labelled as a test dispatch. Natural-language/media function interpretation in chat and event-driven generation remain server decisions; this child should not reintroduce browser-to-ComfyUI calls.

## Failure and Privacy

- Debug route validates persona ownership/existence, returns normal JSON errors, and redacts credentials recursively before responding.
- Debug routes and the browser entry point require `COMPANION_DEBUG_INSPECTOR=1`; otherwise Express returns `404`. Tests opt in explicitly before importing the server.
- Debug prompt data remains bounded by count and character length.
- On a send failure, clear/mark the transient typing entry rather than persisting it.
- All controls remain keyboard reachable with native dialogs and focus restoration.
