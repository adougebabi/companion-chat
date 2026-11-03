# Real backend environment discovery

Measured 2026-10-28 03:46:52–03:47:35 UTC (11:46:52–11:47:35 Asia/Shanghai). Read-only discovery; no services started, login/setup performed, database changed, model generation invoked, or existing volumes removed.

## Actual runtime status

`docker context show` = `orbstack`. Default sandbox denied the Docker socket; approved outside-sandbox read-only inventory succeeded. `docker ps --format ...` returned zero running containers. `docker ps -a` found the retained `fluctlight-v3-delivery-1019` deployment, all stopped:

| Service | Image | State |
| --- | --- | --- |
| Core | fluctlight-core-go:v3-nonllm-1019 | Exited 1, finished 2026-10-26T01:32:46Z |
| Worker | same | Exited 0, finished 2026-10-21T02:07:11Z |
| Web | fluctlight-web:ux-1021 | Exited 0, finished 2026-10-26T01:32:54Z |
| PostgreSQL | pgvector/pgvector:pg16 | Exited 0, 2 days ago |
| Redis | redis:8.0.4-alpine | Exited 0, 2 days ago |
| Temporal | temporalio/auto-setup:1.29.7 | Exited 137, 2 days ago |
| MinIO | fluctlight-minio-source:RELEASE.2025-04-22T22-12-26Z | Exited 0, 2 days ago |
| migrate / cutover / minio-init | deployment images | Exited 0 |

Whitelisted Docker inspect (name, image, timestamps, exit status, Compose labels, mounts and host port bindings only) established that the retained Web publishes host port **18080**, while Core, PostgreSQL, Redis, Temporal and MinIO have **no host bindings**. Their intended endpoints exist only on `fluctlight-v3-delivery-1019_platform`: core:8080, postgres:5432, redis:6379, temporal:7233, minio:9000. Existing PostgreSQL storage is named `fluctlight-v3-delivery-1019_fluctlight_postgres`; it is retained and was not inspected for business records or changed.

Health GETs at 2026-10-28T03:47:20.963077Z, repeated outside the sandbox after sandbox network denial:

| Endpoint | Measured result |
| --- | --- |
| http://127.0.0.1:18080/health/ready | connection refused |
| http://127.0.0.1:13001/health/ready | connection refused |
| http://127.0.0.1:8080/health/ready | connection refused |
| http://127.0.0.1:8188/system_stats | HTTP 200, ComfyUI 0.31.0, MPS device |
| http://127.0.0.1:11234/v1/models | HTTP 200, 18 advertised models |

`lsof -nP -iTCP -sTCP:LISTEN` showed python on 8188 and mlx-serve on 11234. It showed no native Core/Postgres/Redis/Temporal listeners. The media renderer and model inventory are accessible but do not establish a working Core→Worker→Temporal→S3 media path. No generation was invoked.

Any UI fixture server started subsequently by another audit branch is a separate test runtime. A fixture /health or /api response cannot overturn the Docker/real endpoint findings. In particular, `apps/core-go/test/integration/httpapi_e2e_test.go:13` defines `mockIntegrationRepo`; its `TestE2EPlatformPingAndFluctlightRouting` at line 35 is an httptest contract check, not live PostgreSQL closure.

## Build and configuration blockers

Current checkout HEAD = `ef8e1f92` (2026-10-28T11:39:56+08:00). Cached Core image sha256:6700858103ad1a5630a989e0f09e93f619d8f4b9cce15bf9d252ccda503c82fe was built 2026-10-19T19:07:55+08:00. Cached Web image sha256:b27010e49f2b1421683303d796fe7d6618b6fd30d13b437c1e5bf14773c6b050 was built 2026-10-21T08:39:49+08:00. Neither image has `org.opencontainers.image.revision`. Both predate current HEAD and cannot certify current source; there is **no running build**. Core/Web images embed compiled code (`apps/core-go/Dockerfile:11`, `:12`, `:30`), not host source mounts. Docker inspect found Core only mounts its KEV volume; Web has no mounts. Exact source revision equivalence cannot be established from these metadata.

Retained deployment labels refer to `/private/tmp/fluctlight-v3-delivery-1019/deployment.env` and a Web override `/private/tmp/yaoguang-ux-1021/compose.yml`; both files are missing. `infra/compose/fluctlight.env` is absent. `infra/compose/fluctlight.local.env` exists: service key, settings key, PostgreSQL password, S3 access/secret key are present (values deliberately not printed); trusted origin is http://127.0.0.1:13001, Redis endpoint is redis:6379/0, bucket fluctlight-media, Temporal namespace default. Root `.env` supplies no Core/test DB runtime URL. Process environment lacks GO_CORE_TEST_DATABASE_URL and Core runtime URL. A present local env is a configuration input, not proof it matches retained database/keys; do not blindly restart retained volumes with it.

