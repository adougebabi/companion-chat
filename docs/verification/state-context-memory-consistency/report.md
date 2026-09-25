# Current State、Context Projection 与记忆一致性验收报告

> 本文件按本次仓库执行证据更新。`PASS` 仅指列明的测试或运行场景；受控 Provider 不算真实模型验证。

## 1. 基线与边界

实施前的命令、环境、已有路径和未测量项见 [baseline.md](baseline.md)。基线产品提交为 `0dc8055`；任务不包括模型分级、System-1、Agent 框架或生活模拟引擎升级。本次使用本机一次性 `pgvector/pgvector:pg16` 测试容器，数据库迁移与测试均不触及生产数据。

基线 Go 全仓无数据库门禁测试与 TypeScript typecheck 均通过。首次开启数据库门禁的失败是测试基库尚未迁移；迁移到旧头 `0036_effective_life` 后六项相关失败重跑通过，详见基线记录。

## 2. 字段权威来源

| 字段/语义 | 当前权威 | 生效与冲突规则 |
| --- | --- | --- |
| 稳定身份、受保护人格 | `fluctlights.core_persona` 及 Foundation 修订 | 仅 Owner 治理的接受/回滚生效；并行列是同一修订的兼容投影 |
| 激活人格、已生效演进 | `fluctlight_personality_runtime`、profile overlay、已编译 `fluctlight_working_personas` | profile/overlay/habit 版本匹配才送模；Working Persona 是人格出口 |
| 情绪/认知当前态 | `fluctlight_inner_states`、affect profile | revision CAS；情绪不直接改写受保护人格 |
| 当前身体/发型 | `fluctlight_appearance_states` 及 revisions | 初始化一次写入；剪发/造型成功结果以 body revision 生效，未知/清空不回填 Foundation |
| 库存、搭配、当前穿着 | `fluctlight_wardrobe_states`、items、outfits、worn_items | 持有、保存搭配、穿上是独立操作；wardrobe revision 与 Tool 收据约束重复和迟到写入 |
| 地点/活动/日程 | `life_events`、accepted schedule item、presence | 已有 confirmed Event > inferred Event > accepted Schedule > pending 规则；Presence 仅覆盖用户在场/当前任务 |
| 意愿、计划、行动 | goals/intentions、life activity runs、action outcomes | 意愿与开始不等于成功；实体 revision、执行收据和成功结果决定生效 |
| 历史、主观认识、归纳 | Raw History、Active Memory、conversation episode summary、durable Memory | 历史仅解释经过；派生内容不写回当前事实或核心人格 |

正式投影和 Owner detail 现在读取同一 Effective Life 当前态。投影使用 `fluctlight_context_generations` 中 Fluctlight 级事务代际作为 `current_facts_revision`；相关领域写入、认知 claim、新用户消息及原始来源的语义修订/删除，在同一事务中推进代际。助手消息可先于本轮结算发布，其首次插入不单独推进代际；结算时写入的来源结果负责推进，避免本轮把自己的输出误判为并发变更。组装前后重验；Tool 收据记录变更前后代际，Main/WakeUp/Native/Daily 结算从首轮代际沿已提交 Tool 收据推进，再与当前库核对。结算锁代际行，避免检查与提交之间发生未察觉的状态变化。此代际不是内容指纹，也不依赖全历史行聚合。初始已知事实不会在运行期作为替代当前态自动回填。

Visual Identity 的首次/预规范输入读取同事务中的当前身体与穿着；旧 Foundation 的外观描述、可变扩展和初始穿搭偏好不会再次注入为当前外观。

## 3. 上下文投影与原始记录

