# Technical Design

## Boundary

The Go BFF remains a standard-library HTTP transport boundary. It receives
browser cookies and JSON, validates the public contract, maps requests to the
existing Python Core HTTP API, and translates only the visible response
contract. It does not access persistence, Redis, object storage, Temporal, or
domain services.

## Contract evidence

- Keep the existing explicit route handlers and route smoke matrix.
- Add an artifact-backed method/path parity test that reads the committed
  browser OpenAPI document and compares normalized `:param` route patterns.
- Keep route-specific mapping and error policy; do not introduce recursive
  snake/camel conversion or raw Core passthrough.

## Security and error hardening

Core error details are treated as untrusted internal data. Before details are
placed in a browser error response, recursively retain bounded JSON values,
drop sensitive keys (credentials, secrets, tokens, prompts, provider/raw
response fields), and cap depth, keys, collection size, and string length.
Safe validation fields such as `field` remain available.

## Stream and media tests

Use `httptest.Server` for the public BFF and a stateful fake Core so the test
exercises actual HTTP headers, cookies, request contexts, and body closure.
Use a blocking upstream body for downstream disconnect tests. The media
handler must check for a nil body before deferring `Close`, copy the body
directly, and keep the current response-header allow-list.

## Rollback and scope

This stage is behavior-preserving and independently rollbackable. Node BFF,
Python Core, schema/migrations, Compose services, and production cutover are
unchanged. The branch is based directly on `codex/go-build` and will be
merged back only after the local and CI gates pass.
