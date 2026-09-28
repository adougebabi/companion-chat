# 技术设计

## 边界与数据流

真实 Provider 输入以 Eino/OpenAI-compatible HTTP request 为准。Core 的 typed task input、持久化实体、`ContextProjection` 和 Tool receipt 可以保留业务 ID、修订号、哈希和恢复坐标；只有进入 `messages[].content`、多模态 text part、原生 Tool result message 或 Provider response schema 的内容才按“模型必须使用”原则投影。诊断同时保留结构化消息容器和可直接阅读的正文，不能把 JSON envelope 当成提示词正文。Provider 绑定仍沿用 `generic_llm`/`embedding`，`endpoint_id`、`model_id` 只记录来源，不重做展示。

## 诊断记录

非 ADK physical run 写入可辨认的调用元数据；读取层对它们按逻辑 run 与稳定物理请求顺序给出 sequence，对 completed/failed/cancelled/timeout/queued/running 状态推导阶段。ADK 仍以 `adk.model.input/output` 事件和 `model_call_id` 为权威，不用序号猜 Tool/最终语义；外层 ADK completion 不再额外写一条假物理轮次，读取时也排除同一逻辑 run 已有真实物理行的历史外层摘要。Preflight 失败可诊断，但不虚构 physical sequence。只有缺乏新元数据及事件的真正历史行保留 unknown。UI 仅将这些状态翻译成标签，关联和排序在服务端完成。复用 `.trellis/tasks/09-27-adk-diagnostics-display/design.md` 的字段语义，不新建另一套 DTO。

诊断写入、读取与导出统一使用**仅图片替换**规则：识别 data:image URL、原始图片 base64 字段、`image_url` 多模态块（含远程/签名地址）和其他已知图片 payload，在入库前以 `REDACTED_IMAGE_DATA` 代替；所有其他可用文本和结构化字段原样保留，包括 Tool arguments、reasoning 字段、原始错误文字、凭证值和初始化 Prompt/Response。初始化的 metadata-only 包装在新记录路径移除，并修正 ADK physical row 与外层 row 的策略不一致。旧 metadata-only / `[REDACTED]` 行保持原样，不做伪恢复。Owner 授权、现有保留期和导出边界不变。该选择会使凭证及隐藏推理进入 Owner 诊断库与导出，应在 `.trellis/spec/backend/fluctlight-diagnostics-contract.md` 中明确改写，而非让代码与规范互相矛盾。

`einoMessageRaw` 必须序列化 `UserInputMultiContent` 的 text/image parts。图片实际发送给 Provider，但 model-run 保存的同位置 part 为占位；Tool call 和响应结构不因诊断转换而改变。页面对每条消息优先展示 role、text parts 与图片占位，保留查看完整结构化容器的能力；原有 `endpoint_id` 展示样式不变。验证同时捕获 HTTP wire 和诊断行，确保“看到的 Prompt”对应真实请求。

## 模型可见输入投影

为每类任务在发送前构造明确的 provider-facing allowlist，避免对数据库 DTO 做全局递归删键；相同底层字段可能在一个 Agent 中是存储元数据，在另一个 Tool 中是合法选择键。测试以真实 HTTP body 或下一轮 Eino Tool message 为准。

- Visual Identity：Core 保持完整 session/state/renderer 与资产查询数据；Agent user text 只含当前动作、进度、必要视觉身份事实、候选状态、审查历史与反馈，以及实际图片。去掉 session/Fluctlight/media/asset ID、非视觉 `background`/`background_story`、重复 required sections/views 或 snapshot 副本。不能损失 `generate_candidate`、`commit_review`、`finalize` 的恢复和一次性审查契约。
- Schedule generation：将 goal/intention/recent outcome/current life 变为语义 allowlist，剔除 `foundation:*`、存储 ID、evidence/revision/digest 与同值别名；Core 在结果提交时继续自行绑定 evidence refs。日期、时区、约束、活动、意图、状态、最近结果语义保留。
- Media quality/prompt：共享视觉字段 allowlist，只保留模特/外貌/服装/画面/场景和已冻结的 Prompt/重试语义；`background_story`、CAS revision、opaque DB refs、LoRA adapter 权重留在 Core/ComfyUI。Media prompt 对无效 frozen concept 在 Provider I/O 前报错，不再原样透传未知字符串。候选真实图片和重试反馈不改变。
- Wake-up/current-life：隐藏已由 Core 绑定的 `wake_up_id` 和重复嵌套 `life_context.schedule_ref`，移除无模型消费者的 `context_revision`；独立 schedule fact 保留当前活动/时间/状态。`cycle`、`schedule_status` 和模型输出 `influences` 所需的合法 opaque refs 仍可见。
- Schedule replan：使用 planner 专用语义投影，避免顶层/嵌套 Schedule 及 `items`/`current_item`/`upcoming_items` 重复；去掉普通 ref/CAS 修订号。保留已链接 item 的 `intention_id`，因为结果 schema 要精确保留它。`expected_revision`、完成边界等由 Core 在结果规范化/提交时绑定，继续执行 CAS 校验。
- Persona compilation：只发送选定人格语义一次和 `target_max_runes`；`profile_id`、`rules_version`、source/hash/revision 留在 Core。修正该场景复用“解析 Owner 描述”规则所产生的不相称指令。普通 personality switch 的 profile/rule 选择 ID 不受影响。
- Reflection/native/daily/virtual activity：保留合法 evidence/target/influence refs、activity/worn-item 等 Tool 选择键；删除非输出/Tool 参数所需的 policy revision、profile/intention 存储 ID、重复 action text 和 current-life CAS 元数据。Virtual activity 先用真实请求测试确认候选字段，再仅删除已证实无消费者者。
- Tool results：`CapabilityResult` 完整持久化供 Core settlement；下一轮模型消息改用 capability-specific 可见字段投影。mutation 只回传状态、可操作失败信息与必要 chaining key；image/scene/presence/schedule/moment/habit/memory/life/intention 结果不泄露 workflow ID、inbox ID、revision、hash 或 replay 标志。检索类结果保留后续 Tool 接受的选择 ID 和引用。

初始化的模型正文已经只有 Owner 描述；summary/segment/daily memory 的输入已较窄。已注册但无生产调用的 takeover judge/reply 与旧 Visual Identity vision/patch 记录为审计发现，本任务不为其扩大兼容面。对话 response schema 根/子结构重复属于另一输出契约问题，不并入本次入参修改。

## 兼容、验证与回退

无需迁移现有诊断表或业务状态。新记录采取新保留规则，旧行按存储事实展示；只对确实能证明为新格式的单轮记录补全阶段，避免把历史未知行伪装成最终结果。清理只发生在模型边界，Core 所需 ID、引用 index、durable intent、CAS 和重试检查保持原值。按每个 producer 先建立能在当前代码上失败的请求级回归，再改投影；对多模态及 Tool 往返验证真实 Provider 请求与诊断记录。相关测试和规范变更应同一批完成。若某字段删后破坏模型选择或业务提交，回退该场景 provider-facing 投影，不回退 Core 持久化数据。

`request_timeout` 先通过 Settings 的 `generic_llm.timeout_seconds` 配置：其值必须覆盖预期全部模型轮次、排队与 Tool 总耗时，同时不超过对应 Temporal Activity 的有效剩余期限；实际运行值需在运行环境读取。此次不更改 60/120 秒默认、初始化 600 秒下限、15 分钟 HTTP client 或工作流列表独立 5 秒 deadline。
