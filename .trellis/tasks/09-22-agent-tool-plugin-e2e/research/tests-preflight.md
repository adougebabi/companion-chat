# Agent / Tool / Plugin E2E 测试预检

> 预检日期：2026-09-22。仅做静态检查与运行时可用性探测；未启动服务、未执行测试、未写入业务数据。配置仅记录变量名，不记录任何值或凭证。

## 运行时与依赖锁定

- `apps/core-go/go.mod` 声明 `go 1.25.4`；本机为 `go1.26.3 darwin/arm64`，满足最低工具链要求。
- 根 `package.json` 声明 Node.js `>=24.12.0`、`pnpm@11.0.7`；本机分别为 Node.js `v24.12.0`、pnpm `11.0.7`。
- Docker daemon 可达，服务端版本 `29.4.0`（探测限制为 10 秒）。
- Eino 主模块锁定为 `github.com/cloudwego/eino v0.7.37`。
- 与 Eino/OpenAI 边界直接相关的锁定版本：
  - `github.com/cloudwego/eino-ext/components/model/openai v0.1.13`
  - `github.com/cloudwego/eino-ext/components/embedding/openai v0.0.0-20260916065400-2607f61e807f`
  - `github.com/eino-contrib/jsonschema v1.0.3`
  - 间接依赖 `github.com/cloudwego/eino-ext/libs/acl/openai v0.1.17`

## 已有真实 E2E / smoke 启动命令

| 入口 | 命令 | 边界与用途 |
| --- | --- | --- |
| Persona 分层真实 E2E | `pnpm e2e:persona` | 连接已运行平台，真实登录并走 Provider；脚本明确不 mock 认证、Provider、数据库或 HTTP 层。 |
| Live Provider 有界 smoke | `infra/acceptance/run-go-live-provider-smoke.sh` | 先探测 OpenAI-compatible `/models`，再运行 `apps/core-go/internal/core` 中指定 live Provider 测试。 |
| Live Agent/Tool durable E2E | `FLUCTLIGHT_LIVE_PROVIDER_TEST_REGEX='TestLive(HandleTurnUsesRealProviderForPostCognitionPersonalityAssessment|HandleTurnRealToolCallsReachDurableMediaAndReply|WakeUpRealToolCallsReachDurableActionAndReflection)$' infra/acceptance/run-go-live-provider-smoke.sh` | 复用现有 runner，额外要求独立 PostgreSQL 配置；覆盖真实 Provider 原生 tool call、Eino/ADK、Core capability 执行和持久化。ComfyUI 仍为 Mock。 |
| 全平台 Compose smoke | `infra/compose/run-platform-smoke.sh --clean` | 用一次性 Compose project 启动 PostgreSQL、Redis、MinIO、Temporal、core、worker、web 等，检查健康状态后清理卷和孤儿容器。 |
| Actor chat Compose smoke | `infra/acceptance/run-go-actor-chat-smoke.sh` | 启动一次性全平台栈，真实走 auth/setup、创建角色与会话、发消息，并直接核对 PostgreSQL 持久化。Provider 可选。 |
| Phase 8 contract gate | `infra/acceptance/run-phase8-contract-gates.sh [run_dir] [--allow-blocked]` | 执行 Eino/ADK/provider/tool-loop 合同测试；有独立数据库变量时再提升条件数据库行。 |
| Phase 5 gate | `infra/acceptance/run-phase5-gates.sh [run_dir] [--allow-blocked]` | 执行 provider runtime、queue stream、ADK tool loop 与浏览器边界合同测试并收集 JSONL 证据。 |

## 已确认的测试层次与 Mock 边界

