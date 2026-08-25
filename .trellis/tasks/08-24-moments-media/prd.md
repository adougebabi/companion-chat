# T09 Moments 与 Media

## Goal

实现 Moments/comment/reaction/visibility/unread，以及 S3/MinIO media、image/video/ComfyUI/h3、quality/progress/compensation/tombstone/orphan/Range/backup 生命周期。

## Requirements

- 父任务与依赖门禁：T08 completed and merged。
- 实施范围：moments/media modules、feed/comment/reaction/visibility/unread、S3/MinIO assets、image/video/ComfyUI/h3、quality/progress/compensation/tombstone/orphan/Range/backup。
- 遵守父任务 `READ_FIRST.md`、`decisions.md`、`design.md`、capability inventory 和 assigned code-specs；不得静默改变父决策。
- 旧实现/旧测试只读冻结；不兼容、不双写、不分批切换。
- 本 child 在 start 前必须由父任务补齐 child-specific brief、exact decisions/manifests/owned paths/commands/handoff，并通过 no-history dry run。
- 实现实际 media adapter 的 Activity heartbeat/cancel/idempotency 和 Provider-success-before-completion recovery seams；最终 live crash/recovery 验收由 T12 执行，不要求固定运行时长。

## Implementation Evidence (Not Acceptance)

- [ ] 前置 child 已完成并合并；child brief、owned/forbidden paths、implementation-check commands、T12 coverage IDs 和 handoff 模板已由父任务批准。
- [ ] 本 child 只记录 media adapter、object lifecycle、workflow seams、生成物和最小本地检查；这些结果不构成 child PASS、production readiness 或 cutover authorization。
- [ ] Actual MinIO/S3、Temporal media queue、heartbeat/cancel、live crash/recovery、object integrity、proxy authorization 和 backup/restore validation 全部由 T12 重新执行；固定时长/resource soak 仍不属于验收。
- [ ] Future-only、reserved、placeholder-only 功能不在本 child 中做正向验证；相关 deferred risk 和 T12 coverage ID 必须进入 handoff。
- [ ] 冻结旧实现未修改，未触发产品分批交付或 cutover。
- [ ] 完成后提供包含 changed paths、implementation evidence、remaining risks、contract/schema artifacts、`acceptance_owner=T12` 和 `acceptance=pending` 的明确 handoff。

## Planning State

Child exists in `planning` but is not executable yet. The parent program outline is not implementation approval.
