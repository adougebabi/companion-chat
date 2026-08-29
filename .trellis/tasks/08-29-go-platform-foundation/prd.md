# Go 平台基础模块第二阶段

## Goal

建立 Go 运行时共用的平台边界，并让当前 Go BFF 使用这一边界。第二阶段只抽取健康状态、运行角色和依赖 readiness 探针，不迁移领域逻辑、不访问数据库、不改变浏览器 API。

## Requirements

- `internal/platform` 提供受限的运行角色常量：`bff`、`core`、`worker`。
- `internal/platform` 提供稳定的健康响应结构和 live/ready 构造函数，避免各服务手写不一致的 role/status JSON。
- readiness 通过显式的 `Probe` 接口执行下游依赖检查；probe 错误必须映射为 unavailable，不得泄露底层错误给浏览器。
- Go BFF 的 `/health/live` 和 `/health/ready` 使用平台模块；live 不调用 Core，ready 只调用 Core readiness probe。
- 模块保持标准库-only、无全局状态、无数据库/Redis/Temporal 依赖。

## Acceptance Criteria

- [ ] `internal/platform` 单元测试覆盖角色、live/ready 响应和 probe 成功/失败。
- [ ] Go BFF 的健康路由回归测试继续通过，并证明 live 不触发 Core 请求。
- [ ] `go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./...` 通过。
- [ ] 第二阶段没有改变任意 `/api/*`、`/auth/*`、NDJSON 或 media 行为。

## Constraints

- 不修改 Python Core、Node BFF、数据库 schema 或生产中间件配置。
- 不新增未被实际调用的领域 scaffolding；平台模块必须被当前 Go BFF 使用。
