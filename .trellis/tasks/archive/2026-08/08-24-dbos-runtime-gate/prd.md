# T01 DBOS 运行时门禁

## Goal

验证 DBOS 在本地/NAS PostgreSQL 环境中的队列隔离、durable sleep、长任务 heartbeat/cancel/recovery、管理操作、升级回放、资源阈值和诊断关联；失败则退回 Temporal 规划。

## Requirements

- 遵守父任务 D001、D003、D005-D007、D016、D020-D021、D028-D029、D031-D033。
- 严格执行父任务 `research/t01-dbos-runtime-gate-brief.md` 的 manifest、owned paths、persistence/diagnostics slices、operations、resource thresholds 和 failure escalation。
- 验证 `interaction`、`lifecycle`、`media` 三个 DBOS queue 的隔离、并发/rate policy 和同包分进程 Worker 模型。
- 验证 durable sleep、15 分钟 fake h3、heartbeat、timeout、cooperative cancellation、crash/restart 和 stable Provider/workflow ID。
- 验证 list/get/pause/resume/cancel/restart/fork-from-step、active-history upgrade/replay、backup/restore 与 built-in correlation。
- 只实现 runtime gate fixture，不实现 BFF/browser/Fluctlight domain/production schema/final Compose。

## Acceptance Criteria

- [ ] 父任务 artifacts 已获用户批准，本 child 已单独审阅并通过 `task.py start` 进入 `in_progress`。
- [ ] `fluctlight-workflow-contract.md` 的全部 prototype gates 通过。
- [ ] T01 brief 的七项 NAS resource thresholds 均通过三次测量并记录 median/max。
- [ ] 外部成功/checkpoint 前崩溃、Worker/DB restart、cancel/timeout 等故障注入只产生一个最终副作用/result。
- [ ] Canonical management operations 均可通过 official DBOS client/API 或 thin wrapper 完成并写 audit record。
- [ ] `research/dbos-runtime-gate-report.md` 从批准模板生成，含精确版本、命令、证据和 PASS 决策。
- [ ] 固定验证命令全部通过；不存在 Celery/custom queue/OTLP scope creep。

## Out Of Scope

- Fluctlight domain modules、正式 Unit of Work/outbox、Diagnostics tables/UI、Node BFF、Vue browser、final deployment。
- 对 DBOS core gate 的 workaround；任何 core failure 必须返回父任务 planning 评估 Temporal。

## Planning State

Child created but not started. Task creation is not implementation approval.
