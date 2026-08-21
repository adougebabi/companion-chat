# Code Reuse Thinking Guide

This repository is small enough that duplication is easy to miss but costly: the server and browser each encode parts of the same state and event contracts.

## Search Before Adding

Search for existing helpers and fields before adding another one: `cleanUrl`,
`now`, `id`, typed API clients, contract normalizers, Pinia stores and
composables are the established owners for common behavior. If a payload field
crosses the server route and `web/src/api/contracts.ts`, update both
deliberately rather than adding a third shape.

## Reuse Local Owners

- Use `api()` for browser JSON requests so error extraction stays consistent.
- Use `appendDebug()` for console traces and `saveState()` for persistence.
- Use `setWorkflowPrompt()` for all ComfyUI prompt injection; do not implement a second token-marker scanner.
- Use the existing message renderers and CSS classes for new message/job variants.

## Avoid Premature Abstractions

Do not create a generic framework, repository class, or component system for one caller. Extract only when behavior has multiple callers or a contract needs one authoritative decoder/normalizer.

## Checklist

- [ ] Searched for the existing helper, endpoint, event name, and state field.
- [ ] Kept one owner for derived state and provider URL/error behavior.
- [ ] Updated every producer and consumer when a shared contract changed.
- [ ] Avoided copying an async state merge without the existing re-read pattern.
