# Model入口 → Task/Prompt/Runtime 迁移表

| 入口 | 之前 | 当前 Task 边界 | Provider/Runtime | 领域提交仍由 |
|---|---|---|---|---|
| `AnalyzeDescription` | 调用方先构造 initialization messages | `InitializationTaskInput{Description}` | Eino structured initialization | initialization semantic validator / foundation transaction |
| `ProcessMediaIntent` | `RunTextTask(task, messages)` | `RunMediaPromptTask(MediaPromptTaskInput)` | media_prompt Text | media intent/provider job |
| `evaluateMediaQuality` | `RunStructuredTask(task, messages, schema)` | `RunMediaQualityTask(MediaQualityTaskInput)` | media_prompt multimodal structured | quality verdict/retry |
| `ProcessVisualIdentity` vision | raw system/user/image + schema | `RunVisualIdentityVisionTask` | visual_identity_vision structured | timeline/asset transaction |
| `ProcessVisualIdentity` patch | raw review messages + schema | `RunVisualIdentityPatchTask` | visual_identity_patch structured | accepted/regenerate transaction |
| `ProcessConversationSummaryIntent` | raw source_messages + schema | `RunConversationSummaryTask` | reflection structured | source digest/summary settlement |
| `generateInitialSchedule` | raw schedule messages + schema | `RunScheduleGenerationTask` | cognitive_assessment structured | schedule continuity/acceptance |
| `providerSchedulePlanner` | raw planner messages + schema forwarding | `RunScheduleReplanTask(SchedulePlanInput)` | cognitive_assessment structured | schedule capability settlement |
| `ProcessNativeCognition` | caller assembles surface and calls generic tools wrapper | `RunNativeCognitionTask(NativeCognitionTaskInput)` | Eino structured tools | frozen cognition/action settlement |
| `ProcessDailyReview` | caller assembles daily review and calls generic tools wrapper | `RunDailyReviewTask(DailyReviewTaskInput)` | Eino structured tools | autonomy/action/outbox transaction |
| `assessPersistentSwitchAfterCandidate` | caller assembles switch assessment and forwards schema | `RunPersistentSwitchTask(PersistentSwitchTaskInput)` | Eino structured no-tools | persistent switch authorization |
| `processReflectionV2` | caller assembles reflection and forwards schema | `RunReflectionProposalTask(ReflectionProposalTaskInput)` | Eino structured | evolution/overlay/memory transaction |
| Main / takeover / WakeUp ADK | ADK bridge accepted raw `Messages` and `Schema` fields | ADK bridge accepts one `PromptAssemblyResult`; surface-specific caller still owns projection assembly because it is the frozen context seen by the loop | shared Eino + ADK Runner, request-scoped capability trace | existing turn/wakeup freeze, prepare and settlement |

No production caller uses the deleted generic `RunStructuredTask`, `RunStructuredToolsTask`, `RunTextTask` or `RunStreamTask` wrappers. Embedding remains a separate typed contract, and frozen embedding assignments continue to use the pinned assignment on retry.
