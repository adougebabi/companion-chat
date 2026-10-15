# 认知与 Fact-only 回复上下文/结算实施报告

2026-10-14；分支 `codex/fact-only-reply-vision`，源码基线 `d696c44`。本轮已完成代码和本地验证，用户已批准任务 commit-plan 中的四笔本地工作提交，实际记录以 git log 为准。未 push/部署，用户环境验证仍待执行。

## 已实现与代码落点

- `fact_only_context_views.go`：两处局部只读状态、Goal/关系/Actor 投影；使用正式 actor_id，先 profile/recipient 授权，保留人格、有效目标原条件和真实操作约束，去掉旧执行策略；不复制整份 Runtime 再删字段。
- `fact_only_facts.go`：来源、epistemic、时间/scope 分开，精确去重保留内部全部来源/记录指针，同源不同说法只报未裁定差异，unknown/cleared/false 区分；不修写 Memory/关系/Goal。
- `fact_only_history.go`、`prompt_context_assembler.go`：原 role/content 不改，选中历史的元数据另列；保护连续 assistant 及触发用户，必要时按实际 source_fact_id 同会话回补。sender 时间不被 self 显示时区重标；未证明来源的 Markdown 标题保留。
- `provider_context.go`、`cognition_agent.go`、`reply_context.go`：Fact-only 接新视图与精简协议，其他 surface/Moments 保留原选择；工具后 Writer 读有效新投影/人格，完整认知合同不缩水。
- `fact_only_reply_capability.go`、`fact_only_reply_facts.go`、`reply_media.go`：v2 closed schema 移除模型 purpose，服务端用途/旧 draft 恢复与真实引用检查保留；有效 Goal ref 的原始 snapshot 也不能绕过新 Goal 投影。媒体继续走原同 draft 链。
- `tool_publication.go`：仅可信 Fact-only caller 按已有消息 reply_mode 桥接旧/新 purpose 的同主题抑制；legacy 精确匹配保持。被抑制或 replay 时回读已存真实正文，不能回填新候选假装发送。
- `fact_only_request_phase.go`、`eino_model_runtime/request_policy.go`：发布后改用结算说明，合法可选查询继续；原一次无工具/schema 修复不重发。新增实际序列化配置 allowlist 事件，不新增默认完整 wire/reasoning 采集。
- `effective_life.go`、`goal_execution.go`：仅补既有 active_until、依赖状态的只读来源元数据，不改写入或准入规则，不迁移表。

实际调用图、来源表和原件缺失见 `.trellis/tasks/10-14-fact-only-context-settlement/research/call-chains.md`。最终协议及 Tool/schema 绑定逐字导出到 `docs/fact-only-context-protocols.md`。

## 字段保留与过滤

| 来源 | 保留 | 过滤 / 原因 |
| --- | --- | --- |
| Working Persona | 当前完整编译人格、表达偏好与边界 | 不接受父模型重写或统一风格模板 |
| Actor/时钟/历史 | self/profile/recipient 区分、as_of、持久化 sender/时间、真实角色/正文 | 不从正文日期推算现在，不把 actor_user 当昵称；Runtime/Validation/Tool 不进公开聊天 |
| Goal | 原本意/动机/原条件/状态/真实期限、有效可见集合、真实依赖/失败 | next_step/strategy/话术 wait/blocker/旧评估/治理；不按本轮关键词隐藏目标 |
| Memory/摘要 | 候选内容、来源种类、epistemic/置信度、发生/有效时间、不确定性 | 重复候选归组；不把 verified 当事实，不传旧内部链；不做自由文本语义裁决或任意反转义 |
| 当前状态/日程 | 字段级有效状态、当前及必要下一确认安排、真实结果 | 全日表/重复 upcoming/无关 PAD；计划不是执行证明 |
| 原生参数/引用 | offered+live 授权引用、draft_ref/action_result_refs、必要 topic_key | purpose/正文/语气/改名策略由 closed/typed 边界阻断；路由元数据不进 Writer |

## 脱敏输入对照与统计

fixture 明确由文档已知摘录和合成材料组成，Actor v2 按生产形状修正；完整事故请求未提供。before 使用原协议和未改的旧 helper 重建并固定哈希，认知对照固定 v2 工具契约，只比较上下文装配。它们不是原事故 HTTP 请求，也没有真实模型调用。

| 输入 | 调整前估算 tokens | 调整后估算 tokens | 调整前字符 | 调整后字符 |
| --- | ---: | ---: | ---: | ---: |
| Qwen 认知（逻辑装配） | 12646 | 11817 | 10144 | 9481 |
| Cydonia 回复（逻辑装配） | 4229 | 5173 | 3413 | 4168 |

回复输入增加来自保留关系、历史元数据与来源类型，不宣称每个样例都压缩。认知逻辑装配包含完整最终 response schema；真实带工具请求仍按已验证机制省略 response_format，不能把逻辑估算当实际 provider prompt_tokens。

