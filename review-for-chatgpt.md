# MeroMero-first 改造复核报告

状态：**实施中，尚未达到替换旧主链的条件**。本报告记录已经发生的工作与未完成项；`IMPLEMENTED` 不等于真实环境验收。详细工作台账见 [实施台账](docs/verification/meromero-first-ledger-20261109.md)，当前 31 个正式 Agent 及其他模型入口见 [Agent 盘点](docs/verification/meromero-agent-inventory-20261109.md)。

## A. 当前结论

- 已实施若干局部改动：普通图片第二次质量不通过时不自动交付；模型可见的能力目录调用与严格 schema 一致；显式旧模型回退不再把拒绝、业务/工具或输出核验失败当成换模型条件；Agent/variant 模型路由及 Diary 分阶段模型快照；Image/Moment 声明、开关及主要运行门禁；WakeUp 区分等待回应与新事件；连续追发接入现有 superseded 路径。
- 已运行 Go、前端类型检查和受控原生工具循环测试。受控 HTTP 或模拟 Provider **不代表真实模型、真实看图或真实生图通过**。
- 用户提供本地 Docker 授权后，已启动只绑定回环的临时 PostgreSQL 16/pgvector，完成一批真实数据库专项测试；它不是生产数据。本机指定 MeroMero 已有文本及合成工具往返的真实调用证据，但 `direct` 私聊实测失败；真实 ComfyUI 与 UI 到运行消费仍未验收。
- 模型采样迁移、配置保存/读取、Diary 与 Studio 部分阶段恢复已通过隔离 PostgreSQL 专项；其他后台快照及真实 Provider 仍未完整验收。主聊天去固定双模型、完整插件、表达学习、全量记忆治理、管理和重放闭环均未完成，不能宣称本轮总体完成。

已经完成并有对应局部验证的小节点包括：低置信度 claim 的状态持久化；图片第二次审核不通过后的失败与恢复保护；Image/Moment 停用后的后端准入与已受理回执重放；表达学习的来源、审核、撤销及旧 Writer 发布前复核；Diary/Studio 阶段模型快照恢复。这些节点不代表真实图片、实验 `direct` 或全领域验收通过。

2026-11-10 应用户要求已分批提交代码：`21c70620`、`6b0b9af1`、`5228258a`、`5229a5b8`、`016f2752`。后端隔离 PostgreSQL 定向回归 25 个主测试通过，客户端 44 项和前端 101 项通过；仍未推送或发布。此前本机 MeroMero 在实验 `direct` 私聊中曾连续生成不合法认知参数，三次后终止、没有发布。表达参考现已接入实验 `direct` 的受控路径，但真实模型未验收；媒体正文一致性与记忆纠错并发仍未完成。

提交后的全量回归发现表达学习 API 缺少 OpenAPI 声明、重新生成会丢失客户端方法；已补齐生成源及回归并追加提交 `b33df6e5`。该提交的 Go 全量回归 18 个有测试包通过，按测试事件计数 3053 通过、977 跳过。后续提交 `e0113794` 对齐审核字段的可空语义及响应描述，再次通过路由一致性、客户端 46 项和 Web 类型检查；相关 React 14 项在前一提交通过。上述跳过项及真实模型、真实图片仍不能记为验收通过。

对主链格式问题的进一步核对：模型实际处理的是原生工具参数 schema，而非同轮最终 `response_format`；此前 18 个 `decision` 字段没有语义描述，15 个原可选字段在工具传输时变为必填但可为 null。已在 direct 工具补字段说明并验证物理传输，代码及测试提交 `d32155b0`。合成单次对照中，补说明的模型输出通过参数校验，但样本不足；正式私聊首次仍有字段类型错误，第二次受 GPU 内存不足阻断，尚未验证成功，也未将实验链设为默认。详见 [实施台账](docs/verification/meromero-first-ledger-20261109.md)。

用户要求后的最新一次真实重试没有遇到显存不足；主 Agent 连续三次调用回复工具后仍因认知字段类型错误终止，未获得成功发布回执。前两次可见错误为 `self_evaluation` 和 `state_expression` 应为对象，模型却填写了数组；第三次具体字段未被证据捕获。日志 `/private/tmp/fluctlight-direct-user-retry.log`。

进一步按用户要求把 direct 工具的 15 个业务可选字段改为真正可以省略；有值仍做原样类型和业务校验，`null` 按缺省处理，代码及测试提交 `33a10b1d`。受控物理请求及隔离数据库回归通过，本机单次合成对照的可选版本通过参数校验。正式私聊一次未产生可用 ToolCall，原始响应复核又遇本机 GPU 内存不足，仍不能宣称端到端通过；其他任务的严格 schema 保持原样。详见 [实施台账](docs/verification/meromero-first-ledger-20261109.md)。

实验 `direct` 主聊天现已接入已审核的表达参考并在发布事务内复核，代码及测试提交 `a0cbb7ef`。受控 Provider + 隔离 PostgreSQL 验证了正常使用、失效、工具行动后续接、取消及重放；真实 MeroMero 的完整私聊和表达使用仍未验收。

聊天页面现在支持刷新或重新进入后向已确认的后台对话追加消息，并保留未知受理结果的身份与草稿；代码及测试提交 `7b6ad337`。受控 React/HTTP 回归 84 项和前端类型检查通过，本地浏览器内存夹具的追加操作通过；真实数据库多轮并发及模型综合回应仍未验收。

T02 已修复人格编译同次语义纠错中途换模型的问题：初次请求后即使模型、端点和采样设置变化，修复请求仍使用原绑定；新的编译任务使用新设置。隔离 PostgreSQL 与受控 Provider 的 6 项回归及 2 项初始化/人格链回归通过。后一组旧夹具缺少当前默认聊天模型，已在原提交的隔离副本复现并补齐夹具；真实 Provider 及跨进程多 profile 恢复尚未验收。

衣柜准备现可在库存准备、选衣和重启恢复间保留原模型绑定。旧任务缺少可核实快照时保留已完成库存并明确失败，Owner 显式恢复后才为剩余阶段选择当前配置；隔离 PostgreSQL 的恢复、隐私和迁移回归通过，真实模型选衣效果未验收。

新媒体任务的规范化、普通文本、头像提示词和视觉质量审核阶段已保存无密钥模型快照，隔离数据库中的重启恢复、视觉开关、损坏快照阻止错误交付及诊断脱敏通过。模型绑定沿用首次选择，审核提交仍使用本次视觉设置版本作围栏。旧媒体任务暂保留原选模行为并记录未冻结诊断。

新视觉身份会话的文本决策与视觉审核分别冻结模型，跨候选和重载保持绑定；视觉审核仍按本次设置版本授权。隔离数据库及受控图片/Provider 回归通过；旧会话暂保留原选模行为并记录未冻结诊断。

