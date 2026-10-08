# Goal closed-loop operations

This branch introduces 0050_goal_execution → 0051_goal_closure → 0052_goal_governance → 0053_goal_reconciliation. The implementation report and 46-row acceptance matrix still distinguish verified layers, partial scenarios and blocked live semantics. This document is an operator procedure, not authorization to modify production.

## Deployment and data preservation

Before deployment, stop API/Worker writes, take and test a PostgreSQL backup, save the currently deployed binary/image and record the schema head. Use the existing `apps/core-go/cmd/migrate` command with the explicitly selected deployment database. API/Worker do not auto-migrate. Check readiness against 0053, then start workers and monitor pending/retry/processing Goal Evaluation requests, failed workflow intents and `needs_reconciliation` Attempts.

Stable criterion IDs/version are deterministically backfilled by 0050. No migration invents completed stages, proof, inventory, messages or Attempts. Original descriptions, standards, lifecycle, progress and old history stay available. Older completed Goals lacking a verifiable Resolution are preserved and explicitly flagged for review by the controlled reconciliation command, not reopened or silently approved.

## Bounded stock inspection and application

Select one authorized Owner and Fluctlight. The default is read-only and makes no model calls:

```sh
go -C apps/core-go run ./cmd/goal-reconcile --owner OWNER_ID --fluctlight FLUCTLIGHT_ID --limit 20
```

Review each reason, Goal revision and source waterline. Apply only that reviewed batch:

```sh
go -C apps/core-go run ./cmd/goal-reconcile --owner OWNER_ID --fluctlight FLUCTLIGHT_ID --limit 20 --apply --digest REVIEWED_DIGEST
```

Continue with `--after NEXT_CURSOR` and a fresh preview/digest. A changed batch conflicts. Repeating a committed digest returns its saved result. Stop between batches by stopping the CLI; no background backfill loop is installed. Active missing plans and unprocessed actual sources enter bounded existing Goal Evaluation workflows; simple goals can remain short chains. SQL never calls a model or fills every day with stages.

The report identifies unsettled/unknown operations but the command does not reissue purchases, sends or external jobs. Existing Worker `ReconcileGoalAttempts` checks committed operation authority using the original logical identity, records finite recovery checks and exposes `needs_reconciliation` when still unknown. Review that operation and its receipts before any explicit next action. Cancelled/abandoned Goals are not queued for automatic progression.

## Failures and history

Provider/model failure retains a retry/failed evaluation request; it does not write a false `no_change` or completion. Processing leases use claim revisions. Source corrections withdraw current associations and queue active Goals; ended Goals retain completion history with review flags. Review cycles use Actor IANA timezone, UTC boundaries and date arithmetic for DST. Late sources reopen the existing local cycle with a new revision, without adding a daily quota.

Owner operations use session/service authorization, expected revision, reason and stable idempotency key. The browser preserves 409; drafts retain their target Goal and version. Completion requires real evidence and evaluation. Query Goal details/history for evidence, standard versions, original/follow-up Resolution, current Stage/Commitment, recent Attempt/outcome, retry and Review records.

## Recovery and rollback

Prefer forward repair after new schema records exist. Do not describe dropping the new tables as a lossless rollback: the old binary cannot understand new attempts, criteria revisions and evaluation authority. A full rollback requires stopping writes and restoring the reviewed predeployment backup and compatible binary together. Retain outbox/workflow identities and audit records; never delete real side effects to simulate cancellation.

## Validation layers

Use a disposable PostgreSQL URL with `GO_CORE_TEST_DATABASE_URL`. Real Worker joint tests additionally require task-owned `GO_GOAL_TEST_TEMPORAL_ADDR` and `GO_GOAL_TEST_REDIS_ADDR`; they use scripted Provider semantics. Live model tests require `FLUCTLIGHT_LIVE_PROVIDER_URL`, `FLUCTLIGHT_LIVE_PROVIDER_MODEL`, optionally the API key and explicit live flag. Real media needs isolated `FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE` and storage configuration. Missing live/media settings are BLOCKED/SKIP, never counted as semantic/media quality PASS.

## 2026-10-08 唤醒与反思的安静期规则

新版唤醒从最后实际用户或助手聊天起10分钟到期，之后每10分钟一轮；
新的聊天重置计时，错过的周期不会集中补发。Worker启动保留已有Redis
key，只有缺key且没有同实例待处理认知才补一轮。

反思在最后聊天后30分钟、存在未反思真实证据时才调用模型；无新情况
返回no_op/no_real_evidence。静默唤醒和纯检查不会创建下一轮反思，也不
推进Goal复核来源水位。Goal Evaluation是实际证据/显式复核驱动的目标
标准评估，2秒用于合并事件，不是每2秒执行一次的定时任务。

新认知入队会取消同实例的排队/执行中唤醒和反思，并拒绝旧结果提交。
实际业务结果仍保留；唤醒自己真正发布的消息不会被误记为取消。

发布时同步更新API、Worker和Web，以保证600秒设置投影、时钟和说明一致。
本轮没有部署或清理用户实际队列。验收应查看实际last-chat时间、下一次
到期、model-run queued/running/cancelled状态及来源水位；ADK多轮和有限
失败重试可能让一个触发产生多个模型调用，不能把调用次数当触发频率。
