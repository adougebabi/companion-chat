# 摇光视觉身份工作流

## Goal

为摇光建立可持久化、可迭代、可追溯的视觉身份（Visual Identity）系统：摇光可以被触发去“创造自己”，通过纯文生图得到候选图，再经过“图像 → 视觉理解 → LLM 评审/补丁 → 再生图”的内循环逐步收敛，最终保存 canonical reference / character sheet，并让既有 Scene Image 生成沿用该身份约束。

## User value

- 用户能看到每次视觉身份工作流的明确时间轴、阶段、状态和生成图片，而不是只看到一个最终结果。
- 用户可以在候选图“不像自己”时再次触发生成，且每一轮都保留历史证据，不覆盖旧图。
- 摇光的外观连续性由持久化的 visual_identity 和 renderer constraints 保证；胸部 LoRA weight 作为 -10 到 10 的可调参数随身份保存并传入渲染框架。
- 本次只搭建工作流、状态、事件、存储和 API/UI 框架；具体生图 JSON/ComfyUI 节点内容由用户补全和测试。

## Confirmed facts from the repository

- 活跃后端是 Go Core + Go Worker，Temporal 负责 durable workflow；公共浏览器边界是 Go BFF，前端是 Vue 3 + TypeScript。
- 现有 capability/runtime 已有 `media.image.generate` 外部异步 slot、`scene_event` 原生 slot、稳定 intent/workflow/provider IDs、media intent/asset/reference 表和 NDJSON/SSE 事件路径。
- 现有媒体链路通过 `media.comfyui` runtime setting 读取 ComfyUI workflow JSON，并用 `{{prompt}}` 替换；本任务不能假设用户尚未提供的节点 JSON。
- 现有 Fluctlight detail 已暴露 `drive_slots`、`preference_slots` 和 `trigger_preferences`；typed slot 具备 revision、evidence、provenance、outbox 变更事件。
- 现有 workflow 控制/诊断 API 能查询状态和 history，但没有 visual-identity 专用进度时间轴或图片卡片。
- 现有媒体资产是私有对象，经 BFF 代理读取；生成结果可绑定 conversation message 或 moment。
- `media.image.generate` 当前只接受开放的 `concept` JSON，Media Worker 会把 `media_intents.prompt` 交给 `media_prompt` Provider role，再注入 `media.comfyui` workflow；不存在 visual identity 专用 schema/role 或快照字段。
- 当前 `ContextProjection` 是媒体上下文的共享入口，但外观绑定只读取 `projection.Identity["appearance"]`；创建时默认外观实际位于 `life_profile.appearance`，需要在本任务中建立明确的 visual identity 投影，避免层级不一致。
- 当前 trigger preferences 只有持久化/读回，没有 evaluator、timer 或到 visual-identity intent 的生产闭环；因此 Visual Identity 初始化需要自己的明确 trigger/intent producer。
- 前端已有聊天消息图片展示和 NDJSON `media` 事件类型，但 conversations store 尚未处理 `media` 事件，也没有 media attempt 状态、质量反馈或“不是自己，再次生成” API/UI。

## Requirements

### R1. Visual identity slot

新增 `visual_identity` slot，对外遵循现有 typed-slot 的读取/修订语义，内部使用专用 `fluctlight_visual_identity` 聚合/表保存可版本化的身份描述、canonical reference/character sheet 引用、renderer constraints、provenance/revision 和生成轮次关联；读写遵循稳定 ID、CAS/幂等、outbox 和 detail 投影约束。身份数据不能被普通 Scene Image 生成隐式覆盖。

### R2. Initialization trigger

新增明确的 `Visual Identity` 初始化 trigger。触发后创建一条可恢复的 durable workflow intent，并在时间轴中记录 queued/running/awaiting-review/completed/failed 等阶段；重复触发复用或开启可区分的新一轮，而不会产生重复外部副作用。

### R3. Pure text-to-image creation workflow

新增只依赖文本概念和用户提供 workflow JSON 的首轮生图阶段，用于创造“自己”。框架负责输入/输出契约、持久化、provider request/job ID、媒体资产绑定和进度事件；不负责替用户补全 ComfyUI 节点 JSON。

### R4. Image → vision → LLM → patch → regenerate loop

工作流支持对每轮候选图进行视觉理解，调用 LLM 生成结构化 patch，再把 patch 应用于下一轮渲染输入并重新生成。每一轮必须保存原图、vision 结果、LLM patch、输入快照、状态和错误；失败或取消可从最近稳定检查点恢复，不能丢失已完成轮次。

### R5. Canonical reference / character sheet

工作流完成后可将用户/LLM 选定的结果标记为 canonical reference，并生成/保存 character sheet 资产及其元数据；canonical 选择是显式、可审计、可回滚的版本变更，不删除历史候选图。

### R6. Scene Image integration

改造现有 Scene Image workflow/媒体概念输入，使其接收当前有效的 `visual_identity` 快照和 renderer constraints；未初始化时保持明确的 identity-missing/pending 状态，不通过代码猜测默认外观，也不改变既有 scene/context authority。

### R7. Chest LoRA renderer constraint

用户在身份设定中填写语义化的胸部尺寸（例如罩杯 A/B/C），Core 通过专用 adapter 将其转换为 workflow 使用的胸部 LoRA weight。映射表不是散落在代码中的不可见常量，而是可配置且带版本的 renderer mapping（例如 A→-5、B→-3 只是待验证的配置示例）；解析后的 weight 严格限制在 `-10..10`（含边界）。正式 visual identity 同时保存语义输入、解析后的 weight 和 mapping 版本；每轮渲染冻结当时的三者快照，Scene Image 和 Visual Identity workflow 都读取同一约束。映射缺失、越界、NaN、非数值或不支持的罩杯按可诊断配置错误/pending 处理，不静默猜测。

