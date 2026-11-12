# MeroMero-first 改造复核报告

> 复核快照：2026-11-12；主要受检代码提交 `42b43abf`，其后 `45489e84` 只新增一项回退边界测试并已单独复测。**本轮尚未完成，也未达到替换旧主聊天链的条件。** 本报告替换早期实施中快照；后续代码如有变化，应重新核对对应证据。工作过程和历史失败保留在 [实施台账](docs/verification/meromero-first-ledger-20261109.md)，正式入口盘点见 [Agent 清单](docs/verification/meromero-agent-inventory-20261109.md)。

## A. 当前结论

- **主聊天仍未达到原目标。** 当前无保存配置时默认 `fact_only`，该路径仍有独立 Writer；`direct` 可显式选择，但存在未调用媒体仍可发布“照片已发送”的缺口，真实 MeroMero 私聊也尚未通过。见 `apps/core-go/internal/core/reply_vision_settings.go:43`、`reply_provider.go:212` 及台账第 108 行。
- T01 的正式入口盘点已完成。T02 在 AEL 中标记完成，但真实 Qwen 覆盖、真实 Temporal 和旧任务原绑定等并未全部验收，不能据该状态声称 R01 的所有原始要求均已证明。T03–T11 在 AEL 中仍为待修复；T12 的这份矩阵是进行中的验收和交付材料，不代表 T12 已通过。
- 快照后的 T03 增量：Moment 选中的 Active Memory 不再把 Core Actor ID 直接传给 Writer，而是输出关联人物别名或未知，且不把关联者断言为内容主体。隔离 PostgreSQL 和受控 Writer 验证了实际 Moment 准备路径；外貌与其他证据类型仍未完成，详细记录见 [T03 专项](docs/verification/meromero-t03-context-isolation-20261111.md)。下文的全量和矩阵统计仍对应各自标明的受检提交，不自动延伸到此增量。
- 快照后的 T06 增量：能力目录与原生调用区分未安装、运行范围无权限、插件停用和安装状态未知，并报告当前已加载的披露窗口；正式 Agent 按本次任务冻结可执行工具集合，分阶段 Agent 不能执行其他阶段工具。隔离数据库及受控原生循环的 21 个顶层专项通过，验证权限、已提交回执重放和无重复动态，见 [脱敏证据](evidence/meromero-t06-catalog-verification-20261112.txt)；正文与媒体语义一致性仍未解决。
- 快照后的 T11 增量：Moment 管理卡片已接 Owner 授权的只读健康检查，受控 React、进程内浏览器路由到隔离 PostgreSQL 及客户端合同测试通过；检查只覆盖当前进程注册和列表读取，不等于发布、模型或事件投递可用，也不是完整真实浏览器端到端。[本批证据](evidence/meromero-t11-moment-health-20261112.txt)。
- 已实施并局部验证：模型按 Agent/variant 路由及多类异步快照；显式 `direct` 的原子消息/工具/认知结算；Image/Moment 的声明和主要运行门禁；连续消息栅栏；表达候选审核启停；记忆来源屏障；诊断导出隐私投影、物理重试记录和图片插件连接检查。各项的边界和失败见下表。
- 本次基于干净提交副本完成 Go 全量回归、14 个隔离 PostgreSQL/受控工具专项、8 个受控 React 测试及 Web 类型检查。**全量 Go 有 1,073 个测试事件跳过**，不能把跳过当成通过。真实模型历史上有合成聊天和测试工具的有限证据，真实媒体、真实视觉及浏览器到运行消费的完整链仍未验收。
- 用户要求非阻断问题先登记、不顺手扩展。已登记的例子包括 `provider-endpoint-credential-atomic-save`、`media-normalization-quality-retry-feedback`、`goal-evaluation-cadence-domain-failures` 和本次发现的 `reply-wear-fallback-correction-regression`。四组 Goal/活动测试在起始版本也失败；换衣回退纠错的两个子场景则在起始版本通过、当前版本失败，原因仍未确定。

## B. 基线与变更

