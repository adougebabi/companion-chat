# Go BFF contract closure handoff

## Delivered

- Created branch `codex/go-bff-acceptance` directly from `codex/go-build`.
- Added Browser OpenAPI method/path parity coverage against the committed
  `packages/browser-client/openapi.json` artifact.
- Added a public HTTP integration path through Go BFF for anonymous session,
  setup, authenticated session, Fluctlight creation, conversation creation,
  and streamed conversation turn.
- Added real HTTP downstream-disconnect coverage proving upstream Core body
  closure and request cancellation.
- Added a full protected-route missing-session/CSRF matrix that asserts Core is
  not called before the browser guard passes.
- Bounded and sanitized Core error details at the browser boundary, while
  preserving safe field-level validation details and stable public messages.
- Hardened media proxy handling for missing/empty upstream bodies and covered
  Range, ETag, MIME, content-range, and header allow-list behavior.
- Updated the active BFF code-spec with the executable error/media/parity rules.

## Verification

Passed:

```text
gofmt (Go BFF/config/platform/cmd)
go test ./...
go test -race ./...
go vet ./...
go build ./...
pnpm generate (no generated artifact drift)
pnpm typecheck
pnpm test (5 browser-client, 23 Node BFF, 18 Web)
pnpm build
pytest -q apps/core/tests (208 passed, 1 skipped)
```

The skipped Core test is the existing external local-socket integration case
in `apps/core/tests/providers/test_adapters.py`; it is unrelated to this
transport-only stage.

Existing Core static-gate baseline remains unchanged and is not part of this
branch's scope:

- `ruff format --check` reports 12 pre-existing files requiring formatting.
- `ruff check` reports the existing 527-character prompt line in
  `providers/runtime.py`.
- `mypy` reports five existing errors in `cognition/test_background.py` and
  `providers/adapters.py`.

No Python Core, database/migration, Compose, Node BFF, or generated client
files were modified. A real Docker/Compose deployment smoke was not run on
this host because the local Docker daemon is unavailable; the CI workflow
remains responsible for image build execution.
