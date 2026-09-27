# 设计

`ContextProjection` 维持摇光 Life Context 的单一权威，但 Provider-facing DTO/运行协议显式标记其 `subject=actor_self`。用户位置只能来自用户明确表达且授权的会话/Active Memory 事实，作为独立 `actor_user` 语义进入上下文；没有可靠来源时不提供“当前用户地点”。`scene_event` 的 Tool 描述和可执行参数契约明确目标为 `actor_self`，用户自述不是对该 Tool 的授权。以首次漂移 turn 的 Tool/`life_events` 证据区分错误持久写入和纯回复指代漂移；两种路径都加入受控模型/工具回归。

在 `compactRecentActionOutcomesForSurface` 前做 surface-aware 选择：已反映在 current_state 的成功 affect 结果、已出现在会话里的成功 reply、空 aggregate outcome不再重复投影；失败仅保留最新未被成功覆盖的可行动信号，待完成媒体/外部任务保留时间语义。减少 model-facing mutation receipt 的已刷入状态和已发布文本，保留状态、必要 target 和错误；仅在当前 ADK continuation 的 outbound view中排除同轮已发表 reply的重复 recent fragment，不动数据库和下一轮正常历史。

Current State 采用紧凑 appearance/drive 投影，保留与当前场景、身体、穿着、记忆引用及工具决策相关的值，省略版本/捕获时间等 Core 已持有元数据；零意义 drive不出现。必要 facts 在 Working Memory 第一层被标为 critical 或拆为较小的有界片段，超限留 trace/显式错误，不能静默消失。Runtime Context 保持现有 YAML/TOON hybrid；原生 Tool 协议 JSON 不改。

预计主要修改 `provider_prompt_composer.go`、`provider_context.go`、`working_memory.go`、`native_capabilities.go` 与 Tool receipt/refresh 相关代码及测试；若独立用户地点事实需持久化，必须先复核现有 ActiveMemory/关系 scope 后再确定最小表/字段，不把 Life Context 改成双主体共用。相关规格为 cognitive runtime、life-world、structured turn 和 memory contract。
