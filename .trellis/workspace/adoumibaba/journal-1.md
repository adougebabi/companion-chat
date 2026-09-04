# Journal - adoumibaba (Part 1)

> AI development session journal
> Started: 2026-08-17

---



## Session 1: 插件化媒体生成

**Date**: 2026-08-18
**Task**: 插件化媒体生成
**Branch**: `master`

### Summary

实现 ComfyUI/h3 媒体 provider 注册表、受控本地 h3 命令执行、选择器与测试；补足长视频 lease 和媒体规范。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `09a13ca` | (see git log) |

### Testing

- `npm test` passed: 58 tests, 0 failures.
- `node --check src/companion-main.js`, `node --check src/main.js`, and `node --check server.js` passed.
- Trellis `implement.jsonl` / `check.jsonl` validation, `git diff --check`, compatibility scans, and Express smoke checks passed.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 2: h3 reuse 参数校正

**Date**: 2026-08-18
**Task**: h3 reuse 参数校正
**Branch**: `master`

### Summary

将设置页 Reuse 改为数值输入，匹配 h3 的 --reuse 2 参数语义；复跑全部测试。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `5fbba8a` | (see git log) |

### Testing

- `npm test` passed: 64 tests, 0 failures.
- Native pending, native-first fallback, media batch idempotency, missing `[DONE]`, provider error, and browser disconnect abort tests passed.
- `node --check server.js`, Trellis manifest validation, and `git diff --check` passed.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 3: Implement persona life timeline

**Date**: 2026-08-19
**Task**: Implement persona life timeline
**Branch**: `master`

### Summary

Implemented and verified the persona life-model v2, time-line slots and decisions, safe opportunity events, scene/location projection, cross-scene chat, sleep deferred reply batches, timezone-aware scheduling, audit/debug views, documentation, and regression coverage (31 tests).

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `e9c5f19` | (see git log) |
| `91936fe` | (see git log) |
| `92d4e9f` | (see git log) |
| `8d25bd0` | (see git log) |
| `42f4062` | (see git log) |
| `8606bc3` | (see git log) |
| `30b08d0` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 4: Align daily plan state and trusted chat time facts

**Date**: 2026-08-19
**Task**: Align daily plan state and trusted chat time facts
**Branch**: `master`

### Summary

Implemented continuous ready-plan state projection with default-room baselines, legacy blueprint fallback, explicit schedule overlays, trusted time boundaries, sleep-aware deferred chat decisions, media/chat source consistency, deterministic time replies, regression coverage, and synchronized Trellis specs. Preserved unrelated dirty work and archived only 08-19-state-plan-consistency.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `6064745` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 5: 媒体生成进度与简化调试

**Date**: 2026-08-19
**Task**: 媒体生成进度与简化调试
**Branch**: `master`

### Summary

新增 h3 provider 进度快照、最终提示词检查器、poll 子任务聚合与简化媒体模式；验证 40 项测试通过。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `a344629` | (see git log) |
| `b2e78a0` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 6: Freeze persona media intent before generation

**Date**: 2026-08-19
**Task**: Freeze persona media intent before generation
**Branch**: `master`

### Summary

Moved persona media concept generation to the AI capability-call boundary; persisted frozen concept, event, and temporary appearance across chat/activity/debug media jobs; removed worker concept fallback and made legacy jobs terminal failures; added fixed-template retry constraints, one-time C-stage visual acceptance with pass/retry/reject/skipped behavior, bounded video keyframes, redacted diagnostics, regression tests, and updated media contract specs.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `1d7ff6d` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 7: Implement proactive persona messages

**Date**: 2026-08-19
**Task**: Implement proactive persona messages
**Branch**: `master`

### Summary

Implemented life-event proactive messaging and chat-declared pending events with durable scheduling, strict marker validation/redaction, one-shot structured decision freezing, active-chat safeguards, provenance, diagnostics, migration v8, tests, and backend spec updates.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `311b235` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 8: AI 人格与联系人分组

