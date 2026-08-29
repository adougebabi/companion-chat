# Fluctlight Clean-Start 架构重构规划

## Goal

为 Fluctlight 完整重建一套面向后续扩展的 clean-start 系统：Node.js / TypeScript 负责浏览器与 BFF，Python modular monolith + Worker 负责全部 Fluctlight 领域语义、状态、事务与工作流，PostgreSQL / Redis / S3-compatible storage 承担明确且唯一的数据职责。

工程通过严格串行的内部子任务持续构建和验证，但不分批交付产品。只有完整 capability inventory、故障门禁和运维恢复全部通过后，才一次性切换并统一删除旧实现。本父任务只负责规划与跨任务契约；未经最终文档审阅和另行批准，不创建实施 child、不运行 `task.py start`、不修改产品代码。

## Background

- 旧 Node 进程同时承担浏览器传输、领域 flow、SQLite、Worker 和 Provider，后台任务已经出现长任务超过 lease、缺少续租/取消、应用层幂等无法支撑多 Worker 等具体正确性问题；证据见 `research/evidence-baseline.md`。
- 2026-08-17 与 2026-08-20 两次暂缓整体换语言是合理的：当时应先解决单行状态、巨型入口、事务与耐久队列。那些前置问题已大幅改善，当前复杂度和新的任务证据支持重新划分 Node / Python 边界。
- 用户要求完全弃用旧数据、旧 API、旧任务与兼容逻辑，同时完整重建当前产品能力并补齐 Fluctlight 核心闭环。
- 用户重新定义了 Fluctlight，并要求本次实现完整状态/cognitive runtime，为未来 Human/Fluctlight 混合群聊留下 Actor/Participant/Conversation/Relationship 基础；群聊产品功能本次不实现。
- 未来可能由多个新 session 继续实现，因此文档、决策、任务所有权、门禁和 code-spec 必须自包含，不能依赖聊天历史。
- 目标 NAS 有 16 GiB RAM；MTPLX、ComfyUI 和 h3 在另一台机器。Fluctlight 基础设施需要低空闲占用和长期稳定运行，不承担模型/GPU 资源。

## Normative Sources

- `decisions.md`：D001-D040 均为已批准约束；替换任何决定必须退回父任务 planning 并获得用户批准。
- `design.md`：目标拓扑、模块、数据流、事务、并发、存储、运行时和切换设计。
- `research/fluctlight-domain-model.md`：Fluctlight 状态与 cognitive runtime 的源需求。
- `research/capability-inventory.md`：最终必须重建、补闭环、删除和仅预留的能力全集。
- `READ_FIRST.md` 与 `implement.md`：跨 session 阅读顺序、严格串行 task map、所有权、门禁与回滚点。
- `.trellis/spec/backend/fluctlight-*.md`：实现必须满足的可执行 contract、错误矩阵与测试。
- `CONTEXT.md`：规范领域语言；新系统不得使用其中禁止的旧术语。

## Requirements