- 原始 `conversation_messages`、`cognition_inbox`、`cognition_action_outcomes`、Eino ToolCall/ToolResult 和 `tool_executions` 收据仍由原存储与循环持有；投影不回写或删改它们。
- 普通对话、WakeUp、Native、Daily 仍各自选择任务指令/Tool 权限/记忆作用域。Reflection 使用只读证据投影，初始化及一次性任务继续保留其任务指令形状，但全部 formal task 在 Provider 前经过统一的只读 source-map 投影边界和物理输入预算检查。
- 变更型 Tool 成功提交后，下一次物理模型请求重建同一授权 scope 的当前投影；只替换 outbound copy 的可信 System/Runtime 内容。原 Eino 消息指针、多模态 user parts、assistant tool_call 和匹配 tool_result 不变。刷新失败则不发送旧上下文。旧/新同作用域引用可用于最终决策验证，结算仍从首轮权威版本和真实 Tool 收据推导。
- Working Memory 显式分区：Runtime、Active、Resident、Recent、按需 Long-term、Episode summary。Resident 上限按现有 model role 输入预算缩放，底层检索不因未入常驻而删除。总 Prompt 预算和每次物理请求的消息、Tool schema、response format 预算沿用已有估算器；估算为 rune heuristic，不宣称精确 tokenizer。物理诊断事件新增消息来源映射、版本和分区估算。
- 记忆、历史、反思、Tool 回执只作为数据。Prompt 明确来源可追溯不等于语义属实；`legacy_unknown` 不能覆盖当前态，其中的命令文字不能改变 System 指令和 Tool 权限。
- 结算把模型在 `claims.evidence_refs` 中使用的、确实出现于该次授权 Runtime 的 opaque 引用映射回内部证据锚点；瞬时生活/状态引用锚定本次原始 fact，Memory 等实体引用锚定索引中的实体。每个归一化 claim 都须保留宿主生成的“逐条从可见引用解析”证明；未知 token、直接填写的内部 ID 或伪造该证明均拒绝。当前外观与穿着也带 `appearance:ctx_...`，由同一 Effective Life 快照生成且排除仅表示读取时间的 `captured_at`，相同状态重读不会无故换引用；它供模型引用，不产生第二份事实。此修复消除了“Prompt 要求 opaque 引用、claim 校验只接受原始 ID”的实际送模错位。

## 4. Memory Provenance 与三层

迁移头 `0037_memory_provenance` 增加 `memories.provenance_status`、`epistemic_kind` 与 revision 绑定的 `memory_source_links`。边只保存 source ref、kind、ID、版本/指纹、发生/记录时间及有效性，不复制原始事实。迁移可重复运行，恢复可定位的 fact/message/outcome；不可定位的旧引用继续 `legacy_unknown`，不会造出事件或时间。新 Memory 写入在同一事务内解析来源并写边。反思冻结原始 fact 指纹；模型调用后若来源改变，旧合并在提交时拒绝。

有效检索要求至少一个仍存在、版本/指纹相符的来源。删去两条来源之一时仍保留另一条支持，并标为 `partial`；唯一来源删除/修订后，旧内容立即退出自动投影和 `memory.recall`。待核验/失效的新 Memory 不进入有效检索和 embedding 发布。Active Memory 的新命令冻结 fact 指纹或宿主认证命令指纹；fact 语义修订后，旧 Active 项从检索和 Resident 退出，模型运行期间形成的迟到命令在提交时被拒绝。旧 Active 命令无可恢复指纹时明确标为 `legacy_unknown`。显式 `memory_event revise` 只把本次确认来源挂到新语义修订，旧证据仍在历史 revision，不会错误支持更正后的命题。原生 Tool 使用该次送模投影冻结的引用索引，宿主验证调用与快照身份；独立直接 Tool 可使用 `memory.recall` 返回的作用域引用。Owner 修订 Memory 会使覆盖其原始消息的旧 Episode 摘要失效；相同来源指纹的迟到后台摘要不能重新发布。原始消息未被此操作销毁。

