# 两处局部投影与结算边界设计

## 1. 架构与不变量

复用 BuildContextProjectionFor、人格编译、ContextReferenceIndex、WorkingMemory、AssemblePromptContext、原 Eino Runner 和 publication。局部具名只读视图，不新增通用上下文平台、持久化快照或第三个 Agent。

两处共享身份/时间/来源语义和 turn lineage，不共享整份 Runtime。工具后 Writer 重读已提交状态及有效人格；as_of 可以更新，需记录来源版本/刷新原因，不能为一致而冻结旧值。

```text
真实入站 + 已授权状态/人格
  → CognitionContextView → Qwen / 原工具目录
  → ExecuteTool / receipt / refresh
     → conversation.reply → 新有效投影/人格
        → ReplyContextView → Cydonia / reply.image.request
        → 原 draft / 媒体恢复 / publication
  → 原完整认知合同校验 / 一次隔离修复 / 原结算
```

真实 Tool 提交独立存活。只有原 conversation.reply 事务发布，final/response_plan/reasoning/修复提示不能发布；保留原生 ToolCall/result 配对。

## 2. 实际落点与隔离

- `cognition_agent.go`、`provider_context.go`、`projection_refresh.go`：仅 FactOnlyCognition surface 接入新视图，不再直接重映射为 ConversationMain；其他 Agent 默认不变。
- `reply_context.go`、`fact_only_reply_capability.go`：从结构化源生成 Reply View，替换通用 compact 后删字段路径。局部身份/历史、Goal、事实 helper 采用具名类型与 allowlist，不造泛化配置框架。
- `fact_only_cognition_protocol.go`、`eino_model_runtime.go`、`eino_model_request_policy.go`：阶段说明、原修复边界与最终请求元数据。
- `reply_media.go`、`reply_continuation.go`：仅服务端用途和旧 draft 恢复兼容，不改媒体状态机。
- Moments 也复用 Writer，必须按 Scene/Surface 选择私聊新视图，保持 feed 协议/工具/历史行为。共享 helper 有 Moments 对照测试。

## 3. 身份、时钟与历史

实际 self Actor、有效 profile、recipient 单独映射。称呼只来自已有解析结果和 Actor facts；无可靠称呼时表示对方角色，不用 actor_user 作昵称。Core 保存真实 ID；Qwen 使用当前工具契约支持的 Actor 标识/refs，不为了附件示例向 Provider 塞 raw ID。

当前时钟取 projection.as_of/原业务 Clock，消息时间取 created_at 和验证过的 sender snapshot；显式区分两者，显示时区不能证明对方地点。状态继续遵循各自 Life/Event/Presence/Actor-fact 有效期，可在只读投影补齐 activity.active_until 等已存在字段，不改过期规则。known/unknown/cleared/false 分开。

已证实生成的 sender/time decoration 改为元数据；正文夜间 Markdown 标题未证明来源，保留并视为文本。只有明确来源证明的前缀才允许模型视图归一化，记录原文/diff，不改库，不全局正则删除日期或 Markdown。

历史优先按 turn_id/source lineage 分组，缺失时沿真实连续 user 输入与后续 assistant 构成可证明片段，不按条数凑问答。必要时用原 RawHistoryReader 按已有来源回溯；确实缺失触发消息就报告 unanchored，不制造内容。连续 assistant 保留角色/顺序，当前输入优先以入站身份去重，旧相同文字不误删。

保护必要最近完整片段，历史仅真实 user/assistant。原 role/content 与作者/时间分开：通过已有系统数据槽附有序 history metadata，按最终选中的消息同步生成，不能索引错位、发明 role 或伪装用户发言。Runtime、Tool 与 Validation 留各自阶段。

## 4. 来源、事实与冲突

局部视图读取已授权 Actor facts、关系、Active/Resident/检索 Memory、摘要和真实结果。内部保留 source pointers、scope/主体、时间、认知类型、来源存活、置信度及可用 ref；Memory.actor_refs 只是关联集合，不能等同于主体。

- verified 不把 inference 升格；关系 category 不保证任意标签被原话支持。Writer 用来源类型/不确定性代替内部 ref/评估链，Qwen 用现有 registry 当前有效 refs。
- 仅同源或可确定同内容且 Actor/时间/scope 一致时归并显示，保留所有来源指针；同源不同说法保留差异，不被 SourceRefs 交集吞掉。不同日同文承诺不合并。
- 确定性比较限于同主体属性值、范围、状态/版本等明确结构；自由文本关系/承诺只保留必要来源摘录和未裁定说明，不新增 NLP、模糊去重、硬编码结论或要求用户重新证明。
- 引号归一化只处理来源明确的序列化展示层，保存 diff；不反转义任意代码/用户文字。
- 原始消息/summary ancestry 无 registry 条目时只作来源，不能伪造可执行 ref；旧局部 eN 不注册进本轮，不能正则删掉所有 eN 而损坏合法型号。
- Assistant 说过只证明说过；成图/发布读真实结果，未穿与未拥有分别读取。

字段缺失时允许局部扩展已有只读查询，不新增业务写入或表，也不把缺失默认为 verified。

## 5. Goal 专门投影

先复用 shared/current-profile 与 recipient 授权，再投影原目标、动机、原完成条件、状态、主体/对象、真实期限。Qwen 增加本轮操作需用的 refs 和真实依赖/失败；Cydonia 获全部有效可见目标简表，不按关键词、冷淡语气或 Qwen 推断隐藏。

