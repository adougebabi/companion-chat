# T0—T9 实施交付：私聊表达媒介与人格保真

代码已落地并通过本地自动化检查，继续停留在 `codex/fact-only-reply-vision`。实现基线为 `b24fdd13d1ee1376158fd7cdd259fc6499531aad`。用户授权将本任务与同工作区的工具恢复实现一起提交；没有合并、推送、部署、真实用户发送或原人格/已发送历史改写。

提交保留实现、回归所需fixture、规范和交付记录。`verification/` 的原始JSON/日志/请求快照、源码备份与临时审阅补丁只保存在本地，不纳入Git。下文引用这些本地证据；新的受控回放可按验证文档导出。

**真实Cydonia语义、持久化PostgreSQL闭环和运行环境生效版本尚未验收。** 下文PASS只表示明确的结构/受控层次；不宣称模型已稳定消除小说化或保住人物魅力。

## T0—T9 修改落点与完成边界

| 项 | 实际落点 / 复用 | 本轮交付与限制 |
| --- | --- | --- |
| T0 | reply_expression_replay_test.go、testdata/private_chat_expression、verification/source-before；原wireCaptureTransport | 修改前A完整实际SDK请求已冻结，当前A逐字段相同；原始Persona/小说历史SHA和材料边界保存。温知意是重建fixture，原完整wire/采样/3705 tokens未找到；运行DB配置未读取 |
| T1 | reply_projection.go、fact_only_context_views.go、cognition_agent.go、provider_context.go、intelligence.go | 两处同源text_chat；co unknown/same/different及双方地点分开；current co事实与来源/有效期局部补齐，生活圈/online/门口位置不变成同处；结构通过，真实输出未运行 |
| T2 | reply_expression.go、reply_context.go；原operation slot | v5人物亲自发送消息协议替换，v4原文只用于回退/旧草稿。最终HTTP只一份稳定私聊协议，无旧神态规则追加；B仅协议差异可独立查看 |
| T3 | reply_projection.privateReplyPersonaView、writerTaskInput | 当前完整已存portrait或结构化Working Persona保留，增加解释边界；canonical Persona/compiler/raw-view/reply-view hash与profile/source版本诊断。不能把compiled portrait冒称用户最初导入全文；人格存储语义和编译器未改 |
| T4 | applyPrivateReplyView、reply_detail_query.go；复用persona.detail | 普通回复少无关body；实际安装当前profile纯查询时可按需读外貌。工具关闭/不可用时保留authority数据，媒体全量状态未损失。活动/时间/原Goal及空criteria保留，无关键词切割或新deadline |
| T5 | labelPrivateReplyHistory；复用factOnlyHistory/最终history_metadata | 原角色/正文/顺序/hash不改；只标原消息/过去文体非当前规范与非事件证明。混合文字不抽引号。未加引号承诺/拒绝/连续assistant/正文夜标题回归通过 |
| T6 | reply_style.go、reply_provider.go、reply_failure_policy.go、claimReplyStyleRepair | 同Cydonia发送前最多一次0tool修复，CAS额度跨restart/media；保存首稿、诊断、repair raw/finish/cause、终稿。普通/存疑0额外style调用，作品/自称/引用豁免；503/OOM/claim DB失败不重跑整轮、不转Qwen修复。PG原子竞争未执行 |
| T7 | reply_media.go、reply_continuation.go、原publisher/最终JSON repair | 原生image意图不经文体过滤；media request/附件冻结；原prepared/published重放/唯一提交保留。draft单独存真正committed_text/metadata，重复抑制也回读实际旧message；Qwen读取实际正文。持久媒体/提交组合PG用例SKIP |
| T8 | reply_vision_settings.go、private_reply_run.go、reply_expression.go、现有ReplyVisionSettings.vue | expression_version新运行缺省v5，旧draft缺字段v4；binding保存protocol/hash/view/medium，失败重领/media恢复不覆盖原模型/配置。effective fallback ready合并真实无secret binding，不重置额度。actor/profile缓存围栏保留，回退仍fact_only |
| T9 | reply_expression*_test.go、acceptance-matrix、verification、docs/private-chat-expression-verification.md | R01—R36逐项结果；24个受控A/B/C/D样本、全部请求、失败样本、成本与原/新input diff。live单轮及温知意9轮/3类合成voice入口已实现但未执行。没有新测试镜像 |

