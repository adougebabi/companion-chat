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
