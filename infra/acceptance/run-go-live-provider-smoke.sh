#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

: "${FLUCTLIGHT_LIVE_PROVIDER_URL:?set FLUCTLIGHT_LIVE_PROVIDER_URL to an OpenAI-compatible /v1 endpoint}"
: "${FLUCTLIGHT_LIVE_PROVIDER_MODEL:?set FLUCTLIGHT_LIVE_PROVIDER_MODEL to a model exposed by the endpoint}"

base_url=${FLUCTLIGHT_LIVE_PROVIDER_URL%/}

# Fail before starting Go tests when the endpoint is not reachable. The body is
# intentionally discarded so model metadata or provider diagnostics never land
# in a shell log.
if [[ -n "${FLUCTLIGHT_LIVE_PROVIDER_API_KEY:-}" ]]; then
	curl --fail --silent --show-error --max-time "${FLUCTLIGHT_LIVE_PROVIDER_PROBE_TIMEOUT_SECONDS:-5}" \
		-H "Authorization: Bearer ${FLUCTLIGHT_LIVE_PROVIDER_API_KEY}" -o /dev/null "${base_url}/models"
else
	curl --fail --silent --show-error --max-time "${FLUCTLIGHT_LIVE_PROVIDER_PROBE_TIMEOUT_SECONDS:-5}" \
		-o /dev/null "${base_url}/models"
fi

export FLUCTLIGHT_LIVE_PROVIDER_TEST=1
export FLUCTLIGHT_LIVE_PROVIDER_REQUEST_TIMEOUT_SECONDS="${FLUCTLIGHT_LIVE_PROVIDER_REQUEST_TIMEOUT_SECONDS:-600}"

# The default set is the bounded persona/tool smoke. Memory and continuation
# cases are intentionally opt-in because a local reasoning model can spend
# several minutes on each long-context request.
test_regex=${FLUCTLIGHT_LIVE_PROVIDER_TEST_REGEX:-'TestLiveProvider(RecognizesImageGenerationIntent|RoleOrganization|ComplexMultiPersonalityInitialization|PersonalityDecision)$'}
test_timeout=${FLUCTLIGHT_LIVE_PROVIDER_TEST_TIMEOUT:-30m}
go -C apps/core-go test -count=1 -timeout "$test_timeout" -v \
	-run "$test_regex" \
	./internal/core
