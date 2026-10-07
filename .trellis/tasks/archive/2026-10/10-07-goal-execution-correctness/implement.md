# A：执行资格与 Attempt 正确性 实施清单

## 前置
无前置子任务。用户批准父任务最新最终计划后，task.py validate本任务并start。主代理写代码/方案/最终验证，探子仅探索核验。按父 implement.md 的验证命令、隔离设施、最终报告与回滚点执行。

- [ ] 完整读父PRD/design/implement、对应规范与待改代码；确认HEAD/worktree/迁移head。
- [ ] 根据research入口复现本批缺口并编写有意义回归。
- [ ] 完成：统一关联执行许可；认领即持久化 Attempt；首 Tool 前失败与同步/异步完整结算；有界分类退避和未知副作用核对；稳定标准 ID/版本、混合修改拒绝。
- [ ] 同步增量迁移/存量夹具/生产调用迁移/幂等、权限与并发回归。
- [ ] 更新父 acceptance-matrix.tsv 为真实测试名、叶子结果与证据；本批验收：A01–A10、B01–B03/B11；native/scheduled/direct Tool/恢复均不可绕过 Goal 状态；首工具失败不永久 due，纯QUERY不永久 pending。
- [ ] trellis-check与适当Go/client/web验证；环境失败记录BLOCKED和重跑，不改条件凑PASS。
- [ ] 更新spec、报告、任务进度/提交与journal；仅实际完成后archive，父目标还需D集成门禁。

## 高风险入口与停止点
goal_intention.go / evolution_persistence.go / intention_runtime.go / scheduled_intention_runtime.go / agent_result_adapter.go / action_outcome.go / life_activity_capabilities.go / workflow/workflow.go。schema与worker/消息提交/完成编排要验证事务回滚、版本冲突、重放。未知副作用只核对，禁止failed Agent整轮重放；不修改生产数据。

## 已验证检查点
见父 research/a-checkpoint.md。执行资格、Attempt、标准基础契约已落地；下一批沿同权威服务补证据与完整Evaluation。所有整体门禁留父/D验收，不将42叶子测试当完整E2E。
