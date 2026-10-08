# D：迁移与联合验收

## 目标与价值
完成存量修复对账与旧入口清理；upgrade/重启/Worker故障；46矩阵与三E2E分层；真实Provider/媒体证据；操作说明和最终报告。

## 来源与边界
父任务 ../10-07-fluctlight-goal-closed-loop/prd.md 与 source-requirements.md 为完整需求权威。本子任务负责 G11–G13，按父计划保持语义/权限/人格/真实来源边界；相关G11迁移/G12测试同步实施，不将只建模作为完成。
基线 master / ee1e468，工作区初始干净；已知事实与file:line见父 research/implementation-map.md。

## 需求与验收
完整A01–A10/B01–B12/C01–C12/D01–D12及G13六门禁；SKIP/BLOCKED不计通过，未满足保持进行中。
产品范围按父PRD全部保留；脚本Provider不当真实模型、数据库SKIP不当通过；最终范围以 source-requirements 对应 G 编号为准。

## 依赖
依赖 A/B/C生产接线完成，迁移/测试从首批就开始。当前planning，计划统一审阅批准后按顺序激活，不因创建任务提前实施。

## 2026-10-08 实际后台队列持续占用修复

最新用户规则与最小修改边界见 `research/background-trigger-contract.md`。唤醒为实际最后聊天后10分钟及后续每10分钟，启动无Redis key才补一次；反思为最后聊天后30分钟且仅新未处理证据；新认知入队取消同实例排队/执行中的唤醒与反思并阻止晚到提交；检查/静默回执不驱动Goal来源水位自循环。保留真实结果、独立Goal标准writer、已有Agent/Tool/短事务/outbox/Temporal。用户运行环境只读，本地修复不等于已部署/真实验收完成。
