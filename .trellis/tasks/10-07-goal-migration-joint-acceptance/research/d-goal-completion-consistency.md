# D：模型完成声明与领域完成一致性修复（2026-10-08）

## 现场结论
只读登录用户授权的现场，目标状态 active / progress 0 / revision 1；evaluations 为空。
评估请求 failed，attempt_count=5，error_code=goal_evaluation_wire_criterion_ref_invalid。
成功标准原文：晴岚根据对方实际提供的阅读偏好，在正式消息中给出一本具体小说的推荐及相关理由。
无小说篇幅比例阈值。结束后退出登录并验证 authenticated=false，删除本次 cookie。

用户提供的响应首先因同一 criterion_ref 重复而在 hydrateJudgments 拒绝；impact=progressed 并非结构化完成请求。
另外还包含无效 stage/object 引用、quotation 正向判断、Goal 2 有计划却无评估。不能以自由文本声明绕过这些门禁。

## 修改与边界
- 固定协议明确每 Goal 必须一条 evaluation、每 criterion 仅一条 judgment，多证据合并；all/any/optional 完成策略与 impact 一致；禁止自创阈值；无对象省略可选评估，新建不填 object_ref。
- ApplyGoalEvaluation 在来源/标准政策验证后拒绝 complete=true 且 impact!=completed，错误 goal_evaluation_completion_impact_mismatch；保留既有反向未满足却请求完成校验。
- Core 评估策略 goal.evaluation.v2；持久 memo 的 evaluation_policy_version 必须匹配当前策略，缺失/旧版缓存失效。版本不进入模型协议或提示词。
- 不从文字推导状态，不自动修补无效模型字段，保留批事务/CAS/关系确认/暂停治理；无 API、数据库迁移、前端或生产状态变更。

## 验证
最终隔离 PG 运行：go test -race -json ./internal/core -run '^TestGoal' -count=1。
108 个测试 PASS / 97 个叶子 PASS / 0 FAIL / 0 SKIP；含重复 criterion、满足标准但 progressed、两个目标漏一评估不消费来源、不写 Evaluation/Resolution、合法推荐完成、all/any/optional、暂停、关系确认、旧缓存失效与正常缓存。
go vet ./...、go build ./...、gofmt、git diff --check 通过。
独立只读审查未发现暂停/终态/关系/缓存调用路径问题；策略版本补充后主线程核验读取/匹配/持久化闭环。

早期新增双目标测试误用 JSON 中省略的 EntityID 定位，触发 criterion 错误；已改用冻结入口 GoalID，复跑通过。保留该失败输出和全部阶段证据，不把失败计通过。
最终原始结果：d-goal-completion-policy-final.jsonl。

## 待验收
本地改动尚未部署，也未运行用户真实 Provider；不能保证模型仅凭提示词改动就消除所有语义错误。
现场仍是原失败状态。部署后须通过现有正式重新评估入口重试，以实际成功 Evaluation / Goal completed / Resolution 为验收；不直接改状态。
父任务仍 in_progress，保留 docs/fixtures/ 未提交人格文件。
