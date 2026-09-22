#!/usr/bin/env bash
set -uo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

usage() {
  cat <<'EOF'
Usage:
  infra/acceptance/run-go-live-provider-smoke.sh
  infra/acceptance/run-go-live-provider-smoke.sh --suite tools [--tool <formal-tool-name>]
  infra/acceptance/run-go-live-provider-smoke.sh --suite agents [--agent <registered-agent-id>]
  infra/acceptance/run-go-live-provider-smoke.sh --suite all

Options:
  --suite smoke|tools|agents|all
      Select the ordinary Provider smoke or the strict Tool/Agent acceptance runners.
      With no arguments, the command remains the ordinary Provider smoke.
  --tool <name>
      Run one of the fixed 18 product Tool rows. Requires --suite tools.
  --agent <id>
      Run one mapped FormalAgent E2E row. Requires --suite agents.
  --run-dir <new-path>
      Write evidence to this new directory. Existing paths are rejected.
  -h, --help
      Show this help.

Required environment:
  smoke: FLUCTLIGHT_LIVE_PROVIDER_URL, FLUCTLIGHT_LIVE_PROVIDER_MODEL
  tools: GO_CORE_TEST_DATABASE_URL; the full suite and schedule.replan also require
         FLUCTLIGHT_LIVE_PROVIDER_URL and FLUCTLIGHT_LIVE_PROVIDER_MODEL
  agents/all: GO_CORE_TEST_DATABASE_URL, FLUCTLIGHT_LIVE_PROVIDER_URL,
              FLUCTLIGHT_LIVE_PROVIDER_MODEL
  visual_identity Agent row (and full agents/all):
              FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE. The JSON file contains the
              exact media_comfyui runtime setting and an isolated S3 bucket
              prefix/credentials; FLUCTLIGHT_VISUAL_LIVE_S3_* may override its
              S3 fields. See research/visual-live-harness.md for the schema.

FLUCTLIGHT_LIVE_PROVIDER_API_KEY is optional for endpoints that do not require
authentication. Its value is never written to evidence. Every Go command uses
-p 1 -parallel 1. A host-wide lock prevents overlapping invocations of this
runner from competing for the same Provider/GPU.

The tools/agents/all modes are strict: a nonzero Go exit, zero matches, a
missing required test, SKIP, BLOCKED, or FAIL makes the overall exit nonzero.
An exit zero proves only the rows mapped by this runner; it does not turn the
ordinary smoke into dual E2E evidence or fill separately documented matrix gaps.
EOF
}

tool_ids() {
  cat <<'EOF'
conversation.reply
moment.publish
media.image.generate
visual_identity.initialize
visual_identity.generate_candidate
visual_identity.commit_review
visual_identity.finalize
scene_event
presence_event
schedule.replan
memory_event
active_memory_event
affect_event
memory.recall
relationship.lookup
capability.request
persona.takeover
persona.switch
EOF
}

