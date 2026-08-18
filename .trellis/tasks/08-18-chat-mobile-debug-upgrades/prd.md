# Chat interaction, mobile usability, and debug observability

## Goal

Make chat feel like a messaging product rather than an AI control panel, eliminate the mobile composer failure, move manual generation controls into a development-only surface, and add a safe local inspection path for prompt/job debugging.

## Requirements

- The composer must send on Enter and preserve Shift+Enter for a newline. It must prevent duplicate sends while the current request streams.
- At narrow phone widths, the send button and composer actions must stay inside the visual viewport, above the bottom navigation/safe area, and remain tappable with a 44px minimum coarse-pointer target.
- While the assistant stream has not produced persisted content, render a natural typing indicator such as “正在输入…” rather than “正在组织回复...”. The indicator must not be stored as a real assistant message or become unread after refresh.
- Remove consumer-facing “请求一段图片/视频” controls from beside the composer. Media is requested in natural language through chat, or autonomously initiated by a life event. Keep optional manual media dispatch only in the clearly labeled development inspector.
- Activity cards initially show no comment field. Clicking “评论” opens/focuses an inline comment composer for that single activity; cancelling or submitting collapses it without affecting other cards.
- Add a local development-only observability drawer/inspector that can inspect the selected persona’s composed prompt layers, recent chat request/response metadata, media-intent inputs, enriched media prompt, ComfyUI workflow request summary, and durable job status. Provider credentials, raw authorization headers, and another persona’s data are forbidden.
- Debug data is bounded, redacted, persona-scoped, and not requested/rendered in normal chat or Moments UI.
- The inspector and its APIs are enabled only with the explicit local `COMPANION_DEBUG_INSPECTOR=1` process flag. When disabled, the client has no entry point and server debug routes return a normal not-found response.

## Acceptance Criteria

- [ ] Pressing Enter sends one non-empty message; Shift+Enter adds a newline; Enter cannot queue another send while a stream is active.
- [ ] At 320px and 375px widths, including simulated keyboard/reduced viewport height, the composer text area and send button have no overlap with the mobile navigation and the send button can be tapped.
- [ ] A pending streamed reply reads “正在输入…” (or equivalent natural typing copy), then resolves into the original assistant message without a duplicate/ghost record after refresh.
- [ ] The consumer composer has no manual image/video generation buttons. The lifecycle/development inspector can still intentionally enqueue a test image/video job with a clear warning.
- [ ] Clicking an activity’s comment action shows only that activity’s labelled comment input; submitting persists a scoped comment and collapses the input; no blank comment input is rendered in untouched cards.
- [ ] The debug inspector shows only the selected persona’s bounded/redacted composition/job data and rejects or omits credentials.
- [ ] With `COMPANION_DEBUG_INSPECTOR` absent (including deployed environments), neither the inspector entry point nor either debug route is available; with the flag set to `1`, the inspector is available only after the user explicitly opens it.

## Out of Scope

- A general attachment upload system, external push notifications, public prompt sharing, provider configuration changes, and user-facing editing of automated activity/media records.

## Open Questions

None blocking. The planned default is a local development-mode inspector protected by an explicit entry point rather than a production-facing settings toggle.
