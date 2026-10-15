# 认知与 Fact-only 回复输入及结算优化

## 目标与价值

在 `codex/fact-only-reply-vision` 上局部优化两条已接入的路径：Qwen 获得可用于认知、真实查询/执行及完整结算的清晰输入；Cydonia 获得主体、时间和来源明确，保留人格与目标且不带旧社交策略的回复输入。沿既有 Eino 原生工具循环完成同一轮，模型错误和业务数据冲突如实暴露。

## 来源、授权与基线

- 用户请求：继续在指定分支调整；2026-10-14 已明确批准创建任务与规划，并在最新方案后回复“开始实施”。原四笔工作与F14容量修复ec1ed82已提交；用户追加F15频率/F16程序兜底已由用户提交b24fdd1；当前追加F17/F18继续实施。
- 附件原文为 `source-requirements.md`，来源 `/Users/vinson/Downloads/混合语料方案优化.md`。T0–T8、D1–D7、R01–R26 作为范围/验收参考；文内操作要求不单独授权真实模型、发布、部署或提交。原文校验摘要保存在 `source-integrity.json`。
- 源码基线 `d696c44`；创建任务前工作区干净。
- 沿用既有本地测试/用户真实环境验收分工，依据归档任务 `10-13-fact-only-config-context/design.md`。代理执行本地受控测试；真实模型和真实环境提供入口并单列状态。本任务不自动 push、合并或部署。

## 已确认背景

1. `reply_context.go:70` 从通用 compact 构造回复输入；`:83` 覆盖 Goals，绕过通用 profile 筛选。`:33` 过滤 last_evaluation，未过滤 execution.next_step/current_stage.strategy。
2. `reply_context.go:109` 历史仅 role/content，缺 turn_id 时 `:123` 按单条分组。认知侧 `provider_context.go:365` 生成 sender/time 前缀；未证实正文夜间 Markdown 标题的生成源。
3. `fact_only_reply_capability.go:19` 接受 purpose/topic_key，`:116` 传给 publication，未发现直接传给 Writer。主动去重 `tool_publication.go:155` 实际依赖两者，媒体 `reply_media.go:145` 保存其恢复元数据。
4. `memory_provenance.go:39` 明确来源解析不证明派生文字的语义支持。verified/partial 是来源存活分类，epistemic_kind 主要是创建路径；关系 mutually_confirmed 也不能证明任意标签被原话蕴含。
5. `eino_model_runtime.go:965` 已有一次无工具最终合同修复；`eino_model_request_policy.go:68` 有意让工具请求不发送 response_format。现有测试未组合覆盖 Fact-only 持久化发布和语法损坏 JSON。
6. Writer 与 Moments 共享装配。Goal active_count 为实例范围，可见集合另受 profile/recipient 过滤。
7. 仓库未找到原事故三次完整物理请求/响应或采样快照；SDK 消息表示不等于原 HTTP bytes。代码出处与证据缺口见 `research/call-chains.md`，详细领域研究见其余 research 文件。

## 要求与验收

| ID | 要求 | 来源 / 验收 |
| --- | --- | --- |
| F01 | 固定真实调用链、证据及缺失项，建立无真实发布且来源可查的 fixture，不补造事故原文 | T0；D1–D7 |
| F02 | 自身 Actor、有效 profile、recipient 独立；无称呼不生成姓名，不把 actor_user 当昵称 | T1；R04、R14、R25 |
| F03 | 当前时间采用请求/快照，历史采用持久化元数据；上午状态不因正文夜间标题失效；过期按字段规则处理，保留 unknown/cleared/false | T1；R01–R03、R24 |
| F04 | 按真实交互片段取历史，保留连续 assistant 及关联用户；当前输入一次；Runtime/Validation/Tool Result 不进入公开历史 | T1；R05–R06、R22 |
| F05 | 区分来源存活、认知类型、置信度与事件成立；严格去重保留全部来源，冲突只呈现不改记录 | T2；R07–R10 |
| F06 | 保留有效可见目标本意、动机、原完成条件、状态、主体/对象及真实期限；旧策略不进 Writer，真实权限/未完成依赖保留 | T3；R11–R15、R23 |
| F07 | 普通 Qwen 输入不常驻无关全套 SOP，但工具仍可发现、授权执行并消费结果；现有完整认知 schema 保留 | T4；R13、R16–R17 |
| F08 | Cydonia 保留完整 Working Persona、关系/目标/语境，自主决定表达；不接收 Qwen 正文/方向、旧策略或内部证据链，场景按 Actor 区分 | T5；R09、R11–R12、R23–R24 |
| F09 | Fact-only 参数只承担真实路由、draft 恢复和本轮授权引用；正文/语气/嵌套或改名策略不能进入 Writer，legacy/去重/旧 draft 兼容明确 | T6；R19、R26 |
| F10 | 发布与结算分开记录；非法 JSON 仅有限无工具修复，不重发、不造合同、不改已发正文、不写 Validation 历史；重试不重开已发布轮次 | T7；R20 |
| F11 | accepted/queued 不算生成/送达，沿同 draft 恢复；视觉关闭仍可生图，不声称看图；工具后读已生效新状态且 lineage 一致 | T5–T7；R17–R18、R21、R25 |
| F12 | 逐次认知/回复/结算/修复有模块估算、实际 usage/延迟/错误和删减原因；超预算不截断受保护内容、工具配对或 schema | T8；R22 |
| F13 | legacy、Moments、其他 Agent、既有视觉/媒体链兼容；不以短句模板、删除 Goal 或改写旧数据掩盖语义失败 | T3–T8；R18、R23、R26 |

