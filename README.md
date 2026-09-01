# Fluctlight 系统

Fluctlight 是一个面向“持续存在的 AI 人格实例”的运行平台。每个
Fluctlight 实例都有自己的名称、身份、人格、情绪与内部状态，并能够在持续
对话、记忆、关系、生活上下文和受约束的自主行动中保持连续性。

它的目标不是提供一个泛用聊天窗口，而是让一个 AI 人格能够被创建、使用、
观察、治理和逐步演进。系统强调事实可追溯、状态可恢复、能力可替换，以及
所有者对身份、记忆、自治和敏感配置的明确控制。

> 当前仓库采用 clean-start 运行主线，仍在持续演进中。本文档描述当前实际
> 代码和 Compose 拓扑；未在 Web 页面接入的底层接口不会被表述为已完成的
> 用户功能。

## 一眼看懂

```text
浏览器
  │
  ▼
Vue Web（Vite + Pinia，生产环境由 Nginx 提供静态文件）
  │  浏览器会话、CSRF、camelCase DTO、NDJSON 流
  ▼
Go BFF（唯一浏览器公网边界）
  │  内部 service key / human session
  ▼
Go Core API（唯一领域写入者）
  ├── PostgreSQL + pgvector   领域事实、修订、审计、outbox
  ├── Redis Streams           事件投递与 durable consumer groups
  ├── Temporal                可恢复的生命周期、交互、媒体工作流
  ├── MinIO / S3              私有媒体对象与版本化存储
  └── 外部 Provider/服务       OpenAI-compatible 模型、Embedding、ComfyUI
          ▲
          │
Go Worker（Temporal poller、intent dispatcher、outbox publisher）
```

核心边界只有一个原则：浏览器不直接访问 Core、数据库、Redis、Temporal 或
对象存储；BFF 不承载领域规则；所有领域事实和写入都由 Go Core 统一完成。

一次交互大致经过以下路径：用户消息先以幂等方式写入 PostgreSQL，Core 生成
认知上下文并调用 `cognitive_assessment`，冻结可执行决策后再执行能力调用，
最后通过 `action_realization` 生成可见回复并写回对话。需要跨进程、重试或长
时间运行的工作会先记录为 durable intent，再由 Worker 投递给 Temporal 执行。

## 当前能力

### Web 已提供的用户能力

- **Owner 认证与会话**：一次性 setup token、密码登录、退出、修改密码、撤销
  全部会话，以及 HttpOnly 会话 Cookie 和 CSRF 保护。
- **Fluctlight 实例目录**：查看实例状态、未读数和最近对话；按自定义分组筛选；
  创建分组并管理成员。
- **实例创建**：支持“白纸创建”，也支持提交自然语言描述，由初始化模型生成
  Foundation 预览；确认后才会激活实例，并保留 provenance、初始目标和意图。
- **直接对话**：加载实例的直接会话，接收 NDJSON 流式 token、消息、媒体、完成
  和错误事件；支持取消、失败重试、历史分页、发送顺序保护和已读位置上报。
- **动态（Moments）**：浏览全部或单个实例的动态，查看图片、视频和音频；可
  发表评论、回应、隐藏或恢复动态。
- **实例治理**：查看身份、人格、表达策略、内部状态、目标、意图、生活上下文、
  关系和记忆；暂停或恢复自治；维护 Presence、生活事件和日程；修正、接受、
  拒绝或回滚 Foundation 与演进修订；归档或删除实例。
- **模型与运行设置**：配置初始化、认知判断、回复生成、反思、Embedding 和
  媒体提示词等 Provider 角色；管理 Endpoint、模型列表、ComfyUI 图片生成服务、
  自治开关、定期唤醒周期和诊断保留策略。
- **诊断与工作流控制**：查看脱敏系统事件和模型运行记录，按 correlation ID
  追踪一次操作；查询工作流状态与历史，并执行暂停、恢复、取消、重启和 Reset。

### Core 已具备的运行能力

