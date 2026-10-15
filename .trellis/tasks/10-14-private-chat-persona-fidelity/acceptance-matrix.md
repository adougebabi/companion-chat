# R01—R36 验收结果

U=结构，W=受控实际SDK HTTP/模型与fixture模拟，P=隔离PostgreSQL+模拟模型，L=真实模型+模拟发布，H=人工语义检查。下列短测试名均为 `TestPrivateChat` 前缀（写出原有全名的除外），源码在apps/core-go/internal/core。PASS只限明确的层次；SKIP/NOT_RUN不能计作语义或持久化通过。

| ID | 场景 | 测试 / 证据 | 结果 |
| --- | --- | --- | --- |
| R01 | 旧配置缺version/新协议选择 | ExpressionSettingsAndFrozenContract；ExpressionProtocolOnlyWireComparison | U/W PASS；运行DB配置 NOT_RUN |
| R02 | 旧神态规则不追加 | ExpressionProtocolOnlyWireComparison，最终HTTP system | W PASS |
| R03 | 空communication→text_chat，co unknown | MediumUsesCurrentSources/empty | U PASS |
| R04 | 生活圈/常去店/找找不推到场 | MediumUsesCurrentSources/relationship_background | U PASS；L NOT_RUN |
| R05 | 未知位置不造动作 | live ablation/multiround待执行 | L NOT_RUN |
| R06 | 同处/不同处/未知三分 | MediumUsesCurrentSources/same_place、different_place、empty | U PASS；L NOT_RUN |
| R07 | 隔空/假设表达不拦 | StyleDiagnosisConservative 负例 | U PASS；L NOT_RUN |
| R08 | 门口位置按陈述来源，不标已核验同处 | ProductionCommunicationShapeAndEffectiveBinding | U PASS |
| R09 | 温知意真实人物口吻 | live温知意9轮入口 | L/H NOT_RUN |
| R10 | 原文复杂性与hash保留 | PersonaAppearanceHistoryFinalSelection；PersonaViewKeepsArbitraryVoiceAndProfile；manifest | U/W PASS；存储hash P、生产人工 H NOT_RUN |
| R11 | 任意自由文本无清洗Agent | PersonaViewKeepsArbitraryVoiceAndProfile；正常调用一次 | U/W PASS |
| R12 | 无法分类原文回退；编译失败不换人格 | PersonaViewKeepsArbitraryVoiceAndProfile未分类原文；既有CompiledWorkingPersonaNativeDetailChain | U PASS；编译/存储恢复 P SKIP；缺失/过期portrait沿原明确失败 |
| R13 | 生效profile不串视图/查询 | DetailBridgeIsScopedAndRepairIsToolFree；PersonaDetailSectionsKeepProfileScopeAndFullFacts；原profile链路 | U PASS；真实切换/缓存 P SKIP |
| R14 | 多样人格不统一语气 | PersonaViewKeepsArbitraryVoiceAndProfile；3类合成live入口 | 输入U PASS；输出L/H NOT_RUN |
| R15 | 作品及继续创作豁免 | StyleDiagnosisConservative作品/继续/否定作品 | U PASS；L NOT_RUN |
| R16 | 姓名自称/引用不单词拦 | StyleDiagnosisConservative负例 | U PASS；L NOT_RUN |
| R17 | 真实活动可见，无额外背景事实 | PersonaAppearanceHistoryFinalSelection；C/D完整wire | W PASS输入；L NOT_RUN输出 |
| R18 | 闲聊少无关body，底层/media不损失 | PersonaAppearanceHistoryFinalSelection；StyleRepairPreservesNativeImageIntent | U/W PASS |
| R19 | 当前外貌按需读取及跟进 | AblationRequests/C原生fixture query；DetailBridgeIsScopedAndRepairIsToolFree | W PASS工具可用/结果；P授权/真实来源 SKIP；L跟进选择 NOT_RUN |
| R20 | 稀疏配饰不补衣着/反推裸体 | sparse reconstructed appearance + live入口 | 源数据U保留；L NOT_RUN |
| R21 | 同脏历史不改原文/顺序 | PersonaAppearanceHistoryFinalSelection；A/B/C/D hash/input diff | U/W PASS；L回到消息文体 NOT_RUN |
| R22 | 未加引号拒绝/邀约/承诺保留 | HistoryKeepsUnquotedPromisesAndRefusals | U/W PASS |
| R23 | 无可靠分段原文回退 | HistoryKeepsUnquotedPromisesAndRefusals；history projection标签 | U/W PASS |
| R24 | 连续assistant真实顺序 | HistoryKeepsUnquotedPromisesAndRefusals；FactOnlyConsecutiveRepliesKeepTheirTrigger | U/W PASS；原触发backfill P SKIP |
| R25 | 夜间正文标题不改当前clock | HistoryKeepsUnquotedPromisesAndRefusals；原morning view tests | U PASS；L NOT_RUN |
| R26 | 同Cydonia一次无工具修复 | StyleRepairUsesSameModelNoTools；DurableStyleQuotaAcrossResume | W PASS；P CAS SKIP |
| R27 | 主动邀约不被程序降级 | StyleRepairUsesSameModelNoTools/success：合成候选含周六邀约，修复输入原意保留 | W PASS边界；L/H语义保真 NOT_RUN |
| R28 | 旁白动作不能变一人称真实动作 | repair指令/权威unknown context完整wire | W PASS输入边界；L/H输出语义 NOT_RUN |
| R29 | 正常/单句文艺0repair | StyleDiagnosisConservative；StyleRepairUsesSameModelNoTools/normal | U/W PASS；L NOT_RUN |
| R30 | 超时/OOM/503/cancel/仍失败有限 | StyleRepairUsesSameModelNoTools全失败场景；原ReplyTransientRetryIsBoundedButOOMAndTruncationDoNotRetry | W PASS；lease竞争P SKIP |
| R31 | 详细画面intent合法不改 | StyleRepairPreservesNativeImageIntent；原ReplyWriterNativeImageRequestOwnsPrivateHandoff | W PASS |
| R32 | pending→同draft结果恢复→唯一提交 | 原ReplyMediaPendingIsAuthoritativeWaitingButNeverCommitted；ReplyMediaCatalogDenialCompletesSameDraft；publication/continuation | W pending PASS；持久闭环P SKIP |
| R33 | 视觉关不关闭生图/emoji | VisionDisabledDomainCallsNeverRequestAndDoNotDisableTextStages；原native image/placeholder/Unicode tests | U/W PASS；真实资产/视觉 P/L NOT_RUN |
| R34 | 发布后坏JSON只修结算 | PrivateChatCommittedStyleRepairAndMalformedSettlement；FactOnlyCommittedReplyMalformedFinalSettlement；原HTTP/SSE final-schema separation | W PASS隔离；P持久发表 SKIP |
| R35 | 目标仍可见/不回guided | PersonaAppearanceHistoryFinalSelection；原Goal scope/strategy tests | U/W PASS输入；L积极配合/自主推进 NOT_RUN |
| R36 | 热更/原binding/旧v4/额度/profile/回退 | RestoredBindingIgnoresHotUpdates；ExpressionSettingsAndFrozenContract；ProductionCommunicationShapeAndEffectiveBinding | U/W PASS；媒体重启/竞争/回退组合P SKIP；部署演练 NOT_RUN |

## 已执行与缺口

- 最终Go全scope race无失败；完整计数和服务用例跳过列表见verification/go-race.jsonl、test-summary.json。父/子测试事件不得当独立验收数。
- 24个A/B/C/D controlled样本，clean/dirty每组3次，完整HTTP输入与差异已保存；这些输出都是测试脚本，不是Cydonia合格率。
- 原温知意persona/history SHA256与来源分类在testdata/private_chat_expression/manifest.json；未修改原人格或已发送消息。
- style成功、仍旁白、OOM、503、quota DB失败、取消、正常、额度已用等全部受控输出在verification/style-*-controlled.json，不只保存最好结果。
- live ablation +温知意9轮/3类合成voice入口已经实现，当前缺显式live配置未执行；人格魅力、积极信号和事实语义判断未认证。
- GO_CORE_TEST_DATABASE_URL缺失，Docker daemon/本地PG工具不可用；并发CAS、真实profile切换、媒体持久恢复与实际提交必须在隔离PG补验。
- 原事故完整API wire/原采样/原3705 token输入未找到，本材料明确重建；运行环境DB mode/model/version和新镜像未核验。
