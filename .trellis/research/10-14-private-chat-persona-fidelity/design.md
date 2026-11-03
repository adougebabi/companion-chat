# 私聊表达与人格保真的局部设计

## 1. 边界与复用
在现有 `codex/fact-only-reply-vision` / HEAD b24fdd1 的 fact-only 改动上实施。原始 Persona / Working Persona 语义、已发送消息、领域状态、权限、Goal 条件和原 native tool loop 不变。子代理只探索与独立核验；实现、方案取舍和最终验证由主代理负责（用户本次子代理规则优先于技能中的默认写代码分工）。

```text
真实私聊入口 / 已授权主动私聊
  → 原 ContextProjection + 有来源的媒介/空间只读投影
  → Qwen 原认知/真实工具 / conversation.reply(refs)
  → 原 durable draft + 固定表达合同/模型/配置
  → 独立 Reply View / 完整人格原文与解释边界 / 原历史
  → Cydonia native text / 真实 image request
  → 现有 output validator + 文体诊断
       → 正常/存疑：0 额外调用
       → 高置信非作品小说化：同模型最多一次无工具修复
  → 原媒体待执行/同 draft 恢复 → 原唯一 publisher
  → 真实正文/附件 Tool Result → 原 Qwen 结算/一次 JSON repair
```

Moments / legacy / 媒体 intent / visual analysis / Qwen JSON 不经过私聊文体过滤器。旧 legacy 文体规则保留在旧作用域，私聊新稳定合同只能替换一次。

## 2. 实际协议、配置、在途版本
复用 Prompt operation slot 和 existing settings `conversation.reply`，以任务书第4节替换私聊协议，提升到 `fact-only-reply.v5`；保留 v4 文本作为表达版本回退与旧 draft 的兼容合同，不增 B/C/D 常驻生产实验模式。

在既有 reply setting 加一个可选表达版本字段，缺省的新 run 使用 v5，显式 v4 仅回退表达/view 合同，仍走 fact-only。复用现有设置入口，仅必要的最小字段/选择控件，不另建配置中心、不修改模型采样。

新 draft 创建时将 protocol/version/hash、reply-view version、medium rule version、config revision 与已冻结 assignment 存入现有 `binding_snapshot` 的 Core-owned reply contract；配置/绑定重放读取同一冻结合同。旧 durable draft 缺该字段按 v4 兼容识别，绝不标成当前 v5。prepared/published 缓存直接回读存量结果；waiting draft 用原合同恢复，同时按已有事实/profile有效性规则刷新必要动态状态，不冻结陈旧事实。

失败/过期 lease 重领也复用 stored contract/config，不能在旧 config snapshot 上覆盖新模型 assignment；binding 更新保留 frozen contract 和 durable quota。publish、request、prepared、resume diagnostics 从 draft contract 取真实版本，不读全局当前常量。

未知表达版本显式诊断并沿已有 unpublished cancel/rebuild 路径处置；不重新生成已发布正文。回旧二进制前按现有文档排空在途任务，并恢复兼容 settings；新字段移除不能影响原人格/历史。无需新表、工具或持久化领域系统。

## 3. 媒介与空间来源
从已验证 direct private conversation 入口 / scoped run 提供 `channel=text_chat`，不能让 Qwen 基于“找你”推断；同一源在 Cognition/Writer一致。

共同在场使用既有有效 Presence / 当前 scoped Actor facts 的明确证据；分别表示 unknown、明确同处、明确不同处，并保留来源/有效期。分别给 self/recipient location 及 unknown；关系距离、常去店里、同生活圈不能成为本轮 co-presence。自然语言位置陈述继续进入原用户陈述和 Actor fact 规则，不能解析关键词直接升级核验事实。

保留原 communication suppression 数据在原执行边界；新增只读媒介视图不要混淆 awaiting_response。helper 加适用范围，Moments/其他渠道不被错误强制 text_chat。

## 4. 人格、动态状态和按需外貌
优先保留生效 profile 的完整已存 Working Persona portrait_text，不做正则删修饰、抽引号或每轮 LLM 清洗。用独立回复视图解释稳定性格/交流倾向、条件习惯、长期喜好、原始视觉设定和当前状态的含义，原字段不冒充当前事实。已有结构化原文可保留原字段跨度/path和来源；无结构化来源时原文+边界为保守回退，不编造分类。

原用户导入全文、accepted Foundation、编译 portrait 三者分别标明；用 source/profile/revision/overlay/compiler version、raw/view hash 追溯。编译视图缺失或无法映射时，仅退回当前授权 profile 的可用原文，不静默换人格、不绕过跨 profile/版本权限。编译器原存储意义不变；不在聊天中再调用编译模型。原 source 和 profile snapshot hash 均须不变。

