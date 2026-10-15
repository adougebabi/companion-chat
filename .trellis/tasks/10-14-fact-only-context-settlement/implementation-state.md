# 当前实施状态

分支 codex/fact-only-reply-vision；任务仍 in_progress。原四笔工作 1876275、01e8243、4a389d6、26bfc58，F14 ec1ed82，F15/F16 b24fdd1，F17/F18与private-chat实现42a2d7c已提交。本轮F19 Planner commit和F20 reply draft恢复已实施，均未提交；不 push、部署、归档或操作真实工作流。保留以前生成报告/JSON/日志/源码备份及其他任务未跟踪文件。

## 当前行为

- 固定48注册+1 Writer私有名称，Conversation38；49行审计和每项保留的真实效果前提见 tool-availability-audit.md。
- nullable/union与Schema前表示修复；raw参数及ActionOutcome经验容量改告警；Kev No/config/window仅建议，安装/真实资源授权独立。
- handled端到端表示无请求效果；业务保存点回滚后只写技术ledger；原先durable领取的due Attempt走suppressed结清，无失败计数/Goal进度。两套Runtime都处理fresh Prepare marker与取消。
- F17可选hint对账排除后继续独立Writer，safe worker诊断；真实draft/action/source/Provider边界和发布重放保持。媒体handled标unavailable，继续同草稿，不制造已生成图片。
- Goal缺Review不回滚有效根；completed根superseded待Review，未完成technical wait；薄字段/skip/abandon Schema；Planner查询次数只建议，缺技术版本仅绑定此前消费快照。30min/空位/合法episode不旁路。
- Memory/life/schedule/affect等闭集普通语义错误无效果兜底；Actor SQL错误分类；Persona/Habit/物品/穿着重复状态不假成功/多写，真实来源/对象/版本不猜。

## 验证与待验收

详见 verification/tool-availability-results.json。最终可提交快照Go全量1829 PASS事件、564 SKIP、0 FAIL；Core/Workflow race 1489 PASS、499 SKIP、0 FAIL；实际PG/race重点92 PASS、无SKIP/FAIL；最终真实故障/锚点完整性回归另10 PASS。pnpm typecheck、128测试、build通过，最终快照Go vet/build通过。数字包含父/子测试事件，不能当49个真实外部业务均已执行。

完整PG对照：b24fdd1共275 FAIL事件（251个叶失败），中间当前快照160 FAIL事件；并非最终全绿。观察到的24个current-only叶失败都已在最后92项重点回归实际重跑并PASS；其他完整PG失败保留，不一概推断根因。真实Provider、ComfyUI/对象存储、Redis、Temporal与线上恢复NOT_RUN。独占PG已确认停止并删除整个临时安装/cluster目录。

同一工作区另一个private-chat任务并行编辑源码，包含与本轮共享的fact_only_reply_capability.go、reply_image_request.go、reply_media.go。主线程没有覆盖其内容。最终验证使用/tmp/codex-tool-final-snapshot，只含本轮hunk；源码SHA/排除范围见verification/tool-snapshot-manifest.json。上述验证针对当时的工具实现独立快照。最终提交按用户授权同时包含private-chat实现，完整提交快照另外复验；不包含测试生成的turn_path_cost_report.json或临时verification产物。

已完成两项只读独立核验并由主线程修复其发现；本轮使用用户指定的探索型子代理分工，领域实现和最终验证主线程负责。可执行spec已新增，当前提交已获用户明确授权，真实环境验收后才考虑归档。旧F15/F16提交计划仅作历史记录。

## F19 当前追加状态

前文F17/F18验证是历史快照，本轮实际源码基线42a2d7c。

- context_review_required伪dependency只作告警，flag-only不替换真实依赖、不自动复核。reviewed_goal_ids依旧为显式、真实复核提交入口，空初始criteria不改。
- 缺省技术fence优先绑定早一物理请求实际snapshot结果，否则绑定本run初始prompt已给的不可变snapshot；显式stale值和真实claim/权限保持。
- 参数错误有具体代码/固定指导，ADK和Planner final保留warning/disposition。最后handled提交不能冒充应用/政策拒绝；任何真实Tool失败会保留原原因。
- 两个诊断探子及一个独立只读审阅完成；主线程修复审阅P2并补前/后query失败回归。
- 实际临时PG/race定向39 PASS事件、0 SKIP/FAIL；Go全量1898 PASS/574 SKIP/0 FAIL；Core/Workflow race1558 PASS/509 SKIP/0 FAIL；vet/build通过；工作区typecheck、128tests、build通过。计数包括parent/subtest/package事件。原42a2d7c一条旧测试断言也失败，已仅校正后commit no-effect断言并检查保护Goal原状态。
- 临时PostgreSQL17.6+pgvector0.8.1+pgcrypto绑定127.0.0.1:57484；已确认停止并删除本轮/tmp安装/data。完整PG、真实Provider/ComfyUI/S3/Redis/Temporal/生产工作流NOT_RUN。
- 报告只有摘要，第二次实际参数及warning_code未获得；不得声称原needs_input唯一根因已确诊。本轮确证/修复可重现的拒绝和排查缺口。详情见research/goal-planner-commit-diagnosis.md和verification/goal-planner-commit-results.json。
- 本轮工作未提交，拟一笔fix(core): recover goal planner review commits；17项源码/spec/task文档及排除项见goal-planner-commit-plan.md。任务仍in_progress，用户真实回放后再验收归档。

## F20 当前追加状态

- reply_draft_scope_invalid直接比较已移除：模型可选draft_ref不一致只发reply_draft_hint_ignored告警，canonical主体/会话/source/run身份和查询不改；不读foreign草稿。
- 原生private reply单独decodePrivateReplyArguments，使未知可选draft alias到恢复层；Moment/legacy/action目标仍严格。告警prepared→实际发布/重放/accepted媒体，OutputSchema同步。
- 当前cache/published无额外Writer/消息；canonical媒体needs_action/waiting/ready按真实结果恢复。source关闭/取消/真实Provider/DB错误仍失败。
- 按当前auto模式由trellis-implement实施、trellis-check复核/修测试；主线程负责边界/spec与最终检查。六项F20 PG全部真实执行；check的scoped PG/race23顶层PASS无SKIP/FAIL；主线程F19/F20联合PG/race40 PASS事件无SKIP/FAIL。
- 最终Go全量1907 PASS事件/580 SKIP/0 FAIL；Core/Workflow race1567 PASS/515 SKIP/0 FAIL；vet/build、工作区typecheck/128tests/build、gofmt/diff-check通过。事件数字包括父子/包，非独立真实业务数。
- 扩大PG检查发现TestPrivateChatCommittedStyleRepairAndMalformedSettlement失败。主线程在42a2d7c独立源码快照+同临时PG复现完全相同行号/文本失败，证明基线已存在；本轮不改v5 style算法，不把宽检查称为通过。证据保存在verification/reply-draft-recovery-results.json。
- 真实事故raw draft_ref缺失，仅依据唯一代码触发点及受控复现修复，不声称生产回放/部署已成功。真实Provider/ComfyUI/Redis/Temporal/完整PG未执行。
- F20独占PG17.6/pgvector0.8.1/pgcrypto（127.0.0.1:57485）已确认停止，整个/tmp/codex-reply-draft-pg-xJ0xuB53安装/data及bootstrap脚本/marker已清理；测试日志/报告保留。
- 本轮与F19未提交变更合并更新一笔恢复修复清单，见tool-recovery-commit-plan.md；保留旧清单为历史。真实用户回放前不归档。