**Date**: 2026-08-19
**Task**: AI 人格与联系人分组
**Branch**: `master`

### Summary

新增默认联系人分组、分组创建与人格归属切换；联系人页支持按分组筛选，点击联系人可在弹窗中切换分组；补充 SQLite migration、API 测试、前端响应式样式与规范契约。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `44c28c5` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 9: 自然语言人格初始化

**Date**: 2026-08-19
**Task**: 自然语言人格初始化
**Branch**: `master`

### Summary

将当前人格创建向导改为单段自然语言描述，由服务端 LLM 严格抽取结构化人格字段；新增分析预览 endpoint、默认值与 provenance、失败无副作用处理、确认前编辑和覆盖测试，并完成全量测试。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `72dad14` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 10: Fluctlight 展示层改名

**Date**: 2026-08-20
**Task**: Fluctlight 展示层改名
**Branch**: `master`

### Summary

确认并落实摇光（Fluctlight）领域术语；完成 README、活跃前端 title/品牌/无障碍/空状态/身份核心文案改名，保留 companion API、SQLite、环境变量、Docker、localStorage 与 legacy 入口兼容标识；通过 58 项测试、语法检查、Trellis 校验、兼容性扫描和 Express smoke，并归档任务。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `925795c` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 11: Native tool-call 迁移

**Date**: 2026-08-20
**Task**: Native tool-call 迁移
**Branch**: `master`

### Summary

完成 native scene_event/media_event/pending_event registry 与统一 dispatcher；按 index/id 累加流式 tool calls，收集 parseErrors，屏蔽 reasoning/tool JSON，支持 native-first marker fallback、SQLite payload 幂等、媒体 count 原子批量、pending job 修复、一次 continuation、provider error 和浏览器断线 abort；更新 shared-scene/media/error specs，新增失败路径与断线测试，npm test 64/64，通过语法和 manifest 校验并归档任务。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `0a733d4` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 12: Remove legacy backend compatibility layer

**Date**: 2026-08-21
**Task**: Remove legacy backend compatibility layer
**Branch**: `master`

### Summary

Completed modular backend cutover: removed server.js, legacy compatibility harness/tests and boundary inventory; fixed modular runtime, life/timeline, context/debug, provider/job, and chat commit/continuation paths. Remaining regression surface excludes deleted compatibility tests. npm test passes 307/307 after compatibility removal; external provider/performance/logging/frontend checks intentionally skipped per user scope.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `23abe9a` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 13: 前端 Vue 运行时切换与性能收口

**Date**: 2026-08-21
**Task**: 前端 Vue 运行时切换与性能收口
**Branch**: `master`

### Summary

完成 Vue 3 + TypeScript + Vite + Pinia 前端切换；接通联系人、会话历史、SSE、动态、设置、人格创建与详情管理、媒体和 debug inspector；删除旧 src 入口，Express/Docker/CI 改为 dist；修复动态评论事件、历史锚点、重试、draft/IME 和 polling guard。npm run typecheck、npm run build、node --check server/index.js、npm test (307/307) 与 Express dist cache smoke 通过；浏览器视觉/性能回归按本次范围暂缓。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `4b3d6d3` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 14: 恢复摇光实例跨日自动安排

**Date**: 2026-08-24
**Task**: 恢复摇光实例跨日自动安排
**Branch**: `master`

### Summary

补齐按 persona 本地日期幂等生成 daily plan、baseline slot 和 durable daily_plan job 的跨日追赶链路；新增停机补偿、DST/时区和重复执行验证，并更新 backend daily-plan code-spec。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `ead2dbc` | (see git log) |
| `e142d40` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 15: Go BFF contract closure

**Date**: 2026-08-29
**Task**: Go BFF contract closure
**Branch**: `codex/go-bff-acceptance`

### Summary

Created codex/go-bff-acceptance from codex/go-build; added Browser OpenAPI route parity, public HTTP auth/domain/NDJSON integration, security matrix, error detail sanitization, media nil-body/range hardening, disconnect cancellation tests, and verified Go/Node/Web/Python gates. Python format/lint/mypy baseline issues remain documented.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `cb7bba8` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 16: Complete Go BFF cutover