新的人格准备任务在首次需要模型时冻结绑定，多个 profile 和自动重试复用；Owner 显式恢复产生的新任务可使用当前配置。隔离数据库验证了中途失败、并发选择和服务实例重建后的凭据恢复；旧任务仍记录未冻结诊断。

新 Goal 评估请求在首次调用前冻结无密钥模型绑定，部分目标已提交后重试仍使用原模型；新请求使用当前配置。隔离 PostgreSQL 验证了并发选择、进程重建、损坏快照拒绝和旧请求兼容，Provider 使用受控替身。凭据恢复已防止地址变更及 A→B→A 后误用已轮换密钥，但地址和密钥分开保存时仍有中间状态，模型目录及 embedding 的既有分次读取也待处理。

每日记忆与片段摘要的来源代次已从调度延伸到 Activity 和 Core 结算，新代按首次选择保存模型绑定。未绑定代次的旧 Temporal Activity 先失败，再由调度器按当前代次重新启动。隔离数据库与受控调度回归覆盖同代重试、新代选模、纠错与并发；真实 Temporal 多版本并发仍未完成，旧版 Worker 在升级期间仍运行时不受新版围栏约束。

## B. 代码基线

- 开始 commit：`ffa808bc81cc4d800376432216f3ea679da8de30`；当前分支：`codex/fluctlight-meromero-plugin-humanization`；结束 commit 尚未确定。
- 起始时有未跟踪的 `docs/llm-agent-prompts.md`；用户明确告知已删除、不用处理，复查工作区为空，本轮未恢复。没有执行 reset、clean 或删除业务数据。
- 新增前向迁移 `0105_agent_model_sampling`、`0106_summary_correction_guard`、`0107_expression_learning`、`0108_wardrobe_model_snapshot`、`0109_media_model_snapshot`、`0110_visual_model_snapshot`、`0111_persona_model_snapshot`、`0112_goal_model_snapshot`、`0113_daily_model_snapshot` 和 `0114_segment_model_snapshot`；分别支持模型采样、摘要纠错屏障、表达候选/审核及衣柜、媒体、视觉身份、人格准备、Goal 评估、每日记忆和片段摘要模型快照。Agent/variant 覆盖使用现有模型用途表的 `agent:<id>[:<variant>]` 键。已在一次性隔离 PostgreSQL 测试，未在业务数据库执行。AEL 创建了本轮任务记录，变更的来源索引由 AEL 更新。
- 环境：Go 1.26.3 darwin/arm64、Node 24.12.0、pnpm 11.0.7。未读取或输出用户既有凭据；临时容器使用单独生成的凭据，仅保存在本机受限临时文件。未连接生产环境写入。
- 工作区 `.env` 含旧 `MTPLX_*` 和 ComfyUI 配置键，所列 URL 均为非 loopback；仅检查了配置存在及地址类别，没有读取/输出凭据值，也没有把旧文件视为当前 Go Provider 绑定或连接这些服务。

## C. 需求映射与工作包

| 编号 | 状态 | 已有落点、测试或下一步 |
| --- | --- | --- |
| R01 / T02 | IN_PROGRESS | `agent_model_routing.go`、`provider.go` 使用现有表按 variant → Agent → 默认用途解析；新人格准备、衣柜准备、新媒体提示词及质量审核、新视觉身份会话、新 Goal 评估、每日记忆和片段摘要新代、Diary/Studio 部分阶段快照已用隔离 PostgreSQL 和受控 Provider 检查。旧任务、凭据配置原子性及真实 Provider/Temporal 仍有缺口。 |
| R02 / T04 | IN_PROGRESS | 已修复低置信度 claim 状态；显式 `direct` 实验把主 Agent 正文、原生回执和认知结算放在同一事务，受控 PostgreSQL 的提交、重放与回滚通过。真实 MeroMero 对该合同的字段输出尚未通过，默认 `fact_only` 私聊仍另调 Writer，旧草稿兼容和默认切换仍需完成。 |
| R03 / T03 | IN_PROGRESS | 目录查询指令与实际 schema 对齐；全 Agent 输入契约和来源追踪未完成。 |
| R04 / T05 | IN_PROGRESS | `domain_plugins.go` 组合既有 Image/Moment 能力并声明配置、事件、依赖和在途策略；隔离 PostgreSQL 已验证设置、独立调用及部分原生回执重放；完整重启恢复和真实服务尚未验证。 |
| R05 / T06 | IN_PROGRESS | `media.go`、`media_quality.go`、`agent_result_adapter.go`；普通图片非通过不交付，同步回复事件和历史重放按已提交消息 ID 去重。部分数据库恢复回归通过；正文与媒体结果的语义一致性和真实媒体仍待验证。 |
| R06 / T07 | IN_PROGRESS | WakeUp 协议区分同主题未回应与真实新事件；React 可在前一消息被真实接收后追发，包括刷新或重新进入后追加。未知结果保留原身份，避免被旧轮终态或另一端消息覆盖；受控页面、状态恢复和本地浏览器夹具已测。数据库并发及真实模型对连续消息的综合回应未验收。 |
| R07 / T08 | IN_PROGRESS | 已接 Owner 选择的真实来源、候选持久化、审核及启停/撤销；fact-only Writer 与实验 `direct` 主 Agent 均按会话及触发词选择并在发布前复核。隔离 PostgreSQL 和受控 React 恢复通过；真实模型及 UI 到运行的完整验收未完成。 |
| R08 / T09 | IN_PROGRESS | `0106` 对已解析的纠错来源保护摘要、派生记忆、混合来源及常驻快照；每日任务在模型前和提交前检查整日来源。隔离 PostgreSQL 已覆盖旧 Confirm、同一 ref 重新冻结、调用中纠错和受控调度重开交错；未知来源、生成中的请求、可证明吸收纠错的重建、既有锁序及真实 Temporal 并发仍未闭环。 |
| R09 / T10 | TODO | 保留现有领域代码；Goal、日程、人格、NPC、Studio、日记等尚无本轮完整验收。 |
| R10 / T11 | IN_PROGRESS | 正式 React 管理页已接任务模型覆盖、恢复继承及插件开关；受控页面测试通过。数据库到真实运行的闭环、完整 Prompt/Trace 和安全重放仍未完成。 |
| R11 / T00、T01、T12 | IN_PROGRESS | 范围台账与 T01 源码入口盘点已完成；T12 完整证据矩阵及真实验收未完成。 |

## D. 当前关键链路

