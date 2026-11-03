# Serial Tool / Agent E2E runner

Date: 2026-09-22 (Asia/Shanghai)

## Delivered entry point

`infra/acceptance/run-go-live-provider-smoke.sh` now has four identities:

- no arguments, or `--suite smoke`: the existing bounded Provider smoke;
- `--suite tools`: the fixed 15-product-Tool independent test matrix;
- `--suite agents`: one real-Provider `TestFormalAgentE2E` subtest per process;
- `--suite all`: Tool rows followed by Agent rows.

The default remains an ordinary Provider smoke. Its run metadata says
`identity=ordinary-provider-smoke`; it is never reported as dual E2E evidence.
The strict suites use `identity=dual-e2e`, but exit zero means only that every
row currently mapped by the runner passed. It does not fill a matrix row that
does not yet have a real test.

The supported commands are:

```sh
infra/acceptance/run-go-live-provider-smoke.sh --help
infra/acceptance/run-go-live-provider-smoke.sh --suite tools --tool memory_event
infra/acceptance/run-go-live-provider-smoke.sh --suite agents --agent conversation_cognition
infra/acceptance/run-go-live-provider-smoke.sh --suite tools
infra/acceptance/run-go-live-provider-smoke.sh --suite agents
infra/acceptance/run-go-live-provider-smoke.sh --suite all
```

Tool names are the formal registry names, including punctuation such as
`conversation.reply`, `media.image.generate`, `schedule.replan`, and
`capability.request`. Agent selectors are registered `FormalAgentID` values,
such as `takeover_judge`, `visual_identity_vision`, and `schedule_replan`.

## Environment and isolation

The runner never reads a default database URL. Tool, Agent, and all-suite runs
require an explicitly supplied `GO_CORE_TEST_DATABASE_URL`. That URL must point
to a disposable PostgreSQL server on which the existing Go fixtures may create
and drop randomly named databases. A selected DB-only Tool requires only that
database variable. The complete Tool suite additionally requires the live
Provider because `schedule.replan` uses it.

Provider rows require `FLUCTLIGHT_LIVE_PROVIDER_URL` and
`FLUCTLIGHT_LIVE_PROVIDER_MODEL`. `FLUCTLIGHT_LIVE_PROVIDER_API_KEY` remains
optional for unauthenticated local endpoints. The runner records only whether
these names are present; it does not record their values. It probes `/models`
with the response body discarded before starting any live row.

The existing Compose smoke remains available as an independent disposable
platform check:

```sh
FLUCTLIGHT_ENV_FILE=infra/compose/fluctlight.local.env \
  infra/compose/run-platform-smoke.sh --clean
```

That script already creates a unique Compose project, random web host port,
private volumes, and cleanup. Its PostgreSQL service is not exposed as a host
port by the current Compose file, so it is not presented as a one-command
database source for this Go E2E runner. The runner does not start a second test
platform and does not claim Compose integration that does not exist.

## Serialization, evidence, and failure semantics

Every Tool and every Agent is a separate `go test` process with
`-p 1 -parallel 1`. One host-wide atomic lock under `${TMPDIR:-/tmp}` covers the
Provider probe and the entire selected run, so two runner processes cannot
compete for the same GPU. `FLUCTLIGHT_LIVE_PROVIDER_LOCK_DIR` may select a
different host-local lock path when separate Providers are intentionally used.

Each invocation creates a new `serial-<suite>-<UTC>-<random>` directory under
the task's `research/runs/` directory. `--run-dir` accepts only a path that does
not already exist. Each command records:

- the sanitized command and exit code;
- the repository HEAD;
- SHA-256 for every tracked or untracked `*.go` source under `apps/core-go`;
- only the presence/missing state of the four configuration names;
- raw Go JSON events, strict verification result, and suite summary.

`verify-go-e2e-events.mjs` rejects a nonzero Go exit, malformed event log,
zero passing tests, any required test without a pass event, any child or parent
SKIP/FAIL event, and explicit `BLOCKED` output. This catches `go test` exit zero
with no matching test. The failure simulator in
`verify-go-e2e-events.test.mjs` covers zero-match, skip, fail, missing expected
test, blocked output, and hidden nonzero exits without making a model request.