- R1：盘点当前运行时、模块、数据存储、后台任务、外部集成、事件流和部署，以代码、测试、配置、文档或历史任务为依据。
- R2：Node BFF 只拥有浏览器 transport/session/DTO/media proxy；Python Core 独占 Fluctlight 状态、授权、事务、workflow 和 media metadata。
- R3：Python modular monolith 使用十个已确认领域模块与 platform adapters，并定义深 interface、table ownership、依赖方向和 Worker 职责。
- R4：DBOS 已拒绝，Temporal 已被父任务接受为唯一 runtime；T02 继承已验证的 timer/cancel/signals/queries/updates/history replay/versioning/continue-as-new 语义并补齐剩余平台管理集成。
- R37：Temporal 使用 PostgreSQL visibility、无 Elasticsearch、默认关闭 UI/Prometheus 的 grouped non-HA server；资源与运行时长只做普通观测，不作为 T02 或发布硬门禁。
- R38：T02 保留平台 readiness implementation gate；T03-T11 只记录实现证据，不拥有 child acceptance、PASS、production readiness 或 cutover 权限。完整 Compose/capability/e2e/failure/security/backup 回归只由 T12 执行。
- R39：T12 是 T03-T11 全部功能、契约、跨模块、失败、安全、备份、恢复和删除证明的唯一最终验收 owner；T03-T11 的局部检查结果必须作为 evidence handoff 交给 T12，并由 T12 重新执行相关最终场景。
- R5：定义 PostgreSQL、Redis/cache、Redis Streams 和 S3-compatible storage 的权威数据、生命周期与一致性；不得形成第二权威源。
- R6：按 `implement.md` 严格串行建设；全部 child 完成后只执行一次整体切换，不导入旧 SQLite/media/job，不双读双写，不恢复旧任务。
- R7：两次历史暂缓理由必须映射为本方案的 ports/contracts、量化原型、迁移安全和防回归约束。
- R8：每项基础设施必须解决已确认需求；首版不引入 Kubernetes、独立向量库、外部 telemetry stack、第二消息中间件或第二 workflow runtime。
- R9：文档明确区分代码库事实、用户决定、技术建议和原型待验证项。
- R10：旧 `server/web/test` 冻结为证据；不修旧测试/回归、不适配新契约；T12 统一删除旧代码、依赖、CI、文档和入口。
- R11：按 capability inventory 重建全部当前用户能力，并让 Relationship/Memory/Personality/Agency/Schedule/Workflow/Media 等半成品形成生产闭环；无价值 scaffolding 不复制。
- R12：实现 `research/fluctlight-domain-model.md`；每个动态状态都有来源、历史/审计、衰减或生命周期和实际消费点。
- R13：本次不做群聊，但 Actor/Participant/Conversation/Message/Relationship/Memory/Event schema 可表达 Human + Fluctlight、Fluctlight + Fluctlight 和混合多人。
- R14：所有 author/target/subject/participant 使用 typed Actor reference；Relationship 是 Fluctlight-owned directed state，反向关系独立。
- R15：Identity 字段分 immutable/human-governed/lived；Personality 只允许长期 evidence-backed reflection；全部 revision 可审计、纠正、回滚。
- R16：PAD/momentum 使用 `-1..1`，其他 normalized state 使用 `0..1`；Python policy 按 wall time 计算 requested/applied delta 并拒绝非有限/越界值。
- R17：实现 LLM-first semantic ownership；禁止关键词、正则、字符串/词典/emoji/标点、固定分数和默认语义 fallback。
- R18：互动采用 two-stage turn：不可见 assessment/decision → Python state/policy/freeze → 独立 realization；realization 不得产生新状态副作用。
- R19：每个 Fluctlight 使用 durable sequenced inbox 和 single cognitive writer；跨 Fluctlight 并行；reflection 用 watermark/CAS；media result 作为 inbox fact 回流。
- R20：模块仅经公开 interface 协作；禁止跨模块 table/repository、domain 依赖 HTTP/Temporal/Redis/S3/Provider SDK 和 ORM entity 越界。
- R21：实现 application Unit of Work、单 `public` schema/table ownership、短 PostgreSQL transaction、stable intent/outbox/inbox 和外部 I/O 幂等恢复。
- R22：Memory 使用 typed PostgreSQL authority + pgvector/FTS hybrid retrieval；embedding async/versioned/rebuildable；exact first、benchmark-gated HNSW、先权限后相似度。
- R23：Media 使用 private S3-compatible storage/MinIO；Python 授权和生命周期，BFF proxy；稳定 key、SHA-256/version、reference/tombstone/orphan/backup 完整。
- R24：Redis Streams 仅一条 outbox-driven durable stream 和一条 ephemeral progress stream；PostgreSQL outbox/inbox 负责幂等、重放和灾难恢复。
- R25：Temporal 独占 workflow/task queue/timer/recovery；三 application task queues、stable ID、Activity heartbeat/cancel、Signals/Queries/Updates、history replay/versioning、continue-as-new 和管理操作满足 workflow contract。
- R26：Python 3.13 + FastAPI/Pydantic v2/Uvicorn；OpenAPI 生成 Core client；Core→BFF 使用 ordered cancellable NDJSON；domain framework-free。
- R27：Node 24 LTS + Fastify/TypeBox/strict TypeScript，Vue 3/Vite/Pinia + pnpm；BFF OpenAPI 生成 browser client；browser POST NDJSON。
- R28：首版只有一个强制认证 Owner Human；Python 拥有 Argon2id account/session/authorization，BFF 只负责 secure cookie/CSRF，Core 仅内网访问。
- R29：启动配置放 `.env`，runtime settings 放 PostgreSQL；单 `FLUCTLIGHT_SETTINGS_KEY` AEAD 加密 write-only 敏感设置，缺失/错误不降级明文。
- R30：使用六个显式 Model Role；generative roles 可共享 chat model，embedding 独立；per-role preflight/version/budget/provenance，无隐式 fallback。
- R31：Schedule 是 reflection-generated full local-day immutable version；future-only replan；Context authority 为 Event > Schedule > explicit pending；无默认作息/身份时钟猜测。
- R32：Fluctlight 可在 Owner 预授权 budget/quiet-hours/cooldown 内自主形成 Goal/Intention/Action；Owner 可治理但不能抹掉历史事实。
- R33：内置 Owner Diagnostics 快速查看日志、redacted prompts/model responses、turn/workflow/event/media correlation；首版无外部 telemetry stack。
- R34：Diagnostics 使用 `diagnostic_*` tables，model/turn 默认 30 天/10k、logs 14 天/50k；typed redaction、live/filter/export，sink 失败降级 stdout 且不影响业务。
- R35：SQLAlchemy 2 + Psycopg 3 + Alembic；单 schema/MetaData/linear graph；Core-first/ORM-selective；生产显式 migration，测试使用真实 PostgreSQL；Python tooling 使用 pinned uv/lockfile。
- R36：维护跨 session 的 READ_FIRST/decision/task/spec/manifests；默认一个 active child + 一个 writer。每个 child 在 start 前必须有专属 brief/manifests/paths/commands 和 no-history dry run；program outline 不构成实施授权。

