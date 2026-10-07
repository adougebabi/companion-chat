# C：Review、上下文与 Owner 实施清单

## 前置
依赖 B 统一证据/评价/推进。用户批准父任务最新最终计划后，task.py validate本任务并start。主代理写代码/方案/最终验证，探子仅探索核验。按父 implement.md 的验证命令、隔离设施、最终报告与回滚点执行。

- [ ] 完整读父PRD/design/implement、对应规范与待改代码；确认HEAD/worktree/迁移head。
- [ ] 根据research入口复现本批缺口并编写有意义回归。
- [ ] 完成：周期Review与停滞原因、软硬期限；通用execution/关键Runtime Facts；Goal查询/命令/重评Tool；Owner正式API、typed client、目标详情/历史/治理UI。
- [ ] 同步增量迁移/存量夹具/生产调用迁移/幂等、权限与并发回归。
- [ ] 更新父 acceptance-matrix.tsv 为真实测试名、叶子结果与证据；本批验收：D01–D11及C04/C06–C08；各Surface同态、Actor时区/DST、Owner CAS/权限/分页；browser actionPlan拒绝仍有效；native loop真实Tool反馈续轮。
- [ ] trellis-check与适当Go/client/web验证；环境失败记录BLOCKED和重跑，不改条件凑PASS。
- [ ] 更新spec、报告、任务进度/提交与journal；仅实际完成后archive，父目标还需D集成门禁。

## 高风险入口与停止点
goal_execution.go / autonomy.go / provider_context.go / reflection_runtime_v2.go / agent_result_adapter.go / httpapi/server.go / httpapi/browser/routes.go / OpenAPI与browser-client / InstanceDetailsDialog.vue / GovernanceView.vue / control-center.ts。schema与worker/消息提交/完成编排要验证事务回滚、版本冲突、重放。未知副作用只核对，禁止failed Agent整轮重放；不修改生产数据。