- `eino_adk_runtime_test.go` 广泛使用 `httptest.NewServer` 替代 OpenAI-compatible Provider HTTP 端点，用确定性响应验证 Eino/ADK 编排、结构化响应和工具循环；它不是 live Provider E2E。
- Redis 队列/管道测试使用 `miniredis`，隔离 Redis 网络与持久化；本机无法监听时会 skip。
- PostgreSQL 集成测试通过 `GO_CORE_TEST_DATABASE_URL` 显式启用，未提供时 skip，避免默认连接共享数据库。
- `provider_live_e2e_test.go` 的模型调用是真实 Provider；媒体执行在 `httptest.NewServer` 模拟的 ComfyUI HTTP 边界止步。因此它是“真实模型 + 真实数据库 + Mock 媒体后端”的混合 E2E。
- 上述 live durable 用例覆盖三个生产链：交互 turn 后认知人格切换；交互 turn 同时调用 `media.image.generate` 与 `conversation.reply`；后台 Wake-up 产出 action、media intent 与 reflection intent。它们使用正式 Eino/ADK tool call 和 Core 内置 capability registry，没有 fake router 或脚本化 completion。
- `persona-layers-real.mjs` 明确是“真实登录 + 真实 Provider + 真实数据库 + 真实 HTTP”的平台外部 E2E，不提供自建依赖或自动清理平台数据。
- `run-go-actor-chat-smoke.sh` 允许 Provider 未配置；它验证的是平台、认证、会话及 PostgreSQL 持久化链，模型结果不属于硬性通过条件。
- 仓库当前没有检索到 plugin manifest、插件安装/发现、PluginRegistry、MCP/plugin transport 等生产边界。现有“工具”是静态注册的内置 capability；因此现有真实 E2E 不能证明第三方插件生命周期或动态插件调用。

## 配置变量名称（不含值）

- Persona E2E：`FLUCTLIGHT_E2E_BASE_URL`、`FLUCTLIGHT_E2E_ORIGIN`、`FLUCTLIGHT_E2E_PASSWORD`、`FLUCTLIGHT_E2E_TIMEOUT_MS`、`FLUCTLIGHT_E2E_POLL_MS`、`FLUCTLIGHT_E2E_POLL_REQUEST_TIMEOUT_MS`。
- Live Provider：`FLUCTLIGHT_LIVE_PROVIDER_TEST`、`FLUCTLIGHT_LIVE_PROVIDER_URL`、`FLUCTLIGHT_LIVE_PROVIDER_MODEL`、`FLUCTLIGHT_LIVE_PROVIDER_API_KEY`、`FLUCTLIGHT_LIVE_PROVIDER_PROBE_TIMEOUT_SECONDS`、`FLUCTLIGHT_LIVE_PROVIDER_REQUEST_TIMEOUT_SECONDS`、`FLUCTLIGHT_LIVE_PROVIDER_TEST_REGEX`、`FLUCTLIGHT_LIVE_PROVIDER_TEST_TIMEOUT`。
- 数据库/Compose：`GO_CORE_TEST_DATABASE_URL`、`FLUCTLIGHT_ENV_FILE`、`WEB_HOST_PORT`、`POSTGRES_USER`、`POSTGRES_DB`、`CI`。
- Compose 私有配置与覆盖项：`POSTGRES_PASSWORD`、`FLUCTLIGHT_CORE_SERVICE_KEY`、`FLUCTLIGHT_SETTINGS_KEY`、`FLUCTLIGHT_TRUSTED_ORIGIN`、`S3_ACCESS_KEY`、`S3_SECRET_KEY`、`S3_BUCKET`、`TEMPORAL_NAMESPACE`、`TEMPORAL_WORKER_BUILD_ID`、`REDIS_URL`、`CORE_GO_IMAGE`、`WEB_IMAGE`、`DOCKERHUB_USERNAME`。
- Compose 资源限制：`POSTGRES_MEMORY_LIMIT`、`REDIS_MEMORY_LIMIT`、`MINIO_MEMORY_LIMIT`、`MINIO_INIT_MEMORY_LIMIT`、`TEMPORAL_MEMORY_LIMIT`、`MIGRATE_MEMORY_LIMIT`、`CUTOVER_MEMORY_LIMIT`、`CORE_MEMORY_LIMIT`、`WORKER_MEMORY_LIMIT`、`WEB_MEMORY_LIMIT`。
- Live 初始化素材：`FLUCTLIGHT_LIVE_INITIALIZATION_CARD_PATH`、`FLUCTLIGHT_LIVE_INITIALIZATION_EXPECTATIONS_PATH`。

## 隔离配置缺口与执行前置条件