Reply current_state 保护身份、时钟、当前有效活动、必要关系/冲突与原 Goal/criteria；普通闲聊不默认展开无关完整 body_fields。必要外貌按语义任务由既有 `persona.detail` 当前信息只读查询取得，复用其权威实现和分段契约；如需向 Writer 提供查询，沿现有私有 registry 限制当前 subject/profile，不能暴露任意 Actor 或新写权限。查询及现有工具真实结果可使跟进/暗指获得必要字段，不用单关键词猜测删字段。媒体执行仍读完整有效外貌。

稀疏 worn_items 标为部分已知穿戴，不反推裸露或补衣物。Goal 空 criteria 如实保留，不造 deadline / 进度 / 完成指标。Qwen与真实媒体工具的 IDs/状态/失败信息不随 Reply View 精简丢失。

## 5. 历史与来源语义
复用 factOnlyHistory 的完整交互片段及 final-selection history metadata。原 role/content/hash/顺序及用户拒绝、邀约、承诺均不改；没有可靠结构化分段标记就保留全文。历史证明过去发过这些字，不证明其中雨、杯子、身体接触或附件真实发生；协议和两处来源语义一致，结算继续依真实 Tool result。

可增只读文体风险标签，但不把标签当真假判据，不改写为理想口语或用 quote extraction 丢内容。只处理已证实来自展示层的作者/时间装饰；用户自己写的 Markdown、日期、引用原样保存。

## 6. 高置信诊断与受控修复
在 `RunConversationReplyTask` 的 parse/native-receipt/media boundary 校验之后、prepared 保存之前复用校验入口。诊断分形式、已知事实/能力冲突、发送状态；长度、冷淡、不同意、括号、单个姓名/她都不作为错误。

作品请求范围来自当前明确任务与相关完整历史；不确定作品/文体仅记录。持续第三人称主语 + 多处旁白包裹台词/连续镜头等多信号组合才自动处置。保留规则局限，真实输出与人工判断验准确性。

高置信错误用同冻结 Cydonia assignment、同 persona/权威状态/真实媒体结果、任务书局部修复指令，候选显式是未发草稿。禁所有 tools、tool_choice、原生附件请求和发布；不使用 Qwen，不把“她触碰了用户”改写成“我刚触碰了你”。修复只改媒介，邀约/态度/主动程度/拒绝/Goal策略保持；语义保真不能仅靠规则声称成功。

在同 durable draft 的现有 snapshot 中 CAS 消耗 style repair quota（≤1），保存 claim、首稿、诊断、修复稿、终稿和版本/阶段/hash；media adjustment 与重启共享该额度，不复用 resume_attempts，也不只靠内存 bool。失败/崩溃不能再次耗用额度。结合现有 lease、deadline、cancel、physical input预算和 bounded transport/envelope policy，不为文体错误开启 fallback 到 Qwen；OOM/超时/仍失败按既有显式失败路径终止未发 draft，不机械删段保底。

已有 MediaRequests / AssetIDs 由 server 冻结并带回最终 draft；文体规则只看正文，不过滤 image intent。首稿和修复候选只存私有 draft/已有授权诊断，不进公开历史/事实引用。最终 Qwen 回读 publisher 的真实提交结果。

## 7. 基线、消融和诊断交付
温知意任务书附录标 reconstructed；另外三类人格若非真实配置标 synthetic。A 为修改前实际装配冻结输入，B仅协议替换，C加媒介/人格/状态视图，D加可靠历史展示与受控修复。均保持相同 fixture、模型绑定、采样和工具版本，干净/原小说式历史分别运行；不永久注册对照生产框架。

使用现有 wire capture 在最终 HTTP request 层记录脱敏 messages / tools / config摘要，给分模块差异、选择理由/索引、raw/view hash与 source/version。真实运行配置未读取的项目明确未核验，不从常量推断已部署。首稿、修复次数、终稿、media/result/publication/cognition 同一 draft tracing。

结构/模拟/隔离PG/真实模型/人工人格判断分列。只把 Provider usage 当实际 tokens，估算单独标；延迟、物理调用数、错误/OOM各阶段记录。真实回放不向用户发布或修改真实状态，未能连接的项目明确 NOT_RUN，并交付既有 test入口及前提。按原本地/用户真实环境分工，不自动从 .env 旧服务读取密钥或部署。

## 8. 风险与回退
自然语言形式检测不可能覆盖所有人格/作品/事实；宁可记录存疑，也不能粗拦正常人格。修复后的主动性/人物魅力仍需真实样本人工判断，不用统一分数或字数代理。

新增字段必须兼容旧 JSON snapshots 和旧设置保存；新 settings UI不能把表达版本无意丢掉。发布永远回读真实版本/正文。当前不存在本任务 build manifest，CI无 paths过滤，无需改workflow。最终明确 branch SHA、未构建镜像状态和已存旧清单来源，不自动 push/合并/部署。
