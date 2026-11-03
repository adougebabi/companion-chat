# T02 isolated runtime evidence

Captured 2026-10-15 23:47 Asia/Shanghai. Scope: existing task Docker project `fluctlight-recovery-1015`, temporary runtime `/tmp/fluctlight-recovery-1015`, task test Owner and two existing `recovery-a-1015` / `recovery-b-1015` objects. No formal environment or other volumes changed. No source/client/migration changes made by this runtime slice.

## Running state

- Core API PID **6508**, listening **127.0.0.1:8080**. Existing binary built at 20:08 local; SHA256 is recorded in `t02-process-model-metadata.json`. This is baseline binary evidence, not current concurrent source acceptance.
- Frontend proxy **http://localhost:5173** was already running. Configured trusted Origin is `http://localhost:5173`. API live/ready return 200 through proxy.
- PostgreSQL **55432**, Redis **56379**, Temporal **57233** remain healthy. Docker images: pgvector/pgvector:pg16, redis:8.0.4-alpine, temporalio/auto-setup:1.29.7. Compose CLI v5.5.0; Go go1.26.3 darwin/arm64; Node v24.12.0.
- Worker PID **6509** passed working-persona/effective-life startup gates, started six Temporal queues, configured deployment build `recovery-1015`, and produced its ready signal. It was then **stopped deliberately** because it automatically resumed existing wake-up/schedule/visual-identity jobs, outside the bounded metadata-only verification. Wake-up logs show `provider_request_failed`; no successful model/media generation was demonstrated. Logs remain local at `/tmp/fluctlight-recovery-1015/worker.log`; API log at `api.log`.
- MinIO **59000** is unavailable. Two bounded attempts to start the repository-pinned images failed: first Docker Hub TLS handshake timeout, second `pull access denied for minio/mc`. Neither pinned MinIO image is cached locally. No image/tag/provider policy was replaced.

## Authentication, transport, generated client

Evidence: `t02-runtime-http.json`, `t02-browser-client.json`.

- Anonymous session 401, anonymous `/api/fluctlights` 401 `unauthenticated`; existing task Owner password login 200 and session 200.
- Session cookie HttpOnly + SameSite=Lax, CSRF cookie SameSite=Lax. Secure absent is consistent with the explicit local HTTP origin.
- Authenticated direct `http://localhost:8080` and proxied `http://localhost:5173` list/settings/provider reads return 200. Both existing objects detail and world reads return 200; no new objects created by this slice.
- Missing CSRF and wrong Origin rejected with 403 `invalid_origin`. Wrong-Origin test is asserted on direct Core because Vite development proxy rewrites Origin to its configured upstream Origin.
- Direct `/internal/settings` without service identity is 401. Proxy `/internal/settings` is Vite SPA fallback 200 HTML, not a successful Core response.
- Generated `BrowserClient` executed against real HTTP with the task session: `session`, `listFluctlights`, `detail` for both objects, `providerEndpoints`, `providerBindings`, `settings` all pass. Details include current_state. This validates actual generated client transport, not only raw HTTP.
- One exploratory wrong-Origin empty settings PUT through Vite returned 200 because Origin was rewritten; payload `{}` made no policy/model change. Final direct wrong-Origin and missing-CSRF checks return 403. Guessed `/profiles` and plural `/conversations` reads were 404 and discarded as incorrect route probes, not reported as positive evidence.
- Streaming buffering, chat inference, media object read/write, and full workflow result delivery were **not** verified in this metadata-only slice.

## Effective Owner configuration and reachable services

The authenticated task Owner has **0 endpoints**, **0 role bindings**, and **0 configured secret purposes**. Therefore no selected cognitive Qwen, independent Cydonia reply, embedding, or vision binding can be certified in this isolated database. `conversation.reply` is `mode=legacy`; capability status is disabled/config_complete=false. Vision understanding is disabled/config_complete=false. `media.comfyui` setting does not exist. No provider or system policy was changed.

Metadata-only requests returned 200: LM Studio 1234 has 4 models, MLX 11234 has 14 models, ComfyUI 8188 `/system_stats` responds. Model IDs are captured in `t02-process-model-metadata.json`. This proves reachability/catalog availability only; it does not prove Owner-selected configuration, inference quality, embedding compatibility, or renderer workflows. Missing concrete inputs: intended endpoint/role/model choices in this test Owner, independent reply enable/config, vision choice/enable if required, Comfy workflow settings; MinIO pinned image retrieval.

## Database regression

`GO_CORE_TEST_DATABASE_URL` is populated only inside the child from the private task startup environment. `TestWorldRuntimeFreshPriorHeadUpgradeAndRerun` uses the repository helper to CREATE a uniquely named disposable `lac_affect_migration_*` database, runs fresh migration to 0064, regresses that disposable database to prior head 0063, upgrades and reruns, verifies historical facts, and DROPs only its own database on cleanup.

Actual result: **PASS**, 1.30s test / 1.831s package. Evidence: `t02-world-migration-test.log`. Separate read-only check: task application database remains `0064_world_runtime`, endpoint count 0, no `lac_affect_migration_*` databases remain. No task/user database reset or downgrade occurred.

## Reproduction and handoff

Private startup files retain mode 0600; do not print env/DSN/cookies/password/setup-token. From repository root:

```sh
python3 /tmp/fluctlight-recovery-1015/envexec.py python3 /tmp/fluctlight-recovery-1015/start-runtime.py api
python3 /tmp/fluctlight-recovery-1015/envexec.py python3 /tmp/fluctlight-recovery-1015/inspect-runtime.py
packages/browser-client/node_modules/.bin/tsx /tmp/fluctlight-recovery-1015/browser-client-smoke.mts
python3 /tmp/fluctlight-recovery-1015/envexec.py python3 /tmp/fluctlight-recovery-1015/run-migration-test.py
```

`start-runtime.py` starts only API by default and refuses duplicate live PID. Explicit argument `worker` starts the consumer and may immediately resume existing jobs; use only when the intended bounded model/media/worker verification is ready. To stop a process use its task-local `.pid` file, never broad process-name kills.

Current-source API/Worker builds were attempted once and failed during concurrent persona implementation: `owner_surface_read.go:18 dispatchOwnerPersonaSwitch undefined`, `:25 applyOwnerPersonaSwitch undefined`. Parent must rebuild/restart after that implementation settles; existing binary checks are not final acceptance. No paid global persona backfill was run; startup gates passed for existing binary and task objects.
