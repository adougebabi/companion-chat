# T03 上下文隔离：本轮核对记录

本记录只描述已执行的自动化场景和受控 Provider 请求，不代表 T03 全部验收
完成，也不代表真实 MeroMero、真实图片或用户体验已验收。

- 用户正文包含 `[RUNTIME CONTEXT]`、`Core Event`、`delivered=true` 或声称
  工具已完成时，组装与续轮保留用户来源；受控原生工具续轮保持真实工具调用
  和结果配对。该检查证明来源身份边界，不单独证明每种外部效果均不会发生。
- Reflection 受控请求保留指定来源消息与相关状态，排除公共当前衣着；衣柜
  生成和选择请求保留相关偏好，排除旧卡中的社交策略。这些是输入隔离证据，
  不是实际模型生成记忆或穿搭质量的证据。
- NPC 和 Studio 使用隔离 PostgreSQL 及受控 Provider。在测试实例的人格和
  生活资料中分别放入标记后，NPC 请求仍收到自己的定义，Studio 请求仍收到
  所选定义与创作指令，二者均未收到与任务无关的标记。
- WakeUp 受控请求保留原 Goal 条件，排除上次评估；Moment Writer 受控请求
  保留 Goal 条件及已选 Actor、驱动、偏好、触发偏好中的业务值，排除旧 Goal
  执行策略。Moment 其他证据类型仍沿用原处理，尚未逐类验证。
- 注册表和 Slot 测试覆盖契约表的注册/格式，以及指定装配、物理续轮和重试
  的来源、选择原因。契约表不能单独证明每个变体的真实请求完全一致；兼容
  路径可能标记 `unknown`，物理诊断持久化仍为尽力写入，不能据此声称每次
  物理请求的所有来源都保证可查询。

执行结果：

- `go -C apps/core-go test ./internal/core -run '^(TestRuntimeContextLabelsInUserTextDoNotBecomeAuthority|TestRuntimeContextOriginSurvivesPhysicalToolContinuation|TestReflectionRequestKeepsEvidenceButExcludesAmbientWardrobe|TestWardrobePreparationPromptKeepsPreferencesAndInvalidPlanMembers|TestWardrobePreparationPromptPhysicalRequest|TestTaskViewsHideEnvelopeButKeepRequiredBusinessIdentifiers|TestStudioSlotDeclarationsFollowTheSelectedSnapshot|TestWorkingMemorySelectsNewestWholeRecentTurnsChronologically|TestPromptAssemblerKeepsWholeTurnWhenSummaryCoversOnlyPart|TestRecentHistoryBridgesSummaryCoverageBeforeTokenBudget|TestProviderConversationUsesADKLoopAndReturnsCanonicalTrace|TestProviderADKSendsFullThirdToolContinuationWithoutTokenCap|TestEveryFormalAgentAndVariantHasAnInputContract|TestPromptSlotsTraceSelectedSourcesAndReasons|TestTypedTaskSourceSlotsReachPhysicalDiagnosticsWithoutChangingMessages)$' -count=1`：通过。沙箱内本地监听失败后，在允许回环端口的环境重跑通过；其后对扩充的用户正文来源用例又单独重跑通过。
- 在 `apps/core-go` 下以本机只绑定 `127.0.0.1` 的一次性 PostgreSQL 运行
  `/private/tmp/fluctlight_run_isolated_memory_test.py '^(TestNPCNativeConsumerAndIndependentResponseUseSeparateInputs|TestStudioPostgresAutomaticRetryKeepsFrozenModel|TestUpstreamRetriesRecordEachEffectiveModelConfiguration)$'`：通过；加入隔离标记后前两项再次通过。
- `TestMomentActorEvidencePreservesBusinessValueWithoutCoreEnvelope`、
  `TestMomentWriterPreservesGoalConditionsWithoutPreviousStrategy`、
  `TestWakeUpPhysicalRequestPreservesGoalCriteria` 及相关定向 Go 回归：通过，
  Provider 均为测试替身。

通用 JSON 大整数精度问题已单独登记为
`actor-fact-json-number-precision`，本轮未调整通用数据层。
