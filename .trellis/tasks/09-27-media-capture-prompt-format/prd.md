# 媒体拍摄视角与提示词格式

## Goal

恢复自拍和第一人称契约并收敛媒体上下文及多模态结构化文本

## Confirmed facts

- 当前 `media.image.generate` 只有自由文本 `intent`；media concept allowlist 虽支持 capture/camera/framing，但普通能力路径不结构化声明这些值。
- `MediaPromptInstruction` 固定“写真”“不是普通自拍”，与 `.trellis/spec/backend/media-prompt-contract.md` 中已有的自摄/镜头物理规则冲突；相关断言目前被注释。
- 初次 media prompt 出站是 YAML；`media_quality_acceptance`、quality retry 和两个 Visual Identity 多模态 text part存在 JSON-string 嵌套/未格式化问题。appearance未知字段及零压力 drive给 context binding 添噪声。详见父任务 `research/confirmed-paths.md`。

## Requirements

- M1：恢复结构化捕捉关系，区分第一人称、手持自拍、镜前自拍、外部第三人称；显式 camera/angle/framing/device 约束优先，未指定时依已有媒体规格做保守自摄补全。基础指令不得用固定年龄、体型、写真审美覆盖已冻结的视觉身份和拍摄意图。
- M2：冻结 concept、Prompt Agent 与质量验收对同一 capture authority 保持一致；不得用 Go 关键词/人物计数推断视觉语义。
- M3：媒体 context binding 移除未知 body field、无视觉意义的 drive、重复描述和传输元数据，使用紧凑 TOON 表达实际有效语义。
- M4：媒体质量验收及同类多模态 text part以可读结构化文本传输，不嵌套转义 JSON；原生 ToolCall/ToolResult JSON 不变。

## Acceptance criteria

- [ ] 手持前/后摄、镜前全身、operator POV/第一人称、外部第三人称受控 fixture均给出物理一致的构图、设备和摄影者可见性；显式视角不被写真默认覆盖。
- [ ] 缺少拍摄指定时恢复已有 media contract 的自摄默认；冻结 concept 到生成、重试、验收的 capture 字段不丢失或改义。
- [ ] 已有视觉身份和当前外貌不被基础指令中的固定年龄/身形/写真模板覆写；显式用户风格仍可表达。
- [ ] 媒体模型输入不含 `{}` 未知身体字段、低意义 drive 的 description/direction/key 及不必要版本/时间元数据；TOON/可读文本不出现双层 JSON 转义。
- [ ] `media_quality_acceptance` 和 Visual Identity 多模态入口只格式化 text part，image URL/data 不被破坏；现有生成和质量重试测试仍通过。

## Out of scope

- 不让 Core 解析自然语言猜视觉内容；不更换 ComfyUI/Provider 或改原生 Tool 协议格式。