| 项目 | 事实 |
| --- | --- |
| 起始提交 / 本次受检提交 | `ffa808bc81cc4d800376432216f3ea679da8de30` / `42b43abf`；报告自身及其后提交可用 `git log -1 -- review-for-chatgpt.md` 查询。 |
| 分支 | `codex/fluctlight-meromero-plugin-humanization`；本报告不表示已推送或发布。 |
| 受检范围 | 起始提交到受检提交共有 108 个提交，`git diff --stat` 为 397 个文件、31,384 行新增及 5,290 行删除；包括测试、文档和 AEL 记录。该统计不是功能完成度。 |
| 原有文件与草稿 | 开始时未跟踪的 `docs/llm-agent-prompts.md` 已按用户说明不恢复。本次 Go 验证使用 `git archive` 的干净提交副本；工作树另有四个已登记媒体重试 bug 的草稿，未纳入本批提交或受检源码。 |
| 数据 | PostgreSQL 16/pgvector 使用仅监听本机的临时容器；测试自行创建并清理隔离数据库。未对真实业务数据库执行写入，也没有在报告中保存连接凭据。 |
| 迁移 | 新增 `0105_agent_model_sampling` 至 `0116_runtime_summary_snapshot`：采样、摘要纠错屏障、表达学习，以及衣柜、媒体、视觉身份、人格、Goal、每日/片段/累计摘要和 embedding 的模型快照。当前代码 head 为 `0116_runtime_summary_snapshot`；只在隔离库验证过，不代表业务库已迁移。 |
| 配置 | 模型覆盖复用 `model_roles` 的 Agent/variant 绑定；插件复用 `plugins.domains` 设置。没有把本机指定的 MeroMero ID 写死到业务逻辑。 |

## C. R/T 需求到代码的映射

状态中的“部分”指有代码和局部证据，**不是整项实现或验收完成**。

| 需求 / 任务 | 当前状态 | 关键落点与剩余工作 |
| --- | --- | --- |
| T00 范围基线 | 已建立 | [实施台账](docs/verification/meromero-first-ledger-20261109.md)记录基线、限制、失败和分批结果。 |
| T01 入口盘点 | 已完成盘点 | [Agent 清单](docs/verification/meromero-agent-inventory-20261109.md)列出 31 个正式 Agent、共享变体、非正式调用和工具入口；盘点不等于真实能力通过。 |
| R01 / T02 模型路由 | 部分验收；AEL 标记完成 | `agent_model_routing.go`、`provider.go`、`agent_model_routing_test.go:246`；隔离数据库验证单任务覆盖及运行消费，真实 Qwen、真实 Temporal 和旧任务原绑定仍有限制。对应 E01。 |
| R03 / T03 Prompt/Context | 部分 | `prompt_context_assembler.go`、`prompt_slots.go`；局部任务的输入隔离已测，全 Agent/variant 的输入契约及真实语义未完成。对应 E02。 |
| R02 / T04 单主聊天 | **未完成** | `reply_vision_settings.go:43`、`adk_conversation_runtime.go:615`、`cognition_growth.go:225`、`goal_sources.go:305`；显式 direct 的局部原子结算通过，但默认仍是 fact-only + Writer，真实 direct 私聊和实时流未通过。对应 E01、E03、E08、E16。 |
| R04 / T05 插件 | 部分 | `domain_plugins.go:111`、`domain_plugins_integration_test.go:14`；已接设置、独立调用和主要门禁，真实渲染、所有在途重启及完整照片/创作边界未验收。对应 E09。 |
| R05 / T06 异步与发布 | **未完成** | `reply_media.go:13`、`media_quality.go:365`；取消/失败附件投影、第二次非通过停止及同批调用保护有局部证据；正文和附件语义一致性、工作流取消竞态仍有缺口。对应 E04–E08、E16。 |
| R06 / T07 节奏与主动交流 | 部分 | `conversation_followup_concurrency_test.go:17`、`wakeup_event_context_integration_test.go:12`；隔离测试证明追发栅栏及受控的新事件/静默选择，真实模型自主性与无事退避未验收。对应 E08、E12。 |
| R07 / T08 表达学习 | 部分 | `expression_learning_use.go:32,60,105`、`expression_learning_lifecycle_integration_test.go:13`；同一候选的来源、启用、受控请求使用和停用有证据，真实模型采用及所有注入/诊断文本排除未验收。对应 E10。 |
| R08 / T09 记忆治理 | 部分 | `runtime_summary.go:251`、`memory_management.go:128`、`memory_correction_conversation_integration_test.go:9`；已解析来源有保守读取屏障，未知旧来源、纠错后可证明的摘要重建及锁序仍未解决。对应 E11、E17。 |
| R09 / T10 领域保护 | **未完成** | `goal_t10_protection_test.go`、`wardrobe_borrow_capability_test.go`、`working_persona_chain_test.go` 等有隔离样本；四组 Goal/活动测试在本轮和起始提交均失败，完整领域矩阵未验收。对应 E14。 |
| R10 / T11 管理、Trace、重放 | **未完成** | `configuration_catalog.go:229`、`diagnostic_export_privacy.go`、`agent_run_record.go:213`；Moment 只读健康检查已接管理卡，独立调试权限、通用安全 Trace 重放和真实浏览器贯通仍未完成。对应 E15、E18。 |
| R11 / T12 验收与交付 | 进行中 | 本报告的 E01–E18、[脱敏测试结果](evidence/meromero-t12-verification-20261112.txt)和台账提供当前证据；未通过和未执行项仍保留。 |