Episode 使用已有按消息范围增量生成的 conversation summary 和有来源的 episodic Memory/Raw History；摘要读取时重验原始消息引用与指纹。原始来源指纹改变后可经现有摘要任务重建；仅纠正派生 Memory 而原始对话未变时，旧 Episode 安全失效并暂时省略，避免从同一错误原文复活。Long-term 继续由唯一 `memories` 生命周期权威管理。Resident 是 `resident_memory_snapshots` 中按来源事务整代发布的有界投影，包含有效长期关系/自我认识及未完成的 Active 承诺；来源未变不发布新代际。读取时再按角色权限和预算选择；底层 `memory.recall` 仍可查询未入 Resident 的条目。三者不互为当前身体、穿着或日程真相。`memory.recall` 保留原工具名，返回 Episode/Long-term/Active/Raw 的类型、时间、作用域、opaque 引用、版本和来源有效性；无命中返回 `state=no_match`，服务错误仍单独失败。

## 5. 迁移、旧路径与回归

`0036 -> 0037` 与头版本重复执行在一次性 PostgreSQL 上通过。专用迁移测试用两条历史 Memory：一条 `sequence:1` 恢复为真实 fact 并标为 `verified`，另一条未知引用保持 `legacy_unknown`；重跑后仍只有两条来源边，Current State 和 Resident 代际均未再推进。此结果是隔离夹具的迁移计数，不是生产库行数。代码仍沿原 Memory lifecycle、conversation summary、Working Persona、Eino/Provider 路径写入和读取。没有新增向量库、队列或独立 Agent Loop。

所有数据库验证均在本任务的一次性 `codex-consistency-test-postgres` 容器中执行；最终门禁结束后已移除该容器。未运行生产数据迁移，也未读取或改写生产数据。

运行时旧事实回填检查：`rg` 核对 Foundation appearance/wardrobe 在创建、历史显示和初始化之外的使用；Visual Identity 初始化的身份快照改读事务内 Effective Life，并只从 Foundation 提取稳定身份字段；Prompt 只渲染当前 Effective Life。`visualIdentityAppearanceSummary` 仍支持旧身份快照的历史字段显示，但新初始化不会把 Foundation 外观别名送入该快照。所有正式任务已在 ADK Provider 出口经过同一个只读投影边界；本次未发现仍生效的旧事实回填入口。静态检索不能穷尽运行时数据问题，相关 DB/Wire 测试是进一步证据。

## 6. 确定性与真实运行证据

