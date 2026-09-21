#!/usr/bin/env bash
set -uo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

run_dir="${1:-$repo_root/docs/verification/phase-8/$(date -u +%Y%m%dT%H%M%SZ)}"
mkdir -p "$run_dir/commands" "$run_dir/evidence/tests"

status=0
run_and_record() {
  local name="$1"
  shift
  local output="$run_dir/commands/${name}.txt"
  echo "[P8] $*"
  "$@" > >(tee "$output") 2>&1
  local code=${PIPESTATUS[0]}
  printf '\nexit_code=%s\n' "$code" >> "$output"
  if [[ "$code" != 0 ]]; then status=1; fi
}

run_and_record p8-checker-tests node --test infra/acceptance/verify-phase5-evidence.test.mjs

events_file="$run_dir/commands/p8-required-go-test.jsonl"
executions_file="$run_dir/executions.json"
set +e
go -C apps/core-go test -mod=readonly -count=1 -json \
  -run 'Test(Phase8|EinoNative|EinoADK|ADKToolSuccess|RunADKLoop|ProviderConversation|ProviderWakeUp|ProviderTakeover|ADKLoopSchemaMatrix|RunADKStructuredTask|HandleTurnProductionADKToolLoop)' \
  ./internal/core ./internal/ai/agent > >(tee "$events_file") 2>&1
go_code=${PIPESTATUS[0]}
set -e
node "$script_dir/collect-go-test-events.mjs" "$events_file" "$go_code" \
  "$script_dir/phase8-required-tests.json" "$executions_file"
collector_code=$?
if [[ "$collector_code" != 0 ]]; then status=1; fi

if [[ -f "$run_dir/acceptance-matrix.csv" ]]; then
  node "$script_dir/verify-phase5-evidence.mjs" "$run_dir" --executions "$executions_file"
  checker_code=$?
  if [[ "$checker_code" != 0 ]]; then status=1; fi
else
  echo "P8 evidence gate: acceptance-matrix.csv is required in $run_dir" >&2
  status=1
fi

if [[ "$status" == 0 ]]; then
  echo "P8 contract gate: PASS"
else
  echo "P8 contract gate: FAIL (see commands/ and executions.json)" >&2
fi
exit "$status"
