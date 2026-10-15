# R01–R26 验收映射

实施后的状态如下；PASS 仅指对应本地层次，不是完整真实环境验收。U=纯装配/投影，W=真实 SDK/Eino 到本地 HTTP/SSE 脚本 Provider，P=隔离真实 PostgreSQL，L=用户真实模型/环境。U/W 不代表 L，P 缺配置必须 SKIP。

| 编号 | 验收断言 | 证据 / 承接点 | 需求 | 当前状态 |
| --- | --- | --- | --- | --- |
| R01 | 上午 as_of、23:58/23:59 正文仍用上午时钟并保留有效活动 | U：时钟/消息元数据/原始标题 fixture，旧新快照 | F03 | U PASS；L NOT_RUN |
| R02 | 真超期只过滤该活动，不废弃整个人物状态 | U/P：Life 有效期、body/presence 分别检查 | F03 | U PASS；P SKIP |
| R03 | 用户日期/Markdown/代码不被清除 | U：原文保留，证明 renderer decoration 单列 | F03 | U PASS |
| R04 | 林岚 Actor/晴岚 profile/无称呼对方独立 | U：姓名缺失/同名/异名，actor_user 不当昵称 | F02 | U PASS |
| R05 | 连续 assistant 和真实关联用户保留 | U：含/缺 turn_id/source、unanchored、whole group 预算 | F04 | U PASS；P SKIP |
| R06 | Runtime/Validation/Tool 不进公开历史 | U/W/P：slot 分类与 conversation_messages | F04 | U/W PASS；P SKIP |
| R07 | 同源重复只一组，来源保留且不同日/Actor 不误合并 | U：重复转义、同文异事件、相反内容负例 | F05 | U PASS |
| R08 | inference + verified 仍非已发生事实 | U：epistemic/provenance，原 provenance tests | F05 | U PASS |
| R09 | 关系/其他来源差异呈现不裁定，无数据改写/强制重确认 | U/P：结构比较/文本摘录，源对象和业务表不变 | F05、F08 | U 部分：结构差异/不改源；语义 L NOT_RUN；P SKIP |
| R10 | 旧评估号不作当前 ref，不传内部链 | U/W：codec/foreign/stale，合法 e6 型号负例 | F05 | U/W（既有 ref 校验）PASS |
| R11 | next_step/strategy/wait prose 消失，原目标条件保留 | U/W：ReplyContextUsesAuthority 扩展和嵌套计划负例 | F06、F08 | U/W PASS |
| R12 | 未提目标/简短回复不隐藏有效目标 | U：不同输入同 visible-goals；L 表现单列 | F06 | U PASS；人格互动 L NOT_RUN |
| R13 | 权限/真实依赖/失败保留，去策略不放开执行 | U/W/P：typed conditions 与实际 Tool admission | F06、F07 | U/W PASS；P SKIP |
| R14 | shared/current/other profile/recipient 可见性正确 | U/P：权限矩阵、global/visible counts | F02、F06 | U PASS；P SKIP |
| R15 | completed 只必要事实，不带全执行对象或重激活 | U/P：混合 lifecycle，source/DB 不变 | F06 | U PASS；P SKIP |
| R16 | 情绪聊天无全套衣物 SOP，工具仍可发现 | U/W：cognition 快照、原 discover/load 回归 | F07 | U/W PASS |
| R17 | 真实衣物/图像准入、结果消费、同 draft 恢复 | W/P：ReplyMediaPublication/ReplyEvidenceRecovery 扩展 | F07、F11 | W（手交/失败）PASS；P SKIP；外部环境 NOT_RUN |
| R18 | 视觉关闭不理解图片但仍生图/保存 | W/P：VisionOptionalReview/MediaQualityFinalProviderRequest | F11、F13 | W（既有视觉/媒体边界）PASS；P SKIP |
| R19 | purpose/reply_intent/正文/语气/嵌套改名策略拒绝，refs/路由有效 | U/W/P：closed schema/typed Writer、legacy/去重/旧 draft | F09 | U/W PASS；跨版本 P SKIP |
| R20 | Fact-only 真提交后语法坏 JSON，一次无工具修复，消息数始终 1 | W：Generate/Stream/成功失败取消；P：draft/receipt/inbox/replay | F10 | W 20变体 PASS；真实事务组合 P SKIP |
| R21 | queued/accepted 不算生成送达，同 source/draft 终态续接 | W/P：waiting/cancel/replay 与真实状态 mock | F11 | W PASS；持久化/重启 P SKIP |
| R22 | 超预算不截当前输入/原条件/配对/schema，有诊断 | U/W：RequiredGoalOverflow/WorkingMemory/wire cap 负例 | F04、F12 | U/W PASS |
| R23 | 长分析/寡言/强硬人格保留，无统一模板 | U：完整 persona/预算；L 体验单列 | F06、F08、F13 | U PASS；自然表现 L NOT_RUN |
| R24 | 标题/会议/同室幻觉记真实失败，不伪造通过或改历史 | W/P：原校验、原始/已发文本不改；L 效果单列 | F03、F08 | U/W 保留原文/原校验；模型幻觉效果 L NOT_RUN |
| R25 | 工具后实际状态/profile 更新，Writer 新版本同 lineage | W/P：refresh/persona/schedule 结果、draft source/turn | F02、F11 | 既有 W 刷新回归 PASS；P SKIP |
| R26 | legacy/Moments/其他 Agent 无意外改变 | U/W/P：旧 schema/FactOnlyMoment/Goal/VisualIdentity/格式回归 | F09、F13 | U/W PASS；P SKIP |

## 效果回放限制

先固定已知摘录与 missing 清单，synthetic 单独标识，没有原件不称事故完整复现。U/W/P 默认无真实用户发布。用户真实模型回放复用现有 fact-only/live 绑定；每组模型/采样/工具契约固定，合同变化另组。

检查时间/主体/目标/事实、旧策略/内部 ref、人格连续性及唯一发布/结算，不把每轮推进、短句或讨好当标准。逐请求报 token/调用/延迟/错误；附件 token 是原报告值，不是本次测得值。真实模型本轮 NOT_RUN。
