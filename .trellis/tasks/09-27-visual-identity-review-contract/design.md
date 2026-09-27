# 设计

Visual Identity Agent 仍一次性查看真实图片并调用专用 `commit_review`，不恢复旧的 Vision/Patch 双 Agent。统一观察字段的内部表示为有界字符串数组。Tool 边界接受该数组，以及有界单条字符串或结构化对象；对象只把可解释的文本值按稳定键序变成观察条目，拒绝空值、无界嵌套和非文本数据。Provider schema、Go 校验与规范化逻辑须保持一致，Agent 指令明确首选数组。`missing_sections`、评分和反馈字段在规范化后仍按现有业务规则检查，不由 Core 虚构视觉判断。

通用的 `ErrInvalidArguments` Prepare 分类由 `09-27-capability-contract` 子任务先完成：模型可纠正错误返回非 retryable、有界字段名/预期类型的 Tool 结果，而不是可重试系统故障。本子任务据此允许同一 Visual Identity run 在既有请求期限内修正调用，并把进度验证改为统计成功提交次数；无论前面有多少无效尝试，成功审查至多一次。遵守 Structured Turn 契约，不额外施加固定模型/Tool 轮次上限。依赖/数据库/授权错误仍遵守原有失败策略，不被吞掉。

用于诊断的 Tool 参数必须继续脱敏；可展示参数键、错误码和 schema 失败字段，不保存完整模型原始参数。旧 session 不迁移，失败 session 通过既有恢复机制重新执行。