| 链路 | 当前事实 |
| --- | --- |
| 普通聊天 | 生产入口是 durable turn，经 `conversation_cognition`；fact-only `conversation.reply` 仍启动独立 Writer。具体模型取配置，不能仅凭代码声称一定是 Qwen。 |
| 工具聊天 | Eino/ADK 原生循环和 Capability Registry 已存在。受控请求验证目录 → 命名加载 → 执行 → ToolResult 续接；真实 MeroMero 已完成合成测试工具的调用与结果续接，但实际领域工具链尚未完整验收。 |
| 异步媒体 | 持久 intent、Provider job、资产与发布入口保留。第二次非通过普通图片现在失败；历史未完成 `retry_accepted` 只有全部原目标及附件回执证明已交付时才只补结算、不重发。隔离 PostgreSQL 的部分恢复与重放用例通过；真实渲染服务未测。 |
| 动态 | 继续使用现有 `moment.publish`、Writer 和统一发布事务；Moment 声明及停用门禁已接主要入口，真实 PostgreSQL 和在途恢复未验收。 |
| 认知结算 | `settleAgentConversationTurn` 仍消费 claims、influences 和 Goal 候选；发布成功后的本地最小终态可能没有产生这些字段，`evaluateOutputPreferenceAction` 尚无生产调用；可靠结算恢复尚未迁移。 |
| WakeUp | 复用现有调度和自治权限；协议不再把未回应当成对新话题的永久禁令，但尚未证明真实模型有新事件时正向联系及无理由时静默。 |
| 连续消息 | `conversation.ts` 保存独立追发身份，收到精确匹配的已提交用户消息后可提交追发；刷新或重新进入后也可向已确认的 pending/running 会话追加，不显式取消旧轮。未知结果保留身份并防止跨客户端乱序；Core 仍负责 superseded。受控页面和本地浏览器夹具已测，真实数据库并发及上下文合并未验证。 |
| 回复重放 | 同步处理路径对指向同一消息的多个成功回执和历史 `message_ids` 去重，保留第一次出现的消息顺序及原始回执；WakeUp 的重复抑制先于去重。持久重放已通过隔离 PostgreSQL 回归。 |
| 表达学习 | Owner 可从真实来源提取、审核和启停候选；fact-only Writer 与实验性 `direct` 主链均能在受控请求中选择匹配参考并在发布前复核。真实模型和完整页面到运行消费尚未验收。 |
| 记忆纠错 | 现有记忆修订和来源仍为权威。`0106` 对已解析来源保护摘要、派生记忆、混合来源及常驻快照；每日任务的整日来源在调用前及提交前检查，并用来源代次保护重开。该屏障不改变原消息或摘要状态，也不能证明后台重建已吸收纠错；未知旧来源、部分在途请求、既有锁序及真实 Temporal 并发仍需处理。 |

默认私聊仍存在固定认知／Writer 结构，不能回答“普通聊天永不调用 Qwen”或“正文不再隐式生成两遍”为是。一次旧链真实样本为同一 MeroMero 的主 Agent 两次加独立 Writer 一次；另一次实验 `direct` 真实样本为同一主 Agent 三次参数纠错后失败、没有发布。正常请求次数取决于工具和纠错，并非固定三次。没有新增第二套 Runtime、事实系统或模型配置源。

## E. 模型与任务清单

[Agent 盘点](docs/verification/meromero-agent-inventory-20261109.md)按当前源码列出 31 个注册 ID、已声明的任务 variant、生产上游状态、绑定来源、Prompt/输入、工具、输出及发布边界，并补列非注册 Provider、embedding、Kev、独立 Tool 和后台入口。Writer 的实际基础 variant 是 `private_reply`、`proactive` 和 `moment`，另有修复后缀；`conversation_reply_text` 是输出 schema。Prompt 内容并非均有独立版本/hash，不能用通用诊断标签冒充；各模型的实际运行配置、后台快照和真实能力仍需后续任务验证。

## F. 插件边界

`domain_plugins.go` 通过现有 Capability Registry 注册 Image 与 Moment 的实际 Tool，声明版本、配置、Agent、事件、持久任务及依赖；没有新增 Runtime。`plugins.domains` 默认两者开启，复用设置事务与版本/幂等回执，React 管理页显示开关、可选依赖及 `not_checked` 健康状态。

新 Tool 调用在既有回执查询之后检查停用；Writer 私有图片请求、Owner 图片及动态、评论/回应、Studio 新任务和视觉新候选也有门禁。纯文字动态不依赖 Image。当前进程内已开始的运行沿用快照；已有媒体 intent 可以恢复或重试，视觉 seed 已受理时可完成设定图和头像派生；重启后无已受理 intent 的工作重新检查设置，未开始的 NPC 动态机会在停用时抑制，已有有效 checkpoint 可收尾。历史读取和已提交结果保留。

定向受控请求与 React 夹具验证了目录状态、实际 schema、私有 Writer 和开关回读；隔离 PostgreSQL 已执行设置、独立 Tool 和原生回执回放专项。真实服务、全部在途重启与 E09 完整生命周期仍未验收。

## G. 验证摘要

