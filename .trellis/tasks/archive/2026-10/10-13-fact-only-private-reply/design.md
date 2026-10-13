# FO-C Fact-only 原生私聊闭环 技术设计

## 整体设计依据

依父任务 ../10-13-fluctlight-fact-only/design.md 执行；共同跨子任务合同不可被局部替换。

## 本子任务技术边界

每run只有一个reply合同：legacy text v1与fact_only refs/draft_ref/action_result_refs分离。能力业务在composition root创建run registry/execution view，Native binding只校验，不放生成I/O；生成在CapabilityPreparer内，PreparedPayload冻结body/provenance，DirectTx复用PublicationService和ledger。稳定逻辑reply key生成前查已发布与候选缓存，保持一个turn一条assistant及生成后source/life fence。fact_only model result回实际正文/提交状态，当前visible_output_committed不伪造network delivered。更新Main协议源与physical policy，最终普通text/schema不发送，认知字段不删除。只做确定事实/标识/能力检查，最多一次Cydonia纠正，fallback显式并同幂等入口。

## 兼容与失败

T02/03：真实registry→handler→Cydonia→消息receipt→ADK回读→final contract。T10/11：最终认知失败/网络重试/截断/OOM/取消无假成功或二次发送。T16/17：WakeUp静默/睡眠/权限与run固定，群聊/朋友圈不改。

## 证据

- ../10-13-fluctlight-fact-only/research/reply-runtime.md
- ../10-13-fluctlight-fact-only/research/settings-context.md
- ../10-13-fluctlight-fact-only/research/external-docs.md


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