fixed_product_tool_ids() {
  awk '
    /var independentToolProductInventory = \[\]string\{/ { in_inventory=1; next }
    in_inventory && /^}/ { exit }
    in_inventory {
      line=$0
      sub(/^[[:space:]]*"/, "", line)
      sub(/",?[[:space:]]*$/, "", line)
      if (line != $0 && line != "") print line
    }
  ' apps/core-go/internal/core/independent_tools_e2e_test.go
}

# These are the 17 concrete TestFormalAgentE2E subtests currently owned by the
# formal Agent E2E slice. A newly registered Agent must be added here only when
# its own real-Provider subtest exists. Full agents/all runs fail closed while a
# registered Agent is unmapped.
agent_ids() {
  cat <<'EOF'
conversation_cognition
wake_up
takeover_judge
takeover_reply
initialization
media_prompt
media_quality
visual_identity_vision
visual_identity_patch
visual_identity
conversation_summary
schedule_generation
native_cognition
daily_review
persistent_switch
reflection
schedule_replan
EOF
}

# Fixed controlled cross-Agent regressions. These exercise the production
# Runner and persistence boundaries without contacting the configured live
# Provider. Keep this list explicit so a renamed or deleted contract test makes
# the full agents/all suites fail during preflight.
controlled_agent_gate_tests() {
  cat <<'EOF'
TestFormalAgentE2ERejectsBrokenToolResultFeedback
TestRunADKLoopContinuesAcrossBusinessFailureForThreeModelDecisions
TestRunADKLoopPreservesSameRoundMultipleCallAssociation
TestRunADKLoopToolSuccessThenModelErrorPreservesPartialResult
TestCommittedReplySurvivesInvalidFinalAndFailedRunDoesNotReplay
TestVisualIdentityAgentPreservesAcceptedGenerationWhenFinalModelFails
TestAgentRunRecordRetainsCommittedToolAfterCancellationAndPreventsReplay
TestInitializationProviderErrorsDistinguishTimeoutAndCancellation
TestPostgresProviderTimeoutPersistsOneTypedTerminalState
TestPostgresProviderCancellationPersistsTerminalStateAfterContextCancellation
TestRunADKLoopMaxIterationsReturnsErrorWithPartialFacts
TestRunADKLoopCancellationDoesNotFabricateFinalMessage
TestRunADKLoopDoesNotReuseTextBeforeFinalEmptyAssistant
TestContextReferenceIndexContainsOnlyCurrentProviderScope
TestProviderFormalAgentStreamingUsesTheSameRunnerStream
TestDirectConversationStreamsCommittedUserBeforeProviderAndAssistantAfterCommit
EOF
}

live_agent_gate_tests() {
  cat <<'EOF'
TestLiveStreamTurnFormalAgentNDJSON
EOF
}

contains_line() {
  local needle=$1
  local values=$2
  while IFS= read -r value; do
    [[ "$value" == "$needle" ]] && return 0
  done <<< "$values"
  return 1
}

tool_test_regex() {
  case "$1" in
    conversation.reply) printf '%s\n' '^(TestExecuteToolConversationReplyPublishesReplaysAndRejectsPayloadConflict|TestExecuteToolConversationReplyRejectsUnownedTargetWithoutProduct)$' ;;
    moment.publish) printf '%s\n' '^TestExecuteToolMomentPublishCommitsOutboxAndReplays$' ;;
    media.image.generate) printf '%s\n' '^(TestExecuteToolImageGenerateAcceptsDurableTaskAndRejectsConflict|TestExecuteToolImageGenerateRejectsInvalidTargetWithoutIntent|TestExecuteToolImageGenerateDependencyFailureCreatesNoIntent)$' ;;
    visual_identity.initialize) printf '%s\n' '^TestIndependentToolE2EVisualIdentityInitialize$' ;;
    visual_identity.generate_candidate) printf '%s\n' '^TestVisualIdentityGenerateCandidateToolCommitsDurableIntentAndReplays$' ;;
    visual_identity.commit_review) printf '%s\n' '^TestVisualIdentityCommitReviewToolPreservesRejectedAssetAndCreatesNextAttempt$' ;;
    visual_identity.finalize) printf '%s\n' '^TestVisualIdentityFinalizeToolCommitsCanonicalCharacterSheetAndCompletion$' ;;
    scene_event) printf '%s\n' '^TestIndependentToolE2ESceneEvent$' ;;
    presence_event) printf '%s\n' '^TestIndependentToolE2EPresenceEvent$' ;;
    schedule.replan) printf '%s\n' '^TestIndependentToolE2EScheduleReplan$' ;;
    memory_event|memory.recall) printf '%s\n' '^TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay$' ;;
    active_memory_event) printf '%s\n' '^TestPostgresDirectActiveMemoryToolCommitsRejectsReplaysAndPersistsAudit$' ;;
    affect_event) printf '%s\n' '^TestIndependentToolE2EAffectEvent$' ;;
    relationship.lookup) printf '%s\n' '^TestIndependentToolE2ERelationshipLookup$' ;;
    capability.request) printf '%s\n' '^TestIndependentToolE2ECapabilityRequest$' ;;
    persona.takeover|persona.switch) printf '%s\n' '^(TestPersonaToolsAreAvailableToAgentsAndCommitThroughDomainService|TestPostgresDirectPersonaToolsCommitRejectReplayConflictAndAudit)$' ;;
    *) return 1 ;;
  esac
}

tool_expected_tests() {
  case "$1" in
    conversation.reply)
      printf '%s\n' TestExecuteToolConversationReplyPublishesReplaysAndRejectsPayloadConflict TestExecuteToolConversationReplyRejectsUnownedTargetWithoutProduct ;;
    moment.publish) printf '%s\n' TestExecuteToolMomentPublishCommitsOutboxAndReplays ;;
    media.image.generate)
      printf '%s\n' TestExecuteToolImageGenerateAcceptsDurableTaskAndRejectsConflict TestExecuteToolImageGenerateRejectsInvalidTargetWithoutIntent TestExecuteToolImageGenerateDependencyFailureCreatesNoIntent ;;
    visual_identity.initialize) printf '%s\n' TestIndependentToolE2EVisualIdentityInitialize ;;
    visual_identity.generate_candidate) printf '%s\n' TestVisualIdentityGenerateCandidateToolCommitsDurableIntentAndReplays ;;
    visual_identity.commit_review) printf '%s\n' TestVisualIdentityCommitReviewToolPreservesRejectedAssetAndCreatesNextAttempt ;;
    visual_identity.finalize) printf '%s\n' TestVisualIdentityFinalizeToolCommitsCanonicalCharacterSheetAndCompletion ;;
    scene_event) printf '%s\n' TestIndependentToolE2ESceneEvent ;;
    presence_event) printf '%s\n' TestIndependentToolE2EPresenceEvent ;;
    schedule.replan) printf '%s\n' TestIndependentToolE2EScheduleReplan ;;
    memory_event|memory.recall) printf '%s\n' TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay ;;
    active_memory_event) printf '%s\n' TestPostgresDirectActiveMemoryToolCommitsRejectsReplaysAndPersistsAudit ;;
    affect_event) printf '%s\n' TestIndependentToolE2EAffectEvent ;;
    relationship.lookup) printf '%s\n' TestIndependentToolE2ERelationshipLookup ;;
    capability.request) printf '%s\n' TestIndependentToolE2ECapabilityRequest ;;
    persona.takeover|persona.switch)
      printf '%s\n' TestPersonaToolsAreAvailableToAgentsAndCommitThroughDomainService TestPostgresDirectPersonaToolsCommitRejectReplayConflictAndAudit ;;
    *) return 1 ;;
  esac
}

