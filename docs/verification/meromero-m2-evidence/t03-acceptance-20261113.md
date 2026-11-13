# 原 T03 输入契约与证据投影验收

本轮继续原 `meromero-t03-prompt-context`。原记录中的已选外貌、其他证据类型、
全任务输入声明和来源覆盖分别核对。分组静态审阅是模型代码审查，运行记录来自
真实执行的命令。测试 Provider 使用受控响应。

| 原验收条目 | 当前实现及核查 | 运行证据 |
| --- | --- | --- |
| 各 Agent/variant 声明输入、来源、人格、历史、状态、工具及输出契约 | `docs/llm-task-input-contracts.md` 与 `agentModelVariants` 对照；分组审阅核对 `model_tasks.go`、`cognition_agent.go`、`reply_provider.go`、`diary_runtime.go`、`studio_runtime.go`、`world_encounter_response.go`、`moment_npc_interactions.go` 等实际组装。未确认生产上游的 takeover 与独立 vision/patch 入口继续按已有标记保留 | `TestEveryFormalAgentAndVariantHasAnInputContract`；45 项合并回归 |
| 权威状态、历史、真实工具回执、推断及候选按需分离 | Reflection surface 排除自动衣柜与活动；Writer 隔离父模型策略；Studio/NPC 使用自身作用域。Moment 已选证据及默认背景按生产类型投影，保留业务 JSON；身体事件按同角色已确认来源核对，Outcome 采用实际状态视图 | `t03-original-contracts.log`、`t03-appearance-postcommit.log`、`t03-typed-regression.log`、`t03-default-regression.log` |
| 保留原生工具与完整对话组，当前正文原值，遵循无 token 上限合同 | 共享 FormalAgent 组合保留原始消息、image parts、tool message；按完整组与来源覆盖组织历史。初始化 Owner JSON 保持用户数据身份。源代码的本轮 Moment 通用递归删键函数已移除 | `TestEveryFormalAgentSharesLosslessProtocolComposition`、`TestInitializationTaskPreservesOwnerJSONWithoutSystemPromotion`、同来源不同事实与 opaque 数据回归 |
| 实际请求可追溯 Slot 及来源，核对记忆/衣柜/NPC/Studio 隔离 | Typed Task 声明来源并到达实际请求诊断；媒体与人格子任务清除父工具强制策略；衣柜生成/选择实际请求使用任务投影；Reflection、NPC 与 Studio 字段边界逐项核对生产来源 | `TestTypedTaskSourceSlotsReachPhysicalDiagnosticsWithoutChangingMessages`、`TestWardrobePreparationPromptPhysicalRequest`、`TestReflectionRequestKeepsEvidenceButExcludesAmbientWardrobe`、Studio/NPC 视图回归 |

本轮修复链已有两个代码检查点 `2b90d4bd` 和 `8e24ad0d`。随后默认背景红测
`t03-default-red.log` 确认同源业务值仍被损伤，继续将该路径迁入类型视图。
`t03-default-regression.log` 的 45 个顶层用例退出 0、无 SKIP；含四处原有草稿的
`t03-default-working.log` 六个顶层对照退出 0、无 SKIP。

所有失败样本均保留。外角色夹具的两次初始化/约束错误见外貌报告；默认外貌
去重测试首次使用 `mapValue(nil) != nil` 作断言而失败，改为检查键存在性，见
`t03-default-test-correction.log`。此测试错误未引起生产策略调整。

已登记的通用 JSON 数值及媒体规范化质量重试反馈问题继续按用户授权作为
非阻断工单保留。本 T03 结论对应输入声明、组装和投影合同。原 M2 的真实
主模型能力、真实媒体成功链及其他里程碑验收继续按各自原条目保留。
