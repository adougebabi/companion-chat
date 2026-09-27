# 执行计划

- [x] 审核 26 个 conversation Tools 的 provider-visible 参数与运行时校验，记录最小改动清单。
- [x] 在意图/当前活动唯一时由 Core 绑定目标；多候选返回明确选择错误，其他必要目标在紧凑 Tool 描述中说明。
- [x] 新增记忆结果返回可复用 ref；缩短模型收到的通用 receipt，保留查询答案与内部审计。
- [x] 通用 Prepare schema/缺参错误变为非 retryable Tool 结果；区分业务目标和 native ToolCall ID，不放宽传输关联。
- [x] 运行 Go Core 全包测试、`go vet` 和相关隔离 PostgreSQL Tool/ADK 集成测试，验证默认 Prompt 工具预算。
- [x] Trellis check 已复核并修复 replay/same-batch 模型结果泄漏及活动跨 profile 目标问题；复核后纯 Go 测试、vet 与焦点 PostgreSQL 测试通过。