完整文件列表、当时源hash、分支和基线见本地 `verification/implementation-files.json`；实际提交文件以 `git show --stat` 为准。新增文件均是现有模块的局部契约/投影/验证/测试帮助代码，没有第二套回复、发布、媒体或人格组件。

## 生效来源、媒介与人格保真

当前代码唯一私聊协议源是reply_expression.go常量，选择入口是既有conversation.reply设置；数据库没有另一段Prompt文本覆盖。v4“自然人物正文/轻微神态”原文保存为历史合同，普通v5不会与之叠加。Moment保留独立协议，Qwen/媒体/视觉输出不经过私聊文体检查。

| 字段 | 来源与含义 |
| --- | --- |
| channel=text_chat | 经过direct private conversation验证的真实入口/scoped run，不由Qwen从“找你”推断 |
| co_presence | 仅现有当前有效、非inference的明确co_located记录；保留epistemic/source/effective_at/valid_until。不把用户陈述标成现场核验，不从关系摘要或同城推出 |
| self_location | 有效Life context，过期后unknown |
| recipient_location | 当前Actor location陈述及来源/时间，缺失unknown；“门口”不自动变成同处 |
| 原Persona / working text | canonical_persona_hash、compiler source_hash/版本、完整portrait或原结构字段；自由文本无清洗/概括 |
| Reply view | 原文+interpretation_boundary及view hash，习惯条件性、偏好非现在事实、原视觉设定非当前外貌 |
| 历史 | 原role/content和最终选择索引；过去发过这些字不证明场景、接触或媒体发生 |

已编译portrait缺失/版本过期仍遵守原明确错误，不以另一人格或全source dump绕过授权；视图无可靠分类时原字段保守保留。没有在聊天中新增persona编译、清洗或历史改写模型。

## 实际输入对照与成本

每组clean/dirty各3次，共24个受控样本；模型controlled-cydonia、temperature=0.8、top_p=0.9、output cap=1024固定。参数均为明确重建参数，不是原事故采样。A与修改前捕获完全相同；A→B除system协议外，其他messages/tools/model/sampling逐字段相同。

| 组 | clean首请求估算tokens | dirty首请求估算tokens | 每样本物理调用 | style尝试 |
| --- | ---: | ---: | --- | --- |
| A 原装配 | 4388 | 5608 | 1 | 0 |
| B 仅新协议 | 4894 | 6114 | 1 | 0 |
| C 加媒介/人格/状态 | 6170 | 7390 | 2：测试脚本明确请求一次当前外貌查询 | 0 |
| D 加history来源与repair | 6170 | 7852 | clean 1；dirty 2 | dirty各1，脚本故意注入已知旁白首稿 |

这些是保守估算与受控调用统计，**不是实际tokenizer计费、Cydonia首稿成功率或推理耗时**。新协议、人格边界和query schema增加了输入成本；不以“更短”验收。实际生产正常无query/无repair时仍一个Writer逻辑调用；native query、media和repair的额外物理请求分别计数，不能固定称“两次HTTP”。

具体证据：

- [A→B dirty完整输入差异](verification/A-B-dirty.diff)、[B→C](verification/B-C-dirty.diff)、[C→D](verification/C-D-dirty.diff)，clean同名文件也存在。
- [24个样本与完整实际请求](verification/ablation-controlled-summary.json)、[成本/配置一致性](verification/input-comparison-metrics.json)。
- `style-success/still_narration/oom/repair_503/oom_503/claim_database_error/cancelled/normal/quota_spent-controlled.json` 保存全部受控结果。失败HTTP没有可用正文时原样标无输出，保留错误类别；仍旁白样本保存失败repair文本，没有只挑最好结果。

