# Backend Quality Guidelines

## Required Patterns

- Apply [Tool Availability](./tool-availability-contract.md) to internal Tool
  calls. Recover ordinary domain preconditions with truthful handled receipts,
  refresh stale projections, and settle preclaimed attempts as suppressed. Keep
  real transaction/Provider/cancel/ownership failures distinct; explain any
  retained gate by Tool, condition, effect prerequisite and alternative receipt.

- Validate IDs, required text, cursor/query shape, and JSON body types at the
  Go browser boundary route boundary before calling Core.
- Keep browser camelCase↔Core snake_case mappings explicit per route. Do not
  recursively guess field names or proxy arbitrary `/internal/*` paths.
- Forward the Core service identity and opaque human session independently;
  never authorize from a browser-supplied Actor ID.
- Stream NDJSON incrementally across arbitrary byte boundaries, enforce one
  monotonic sequence and one terminal event, and suppress writes after
  cancellation.
- Proxy media bodies directly with bounded allow-listed headers; never expose
  bucket/object keys, credentials, or provider details.
- Sanitize Core error details recursively with bounded depth/collections/text
  before returning browser errors.
- Keep Go Core as the only domain writer; domain repositories and workflow
  clients do not cross into the Go browser boundary.
- Treat internal reference/frozen-context optimization thresholds as advisories:
  preserve complete valid data through both capability runtimes and emit safe
  pre-model metadata. Verify every downstream copy/codec/settlement path; removing
  a front-door check alone can leave silent nil/truncation or another count gate.
  Keep authorization, integrity and external argument/schema validation separate.
  See the internal-capacity scenarios in the structured-turn and diagnostics contracts.
- For Goal Agent recovery, test actual Agent admission and native-loop progress:
  a timer/prompt-only change is insufficient. Deterministic representation errors
  use truthful handled receipts; accepted targets replay without writes. A stalled
  cycle must close even if the Provider ignores an empty Tool catalog, while actual
  Provider/DB/cancel errors remain errors and unresolved evidence stays pending.
  Test cadence/episode persistence with a disposable PostgreSQL; compile/SKIP
  evidence cannot certify SQL, upgrade or concurrent atomicity.

## Forbidden Patterns

- Restoring `apps/browser`, a Node browser boundary runtime, or a second public gateway.
- Adding a second database, external queue, or storage client to the Go browser boundary.
- Making model, ComfyUI, PostgreSQL, Redis, MinIO, or Temporal calls from
  browser code or browser boundary route handlers.
- Completing a job/event with an expired or different lease owner.
- Changing browser event names or resource JSON fields without updating the
  Browser OpenAPI artifact, generated client, and Go route tests together.
- Logging bearer tokens, passwords, setup tokens, provider secrets, or
  committing private deployment data.

## Testing And Review

Run the owning Go checks and full browser workspace checks before committing:

```bash
gofmt -l apps/core-go/internal/httpapi/browser
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go/internal/httpapi/browser test -race ./...
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go/internal/httpapi/browser vet ./...
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go/internal/httpapi/browser build ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
```

Run Go Core tests and the disposable Compose/public-boundary smoke when the
environment is available. Do not test against a checked-in or shared
production database unless the task explicitly authorizes a real regression
run and the scope of created records is recorded.

Review the full browser→browser boundary→Core path for every API change. Verify normal and
failure responses, stable error codes, cookie/CSRF behavior, stream terminal
errors, media Range behavior, and cancellation. Keep the Browser OpenAPI
artifact and generated clients drift-free.

## Scenario: Isolated fact-only image publication

### 1. Scope / Trigger
Push or dispatch of `codex/fact-only-reply-vision` or
`codex/fluctlight-functional-recovery` publishes the two existing Docker Hub app
images; PRs only run the formatting job. Per the user's 2026-10-20 instruction,
the sole CI validation job is Go formatting; unit/integration tests, type checks,
client regeneration and historical acceptance gates are not CI prerequisites.
Workflow branch filters must include each supported branch. Local and user-run
business acceptance remain separate from image packaging success.

### 2. Signatures
`python3 infra/release/fact_only_release.py record|promote --output <json>`; promote also requires `--directory` with core.json/web.json. `switch_images.py --env-file <private> --manifest <published-json>` previews, and `--apply` performs an explicit switch.

### 3. Contracts
Immutable tags include full source SHA/run/attempt. All image records must share that identity. Promotion checks remote branch HEAD before each alias, copies by digest and verifies the resulting registry digest. The exact source-ref allowlist maps to `fact-only` or `functional-recovery`; immutable tags, artifacts, manifests and mutable `<channel>-latest` aliases use that same channel. Cross-channel sets, ref/channel mismatches and unlisted refs fail before publication. Promotion concurrency is scoped to the source branch. Two repositories are not atomic; only status=published is a complete release. Compose switches the actual CORE_GO_IMAGE and WEB_IMAGE variables; worker build ID identifies the artifact and its channel.

### 4. Validation & Error Matrix
Missing/mixed records or stale HEAD reject promotion. A partial failure retains promotion_failed plus already promoted component evidence. A changed alias digest fails. No PR release credentials or pull_request_target execution. On rollback, explicitly choose stable refs and restore compatible settings/drain new tasks; skip the old migration command.

### 5. Good/Base/Bad Cases
Good: both immutable images → verified complete manifest → aliases. Base: preview selects refs without deployment. Bad: each matrix member overwrites latest, or rebuild master while deployment still uses fact-only-latest.

### 6. Tests Required
Local release tests assert missing/mixed sets, stale head before/after one component, exact digest verification, both channels' isolated tags and deployable digest selections, cross-channel rejection and absence of stable alias writes. User-owned real-environment regression remains not run by the agent; YAML parsing and mocked registry tests do not prove CI publication/deployment.

### 7. Wrong vs Correct
Wrong: mutable tags are the release identity. Correct: deploy Core/Web digests from the same complete source/build manifest and check the running containers.
