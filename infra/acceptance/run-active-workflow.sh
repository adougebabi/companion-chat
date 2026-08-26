#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

project="fluctlight-t12-active-$$"
compose_file="infra/compose/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-infra/compose/fluctlight.env.example}"
compose=(docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file")
source "$script_dir/read-compose-env.sh"
assert_disposable_compose_project "$project"
compose_started=0

cleanup() {
  if [[ "$compose_started" != 1 ]]; then return; fi
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

compose_started=1
"${compose[@]}" up --build --detach
ready=0
for _ in $(seq 1 60); do
  if "${compose[@]}" exec -T core python -c 'import urllib.request; urllib.request.urlopen("http://127.0.0.1:8080/health/ready")' >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
if [[ "$ready" != 1 ]]; then
  "${compose[@]}" ps
  "${compose[@]}" logs --no-color core worker temporal
  exit 1
fi

"${compose[@]}" exec -T temporal temporal --address temporal:7233 --namespace default workflow start \
  --workflow-id t12-active-workflow \
  --type PlatformControlWorkflow \
  --task-queue interaction \
  --input '{"intent_id":"t12-active-workflow","delay_seconds":1}'
description=""
for _ in $(seq 1 30); do
  description=$("${compose[@]}" exec -T temporal temporal --address temporal:7233 --namespace default workflow describe \
    --output json --workflow-id t12-active-workflow)
  if printf '%s\n' "$description" | grep -qiE '"status"[[:space:]]*:[[:space:]]*"COMPLETED"'; then
    break
  fi
  sleep 1
done
printf '%s\n' "$description"
if ! printf '%s\n' "$description" | grep -qiE '"status"[[:space:]]*:[[:space:]]*"COMPLETED"'; then
  "${compose[@]}" ps >&2 || true
  "${compose[@]}" logs --no-color worker redis 2>&1 | tail -n 120 >&2 || true
  echo "active workflow did not reach COMPLETED" >&2
  exit 1
fi
if printf '%s\n' "$description" | grep -qiE '"status"[[:space:]]*:[[:space:]]*"(FAILED|CANCELED|TERMINATED|TIMED_OUT)"'; then
  echo "active workflow reached a terminal failure state" >&2
  exit 1
fi
if ! printf '%s\n' "$description" | node -e '
let input = "";
process.stdin.on("data", (chunk) => { input += chunk; });
process.stdin.on("end", () => {
  const info = JSON.parse(input).workflowExecutionInfo ?? {};
  const version = info.versioningInfo?.deploymentVersion ?? {};
  if (Number(info.historyLength ?? 0) < 1 || version.deploymentName !== "fluctlight" || version.buildId !== "platform-v1") {
    process.exit(1);
  }
});
'; then
  echo "active workflow metadata did not include history and deployment version evidence" >&2
  exit 1
fi
