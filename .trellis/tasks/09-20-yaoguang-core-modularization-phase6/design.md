# 摇光第六阶段：架构设计与模块解耦方案 (Design)

## 1. 架构目标与分层拓扑

通过物理 Package 边界，实现单向无环的静态编译依赖：

```
[cmd/api, cmd/worker, internal/workflow, internal/httpapi]
                          │
                          ▼
                  [internal/core]
               (Composition Root / App)
             ┌────────────┼────────────┐
             ▼            ▼            ▼
     [conversation]   [cognition]  [domain: personality, memory, schedule, media, visualidentity]
             │            │            │
             └────────────┼────────────┘
                          ▼
                 [internal/capability]
                          │
                          ▼
                   [internal/ai]
       (ai/model, ai/prompt, ai/agent, ai/task)
                          │
                          ▼
            [PostgreSQL / Eino SDK / Redis]
```

### 依赖单向规则：
1. **AI 基础模块 (`internal/ai/*`)**：位于所有业务的最底层。不得 import `conversation`, `cognition`, `personality`, `memory`, `schedule`, `core.App`。
2. **Capability 基础模块 (`internal/capability`)**：定义能力的规范、注册表、Invoker、ADK Tool 适配器。不包含具体业务实现，不 import `core.App`。
3. **领域模块 (`personality`, `memory`, `schedule`, `media`, `visualidentity`)**：拥有自身实体、计算规则、只读数据投影。可依赖 `ai/*` 与 `capability`。
4. **业务流模块 (`conversation`, `cognition`)**：协调模型、能力与领域状态。依赖 `ai/*`, `capability`, 领域模型接口。
5. **容器层 (`internal/core`)**：保留 `App` 结构体，持有各子模块服务指针，负责短事务协调 (Unit of Work) 和 Composition Root。

## 2. 详细接口设计与关键解耦模式

### 2.1 ProviderClient 与 Repository 解耦
`internal/ai/model` 中的 `ProviderClient` 原本通过 `p.DB.Pool()` 直接查询模型配置和 Secret。
通过定义窄接口 `ProviderDatabase`:
```go
package model

type ProviderDatabase interface {
    QueryModelRole(ctx context.Context, role string) (ProviderEndpointConfig, error)
    QuerySecret(ctx context.Context, purpose string) ([]byte, []byte, error)
    RecordModelRun(ctx context.Context, run ModelRunRecord) error
}
```
`core.PostgresRepository` 实现该接口，并在 `NewApp` 时注入，使 `ai/model` 完全脱离具体数据库连接池实现。

### 2.2 ADK Runner 与 App 解耦
`adk_conversation_runtime.go` 原通过 `*App` 获取 CapabilityRegistry 和 CapabilityRuntime。
通过将 `ADKCapabilityInvoker` 接口作为外部传入的标准协议：
```go
package agent

type ADKCapabilityInvoker interface {
    ExecuteWithID(ctx context.Context, callID, capabilityName string, argumentsJSON string) (string, error)
}
```
ADK Runner 仅驱动 Eino 官方 Agent Loop、处理最多 2 轮调用、Tool 回填与 Trace 记录，不直接感知任何业务 App 或持久化句柄。

### 2.3 事务完整性保证
跨表操作（如会话消息、Inbox 事实、Outbox 事件、意图创建）依然在 `core.App` 或各协调 Service 层面通过 `withTransaction` 统一开启，领域 Service 接收 `pgx.Tx` 参数（如 `ApplyTx(...)`），确保物理拆包绝不破坏现有 ACID 事务原子性。

## 3. 风险与阶段七候选债务 (Phase 7 Debt Log)

| 模块/文件 | 当前耦合点 | 为什么本阶段不强拆 | 阶段七建议方案 |
|---|---|---|---|
| `mutations.go` (1670 行) | 同时包含 HandleTurn, Schedule 接收, 复杂事务与锁机制 | 牵涉会话与日程的混合事务，强行完全清空 core 会破坏事务原子性 | 在第七阶段引入完整的 Unit of Work 模式下沉 |
| `builtin_capabilities.go` | 集中包含了几乎所有业务能力的执行适配 | 各能力目前直接访问 `app.*` 方法，需要逐步建立领域专属能力包 | 各领域独立定义其 Capability，由 core/registry 进行统一挂载 |
| `cognition.go` (1026 行) | 包含事实 Claim、生命周期抢占、后台意图分发 | 属于后台编排中枢，与 Temporal 调度和 Redis 强绑定 | 细化为 `cognition/pipeline` 与 `cognition/dispatcher` |