工作目录均为仓库根。日志保留在本机临时目录，包含测试名称与错误，不包含真实对话或凭据。

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 隔离 PostgreSQL 专项 | PostgreSQL 16/pgvector 上的配置和快照 16 项、记忆来源 8 项、媒体恢复 10 个主测试通过；最初 5 项因测试夹具缺 Owner 或 SQL 参数类型失败，修正夹具后重跑通过。未连接真实 Provider/渲染器 | `/private/tmp/fluctlight-pg-configuration-final.jsonl`、`/private/tmp/fluctlight-pg-memory-final.jsonl`、`/private/tmp/fluctlight-pg-media-final.jsonl`、`/private/tmp/fluctlight-pg-fixtures-retry.jsonl` |
| 更正来源与摘要读取屏障 | 隔离 PostgreSQL 覆盖 message、fact、无前缀 fact、sequence、摘要来源及递归 Memory；包含旧 Confirm 来源丢失、不同 Owner/会话/窗口、同源重建读取和原文保留。屏障是保守读取控制，不证明后台结果已吸收纠错 | `/private/tmp/fluctlight-pg-memory-fence-red.jsonl`、`/private/tmp/fluctlight-pg-memory-confirm-red.jsonl`、`/private/tmp/fluctlight-pg-memory-parent-red.jsonl`、`/private/tmp/fluctlight-pg-memory-fence-latest.jsonl` |
| `0106` 派生来源与每日任务 | 隔离 PostgreSQL 验证旧快照、混合来源、旧 Confirm、同名 ref 新版本绑定、模型调用前/期间纠错和原始消息修改后的正常重建；旧版本升级及重复运行通过。Provider 为受控替身 | `/private/tmp/fluctlight-pg-refreeze-legacy-final.jsonl`、`/private/tmp/fluctlight-pg-daily-guard-retry.jsonl`、`/private/tmp/fluctlight-pg-resident-guard-retry.jsonl`、`/private/tmp/fluctlight-pg-migration-fixture-final.jsonl` |
| 每日任务重开 | 隔离 PostgreSQL 与受控 Temporal 客户端覆盖旧运行未完成、Describe/Execute 期间重开，及原有日程恢复；未运行真实 Temporal worker | `/private/tmp/fluctlight-pg-daily-dispatch-final.jsonl`、`/private/tmp/fluctlight-pg-daily-generation-final.jsonl` |
| 本机 MeroMero | 指定模型可见且单句真实响应。普通会话重试成功，原生 `conversation.reply` 落库一条；默认私聊的三次请求均使用同一模型，其中前两次属于主 Agent 工具循环，最后一次由独立 Writer 生成正文，未满足 T04。首次普通会话调用因 GPU 空间不足失败，内存回升后原样重试成功 | `/private/tmp/fluctlight-local-meromero-smoke.json`、`/private/tmp/fluctlight-local-meromero-turn.jsonl`、`/private/tmp/fluctlight-local-meromero-turn-retry.jsonl`、`/private/tmp/fluctlight-local-meromero-general-evidence.json`、`/private/tmp/fluctlight-local-meromero-direct-red.jsonl`、`/private/tmp/fluctlight-local-meromero-old-direct-evidence.json` |
| 本机原生工具回合 | MeroMero 真正发出 `get_test_status` ToolCall，收到合成 `ready` 结果后由同一模型继续作答；不代表真实领域工具或权限链验收 | `/private/tmp/fluctlight-local-meromero-native-loop.json` |
| 直接回复实验 | 受控原生调用与隔离 PostgreSQL 验证一次请求、正文/回执/claim 同事务提交、重放和故障回滚；最新真实模型连续三次提交不合法认知参数后终止，未发布、无独立 Writer，尚不能切换默认 | `/private/tmp/fluctlight-current-direct-count-test.log`、`/private/tmp/fluctlight-current-explicit-direct-live.log`、`/private/tmp/fluctlight-current-explicit-direct-evidence.json`；此前 GPU 内存不足：`/private/tmp/fluctlight-local-meromero-explicit-direct-retry.jsonl` |
| 表达学习来源和恢复 | 隔离 PostgreSQL 验证候选、审核、来源版本、使用开关与发布重验；React/Web 49 项通过，浏览器内存夹具验证失败确认和重新提交。真实模型及浏览器到数据库的完整链未验收 | `/private/tmp/fluctlight-current-expression-pg.log`、`/private/tmp/fluctlight-expression-recovery-web-final.log`、`/private/tmp/fluctlight-expression-recovery-typecheck-final.log` |
| PostgreSQL 全量诊断 | 发现旧迁移夹具问题并修复；图片参数错误测试及两项 Goal cadence 在起始版本同样失败。诊断运行主动中断，不能算全量通过 | `/private/tmp/fluctlight-pg-go-full-0106.jsonl`、`/private/tmp/fluctlight-pg-capture-baseline.jsonl`、`/private/tmp/fluctlight-pg-goal-baseline.jsonl` |
| `node --test apps/web/test/configuration-structured-fields.test.mjs apps/web/test/ux-settings-support.test.mjs apps/web/test/runtime-config.test.mjs` | 基线 11 通过 | `/private/tmp/fluctlight-meromero-baseline-web.log` |
| `go -C apps/core-go test -json ./... -count=1`，清除数据库/外部服务变量 | 当前插件改动后 18 个有测试包通过；按事件计数 2,987 通过、927 跳过；两项新增插件 PostgreSQL 测试跳过 | `/private/tmp/fluctlight-meromero-go-plugins-final.jsonl` |
| 插件页面、目录和错误提示 | Generate/Stream 受控工具 schema、Writer 和目录用例通过；React 页面使用内存配置服务完成保存、重载及明确拒绝恢复。本地浏览器人工观察该夹具，图片关闭后动态仍开启；不代表真实数据库或模型消费 | `domain_plugins_test.go`、`apps/web/test/react-flows.test.mjs`；没有保存或提交图片 |
| 插件后的 Web 回归与构建 | 相关 React/设置测试 47 项、客户端测试 44 项通过，Web/客户端 typecheck 通过；仓库外临时 Vite 构建、产物夹具标记检查及 React graph 检查通过 | `/private/tmp/fluctlight-meromero-web-plugins.log`、`/private/tmp/fluctlight-meromero-plugin-build.log` |
| 连续消息与聊天 UI | 62 项聊天、恢复及相关 React 测试通过；覆盖接收先后、旧流/取消隔离、刷新及存储失败恢复、跨客户端顺序和同版草稿去重。本地浏览器检查内存夹具，未连接真实数据库或模型 | `/private/tmp/fluctlight-meromero-chat-followups-final.log`；未保存截图 |
| 连续消息变更后的前端构建 | 仓库外临时 Vite 构建、产物夹具标记检查及 React graph 检查通过 | `/private/tmp/fluctlight-meromero-chat-build.log` |
| Diary 阶段模型快照 | 素材和表达两个正式 Agent 的受控物理请求保留模型、采样且不包含密钥；显式重试仅复用完成阶段的纯测试通过。三个新增 PostgreSQL 恢复用例及原完成阶段恢复用例跳过 | `diary_model_snapshot_test.go`、`diary_model_snapshot_integration_test.go` |
| Studio 阶段模型快照 | 助手、文字预览、角色定义准备及按需媒体主体规划的受控物理请求保留模型和采样；自动／显式重试及旧运行兼容的 PostgreSQL 用例跳过 | `studio_model_snapshot_test.go`、`studio_model_snapshot_integration_test.go` |
| Diary 快照后的全量 Go 回归 | 18 个有测试包通过，按 Go 测试事件（含子测试）计 2,995 通过、929 跳过；跳过不是持久化或真实服务通过 | `/private/tmp/fluctlight-meromero-go-diary-snapshot.jsonl` |
| Studio 快照后的全量 Go 回归 | 18 个有测试包通过，按 Go 测试事件（含子测试）计 2,999 通过、933 跳过；包括新增数据库用例的跳过 | `/private/tmp/fluctlight-meromero-go-workflow-snapshots.jsonl` |
| 当前全量 Go 与相关 Web 回归 | 18 个有测试的 Go 包通过，按事件计 3,000 通过、934 跳过；聊天、设置及 React 相关 Web 测试 93 项通过，Web typecheck 与仓库外构建通过 | `/private/tmp/fluctlight-meromero-go-all-current.jsonl`、`/private/tmp/fluctlight-meromero-web-current.log`、`/private/tmp/fluctlight-meromero-current-build.log` |
| 表达提取及回复重放定向检查 | 内部表达提取器的受控 HTTP、schema 与来源校验通过；重复回执和历史 ID 在修复前复现失败、修复后通过。实际人类来源与审核链尚未接入，PostgreSQL 重放和真实模型未验收 | `/private/tmp/fluctlight-reply-dedup-red.log`、`/private/tmp/fluctlight-reply-expression-focused.log` |
| 表达提取和回复去重后的 Go 回归 | 18 个有测试包通过；按事件计 3,035 通过、948 跳过。新增 PostgreSQL 重放测试因未配置隔离数据库而跳过 | `/private/tmp/fluctlight-expression-reply-go-final.jsonl` |
| 图片工具空白参数恢复 | 受控原生循环先复现整轮失败；修复后空白参数返回 `needs_input`，后续有效意图和此前已登记意图均可保留。不是实际生图或媒体正文一致性验收 | `/private/tmp/fluctlight-image-whitespace-red.log`、`/private/tmp/fluctlight-image-whitespace-green.log` |
| 纠错及原生工具失败边界 | 输出一致性纠错的 503 和工具依赖的 503/504 不再被用作换模型理由；模型自身超时允许显式回退，整体请求超时不回退。同一原生调用重放保留原始终止错误，普通无执行错误的结果仍可继续。均为受控测试，真实 Provider 尚未验收 | `/private/tmp/fluctlight-correction-fallback-red.log`、`/private/tmp/fluctlight-tool-fallback-red.log`、`/private/tmp/fluctlight-native-replay-failure-red.log`、`/private/tmp/fluctlight-timeout-fallback-red.log`、`/private/tmp/fluctlight-failure-replay-final2.log` |
| 最新 Go 全量回归 | 18 个有测试包通过；按 Go 事件计数（含子测试及包事件）3,047 通过、948 跳过，跳过不计为真实数据库或服务通过 | `/private/tmp/fluctlight-meromero-latest-go.jsonl` |
| 超时分类和工具回执并发检查后 | 物理模型超时与整体回复超时的受控场景、原生回执并发的 Go race 检查通过；最新全量 Go 18 个有测试包通过，按事件计数 3,048 通过、948 跳过 | `/private/tmp/fluctlight-failure-replay-final2.log`、`/private/tmp/fluctlight-native-trace-race.log`、`/private/tmp/fluctlight-meromero-go-final-current.jsonl` |
| claim 状态调整前的 Go 回归 | 18 个有测试包通过；按 Go 事件计数（含子测试及包事件）3,049 通过、948 跳过。PostgreSQL、真实模型与媒体相关跳过项仍未验收 | `/private/tmp/fluctlight-meromero-final-verified.jsonl` |
| claim 不确定性结算 | 受控事务测试先复现低置信度 claim 被写为 active，修复后保留 uncertain 与原有效期；真实 PostgreSQL 落库和重放用例跳过，历史错误记录不自动重写。相同 repetition key 的新证据仍可能被冲突忽略 | `/private/tmp/fluctlight-claim-uncertainty-red.log`、`/private/tmp/fluctlight-claim-uncertainty-final.log` |
| 当前 Go 全量回归 | 18 个有测试包通过；按 Go 事件计数（含子测试及包事件）3,050 通过、949 跳过。隔离 PostgreSQL 和真实服务相关跳过项仍未验收 | `/private/tmp/fluctlight-meromero-claim-go-current.jsonl` |
| `go -C apps/core-go test -json ./... -count=1`，清除真实数据库/外部服务变量并使用本机回环 | 18 个有测试的包通过；按 Go 测试事件计数（含子测试）2,944 通过、919 跳过、0 失败。867 条跳过输出明确为缺隔离 PostgreSQL，23 条为未启用真实 Provider | `/private/tmp/fluctlight-meromero-go-current.jsonl`；首次沙箱运行因监听本机端口受限退出 1：`/private/tmp/fluctlight-meromero-go-all.log` |
| 同上，包含最新模型路由与视觉快照修复 | 18 个有测试包通过；2,955 通过、921 跳过、0 失败，按 Go 测试事件（含子测试）计数。跳过输出中 876 条提到 PostgreSQL 配置、24 条提到真实 Provider 开关 | `/private/tmp/fluctlight-meromero-go-routing.jsonl` |
| 同上，增加旧消息覆盖回归并撤回有锁风险的记忆扩展 | 当前代码 18 个有测试包通过；2,956 通过、921 跳过、0 失败，按事件计数。跳过输出中 876 条提到 PostgreSQL 配置、24 条提到真实 Provider 开关 | `/private/tmp/fluctlight-meromero-go-post-review.jsonl`；此前尝试的日志 `/private/tmp/fluctlight-meromero-go-continuity-final.jsonl` 不代表当前代码 |
| 同上，包含 Writer 任务超时修复 | 18 个有测试包通过；2,958 通过、921 跳过、0 失败，按事件计数。876 条跳过输出提到 PostgreSQL 配置、24 条提到真实 Provider 开关 | `/private/tmp/fluctlight-meromero-go-latest.jsonl` |
| 同上，包含媒体规范化同次纠错快照 | 当前代码 18 个有测试包通过；2,960 通过、923 跳过、0 失败，按事件计数。878 条跳过输出提到 PostgreSQL 配置、24 条提到真实 Provider 开关 | `/private/tmp/fluctlight-meromero-go-current-final.jsonl` |
| 同上，包含采样配置与迁移代码 | 18 个有测试包通过；2,970 通过、925 跳过、0 失败，按事件计数。880 条跳过输出提到 PostgreSQL 配置、24 条提到真实 Provider 开关 | `/private/tmp/fluctlight-meromero-go-sampling.jsonl` |
| 同上，包含采样 HTTP 边界与原生工具证据修正 | 当前代码 18 个有测试包通过；2,976 通过、925 跳过、0 失败，按事件计数。880 条跳过输出提到 PostgreSQL 配置、24 条提到真实 Provider 开关 | `/private/tmp/fluctlight-meromero-go-native-evidence.jsonl` |
| `go -C apps/core-go test -json ./internal/core ./internal/workflow -count=1`，同样隔离配置 | 最新两个受影响包退出 0；按 Go 测试事件计数（含子测试）2,234 通过、842 跳过、0 失败。数据库与真实 Provider 跳过项不能算通过 | `/private/tmp/fluctlight-meromero-core-latest.jsonl` |
| 受控目录/工具循环定向测试 | Generate/Stream 各 1 次四轮物理请求路径通过，执行的是测试 Provider/工具 | `/private/tmp/fluctlight-meromero-catalog.log` |
| 原生工具证据边界 | 生产路径静态复核未发现正文转 ToolCall；移除真实模型测试助手中的正文回退并验证标准原生字段，未执行真实模型 | `/private/tmp/fluctlight-meromero-native-evidence-final.log` |
| 图片质量策略回归 | 修改前两项断言失败；修改后单元、受控图片请求和工作流检查通过。数据库用例跳过 | `/private/tmp/fluctlight-meromero-red.log`、`/private/tmp/fluctlight-meromero-core-focused.log`、`/private/tmp/fluctlight-meromero-workflow.log`、`/private/tmp/fluctlight-meromero-recovery-tests.log` |
| 旧模型回退策略 | 修复前受控图片声明失败后调用了第二模型；修复后拒绝及核验失败不触发回退，显式技术回退仍通过 | `/private/tmp/fluctlight-meromero-fallback-red.log`、`/private/tmp/fluctlight-meromero-fallback-full.log` |
| 组装与模型绑定 | 9 项定向测试通过、1 项 PostgreSQL 测试跳过；受控物理请求检查同次运行采用原快照、任务标识、来源版本、结构化 helper 转交、冲突拒绝和凭据不序列化。旧草稿的凭据用途未知且端点地址变化时安全失败；数据库配置更新及恢复重读仍未测 | `/private/tmp/fluctlight-meromero-model-snapshot.log` |
| WakeUp 协议 | 两个受控物理请求场景及协议边界测试通过；脚本响应不证明真实自主行为 | `/private/tmp/fluctlight-meromero-wakeup-protocol.log` |
| 任务变体路由 | 受控物理请求检查嵌套 Agent 不复用父快照、视觉快照及 Moment/私聊来源隔离；数据库中的配置保存和真实模型请求仍跳过 | `/private/tmp/fluctlight-meromero-route-isolation-red.log`、`/private/tmp/fluctlight-meromero-route-isolation-green.log`、`/private/tmp/fluctlight-meromero-go-routing.jsonl` |
| Writer 任务物理超时 | 修改前 Agent 和 variant 的 25 秒超时均被改成全局 300 秒；修复后受控请求使用任务超时。私聊/主动私聊/动态另受回复总时限约束，日记和 Studio 受各自工作流时限约束 | `/private/tmp/fluctlight-meromero-writer-timeout-red.log`、`/private/tmp/fluctlight-meromero-writer-timeout-green.log` |
| 模型采样配置 | 受控 Writer 与结构化 Agent 物理请求验证 temperature=0 及 Top P；浏览器 HTTP 边界保留省略/null/零值，正式 React 表单与旧草稿、迁移端点重绑测试通过。数据库升级和配置保存/回读两项 SKIP | `/private/tmp/fluctlight-meromero-sampling-wire.log`、`/private/tmp/fluctlight-meromero-sampling-http.log`、`/private/tmp/fluctlight-meromero-sampling-go-final.log`、`/private/tmp/fluctlight-meromero-sampling-web-post-review.log`；客户端 44 项 `/private/tmp/fluctlight-meromero-sampling-client-test-loopback.log`，首次沙箱 IPC 失败保留在 `/private/tmp/fluctlight-meromero-sampling-client-test.log` |
| 媒体规范化纠错 | 受控 Provider 的首次规范化及纠错使用同一进程内绑定，非法输入先于模型读取且两次尝试之间取消不再发请求；配置在两次调用间变化及失败诊断 PostgreSQL 用例跳过，跨进程恢复未验证 | `/private/tmp/fluctlight-meromero-media-normalization-verified.log` |
| 正式 React 任务模型管理 | 受控页面模拟保存覆盖、恢复继承、回执丢失后恢复、更新默认模型及保留并发输入；不代表真实数据库或模型消费 | `/private/tmp/fluctlight-meromero-react-recovery.log` |
| 浏览器本地夹具 | 实际打开 React 管理页并选择变体、恢复继承，验证回读；所选卡片调整到选择器之后，并在窄视口核对温度 `0` 与 Top P `0.75` 的呈现。夹具使用内存数据，非真实后端验收；没有保存或提交截图 | `/private/tmp/fluctlight-meromero-react-post-visual.log`、`/private/tmp/fluctlight-meromero-web-post-visual.log` |
| Web 设置与正式 React 相关回归 | 56 项通过；类型检查、仓库外临时目录 Vite 构建及产物标记/React graph 检查通过，未覆盖现有 dist | `/private/tmp/fluctlight-meromero-web-sampling-full.log`、`/private/tmp/fluctlight-meromero-web-sampling-final-typecheck.log`、`/private/tmp/fluctlight-meromero-vite-sampling.log`、`/private/tmp/fluctlight-meromero-product-markers-sampling.log`、`/private/tmp/fluctlight-meromero-react-graph-sampling.log` |
| 旧消息覆盖 | 修复前受控事务样本失败；修复后通过并保留已提交副作用。记忆来源的同批尝试随后因潜在死锁被撤回 | `/private/tmp/fluctlight-meromero-continuity-red.log`、`/private/tmp/fluctlight-meromero-continuity-final.log` |
| Prompt、Goal、衣柜和无人主体契约 | 11 个顶层测试通过（含 32 个子测试）；覆盖 30 Agent 协议、来源不同事实、目标作用域、衣柜任务投影及受控无人主体。不是数据库或真实图片验收 | `/private/tmp/fluctlight-meromero-domain-contracts.log` |
| `pnpm --filter @fluctlight/web typecheck` | 退出 0 | `/private/tmp/fluctlight-meromero-typecheck.log` |
| `pnpm exec vite build --outDir /private/tmp/fluctlight-meromero-vite-final --emptyOutDir`，并对临时产物运行仓库 product-build 检查和 React graph 检查 | 构建与两项检查退出 0；使用临时输出，未覆盖现有 `apps/web/dist` | `/private/tmp/fluctlight-meromero-build-final.log`、`/private/tmp/fluctlight-meromero-react-stack.log` |
| 媒体质量和恢复 UI 定向断言 | 8 通过；使用隔离 UI 夹具，非真实后端端到端 | `/private/tmp/fluctlight-meromero-ui-quality-recovery.log` |
| 历史交付、双目标部分交付和 Studio 恢复 | 测例已编译但因缺隔离 PostgreSQL 跳过，未验证实际 SQL/事务 | `/private/tmp/fluctlight-meromero-legacy-recovery.log` |
| 完整诊断详情测试 | 14 通过、5 失败；相同 5 项在起始 commit 隔离副本重现 | `/private/tmp/fluctlight-meromero-ui.log`、`/private/tmp/fluctlight-meromero-ui-baseline.log` |

