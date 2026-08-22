# 强制 LLM 摇光解析与图片补偿作业

## Goal

把新建摇光实例的长描述解析和图片生成作业都纳入可靠的模块化运行时：人格字段必须由 MTPLX 结构化解析，媒体生成必须有持久化、可重试、可补偿的作业链路。用户确认前不得创建摇光实例；provider 或作业暂时失败时，系统必须保留可诊断、可恢复的状态，而不是静默回退或留下“处理中但永远不会完成”的记录。

## Background and Confirmed Facts

- 当前 `POST /api/companion/interviews/analyze` 在没有显式 analyzer 时回退到 [`server/infrastructure/interview-repository.js:49`](/Users/vinson/Documents/project/个人/local-ai-companion/server/infrastructure/interview-repository.js:49) 的姓名正则、默认角色和原文截断，不调用 MTPLX。
- 当前默认 runtime 没有构造 interview analyzer；聊天 MTPLX port 被注入到 chat flow，但没有复用于访谈分析。
- 聊天媒体 `media-flow` 已能在同一事务中修复缺失 placeholder/job；活动媒体 projection 和 submit -> poll follow-up 仍存在后续 job 丢失窗口。
- 项目已有 `companion_jobs`、lease、attempt、retry、worker dispatcher 和 `createFlowEffectAdapter()`，本任务必须复用这些边界。
- 工作区已有其他未提交改动和现有摇光数据；本任务不得 reset、删除或重新初始化它们。

## Requirements

### R1. 强制 MTPLX 摇光解析

- 用户提交完整描述后，服务端必须调用默认 MTPLX JSON completion analyzer，输出受限的摇光结构化字段，至少覆盖 `name`、`role`、`foundation`，并支持现有 blueprint 所需的 `interests`、`visualBaseline`、`supportingCast`、`routine`、关系初始描述等字段。
- 生产路径不得使用姓名正则、默认“陪伴者”或原文截断作为分析主路径；不得在 analyzer 不可用时静默回退到 `interviewRepository.analyze()`。
- provider 不可用、超时、空响应、非 JSON、未知字段或结构不合法时，接口返回有界的 `502` 错误，不创建 interview row，也不创建 persona。
- 分析成功后创建 ready interview session，返回 `interviewId`、`source: 'llm'`、结构化 preview 和 `inferredFields`；原始 description 只存在于本次 provider 请求，不落库、不回传、不进入后续 prompt。
- 用户确认前可以编辑 preview；只有 activate 成功后才创建 persona，并将确认后的结构化数据写入 foundation/blueprint/life state 所需的现有生命周期路径。

### R2. 图片创建补偿作业

- 图片/视频创建必须继续通过现有 media flow、effect adapter、durable job repository 和 media job service，不在访谈 analyzer 或 HTTP handler 中直接等待 provider。
- 活动媒体的 activity projection 与媒体 effect enqueue 必须使用调用方事务，避免活动已提交但 `activity_image` / `activity_video` job 缺失。
- provider 异步提交后的 source settlement 和 poll job enqueue 必须具备原子性或可重复补偿；任何 source 已完成但 target 仍 processing 且没有 poll job 的状态，都能通过稳定幂等键重新建立 poll job。
- 补偿作业必须复用冻结的 `personaMediaConcept`、external id、source job 和原始幂等键，不重新让 LLM 猜场景，不生成重复的随机目标。
- quality retry / poll follow-up 必须使用确定性幂等键，重启、重复 tick 或 crash-after-enqueue 不得创建重复 successor。
- 最终失败必须投影到用户可见 target（activity 或 chat message）并保留 bounded error；临时网络/ provider 错误走通用 retry，不得提前将目标标记为永久失败。

### R3. 兼容与可观测性

- 保留现有聊天媒体、worker lease 和 retry 契约；只增加必要的 analyzer port、媒体补偿/原子 follow-up 能力和测试。
- 旧 interview rows 继续可读取和激活；新路径的 `source` / inferred metadata 采用 additive 方式，不迁移或删除历史数据。
- 记录 source、job id、idempotency key、attempt 和失败阶段，便于从 debug/数据库确认“未调用 provider”“已重试”或“已终态失败”。

## Acceptance Criteria

- [x] `POST /api/companion/interviews/analyze` 的默认 runtime 在 mock MTPLX 成功时返回结构化 ready preview 和 `interviewId`，且测试证明 repository regex 没有被调用。
- [x] MTPLX 返回 fenced JSON、未知字段、缺失必需结构、空响应、超时或网络错误时返回 `502`，数据库不新增 interview/persona。
- [x] 用户编辑 preview 后 activate，最终 persona 的 foundation、blueprint、routine/interests/cast 等字段来自确认后的结构化数据；原始 description 不在 interview row、响应或 prompt trace 中。
- [x] 活动媒体 enqueue 失败时，activity insert 和 `media_status='queued'` 共用事务，不留下半提交状态；稳定 key 支持单一重放 job。
- [x] source job 已 complete、poll enqueue 中断后，补偿 tick 能建立唯一 poll job并继续完成图片；重复补偿不会重复 provider poll。
- [x] provider 临时失败会按通用 attempt/backoff 重试，达到上限后 target 显式变为 failed 并保留 bounded error。
- [x] quality retry、worker lease recovery、重复 tick 和 stale worker 场景使用确定性 successor key，不重复资产或旧状态覆盖。
- [x] `npm test`、`npm run typecheck`、`npm run build`、相关 Node syntax checks 和 `git diff --check` 全部通过。

## Out of Scope

- 不重写 Node runtime、聊天 SSE 或前端视觉语言。
- 不把所有现有 JSON completion helper 一次性抽象成新的全局框架；只抽出访谈 analyzer 必需的稳定 port。
- 不为每个摇光实例创建独立 Node/数据库进程。

## Notes

父任务负责跨子任务契约和最终回归；具体代码分别由两个子任务实现。实现顺序为先完成强制 LLM 分析，再完成图片补偿作业，最后做跨层回归。
