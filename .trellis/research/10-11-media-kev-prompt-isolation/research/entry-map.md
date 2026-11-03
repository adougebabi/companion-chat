# P0 入口审计（2026-10-11）

两路只读探索，主代理点验关键Prepare/hasCurrentCapture/default selfie/模型路由/Kev配置/最终提交片段。未查询运行数据库、未调用真实Provider/Comfy。

| 环节 | 证据与含义 |
|---|---|
| 来源 | formal_agents.go:70–71；adk_conversation_runtime.go:204–215；tool_execution.go:176–250，Conversation/WakeUp/direct统一ExecuteTool及Prepare |
| 能力上下文 | tool_contract.go:250–281固定RequiredContext四Slot；builtin_capabilities.go:321–374无条件绑定并强制校验身份/生活/外貌/内在状态 |
| 资料查询 | app_capability_context.go:105–160，effective life、visual detail、inner state；未要求CorePersona |
| 冻结 | builtin_capabilities.go:399–428版本校验；mutations.go:644–647写media_intents.prompt |
| 分支 | current_capture.go:27–28按appearance非空判断人物路径，不按是否本人入镜 |
| 规范化 | model_tasks.go:89–105覆盖普通指令，模型仅生成封闭姿态/镜头计划；media_quality.go:245–263；provider_context.go:2247–2440仍传本人身份/外貌 |
| 渲染 | current_capture.go:305–383追加身份、穿着和当前场景；:467–470默认selfie；scene例外无人物但使用当前场景，不能声称所有结果必有人 |
| 最终正文 | media.go:119–128精确对比Core渲染；仅改LLM常量不足 |
| Kev | internal/ai/decision/decision.go:21,51,80,195七点严格map及yes/no/unclear解析；:252,480,503批量/Admit/Finish；core/kev_decisions.go:34,58 settings kev与kev:systemone |
| 工作流 | media.go:499普通workflow/VI变体；:137–163身份reference上传与最终/prompt；:320 renderer merge；:531 placeholder递归；:578–615普通图允许固定字符串 |
| 防线 | current_capture.go:509–545限制正向prompt及LoadImage，但只覆盖当前路径；current_capture_test.go:73,95固定服饰污染fixture。运行workflow是否污染尚未知 |
| 配置 | core/settings.go:118 allowlist无prompt template key；AI media instruction与VI模板是代码常量；media.go:452/media_quality.go:266 provider_prompt持久复用 |
| UI/操作 | KevSettings.vue:7–13固定points和默认；cmd/kev-config/main.go:19 Owner设置；迁移不覆盖已有设置 |
| 诊断 | media.go:344最终 request_payload.prompt可追踪，Kev审计/spool复用 |

## 现有回归抓手
media_context_test.go：TestImageCapabilityPrepareConsumesResolvedContext、TestImageIntentAndCanonicalContextReachMediaPromptInput、TestSceneMediaPromptUsesCurrentAppearanceWithoutHistoricalVisualSnapshot。
current_capture_test.go：TestCurrentCaptureFinalMediaWorkerSubmitsOnlyFrozenFactsAndRejectsOverrides、TestMediaPromptStandardCaptureSupersedesVagueUpstreamHints、TestCurrentCaptureInvalidEnumFallsBackToFirstPersonFullBodyAndKeepsFrozenFacts。
media_quality_test.go：TestMediaPromptConceptFiltersRequestAndCharacterMetadata、TestMediaPromptRejectsInvalidFrozenConceptBeforeProvider。
现有七点decision/workflow TestKev；真实Provider env由provider_live_tool_test.go:470定义。真实VI还需要FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE及隔离S3/Comfy。现有live durable tool测试不代表完整图像生成。

## 证据边界
已确认代码级人物上下文绑定、默认自拍及Core正文拼接。尚未确认数据库定制workflow的固定身份/LoRA、历史消息泄漏或运行时模板覆盖；不把这些可能性写成根因。