## H. 典型证据与缺口

以下均为人工构造的受控夹具，不是用户聊天、真实 MeroMero 或真实图片：

- 质量状态机输入：第二次检查 `retry` 或 `reject`。旧结果为 `accept_after_retry`，修复后为 `stop`；真实 PostgreSQL 写入与交付测试仍跳过。
- 回复回退输入：Writer 两次返回无实际附件的 `（发送图片）`。修复前记录 `writer calls=2, fallback calls=1`，第二模型返回“**不应由另一个模型重新决定**”；修复后 `writer calls=2, fallback calls=0` 并返回错误。另一个样本“我不想这样做。”原样返回且没有回退。
- 原生工具输入依次为 `capability.catalog({"names":[]})`、命名加载 `habit.inspect`、调用该测试工具；第四次受控物理请求包含测试结果 `hidden-service-result`。Generate 与 Stream 各执行一次完整夹具。
- 模型快照夹具在组装结果中冻结 `old-model`，实际受控 Provider 请求的 model 仍为 `old-model`；故意提供不同冻结值时在 Provider I/O 前报 `formal_agent_prompt_binding_conflict`。测试同时检查私有测试凭据不进入 Prompt JSON 和诊断。
- 嵌套任务夹具在会话认知冻结 `parent-model` 后启动日程重排并显式绑定 `child-model`；修复前报快照冲突，修复后受控物理请求实际使用 `child-model`，父上下文快照保持不变。该夹具没有连接真实 Provider。

