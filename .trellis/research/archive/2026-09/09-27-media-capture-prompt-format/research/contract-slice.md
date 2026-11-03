# 媒体 Provider 规格摘录与冲突

- `.trellis/spec/backend/fluctlight-provider-contract.md:31-84`：每个正式 Agent保持模型角色、Tool结果继续/失败边界；物理 Generate各自排队、取消、诊断和预算；`agent_runs`防止已提交 Tool 后整轮重放。
- 同文件约 `:139-152`：`media_prompt`可用于 B 文本提示词与 C 多模态质量验收，C 的 transport、vision、timeout、schema错误须视为基础设施故障，不得推断为内容 pass/reject。
- `.trellis/spec/backend/media-prompt-contract.md:31-80`：概念由模型拥有，Core不得使用语义正则/默认枝；Prompt Master 先处理 capture/camera relationship，再处理主体、身份、场景、衣着、光线、风格与约束。明确自拍、外部拍摄、operator POV/第一人称与镜面/设备可见性规则。
- 当前媒体规格仍写“media_prompt user message 是 YAML concept”，而用户明确要求 compact TOON；实施须更新该段规格，并用最终 wire 测试证明不是仅改函数名。