**Date**: 2026-08-30
**Task**: Complete Go BFF cutover
**Branch**: `codex/go-bff-cutover`

### Summary

From codex/go-bff-acceptance created codex/go-bff-cutover, deleted apps/bff entirely, moved Browser OpenAPI generator, cleaned workspace/lockfile/scripts/Compose smoke/docs/specs, rebuilt and replaced the running Go BFF container, and ran real 1-7 regression through BFF. Cases 1,3,4,5,6 passed; case2 passed after idempotent retry with 3 PNG assets; case7 verified completed proactive_message for 影者, while new proactive fixtures exposed Core no_op/backlog behavior. Core pytest 208 passed/1 skipped; Go and Node gates passed.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `35f39be` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 17: Go Core vertical slice and runtime stability

**Date**: 2026-08-30
**Task**: Go Core vertical slice and runtime stability
**Branch**: `codex/go-core-runtime-stability`

### Summary

Added an opt-in Go Core PostgreSQL-backed read/transport slice; reconciled Core OpenAPI and CI/Compose wiring; fixed compound-effect prevalidation, activation-time direct conversation and daily-review registration, strict reflection validation with atomic watermark/apply, and bounded restart-safe dispatcher priority. Rebuilt the real Docker Core/Worker stack and passed regression cases 1-7, including fresh no-restart Moment and proactive-message paths.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `efecfe5` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 18: Complete Go Core migration and real regression

**Date**: 2026-08-31
**Task**: Complete Go Core migration and real regression
**Branch**: `codex/go-core-full-migration`

### Summary

Removed Python Core runtime, completed Go Core/Worker/BFF/Web-only Compose cutover, fenced legacy Temporal executions, restored LLM schedule generation with durable timers, and passed real Docker regression cases 1-7 plus full Go/frontend/static gates.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `f7aacdb` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 19: Continue Go Core full migration

**Date**: 2026-08-31
**Task**: Continue Go Core full migration
**Branch**: `codex/go-core-full-migration`

### Summary

Closed workflow Temporal management, governance/CAS, Moments/Presence/Context, Schedule validation/replan, Provider preflight/diagnostics, Memory embedding intents, strict Core boundary and media error handling. Go/Gateway/Web checks and real cases 1,3,4,5,6,7 pass. Case 2 remains blocked by real ComfyUI workflow referencing unavailable transformer model; task remains in_progress.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `e2357be` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 20: Close Go Core Worker platform loop

**Date**: 2026-08-31
**Task**: Close Go Core Worker platform loop
**Branch**: `codex/go-core-worker-closure`

### Summary

Added PostgreSQL outbox claim/retry/terminal handling, Redis Streams publisher and durable consumers with inbox/effect/head, duplicate/reclaim/poison handling and PEL-safe trim; added cognition/platform Temporal workflows, queue-specific registration, durable cognition intents and Worker runtime integration. Core race/vet/build and PostgreSQL+Redis/miniredis platform tests pass; Docker worker groups pending=0 lag=0 and fresh cognition intent fan-out to all groups verified. Baseline product flows remain accepted evidence and were not rerun because unaffected.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `59652fa` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 21: Fluctlight Intelligence P1 closure

**Date**: 2026-08-31
**Task**: Fluctlight Intelligence P1 closure
**Branch**: `codex/fluctlight-runtime-phase1`

### Summary

Created a dedicated worktree, retained POST NDJSON for turns, added canonical native/external capability slots and tool calls, implemented ContextProjection, self-evaluation/claim gating, scene/presence, Memory retrieval and revisions, Reflection/evolution with CAS and rollback, and validated Go Core/Gateway tests, vet, build and race suites. Browser pnpm checks remain environment-blocked by npm DNS.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `371b136` | (see git log) |
| `a917517` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 22: 修复跨端 UI、状态加载与列表兼容问题