以下要求的完整真实证据仍 **未取得**：无 Qwen 的默认私聊、完整的 MeroMero 原生发现/执行/续接循环、真实图片成功及失败、无人物实图、表达候选使用与停用、全链路记忆纠错后检索、旧认知结算迁移。本机 `mlx-serve` 已列出并实际响应用户指定的 `llmfan46/G4-MeroMero-26B-A4B-it-uncensored-heretic-4bit-mxfp4`；内存回升后，普通会话真实调用原生 `conversation.reply` 并落库成功。默认私聊的真实回归仍产生独立 Writer，未满足 T04。隔离数据库和受控请求另验证该 ID 的配置传递。此前携带 `.env` 密钥查询非回环地址被自动审批拒绝，随后使用无凭据的本机回环接口。隔离 PostgreSQL 中插件停用和部分记忆纠错读取已执行，不代表完整端到端验收。

## I. 差异和决定

1. 旧媒体合同允许第二张 `retry/reject` 以 `retry_accepted` 自动交付；用户明确要求硬失败不得因重试耗尽自动成功。现代码及媒体合同改为普通媒体失败；视觉身份私有候选保留显式审核，历史记录不改写。旧任务有真实交付证据时只结算，不重新发送；没有交付证据时阻止自动发布。
2. 现有 `ActionOutcome` 是终态。已经结算失败的图片不能在同一动作身份上覆盖为成功，因此未交付的旧候选不能通过恢复改变结论，关联终态动作的重新生成需要新请求。已交付旧候选可在原动作未冲突时只补结算；完整跨尝试恢复方案和数据库验证尚未完成。
3. 原目录描述要求 `{}`，实际模型侧 strict schema 要求 `names`；模型侧改为 `{"names":[]}`，独立调用继续接受 `{}`。没有改变权限或执行入口。
4. 当前任务规模及真实环境检查均未完成；此为明确偏离完整交付的未完成状态，不把部分修复或测试通过算作 R01–R11 的整体验收。
5. 参考了 MaiBot 固定 commit `e5391507531706a8bfce011d55ed94628039f2de` 的插件注册/回复扩展及表达候选/审核文件，具体路径和局限见 [实施台账](docs/verification/meromero-first-ledger-20261109.md#maibot-参考)。借鉴边界和状态概念，未复制其代码、IPC 或插件市场。
6. 旧报告中的摘要迟到结算样本使用了仅存在于 `conversation_summary_legacy_test.go` 的辅助函数，不能证明当前生产累计摘要或分段 worker 的并发保护。此前有锁反转风险的写入扩展已撤回；本次新增保守读取屏障并在隔离 PostgreSQL 核对来源与重建读取，但生成结算并发和纠错覆盖水位仍未完成。
7. 媒体正文核验尚不完整：媒体失败后的自然语言可能仍声称已发送；真实 ready 附件可能与否认或拒绝正文同时发布；初次原生生图请求和拒绝正文也可能同时保留。已有状态与附件校验不等于正文语义通过。
8. 插件停用采用明确的在途边界：本进程已准入运行可完成，已有媒体 intent 可恢复，未受理新工作在恢复时重新检查。视觉 seed 后派生及既有失败媒体重试属于续办；未开始的 NPC 动态机会停用时抑制。隔离 PostgreSQL 已验证设置和部分回执重放，完整进程重启行为尚未实测。
9. 旧 UI 合同让追发等到上轮完整结算；本轮用户要求连续消息和 superseded，现改为仅在精确用户消息的服务端接收回执后提前提交。定向状态查询仍只读；真实数据库并发与模型综合回应尚未验证。
10. Diary 升级前已运行且缺少原模型快照的未完成阶段无法证明旧绑定，现明确失败并保留材料，需显式重试；新 attempt 对未完成阶段使用当时配置。隔离 PostgreSQL 专项已验证阶段绑定和旧任务失败策略，真实 Provider 未测。
11. Studio 三种文本阶段采用同类旧任务兼容策略；自动重试继承未完成阶段的冻结绑定，用户显式新 attempt 可使用更新的配置。隔离 PostgreSQL 专项已通过，真实 Provider 和完整媒体工作流未测。

## J. 风险与回滚

- 隔离 PostgreSQL 中的配置、部分恢复和来源读取已验证，但真实模型语义、真实图片、完整事务/重放与 UI 端到端未验证。Memory 纠错后持久摘要仍可能保留旧内容；已知来源的读取屏障不会证明后台重建吸收纠错，也不保护所有在途请求或未知来源。图片正文仍可能与真实媒体状态冲突。
- Diary 素材／表达和 Studio 文本及按需媒体主体规划的阶段绑定快照通过了隔离数据库恢复用例，Provider 使用受控替身；旧 running attempt 无快照时明确失败。后续独立媒体工作流及其他后台阶段的跨进程快照仍需验证。
- 旧 `retry_accepted` 数据保留原状，只改变展示措辞。没有执行业务数据迁移或批量修改。
- `0105` 至 `0114` 只在测试用临时数据库执行，未改业务数据库。升级业务库后回退到不认识新 head 的代码不能只替换可执行文件；需要先停止写入、备份，并由获授权的操作者核对迁移记录、既有字段、来源函数及在途任务。回退 `0114`/`0113`/`0112`/`0111`/`0110`/`0109`/`0108` 会失去相应片段摘要/每日记忆/Goal 评估/人格准备/视觉身份/媒体/衣柜模型快照保障，回退 `0107` 会影响表达候选及来源版本消费，回退 `0106` 会移除纠错读取屏障，回退到 `0104` 还会失去 Agent 采样覆盖消费；不能通过重建数据库、删除用户数据或重发已完成动作处理。

## 隔离环境恢复验证

在专用的可写 PostgreSQL 测试实例中，通过安全方式设置 `GO_CORE_TEST_DATABASE_URL`；测试夹具会创建并清理自己的临时测试数据库，需要该实例授予相应权限。不要把连接密码写入报告。随后从仓库根目录运行：

```sh
go -C apps/core-go test -v ./internal/migrations -run 'TestAgentSamplingUpgradePreservesBindingsAndLaterSettings' -count=1
go -C apps/core-go test -v ./internal/core -run 'TestAgentModelBindingSaveSelectAndFrozenRun|TestAgentModelConfigurationSavesReadsRunsAndClears|TestAgentSamplingConfigurationSavesRunsAndClearsPerField|TestMediaNormalizationCorrectionDoesNotRebindAfterConfigurationChange|TestMediaNormalizationAssignmentFailureKeepsDiagnostics|TestMediaQualityRetryPersistsFeedbackAndRejectsSecondCandidate|TestRejectedMediaRecoveryDoesNotReplayOrReuseTerminalAction|TestLegacyNonPassRecoveryUsesCommittedDeliveryWithoutRepublishing|TestLegacyNonPassPartialMultiTargetDeliveryDoesNotBecomeComplete' -count=1
go -C apps/core-go test -v ./internal/core -run 'TestDomainPluginPostgresSettingsAdmissionAndReplay|TestDomainPluginPostgresNativeAgentReceiptReplay' -count=1
go -C apps/core-go test -v ./internal/core -run 'TestDiaryPostgresRestoresFrozenExpressionBinding|TestDiaryPostgresRestoresFrozenMaterialBinding|TestDiaryPostgresLegacyRunningWithoutSnapshotFailsExplicitly|TestDiaryTerminalWorkflowRecoveryReusesCompletedStagesWithoutProvider' -count=1
go -C apps/core-go test -v ./internal/core -run 'TestStudioPostgresAutomaticRetryKeepsFrozenModel|TestStudioPostgresMediaPlanningAutomaticRetryKeepsFrozenModel|TestStudioPostgresExplicitRetryCanSelectNewModel|TestStudioPostgresLegacyRunningWithoutSnapshotFailsExplicitly|TestStudioTimeoutCreatesNewAttemptsWithBackoffThenExhausts' -count=1
```

这些用例应实际显示 PASS、无数据库 SKIP；它们仍不能证明真实 MeroMero、视觉理解或渲染质量。记忆纠错锁顺序、分段迟到结算与带纠错覆盖的重建尚无完整实现和测试，不能通过运行上述命令宣称通过。

## E01–E18 当前覆盖

| 验收 | 当前状态 |
| --- | --- |
| E01 模型路由 | 未完成；无真实请求证据 |
| E02 上下文隔离 | 部分 Prompt/目标/衣柜投影单元及受控请求通过；完整场景未完成 |
| E03 原生工具循环 | 受控 Provider 与真实 MeroMero 的合成测试工具续接通过；真实领域工具发现、执行和续接尚未完整验收 |
| E04–E05 无人图片、换衣自拍 | 无人主体投影单元测试通过；未做真实图片/领域链验收 |
| E06 缺失物品 | 未验收 |
| E07–E08 异步、幂等与恢复 | 局部代码和受控测试；数据库及真实服务未验证 |
| E09 插件启停 | 受控目录、物理 Tool schema、Writer 和 React 夹具部分通过；隔离 PostgreSQL 设置、独立 Tool 和原生回执重放通过，NPC 在途和真实服务恢复未验收 |
| E10 表达学习 | 真实来源、审核、启停，以及 fact-only 和 `direct` 发布复核已实现并通过隔离数据库/受控请求；真实模型及 UI 到运行闭环尚未验收 |
| E11 记忆纠错 | 隔离 PostgreSQL 验证了已解析来源、派生记忆、旧快照、确认兼容、每日调用中纠错和受控重开交错；未知来源、已发送请求、带纠错覆盖的重建及真实 Temporal 并发仍未完成 |
| E12–E13 主动交流、人格 | 未做多次真实模型验收 |
| E14 既有业务 | 未做本轮完整验收 |
| E15 管理闭环 | React 页面和受控配置服务夹具部分通过；真实持久化、刷新和下一次运行消费未验收 |
| E16 发布与质量 | 非通过策略的单元/受控工作流通过；真实附件及正文语义未验证 |
| E17 未知与来源 | 未完成全场景验收 |
| E18 实际能力 | 未提供真实视觉、出图及任务能力证据 |

## 最终自检（当前为实施中快照）

| 项 | 当前回答及依据 |
| --- | --- |
| 1. 只更换模型？ | 否；已修改质量、恢复、来源失效和工具目录，但整轮仍未完成。 |
| 2. 仍固定依赖 Qwen？ | 主认知默认已改为继承 `conversation_reply`，显式覆盖仍可选择其他模型；但固定认知/Writer 链尚未移除，真实无 Qwen 聊天未验证。 |
| 3. 合并成万能 Agent？ | 未实施此类合并；31 个现有 Agent 清单见盘点。 |
| 4. 仅插件空壳？ | 已通过插件声明注册现有实际 Tool 并接入领域门禁；完整数据库及在途生命周期尚未验证。 |
| 5. 用 HTTP/JSON 代替模型能力？ | 否；受控请求单独标注，真实模型和视觉均未通过验收。 |
| 6. 丢掉认知结算、Goal 或人格切换？ | 尚不能确认整体无回退；旧代码仍在，但发布终态可能遗漏认知字段，Owner 人格切换有现存生产路径。 |
| 7. 所有内部任务都注入完整人格/状态？ | 尚未完成全量核验；部分 Prompt/Goal/衣柜投影检查通过。 |
| 8. 表达学习、WakeUp、记忆纠错有遗漏？ | 有；表达学习已接入实验 `direct`，但仍缺真实全链路验收；WakeUp 和记忆治理未完成全场景验收。 |
| 9. 仅靠 Prompt/正则保证执行与发布？ | 尚未达到完整目标；新增图片质量状态及真实附件证据检查在代码侧，但正文语义一致性仍不完整。 |
| 10. 关闭插件只是隐藏按钮？ | 已加后端 Tool 和主要业务入口门禁；受控测试通过，PostgreSQL 上的独立调用及恢复尚未验证。 |
| 11. 独立 Tool 绕过授权？ | 现有 `ExecuteTool` 有正式权限与领域入口；全部 Tool 的本轮独立调用验收尚未完成。 |
| 12. 新增第二套 Runtime/调度/事实系统？ | 未新增；本轮快照是进程内字段，未引入新运行时或数据库事实系统。 |
| 13. 删除括号动作或压平人格？ | 未删除；现有 v8 协议允许按需使用。多人格真实输出仍未验收。 |
| 14. 为测试强迫答应或降低业务条件？ | 未作此类修改；受控拒绝样本保持拒绝，Goal/日程完整回归尚待执行。 |
| 15. 声称未执行的真实测试已完成？ | 否；本报告将 PostgreSQL、真实模型、真实图片和端到端未执行项明确列出。 |