以下是 Core/Worker 当前已有的底层运行能力，其中部分能力尚未在 Web 中提供
独立的操作入口。

- **持续人格模型**：身份、人格、行为策略、生活档案、内部状态、目标、意图、
  关系、记忆、任意 Drive/Preference typed slots 和 Foundation revision 都由
  PostgreSQL 持久化，并支持版本、证据和审计追踪。
- **认知运行时**：将感知、评估、状态更新、决策、能力执行、回复实现和反思拆成
  明确阶段；模型负责语义判断，服务器负责权限、数值边界、幂等和最终提交。
- **记忆与检索**：支持工作记忆、情景记忆、语义记忆、关系记忆和自传记忆；检索
  以授权为前提，当前以全文/词法检索为主，并为 pgvector 和异步 Embedding 保留
  扩展位置。
- **自治与生活世界**：围绕 Goal、Intention、Schedule、Event、Presence 和每日
  Review 组织自主行为；自治动作可暂停、取消、重试，并受预算和治理策略约束。
- **能力注册与安全执行**：能力通过 manifest 声明参数、side effect、并发、取消、
  重试和 preflight 属性；Provider 只能看到工具契约，不能直接访问领域表。若模型
  发现缺少能力，可以调用 `capability.request` 写入 Owner 可审核的全局需求池，
  插件由人工接入现有 `CapabilityExecutor` slot。
- **媒体流水线**：通过 ComfyUI 生成媒体，轮询外部任务并把图片、视频或音频写入
  私有 MinIO/S3；媒体带有校验信息、版本和引用关系，浏览器只能通过 BFF 代理读取。
- **可靠异步执行**：PostgreSQL outbox 负责事务内记录事件，Worker 发布到 Redis
  Streams；Temporal 负责生命周期、交互和媒体工作流的重试、暂停、恢复、取消和
  版本化执行。
- **可观测与可恢复**：模型调用、Provider provenance、工作流管理和关键领域变更
  都可以关联到 correlation ID；失败不会替代或删除已落库的领域事实。

### 自我意识的双循环

当前 Core 将摇光的主动性拆成两个相互衔接的循环：

1. **外部/内部唤醒循环**：`Event / Internal State → Trigger → Wake-up → Attention → Thought → Desire → Agency → Action → Experience`。除了对话和生活事件外，每个激活的 Fluctlight 都会启动一个长期存活的 `wake_up.current` Temporal workflow。默认每 30 分钟执行一次，模型在唤醒时读取人格、内部状态、日程、关系、记忆和最近对话，产出结构化的 `attention`、`thought`、`desire`、`agency`；动作通过已安装的 Capability manifest、硬安全和稳定执行边界冻结，交付仍由 Worker 负责。
2. **人格成长循环**：`Experience → Reflection → Self Model → Drive / Preference → 下一次 Attention`。每次唤醒都会写入带 sequence 和 provenance 的 `internal.wake_up` cognition fact，并创建 `reflection.run` intent，沿用现有 Reflection 的证据窗口、水位和 CAS 约束，允许模型在有足够证据时提出 self-model/personality 演进，以及任意 typed Drive/Preference slot 和未来 Trigger 偏好。

唤醒周期可在 Web「运行策略」中通过 `product.wakeup` 调整：

```json
{ "enabled": true, "interval_seconds": 1800 }
```

唤醒记录保存在 `cognition_wakeups`，可随 Core 实例详情读取，并在 Web 治理页查看
近期摘要；它们是私有认知事实，不会自动变成可见消息。外显联系、动态
或媒体仍必须经过自治模式、允许动作和已有冻结/治理边界；暂停自治只阻止新的外显
动作，不会让内部唤醒和反思停止。

当摇光判断当前能力不足时，会调用 `capability.request`，将需求及其经历证据写入
全局需求池。Owner 可以在治理页审核、拒绝或接受需求；真正的插件接入仍由人工完成，
通过 `CapabilityExecutor`/`CapabilityManifest` 注册后再标记对应需求为 `fulfilled`。