## Supported paths for genuine verification

1. **Smallest DB persistence verification:** provision a new task-owned pgvector PostgreSQL instance (not the retained business volume), then run selected current Go integration tests with GO_CORE_TEST_DATABASE_URL pointing to its administrative connection. `apps/core-go/internal/core/project_health_transaction_integration_test.go:82` implements `isolatedCoreTestRepository`: forces admin DB postgres (:94), creates a unique lac_core_* database (:99–105), applies migrations (:131), and drops only that unique database at cleanup (:122–127). A suitable initial selected test is `TestActionOutcomePersistsPerCallAndAsyncCompletionExactlyOnce` (:137), which exercises actual Postgres settlement. `apps/core-go/internal/httpapi/server_test.go:310` tests initialization authority with its own unique database, migrations and cleanup (:326–372). These tests use controlled provider scaffolding and prove their named transaction behaviors, not full browser or real-model acceptance. Require passed test events; missing DB causes skip, never pass. Do not assume all tests are safe against a business DB simply because some helpers create disposable databases.
2. **Smallest existing whole-stack health setup:** `FLUCTLIGHT_ENV_FILE=infra/compose/fluctlight.local.env ./infra/compose/run-platform-smoke.sh` is the existing disposable project path (after inspecting configuration readiness). It creates `fluctlight-smoke-$$` (:6), refuses collisions with existing containers/volumes/networks (:19–38), builds source and starts dependencies (:76), checks real /health/ready and /api/platform/ping (:52–59), verifies service/one-shot status (:94–116), then destroys **only that disposable project's** volumes (:42–45). It does not seed business entities or prove business mutations, and exits/cleans up before interactive browser auditing. It may download/build images. The default origin in local.env is port 13001 while this smoke defaults host publication to an ephemeral port (:8), so an interactive extension must explicitly align the published port and trusted origin before cookie/CSRF testing. Compose dependencies and startup sequence are at `infra/compose/fluctlight.compose.yml:141–225`.
3. **Interactive audit with real persistence:** use the same Compose file under a new explicit audit project name, fresh project volumes, rebuilt current Core/Web, aligned free Web port/origin, and a task-owned Temporal namespace/bucket. Preserve this deployment for actual public API/browser save→reload read-back. Do not resume the old project to evade provisioning. Existing README startup path is `README.md:300–340`; first-install Owner setup is `:322–329`, which creates credentials and is outside this discovery subtask. No generic fullsite seed was identified; existing model regression scripts create their own named run objects through APIs and preserve replay state rather than clear data.
4. **Actual model/media closure:** existing `scripts/e2e/v3-regression/README.md:13–32` requires latest API/Worker/DB/Redis/Temporal plus Provider/Comfy/storage, isolated deployment, matching Origin and explicit FLUCTLIGHT_E2E_ISOLATED=1. A private Owner cookie or password is required. `:43–58` documents preflight/live runs with persisted private state, `:102` states scripts do not clear the DB/delete existing users, and `:86–98` distinguishes HTTP/provider checks from browser and visual review. Real media script L16 and visual L09 cover actual image bytes/authorization; running them invokes business/model operations, not authorized within this read-only subtask. `README.md:285–298` separately distinguishes live-provider smoke from PostgreSQL/Worker/media closure.

## Commands used (sanitized)

- `date -u`, `docker context show`, `docker ps --format <names/images/status/ports>`, `docker ps -a --format <same>`.
- `docker image ls --format <repository/tag/id/created>`; `docker image inspect --format <id/created/revision>` for named Core/Web images.
- `docker inspect --format <whitelisted metadata>` for named retained deployment containers; no full inspect or environment dump.
- `lsof -nP -iTCP -sTCP:LISTEN` and Python urllib read-only GETs listed above with 3-second timeouts.
- Scoped rg/nl source reads; Python configuration inspection reported only key presence and credential-free endpoint metadata.

Result: real application backend persistence and business mutation verification are currently **blocked by stopped infrastructure and stale/missing deployment inputs**. Accessible Comfy/model endpoints and repository test fixtures cannot establish end-to-end business completion. No current live business closure is claimed.
