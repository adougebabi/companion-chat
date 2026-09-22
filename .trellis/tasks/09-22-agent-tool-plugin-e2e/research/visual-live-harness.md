# Visual Identity live acceptance harness

Status: **NOT_RUN**. The harness and strict runner preflight are implemented and
compiled, but no real Provider or ComfyUI generation request was sent while
building this slice.

## What the live row proves

`TestFormalAgentE2E/visual_identity` creates a disposable PostgreSQL database
through the existing formal-Agent fixture and a unique, test-owned S3 bucket.
It creates a real Visual Identity session, then advances it only through the
production `ProcessVisualIdentity` and `ProcessMediaIntent` handlers. The test
does not insert completed media intents or `media_assets`, and it does not use a
mock Provider, renderer, or scripted media result.

The success gate requires all of the following:

- the generated candidate and final character sheet are non-empty objects in
  the unique test bucket, and their object SHA-256 values match the authoritative
  ready `media_assets` rows;
- the profile is active with distinct canonical and character-sheet assets, and
  the session is completed;
- the formal Visual Identity Agent committed ledger rows for
  `visual_identity.generate_candidate`, `visual_identity.commit_review`, and
  `visual_identity.finalize`;
- every ledger row retains the native Provider request ID, ToolCall ID, result,
  and a subsequent model request containing that Tool result;
- the exact candidate object bytes appear in a real `image_url` multimodal
  request before the review decision;
- bounded total deadline, handler-transition count, product attempt count, and
  per-media-intent retry count all fail closed. `waiting`, `awaiting_review`, a
  renderer failure, timeout, missing dependency, or SKIP cannot pass the row.

This row calls the production handlers that Temporal activities call. It does
not start a Temporal worker or prove Temporal transport, history replay, task
queue routing, or worker recovery. Those remain connected through the existing
`visual_identity.initialize` and `media.generation` workflow intents and the
platform/Compose workflow verification; they must be reported separately from
this handler-level live acceptance row.

## Explicit local configuration

The runner requires `FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE` for the complete Agent
suite and for the selected `visual_identity` Agent row. The file is a local JSON
file that must not be committed. It contains the exact value to install as the
isolated database's `media.comfyui` runtime setting plus S3 connection data. The
bucket value is a prefix, not an existing bucket: the test appends a random
suffix, creates the bucket, removes all objects, and removes the bucket during
cleanup.

Example with placeholders only:

```json
{
  "media_comfyui": {
    "baseUrl": "http://127.0.0.1:8188",
    "workflow": {
      "REPLACE_WITH_REAL_COMFYUI_WORKFLOW": {}
    },
    "visual_identity_workflows": {
      "seed": {
        "REPLACE_WITH_REAL_SEED_WORKFLOW": {}
      },
      "character_sheet": {
        "REPLACE_WITH_REAL_CHARACTER_SHEET_WORKFLOW": {}
      }
    }
  },
  "s3": {
    "endpoint": "http://127.0.0.1:9000",
    "region": "us-east-1",
    "access_key": "REPLACE_LOCALLY",
    "secret_key": "REPLACE_LOCALLY",
    "bucket_prefix": "lac-visual-live",
    "use_ssl": false
  }
}
```

S3 values may instead be supplied by these environment variables, which
override the JSON fields without being copied to evidence:

```sh
FLUCTLIGHT_VISUAL_LIVE_S3_ENDPOINT=http://127.0.0.1:9000
FLUCTLIGHT_VISUAL_LIVE_S3_REGION=us-east-1
FLUCTLIGHT_VISUAL_LIVE_S3_ACCESS_KEY=REPLACE_LOCALLY
FLUCTLIGHT_VISUAL_LIVE_S3_SECRET_KEY=REPLACE_LOCALLY
FLUCTLIGHT_VISUAL_LIVE_S3_BUCKET_PREFIX=lac-visual-live
FLUCTLIGHT_VISUAL_LIVE_S3_USE_SSL=false
```

Optional guards are `FLUCTLIGHT_VISUAL_LIVE_TIMEOUT` (default `25m`),
`FLUCTLIGHT_VISUAL_LIVE_MAX_TRANSITIONS` (default `24`), and
`FLUCTLIGHT_VISUAL_LIVE_MAX_MEDIA_RUNS` (default `3`). The fixed product
attempt maximum remains authoritative and cannot be raised by test input.

The JSON may be assembled by copying the already verified `media.comfyui`
runtime setting from a dedicated local stack into a private file. The harness
only reads that file and writes the value into its disposable database; it does
not read, migrate, or update the source stack. Provider and PostgreSQL variables
may likewise be exported from a private env file before invoking the repository
runner. The runner never prints their values.

## Commands for the final user-run acceptance

First export the existing isolated PostgreSQL and real Provider settings, then
point to the private JSON file:

```sh
export GO_CORE_TEST_DATABASE_URL='postgresql://.../test_admin'
export FLUCTLIGHT_LIVE_PROVIDER_URL='http://127.0.0.1:.../v1'
export FLUCTLIGHT_LIVE_PROVIDER_MODEL='...'
export FLUCTLIGHT_LIVE_PROVIDER_API_KEY='...'
export FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE="$PWD/.local/visual-live.json"
```

Run only the non-generative dependency preflight (it checks ComfyUI
`/system_stats` and creates/removes one empty, uniquely named S3 bucket):

```sh
FLUCTLIGHT_LIVE_PROVIDER_TEST=1 \
  go -C apps/core-go test -mod=readonly -count=1 -timeout 5m -p 1 -parallel 1 \
  -run '^TestVisualIdentityLiveE2EPreflight$' ./internal/core
```

Run the complete Visual Identity Agent row through the strict repository
runner. This command performs real model and image generation requests:

```sh
infra/acceptance/run-go-live-provider-smoke.sh \
  --suite agents --agent visual_identity
```

Run the complete fixed matrices serially after the selected row succeeds:

```sh
infra/acceptance/run-go-live-provider-smoke.sh --suite tools
infra/acceptance/run-go-live-provider-smoke.sh --suite agents
infra/acceptance/run-go-live-provider-smoke.sh --suite all
```

Every strict run writes a new evidence directory. Missing configuration,
unreadable/invalid JSON, failed ComfyUI or S3 preflight, zero matched tests,
SKIP, BLOCKED, or any Go/test verifier failure makes the runner exit nonzero.