## 架构分层

### 1. 产品层：Web

`apps/web` 是 Vue 3 + Vite + Pinia 的静态浏览器应用。它负责页面、交互、响应式
布局、流式消息展示和 Control Center，不直接连接任何基础设施。生产镜像使用
Nginx 提供静态资源，并在启动时写入 `/runtime-config.js`，因此更换浏览器可达的
BFF 地址通常只需重建或重启 Web 容器。

### 2. 边界层：Go BFF

`apps/gateway-go` 是唯一的浏览器公网入口，负责：

- Owner session Cookie、Origin 和 CSRF 校验；
- 将浏览器 camelCase DTO 转换为 Core 的 snake_case 请求；
- 收敛 Core 错误，避免把数据库、Provider 或内部堆栈泄露给浏览器；
- 转译对话的 `application/x-ndjson` 流；
- 代理受授权的私有媒体和 HTTP Range 响应。

BFF 只通过 HTTP 调用 Core，不访问 PostgreSQL、Redis、S3 或 Temporal，也不复制
领域规则。

### 3. 领域层：Go Core API

`apps/core-go` 是当前唯一的 Core/Worker 运行时。`internal/core` 组合领域模型、
Repository、Provider、Capability Registry、媒体服务、诊断和工作流意图；
`internal/httpapi` 只负责内部 HTTP 路由、认证边界和响应编码。

Core 以 PostgreSQL 中的领域事实为权威来源。Temporal 是 intent 的执行器而不是
事实来源；Redis 是事件投递通道而不是领域状态库；MinIO/S3 只保存媒体对象。
这种分离使同步请求、异步工作流、重试和故障恢复可以共享同一套事实与审计语义。

### 4. 异步层：Worker、Temporal 与 Redis

Go Worker 注册三个规范队列：`interaction`、`lifecycle` 和 `media`。它同时负责
读取待处理 intent、启动 Temporal workflow、发布 PostgreSQL outbox 事件，并运行
durable consumer groups。Temporal Worker 使用 `TEMPORAL_WORKER_BUILD_ID` 和
Worker Deployment 版本，Compose 的 `cutover` 服务会在新 Worker 接管前处理旧运行
时的执行围栏。

### 5. 契约层：OpenAPI 与生成客户端

`packages/browser-client` 保存浏览器契约与生成的 TypeScript 客户端，
`packages/core-client` 保存 Core 内部契约与参考客户端。Core 使用 snake_case，
浏览器使用 camelCase；两者不能混用。修改接口时需要同步 OpenAPI artifact、生成
客户端、BFF route inventory、契约/Parity 测试和 Web store，否则 CI 或运行时检查会
拒绝不一致的边界。

## 目录结构

| 目录 | 职责 |
| --- | --- |
| `apps/core-go/` | Go Core API、领域应用层、PostgreSQL Repository、Provider、媒体与诊断；同时包含 Temporal Worker、迁移、初始化 token 和 cutover 入口。 |
| `apps/gateway-go/` | 唯一浏览器公网 BFF：认证、浏览器路由、DTO/错误转换、NDJSON 和媒体代理。 |
| `apps/web/` | Vue 3/Vite/Pinia 产品 UI 与 Control Center；构建产物由 Nginx 提供。 |
| `packages/browser-client/` | 浏览器 OpenAPI artifact、生成脚本、TypeScript 客户端及客户端测试。 |
| `packages/core-client/` | Core 内部 OpenAPI artifact、生成脚本和参考 TypeScript 客户端。 |
| `infra/compose/` | 完整平台的 Docker Compose 拓扑、环境变量示例和整栈 smoke 入口。 |
| `infra/acceptance/` | Compose bind mount、OpenAPI、路由边界、迁移和平台验收脚本。 |
| `infra/backup/` | PostgreSQL、Temporal、MinIO/S3 和部署环境的备份清单、校验与恢复演练。 |
| `infra/minio/`、`infra/postgres/`、`infra/redis/` | MinIO bucket、Temporal 数据库初始化 SQL 和 Redis 配置等部署默认值。 |
| `.trellis/` | 项目开发流程、领域/分层规范、任务规划和工作记录，不参与运行时部署。 |