## D. 关键链路和调用次数

| 链路 | 当前实现与证据边界 |
| --- | --- |
| 普通私聊 | 缺省 `fact_only`，认知后仍有独立 Writer。旧真实样本曾出现同一 MeroMero 主 Agent 两次加独立 Writer 一次；这只是该样本，不能推断每轮固定三次或一定调用 Qwen。 |
| 显式 direct 私聊 | 目标是同一主 Agent 通过原生 `conversation.reply` 提交正文和决策。隔离数据库与受控 Provider 的单次请求、事务回滚及幂等通过；一次真实尝试因参数不符合契约连续三次纠错后停止且未发布。普通无工具场景不要求固定三次。 |
| 工具循环 | 现有 Eino/ADK 循环可在受控场景完成发现 → 加载 schema → ToolCall → ToolResult → 再决策；真实 MeroMero 曾完成合成测试工具往返，尚未证明真实领域工具的完整链。 |
| 媒体恢复和动态 | 沿用既有 intent、资产、发布和回执；部分 ready 夹具重启后重复处理只交付一次。真实渲染、未 ready 的完整恢复、正文与附件一致性仍未通过。 |
| 认知结算 | 显式 direct 在相同事务内消费部分 claim、influence、认知阶段和 Goal 候选；局部隔离测试通过，其他引用种类、结算恢复及真实模型语义尚未全覆盖。 |
| WakeUp、表达和记忆 | 分别通过受控事件/静默、候选启停、新旧发言及纠错后召回的局部测试；不证明真实模型会主动分享、模仿候选或自动完成纠错。 |

没有发现本轮为这些功能新增第二套 Runtime、调度器或事实存储；这不是对所有历史兼容路径均已清理的证明。

## E. 模型与插件边界

正式 Agent/variant 的调用入口、任务职责、生产使用状态及当前已知模型来源见 [Agent 清单](docs/verification/meromero-agent-inventory-20261109.md)。默认模型和覆盖值取运行配置，而非源码中的固定 ID；默认优先 MeroMero 不意味着所有任务均已证明适合它。真实视觉使用独立配置，不能以文本成功代替看图。

Image 和 Moment 使用既有 Capability Registry 与领域实现。Image 声明 ComfyUI 依赖；Moment 的 Image 依赖是可选的。停用后新工作受门禁，已提交回执可重读，Image 关闭时纯文字动态的隔离测试通过。图片插件页面能显式检查已保存的连接；Moment 页面能显式检查当前进程的注册及动态列表读取。前者不证明生成，后者不证明发布或模型；真实 ComfyUI 及所有在途策略未验收。

