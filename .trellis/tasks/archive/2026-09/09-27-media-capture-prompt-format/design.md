# 设计

由模型在 `media.image.generate` 的 bounded Tool input 声明 capture relationship、framing/angle、设备/镜面可见性（未指定可留空）；Core 仅做 schema、大小、来源与冻结，不按文字语义补值。用户/人格已明确的要求随 `context_binding` 进入 frozen concept，再由 Media Prompt Agent 按 `.trellis/spec/backend/media-prompt-contract.md` 的 framing-first规则完成缺失视角。移除现行“不是普通自拍”以及固定年轻年龄、丰腴体型、写真审美等无条件模板偏置；视觉身份/当前外貌与显式意图优先，仅在完全未指定拍摄关系时使用规格中的保守前摄自摄默认。

`compactMediaConceptForProvider` 增加 media-specific projector：已知 body_fields变成简短可见值/状态，未知空对象过滤；current_state只保留与画面有关的 mood/显著 drive；视觉身份、当下穿着与用户显式 capture保留。Durable frozen concept依旧完整供 Core 重试/审计，Provider-facing copy才缩减。初次 `media_prompt` 的格式器改为 TOON-capable，必要时把嵌套 appearance/drive整理成同构表或简短 scalar，而不是仅切换 serializer 开关。

将 frozen concept 作为结构对象嵌入质量输入，统一 formatter递归处理多模态 `content[]` 中 `type=text` 的内容；`image_url`、bytes/data URL原样传递。Quality retry同样嵌套对象。用该路径核对 Visual Identity vision/formal agent 的多模态 text part，避免引入新的转义 JSON。媒体验收 Agent仍按 frozen facts检查而不新增视觉故事。

兼容：旧 media intent只有自由文本时照原有 bounded concept读取，缺失 capture交给 Media Prompt Agent，不在 Core 猜。预计修改 `tool_contract.go`、`builtin_capabilities.go`、`provider_context.go`、`internal/ai/prompt/prompts.go`/`format.go`、`media_quality.go`、两个 Visual Identity入口及对应测试。规格中目前“YAML media concept”与本次 TOON要求冲突，需同步更新为可读 TOON结构契约。