### R8. Timeline + image presentation

提供面向用户的工作流时间轴：每个阶段显示时间、状态、轮次、简短说明和关联图像（候选图、vision/patch 摘要、canonical/character sheet）；支持“觉得不是自己，再次生成”的后续触发，并保留前后图像对比和关联 workflow/asset IDs。图像使用现有安全媒体代理，不直接暴露 provider/MinIO 地址。

### R9. Contracts and compatibility

更新 Go domain/API/BFF、browser client、Vue store/view、迁移、OpenAPI/契约测试和 worker registry；保持现有 media.image.generate、Scene Image、NDJSON/SSE、workflow control 与旧数据兼容。新的 provider-facing JSON 字段允许留作用户补全的扩展点，但结构校验和安全边界必须先存在。

## Acceptance criteria

1. 可在新的独立工作树中启动并运行 Visual Identity 初始化；初始化会产生稳定 intent/workflow ID 和可查询的阶段时间轴。
2. 首轮纯文生图可提交用户提供的 workflow JSON，生成候选图片并持久化 media asset；没有 workflow JSON 时返回可诊断的 pending/configuration 状态，不伪造成功。
3. 至少两轮内循环可被驱动：每轮都按 `image → vision → LLM patch → regenerate` 顺序留下持久化记录；重启/重试不会重复已完成 provider job。
4. “不是自己”后再次触发会创建新的轮次/检查点并在 UI 时间轴中显示旧图与新图，而不是覆盖旧候选。
5. 明确选定的候选图可以成为 canonical reference，并关联 character sheet；读取 Fluctlight detail 能看到当前 visual_identity 版本及 renderer constraints。
6. Scene Image 请求携带同一份 visual_identity 快照和胸部 LoRA weight；未初始化时返回可解释的 identity pending/required 状态。
7. 胸部 LoRA weight 接受且仅接受 `-10..10`，持久化后跨重启/重试保持一致；越界输入不会创建 provider job。
8. 所有新时间轴/图片 API 通过 BFF 授权和安全媒体代理；前端刷新后仍能恢复相同状态，且无 provider 内网地址泄露。
9. 现有 Go Core、Worker、BFF、browser-client 和 web 测试/构建通过，新增契约测试覆盖幂等、取消/失败、恢复、CAS/revision、越界和旧数据兼容。

## Out of scope

- 本次不替用户设计或填充 ComfyUI/Comfy workflow 节点 JSON、模型选择、采样器参数或具体 vision 模型 prompt；只定义可插入的 JSON/adapter 契约。
- 本次不实现通用视频/音频身份、不扩展多人物身份管理、不改变现有人格核心层语义。
- 本次不删除或覆盖历史 media assets，不绕过现有媒体权限、BFF 代理或 Temporal 单运行时约束。

## Resolved product decisions

- `visual_identity` 采用“专用 `fluctlight_visual_identity` 聚合/表 + 对外 `visual_identity` slot”方案；不把轮次、资产关系和 canonical 版本压缩进通用 preference slot JSON。
- “再次触发”采用 session/attempt/revision 三层语义：每次生成是同一 Visual Identity session 下的新 attempt；“不是自己，再次生成”保留上一轮并标记 `rejected_not_self`，新 attempt 继承身份快照、renderer constraints 和用户反馈；只有显式选定 canonical 才提升 visual identity revision。未结束的 workflow 继续，已结束的 workflow 以同一 session 的新 attempt 重启。
- 产品入口限定为两个内建触发点：Fluctlight 初始化时自动触发 Visual Identity 初始化 workflow；Wake-up 发现当前没有有效 visual identity 时，由摇光自己的认知/行动链表达“需要创建自己”。不以独立按钮/API 作为产品入口；如实现需要内部命令，必须复用同一 Core/intent 边界，不新增第三套语义。
- Wake-up 缺失身份时采用“提醒 + 同次自动排队”：同一 wake-up 先让摇光表达需要创建视觉身份，再创建或复用 Visual Identity session 并幂等排队首轮 attempt；已有运行中/已完成 session 时不重复排队。
- 内循环配置为独立的 `visual_identity_vision` 与 `visual_identity_patch` Provider role，分别承担候选图视觉观察和身份收敛 patch；不复用普通认知/反思 role 的领域语义。
- 内循环由 `visual_identity_patch` 返回 `accepted` 或 `regenerate` 决策；`regenerate` 在有界 attempt 次数内继续并保留历史，`accepted` 才提升 canonical 并生成 character sheet。
- 初始化首轮由 `visual_identity_patch(stage=seed)` 基于 Foundation/visual identity 结构化快照生成纯文本 seed prompt；视觉信息不足时显式进入 pending/awaiting-input，不由服务端关键词或默认值补全。
- 胸部外观采用“语义罩杯输入 → 代码内专用 adapter（明确 if/else 映射）→ `-10..10` LoRA weight”的双重持久化；映射变化频率低，不新增 settings 配置面；每轮冻结输入、解析值和 adapter/schema 版本，只有 accepted attempt 才提升正式约束。

## Open product decisions

- 罩杯语义字段先支持常见罩杯字符串（至少 A/B/C/D，大小写和空白归一化）；不支持的值进入 `renderer_config_pending`，不做自定义值的隐式线性外推。自动 regenerate 最大 attempt 次数默认为 3。
