# 实施与验证顺序

1. 固化基线：记录原有 diff、配置与真实请求链路，用同一样本量测各部分和总输入。
2. 核验模型标识、Eino/OpenAI 适配器模板与输出字段；修复计数模式、硬限、显式输出冲突及 Provider 生成上限对齐，使每次物理调用与直接调用共用预算策略。
3. 精简重复协议与 Persona/Current State，修复 Runtime 面向任务的投影，保留动态事实、引用及人格/行动语义。
4. 完成摘要、原文、检索记忆的 source-ref 覆盖和低价值重复控制；保留不同事件、跨天未完成事项；限制无变化 WakeUp 的重复记忆写入。
5. 对大工具结果提供真实可操作的模型视图或分页；验证原始收据、Eino 配对、三轮以上续接与状态替换。
6. 补齐每轮预算、收敛、usage 诊断和分类错误；删除被替代的完整 dump/重复预算路径并迁移调用方。
7. 运行定向 Go 单测、Agent Loop Mock、相关全量回归；真实 tokenizer、Provider、长对话回放按环境可达性执行，未执行明确标记。
8. 生成 Markdown 验收报告，含同一样本前后对比、证据、剩余绕过路径及风险；完成质量检查、必要 spec 更新与任务收尾。

## 验证关卡

- 在 `apps/core-go` 运行 `go test ./internal/core ./internal/ai/agent ./internal/ai/model` 及受影响模块定向测试，随后运行 `go test ./...` 和仓库现有 lint/type/build 命令。
- 对消息构造、三轮 Tool 续接与 30 轮历史使用受控测试；真实 Provider 证据与 Mock 分开记录。
- 检查 `git diff --check`、`git status --short`，确认初始未提交内容未丢失且没有无关重排。
