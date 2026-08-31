# Go BFF contract closure

## Goal

Close the evidence and transport-hardening gap around the Go browser BFF
without changing the browser wire contract or moving any domain ownership.
The stage must prove one real browser-shaped HTTP path through Go BFF to Core
and make the existing route, security, stream, and media behavior regression
resistant.

## Requirements

- Keep Python Core as the only domain writer and keep the Node BFF as a
  rollback/reference implementation.
- Preserve the checked Browser OpenAPI operation set, including every method
  and path plus `OPTIONS` behavior; no generic internal-path proxy may be
  introduced.
- Add HTTP-level tests for the auth → Fluctlight → conversation → turn path,
  including service-key and opaque human-session forwarding and browser
  camelCase/NDJSON behavior.
- Harden bounded Core error details so credentials, tokens, prompts, provider
  responses, and other internal fields never leak through diagnostics or
  creation errors while keeping safe validation fields available.
- Make media proxy failure-safe for missing upstream bodies and preserve
  streaming, Range, ETag, MIME, and allow-listed header behavior.
- Prove downstream disconnect cancellation closes the upstream Core body and
  suppresses later browser writes.
- Do not change PostgreSQL/Alembic, Redis, S3, Temporal, Python Core domain
  behavior, or the browser client wire schema.

## Acceptance Criteria

- [x] Browser OpenAPI method/path inventory matches the Go route inventory;
  unknown paths and wrong methods do not reach Core.
- [x] Public HTTP integration test completes setup, authenticated Fluctlight
  creation, conversation creation, and a streamed turn through Go BFF.
- [x] Auth/CORS/CSRF/session/service-identity matrix and stable Core-error
  mapping tests pass.
- [x] NDJSON split-frame, hidden-payload, sequence, terminal, incomplete EOF,
  and HTTP disconnect cancellation tests pass.
- [x] Media 200/206 and nil-body/upstream failure tests pass without buffering
  or leaking storage details.
- [x] `gofmt`, `go test -race ./...`, `go vet ./...`, and `go build ./...`
  pass; the changed workflow/docs are YAML and diff clean.
- [x] Stage changes are committed on `codex/go-bff-acceptance` with no Core,
  database, or legacy-runtime modifications.

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
