# 规范化修复预算与 Activity 重试

现有 MediaWorkflow 的 Activity 最大尝试3次。仅限制规范化函数内部循环为2次，仍可能在最终校验失败后由 Activity 再次运行模型。修复后的 Core 返回 `CapabilityError{Code:media_prompt_validation_failed,Retryable:false}`，Worker 保留 cause/type 并使用 `temporal.NewNonRetryableApplicationError`，第一尝试即记录终态错误。普通网络错误与取消不转换。首次结构响应无法解析也可使用一次修复，修复阶段基础设施失败同样终止预算，不重新启动规范化。

通过 Context7 在沙箱外核实：
- library Temporal → /temporalio/documentation（High，12864 snippets，85.39）。
- docs query: Temporal Go SDK NewNonRetryableApplicationError function signature and example for stopping Activity retries after a bounded validation failure。

当前官方来源：
- [Non-retryable errors](https://github.com/temporalio/documentation/blob/main/docs/design-patterns/non-retryable-errors.mdx)
- [Retry policies](https://github.com/temporalio/documentation/blob/main/docs/encyclopedia/retry-policies.mdx)

文档确认构造函数 `(message, errorType, cause, details...)` 的非重试标记阻止 Activity Retry Policy 再试。新的 TestMediaValidationFailureDoesNotRestartNormalization 运行实际 ProcessMediaActivity 处理器与 SDK 测试工作流，断言一次处理、一条终态记录，保留 media_prompt_validation_failed 类型。TestMediaActivityKeepsTransportRetryAndCancellation 断言普通传输错误和取消保持原行为。测试不是实际 Temporal 部署验收。