## F. E01–E18 验收矩阵

以下“通过”均指明确列出的局部测试；每行最后一列列出不能由其证明的内容。测试函数路径均位于 `apps/core-go/internal/core/`，除非另有说明。

| 项 | 局部证据 | 当前缺口 / 完整结论 |
| --- | --- | --- |
| E01 模型路由 | `agent_model_routing_test.go:246` 在隔离库和受控 Provider 验证保存、回读、单任务覆盖、其他任务不变及清除；此前本机模型的一次合成普通会话发出一次 `conversation.reply`。 | **未完成。** 真实私聊默认链仍有 Writer；未证明所有迁移任务在无 Qwen 下可运行，也未完成真实 Qwen/Temporal 验收。 |
| E02 上下文隔离 | `reflection_context_isolation_test.go`、`wardrobe_preparation_prompt_test.go` 和 [T03 专项](docs/verification/meromero-t03-context-isolation-20261111.md)提供部分投影及受控请求证据。 | 事实检查、全部任务及 NPC/Studio 的真实语义和边界未全验收。 |
| E03 原生工具循环 | `kev_test.go:548` 的 Generate/Stream 受控四个子场景通过发现、加载、执行和续接；后续 T06 增量覆盖目录权限状态、分阶段 Agent 拒绝及停用后真实回执重放。 | 受控 Provider 和测试工具不证明真实模型完成全部真实领域调用；各独立 Tool 权限矩阵未全验收。 |
| E04 无人图片 | `media_capture_plan_test.go:130,243,518` 覆盖无人主体投影和自拍矛盾的局部约束。 | 冻结时间/主体贯穿实际规范化和真实渲染未证明；无真实无人图片证据。 |
| E05 换衣自拍 | 本次 `r6_wear_contract_test.go:52` 的真实持久换衣及重放、`current_capture_test.go:16` 的当前穿着约束和质量状态测试通过。 | 没有一次完整“实际换衣 → 冻结穿着 → 真实生图 → 真实质量检查 → 交付”的通过证据；私聊换衣不可用后的两个回退纠错子场景失败，见 G。 |
| E06 缺失物品 | 本次 `wardrobe_capabilities_test.go:114` 验证不可用衣物不穿、缺失物品有明确回执和跨 Owner 查询被拒；`current_capture_test.go:16` 验证愿望衣物不能代替实际穿着。 | 缺失紫色大衣、角色自主拒绝或未来购买的完整真实模型/图片场景未验收；回退纠错两项失败不能忽略。 |
| E07 异步状态 | 本次 `reply_media_cancellation_test.go:8` 验证取消、失败和 ready 附件区别；`media_quality_test.go:399` 验证第二次 reject 保持失败。 | queued、ready、delivered、failed、canceled 的完整真实生命周期、取消竞态和未看图的正文声明仍未全验收。 |
| E08 幂等和恢复 | 本次 `conversation_followup_concurrency_test.go:17,120,193` 验证新消息栅栏、旧已提交效果保留和 fact-only 旧草稿不发布。 | 真实 Temporal、真实渲染、进程崩溃时所有在途状态及全部附件去重未验收。 |
| E09 插件启停 | 本次 `domain_plugins_integration_test.go:14` 验证两个插件停用、新工作拒绝、旧回执及纯文字路径；React 插件状态测试通过。 | 独立前后端测试不能代替真实浏览器到运行消费；NPC 和未 ready 媒体完整恢复、真实 ComfyUI 未验收。 |
| E10 表达学习 | 本次 `expression_learning_lifecycle_integration_test.go:13` 从真实受理的测试消息提取候选，审核启用后进入受控请求，停用后不再注入，并核对人格/关系语义。 | 模型是否真实采用、所有错误草稿和诊断文本排除、关系称呼兼容性及浏览器到运行链未验收。 |
| E11 记忆纠错 | 本次 `memory_correction_conversation_integration_test.go:9` 保留两次发言、经 Owner 修订后旧购票判断不再作为事实返回；`memory_recall_speaker_integration_test.go:10` 验证说话主体。 | 这是受控人工纠错；未知旧来源、生产摘要正确重建、在途请求和锁序仍未解决，不代表所有 Actor 均已覆盖。 |
| E12 主动交流 | 本次 `wakeup_event_context_integration_test.go:12` 在受控 Provider 下覆盖无新理由静默、有新事件分享及未回应状态。 | 脚本结果不能证明真实模型自主选择；源码静态复核确认现有固定十分钟周期没有连续无事退避，避免重复播报和无限追问也未做完整真实验收。 |
| E13 人格与普通交流 | 人格切换/接管及拒绝不触发模型回退有受控测试。 | 两种人格在普通交流、分歧、玩笑、认真沟通和主动分享的多次独立真实模型样本未完成；不能评价是否压平人格。 |
| E14 既有业务 | Goal 上限、暂停、已完成旧愿望、衣柜、人格、视觉候选、NPC、Studio 和日记有局部隔离回归。 | **未通过。** 四组 Goal/活动闭环在当前 T10 和起始提交均失败，部分显示 `evaluation_cadence`，根因未确定，已另建 bug；完整领域矩阵未验收。 |
| E15 管理闭环 | 本次后端配置保存/回读/受控下一次请求，以及 React 图片插件设置/检查分别通过。 | 没有真实浏览器完成保存 → 刷新 → 回读 → 下一次运行消费的完整证据；其他管理入口及独立调试权限未全完成。 |
| E16 发布与质量 | 本次第二候选 reject 后状态为 failed；失败/取消资源不作为附件。同批图片与回复的受控保护此前通过。 | **未完成。** 没有媒体调用时正文仍可能声称已发送；失败或 ready 附件也可能与正文语义冲突。真实图片和视觉检查未验收。 |
| E17 未知与来源 | `intelligence_test.go:166`、`native_tool_contract_test.go:68` 有不接受未知引用和伪造来源参数的契约测试。 | 用户正文伪造 Tool/Core 的端到端状态边界、未检索到承诺的未知语义、明确接受与拒绝未全通过；相关 Goal 明确接受场景见 E14 的失败。 |
| E18 实际能力 | 受控工具续接和历史真实模型合成工具往返分别有证据；连接检查明确不声称生图。 | 无真实视觉理解、真实生成图片及完整日程规划证据；配置 Tools、HTTP 成功、文字检测和简单冲突判断均不能代替这些能力。 |