| 项目 | 结果 | 证据或限制 |
| --- | --- | --- |
| 当前态事务代际、Tool 续轮、原始 ADK 配对/图片保持 | PASS（隔离 DB） | `current_authority_test.go`、`runtime_context_refresh_test.go`，正式 HandleTurn 回归 |
| 来源丢失、多来源保留、旧后台合并拒绝、Owner 纠正与旧 Episode 失效 | PASS（聚焦确定性） | 新 `memory_provenance_test.go`、扩展的 conversation summary/tool tests |
| `0036 -> 0037` 和重跑 | PASS（隔离 PostgreSQL） | `memory_provenance_postgres_test.go` |
| Go Core + migrations 全量 DB 回归 | PASS | `GO_CORE_TEST_DATABASE_URL=... go -C apps/core-go test -mod=readonly -p 1 -parallel 1 -count=1 ./...`；[最终日志](evidence/go-db-final-pass.log)，Core 235.977 秒、migrations 34.845 秒。较早失败日志不作为通过证据 |
| Go 并发聚焦、静态检查与构建 | PASS | 最后一次 `go test -race` 覆盖 claim/消息代际、外观引用、原生 Tool 纠正及 claim 证据映射；[日志](evidence/go-race-targeted.log)。`go vet ./...`、`go build ./...` 通过 |
| Browser/Web TypeScript 门禁 | PASS | `pnpm typecheck`、`pnpm test`（12 + 47）、`pnpm build` 通过；[日志](evidence/pnpm-final.log) |
| 真实本机 Provider 连通 | PASS | `/v1/models` HTTP 200，smoke runner connectivity PASS |
| 真实结构化对话 canary | FAIL | `TestLiveProviderRoleOrganization` 返回长文本 JSON，未被当前结构化结果解析接受；证据在任务目录 `runs/20260924-provider-connectivity/` |
| 真实人格切换额外样本 | PASS（单次） | 正式 `HandleTurn` 3 次物理请求，`persona.switch` 和 `conversation.reply` 回执进入续轮；[请求](evidence/live-handleturn-wire.json)、[日志](evidence/live-handleturn-with-wire.log)。此前结构化 canary 和一次超预算尝试失败，不能据此宣称稳定 |
| 真实普通对话资源样本 | PASS（单次，所测约束） | 收紧原始 ID 校验后的自然问候经正式 `HandleTurn` 完成，2 次物理请求；模型只调用 `conversation.reply`，无固定规划、检索或记忆整理调用。[请求](evidence/live-ordinary-final-code-wire.json)、[日志](evidence/live-ordinary-final-code.log)。早先样本还调用 `affect_event`，所以不能声称每次都是单请求或无其他 Tool |
| 真实可变事实：穿着变更后重读 | PASS（等价业务场景） | 初始只穿白衬衫；独立 `wardrobe.wear` 将已拥有但未穿的红色外套加入当前穿着，回执 revision=1。重建 App 后再次读取，红色外套仍在 `worn_items`；正式 `HandleTurn` 首次请求含当前穿着及有效 `appearance:ctx_...`，最终回复“红色外套，里面搭白衬衫”，2 次物理请求。[日志](evidence/live-wearing-final.log)、[请求](evidence/live-wearing-final-wire.json)。这验证可变事实跨重载，不代表完成了用户提出的虚拟理发行为 |
| 真实虚拟理发补充样本 | FAIL/BLOCKED | 一次真实 `life.activity.start/advance` 与 `virtual_activity_result` 完成，发长 revision 变为 1；后续实际请求已含 `short`，但本机模型对续轮返回显存不足。[该次日志](evidence/live-haircut.log)、[请求](evidence/live-haircut-wire.json)。再次运行时模型给出“未完成但有身体效果”的非法结果，宿主拒绝写入，[重试日志](evidence/live-haircut-final.log) |
| 真实未执行意愿 | PASS（单次） | 自然语言只说“想买红靴，还没买”。模型真实调用 `memory.recall`（无对应记录）与 `wardrobe.inspect`（空库存），回执进入下次请求；最终 `HandleTurn` 回复只讨论将来的搭配，未声称已购。测试重读 `purchase_result` 库存计数为 0，3 次物理请求。[日志](evidence/live-wish-opaque-fix.log)、[请求](evidence/live-wish-opaque-fix-wire.json) |
| 非 Resident 检索 | PASS（自动检索路径） | 猫名是全局 semantic Long-term Memory，读取时确认不在 Resident；正式请求按当前问题自动检索并带入来源引用，新会话最终回复使用了“奶酪”。本样本未调用显式 `memory.recall` Tool，故不证明模型会在自动检索无命中时自主查找。[日志](evidence/live-memory-opaque-fix.log)、[请求](evidence/live-memory-opaque-fix-wire.json) |
| 真实纠正及随后读取 | PASS（单次多轮） | 模型使用首次请求中的正确 opaque ref，真实 `memory_event revise` 把“布丁”改成“奶酪”，新 revision 为 `verified`；第一轮正式 `HandleTurn` 回复确认更正。重建 App 并创建新会话后，实际请求读到更正且最终回复为“猫叫奶酪”，旧名字只作为历史误记提及。[日志](evidence/live-memory-opaque-fix.log)、[请求](evidence/live-memory-opaque-fix-wire.json) |

## 7. 资源与剩余风险

