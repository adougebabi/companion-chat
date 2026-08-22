# Technical Design

## Transaction Boundary

Make activity media publication accept the caller's transaction/transaction runner. Activity row creation, `media_status='queued'`, and the effect adapter enqueue must commit or roll back together. Preserve the existing frozen concept requirement and stable key `activity:<activityId>:media`.

## Source-to-Poll Follow-up

Introduce a media-specific atomic follow-up operation or a deterministic compensation handler that can inspect a source job and target. It must:

1. verify the source has a provider/external locator and the target is still processing;
2. find the existing poll job by deterministic `sourceJobId`/external id;
3. enqueue exactly one `activity_media_poll` or `chat_media_poll` job when absent;
4. leave a completed/ready/failed target unchanged on replay.

The source settlement and poll insertion should share one SQLite transaction when the provider returns pending. Where the current handler API makes that impossible, the compensation job is the durable recovery path and must be scheduled with a stable key.

## Quality Retry

Derive a stable successor key from source job id and retry count. Use the existing effect/job idempotency lookup before enqueue. Poll acceptance and successor enqueue must be replay-safe, and a stale worker must not update the target after lease loss.

## Worker Registration

Register the compensation handler with the existing media job service/dispatcher. The generic dispatcher remains responsible for lease, retry, backoff, and terminal settlement; media compensation owns only target/job consistency. Bounded errors are projected to the target only at terminal failure.

## Compatibility and Rollback

Existing chat media replay/repair behavior remains unchanged. New compensation logic must tolerate old jobs without extra metadata by returning a terminal, diagnosable failure rather than inventing a prompt. No schema deletion or data rewrite is required; additive payload fields are used for new jobs.