## G. 分层验证与原始结果

| 层次 | 实际结果、受检环境与限制 | 证据 |
| --- | --- | --- |
| Go 全量单元/契约 | `42b43abf` 干净副本，`go test -json ./...` 退出 0；18 个包通过，测试事件（含子测试）3,108 通过、0 失败、1,073 跳过。已移除数据库和真实服务环境变量。 | `/private/tmp/fluctlight-t12-clean-go-all.jsonl`；[过滤后的结果](evidence/meromero-t12-verification-20261112.txt) |
| 独立 Tool / 原生 Agent / 隔离集成 | 同一干净副本、一次性 PostgreSQL 和受控 Provider；14 个顶层测试全部通过，其中原生工具循环含 4 个子场景。覆盖模型配置、连续消息、诊断、插件、表达、媒体质量、记忆与 WakeUp。 | `/private/tmp/fluctlight-t12-head-pg-matrix.log`；[测试结果](evidence/meromero-t12-verification-20261112.txt) |
| Web 受控测试 | 当前工作树的 Web 源与受检提交一致；插件健康和诊断详情共 8 项通过、0 跳过；`pnpm --filter @fluctlight/web typecheck` 退出 0。 | `/private/tmp/fluctlight-t12-head-web-matrix.log`、`/private/tmp/fluctlight-t12-head-web-typecheck.log` |
| 衣柜、无人主体及来源补验 | 同一干净提交和隔离 PostgreSQL：9 个顶层测试中 8 通过、1 失败；子场景 28 通过、2 失败。失败的两个私聊换衣回退纠错子场景在起始提交通过；将旧夹具的 400 临时改为 503 后仍失败，实验未保留，原因未确定。 | `/private/tmp/fluctlight-t12-head-capture-wear.log`、`/private/tmp/fluctlight-t12-baseline-wear-fallback.log`、`/private/tmp/fluctlight-t12-wear-fallback-updated.log`；[脱敏结果](evidence/meromero-t12-verification-20261112.txt) |
| HTTP 400 回退边界 | 新增 `TestReplyBadRequestDoesNotInvokeExplicitFallback`，提交 `45489e84` 后在干净副本单独运行通过；受控 Writer 返回 400 时没有调用备用模型。 | `/private/tmp/fluctlight-t12-bad-request-postcommit.log`；不代表前述两项换衣纠错失败已解决。 |
| 真实模型 | 本机指定 MeroMero 的一次合成普通会话测试通过，日志记录一次物理请求；一次显式 direct 私聊样本三次纠错后失败。它们是历史单次样本，不是本次重跑或多次独立验收。 | `/private/tmp/fluctlight-t02-meromero-live-current.log`、`/private/tmp/fluctlight-current-explicit-direct-live.log` |
| 真实媒体 / 外部服务 | **未执行本次真实出图或真实视觉验收。** 本机受控 HTTP 连接检查和模拟媒体不能充当真实能力。 | T11 对应记录在实施台账；无真实成功证据。 |
| UI 到运行消费 | **未完成端到端验证。** React 受控服务和后端隔离集成是分开的运行。 | 上述 Web 日志和隔离数据库日志。 |
| 既有失败 | T10 四组 Goal/活动闭环在当时工作区及起始提交均失败，部分原因显示 `evaluation_cadence`；这是已登记的非阻断旧问题，根因未确认。 | `/private/tmp/fluctlight-t10-goal.log`、`/private/tmp/fluctlight-t10-life.log`、`/private/tmp/fluctlight-t10-goal-baseline.log`；[失败测试名摘录](evidence/meromero-t12-verification-20261112.txt) |