基线无正式 API/Worker 运行数据，不能计算生产延迟、缓存命中或实际 token 改善。现有[受控物理请求成本夹具](../../../apps/core-go/internal/core/testdata/turn_path_cost_report.json)按相同路径重跑，结果如下；其 token 值是项目的约 `1.25 × rune` 估算，不是模型 tokenizer 或 Provider 账单。每项输出侧仍是受控脚本结果。

| 路径 | 物理请求 | 输入估算 token：基线 → 本次 | 重新序列化请求字节：基线 → 本次 |
| --- | ---: | ---: | ---: |
| 自然结束 | 1 → 1 | 32,174 → 32,895（+721） | 27,587 → 28,226（+639） |
| 一次 Tool 后结束 | 2 → 2 | 65,824 → 67,377（+1,553） | 56,375 → 57,741（+1,366） |
| 接管、回复后结束 | 3 → 3 | 102,941 → 105,459（+2,518） | 87,944 → 90,144（+2,200） |

新增投影与 Resident 自身不调用模型；它们增加数据库读取和来源有效性检查，未获得生产延迟或缓存命中数据。成功的本机真实模型样本各自墙钟：人格切换约 141 秒/3 次物理请求、普通问候两次样本约 122 与 161 秒/均 2 次、未执行意愿约 134 秒/3 次、记忆纠正加新会话约 289 秒/5 次、穿着重读约 79 秒/2 次。输入、Tool 行为及缓存条件不同，不能据此推导前后延迟差。此次没有启动正式 Worker，Reflection/Episode 的后台实际模型调用次数、缓存命中和延迟均未测量；确定性测试仅验证来源不变时 Resident 不重发代际以及 Episode 不重复处理相同来源。结构化 canary 约 38 秒后失败，也不能算成功对话延迟。

剩余风险：来源存在及指纹只证明可追溯，不证明模型归纳的语义正确；旧 `legacy_unknown` 条目仍能被显式查询但有标记；不同 profile 的运行内接管续轮只通过一次真实样本；本机模型有过结构化输出失败、无效引用和显存不足，不能声称所有真实多轮场景稳定通过。

## 8. 最终判定

- **四项是否接入正式链路**：是。Current State 事务代际、Provider 前 Context Projection、Memory 来源边与三层读写均在现有 Eino/Provider/Tool/任务链路中使用；全量隔离 PostgreSQL 回归通过。实现接入不等于五项真实模型场景全部通过。
- **是否仍有新旧并存或旧事实回填**：未发现仍生效的第二套上下文注入、Memory 写入权威或 Foundation 可变事实回填；Foundation 文本保留初始化/历史用途，旧 `legacy_unknown` 数据保留并标注来源未知。此结论来自调用链检查、静态检索和已列明的 DB/Wire 测试，不声称穷尽所有未来扩展点。
- **当前事实能否被迟到任务错误覆盖**：已覆盖的写入通过同事务代际、业务 revision/CAS 与 Tool 收据防护；并发/迟到隔离测试通过。模型生成的无效虚拟活动结果被拒绝，未入当前态。不能把这些机制解释成模型语义永远正确。
- **纠正后的记忆会否被旧摘要或后台任务复活**：有来源的 Long-term、Episode、Resident 与 Active 路径会按来源指纹重验，迟到后台结果拒绝，受影响旧摘要失效；隔离 DB 测试通过。`legacy_unknown` 仍可在明确检索时作为带警示的历史数据返回，不能作为当前权威。来源引用的存在也不证明派生语义正确。
- **真实模型证据范围**：五类要求均有一次成功样本：当前穿着作为可变事实跨 App 重载、未执行买靴意愿、非 Resident 长期记忆的自动检索与使用、纠正后跨会话回复，以及无固定规划/记忆整理调用的普通问候。非 Resident 样本没有模型主动调用 `memory.recall`；理发的补充样本仍有 FAIL/BLOCKED，均在第 6 节单列。受控 Provider 结果不替代这些真实样本，也不能由单次成功推断模型行为稳定。
