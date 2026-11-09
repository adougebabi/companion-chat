# Goal evaluation quiet-period review

Candidate: `0717ec06bea896939251daa01ece1ae11687417a86c0e244145bc3547bd84b9c`
Method: source-level model review; no tests or build run.

## Review

- New source events update `goal_agent_cadence.last_activity_at` and increment `activity_revision` under the existing per-Fluctlight Life advisory lock. Existing unprocessed committed/withdrawn sources are backfilled once.
- The 30-minute due repair requires `last_activity_at + 30m <= now()` and `activity_revision > evaluation_activity_revision`. Admission consumes the current revision once, so the minute repair loop does not create another request without newer activity.
- Queue insertion includes the current quiet deadline and pending-request merges push `available_at` to the latest deadline. `claimGoalEvaluation` independently rechecks the latest activity before starting a model call, so requests created by direct SQL triggers, retries, and already-running workflow intents cannot bypass a reset deadline.
- Activity added while a request is processing increments a later revision. The active request consumes only its admitted revision; the new revision remains eligible after its own quiet window.
- `owner_reassess` bypasses the activity quiet deadline and keeps the existing post-admission cadence fence. Tool/review requests, retries, and internal remainders retain explicit paths while respecting a recent activity deadline where applicable.
- No-activity automatic requests settle as skipped without a model admission.

## Result and limits

Pass for the approved code-path behavior by source review. `gofmt` completed and `git diff --check` returned clean. Automated tests, compilation, database migration execution, and live timing were not run; the task explicitly excludes tests for this turn. The repair loop polls once per minute, so message-only activity discovered by repair can start up to about one minute after the 30-minute quiet deadline.