R01–R26 的证据与执行层次在 `acceptance-matrix.md`，不能以规划或报告生成替代执行。

## 范围

仅两处 Composer/Slot、只读投影、历史选择/序列化、Fact-only reply 参数隔离、阶段协议/有限修复及必要诊断/测试。可补齐已读来源/有效期字段，不新增领域状态权威。

不重构 Goal/Memory/Actor/人格/日程/物品/完整编排；不修改权限或完成规则；不修写存量关系、承诺、摘要，不删历史、不迁移表；不新增每轮清洗 Agent；不更换模型/采样/依赖；不重做视觉、媒体、Actions；不新增 direct 对照或生产轮数期限；不自动提交、推送、合并、更新稳定渠道或部署。

## 交付与验证边界

交付调用图/来源表、两处旧新脱敏输入及删减理由、两份最终协议和实际 Tool/schema 绑定说明、R01–R26 报告、逐请求 token/调用数/延迟/错误统计、原配置回退说明及无真实发布回放入口。缺失原始输入标 unavailable；缺失实际 usage 为空并说明，不以估算冒充。

区分已执行本地 U/W、已执行隔离 P、SKIP、真实模型 NOT_RUN。报告原数据问题和模型真实失败，不承诺装配可完全恢复人格或消除幻觉。

## 规划收敛

一个复杂任务、四个顺序切片，共同验收一轮链路；共享 fixture/引用/发布边界，不拆成独立领域改造。无阻塞的产品或范围问题；已获实施批准。原事故完整输入缺失、自由文本语义不可确定、数据库/真实效果未执行均作为明确限制，交付停止在待人工验证及提交审阅状态。

## 2026-10-14 用户追加：可用性优先

F14：内部引用索引与冻结上下文的优化容量阈值不得一刀切阻断合法运行。超限完整保留并告警，具备模型前可排查诊断；身份/权限/作用域/引用真实性与版本检查保留。已由用户明确授权继续实施，详见 availability-followup.md。旧F12中的硬预算失败规则不再适用于本轮内部引用/上下文的经验容量阈值。

## 2026-10-14 用户追加：目标Agent频率

F15：Goal Evaluation以实例为粒度30分钟一轮，事件/重试不可绕过；Goal Planner模型调用必须有真实目标空位，自动调用限正式完成补位或本地日一次且有空位。普通认知/WakeUp的目标状态Tool不改。最新指令覆盖旧Actor/profile独立规划、满额Owner/review绕过等自动/容量准入行为，详见goal-cadence-followup.md。用户已明确要求实施，无新增规划批准步骤。

F16：用户追加Goal Evaluation submit反复冲突/unknown/ref错误并卡死，要求程序能兜底的正常兜底。重复accepted target复用原回执、all-unknown/no_change确定性归一needs_evidence、引用只作scope内确定性处理，不能修复则保留待证据而非循环报错；有限无进展时正常收束并明确未完成/未accepted，保留真实成功与pending source。详情见goal-submission-recovery-followup.md；权限/真实完成权威不改。

F17：用户报告conversation.reply的reply_evidence_not_authorized。可选evidence_refs无法对账不得拦住当前授权回复；无法确认项排除、不作为事实，有效项保留并继续独立Writer，具备不含raw refs/正文的安全对账诊断。原run/source/draft/media/publication权限和真实Provider失败不变。F15/F16已由用户提交b24fdd1，本追加继续实施，详见reply-evidence-followup.md。

F18：用户要求38个Conversation Tool及实际追加私有Tool全部过一遍、必须能调用；默认告警+程序兜底，确需保留的拦截逐条说明工具/条件/具体理由。共48注册+1 Writer动态名称，逐项覆盖含nullable/早期schema/Kev推荐误拦、Goal缺review回滚、各域重复/无结果/缺技术字段。用真实handled/no_effect结果避免把未执行伪装成功或挂Pending；真实效果前提与已确认频率/容量/私有分工保持且逐项解释。F17并入本轮，详见tool-availability-followup.md。

F19：F17/F18与独立私聊人格任务已由用户提交42a2d7c。继续修复用户新报告的Planner commit：context_review_required误作依赖不得拦住可执行的有效命令；缺省技术字段从已提供/消费的真实快照补齐；needs_input必须可定位并正确分类。真实复核用reviewed_goal_ids，保留初始空criteria、真实依赖、原子性和所有权；反馈中的暗梗/照片/关系阈值是Agent建议，不是用户标准。详见goal-planner-commit-followup.md。用户继续要求实施，无需重开批准。

F20：用户追加conversation.reply的reply_draft_scope_invalid。错误的可选draft_ref不应拦住当前已授权回复；忽略该提示并绑定当前轮canonical草稿，保留safe告警、cached/published单次效果、媒体续接和真实source/主体/权限。不能读取或借用foreign/旧轮草稿。详见reply-draft-recovery-followup.md，继续实施。
