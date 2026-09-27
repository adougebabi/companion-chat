# 执行计划

- [ ] 定位 `diagnostic_model_runs` 与 `adk.model.input/output/tool.*` 的现有关联字段，设计 bounded Owner-only 轮次 DTO。
- [ ] 将轮次/阶段/安全 Tool 摘要贯穿 Core read、Browser BFF、browser-client 和诊断视图。
- [ ] 按 correlation 分组、稳定排序，标注中间 Tool 决策与最终/失败 Response；时间显式格式化。
- [ ] 验证旧行兼容、两轮一工具、三轮失败、刷新/过滤及公开聊天无 trace 泄漏。
- [ ] 运行 Go BFF/diagnostics 测试、browser-client 测试、Vue build 和相关界面检查。