本次隔离专项从仓库根目录使用临时脚本将隔离库连接注入干净副本；等价的可复核命令如下。先通过安全方式设置 `GO_CORE_TEST_DATABASE_URL` 指向**允许创建和删除临时测试数据库的一次性实例**，不要把连接串或密码写入报告：

```sh
go -C apps/core-go test ./internal/core -count=1 -v -run '^Test(AgentModelConfigurationSavesReadsRunsAndClears|DirectFollowupsSupersedeLateReplyAndKeepWholeInput|DirectFollowupKeepsPriorCommittedAction|FactOnlyFollowupRejectsStaleWriterDraft|AgentDiagnosticErrorDoesNotExposeBearerCredential|AgentDiagnosticProjectionRedactsEncodedFailureOutput|DomainPluginPostgresSettingsAdmissionAndReplay|ExpressionCandidateEnableUseAndDisableAcrossNewTurns|KevNativeLoopDiscoversAndExecutesHiddenTool|MediaQualityRetryPersistsFeedbackAndRejectsSecondCandidate|MemoryCorrectionPreservesStatementsWithoutRevivingTicketClaim|MemoryRecallDistinguishesActualMessageSpeakers|ReplyMediaTerminalStateDoesNotAttachCancelledOrFailedAsset|WakeUpRequestReceivesNewEventAndUnansweredDelivery)$'
node --test apps/web/test/plugin-health.test.mjs apps/web/test/diagnostic-model-calls.test.mjs
pnpm --filter @fluctlight/web typecheck
go -C apps/core-go test ./internal/core -count=1 -v -run '^Test(R6OwnerOutfitNativeDiscoveryActuallyWearsAndReplays|R6PrivateChatDiscoveryWearReceiptAndWriter|WardrobeIndependentToolRejectsUnavailableAndForeignOwner|CurrentCapturePlanCannotUseAnUnheldObject|CurrentCaptureRejectsClothingFreeTextAndKeepsActualWearing|MediaBranchM30M31SequenceAndActorIsolation|MediaBranchNoMandatoryPersonSlotsAndDeferredReadCounts|NativeActorContractRequiresAssertionAndBindsCurrentSource|EvaluateClaimsResolvesOnlyVisibleOpaqueEvidence)$'
go -C apps/core-go test ./internal/core -count=1 -v -run '^TestReplyBadRequestDoesNotInvokeExplicitFallback$'
```

