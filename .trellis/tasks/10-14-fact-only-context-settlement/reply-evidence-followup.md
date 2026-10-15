# F17：可选回复引用对账与可用性

## 用户请求与基线

用户再次报告conversation.reply失败reply_evidence_not_authorized，并追问为何称“引用越权”。沿前述“程序能兜底的兜底，不应硬拦截合法使用”的明确指令继续处理。F15/F16已由用户提交b24fdd1，当前工作区干净；当前任务仍in_progress，不新建规划或更改分支。

该错误不能单独证明真实权限异常：当前replyEvidenceEntry将live缺失、未offered、snapshot缺失、身份/条目不一致统一返回false；resolveReplyEvidence因此拒绝整个Writer。合法旧ref可因更新/检索条件而消失，且wake-up与Writer检索operation不同。这是引用对账失败被上升成回复准入失败。

## 最小行为差距与决定

evidence_refs是可选补充信息，Writer无refs也会自己读取当前授权persona/Goals/state/history。无法确认一条可选hint不应阻断整次生成。程序排除无法确认/重复的可选ref，保留有效ref，继续原独立Writer；不猜外国entity、不使用旧快照绕过live，不把不可信来源升为事实。

## 必须保持的边界

1. 不放宽replyEvidenceEntry的offered/live精确匹配与scope/entity/revision/snapshot条件；可抽出同义的分类helper，但不能将“忽略不可确认hint”变为“授权该hint”。
2. Live projection的读取与调用owner/instance/conversation/profile、私有run、source/lease/取消、draft_ref/action_result_refs与publication权限仍是独立硬边界，真正不一致继续原拒绝。原snapshot内容不可修改。
3. 无效/过期/历史message/source_refs、缺offered snapshot和外部范围引用，都不作为Writer附加事实；原RefIndex、args、source data不改。有效引用的private dedicated view、策略剔除与不确定性保留不变。
4. shared moment.publish复用同一selection，可按同样可选hint政策继续当前授权feed Writer；保留其inference/memory/intention/developing-self语义过滤与scene/history隔离，不泄漏私聊。
5. prepared/published replay仍先于验证/Writer，保证一次正文和一次发布；可选hint变化不重开同logical draft。真正Provider failure仍原有同run terminal，不自动重调模型或伪造回复。

## 诊断契约

出现无法确认hint时，在Writer/lease前记录安全对账元数据：event_type=reply.evidence.reconciled、decision=continue_authorized_context、surface/correlation/instance/source identities、requested/matched/selected_fact/omitted/duplicate counts及reasons。class至少区分当前集合缺失、未offered/缺snapshot、offered scope/entry mismatch；即使DB不可用worker仍可见。名称不把普通对账失败称为“越权”。

不打印raw模型ref、输入文字、snapshot正文或凭据，不将诊断喂给Writer作为事实；无法安全分类的只计数。复用原best-effort sink，诊断错误不能替代业务结果。

## 预期文件与实施路径

- fact_only_reply_capability.go：抽出等价旧pure selection seam，随后改为完整确认有效项+排除不可确认hint，service继续使用当前授权projection；必要时独立reply_evidence_reconciliation.go承载小helper/安全report。
- 纯selection/授权负例、最终Writer context或真实HTTP/native Writer行为测试。
- reply_evidence_recovery_test.go旧PG“坏optional refs整个阻断”断言按新合同调整，保留Provider terminal/replay/draft/source/权限场景；共享Moment可选引用对照。
- 主代理负责spec/报告/任务上下文；trellis-implement代码，trellis-check独立验证。无migration/配置/API变更、无真实数据写入/部署。

## 必需回归与诚实证据

先抽出与旧代码等价的生产selection helper，写实际行为RED：valid+无效/旧ref应继续并保留valid，旧行为实际返回reply_evidence_not_authorized。不能用缺符号编译失败代替业务RED。再改helper GREEN，并证明service调用它。

纯测试覆盖all invalid、mixed、duplicate、missing offered、live changed/消失、scope mismatch；无效refs/snapshot内容不进Writer与诊断，输入/索引不变，有效fact语义原样。真实run/owner/conversation/draft/media/publication负例继续拒绝；不以放宽授权变绿。

尽量实际执行脚本HTTP/native Writer并检查最终输入与一次Writer调用；若全prepare/publication依赖PG且无可用环境，单列隔离PG场景SKIP，不能声称真实消息已送达。现环境无PG/pgvector/Docker daemon、不安装启动系统服务，测试明确关闭真实Provider/生产访问。
