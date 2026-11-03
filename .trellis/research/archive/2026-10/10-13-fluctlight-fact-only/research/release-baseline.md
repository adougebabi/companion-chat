# Release baseline (read-only inventory, 2026-10-13)

Inspected checkout HEAD/origin/master: `e36b3be8b2efb91e02fbed2771352f49557a60df`. Local master workflow also retains the same direct-latest publish tail. No secrets/private env values read; no login, push, dispatch, deployment or merge performed.

## Registry, artifacts and platform

- `.github/workflows/fluctlight-ci.yml:195-199,237-241`: existing Docker Hub login uses `DOCKER_HUB_USERNAME` and `DOCKER_HUB_ACCESS_TOKEN`; registry namespace is a secret, thus exact public repository owner is **unknown**, not the GitHub owner by implication.
- `.github/workflows/fluctlight-ci.yml:182-190,223-232`: complete app publication set is two repositories: `${DOCKER_HUB_USERNAME}/fluctlight-core-go` and `${DOCKER_HUB_USERNAME}/fluctlight-web`.
- `.github/workflows/fluctlight-ci.yml:212-213`: integration tags `go-build` and `go-${github.sha}`. Production line 253 publishes only mutable `latest`; no stable immutable release tag/digest capture.
- `.github/workflows/fluctlight-ci.yml:179,221,204-215,246-255`: ubuntu-latest runners, no `platforms` override. Expected current default is linux/amd64; exact existing registry manifests remain unverified. Preserve this behavior/architecture; do not introduce multiarch opportunistically.
- `apps/core-go/Dockerfile:1-20,28-34`: unchanged Go/alpine build (CGO=0, GOOS=linux, no GOARCH override), one Core image embeds API, Worker, migrate, cutover and operator utilities. `apps/web/Dockerfile:1-20`: Node build / nginx runtime. Existing build args/proxies and registry-prefix mechanism can remain intact.

## Workflow trigger / release gaps

- `.github/workflows/fluctlight-ci.yml:3-14`: push/PR targets main/master/codex/go-*, workflow_dispatch exists without inputs; no paths filter. `codex/fact-only-reply-vision` does not currently trigger push CI.
- Lines 23,95,144 disable Go/Web/Compose validation; aggregate validate at 135-139 is a placeholder and does not depend on those checks. Publishing needs only that placeholder (177,219).
- Publish line 220 accepts only push main/master. Dispatch on master currently runs validate but **does not publish**. A master push republishes stable latest; a GitHub rerun of an old eligible push also remains eligible (no freshness check).
- Line 178 accepts only push codex/go-build; fact-only branch has no release lane.
- No explicit workflow/job `permissions`; PR build jobs presently cannot push because of push-only job conditions, and no pull_request_target trigger. Preserve explicit event/ref guards and read-only token scope when adding a release lane; PR should validate only.
- Lines 16-18 concurrency is per ref; main/master runs are not canceled. Distinct main/master refs may both publish same latest, and there is no remote branch-head comparison. Matrix members directly overwrite latest independently; a partial matrix failure can mix releases. No deployment/auto-merge steps were found in this workflow.

Minimum implementation scope: workflow trigger/job gates and a reusable release helper (build unique commit/run/attempt tags for both images; require full matrix success; compare remote authorized branch HEAD with run SHA immediately before alias promotion; serialize alias promotion; promote test lane only to fact-only-latest, keep stable lane latest). Keep Docker Hub credentials, existing Dockerfiles and app image set. Add master dispatch publish eligibility if a stable rebuild entry is required; reruns must still pass freshness guard. A failed promotion of one alias after the other cannot be atomically rolled back by Docker Hub; record both unique image digests/tags so failure is explicit rather than treating partial promotion as success.

## Compose switch and worker isolation

- `infra/compose/fluctlight.compose.yml:105,119,142,182` all use CORE_GO_IMAGE for migrate/cutover/core/worker; line 215 uses WEB_IMAGE. Fallbacks use `${DOCKERHUB_USERNAME:-yourname}/...:latest`.
- `infra/compose/fluctlight.env.example:24-26` explicitly sets both full image refs to `yourname/...:latest`, so changing only DOCKERHUB_USERNAME or an unused tag variable **does not switch images**. Test/stable override must set CORE_GO_IMAGE and WEB_IMAGE, preferably unique references/digests; `config --images` can verify without dumping private env config. Use pull and up --no-build for remote release verification, avoiding existing build definitions.
- `infra/compose/run-platform-smoke.sh:6,19-38,76` and `infra/acceptance/run-go-active-workflow.sh:8,13,34` create disposable project names but use `up --build`, proving local source rather than registry artifacts. Their guards reject prior project resources; no need to weaken those guards.
- `apps/core-go/internal/workflow/workflow.go:33-34,133-138,174,1197-1203`: fixed deployment name `fluctlight`, default build ID `platform-v1`, env TEMPORAL_WORKER_BUILD_ID, UseVersioning true, AutoUpgrade. `apps/core-go/cmd/worker/main.go:121-126` actively sets deployment current version on **every startup**. Different build ID in shared namespace is not sufficient test isolation: it redirects current workflow routing. Use separate namespace and independent Compose/data context; unique build ID should correspond to unique published artifact. Compose exposes namespace at 94,127,160 and worker build ID at 129,191.
- `infra/acceptance/run-go-active-workflow.sh:14-18,94-97` checks returned version deploymentName/buildId. Can reuse as metadata check once registry-image mode is available.

## Migration / backups / unknown production evidence

- `apps/core-go/internal/migrations/runner.go:15`: compiled current head `0060_media_kev`. Its source filename is `apps/core-go/internal/migrations/media_kev.go` (previous head `0059_fact_generation_provenance`, line 3); there is no numbered SQL migration filename for this head. Actual DB ledger is `public.alembic_version.version_num` (runner.go:73,255). Code head is not proof of deployed DB version.
- `infra/backup/recovery-drill.sh:4-19` only validates disposable tmp target then prints manual recovery steps. It does not execute pg_dump, restore, configuration backups or recovery validation. No executable backup/restore implementation found under infra shell scripts.
- Source-known backup requirements from that script: application + temporal + temporal_visibility snapshots, private object bucket with manifest count/SHA256 samples, application schema head, active workflow evidence. Production configuration/private env backup completeness and actual restore readiness are **unknown**; no private env read, backup action or restore performed.
- `gh auth status` and `gh run list` **not executed**: shell reports `gh: command not found`. Thus GitHub runs, current remote stable build and rerun IDs not verified.
- Docker CLI/buildx available (`v0.37.2`); read-only local image inventory failed because OrbStack daemon socket is absent. No local running image/digest obtained.
- Stable registry digest remains **unknown**: secret registry owner not available from allowed files, no authenticated registry lookup and no Docker daemon. Do not fill in a guessed owner/digest or claim production image, DB head, CPU manifest or configuration backup verified.
