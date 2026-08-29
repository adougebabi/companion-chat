# T09 No-History Handoff Dry Run

Date: 2026-08-25

T09 consumes T08 action intents and owns Moments and Media authority. It does
not modify the old implementation or claim T10/T11/T12 ownership.

## Execution

1. Define Moment/feed/comment/reaction/visibility/unread values and tables.
2. Define Media intent/asset/reference/tombstone/grant values and tables;
   create migration `0008`.
3. Implement private object upload/checksum/version/authorization lifecycle and
   injected Provider heartbeat/cancel/idempotency seams.
4. Implement progress/compensation/orphan/tombstone transitions and local
   contract checks.
5. Hand live MinIO, Range, crash/recovery, Redis replay and backup validation to
   T12.

## Exclusions / Risks

No BFF media proxy or UI is implemented here. No public bucket, local path,
second queue, or semantic media fallback is allowed. Fixed-duration resource
soak is excluded; actual crash/recovery remains a T12 scenario.

Conclusion: T08 handoff and assigned media/event/workflow contracts resolve the
planning boundary required to start this child.
