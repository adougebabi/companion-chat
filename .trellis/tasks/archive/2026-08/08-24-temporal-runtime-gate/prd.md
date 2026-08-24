# T01B Temporal 运行时门禁

## Goal

验证 grouped non-HA Temporal Server + PostgreSQL visibility 在 16 GiB NAS 上的 signals/queries/cancel、history replay/versioning、continue-as-new、长任务 heartbeat/recovery、管理操作、资源与长期运行；通过后才解除 T02 阻塞。

## Requirements

- 遵守父任务 D001/D003/D005-D007/D016/D020-D022/D028-D029/D031-D035。
- 严格执行父 `research/t01b-temporal-runtime-gate-brief.md` 的 exact manifest、owned paths、topology、operations、history/versioning、failure 和 resource gates。
- 使用 grouped non-HA Temporal Server + PostgreSQL default/visibility；禁用 Elasticsearch/UI/Prometheus/OTLP 默认依赖。
- 验证 three task queues、stable Workflow/Provider IDs、durable timers、Signals/Queries/Updates、Activity heartbeat/timeout/cancel/recovery。
- 验证 saved-history Replayer、current Worker Deployment Versioning、rollback/drain、continue-as-new、reset/restart/repair 和 active-workflow backup/restore。
- 只实现 Temporal runtime gate，不实现 Fluctlight domain、BFF/browser、正式 schema 或 Control Center。

## Acceptance Criteria

- [ ] Parent Temporal switch 已批准，本 child 独立审阅后通过 `task.py start` 进入 `in_progress`。
- [ ] `fluctlight-workflow-contract.md` 与 `fluctlight-temporal-gate-contract.md` 全部门禁通过。
- [ ] 所有 canonical operations、live failure injection、v1→v2 history/versioning 和 continue-as-new 通过。
- [ ] 九项 NAS resource/disk thresholds 经过三次测量并全部通过。
- [ ] 在实际目标 NAS 完成 12 小时固定 workload/restart soak，RSS/connection/backlog/disk 和零重复/卡死判据全部通过。
- [ ] PostgreSQL default+visibility backup/restore 能恢复 active workflow。
- [ ] Ruff format/lint、mypy、完整 Core test suite、clean-volume functional/soak runner 全部通过。
- [ ] Current production/default dependency、script、source/test、Compose 路径不再包含 DBOS；历史 archive/report/commits 保留。
- [ ] `research/temporal-runtime-gate-report.md` 含精确版本/命令/证据并判定 PASS。
- [ ] 不存在 start-dev/Elasticsearch/default UI/metrics、DBOS/Celery/custom runtime 或旧系统修改。

## Out Of Scope

- T02 平台基础、Fluctlight domain、BFF/browser、正式 migrations、Diagnostics tables/UI。
- 对 Temporal core gate 的 workaround；失败必须返回父 workflow-runtime planning。

## Planning State

Child-level planning is complete. Consult `task.json.status` for live state; implementation is authorized only when it is `in_progress` with an exclusive writer.
