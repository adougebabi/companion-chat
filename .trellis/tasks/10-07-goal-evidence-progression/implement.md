# B：真实证据与阶段推进 实施清单

## 前置
依赖 A 执行许可、Attempt与标准契约。用户批准父任务最新最终计划后，task.py validate本任务并start。主代理写代码/方案/最终验证，探子仅探索核验。按父 implement.md 的验证命令、隔离设施、最终报告与回滚点执行。

- [ ] 完整读父PRD/design/implement、对应规范与待改代码；确认HEAD/worktree/迁移head。
- [ ] 根据research入口复现本批缺口并编写有意义回归。
- [ ] 完成：独立 Stage/Commitment；真实来源引用与普通对话候选；durable Evaluation、CAS逐标准提交；统一购物/Reflection完成；下一步/Resolution/有条件后续目标。
- [ ] 同步增量迁移/存量夹具/生产调用迁移/幂等、权限与并发回归。
- [ ] 更新父 acceptance-matrix.tsv 为真实测试名、叶子结果与证据；本批验收：B01–B12、C01–C12、D02/D12；三E2E脚本Provider接线；无Intention表达可完成、单方表达不完成关系、真实库存完成获取。
- [ ] trellis-check与适当Go/client/web验证；环境失败记录BLOCKED和重跑，不改条件凑PASS。
- [ ] 更新spec、报告、任务进度/提交与journal；仅实际完成后archive，父目标还需D集成门禁。

## 高风险入口与停止点
goal_execution.go / tool_publication.go / agent_result_adapter.go / reflection_domains_v2.go / life_activity_resolution.go / builtin_capabilities.go / formal_agents.go。schema与worker/消息提交/完成编排要验证事务回滚、版本冲突、重放。未知副作用只核对，禁止failed Agent整轮重放；不修改生产数据。