The original 15 Tool expectation is literal in the runner and is not generated
from the current registry. The runner cross-checks it against the separate
literal `independentToolProductInventory` maintained by
`TestIndependentToolE2EFixedProductInventory`. The runner executes tests from `tool_execution_test.go`,
`tool_publication_test.go`, `persona_action_capabilities_test.go`,
`active_memory_tool_test.go`, and `independent_tools_e2e_test.go`. A supplied
native-looking correlation ID in those direct tests remains correlation
evidence only. It is not reclassified as proof that a real Eino adapter created
the call. The complete acceptance matrix must continue to track that adapter
evidence separately.

## Current fail-closed extension points

The visual-completion slice has now added three names to the fixed product Tool
inventory: `visual_identity.generate_candidate`,
`visual_identity.commit_review`, and `visual_identity.finalize`. Their direct
tests live in `visual_identity_tools_test.go`, outside the original five-file
Tool matrix assigned to this runner slice. A full `--suite tools` or
`--suite all` currently stops before the Provider probe and names all three
unmapped Tools. The main integration thread must add their selectors and
expected test names after reviewing that slice. A selected original Tool can
still run independently.

Full Tool acceptance also requires two test entry points that do not exist yet:

- `TestFormalToolEinoAdapterE2E`, with one passing subtest named after every
  Tool in the fixed product inventory;
- `TestIndependentToolE2ERejectsFalseSuccessWithoutWrite`, the isolated
  mutation proving that a reported success without the real write is detected.

The first gate prevents direct `ExecuteTool` tests with a supplied
`NativeToolCallID` from being misreported as Eino adapter evidence. Full
Tool/all modes fail before Provider access while either entry is missing. Once
implemented, the runner executes both and checks every fixed adapter subtest
before starting the individual Tool rows.

The runner maps the 16 existing `TestFormalAgentE2E` subtests listed in
`formal_agent_e2e_test.go`. The current formal registry also contains
`visual_identity`, whose complete-task real E2E is being added in a separate
slice. A full `--suite agents` or `--suite all` therefore currently stops before
the Provider probe with:

```text
registered FormalAgent has no runner mapping: visual_identity
```

This is intentional. Once that slice supplies the formal ID's real subtest,
the runner's explicit Agent list and selector must be updated together. A new
registered Agent cannot silently disappear from a full acceptance run.

Full Agent acceptance additionally requires
`TestFormalAgentE2ERejectsBrokenToolResultFeedback`, the isolated mutation that
disconnects Tool-result feedback and proves the target Agent E2E detects it.
The runner fail-closes before Provider access while that test is missing and
executes it before the serial live rows once supplied. Single `--agent` and
`--tool` selectors intentionally remain usable for focused diagnosis.

No live Provider request or real PostgreSQL E2E was executed while implementing
this runner. Passing evidence still requires the user-run serial commands with
an isolated database and available Provider/GPU. Existing earlier failures and
passes under `research/runs/` were not overwritten.

## Local verification performed

The runner implementation was checked without external calls:

```text
bash -n infra/acceptance/run-go-live-provider-smoke.sh
node --check infra/acceptance/verify-go-e2e-events.mjs
node --check infra/acceptance/run-go-live-provider-smoke.test.mjs
node --test infra/acceptance/verify-go-e2e-events.test.mjs infra/acceptance/run-go-live-provider-smoke.test.mjs
```

The final Node run passed 13 tests. These cover CLI help and selection,
cross-process lock rejection, fail-closed Tool/Agent inventory drift, the
full-matrix gate gaps, a fake local DB-only Tool run with source manifests, and
zero-match/SKIP/FAIL/BLOCKED/missing-test result rejection.

An attempted real Go fixed-inventory check did not reach the test because the
shared worktree was concurrently inside retired-path cleanup: `internal/core`
failed to compile on missing `capabilityInvocationBatchError`,
`ExecuteCapabilities`, `planCapabilitiesForTransaction`, and related removed
symbols still referenced by tests. This runner slice did not change those
files. The main integration thread must rerun the fixed inventory and package
checks after that concurrent cleanup settles.
