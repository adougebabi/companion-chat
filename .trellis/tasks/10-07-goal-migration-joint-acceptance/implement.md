# D：迁移与联合验收 实施清单

## 前置
依赖 A/B/C生产接线完成，迁移/测试从首批就开始。用户批准父任务最新最终计划后，task.py validate本任务并start。主代理写代码/方案/最终验证，探子仅探索核验。按父 implement.md 的验证命令、隔离设施、最终报告与回滚点执行。

- [ ] 完整读父PRD/design/implement、对应规范与待改代码；确认HEAD/worktree/迁移head。
- [ ] 根据research入口复现本批缺口并编写有意义回归。
- [ ] 完成：完成存量修复对账与旧入口清理；upgrade/重启/Worker故障；46矩阵与三E2E分层；真实Provider/媒体证据；操作说明和最终报告。
- [ ] 同步增量迁移/存量夹具/生产调用迁移/幂等、权限与并发回归。
- [ ] 更新父 acceptance-matrix.tsv 为真实测试名、叶子结果与证据；本批验收：完整A01–A10/B01–B12/C01–C12/D01–D12及G13六门禁；SKIP/BLOCKED不计通过，未满足保持进行中。
- [ ] trellis-check与适当Go/client/web验证；环境失败记录BLOCKED和重跑，不改条件凑PASS。
- [ ] 更新spec、报告、任务进度/提交与journal；仅实际完成后archive，父目标还需D集成门禁。

## 高风险入口与停止点
migrations/runner.go / isolatedCoreTestRepository / infra/acceptance / core与workflow与httpapi测试 / web测试 / docs报告。schema与worker/消息提交/完成编排要验证事务回滚、版本冲突、重放。未知副作用只核对，禁止failed Agent整轮重放；不修改生产数据。

## 2026-10-08 实际后台队列持续占用修复

最新用户规则与最小修改边界见 `research/background-trigger-contract.md`。唤醒为实际最后聊天后10分钟及后续每10分钟，启动无Redis key才补一次；反思为最后聊天后30分钟且仅新未处理证据；新认知入队取消同实例排队/执行中的唤醒与反思并阻止晚到提交；检查/静默回执不驱动Goal来源水位自循环。保留真实结果、独立Goal标准writer、已有Agent/Tool/短事务/outbox/Temporal。用户运行环境只读，本地修复不等于已部署/真实验收完成。

## 2026-10-08 Provider语义边界、稳定前缀和Goal重复评估

新增用户要求见 `research/goal-provider-semantic-cache-contract.md`：去掉模型输入/输出的内部ID与版本，改冻结短引用及Core绑定；同数据评估JSON/TOON，选择实测适合的编码；同Agent稳定协议/人格/定义置前，动态事实后置；成功判断的相同证据持久去重，新证据/约束/标准/复核或显式Owner重评仍触发。完整CAS/作用域/证据/续批门禁保留。真实KV命中和模型语义需实际环境指标，不将脚本估算当验收。
