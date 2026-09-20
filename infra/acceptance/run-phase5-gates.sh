#!/usr/bin/env bash
set -uo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

run_dir="${1:-$repo_root/docs/verification/phase-5/20260919T200912+0800}"
require_clear=1
if [[ "${2:-}" == "--allow-blocked" ]]; then
  require_clear=0
fi
mkdir -p "$run_dir/commands" "$run_dir/evidence/tests"

status=0
run_and_record() {
  local name="$1"
  shift
  local output="$run_dir/commands/${name}.txt"
  echo "[G0] $*"
  "$@" > >(tee "$output") 2>&1
  local code=${PIPESTATUS[0]}
  printf '\nexit_code=%s\n' "$code" >> "$output"
  if [[ "$code" != 0 ]]; then status=1; fi
}

run_and_record g0-checker-tests node --test infra/acceptance/verify-phase5-evidence.test.mjs

events_file="$run_dir/commands/g0-required-go-test.jsonl"
executions_file="$run_dir/executions.json"
set +e
go -C apps/core-go test -mod=readonly -count=1 -json \
  -run 'TestProviderRuntimeSupport|TestRunProviderQueuedStream|TestRunADKLoopDoesNotReuseTextBeforeFinalEmptyAssistant|TestHandleTurnProductionADKToolLoopSettlesOneAssistant|TestBrowserRouteMatrixMatchesOpenAPIArtifact|TestEveryBrowserOpenAPIRouteReachesTheInProcessBoundary|TestBrowserHandlerKeepsAuthAndCSRFAtPublicBoundary' \
  ./internal/core ./internal/httpapi/browser > >(tee "$events_file") 2>&1
go_code=${PIPESTATUS[0]}
set -e
node "$script_dir/collect-go-test-events.mjs" "$events_file" "$go_code" \
  "$script_dir/phase5-required-tests.json" "$executions_file"
collector_code=$?
if [[ "$collector_code" != 0 ]]; then status=1; fi

checker_args=(node "$script_dir/verify-phase5-evidence.mjs" "$run_dir" --executions "$executions_file")
if [[ "$require_clear" == 1 ]]; then checker_args+=(--require-clear); fi
"${checker_args[@]}"
checker_code=$?
if [[ "$checker_code" != 0 ]]; then status=1; fi

if [[ "$status" == 0 ]]; then
  echo "G0 total gate: PASS"
else
  echo "G0 total gate: FAIL (see command output and executions.json)" >&2
fi
exit "$status"
