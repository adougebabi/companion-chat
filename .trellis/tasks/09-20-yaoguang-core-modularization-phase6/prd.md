# 摇光项目第六阶段：internal/core Package 模块化拆分

## Goal

将当前集中在 `internal/core` 巨型 package 中、已具有相对明确职责边界的代码，拆分成清晰的 Go package（如 `internal/ai/*`, `internal/capability`, `internal/conversation`, `internal/cognition/*`, 以及按领域划分的 `personality`, `memory`, `schedule`, `media`, `visualidentity` 等），使既有逻辑边界进一步成为编译器能够约束的物理依赖边界。

本次重构为**结构性重构（Structural Refactoring）**：
- 优先搬代码、整理依赖、建立 package 边界；
- 原则上保持业务行为、测试断言与对外契约 100% 不变；
- 不重新设计业务流程、不修改人格规则、不改变 Agent Loop / Prompt / Tool 语义、不改变事务边界与数据库 Schema、不修改 Temporal 与 HTTP/Browser API 契约；
- 职责本身存在历史问题但不阻碍物理拆分的，记录为“阶段七候选债务”，本阶段不借机横向修改。

## Confirmed Baseline Facts

1. **Git State**:
   - Current branch: `codex/yaoguang-core-modularization-phase6` (branched from `master` @ `c19dd9ea6b8bbcfb26b25c7557bc71038aa50408`)
   - Working tree clean: only untracked docs/spec modifications from previous sessions
2. **Package Topology**:
   - Go Module: `github.com/fluctlight/local-ai-companion/apps/core-go`
   - Active packages: `cmd/{api,cutover,migrate,setup-token,worker}`, `internal/{config,core,httpapi,httpapi/browser,migrations,platform,workflow}`
   - `internal/core` consists of 200 files (97 `.go` source files, 103 `*_test.go` test files).
   - Core importers: `cmd/api`, `cmd/worker`, `internal/httpapi`, `internal/workflow`.
3. **Test Baseline**:
   - Baseline command: `go test ./...` exits with code 0 (all packages pass).
   - `internal/core` all unit tests PASS (with expected environment skips for `GO_CORE_TEST_DATABASE_URL` when PostgreSQL container is not present).

## Proposed Grouping & Phase Sequencing

根据指导原则与物理依赖拓扑，分组分批次实施，每组完成后严格执行“编译 + 单元测试 + 回归测试 + 依赖无环验证”：

1. **Group 1: AI 基础模块 (`internal/ai/*`)**
   - `internal/ai/model`: Eino Model 工厂、ProviderClient/Queue/RedisQueue、运行时支持、流式处理、模型配置与审计等
   - `internal/ai/prompt`: Prompt Composer、Prompt Fragments/Slots、Message Assembly、Budget 管理
   - `internal/ai/agent`: Eino ADK Runtime、Runner、Loop Budget、Termination、Trace
   - `internal/ai/task`: 通用模型任务定义与契约 (如 Model Tasks / Schemas)
2. **Group 2: Capability 基础 (`internal/capability`)**
   - Capability 核心接口、定义与元数据
   - Registry、ContextSlot / Resolver 接口定义
   - Invocation, Prepare, Executor, ADK Tool Adapter, 错误与状态
   - 具体领域能力保留在对应领域或依赖接口，不形成 God Facade
3. **Group 3: Conversation 领域 (`internal/conversation`)**
   - `HandleTurn`, turn decision, query continuation, takeover rules/arbitration
   - Conversation runtime, reply binding, raw history
   - 依赖 `ai`, `capability`, 业务只读投影接口；AI 基础不得反向依赖 conversation
4. **Group 4: Cognition 领域 (`internal/cognition/*`)**
   - `internal/cognition/wakeup`: WakeUp 周期、Intents、ADK 决策边界
   - `internal/cognition/reflection`: Reflection V2 闭环、Domain evidence/proposal、Watermark/Window
   - `internal/cognition/autonomy`: Daily Review, Native Cognition, Growth
5. **Group 5: 业务与领域模型 (`personality`, `memory`, `schedule`, `media`, `visualidentity`)**
   - 逐步物理拆出 Personality, Memory, Schedule, Media, Visual Identity
   - 领域专属 Capability 与 Repository 随业务 package 迁移
6. **Group 6: App Container 收拢与整体回归验证**
   - `core.App` 演进为 Composition Root / Container
   - 保证 `cmd/api`, `cmd/worker`, `workflow`, `httpapi` 顺利衔接
   - 执行全量回归与依赖无环静态分析

## Requirements & Invariants (Behavior Freeze)

- [ ] **严格保持业务行为不变**：
  - 普通对话、Query Tool、ADK Tool 回填、Takeover approve/decline、Persistent Switch
  - WakeUp no-op / 主动动作、Reflection、Daily Review、Media 生成、Visual Identity、Schedule
- [ ] **依赖单向无环**：
  - `ai/*` 位于底座，不得 import 任何具体业务或 `core.App`；
  - `capability` 基础层不得 import 具体业务实现；
  - 业务模块依赖 `ai` / `capability`；`workflow` 依赖 application service，反向禁止；
  - 严格禁止通过定义 `AppFacade` / `CoreService` 等 God Interface 来绕过循环依赖。
- [ ] **测试同步迁移**：
  - 测试文件必须跟随实现文件移动，不得丢弃任何现有断言；
  - 跨模块测试保持端到端断言，基线通过率 100% 对齐。
- [ ] **事务完整性**：
  - 领域拆分不得切碎现有跨表短事务，Unit of Work 由 Application Service / Composition Root 统一协调。

## Acceptance Criteria

- [ ] 输出完整的 `package-migration-map.md`。
- [ ] Group 1 ~ Group 5 迁移完成，每组代码编译无警告、测试全部通过。
- [ ] `internal/core` 大体量职责明确代码完成下沉，剩余文件仅作为 Composition Root 或过渡桥接，职责明确并在阶段七债务清单中说明。
- [ ] `go test ./...` 全绿。
- [ ] 生成实际 package 依赖拓扑图，无循环依赖。