首稿、repair和最终draft使用同一draft输入；媒体恢复trace累加并共享一次额度，真正commit另存committed_text/metadata，不能用prepared text冒充实际发布。真实一条完整durable lineage仍需PG验证。

## 测试结果与未执行项目

最终 `go test -race ./...` 全工作区通过，17个有测试结果的包PASS；1870个测试/子测试PASS事件、555个SKIP事件，无失败。计数包含父/子事件，一些PASS父测试包含服务用例SKIP，不能当作1869个独立验收或555个缺陷。

`go vet ./...`、`go build ./...`通过。`pnpm typecheck`、`pnpm test`、`pnpm build`通过：browser-client 21、core-client 2、Web 105，共128个前端/客户端测试，无失败或跳过。初次本地监听/tsx IPC受沙箱限制，同一受控检查已在沙箱外通过。

新增针对性用例、原Persona/历史保护回归均通过。具体每个R编号的层次、符号与限制见 [R01—R36表](acceptance-matrix.md)，总结果见 [test-summary.json](verification/test-summary.json)。只读独立核验及修复记录保存在本地 `research/review-resolutions.md`。

未执行：

- 真实Cydonia输出、人物魅力/主动性/邀约或拒绝保真、场景幻觉率及实际token/模型延迟；live配置未提供。
- PostgreSQL并发CAS、重启/media/profile切换/取消竞争及真实一次提交与JSON结算；缺隔离DB，Docker/local PG不可用。新增P用例已编译但SKIP。
- 运行环境数据库模式、实际绑定、在途draft和已部署版本核验；不能从源码v5推出运行中已经生效。
- 生产构建浏览器交互/窄屏手动检查、真实媒体生成、真实发送、回滚演练。
- 远端CI新构建、镜像晋升、合并和部署。

真实回放/隔离PG的可运行命令在 [验证与回退文档](../../../docs/private-chat-expression-verification.md)，不需要新生产实验服务。当前live两入口均明确SKIP，没有静默当通过。

## 分支、镜像与回退

保持功能分支，基线 `b24fdd1` 不包含本次实现；当前实现提交SHA应读取 `git log -1`，源hash证据按生成时刻独立保存。CI exact branch过滤涵盖当前分支，没有paths排除；没有改GitHub Actions或稳定标签。

**本次v5镜像未构建，无法交付本次新镜像引用。** 历史已发布清单只可作旧渠道证据：source b024f35c7d0d585f748817bd0297366c14a6d194、run38423006085。

- 旧Core：`adoumibaba/fluctlight-core-go@sha256:d39f7c59cc73f03b88d70aefa802f4690a994c2a8670730bee451e607055a636`
- 旧Web：`adoumibaba/fluctlight-web@sha256:953c370c321cd8e8f5a9f4817e7bfe4c8bb96eb431663a523415824edcfe6697`

上述引用来自归档已验证清单，本轮未再探测注册表，也不是本次v5产物。表达回退选择v4并保留其他参数；旧二进制回退继续采用原drain/cancel/兼容settings/稳定digest步骤，由用户执行。

## 剩余根因假设与验证边界

已确认代码问题是媒介投影来源错位、无关body常驻、表达协议含糊、在途协议未冻结及缺一次文体修复；新代码和输入证据覆盖了这些落点。文学化人格/脏历史是否仍诱导Cydonia叙述是待真实回放的假设，不能由mock宣布消除。

文体诊断仅处理姓名锚定、连续第三人称、旁白交织台词等高置信形状，其他存疑观察；不判断所有人格、事实或同处语义。模型修复是否自然、是否保持原意，仍需保留真实首稿/失败样本并人工判断。已发送历史和人格没有被用作“清理后假通过”。
