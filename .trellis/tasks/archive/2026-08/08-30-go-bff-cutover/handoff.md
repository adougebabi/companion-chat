# Complete Go BFF cutover handoff

## Scope completed

- Branch `codex/go-bff-cutover` was cut directly from
  `codex/go-bff-acceptance`; unfinished `master` changes were not merged.
- Deleted the complete `apps/bff` Node/Fastify tree, Dockerfile, tests,
  package importer, and runtime build path.
- Moved Browser OpenAPI generation to
  `packages/browser-client/scripts/generate-openapi.mjs` and updated the root
  generation command.
- Removed the old BFF importer and dependencies from `pnpm-workspace.yaml`,
  `package.json`, and `pnpm-lock.yaml`.
- Kept Compose service/image names stable (`bff` and `fluctlight-bff`) while
  making their process and Dockerfile Go-only.
- Updated active README/spec/directory/quality documentation and changed
  auth/domain/media smoke scripts to send browser operations through the public
  Go BFF instead of Core `/internal/*` routes.

## Code/deployment verification

Passed:

```text
pnpm install --frozen-lockfile (4 workspace projects; no apps/bff importer)
pnpm generate (all artifacts unchanged)
pnpm typecheck
pnpm test (5 browser-client + 18 Web tests)
pnpm build
bash -n acceptance/compose smoke scripts
gofmt -l apps/gateway-go (no output)
go test -race ./... (Go BFF/config/platform)
go vet ./... (Go BFF/config/platform)
go build ./... (Go BFF/config/platform)
docker build apps/gateway-go/Dockerfile (success)
docker build apps/web/Dockerfile (success)
docker compose config (success; no legacy BFF refs)
active legacy-reference scan (no apps/bff/@fluctlight/bff refs)
```

The Go BFF image was rebuilt from this branch and replaced in the existing
Compose project without restarting Core or deleting volumes. Post-rebuild
`/health/live`, `/health/ready`, `/auth/session`, detail, moments, actions,
conversation, and media reads all succeeded.

Python Core pytest also passed in the existing environment (`208 passed, 1
skipped`). Existing Core ruff-format/ruff-lint/mypy baseline failures were not
modified because this task is the BFF cutover and the user explicitly excluded
unfinished `master` fixes.

## Real Docker regression

Detailed evidence, IDs, statuses, and the one recoverable first-attempt Core
failure are recorded in [`real-regression.md`](./real-regression.md).

Summary:

- Case 1 (影者 text chat): PASS, real 200 stream, 121 visible events, one
  terminal.
- Case 2 (影者 photo): PASS after contract-prescribed idempotent retry; three
  real PNG assets settled and downloaded through Go BFF.
- Case 3 (blank Fluctlight): PASS.
- Case 4 (description foundation): PASS with complete foundation/agency data.
- Case 5 (detail/schedule): PASS after submitting the required contiguous
  00:00–24:00 schedule through the BFF command.
- Case 6 (Moment): PASS with a completed real autonomy `moment` action and
  visible feed item plus media.
- Case 7 (proactive contact): PASS for the existing 影者 Fluctlight; two
  completed real `proactive_message` actions and matching assistant messages
  were observed through Go BFF. The newly created description test object
  chose `moment`/`no_op` within its ten-minute window, which is recorded as a
  Core LLM decision/backlog observation rather than hidden as a BFF success.

## Remaining promotion note

This branch intentionally removes the old BFF runtime. Before merging to the
Go integration branch, CI should run the updated four-project install,
generated-client checks, Go tests, image builds, and public-boundary smoke. No
remote push was performed by this task.
