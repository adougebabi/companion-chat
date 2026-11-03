# F16 基线只读定位

探子仅通过git show/git grep读取不可变ec1ed82，未碰F15工作区源码。主代理随后亲读现有对应函数。行号为该基线，实施后以符号定位。

- goal_evaluation_tools.go:34–59 physical policy：缺root即forced tool；:62仅stale错误可放行summary，无no-progress gate。
- formal_agents.go:71专用Agent未设MaxIterations；ai/agent/loop.go:264–269与369–374生产0值为最大int；adk_conversation_runtime.go:322–330不可重试业务拒绝也返回模型继续决定。外层5次claim仅在RunGoalEvaluationTask返回后才生效。
- goal_evaluation_tools.go:236–249 prior journal同target存在且digest不同返回goal_submission_conflict；相同digest重放。key=:196–205 capability+goalID+objectRef。错误不是DB并发CAS。
- goal_closure.go:337–361验证all normalized unknown + no_change会unknown_requires_evidence；缺criterion在:295–304补unknown，因此空judgments也触发。native instruction只说unknown，旧goal_evaluation_task.go:18明确unknown/needs_evidence映射却未被原生instruction继承。
- goal_evaluation_wire.go:694–701 hydrateJudgments同时把未知、跨goal、跨object、重复ref归为criterion_ref_invalid。root在Tool:355–360固定goal kind，所以不能用stage/commitment criterion。
- stable refs在goal_evaluation_wire.go:77–139确定，stableDefinitions:294–327逐criteria给ref/text；task:218–224及prompt_context_assembler:310/526提供STABLE TASK CONTEXT。冻结snapshot跨claim稳定，不能从日志推断ref漂移。
- canonical Tool schema tools.go:139–145把criterion_ref/evidence_refs变为任意string，私有binding实际校验；这是运行时绑定而非字段未提供。
- 所有领域拒绝tools.go:264共用“correct ... frozen refs”通用反馈，不给具体错误index/合法scope refs/impact对应规则。
- accepted投影submission_runtime.go:44–49只遍历record.Output，丢capability/kind/key且无序；root/plan都tools.go:449相同assessment_accepted/status/progress shape。task:229跨claim只传此投影，模型无法区分plan/root，旧rejection详情也不在current。
- finalizer submission_runtime.go:141–296只凭durable root journal计算coverage，missing返回native_submission_missing；不造Tool、partial accepted独立存活、未解决source保留。F16需在其上增加明示的操作性awaiting_evidence收束，不能放松业务完成真实性。
- runtime:84–89最多5claim；stale replacement仅针对stale集合，当前三错误不在其中。日志说明错误重复，但缺request/run/claim和原arguments，不能证明现场确实无限执行或确定无效criterion属于哪一种。

本轮用户明确要求确定性软件兜底优先于让模型反复改错，因此实施以goal-submission-recovery-followup.md为准；探子的保守“最终报failed”建议已由主代理收敛为可见、非伪造完成的handled/awaiting_evidence处置。
