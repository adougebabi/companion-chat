# 摇光项目第四阶段：移除独立 BFF，合并浏览器接入到 API

## Goal

在前三阶段已验收的 Go Core/Eino/ADK/Worker 基线上，删除独立 `apps/gateway-go` 进程、镜像和内部 HTTP 转发；由现有 Go API 进程直接提供当前浏览器公共接口。浏览器契约、认证授权、Session/CSRF、DTO、错误映射、NDJSON 和安全媒体代理必须保持有效，领域 App、事务、Outbox、Temporal、Worker 和前三阶段 Agent/Task 语义不变。

## Requirements

1. 以当前真实代码为准完成 BFF 路由、调用方、Core handler/App service、认证职责、输出协议和部署引用盘点；记录公共、管理、内部、运维路由分类和无调用方的旧转发边界。
2. 在 `apps/core-go` HTTP 传输层建立唯一浏览器接入边界：同源/跨域策略、Cookie、CSRF、Origin、请求校验、camelCase DTO、错误脱敏、NDJSON 翻译和媒体 Range/ETag/MIME 白名单均留在传输层。
3. 公共浏览器 Handler 直接调用明确的 Core App/Repository 服务接口；不得通过 localhost、自身 HTTP、伪造 Request/ResponseWriter 或复制另一个 Handler 维持转发。
4. 保留当前 `/auth/*`、`/api/*`、`/health/*` 的方法、路径、字段、状态码、错误码、空值/分页/时间/标识和当前消息输出协议；更新前端统一 BrowserClient、媒体 URL、Vite proxy、runtime 配置和生成 client/OpenAPI（若路由合同本身未变则只更新其来源/校验）。
5. 认证主体只能来自 Core 验证的 opaque Session；浏览器提交的 actor/user/instance/session 字段不能建立权限。保留 Argon2id、Session hash/过期/撤销、Secure/HttpOnly/SameSite Cookie、CSRF 双提交、精确 Trusted Origin、内部 service identity 与 Human Session 分离。
6. 完整迁移 Compose、Docker、CI、开发脚本、健康检查、E2E/验收脚本和文档；最终部署不构建、不启动、不依赖 BFF，API 与 Worker 仍分离，Temporal 不改造。
7. 删除独立 BFF 源码、入口、Go module、Dockerfile、Core HTTP client、转译链和仅用于旧 BFF 的配置/依赖/兼容分支；保留真实内部接口、共享媒体存储和历史契约证据。
8. 使用真实 API Router、浏览器边界和 Fake Provider/模型完成路由、认证/授权、CSRF/CORS、DTO/错误、对话 NDJSON、断连取消、媒体代理、部署和前三阶段回归验证；真实 PostgreSQL/Temporal/Provider 条件缺失时明确记录，不用 curl 冒充浏览器验收。

## Constraints / Out of Scope

- 不移除或重写 Temporal、Worker、Eino、ADK、Prompt、Task、人格接管、后台主动性、领域事务、Outbox、幂等、消息顺序或前端视觉/状态管理。
- 不把所有 `/internal/*` 路由公开；管理、服务、调试和运维边界必须继续受保护或仅内部可达。
- 不新增 JWT/SSO/认证微服务、API Gateway、SSE/WebSocket/Token 直出、上传/OAuth/第三方客户端等不存在的能力。
- 不修改生产数据、Session、资源地址或无关依赖；不保留旧 BFF 运行回退、双 BaseURL 自动重试或 feature flag。

## Acceptance Criteria

- [x] 迁移表覆盖当前所有 public route、真实 Web/脚本/测试调用方、认证/权限、DTO、下游 App service、目标 public Handler 和验证方式。
- [x] Go API 进程直接提供所有已存在浏览器公共路径；源码中无 BFF→Core HTTP client、localhost 自请求、伪 HTTP Handler 转发或第二套运行入口。
- [x] 登录/setup/session/logout/revoke/password、失效 Session 清理、CSRF/Origin/CORS、跨资源授权和 actor spoof 防护由真实公共 Router/领域授权测试覆盖。
- [x] 主对话经过既有 App.StreamTurn/ADK/settlement，NDJSON 事件、断连取消、终端唯一性和单次正式发布与迁移前契约保持；工具/Judge/内部字段不泄漏。
- [x] 所有浏览器业务接口、诊断/workflow、媒体 Range/授权和生成 client/client tests 通过；嵌套用户 JSON/Tool Schema 未被递归改 key。
- [x] Web、开发代理、Compose、CI、Docker、健康检查、验收脚本和文档不再依赖 `apps/gateway-go`、BFF 镜像或 `BFF_*`/`CORE_BASE_URL` 配置。
- [x] Core、Web、browser-client、API/contract/acceptance checks、race/vet/format/build 和前三阶段 Go 回归均有实际结果；真实浏览器/Compose/DB/Provider 未验证项已单列。