## 本地开发与检查

### 依赖与 workspace 检查

项目使用 Node `>=24.12.0`、pnpm `11.0.7` 和 Go 模块。常用检查如下：

```bash
pnpm install --frozen-lockfile
pnpm generate
pnpm typecheck
pnpm test
pnpm build

GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" \
  go -C apps/core-go test -race ./...
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" \
  go -C apps/core-go vet ./...
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" \
  go -C apps/core-go build ./...
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" \
  go -C apps/gateway-go test ./...
```

其中 `pnpm generate` 会依次更新 Core Client、Browser OpenAPI 和 Browser Client。
生成文件头部标记为不可手工修改；接口变更应修改生成源和脚本，再运行生成命令。

### Compose 整栈运行

1. 复制环境示例到未跟踪的私有文件，并填写所有 `REQUIRED` 项：

   ```bash
   cp infra/compose/fluctlight.env.example infra/compose/fluctlight.env
   ```

2. 使用私有环境文件启动完整平台：

   ```bash
   docker compose \
     --env-file infra/compose/fluctlight.env \
     -f infra/compose/fluctlight.compose.yml \
     up --build --detach --wait
   ```

   默认浏览器地址为 `http://localhost:13001`，BFF 地址为
   `http://localhost:13000`。启动顺序由健康检查和依赖关系保护：PostgreSQL、
   Redis、MinIO、Temporal 就绪后，先执行 `migrate`，再执行 `minio-init` 和
   `cutover`，随后 Worker、Core、BFF 和 Web 才会进入可用状态。

3. 首次安装完成后生成 Owner setup token，并在 Web 的 `/auth/setup` 页面使用：

   ```bash
   docker compose \
     --env-file infra/compose/fluctlight.env \
     -f infra/compose/fluctlight.compose.yml \
     run --rm --no-deps migrate \
     /usr/local/bin/fluctlight-setup-token-go --expires-minutes 30
   ```

4. 运行可丢弃的整栈验收。脚本会自动创建独立 Compose project，检查所有服务和
   one-shot 任务，结束后清理容器、网络和 volume：

   ```bash
   FLUCTLIGHT_ENV_FILE=infra/compose/fluctlight.env \
     ./infra/compose/run-platform-smoke.sh
   ```

### 部署配置要点

- `FLUCTLIGHT_TRUSTED_ORIGIN` 是用户实际打开的 Web URL，必须与浏览器地址完全
  一致；`FLUCTLIGHT_BFF_ORIGIN` 是浏览器实际可访问的 BFF URL。
- `CORE_BASE_URL` 是 BFF 到 Core 的容器内部地址，Compose 中通常保持
  `http://core:8080`。不要把容器 hostname 当作浏览器地址。
- `FLUCTLIGHT_CORE_SERVICE_KEY` 和 `FLUCTLIGHT_SETTINGS_KEY` 必须使用私有随机值。
- `POSTGRES_PASSWORD`、`S3_SECRET_KEY` 会被插入连接 URL，建议使用 URL-safe 的
  随机值（推荐 hex）；避免 `@`、`:`、`/`、`?`、`#`、`%` 等字符造成解析错误。
- PostgreSQL 和 Redis 使用严格 bind mount。启动前可运行：

  ```bash
  ./infra/acceptance/check-compose-bind-sources.sh
  ```

- NAS 或其他机器部署时，两个公网 origin 都应填写浏览器所在网络真正能访问的
  地址，不要使用另一台机器上的 `127.0.0.1`。

## 备份与恢复

`infra/backup/` 使用一个操作员 manifest 统筹备份，而不是只备份某一个容器：

- PostgreSQL 应同时覆盖应用数据库和 Temporal 的 `temporal`、
  `temporal_visibility` 数据库；
