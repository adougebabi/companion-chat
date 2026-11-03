# 2026-10-08 真实推荐目标未完成：评估输入超限

用户授权通过其运行环境登录并只读调用接口。真实科幻偏好与已发布作品推荐满足该测试目标的标准；目标仍 active、revision=1、progress=0，无 Evaluation/Resolution。最新查询评估请求为 retry、attempt_count=3，错误为 `prompt_required_budget_exceeded: prompt_current_input_budget_exceeded: current=53609 cap=16384`。并非等待用户读完小说或额外认可。禁止将此实际反馈记为通过。

根因：多轮 goal.inspect 的完整详情和 actor.inspect 审计数据经 ActionOutcome 反复进入 Sources；数量限制没有控制序列化体积，compactGoalEvaluationInput 原样传入 Data。当前输入在 Provider 调用前即被预算门禁拒绝。

修复保留完整 durable/CAS snapshot，对 Provider 视图投影冗余 Goal 快照与 Actor 审计字段，保留 Actor 主体、事实值、生效区间、消息全文和真实领域查询。既有 judgment 证据必须进入本轮；新消息优先，同级按 recorded_at/ID 倒序，Provider Sources 控制在 12000 估算 token。完整标准或必需证据不可容纳时明确失败，不截断证明、不提高 16384 上限。只接受本轮提供的引用，只消费这些来源；遗漏证据为仍活动目标排 assessment_source_remainder。

回归使用一次性真实 PostgreSQL + 脚本 Provider，40 条合法 goal.inspect 历史、16 条真实领域查询形状和完整偏好/推荐消息；推荐目标产生一条 Resolution、零虚构 Attempt，另一个目标的具体遗漏来源保持未消费并在下一批处理。独立只读复核发现 Actor 工具实际输出 actor_id/value/effective_at 与 DB 源不同；已补齐两种形状并用唯一主体/值/时间分别验证。

最终 `d-goal-input-budget-final-race.jsonl`：104 PASS events / 91 leaf PASS / 0 FAIL / 0 SKIP。Go Core internal/core vet、全包 build、gofmt/diff check 通过。脚本 Provider 不证明真实模型语义通过；此前全包结果仍为其当时版本的证据，不覆盖本次线上累积来源负载缺口。

没有更新远端 Worker、写生产库、重新评估或强制完成目标；实际环境需要部署本次 Worker 修复后再按正式评估入口验证。真实验收保持进行中。远端原文/提示词/session/password 不进入仓库；此处只保留脱敏状态与统计。
