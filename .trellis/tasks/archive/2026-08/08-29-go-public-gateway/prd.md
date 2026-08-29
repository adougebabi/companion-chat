# Go Public Gateway 第一阶段

## Goal

建立一个资源轻量、可独立运行的 Go Public Gateway 基础进程，作为后续逐步替换 Node BFF 的兼容入口。第一阶段只验证进程、配置、健康检查和 Core readiness/ping 边界，不接管领域数据，也不改变现有浏览器契约或生产流量。

## Requirements

- 使用 Go 标准库实现可执行入口，避免第一阶段引入 Web 框架和额外运行时依赖。
- 从环境变量读取监听地址、Core 内部地址和 Core service key，并在启动时拒绝缺失或非法配置。
- 提供 `GET /health/live`，只反映 Go Gateway 进程存活。
- 提供 `GET /health/ready`，使用 Core readiness 检查确认下游可用；Core 不可用时返回 503。
- 提供 `GET /api/platform/ping`，向 Core 的 `/internal/platform/ping` 发送 service key，并将状态码和 JSON 响应传回浏览器边界。
- 对外只暴露上述三个路径；其它路径返回 404，不把 Core 的 `/internal/*` 直接暴露给调用方。
- 第一阶段不访问 PostgreSQL、Redis、S3 或 Temporal，不实现 session、CSRF、DTO 转换、NDJSON 脱敏或领域写入。
- 保留 Node BFF 作为当前生产入口；Go Gateway 通过独立端口运行，具备可回滚的旁路验证条件。

## Acceptance Criteria

- [ ] `go test ./...` 通过，覆盖配置校验、live、readiness 成功/失败、ping service key 转发和未知路由。
- [ ] `go vet ./...` 通过。
- [ ] `go build ./...` 通过并生成 Gateway 可执行文件。
- [ ] 使用可控的 HTTP fake Core 可以验证 readiness/ping，不需要真实数据库或中间件。
- [ ] 运行进程不启动任何后台 goroutine、定时器、数据库连接或队列消费者。
- [ ] README 或 Go 模块文档说明第一阶段是旁路 gateway，不得直接切换生产流量。

## Constraints

- 不修改 Python Core、Node BFF、Vue 浏览器 API 或数据库 schema。
- 不与 Python Core 双写任何数据。
- 不在本阶段引入反向代理切流、生产 Compose 服务或 Kubernetes 配置。