## Constraints

- 本父任务保持 `planning`，不实施、不创建/启动 child，直到用户审阅最终 artifacts 并明确批准下一步。
- 新代码只进入 `apps/`、`packages/`、`infra/` 和 new-system `tests/`；旧代码不维护。
- 产品只允许一次完整切换；不 canary、不按 route/domain 分批切流、不新旧混跑、不先发精简版。
- 本地/NAS 和一条命令 Docker Compose 是部署基线；云多租户、高可用和大规模水平扩展本次不承担。
- Python 是唯一 domain writer；Node 不访问 PostgreSQL/Redis domain state/Temporal，不补领域规则。
- 外部 I/O 不在 PostgreSQL transaction 内；Redis Streams 不做 RPC/延迟队列，Temporal 不替代 domain facts。
- 业务/审计与 diagnostics 分离；diagnostics retention 不得删除 domain history。
- Provider/LLM 无效时只允许显式失败、retry、`deferred`、`no_op` 或 terminal failure，不得用代码伪造语义/回复。
- 新术语、API、schema 和 code identifiers 遵循 `CONTEXT.md`；历史旧词只允许出现在证据引用。
- 所有共享 Alembic/OpenAPI/generated clients/Compose/root/spec 文件有唯一 integration owner。

## Acceptance Criteria

- [x] `prd.md`、`decisions.md`、`design.md`、`implement.md`、`READ_FIRST.md`、capability/domain/research 和所有 `fluctlight-*` specs 互相一致，无已解决 Open Questions 或重复冲突。
- [x] D001-D040、R1-R39、全部 file:line evidence 和 capability 分类均可追溯；DBOS FAIL 与 Temporal core acceptance 均有报告/父决策证据。
- [x] `implement.md` 的 T01/T01B/T02-T12 有依赖/所有权/exit/rollback；T01/T01B 已完成评估。T02-T12 必须在各自 start 前补齐 child brief/dry run；T03-T11 只交实现证据，产品只在 T12 完整切换并删除旧实现。
- [x] `implement.jsonl` 与 `check.jsonl` 含真实 spec/research entries，无示例占位行，可供未来 child/sub-agent 裁剪。
- [x] 三次无聊天历史 dry run 已收敛至文档歧义为零；结果见 `research/multi-session-handoff-dry-run.md`。
- [ ] T01B 与 T02-T12 最终实现满足 `research/capability-inventory.md` 的全部 Must Rebuild/Close Loop，删除全部 Old Scaffolding，Future-only 不越界交付。
- [ ] 各 `fluctlight-*` code-spec 条款形成 T12 的完整最终验收并集；T03-T11 的实现检查只作为 evidence context，不构成 acceptance。Future-only、reserved、placeholder-only 能力不生成 T12 正向验收用例，只进入 scope guard。
- [ ] 最终实施前，用户明确审阅并批准规划 artifacts；任务创建许可不等于 implementation/start 许可。

## Out Of Scope

- 规划阶段的产品代码实现或旧系统维护。
- 旧 SQLite 数据、媒体 locator、作业、debug 记录、API/DTO/SSE/marker/命名的迁移或兼容。
- 分批产品发布、新旧系统混合运行、按路由切换或旧测试作为新系统 gate。
- 本次群聊 UI/orchestration/通知、多个 Human 账户/邀请/角色、Fluctlight-to-Fluctlight 自主会话。
- Kubernetes、云多租户、HA、外部 telemetry stack、独立 vector DB、Vault/KMS/key ring、direct browser S3 transfer。
- 代表 Human 执行外部不可逆操作；未来必须另建显式授权模型。
