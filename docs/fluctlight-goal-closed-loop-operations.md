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
