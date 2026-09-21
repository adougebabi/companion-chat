#!/usr/bin/env bash
set -uo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

run_dir="${1:-$repo_root/docs/verification/phase-8/$(date -u +%Y%m%dT%H%M%SZ)}"
mkdir -p "$run_dir/commands" "$run_dir/evidence/tests"

if [[ ! -f "$run_dir/acceptance-matrix.csv" ]]; then
  cp "$script_dir/phase8-acceptance-matrix.csv" "$run_dir/acceptance-matrix.csv"
fi

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
  -run 'Test(Phase8|EinoNative|EinoADK|ProviderStructured|ADKToolSuccess|RunADKLoop|ProviderConversation|ProviderWakeUp|ProviderTakeover|ADKLoopSchemaMatrix|RunADKStructuredTask|HandleTurnProductionADKToolLoop)' \
  ./internal/core ./internal/ai/agent > >(tee "$events_file") 2>&1
go_code=${PIPESTATUS[0]}
set -e
node "$script_dir/collect-go-test-events.mjs" "$events_file" "$go_code" \
  "$script_dir/phase8-required-tests.json" "$executions_file"
collector_code=$?
if [[ "$collector_code" != 0 ]]; then status=1; fi

if [[ -n "${GO_CORE_TEST_DATABASE_URL:-}" ]]; then
  # CI provisions the isolated database; conditional DB rows are promoted only
  # after the required test event actually reports pass.
  if ! node -e 'const fs=require("fs"); const p=process.argv[1]; const e=JSON.parse(fs.readFileSync(p)); const bad=(e.conditional||[]).filter(x=>(e.actual||[]).find(y=>y.package===x.package&&y.test===x.test)?.action!=="pass"); if(bad.length){console.error("conditional tests did not pass",bad); process.exit(1)}' "$executions_file"; then
    status=1
  else
    sed -i.bak 's/^P8-04-DB,\([^,]*\),\([^,]*\),\([^,]*\),\([^,]*\),\([^,]*\),\([^,]*\),BLOCKED,/P8-04-DB,\1,\2,\3,\4,\5,\6,PASS,/' "$run_dir/acceptance-matrix.csv"
    sed -i.bak 's/^P8-05-DB,\([^,]*\),\([^,]*\),\([^,]*\),\([^,]*\),\([^,]*\),\([^,]*\),BLOCKED,/P8-05-DB,\1,\2,\3,\4,\5,\6,PASS,/' "$run_dir/acceptance-matrix.csv"
    rm -f "$run_dir/acceptance-matrix.csv.bak"
  fi
fi

node "$script_dir/verify-phase5-evidence.mjs" "$run_dir" --executions "$executions_file" $([[ "${CI:-}" == "true" && "${2:-}" != "--allow-blocked" ]] && echo --require-clear)
checker_code=$?
if [[ "$checker_code" != 0 ]]; then status=1; fi

if [[ "$status" == 0 ]]; then
  echo "P8 contract gate: PASS"
else
  echo "P8 contract gate: FAIL (see commands/ and executions.json)" >&2
fi
exit "$status"