**Date**: 2026-09-01
**Task**: 修复跨端 UI、状态加载与列表兼容问题
**Branch**: `master`

### Summary

详情页新增身份/人格、生活世界时间轴、关系与记忆只读展示并统一安全格式化；治理页保留操作入口并改进文案；设置 Accordion 切换无需刷新；Go Core 与 Web 统一 actor_ids 并兼容旧 members payload；完成 Web 类型检查、生产构建、回归测试及 Core/BFF Go 测试。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `6fef75f` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 23: Periodic self-awareness wake-up loop

**Date**: 2026-09-01
**Task**: Periodic self-awareness wake-up loop
**Branch**: `master`

### Summary

Added a durable periodic wake_up.current Temporal loop. Each cycle persists bounded attention/thought/desire/agency and internal dynamics as a cognition fact, feeds reflection/self-model evolution, and freezes policy-approved external actions through existing autonomy workflows. Added product.wakeup settings, wake-up history in detail/governance UI, migration 0021, smoke fixture support, README and code-spec updates; all Core/Gateway/Web checks passed.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `1fbd6db` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 24: Complete personality growth and capability request loop

**Date**: 2026-09-01
**Task**: Complete personality growth and capability request loop
**Branch**: `master`

### Summary

Implemented the complete cognition growth vertical: structured appraisal/focus/internal dynamics and action-result facts, arbitrary typed Drive/Preference/Trigger slots with revision/CAS provenance, generic CapabilityActionWorkflow, capability.request tool and global owner-reviewed capability request pool, BFF/OpenAPI/Web governance surface, migration 0022, docs/specs and smoke fixture updates. Verified Core/Gateway tests, Core race/vet, pnpm generate/typecheck/test/build, OpenAPI drift and fixture syntax. Compose smoke was not run because infra/compose/fluctlight.env is absent.

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `9080d70` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 25: Persona 分层与防漂移

**Date**: 2026-09-02
**Task**: Persona 分层与防漂移
**Branch**: `master`

### Summary

完成 Persona-only vertical slice：新增 Core Persona canonical 快照、Developing Self claim/revision 存储与治理、Current State 分层上下文；更新初始化/Reflection/API/BFF/Web 详情治理展示，禁止自动人格写入旧 personality/self_model 路径；Go、BFF、browser-client、Web 全量测试与构建通过。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `9b5b55e` | (see git log) |
| `8502886` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 26: 摇光视觉身份工作流框架

**Date**: 2026-09-03
**Task**: 摇光视觉身份工作流框架
**Branch**: `master`

### Summary

在独立工作树 codex/yaoguang-visual-identity 中完成 visual identity 聚合、初始化与 wake-up 触发、Temporal image→vision→patch→regenerate 框架、canonical/character-sheet 持久化、罩杯到胸部 LoRA adapter、Scene Image context binding，以及 Vue 时间轴和媒体事件合并；Go/Web/BFF/client 验证通过。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `e596b12` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 27: 交互与 LLM 队列调度优化

**Date**: 2026-09-04
**Task**: 交互与 LLM 队列调度优化
**Branch**: `master`

### Summary

完成聊天输入单行布局与提交即清空，修复 Vue 流式 assistant 文本只显示首个 token 的响应式问题；新增 generic_llm/embedding 双绑定兼容、按场景记录的 Provider 优先级队列、独立并发设置、诊断 queued/running/terminal 生命周期和唤醒失败后的 Continue-As-New 恢复。Go Core/BFF/Web/browser-client 全部质量检查通过；保留用户原有三份未提交后端 spec。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `b96d870` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 28: 媒体提示词拍摄视角规则

**Date**: 2026-09-04
**Task**: 媒体提示词拍摄视角规则
**Branch**: `master`

### Summary

将构图优先的前摄/后摄/全身镜/第三方拍摄规则写入 media_prompt 系统指令；补充单人、多人和完全模糊请求的设备可见性约束，更新媒体提示词契约与 Go 测试。go vet ./... 与 go test ./... 均通过。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `264177d` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete
