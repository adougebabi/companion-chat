#!/usr/bin/env bash
set -euo pipefail
: "${GO_CORE_TEST_DATABASE_URL:?Use a task-owned PostgreSQL test endpoint; never production}"
repo_root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$repo_root"
output_dir=${1:-/private/tmp/fluctlight-goal-planner-acceptance}
mkdir -p "$output_dir"
export GOCACHE=${GOCACHE:-/private/tmp/fluctlight-go-cache}
go -C apps/core-go test ./internal/core ./internal/httpapi/... ./internal/migrations ./internal/workflow ./internal/ai/agent -count=1 -json > "$output_dir/go-tests.jsonl" 2>&1
pnpm generate > "$output_dir/generate.log" 2>&1
pnpm typecheck > "$output_dir/typecheck.log" 2>&1
pnpm test > "$output_dir/browser-tests.log" 2>&1
pnpm build > "$output_dir/build.log" 2>&1
bash infra/acceptance/check-core-openapi.sh > "$output_dir/core-openapi.log" 2>&1
if [[ -n ${GO_GOAL_TEST_TEMPORAL_ADDR:-} && -n ${GO_GOAL_TEST_REDIS_ADDR:-} ]]; then
  go -C apps/core-go test ./test/integration -run '^TestGoalJointPostgresRedisTemporalWorkerRestart$' -count=1 -json > "$output_dir/worker-tests.jsonl" 2>&1
else
  printf '%s\n' 'NOT_RUN: task-owned Redis/Temporal endpoints are missing; Worker integration is not accepted.' > "$output_dir/worker-tests.log"
fi
if [[ ${FLUCTLIGHT_LIVE_PROVIDER_TEST:-0} == 1 && -n ${FLUCTLIGHT_LIVE_PROVIDER_URL:-} && -n ${FLUCTLIGHT_LIVE_PROVIDER_MODEL:-} ]]; then
  go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/goal_planner$' -count=1 -json > "$output_dir/real-provider-tests.jsonl" 2>&1
else
  printf '%s\n' 'NOT_VERIFIED: real Provider is unconfigured; scripted Provider is not real-model acceptance.' > "$output_dir/real-provider-tests.log"
fi
printf 'Deterministic checks finished. Evidence: %s. Inspect the acceptance matrix and NOT_RUN/NOT_VERIFIED records before claiming business acceptance.\n' "$output_dir"
