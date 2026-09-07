#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

project="fluctlight-go-active-$$"
compose_file="infra/compose/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-infra/compose/fluctlight.env.example}"
compose=(docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file")
source "$script_dir/read-compose-env.sh"
assert_disposable_compose_project "$project"
temporal_namespace="$(compose_env_value TEMPORAL_NAMESPACE)"
temporal_namespace="${temporal_namespace:-default}"
worker_build_id="$(compose_env_value TEMPORAL_WORKER_BUILD_ID)"
worker_build_id="${worker_build_id:-platform-v1}"
export EXPECTED_WORKER_BUILD_ID="$worker_build_id"

compose_started=0
workflow_id="go-active-workflow-${project}"

cleanup() {
  if [[ "$compose_started" != 1 ]]; then
    return
  fi
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

"$script_dir/check-compose-bind-sources.sh" "$compose_file"
compose_started=1
"${compose[@]}" config >/dev/null
"${compose[@]}" up --build --detach --wait --wait-timeout 180

ready=0
for _ in $(seq 1 60); do
  if "${compose[@]}" exec -T core wget -q -O /dev/null http://127.0.0.1:8080/health/ready; then
    ready=1
    break
  fi
  sleep 2
done
if [[ "$ready" != 1 ]]; then
  "${compose[@]}" ps >&2 || true
  "${compose[@]}" logs --no-color core worker temporal >&2 || true
  echo "Go Core did not reach /health/ready" >&2
  exit 1
fi

"${compose[@]}" exec -T temporal temporal --address temporal:7233 --namespace "$temporal_namespace" workflow start \
  --workflow-id "$workflow_id" \
  --type PlatformControlWorkflow \
  --task-queue lifecycle \
  --input "{\"intent_id\":\"$workflow_id\"}"

# PlatformControlWorkflow is intentionally signal-driven. Send the stop signal
# after Temporal accepts the start so the acceptance proves a real history,
# Worker deployment routing, and graceful completion rather than leaving a
# forever-running control workflow behind in the disposable namespace.
sleep 1
"${compose[@]}" exec -T temporal temporal --address temporal:7233 --namespace "$temporal_namespace" workflow signal \
  --workflow-id "$workflow_id" \
  --name stop \
  --input 'true'

description=""
for _ in $(seq 1 45); do
  description=$("${compose[@]}" exec -T temporal temporal --address temporal:7233 --namespace "$temporal_namespace" workflow describe \
    --output json --workflow-id "$workflow_id")
  if printf '%s\n' "$description" | grep -qiE '"status"[[:space:]]*:[[:space:]]*"(WORKFLOW_EXECUTION_STATUS_)?COMPLETED"'; then
    break
  fi
  sleep 1
done
printf '%s\n' "$description"

if ! printf '%s\n' "$description" | grep -qiE '"status"[[:space:]]*:[[:space:]]*"(WORKFLOW_EXECUTION_STATUS_)?COMPLETED"'; then
  "${compose[@]}" ps >&2 || true
  "${compose[@]}" logs --no-color worker temporal >&2 || true
  echo "Go active workflow did not reach COMPLETED" >&2
  exit 1
fi
if printf '%s\n' "$description" | grep -qiE '"status"[[:space:]]*:[[:space:]]*"(WORKFLOW_EXECUTION_STATUS_)?(FAILED|CANCELED|TERMINATED|TIMED_OUT)"'; then
  echo "Go active workflow reached a terminal failure state" >&2
  exit 1
fi

if ! printf '%s\n' "$description" | node -e '
let input = "";
process.stdin.on("data", (chunk) => { input += chunk; });
process.stdin.on("end", () => {
  const info = JSON.parse(input).workflowExecutionInfo ?? {};
  const version = info.versioningInfo?.deploymentVersion ?? {};
  const historyLength = Number(info.historyLength ?? 0);
  if (historyLength <= 0 || version.deploymentName !== "fluctlight" || version.buildId !== process.env.EXPECTED_WORKER_BUILD_ID) {
    process.exit(1);
  }
});
'; then
  echo "Go active workflow metadata did not include history and deployment version evidence" >&2
  exit 1
fi
