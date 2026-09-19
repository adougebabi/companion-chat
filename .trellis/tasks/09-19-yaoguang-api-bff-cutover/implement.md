# 第四阶段实施计划：移除独立 BFF并合并浏览器接入

## Phase A — Baseline, inventory and planning gate

- [x] 读取最新阶段需求、AGENTS.md、Trellis workflow/backend/frontend specs。
- [x] 阅读阶段一 Eino 基础、阶段二 ADK 对话、阶段三后台 ADK 的 PRD/design/implement/migration，确认不返工 Agent/Task/Temporal。
- [x] 记录当前分支/worktree、BFF/Core/Web/Compose/CI 基线和已有测试状态。
- [x] 完成跨目录路由/调用方/认证/部署盘点，保留 file:line 证据。
- [x] 任务 planning artifacts review 后执行 `task.py start`。

## Phase B — Public browser boundary inside Core API

- [x] 将 BFF transport-only 组件迁入 `apps/core-go/internal/httpapi/browser`，保留浏览器契约、Cookie/CSRF/CORS、校验、DTO、错误脱敏、NDJSON 和媒体 proxy 行为。
- [x] 定义无 URL/HTTP client 的 `browser.Backend`，在 `httpapi` 中按 operation 直接调用 `core.App`/`core.Repository`；完成 OpenAPI 全量 public route 的 direct mapping。
- [x] 把浏览器 handler 注册到现有 API `Server.Handler`；保留 `/internal/*` 的 service-key/human-session 边界和 `/health/*`。
- [x] 接通 session 401 清理、login/setup token non-empty invariant、trusted-origin startup validation、constant-time service identity。
- [x] 对话走 App.StreamTurn + in-process NDJSON writer adapter；媒体走 App.AuthorizeAsset + ServeMedia；不构造伪 HTTP 请求或捕获另一个 Handler。

## Phase C — Client and deployment cutover

- [x] 更新 BrowserClient/runtime config、Vite proxy、统一媒体 URL，修复视觉身份页面相对 media URL。
- [x] 更新 Compose/env/Docker/Nginx（仅基础 proxy）、CI image/test matrix、root scripts、acceptance/E2E/health checks。
- [x] 删除 `apps/gateway-go` 全目录、gateway module/Dockerfile/entrypoint、Core HTTP client、BFF-only dependencies/config and stale references。
- [x] 更新 OpenAPI/browser-client artifacts、README、MinIO/media and architecture/migration docs；记录 route/responsibility matrix。

## Phase D — Verification and cleanup

- [x] 新增/迁移 API Router httptest：route parity、unknown route、auth/session/CSRF/CORS/actor spoof/cross-resource/error details。
- [x] 新增/迁移 NDJSON/media/abort tests：split frames, sequence/terminal uniqueness, hidden payload redaction, range/etag/mime, nil body, disconnect cancellation。
- [x] 运行 Core/Web/browser-client tests, race, vet, gofmt, pnpm typecheck/test/build, OpenAPI drift and deployment-config validation。
- [x] 检查登录→读取→对话→消费输出→退出的浏览器验证入口；本机未提供可运行的浏览器/完整 Compose/DB/Provider 条件，已在 migration.md 和最终报告单列未验证，不以 curl 代替。
- [x] 运行前三阶段 Core/ADK/WakeUp/Task/Temporal/Worker 所属 Go 回归和旧 BFF/gateway/自转发静态扫描；真实外部依赖缺失项单列。
- [x] 通过 `trellis-check`，更新相关 backend/frontend specs，提交 work commit；随后 archive task + journal。

## Review gates and rollback points

1. Public route inventory and Backend mapping complete before deleting gateway.
2. Core API public router + auth/NDJSON/media tests green before client/deployment cleanup.
3. Client and Compose smoke green before removing `apps/gateway-go` and CI matrix.
4. Full regression green before final commit. If any gate fails, revert only the current worktree changes or return to the last version-level deployment; never add a permanent BFF fallback.

## Required commands (adjust to repository availability)

```bash
go -C apps/core-go test -mod=readonly ./...
go -C apps/core-go test -mod=readonly -race ./internal/httpapi ./internal/core
go -C apps/core-go vet ./...
gofmt -l apps/core-go
pnpm --filter @fluctlight/browser-client test
pnpm --filter @fluctlight/web typecheck
pnpm --filter @fluctlight/web build
pnpm --filter @fluctlight/web test
git diff --check
rg -n 'apps/gateway-go|fluctlight-bff|BFF_|CORE_BASE_URL|VITE_DEV_BFF|bffOrigin' --glob '!**/.git/**'
```