第一条 Go 专项应实际显示 PASS、无数据库 SKIP；衣柜补验命令在当前版本仍预期暴露上述两个失败，不能当成已通过。最后一条 HTTP 400 检查应通过。这些都不是 E01–E18 整体通过。全量 Go 的对应命令是在清除真实数据库和服务变量后执行 `go -C apps/core-go test -json ./...`。

## H. 可复核的典型受控样本

以下均为测试夹具，不含真实用户对话：

1. **插件停用：** 先各受理一个文字动态和图片 intent；关闭 Image/Moment 后，原回执可重读，新调用被拒，普通文字回复仍成功；重新开启 Moment 后，Image 关闭时纯文字动态成功。数据库中原动态和 intent 各保留一条。测试：`domain_plugins_integration_test.go:14`。
2. **表达候选：** 输入测试消息“慢慢来呀”，提取并审核启用；下一次受控请求包含该候选及情境，停用后下一次请求不再包含；测试比较人格和关系语义未改变。测试：`expression_learning_lifecycle_integration_test.go:13`。
3. **记忆纠错：** 先说“I bought a ticket”，再说“the ticket statement was a joke”；Owner 修订后调用实际 `memory.recall`，两条历史发言仍保留，旧“票已购买”判断不再以长期记忆事实返回，购买保持未知。测试：`memory_correction_conversation_integration_test.go:9`。
4. **第二次质量拒绝：** 第一候选产生 retry 与反馈，第二候选 reject 后数据库记录 `verdict=reject`、`status=failed`；相同候选重放不改变结果，不同候选冲突。测试：`media_quality_test.go:399`。此样本未运行真实渲染或视觉模型。
5. **模型配置：** 在隔离库保存 Agent/variant 覆盖并回读，受控 Provider 下一次请求接收到对应模型；其他任务的配置不随之改变。测试：`agent_model_routing_test.go:246`。这是受控 Provider，不是真实 Qwen 推理。

缺失的真实证据包括：实际 Owner/Fluctlight 无 Qwen 单主私聊、真实领域原生工具全链、真实图片成功/失败与无人图、真实模型表达采用、真实视觉判断和 UI 到运行消费。不能用以上夹具替代。

## I. 与原要求的偏差和未完成原因

- 为避免已发现的“无附件却声称已发送”问题成为默认路径，当前回退到 fact-only + Writer；这是对单主链目标的明确未完成，不是最终方案。解决后仍需真实 direct 私聊、工具及发布验证。
- 私聊换衣不可用后的两个回退纠错子场景在本次隔离测试失败、起始提交通过。旧夹具要求 HTTP 400 触发回退，与当前终止性 4xx 策略不一致；但临时换成可回退的 503 后仍失败，不能将它简单归因于过期夹具。新增的独立受控检查确认 400 不调用备用模型；已登记 `reply-wear-fallback-correction-regression`，未调整生产逻辑。
- 现有诊断记录会丢弃图片数据、可能截断流输出，也不是每次诊断都保证落盘；当前业务回执重读是幂等恢复，**不是通用 Trace 重放**。完整重放需要可证明完整的输入、模型可见 ToolResult 和受限执行环境。
- Owner 身份目前不能替代独立开发调试权限；模型运行、媒体提示词、事件和搜索等入口需共同处理。当前错误脱敏仅识别特定凭据形式，不能保证所有未标记秘密被遮盖。
- 摘要纠错使用保守读取屏障，不证明后台重建已吸收纠错；早先可能扩大锁反转的写入实验已撤回。工作流取消同步实验也已撤回，不能作为现行通过证据。
- T10 的四组旧领域失败已另记 bug；不因本任务扩大范围去改动旧 Goal 成功条件。其他未完成项是原任务要求，仍归各自 T03–T11，不应被改称为任务外问题。