Writer 不传 execution.next_step、current_stage.strategy、话术型 wait_condition/blocker、旧评估或重试治理。不能把这些字符串换名为事实。真实权限/dependency/attempt/outcome 来自既有结构；混合文字无可靠拆分时保持未知，不放开工具。needs_evidence 不等于现实未确认。root 原条件保留，不常驻完整 subordinate 执行对象。

已完成目标只给本轮相关最小完成事实，不重新激活。分别报 global_active_count/source_count/visible_count/selected_count 及过滤原因，源 map 不变。上游全局最近三个 completed 的配额、read-model 与 admission 的依赖差异记为来源限制；执行继续以原 admission 为准，不顺便改规则或全局查询。

## 6. 两处输入与协议

Qwen：稳定认知协议 → 身份/人格及切换授权 → 入站事件 → 必要状态/操作边界 → 有效目标/真实依赖 → 事实/冲突/当前 refs → 真实历史 → 当前输入 → 实际工具/完整认知合同。无关长 SOP 下沉原工具说明/准入详情，保留发现目录，不用关键词封锁能力。

Cydonia：精简协议 → 完整 Working Persona → self/profile/recipient → 本轮时钟、必要状态/关系/冲突 → 无旧策略的有效目标 → 有来源事实/实际结果 → 完整语境历史 → 当前输入 → 真实可用内部工具。以当前活动和必要确认安排代替全日表/重复 upcoming；无关 PAD、治理、技术长错误不常驻。

最终协议按附件 T6 结合真实工具语义交付，不用短句/安慰/禁止主动模板替代人格，不强迫讨论后台冲突。reasoning_content 不作另一模型输入。

## 7. Fact-only 参数与兼容

新 native closed schema 保留 evidence_refs、draft_ref、action_result_refs、必要 topic_key；去掉模型可写 purpose。不造 draft_id 或新社交字段，任意正文/语气/reply_intent/嵌套或改名策略由 closed schema + typed Writer 边界拒绝，不能只列黑名单。

publication 的 Purpose 仍由服务端填稳定回复用途，topic_key 只路由/去重，绝不拼 Writer。保留原窗口、inbound_sequence、awake/ownership 等检查；不能因移除模型 purpose 关闭抑制。legacy 原 schema 保留。

prepared/published 先复用缓存/receipt。旧 execution_scope 的 purpose/topic_key 仅原 draft 服务端恢复/去重；不因新 schema 重生正文。实施先核对 frozen run/continuation 版本边界，如需区分仅用现有 schema/prompt/run version，不造产品模式或迁移表。跨版本 replay、同 draft 恢复、改参数冲突和 proactive 去重必测。

## 8. 结算阶段与最终修复

真实 reply committed 后，物理请求用结算说明，读取实际结果并消除再次发送要求；保留原生循环合法的可选后续查询，不能为绑定最终格式关闭整个工具目录。

遵守现有约束：任何带实际工具的请求不发 response_format；自然停止用完整原业务 schema 严格校验。仅原一次 tool-free final-repair 绑定 schema，移除工具/choice/parallel 及业务/Kev/refresh 上下文，保留当前语境、有效 refs、实际配对和身份。普通采样/Thinking 不改，原修复 thinking=false 保持。

发布和结算分别记录。修复失败返回真实错误，不伪造 JSON、不改已发正文、不存 Validation 聊天、不重开整轮。业务级失败结算恢复若有缺口，局限原 Agent-run/receipt 机制补齐，并保留无重复发布围栏，不新增后台恢复服务。

## 9. 预算、实际诊断与回放

复用 WorkingMemory/wire hard budget，按重复/内部诊断 → 无关详情 → 远期完整历史删减。人格关键设定、身份/当前输入、权限、有效目标本意/原条件、实际结果、schema/工具配对受保护。Qwen 3k–5k、Writer 1.5k–3k 是实验参考，不新增硬上限或静默裁剪。

既有事件/ModelRun 逐物理请求补 phase/turn/correlation/model_call_id/as_of/来源版本/刷新原因/prompt_version/binding 与实际生效 sampling/choice/format presence、工具版本。分别报来源/可见/选择数及原因、一级不重叠模块估算、实际 usage/延迟/错误；嵌套 runtime.* 不重复求和，缺失 usage 为 unavailable。

通过原最终 payload modifier 抓 allowlist 配置摘要，先用本地 HTTP/SSE 验证组合顺序与最后序列化值，不直接把整个 raw payload 交日志 redactor。受控测试保留脱敏 wire；服务全量输入仅走已有授权诊断路径，不新增默认长 reasoning 收集。SDK reconstructed 与原 HTTP bytes 区分。

事故 fixture 分附件已知摘录、明确 synthetic 补充、missing 清单；旧装配输出冻结为 baseline，不保留第二套生产框架。上下文 A/B 固定模型/采样/工具版本；参数合同另组比较。默认 mock 模型/发布端，真实模型只提供用户 opt-in 的既有绑定入口，无真实用户发布。

## 10. 回退与限制

沿原 conversation.reply legacy/fact_only 设置、既有测试渠道/镜像说明，不增开关、不改 Actions。无迁移；待恢复 draft/continuation 排空沿原文档，不自动执行。

无法机械证明自然语言无策略/幻觉，旧关系/承诺/摘要可能仍矛盾，时间标题来源未知，真实体验与真实 token/延迟待用户回放。保留失败与来源，不能以数据修写伪造改善。