| 逻辑一级分区 | 认知前 → 后 | 回复前 → 后 |
| --- | ---: | ---: |
| system | 3129 → 1527 | 1041 → 972 |
| runtime_facts | 1651 → 1466 | 1896 → 1609 |
| active_memory | 0 → 0 | 0 → 0 |
| resident_memory | 0 → 0 | 0 → 0 |
| recent | 423 → 979 | 233 → 979 |
| retrieved_memory | 232 → 489 | 114 → 489 |
| conversation_summary | 0 → 0 | 0 → 0 |
| current_input | 62 → 62 | 62 → 62 |
| tools | 957 → 957 | 840 → 840 |
| response_schema | 6094 → 6094 | 3 → 3 |

一级分区分别估算，含封装和取整误差；不要把重叠的 runtime.* 或 protocol/persona 子分区再次加入总量。完整 before 在 `apps/core-go/internal/core/testdata/fact_only_context/`，after 和模块 JSON 在任务 `verification/`。

真实 prompt/completion token、模型延迟与错误率本轮为 NOT_RUN/unknown；不以合成 HTTP usage 或测试耗时冒充。服务运行时按每个物理 model_call_id 分别记录实际 usage/延迟、phase、choice/format/sampling，绝不将多次调用相加当单次上下文。

受控原生线缆矩阵为 Generate/Stream × policy/no-policy × 5 结局，共20个子场景；每个正常案例3次请求，需修复案例4次，均只有一次模拟 reply effect。它验证代码协议，不是真实发布/真实模型效果。

## 本地检查与未执行项

- Go 全套、core/workflow race、vet、build 通过；条件化数据库/真实 Provider/外部服务用例仍 SKIP，详见任务 local-check.md 与 verification/check-summary.json。
- pnpm typecheck/test/build 通过：Browser Client 21/21、Core Client 2/2、Web 105/105。Web 构建保留既有 >500 kB chunk 提示，没有借机改构建或依赖。
- 新增真实事务组合2个子场景、purpose 兼容5个子场景、实际 source 回补1个场景均因未配置 GO_CORE_TEST_DATABASE_URL 而 SKIP，不能称数据库闭环通过。
- 真实 Qwen/Cydonia、图片生成/视觉、Temporal/Redis/S3 环境及人格多轮体验均未执行。R01–R26 逐项层次状态见任务 acceptance-matrix.md。

独立源码核验发现的 Actor 键形状和跨版本 purpose 问题已修正，原报告与处理记录保留在任务 research/。最终线缆门禁发现的记忆元数据分区超额也已修正，并由最终实际输入断言及 Go/race 再验证。

## 可执行入口与回退

离线对照只使用固定合成输入，不发模型请求或真实消息：

```bash
FLUCTLIGHT_LIVE_PROVIDER_TEST=0 GO_CORE_TEST_DATABASE_URL= \
  GOCACHE=/tmp/fluctlight-go-cache \
  go -C apps/core-go test ./internal/core -run '^TestFactOnlyFixtureReplayEstimates$' -count=1 -v
```

如需导出 after JSON，可额外设置测试专用 `FLUCTLIGHT_CONTEXT_REPLAY_OUTPUT` 为本地绝对目录；这不是生产模式或设置项。输入/baseline 的 hash manifest 在 fixture 目录。

数据库验证使用现有一次性测试 PG 入口（具备 CREATE/DROP DATABASE 和 vector 能力；不要填写生产连接），只选择新受控场景：

```bash
FLUCTLIGHT_LIVE_PROVIDER_TEST=0 GOCACHE=/tmp/fluctlight-go-cache \
  go -C apps/core-go test -race ./internal/core \
  -run '^TestFactOnly(CommittedReplyMalformedFinalSettlement|ProactiveSuppressionAcrossPurposeVersions|HistoryBackfillUsesExactStoredSource)$' -count=1 -v
```

真实独立 Writer 复用原 `TestFormalAgentE2E/conversation_reply`，配置仍为现有 FLUCTLIGHT_LIVE_REPLY_PROVIDER_BASE_URL/MODEL/可选 API_KEY，以及既有 live suite 的测试环境与通用绑定。入口要求独立配置，不能悄悄用主模型代替；它只验证独立 task，不代表事故同参数A/B、完整发布或人格长聊已通过。原事故完整同参数回放缺原件，明确未执行。

回退使用既有 conversation.reply mode=legacy 与原渠道/镜像说明；待处理 draft/continuation 的排空/取消沿 docs/fact-only-deployment.md 和 docs/fact-only-user-verification.md。没有新迁移、模式、Actions 或自动 stable/master/生产操作。

## 未解决的数据与效果限制

- 未获得两份粘贴原件、三次认知和完整 Writer 的实际 wire/原始响应/采样，不能确认事故 JSON 错误具体成因。
- Markdown 夜间标题生成源不明，保留原文；修复权威时间输入不等于改写已发历史。
- verified/关系 category 不是任意语义成立证明；旧关系/承诺/摘要可能仍矛盾。自由文本不能可靠确定的内容保留来源与未知，不自动修 DB 或要求用户证明。
- 不保证每次模型均无幻觉、无内部编号或恢复人格表现。保留原校验与真实错误，语义效果需用户真实环境回放，不能自动替模型写成正确答案。
