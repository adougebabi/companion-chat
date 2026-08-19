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

## Media Concept Boundary: Never Recreate Visual Semantics in Server Code

For image/video work, distinguish **factual server context** from **model-owned visual semantics** before adding a helper or a prompt field:

```text
server facts / durable job envelope
  -> AI-persona media concept
  -> image-prompt-master fixed template
  -> provider prompt
```

- The server may attach immutable identity, event-first current state, temporary appearance, source request/event, kind/count, authorization, provider selection, persistence, retries, and redacted diagnostics.
- The AI persona owns scene interpretation, human subjects, non-human objects, capture relationship, action, wardrobe treatment, pose, expression, lighting, composition, and exclusions; the prompt master turns that concept into the fixed eight-section provider template.
- A renderer may join fixed template slots only. It must not translate capture modes, calculate visible humans, emit `共 X 人`, append negative constraints, or repair missing visual content.
- Concision is a prompt-master instruction, not a renderer policy: never slice, summarise, or length-reject a structurally valid provider template in server code. The master must decide what redundancy to remove while retaining the necessary visual facts.
- Never add wording regex/default branches such as `/自拍|POV|宠物|两人/` in media parsers, job creators, schema normalizers, or provider-bound helpers. Such branches silently turn clothes, props, animals, screens, or reflections into people and cause broken camera perspective.
- The AI persona must freeze `MediaCapabilityCallV2` (concept + current event + temporary appearance) before the durable media job is created. A malformed call is rejected before enqueue; an old job without a frozen concept terminally fails without concept/master/provider calls. Prompt-master failures remain retryable, but no server-derived fallback prompt is allowed.
- After provider success, C-stage visual acceptance returns `pass`, `retry`, `reject`, or infrastructure-only `skipped`. C may trigger at most one regeneration while keeping A and the authoritative facts immutable; `reject` or exhausted retry fails only the media target, while `skipped` publishes the successful candidate with a bounded diagnostic.

Before modifying a media flow, trace all four producers (chat, direct activity, model-driven activity, debug inspector) and verify they persist the same factual envelope. Test one fixture each for selfie, external capture/photographer POV, and non-human props alongside a malformed-model response that reaches no provider call.