- MinIO/S3 记录私有 bucket 的对象 key、版本 ID、字节数和抽样 SHA-256；
- 部署环境文件只记录字段是否存在，不把凭据或 `FLUCTLIGHT_SETTINGS_KEY` 明文写入
  manifest。

快照准备好后创建并校验 manifest：

```bash
fluctlight-backup manifest /backup/fluctlight/manifest.json
fluctlight-backup verify /backup/fluctlight/manifest.json
```

恢复前先落到可丢弃的 PostgreSQL、MinIO 和 Temporal volume，执行发布版本的 Go
迁移并启动 Worker，再将服务切换到恢复后的数据。若丢失
`FLUCTLIGHT_SETTINGS_KEY`，保留数据库和对象数据，通过 Owner Settings 重新录入
Provider secrets；不要尝试从旧数据或旧环境变量中解密、复制或重建密钥。诊断数据
清理和 tombstoned media 清理都必须先经过引用失效、校验和版本确认，不能删除领域
修订、证据或治理历史。

## CI 与镜像发布

每个 Pull Request 和受支持分支的 push 都会执行 Go 测试、vet、构建、生成客户端
一致性、路由/OpenAPI 检查、浏览器检查和 Docker 镜像构建。`codex/go-*` 用于 Go
迁移阶段，完成的阶段合并到 `codex/go-build` 后会运行可丢弃的 Compose smoke，并
发布带 `go-build` 与不可变提交 SHA 的验证镜像。只有 `main` 或 `master` push 会
发布生产 `latest` 镜像。

## 演进方向

以下方向都建立在当前已经存在的接口或运行扩展点上，不代表功能已经全部完成：

1. **更强的记忆检索**：在授权优先的前提下，把当前全文/词法检索与 pgvector、
   异步 Embedding、混合排序和 prompt budget 结合，提升长期记忆的召回质量与可解释性。
2. **可插拔能力生态**：沿用 Capability manifest、preflight、取消、重试和审计契约，
   逐步接入更多真实可执行的外部能力。视频、音频、搜索等能力只有在适配器真正
   可执行后才会对模型公开，避免工具契约与实际运行能力不一致。
3. **媒体能力扩展**：复用现有 ComfyUI、MinIO/S3、校验和恢复链路，完善视频、音频
   等媒体类型的任务状态、预览、引用、清理和备份恢复。
4. **认知与生活世界深化**：继续完善日程、事件、关系、反思和自治预算，让短期
   交互与长期身份演进之间保持清晰的证据链和治理边界。
5. **工作流与版本演进**：继续使用 durable intent、Temporal Worker Deployment、
   immutable build ID 和 reconciliation，降低升级、重试、长任务恢复及旧执行切换
   的运维风险。
6. **跨端与契约稳定性**：保持 Web、BFF、Core 之间的 OpenAPI 生成和路由一致性，
   让未来的移动端或其他客户端复用同一浏览器边界，而无需直接接触领域存储。
7. **运维可视化**：补齐诊断导出、清理和工作流观测等已有 API 能力的 UI 入口，
   让问题定位、恢复操作和备份/恢复演练更容易被 Owner 安全地执行。

## 贡献与变更原则

- 先确认变更属于 Web、BFF、Core、Worker、契约或基础设施中的哪一层，不跨层复制
  领域规则。
- 任何 API 变更都要同时更新 OpenAPI artifact、生成客户端、路由清单和对应测试，
  并检查 snake_case / camelCase 边界。
- 新的外部调用必须通过 Core 已有的 Provider、Capability、媒体或工作流边界，
  不能让浏览器直接获得凭据、数据库行、原始 Provider payload 或未限制的错误详情。
- 领域事实、修订、证据、审计和媒体引用应保持可追溯；清理诊断数据不能删除这些
  事实，也不能绕过媒体引用失效和校验流程。

更多细节请查看各目录 README、`.trellis/spec/` 中的契约文档，以及
`infra/compose/fluctlight.env.example` 的配置注释。