## J. 剩余验证与回滚

- 要复核真实 direct 私聊，需本机可用的目标 MeroMero、一次性 PostgreSQL，以及在安全环境中设置 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1`、`FLUCTLIGHT_LIVE_PROVIDER_URL`、`FLUCTLIGHT_LIVE_PROVIDER_MODEL` 和隔离库连接；然后运行：

```sh
go -C apps/core-go test ./internal/core -count=1 -v -run '^TestLiveExplicitDirectConversationDoesNotRequireASecondWriter$'
```

  预期实际发布一次并满足测试中的单主 Agent 合同；失败时保留输出。成功一次也不能代替原任务要求的多次独立运行、不同人格及真实工具场景。当前已知发布一致性缺口修复前，不能仅凭此命令将 direct 切为默认。
- 真实图片还需独立测试 ComfyUI、视觉模型和隔离资源；检查应分别记录实际生成、质量结论、交付与失败，不能将 `/system_stats` 可连接等同于出图。浏览器验收需真实操作管理页保存、刷新、回读并观察下一次运行消费，而非仅运行 React 夹具。
- 已有迁移只在隔离库执行。回滚时应先停止相关写入、备份并核对迁移版本及在途任务，再按获授权的数据库变更方案处理；旧代码可能不认识新 head，回退快照或纠错屏障会失去对应恢复保障。不得重建数据库、删除用户数据或重新执行已完成的购买、换衣、发消息和生图来完成回滚。
- 剩余代码任务：先按 T03–T11 各任务的失败/交接记录恢复处理，再重新生成与最终提交绑定的验收候选。真实模型或外部服务不可用时，保留 BLOCKED/SKIP 和恢复条件，不伪造结果。

## 最终自检（当前快照）

| 原始自检 | 当前回答 |
| --- | --- |
| 1 只更换模型？ | 否，已修改路由、恢复、插件、记忆、表达和诊断；总体仍未完成。 |
| 2 仍固定 Qwen 或中介模型？ | 默认 fact-only 有独立 Writer，不能证明已达单主链；具体模型取配置。 |
| 3 万能 Agent？ | 未将 31 个正式 Agent 合并；各输入契约仍需完整核验。 |
| 4 只有插件接口？ | 已接实际 Tool 与主要领域门禁；全生命周期和真实服务仍未完成。 |
| 5 HTTP/JSON 冒充模型能力？ | 本报告分开记录受控测试、有限真实模型样本和未执行能力。 |
| 6 认知、Goal、人格回退？ | 局部消费/切换有证据；Goal/活动存在明确失败，整体不能回答无回退。 |
| 7 全任务注入完整人格？ | 部分隔离测试通过，尚未完成全入口验收。 |
| 8 表达、WakeUp、记忆有遗漏？ | 有；分别见 E10、E12、E11。 |
| 9 仅靠 Prompt/正则保证发布？ | 已有事务、状态和附件检查；正文与媒体语义一致性仍未完成。 |
| 10 停用只是隐藏按钮？ | 不是；后端有门禁和隔离测试，但完整在途策略未验收。 |
| 11 所有独立 Tool 权限已证明？ | 否；已有部分独立调用和权限证据，未覆盖所有 Tool。 |
| 12 新增第二套 Runtime？ | 未发现本轮新增第二套 Runtime、调度器或事实库。 |
| 13 人格或括号动作被统一压平？ | 未发现一刀切删除；多人格真实交流未验收。 |
| 14 为测试强迫答应或降低条件？ | 当前受控样本允许拒绝；完整业务条件仍需复核，Goal 原条件未为测试降低。 |
| 15 将未执行真实测试称为通过？ | 否；本报告把真实模型有限样本、真实媒体和 UI 端到端分开记录。 |