- Compose smoke 脚本通过唯一 `--project-name` 隔离容器、network 和命名 volume，使用动态 Web host port，并在退出时执行 `down -v --remove-orphans`；这一层已有可回收隔离。Compose 文件自身声明固定 `name: fluctlight`，所以绕过脚本直接执行 `docker compose -f ... up` 时不具备同等的一次性 project 保障。
- 两个 Compose runner 默认读取不同的 env 文件名：平台 smoke 默认 `infra/compose/fluctlight.env`，actor smoke 默认 `infra/compose/fluctlight.local.env`。两者均可由 `FLUCTLIGHT_ENV_FILE` 覆盖，但当前没有统一 E2E env 模板/校验入口，容易出现“同一测试栈、不同配置源”。
- `isolatedCoreTestRepository` 会为每个 Go 集成测试创建 `lac_core_<digest>` 临时数据库、应用迁移，并在 cleanup 中 drop；数据库隔离本身完整。前置 PostgreSQL 账号必须能连接管理库并具备 `CREATE DATABASE`/`DROP DATABASE` 权限，现有 runner 不负责供应这个 PostgreSQL。
- Live Agent/Tool durable E2E 默认不在 `run-go-live-provider-smoke.sh` 的测试正则内，必须显式设置 `FLUCTLIGHT_LIVE_PROVIDER_TEST_REGEX`，并同时提供 `GO_CORE_TEST_DATABASE_URL`；否则只跑轻量 live Provider 识别/结构测试或 durable 用例直接 skip。
- Live Go E2E 共享外部 Provider，尚无每次运行独立的 Provider namespace、配额或确定性 seed；结果受模型版本、服务可用性、速率限制和非确定性影响。
- Live Go E2E 在 ComfyUI 前使用本地 `httptest` 返回预期失败，只证明 prompt 已提交到媒体边界；它不证明插件/媒体后端完成、资产落盘与回传。全真实媒体链仅由 Persona E2E 覆盖。
- `pnpm e2e:persona` 只连接一个已运行的平台。它会创建多个 Fluctlight、会话、消息、图片、记忆、日程、Wake-up、反思与 Moment，脚本末尾只输出报告，没有 teardown，也没有 run-scoped tenant/database。重复运行会在目标环境保留数据。
- 根脚本没有“一条命令启动隔离栈 → 注入真实 Provider/媒体配置 → 执行 Persona E2E → 清理”的编排；目前需要人工先启动平台并确保登录密码、Provider、worker、Temporal、S3/MinIO 和图片后端均可用。
- 现有 Phase 8 gate 验证 agent/tool 合同与 Eino/ADK 数据流，但主要使用 `httptest`/测试模型；它应作为快门禁，不能替代 live Agent/Tool E2E。
- 范围校正：本任务明确采用静态显式注册与依赖注入，不要求动态发现、安装、卸载或 MCP transport。缺少这些能力不是验收缺口；真正缺口是全部业务 Tool 独立执行及全部正式 Agent 使用同一原生循环的真实证据。

## 建议的最小分层门禁

1. 快速合同层：运行 Phase 8 gate，覆盖 Eino/ADK、tool schema、调用身份、循环上限、持久化合同。
2. 混合 live 层：以显式 live durable 正则运行现有 Provider E2E，使用每用例临时 PostgreSQL；接受 ComfyUI 为 Mock，并在报告中标记该边界。
3. 全平台层：用唯一 Compose project 启动完整基础设施，再运行 Persona E2E，完成后销毁 project；在新增测试清理机制前，只允许指向专用一次性环境。
4. 独立插件边界：复用正式静态注册、参数解析和执行实现，覆盖全部 Tool 直接调用及原生适配调用；无需新增动态插件平台。

## 主线程实机复核

命令与退出码见 preflight-results.json，配置只读存在性见 config-presence.json。当前 shell 未设置 live Provider/测试数据库变量，但 `.env` 存在非空 MTPLX_API_KEY、MTPLX_MODEL、COMFYUI_URL，`infra/compose/fluctlight.local.env` 存在基础设施配置；默认 `fluctlight.env` 不存在。实施时优先安全复用这些配置，不因 shell 变量缺失就判定服务不可用。本轮未验证网络业务连通、真实模型或媒体成功，不宣称 E2E 通过。
