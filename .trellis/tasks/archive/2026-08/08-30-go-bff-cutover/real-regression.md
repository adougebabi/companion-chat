# Real Docker regression record

Date: 2026-08-30 (Asia/Shanghai)

## Deployment under test

- Web URL: `http://127.0.0.1:13001/`
- BFF origin discovered from Web `runtime-config.js`: `http://127.0.0.1:13000`
- BFF implementation under test: Go BFF rebuilt from `codex/go-bff-cutover`
- Core, Worker, PostgreSQL, Redis, MinIO, and Temporal were not rebuilt or
  cleared. The Worker was restarted to re-register daily-review workflows.
- Owner password was supplied out-of-band and is intentionally not recorded.

## Results

### Preflight

- Web runtime configuration points to the BFF origin.
- BFF `/health/live`: HTTP 200.
- BFF `/health/ready`: HTTP 200.
- BFF `/auth/session` after login: HTTP 200.
- BFF container rebuilt from this branch and recreated in the existing Compose
  project without restarting Core or deleting volumes.

### Case 1 — 影者 normal text chat

PASS. The real BFF turn returned HTTP 200 and 121 NDJSON events containing a
visible `message`, visible `token` events, and exactly one `completed` terminal;
no error event was emitted.

### Case 2 — 影者 photo request

The first real request returned HTTP 200 with visible text but ended with the
Core error `secondary_effect_must_produce_an_autonomous_side_effect`. The
failed turn was retried with the same `turnId` and `idempotencyKey`, as required
by the Core contract. The retry returned HTTP 200 with one `completed` terminal.
Media generation then settled asynchronously: the conversation exposed three
`media_reference` messages, and all three assets returned through Go BFF as
HTTP 200 `image/png` with non-empty bodies (157462, 205625, and 174778 bytes).

Final result: PASS with one recoverable Core/Provider first-attempt failure
recorded; Go BFF preserved the error and idempotent retry behavior.

### Case 3 — blank Fluctlight creation

PASS. Go BFF `POST /api/fluctlights` returned HTTP 200 and an active Fluctlight
with ID `fluctlight_51399fb48530478cb677c012b86e8152`.

### Case 4 — description-based Fluctlight creation

PASS. Go BFF analysis returned a complete foundation (identity, personality,
behavioral policy, life profile, goals, intentions, and provenance). Activation
returned HTTP 200 and active Fluctlight ID
`fluctlight_aacf071dc856d8018336deeb164c71d6` named `晨雾记录者`.

### Case 5 — complete detail and schedule

PASS after applying the documented schedule contract. The initial schedule
attempt correctly returned 422 because a first schedule must omit
`expectedRevision`; resubmission through Go BFF with a contiguous 00:00–24:00
Asia/Shanghai schedule returned HTTP 200. Detail then showed:

- identity: 13/13 non-null fields;
- personality: 14/14 non-null fields;
- behavioral policy: 15/15 non-null fields;
- life profile: 7/7 non-null fields;
- 3 active goals and 4 pending intentions;
- accepted schedule revision 1 with 8 contiguous items.

### Case 6 — Fluctight publishes a Moment

PASS. After a Worker restart, the new Fluctlight produced a visible Moment via
the real daily-review/autonomy path. The Moment had non-empty text, status
`visible`, and a settled media asset. The corresponding autonomy action had
type `moment` and status `completed`.

### Case 7 — Fluctlight proactively contacts Owner

PASS for the existing real `影者` Fluctlight. Go BFF returned two completed
`proactive_message` autonomy actions, including one created at
`2026-08-30T02:54:15Z`; the direct conversation contained the matching
assistant message at the same time. This proves the public Go BFF can expose a
real autonomous contact result.

The newly created `晨雾记录者` daily review chose a completed `moment` action
instead of a proactive message within the ten-minute observation window. A
separate proactive-only test object also produced `no_op` while the Worker was
processing the existing media/reflection backlog. These are recorded as Core
LLM decision/backlog observations, not hidden as BFF successes.

## Cleanup

The temporary `product.autonomy` setting used to isolate the proactive test was
removed through an exact PostgreSQL cleanup after verifying it was absent in the
pre-test settings snapshot. The settings API again reports only the original
`providerUrl` and `media.comfyui` values. No other test data was removed.
