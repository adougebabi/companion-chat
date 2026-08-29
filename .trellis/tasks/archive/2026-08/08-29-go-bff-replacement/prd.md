# Go BFF 完整替换

## Goal

在保留 Python Core 唯一领域写入者的前提下，用 Go 重新实现当前 Node BFF 的完整浏览器边界。Go 版本必须在路由、认证、Cookie/CSRF/CORS、请求映射、响应 DTO、错误码、NDJSON 流和媒体 Range 语义上与现有 BFF 兼容，完成后才允许切换 BFF 生产入口。

## Requirements

- 保留当前浏览器公开 API 的全部路径和 HTTP 方法（含 `OPTIONS /*`），不得遗漏认证、Provider、Fluctlight、Conversation、Memory、Relationship、Life World、Moments、Diagnostics、Workflow 和 Media 路由。
- 保留 `fluctlight_session` opaque HttpOnly session cookie 和 `fluctlight_csrf` double-submit cookie 的属性、生成、清除和自动补发行为。
- 保留 trusted Origin 精确匹配、constant-time CSRF 比较、credential CORS 和预检响应。
- Go BFF 只能调用 Core HTTP API，所有请求带 Core service key；有 session 时额外带 human session；不得访问数据库、Redis、S3、Temporal 或领域内部实现。
- 保留浏览器 camelCase 与 Core snake_case 的逐路由请求映射、Conversation/Diagnostics/Provider 专用响应 DTO 映射和 204 行为。
- 保留现有稳定错误状态、code、message 及 diagnostics/creation/media/turn 的特殊错误映射。
- 对 conversation turn 实现可取消的 Core 请求和增量 NDJSON 转换：UTF-8/行边界、sequence、turn id、hidden payload、action_result、heartbeat、terminal 和 abort 语义必须与 Node BFF 一致。
- 对媒体实现授权后的流式 Range 代理，保留允许的响应头和非 2xx 到 `media_unavailable` 的映射，不缓冲完整媒体 body。
- 当前 Node BFF 保持为回滚实现；Go BFF 通过独立启动命令和可选 Compose profile 验证，完成 parity 后才切换默认入口。

## Acceptance Criteria

- [ ] Go BFF 路由清单覆盖现有 Browser OpenAPI 的全部 operation，路径和 method 无差异。
- [ ] 每个路由至少有成功、缺 session/CSRF（适用时）、Core 失败三类 `httptest` 覆盖；所有 camelCase/snake_case 和默认 query 值有请求快照测试。
- [ ] Cookie 属性、CORS、OPTIONS、Origin/CSRF 矩阵和 service/human 双身份测试通过。
- [ ] NDJSON parity 测试覆盖 chunk/UTF-8、未知事件、hidden key、sequence/turn id、action_result/message/media、heartbeat、terminal、incomplete、abort 和 no-later-write。
- [ ] Media parity 测试覆盖 200/206、Range、ETag/MIME/Content-*、Core failure、session 缺失和流式转发。
- [ ] `go test ./...`、`go vet ./...`、`go build ./...` 和现有 Web typecheck/test/build 通过。
- [ ] 生产 Compose 切换前，使用 fake Core 以及可用的 disposable Core/BFF smoke 验证所有关键路径；失败时仍可回退 Node BFF。

## Constraints

- 不改变 Python Core API、PostgreSQL schema、Redis/Temporal 语义或浏览器客户端 wire contract。
- 不让 Go 与 Python 双写领域数据；Go 只做浏览器 transport/session/DTO/media proxy。
- 不使用全局 snake_case→camelCase 猜测替代逐路由映射，不解析 Core 错误文本决定业务分支。