agent_test_regex() {
  contains_line "$1" "$(agent_ids)" || return 1
  printf '^TestFormalAgentE2E/%s$\n' "$1"
}

slugify() {
  printf '%s' "$1" | tr '. _/' '----' | tr -cd '[:alnum:]-'
}

suite=smoke
selected_tool=
selected_agent=
requested_run_dir=
while [[ $# -gt 0 ]]; do
  case "$1" in
    --suite)
      [[ $# -ge 2 ]] || { echo "--suite requires a value" >&2; usage >&2; exit 2; }
      suite=$2
      shift 2
      ;;
    --tool)
      [[ $# -ge 2 ]] || { echo "--tool requires a value" >&2; usage >&2; exit 2; }
      selected_tool=$2
      shift 2
      ;;
    --agent)
      [[ $# -ge 2 ]] || { echo "--agent requires a value" >&2; usage >&2; exit 2; }
      selected_agent=$2
      shift 2
      ;;
    --run-dir)
      [[ $# -ge 2 ]] || { echo "--run-dir requires a value" >&2; usage >&2; exit 2; }
      requested_run_dir=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$suite" in
  smoke|tools|agents|all) ;;
  *) echo "unknown suite: $suite" >&2; usage >&2; exit 2 ;;
esac
if [[ -n "$selected_tool" && "$suite" != tools ]]; then
  echo "--tool requires --suite tools" >&2
  exit 2
fi
if [[ -n "$selected_agent" && "$suite" != agents ]]; then
  echo "--agent requires --suite agents" >&2
  exit 2
fi
if [[ -n "$selected_tool" ]] && ! contains_line "$selected_tool" "$(tool_ids)"; then
  echo "unknown formal Tool name: $selected_tool" >&2
  exit 2
fi
if [[ -n "$selected_agent" ]] && ! contains_line "$selected_agent" "$(agent_ids)"; then
  echo "unmapped or unknown FormalAgent ID: $selected_agent" >&2
  exit 2
fi

lock_dir=${FLUCTLIGHT_LIVE_PROVIDER_LOCK_DIR:-${TMPDIR:-/tmp}/fluctlight-live-provider-runner.lock}
lock_acquired=0
cleanup_lock() {
  [[ "$lock_acquired" == 1 ]] || return 0
  rm -f -- "$lock_dir/pid"
  rmdir -- "$lock_dir" 2>/dev/null || true
}
trap cleanup_lock EXIT INT TERM

acquire_lock() {
  if mkdir -- "$lock_dir" 2>/dev/null; then
    printf '%s\n' "$$" > "$lock_dir/pid"
    lock_acquired=1
    return 0
  fi
  local owner=
  if [[ -f "$lock_dir/pid" ]]; then owner=$(sed -n '1p' "$lock_dir/pid" 2>/dev/null || true); fi
  if [[ "$owner" =~ ^[0-9]+$ ]] && kill -0 "$owner" 2>/dev/null; then
    echo "another live Provider runner holds $lock_dir (pid $owner)" >&2
    return 1
  fi
  rm -f -- "$lock_dir/pid" 2>/dev/null || true
  rmdir -- "$lock_dir" 2>/dev/null || true
  if ! mkdir -- "$lock_dir" 2>/dev/null; then
    echo "could not acquire live Provider runner lock: $lock_dir" >&2
    return 1
  fi
  printf '%s\n' "$$" > "$lock_dir/pid"
  lock_acquired=1
}

acquire_lock || exit 1

run_root=${FLUCTLIGHT_LIVE_PROVIDER_RUN_ROOT:-$repo_root/.trellis/tasks/09-22-agent-tool-plugin-e2e/research/runs}
timestamp=$(date -u +%Y%m%dT%H%M%SZ)
if [[ -n "$requested_run_dir" ]]; then
  if [[ "$requested_run_dir" = /* ]]; then run_dir=$requested_run_dir; else run_dir="$repo_root/$requested_run_dir"; fi
  if [[ -e "$run_dir" ]]; then
    echo "refusing to overwrite existing run directory: $run_dir" >&2
    exit 2
  fi
  mkdir -p -- "$run_dir"
else
  mkdir -p -- "$run_root"
  run_dir=$(mktemp -d "$run_root/serial-${suite}-${timestamp}-XXXXXX")
fi
mkdir -p -- "$run_dir/commands"

head_revision=$(git rev-parse HEAD 2>/dev/null || printf 'unavailable')
case "$suite" in
  smoke) identity=ordinary-provider-smoke ;;
  tools) identity="tool-e2e:${selected_tool:-all-tools}" ;;
  agents) identity="agent-e2e:${selected_agent:-all-agents}" ;;
  all) identity=dual-e2e ;;
esac
{
  printf 'suite=%s\n' "$suite"
  printf 'identity=%s\n' "$identity"
  printf 'head=%s\n' "$head_revision"
  printf 'started_at_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'selected_tool=%s\n' "${selected_tool:-<all>}"
  printf 'selected_agent=%s\n' "${selected_agent:-<all>}"
} > "$run_dir/run.meta"
for name in FLUCTLIGHT_LIVE_PROVIDER_URL FLUCTLIGHT_LIVE_PROVIDER_MODEL FLUCTLIGHT_LIVE_PROVIDER_API_KEY GO_CORE_TEST_DATABASE_URL FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE FLUCTLIGHT_VISUAL_LIVE_S3_ENDPOINT FLUCTLIGHT_VISUAL_LIVE_S3_REGION FLUCTLIGHT_VISUAL_LIVE_S3_ACCESS_KEY FLUCTLIGHT_VISUAL_LIVE_S3_SECRET_KEY FLUCTLIGHT_VISUAL_LIVE_S3_BUCKET_PREFIX FLUCTLIGHT_VISUAL_LIVE_S3_USE_SSL; do
  if [[ -n "${!name:-}" ]]; then state=present; else state=missing; fi
  printf '%s=%s\n' "$name" "$state" >> "$run_dir/environment-names.txt"
done
printf 'case\tlabel\tgo_exit\tverifier_exit\tstatus\n' > "$run_dir/commands.tsv"

fail_preflight() {
  printf '%s\n' "$1" | tee -a "$run_dir/preflight-errors.txt" >&2
  printf 'overall_status=FAIL\nfinished_at_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$run_dir/run.meta"
  echo "evidence: $run_dir" >&2
  exit 1
}

require_env() {
  local name=$1
  [[ -n "${!name:-}" ]] || fail_preflight "required environment variable is missing: $name"
}

if [[ "$suite" == tools || "$suite" == agents || "$suite" == all ]]; then
  require_env GO_CORE_TEST_DATABASE_URL
fi
needs_provider=0
if [[ "$suite" == smoke || "$suite" == agents || "$suite" == all ]]; then needs_provider=1; fi
if [[ "$suite" == tools && ( -z "$selected_tool" || "$selected_tool" == schedule.replan ) ]]; then needs_provider=1; fi
if [[ "$needs_provider" == 1 ]]; then
  require_env FLUCTLIGHT_LIVE_PROVIDER_URL
  require_env FLUCTLIGHT_LIVE_PROVIDER_MODEL
fi

needs_visual=0
if [[ "$suite" == all || ( "$suite" == agents && ( -z "$selected_agent" || "$selected_agent" == visual_identity ) ) ]]; then
  needs_visual=1
fi

validate_visual_live_config() {
  require_env FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE
  local config_file=$FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE
  [[ -f "$config_file" && -r "$config_file" ]] || fail_preflight "FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE must name a readable local JSON file"
  local validation_output validation_code
  validation_output=$(node - "$config_file" <<'NODE' 2>&1
const fs = require("node:fs");
const file = process.argv[2];
let root;
try {
  root = JSON.parse(fs.readFileSync(file, "utf8"));
} catch (error) {
  console.error(`invalid JSON: ${error.message}`);
  process.exit(1);
}
const media = root.media_comfyui ?? root["media.comfyui"];
if (!media || typeof media !== "object" || Array.isArray(media)) {
  console.error("media_comfyui object is required");
  process.exit(1);
}
const baseURL = media.baseUrl ?? media.base_url;
if (typeof baseURL !== "string" || baseURL.trim() === "") {
  console.error("media_comfyui baseUrl is required");
  process.exit(1);
}
try {
  const parsed = new URL(baseURL);
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") throw new Error("not HTTP(S)");
} catch {
  console.error("media_comfyui baseUrl must be an HTTP(S) URL");
  process.exit(1);
}
if (!media.workflow || typeof media.workflow !== "object" || Array.isArray(media.workflow) || Object.keys(media.workflow).length === 0) {
  console.error("media_comfyui workflow object is required");
  process.exit(1);
}
const s3 = root.s3 && typeof root.s3 === "object" && !Array.isArray(root.s3) ? root.s3 : {};
const pick = (envName, ...keys) => {
  if (typeof process.env[envName] === "string" && process.env[envName].trim() !== "") return process.env[envName].trim();
  for (const key of keys) if (typeof s3[key] === "string" && s3[key].trim() !== "") return s3[key].trim();
  return "";
};
const required = [
  ["S3 endpoint", pick("FLUCTLIGHT_VISUAL_LIVE_S3_ENDPOINT", "endpoint")],
  ["S3 access key", pick("FLUCTLIGHT_VISUAL_LIVE_S3_ACCESS_KEY", "access_key", "accessKey")],
  ["S3 secret key", pick("FLUCTLIGHT_VISUAL_LIVE_S3_SECRET_KEY", "secret_key", "secretKey")],
  ["S3 bucket prefix", pick("FLUCTLIGHT_VISUAL_LIVE_S3_BUCKET_PREFIX", "bucket_prefix", "bucketPrefix")],
];
const missing = required.filter(([, value]) => value === "").map(([name]) => name);
if (missing.length > 0) {
  console.error(`missing ${missing.join(", ")} (set JSON s3 fields or FLUCTLIGHT_VISUAL_LIVE_S3_* overrides)`);
  process.exit(1);
}
NODE
  )
  validation_code=$?
  if [[ "$validation_code" != 0 ]]; then
    fail_preflight "Visual Identity live configuration is invalid: $validation_output"
  fi
}

if [[ "$needs_visual" == 1 ]]; then
  validate_visual_live_config
fi

validate_agent_inventory() {
  local registered mapped id errors=0
  registered=$(sed -n 's/^[[:space:]]*FormalAgent[A-Za-z0-9_]*[[:space:]]*FormalAgentID[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' apps/core-go/internal/core/formal_agents.go)
  mapped=$(agent_ids)
  if [[ -z "$registered" ]]; then
    echo "could not read any registered FormalAgent IDs" >&2
    return 1
  fi
  while IFS= read -r id; do
    if ! contains_line "$id" "$mapped"; then
      echo "registered FormalAgent has no runner mapping: $id" >&2
      errors=1
    fi
  done <<< "$registered"
  while IFS= read -r id; do
    if ! contains_line "$id" "$registered"; then
      echo "runner maps an unregistered FormalAgent: $id" >&2
      errors=1
    fi
  done <<< "$mapped"
  return "$errors"
}

validate_tool_inventory() {
  local expected mapped id errors=0
  expected=$(fixed_product_tool_ids)
  mapped=$(tool_ids)
  if [[ -z "$expected" ]]; then
    echo "could not read the fixed product Tool inventory" >&2
    return 1
  fi
  while IFS= read -r id; do
    if ! contains_line "$id" "$mapped"; then
      echo "fixed product Tool has no runner mapping: $id" >&2
      errors=1
    fi
  done <<< "$expected"
  while IFS= read -r id; do
    if ! contains_line "$id" "$expected"; then
      echo "runner maps a Tool absent from the fixed product inventory: $id" >&2
      errors=1
    fi
  done <<< "$mapped"
  return "$errors"
}

required_matrix_test_exists() {
  local name=$1
  grep -Eq "^func ${name}\\(" apps/core-go/internal/core/*_test.go
}

validate_tool_matrix_gates() {
  local errors=0 name
  for name in TestFormalToolEinoAdapterE2E TestIndependentToolE2ERejectsFalseSuccessWithoutWrite; do
    if ! required_matrix_test_exists "$name"; then
      echo "required full Tool matrix test is missing: $name" >&2
      errors=1
    fi
  done
  return "$errors"
}

validate_agent_matrix_gates() {
  local errors=0 name
  while IFS= read -r name; do
    if ! required_matrix_test_exists "$name"; then
      echo "required controlled Agent cross-case test is missing: $name" >&2
      errors=1
    fi
  done < <(controlled_agent_gate_tests)
  while IFS= read -r name; do
    if ! required_matrix_test_exists "$name"; then
      echo "required live Agent cross-case test is missing: $name" >&2
      errors=1
    fi
  done < <(live_agent_gate_tests)
  return "$errors"
}

coverage_errors=
if [[ "$suite" == tools || "$suite" == all ]]; then
  if [[ -z "$selected_tool" ]]; then
    inventory_output=$(validate_tool_inventory 2>&1)
    inventory_code=$?
    printf '%s\n' "$inventory_output" > "$run_dir/tool-inventory.txt"
    if [[ "$inventory_code" != 0 ]]; then coverage_errors="${coverage_errors}${inventory_output}"$'\n'; fi
  fi
  matrix_output=$(validate_tool_matrix_gates 2>&1)
  matrix_code=$?
  printf '%s\n' "$matrix_output" > "$run_dir/tool-matrix-gates.txt"
  if [[ "$matrix_code" != 0 ]]; then coverage_errors="${coverage_errors}${matrix_output}"$'\n'; fi
fi

if [[ "$suite" == agents || "$suite" == all ]]; then
  if [[ -z "$selected_agent" ]]; then
    inventory_output=$(validate_agent_inventory 2>&1)
    inventory_code=$?
    printf '%s\n' "$inventory_output" > "$run_dir/agent-inventory.txt"
    if [[ "$inventory_code" != 0 ]]; then coverage_errors="${coverage_errors}${inventory_output}"$'\n'; fi

    matrix_output=$(validate_agent_matrix_gates 2>&1)
    matrix_code=$?
    printf '%s\n' "$matrix_output" > "$run_dir/agent-matrix-gates.txt"
    if [[ "$matrix_code" != 0 ]]; then coverage_errors="${coverage_errors}${matrix_output}"$'\n'; fi
  else
    registered=$(sed -n 's/^[[:space:]]*FormalAgent[A-Za-z0-9_]*[[:space:]]*FormalAgentID[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' apps/core-go/internal/core/formal_agents.go)
    contains_line "$selected_agent" "$registered" || fail_preflight "mapped FormalAgent is not registered: $selected_agent"
  fi
fi
if [[ -n "$coverage_errors" ]]; then fail_preflight "${coverage_errors%$'\n'}"; fi

sha256_file() {
  if command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1"; else sha256sum "$1"; fi
}

record_command_context() {
  local slug=$1
  local command_text=$2
  local context_file="$run_dir/commands/${slug}.context.txt"
  {
    printf 'head=%s\n' "$head_revision"
    printf 'command=%s\n' "$command_text"
    printf 'source_manifest=%s.sources.sha256\n' "$slug"
    printf 'FLUCTLIGHT_LIVE_PROVIDER_URL=%s\n' "$([[ -n "${FLUCTLIGHT_LIVE_PROVIDER_URL:-}" ]] && printf present || printf missing)"
    printf 'FLUCTLIGHT_LIVE_PROVIDER_MODEL=%s\n' "$([[ -n "${FLUCTLIGHT_LIVE_PROVIDER_MODEL:-}" ]] && printf present || printf missing)"
    printf 'FLUCTLIGHT_LIVE_PROVIDER_API_KEY=%s\n' "$([[ -n "${FLUCTLIGHT_LIVE_PROVIDER_API_KEY:-}" ]] && printf present || printf missing)"
    printf 'GO_CORE_TEST_DATABASE_URL=%s\n' "$([[ -n "${GO_CORE_TEST_DATABASE_URL:-}" ]] && printf present || printf missing)"
    printf 'FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE=%s\n' "$([[ -n "${FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE:-}" ]] && printf present || printf missing)"
  } > "$context_file"
  git ls-files -co --exclude-standard -- apps/core-go | LC_ALL=C sort | while IFS= read -r source; do
    case "$source" in
      *.go) [[ -f "$source" ]] && sha256_file "$source" ;;
    esac
  done > "$run_dir/commands/${slug}.sources.sha256"
}

overall=0
record_result() {
  local slug=$1 label=$2 go_code=$3 verifier_code=$4
  local status=PASS
  if [[ "$go_code" != 0 || "$verifier_code" != 0 ]]; then status=FAIL; overall=1; fi
  printf '%s\t%s\t%s\t%s\t%s\n' "$slug" "$label" "$go_code" "$verifier_code" "$status" >> "$run_dir/commands.tsv"
  echo "[E2E] $label: $status"
}

run_provider_probe() {
  local slug=provider-probe
  local base_url=${FLUCTLIGHT_LIVE_PROVIDER_URL%/}
  local sanitized='curl --fail --silent --show-error --max-time ${FLUCTLIGHT_LIVE_PROVIDER_PROBE_TIMEOUT_SECONDS:-5} [optional Authorization header redacted] -o /dev/null ${FLUCTLIGHT_LIVE_PROVIDER_URL%/}/models'
  record_command_context "$slug" "$sanitized"
  if [[ -n "${FLUCTLIGHT_LIVE_PROVIDER_API_KEY:-}" ]]; then
    curl --fail --silent --show-error --max-time "${FLUCTLIGHT_LIVE_PROVIDER_PROBE_TIMEOUT_SECONDS:-5}" \
      -H "Authorization: Bearer ${FLUCTLIGHT_LIVE_PROVIDER_API_KEY}" -o /dev/null "${base_url}/models" \
      > "$run_dir/commands/${slug}.log" 2>&1
  else
    curl --fail --silent --show-error --max-time "${FLUCTLIGHT_LIVE_PROVIDER_PROBE_TIMEOUT_SECONDS:-5}" \
      -o /dev/null "${base_url}/models" > "$run_dir/commands/${slug}.log" 2>&1
  fi
  local code=$?
  printf '%s\n' "$code" > "$run_dir/commands/${slug}.exit.txt"
  if [[ "$code" != 0 ]]; then
    overall=1
    printf '%s\t%s\t%s\t%s\t%s\n' "$slug" provider-connectivity "$code" not-run FAIL >> "$run_dir/commands.tsv"
    echo "[E2E] Provider connectivity: FAIL (see $run_dir/commands/${slug}.log)" >&2
    return 1
  fi
  printf '%s\t%s\t%s\t%s\t%s\n' "$slug" provider-connectivity 0 not-applicable PASS >> "$run_dir/commands.tsv"
  echo "[E2E] Provider connectivity: PASS"
}

export FLUCTLIGHT_LIVE_PROVIDER_TEST=1
export FLUCTLIGHT_LIVE_PROVIDER_REQUEST_TIMEOUT_SECONDS="${FLUCTLIGHT_LIVE_PROVIDER_REQUEST_TIMEOUT_SECONDS:-600}"
test_timeout=${FLUCTLIGHT_LIVE_PROVIDER_TEST_TIMEOUT:-30m}

run_go_case() {
  local slug=$1 label=$2 regex=$3
  shift 3
  local expected=("$@")
  local events="$run_dir/commands/${slug}.jsonl"
  local result="$run_dir/commands/${slug}.result.json"
  local command_text
  printf -v command_text 'go -C apps/core-go test -mod=readonly -count=1 -timeout %q -p 1 -parallel 1 -json -run %q ./internal/core' "$test_timeout" "$regex"
  record_command_context "$slug" "$command_text"
  echo "[E2E] running $label"
  go -C apps/core-go test -mod=readonly -count=1 -timeout "$test_timeout" -p 1 -parallel 1 -json \
    -run "$regex" ./internal/core > "$events" 2>&1
  local go_code=$?
  printf '%s\n' "$go_code" > "$run_dir/commands/${slug}.exit.txt"
  node "$script_dir/verify-go-e2e-events.mjs" "$events" "$go_code" "$result" "${expected[@]}" \
    > "$run_dir/commands/${slug}.verify.log" 2>&1
  local verifier_code=$?
  record_result "$slug" "$label" "$go_code" "$verifier_code"
  if [[ "$verifier_code" != 0 ]]; then sed -n '1,120p' "$run_dir/commands/${slug}.verify.log" >&2; fi
  return 0
}

run_smoke() {
  local regex=${FLUCTLIGHT_LIVE_PROVIDER_TEST_REGEX:-'TestLiveProvider(RecognizesImageGenerationIntent|RoleOrganization|ComplexMultiPersonalityInitialization|PersonalityDecision)$'}
  local slug=ordinary-provider-smoke
  local events="$run_dir/commands/${slug}.jsonl"
  local result="$run_dir/commands/${slug}.result.json"
  local command_text
  printf -v command_text 'go -C apps/core-go test -mod=readonly -count=1 -timeout %q -p 1 -parallel 1 -json -run %q ./internal/core' "$test_timeout" "$regex"
  record_command_context "$slug" "$command_text"
  echo "[smoke] running ordinary Provider smoke"
  go -C apps/core-go test -mod=readonly -count=1 -timeout "$test_timeout" -p 1 -parallel 1 -json \
    -run "$regex" ./internal/core > "$events" 2>&1
  local go_code=$?
  printf '%s\n' "$go_code" > "$run_dir/commands/${slug}.exit.txt"
  node "$script_dir/verify-go-e2e-events.mjs" "$events" "$go_code" "$result" --require-any \
    > "$run_dir/commands/${slug}.verify.log" 2>&1
  local verifier_code=$?
  record_result "$slug" ordinary-provider-smoke "$go_code" "$verifier_code"
  if [[ "$verifier_code" != 0 ]]; then sed -n '1,120p' "$run_dir/commands/${slug}.verify.log" >&2; fi
}

run_tool_inventory_guard() {
  run_go_case tool-fixed-inventory fixed-product-tool-inventory '^TestIndependentToolE2EFixedProductInventory$' TestIndependentToolE2EFixedProductInventory
  local status
  status=$(tail -n 1 "$run_dir/commands.tsv" | cut -f5)
  [[ "$status" == PASS ]]
}

run_tool_matrix_gates() {
  local expected=(TestFormalToolEinoAdapterE2E)
  local name
  while IFS= read -r name; do expected+=("TestFormalToolEinoAdapterE2E/$name"); done < <(fixed_product_tool_ids)
  run_go_case tool-eino-adapter-matrix all-tools-formal-eino-adapter '^TestFormalToolEinoAdapterE2E$' "${expected[@]}"
  run_go_case tool-false-success-mutation false-success-without-write-mutation '^TestIndependentToolE2ERejectsFalseSuccessWithoutWrite$' TestIndependentToolE2ERejectsFalseSuccessWithoutWrite
  local failures
  failures=$(tail -n 2 "$run_dir/commands.tsv" | awk -F '\t' '$5 != "PASS" { count++ } END { print count+0 }')
  [[ "$failures" == 0 ]]
}

run_agent_matrix_gates() {
  local expected=() name regex='^('
  while IFS= read -r name; do
    expected+=("$name")
    if [[ "$regex" != '^(' ]]; then regex+='|'; fi
    regex+="$name"
  done < <(controlled_agent_gate_tests)
  regex+=')$'
  run_go_case agent-controlled-cross-cases controlled-agent-cross-cases "$regex" "${expected[@]}"
  local status
  status=$(tail -n 1 "$run_dir/commands.tsv" | cut -f5)
  [[ "$status" == PASS ]] || return 1

  expected=()
  while IFS= read -r name; do expected+=("$name"); done < <(live_agent_gate_tests)
  run_go_case agent-live-production-stream live-production-streamturn-ndjson '^TestLiveStreamTurnFormalAgentNDJSON$' "${expected[@]}"
  status=$(tail -n 1 "$run_dir/commands.tsv" | cut -f5)
  [[ "$status" == PASS ]]
}

run_visual_dependency_preflight() {
  run_go_case visual-live-preflight visual-identity-live-dependencies '^TestVisualIdentityLiveE2EPreflight$' TestVisualIdentityLiveE2EPreflight
  local status
  status=$(tail -n 1 "$run_dir/commands.tsv" | cut -f5)
  [[ "$status" == PASS ]]
}

run_tool() {
  local name=$1 regex expected=()
  regex=$(tool_test_regex "$name") || { overall=1; return; }
  while IFS= read -r test_name; do expected+=("$test_name"); done < <(tool_expected_tests "$name")
  run_go_case "tool-$(slugify "$name")" "Tool $name" "$regex" "${expected[@]}"
}

run_selected_tool_adapter() {
  local name=$1 escaped regex
  escaped=${name//./\\.}
  regex="^TestFormalToolEinoAdapterE2E/${escaped}$"
  case "$name" in
    visual_identity.*)
      # The visual rows intentionally prepare one durable session across
      # initialize -> candidate -> review -> finalize. Run the full controlled
      # parent so selecting a later visual row does not depend on an omitted
      # sibling, while still requiring this exact child in the event verifier.
      regex='^TestFormalToolEinoAdapterE2E$'
      ;;
  esac
  run_go_case "tool-$(slugify "$name")-eino-adapter" "controlled Tool $name formal Eino adapter" "$regex" \
    TestFormalToolEinoAdapterE2E "TestFormalToolEinoAdapterE2E/$name"
  local status
  status=$(tail -n 1 "$run_dir/commands.tsv" | cut -f5)
  [[ "$status" == PASS ]]
}

run_agent() {
  local id=$1 regex
  regex=$(agent_test_regex "$id") || { overall=1; return; }
  run_go_case "agent-$(slugify "$id")" "Agent $id" "$regex" "TestFormalAgentE2E/$id"
}

if [[ "$needs_visual" == 1 ]] && ! run_visual_dependency_preflight; then
  printf 'overall_status=FAIL\nfinished_at_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$run_dir/run.meta"
  echo "[E2E] Visual Identity dependency preflight failed; no model row was started" >&2
  echo "evidence: $run_dir" >&2
  exit 1
fi

if [[ "$needs_provider" == 1 ]] && ! run_provider_probe; then
  printf 'overall_status=FAIL\nfinished_at_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$run_dir/run.meta"
  echo "evidence: $run_dir" >&2
  exit 1
fi

case "$suite" in
  smoke)
    run_smoke
    ;;
  tools)
    if [[ -n "$selected_tool" ]]; then
      if run_tool_inventory_guard && run_selected_tool_adapter "$selected_tool"; then
        run_tool "$selected_tool"
      else
        echo "[E2E] Tool inventory or selected formal adapter gate failed; direct Tool row was not started" >&2
      fi
    else
      if run_tool_matrix_gates && run_tool_inventory_guard; then
        while IFS= read -r name; do run_tool "$name"; done < <(tool_ids)
      else
        echo "[E2E] Tool matrix gate failed; Tool rows were not started" >&2
      fi
    fi
    ;;
  agents)
    if [[ -n "$selected_agent" ]]; then
      run_agent "$selected_agent"
    else
      if run_agent_matrix_gates; then
        while IFS= read -r id; do run_agent "$id"; done < <(agent_ids)
      else
        echo "[E2E] Agent mutation gate failed; live Agent rows were not started" >&2
      fi
    fi
    ;;
  all)
    if run_tool_matrix_gates && run_agent_matrix_gates && run_tool_inventory_guard; then
      while IFS= read -r name; do run_tool "$name"; done < <(tool_ids)
      while IFS= read -r id; do run_agent "$id"; done < <(agent_ids)
    else
      echo "[E2E] Tool inventory drifted; remaining rows were not started" >&2
    fi
    ;;
esac

status_text=PASS
if [[ "$overall" != 0 ]]; then status_text=FAIL; fi
printf 'overall_status=%s\nfinished_at_utc=%s\n' "$status_text" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$run_dir/run.meta"
echo "[$identity] $status_text"
echo "evidence: $run_dir"
exit "$overall"
