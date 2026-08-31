# Go Core full-migration real regression (2026-08-30)

Environment: existing Docker Compose project `fluctlight-t12-browser-53392`,
with PostgreSQL/Redis/MinIO/Temporal volumes preserved. Requests were sent
through the Go BFF container's public HTTP routes (`127.0.0.1:3000` inside the
container) to avoid an intermittent OrbStack host-port forwarding failure.
The Core, Worker, BFF and Provider were real; no mock Core/Provider/Media was
used. Request ceilings were at most ten minutes.

## Evidence

| Case | Result | Evidence |
| --- | --- | --- |
| 1. 影者 text chat | PASS | New turn returned `message → token → completed`, sequences `0,1,2`, and the assistant message was persisted in the direct conversation. |
| 2. 影者 image request | PASS after first request | The first request returned `completed` with `media_intent_010aedd2bf368037242ad6f35c3dbf3d`; the Go Worker completed the real ComfyUI job `e4455906-d069-43fb-9914-90eccde59199`; PostgreSQL contains a ready `193940` byte `image/png` asset and one `media_reference`; Go BFF full GET and `Range: bytes=0-4` returned non-empty bytes, `206`, `Content-Range` and `image/png`. No same-idempotency retry was used. |
| 3. Blank Fluctlight | PASS | Public Go BFF returned active `fluctlight_21031d39b2a2427bb477b0f8e94e3d9b`. |
| 4. Described Fluctlight | PASS on independent request | Real Provider analysis returned a complete foundation (grouped goals/intentions were structurally normalized); activation returned active `fluctlight_2933b65778ac66d8b292ae43cbb2a5c6` with populated identity/personality/policy/life profile/provenance/goals/intentions. |
| 5. Detail and schedule | PASS on independent schedule fixture | Detail returned the populated foundation and agency fields. A new `Asia/Shanghai` full-day fixture for `2026-09-01` returned schedule revision 1 with two contiguous items. |
| 6. Moment publication | PASS after harness fix | Go Worker created `autonomy_7b3e60fe02da66f7d7833884f45ff4ed` (`moment`, `completed`) and visible `moment_ec8b92b2b25a5b00f1b7192790324560`. Public BFF GETs returned both, and `infra/acceptance/check-go-projections.sh` now verifies them deterministically. |
| 7. Proactive Owner contact | PASS (preserved projection) | Public BFF returned two completed `proactive_message` actions for 影者 and matching assistant messages; `check-go-projections.sh` verifies this path. The current Go daily-review run for 影者 selected `moment`; no new Go proactive action was forced by a manual database write. |

## Failed attempts kept as evidence

- A host-side case-1 request was disconnected by the transient OrbStack port
  forwarding failure; it left only the user message and no assistant result.
- Two independent description-analysis attempts returned the real
  `initialization_foundation_invalid` response before foundation shape
  normalization was added.
- The original current-day schedule fixture returned `schedule_timezone_invalid`
  because the Alpine image lacked `tzdata`; the image was fixed and the
  independent next-day fixture passed.
- The first current-day schedule request and the second next-day fixture are
  not silently replayed under the same idempotency key.
- The first cognition-enabled image request returned `media_concept_invalid`
  because the real Provider encoded `visual_concept` as a string. The parser
  was corrected to normalize that structural shape only; no semantic fallback
  or duplicate-idempotency retry was used.

## Cutover and clean-start evidence

- Go `fluctlight-cutover-go --apply` cancelled 21 running legacy Temporal
  executions and inserted 21 `legacy_fence` audit rows. No Temporal history,
  PostgreSQL facts, or MinIO objects were deleted.
- A fresh PostgreSQL database reached `0020_media_provider_job`, installed the
  `vector` extension, and received a one-time setup token from
  `fluctlight-setup-token-go` (plaintext length 70, digest-only persistence).
- The stray one-shot `relaxed_blackwell` migrator container was removed; the
  Compose project now has one Go Core API and one Go Worker owner.
- The cognition-enabled final image request `go-core-final-image-1788100572`
  returned `media_request` with a completed frozen action; its new ComfyUI
  job produced a ready `197795` byte PNG and one media reference. Full and
  `bytes=0-4` BFF reads both passed.
- A disposable fresh API container completed `setup-status → setup(token,
  password) → login → session`, with all four authenticated/ready checks true.
- The final deployed image produced a new real text turn
  `go-core-final-text-1788102376`; its inbox was `processed`, frozen action was
  `reply/completed`, and the matching `cognition.fact.created` outbox event was
  present. The public projection checker passed after the final canonical
  queue deployment.

## Final gates

- Go Core/Worker/BFF race tests, vet and builds: passed.
- Browser workspace `pnpm generate`, typecheck, tests and build: passed.
- `legacy-scope-guard.sh`, `go-core-reference-guard.sh`,
  `check-core-openapi.sh` and `validate-handoffs.sh`: passed.
- Final Compose Core/Worker images are `yourname/fluctlight-core-go:latest`;
  Core and BFF readiness are healthy, and the legacy fence dry-run reports no
  running retired-runtime execution.

The original polling harness timeout remains listed above as a failed attempt,
but the fixed public projection check passes. A fresh Go proactive decision is
not forced because autonomous action choice is LLM-owned; the preserved
completed projection is the acceptance evidence for this existing persona.

## P1 regression rerun (2026-08-31, post-fix)

The running Compose project was exercised again through the public Go BFF at
`http://127.0.0.1:13000` with a fresh curl cookie jar. All mutations used new
idempotency keys and the normal BFF → Core → Temporal/Worker → Provider path;
no database rows were inserted or altered manually and no Provider/Core mock
was used.

| Case | Result | Evidence |
| --- | --- | --- |
| 1. 影者 text chat | PASS after fix | New conversation `conversation_b2c8fbf36260bb0e2bbd11ac92b5afa5`, turn `turn_6a940385fdd9f46290441aaf40b18664`, returned NDJSON `message → token → completed` with sequences `0,1,2`; the inbox was `processed`. |
| 2. 影者 image request | PASS after fix | New `media_intent_498b2ea86a3114ab32d3ebe25b24d5e7` reached `completed` with real ComfyUI job `7332cee0-896a-4b33-98bf-5245f06300ba`; asset `asset_media_intent_498b2ea86a3114ab32d3ebe25b24d5e7` is `ready`, `image/png`, 185242 bytes, and has one conversation reference. Public BFF full read returned `200`; `Range: bytes=0-4` returned `206` with five bytes and `Content-Range: bytes 0-4/185242`. |
| 3. Blank Fluctlight | PASS | `POST /api/fluctlights` created active `fluctlight_7d652acca750091cd83974fb6a3b58e1` with the normal blank identity. |
| 4. Described Fluctlight | PASS | Real initialization analysis returned identity, personality, behavioral policy, life profile, goals and intentions. Correct activation (including top-level `provenance.foundation`) created active `fluctlight_f8a95d2056f087f29db9222fe78e6ab9`, named `回归晨雾`. |
| 5. Detail and schedule | PASS after lifecycle retry | Final detail for `fluctlight_f8a95d2056f087f29db9222fe78e6ab9` returned the full foundation/provenance, 3 goals, 1 intention, and accepted `schedule_3ebea377bc43a00ad45c16ccd7f976e6` for `2026-09-01`, `Asia/Shanghai`, revision 1 with 18 contiguous items covering the local day. |
| 6. Moment publication | PASS (preserved Go projection) | `check-go-projections.sh` passed against Go-owned `fluctlight_fb0e3e88bd402a65e4afd910cf676531`; it found a completed `go-autonomy:` moment action and visible Moment. |
| 7. Proactive Owner contact | PASS (preserved Go projection) | The same public projection check passed against `fluctlight_60fc292f8a3b95cd8dad33196f5544f3`; it found a completed `go-autonomy:` proactive-message action and persisted assistant message. The new `回归晨雾` daily review intentionally chose `no_op` at 02:23 Asia/Shanghai because its generated schedule was in a sleep block; this is a model-owned decision, not a missing workflow. |
| 8. Historical memory | PASS after claim-label fix | A real `memory_event` stored `memory_66963d6d4a384e6001ac1964b84db14f` (`semantic`, active, confidence 0.95, importance 0.9) for the owner's blue-gray/overcast preference. A subsequent fresh recall turn returned `记得。阴天、蓝灰、克制，不碰饱和橙……` with `message → token → completed` and created no duplicate semantic Memory. |
| 9. Reflection | PASS | Reflection workflow produced and applied `reflection_1e61dbbdb382cea55d697ef81d0430a6` over sequences 41–42. Its evidence-backed memory candidate became active `memory_e876fdc72d7c8fcb21363c028ed40786`; proposal status is `applied` and the watermark advanced. |

The rerun exposed and fixed these deterministic defects before the final
results above:

- `scene_event` and owner life-event inserts now repeat the predicate of the
  partial `(fluctlight_id, idempotency_key)` index, avoiding PostgreSQL
  `42P10`/`scene_persist_failed`.
- Media activities heartbeat while the slow prompt/download path is in flight,
  and retries with a persisted Provider job skip prompt regeneration.
- Schedule planning uses the contract-mandated `cognitive_assessment` role;
  the prior `reflection` role returned the wrong shape and left the lifecycle
  intent pending.
- Pending daily review retries clear a stale activation date, and daily review
  uses the same `Asia/Shanghai` default as schedule generation when the model
  omits an identity timezone. This prevents “schedule exists but daily review
  never sees it”.
- Cognition accepts the real Provider label `user_preference` conservatively as
  `observed_fact`, preserving evidence requirements without semantic promotion.

The original failed attempts remain in PostgreSQL as diagnostic evidence (the
first scene SQL failure, the first unsupported recall label, stale-date schedule
timeouts, and the current fixture's legitimate `no_op` decision). Embedding
jobs continue to report the configured `mlx-serve` endpoint as unavailable;
authoritative Memory rows and full-text recall are independent of that optional
vector index and were not hidden or rewritten.

## Continuation verification (2026-08-31)

The existing Compose project `fluctlight-t12-browser-53392` was rebuilt from
the `codex/go-core-full-migration` worktree. PostgreSQL, Redis, MinIO and
Temporal volumes were preserved; no database reset, manual domain write or
same-idempotency retry was used.

| Case | Result | Evidence |
| --- | --- | --- |
| 1. 影者 text chat | PASS | Fresh public request `go-final-bff-case1-*` returned browser NDJSON `message → token → completed` with sequences `0,1,2` and one terminal. A second fresh request after the final BFF rebuild produced the same sequence. |
| 2. 影者 image request | **ENVIRONMENT FAILURE (not accepted)** | Fresh requests created durable `media_intents`, but the real ComfyUI `/prompt` call returned HTTP 400 because configured `moodyCutieMixKrea2_v30_mixed_4_8.safetensors` is absent from the current ComfyUI transformer list (v40/v60/transformer_mixed are available). Go preserves this failure and does not silently select another model. Existing real asset `asset_media_intent_d8037ae3240a424cb84d210cb9f6e3f4` still passed public BFF full read (`200`, `195045` bytes, `image/png`) and `Range: bytes=0-4` (`206`, five bytes, correct `Content-Range`). |
| 3. Blank Fluctlight | PASS | Public Go BFF created `fluctlight_877345a506db8d1d89259beacb9cc4a4` with `status=active` and default identity. |
| 4. Described Fluctlight | PASS | Real initialization analysis returned complete typed foundation/goal/intention collections; activation created `fluctlight_8f368e697bd4154b2697432e84b01ee5` with populated identity, personality, policy, life profile and provenance. |
| 5. Detail and schedule | PASS | Public detail for `fluctlight_8f368e697bd4154b2697432e84b01ee5` returned three goals, three intentions, an accepted `Asia/Shanghai` current-day schedule with 11 contiguous items, and context sourced from the active schedule item. The fixture was generated before the final `reschedule_policy` readback patch, so its historical policy column is empty; new schedules preserve the Provider policy. |
| 6. Moment publication | PASS | `check-go-projections.sh` passed against the final public Go BFF using preserved Go-owned Moment fixture `fluctlight_fb0e3e88bd402a65e4afd910cf676531`; visible Moment and completed `go-autonomy:` action were both present. |
| 7. Proactive Owner contact | PASS | The same public projection check passed using `fluctlight_60fc292f8a3b95cd8dad33196f5544f3`; completed `go-autonomy:` action and persisted assistant message were present. |

Temporal management was also rechecked after the final runtime-ID change:
status and history for `go:schedule:fluctlight_8f368e697bd4154b2697432e84b01ee5`
returned the running execution, run ID, task queue and real event types.

The only failed continuation acceptance is Case 2's fresh ComfyUI generation,
which is an explicit external model configuration mismatch. It remains a
blocking acceptance item until the operator updates that Provider workflow or
the real endpoint exposes the configured transformer; no Go fallback is
allowed by the Provider contract.

## Provider configuration fixed (2026-08-31)

The operator corrected the persisted ComfyUI workflow transformer to the
available `moodyCutieMixKrea2_v40_mixed_4_8.safetensors` model. A new first
attempt through the public Go BFF then completed successfully:

- media intent `media_intent_8b6705169c4958e76a99017ca6485318` reached
  `completed`;
- the real ComfyUI job `c3b37c7d-ab3a-4ebb-9e9f-b2be5b67bf7d` completed;
- asset `asset_media_intent_8b6705169c4958e76a99017ca6485318` is `ready`,
  `image/png`, `224591` bytes, version `v1`;
- PostgreSQL contains exactly one `media_reference` for that asset;
- public BFF full read returned `200` with `Content-Length: 224591`;
- public BFF `Range: bytes=0-4` returned `206` with five bytes and
  `Content-Range: bytes 0-4/224591`.

This clears the external Case 2 blocker. Cases 1 and 3–7 from the preceding
continuation sweep remain valid; the code migration still requires the
platform/Worker capability gaps described in the task plan before the Trellis
task can be archived.

## Final post-fix rerun (same rebuilt Compose project)

The first post-rebuild Case 1 request (`go-final-case1-1788106207`) timed out
after the pre-existing lifecycle backlog saturated the real Provider. That
failure is retained. Lifecycle Provider activities were then bounded to two
slots, schedule intents were prioritized, and the Worker was rebuilt with the
Temporal `platform-v1` deployment version. No database row was manually changed
to hide the failure.

The following fresh requests were executed through host-published Go BFF
`http://127.0.0.1:13000`, with the authenticated cookie jar in
`/private/tmp/go-full-cookie.txt`; each request used a new idempotency/turn or
activation request ID:

| Case | Final evidence |
| --- | --- |
| 1. 影者 text chat | `go-final-case1-retry-1788106698`: public NDJSON `message → token → completed`, sequences `0,1,2`; assistant `message_8e9a067cc8219b80231e1dda64134855` persisted. |
| 2. 影者 image request | `go-final-case2-1788106775`: `media_intent_d8037ae3240a424cb84d210cb9f6e3f4`, Provider job `f7156081-8842-4ddf-b26d-d8cbd29b882f`, ready `asset_media_intent_d8037ae3240a424cb84d210cb9f6e3f4` (`195045` bytes, `image/png`); BFF full GET `200` and `bytes=0-4` `206` with `Content-Range: bytes 0-4/195045`. |
| 3. Blank Fluctlight | `fluctlight_01238cb12b8051d9717472d9718248f7`, created by `POST /api/fluctlights`, returned active with default identity. |
| 4. Described Fluctlight | Real Provider analysis returned typed foundation collections after the flat-string normalization fix; activation `fluctlight_fcaacf476173c11a2897715acabd61d6` returned active populated identity/personality/policy/life profile/provenance. The originally generated schedule for this fixture failed under the old 30s activity timeout and remains recorded as a failed attempt. |
| 5. Detail and schedule | Final fixture `fluctlight_3ae200f0fc4c683d07df00333ed49ff1` returned complete detail with 5 goals, 4 intentions and accepted `schedule_6d4ccff0c8da251a19235dcd729560bd` containing 12 contiguous full-day items in `Asia/Shanghai`. |
| 6. Moment publication | Final fixture `fluctlight_fb0e3e88bd402a65e4afd910cf676531` (policy explicitly disallowed proactive DMs and included a publish-dynamic intention) produced Go-owned action `autonomy_439a55ca843c4aae28c58ccdceec4a8e`, `action_type=moment`, `status=completed`, workflow `go-autonomy:fluctlight_fb0e3e88bd402a65e4afd910cf676531:2026-08-30`; public BFF returned visible `moment_30955e00521d3a6ef9899b29d186c0fb`. |
| 7. Proactive Owner contact | Final fixture `fluctlight_60fc292f8a3b95cd8dad33196f5544f3` (policy explicitly required a daily Owner plan) produced Go-owned action `autonomy_06d67d02f028cbcfa7cfa1e9552c9d81`, `action_type=proactive_message`, `status=completed`, workflow `go-autonomy:fluctlight_60fc292f8a3b95cd8dad33196f5544f3:2026-08-30`; public conversation `conversation_41301b87c54174528a8b8504b63b71e6` contained assistant `message_facc50a0fc20aac39d43765e3c017c62`. |

The final Worker image logs show `BuildID platform-v1` on all three canonical
queues. The final cutover service terminated the retired `DailyLifeReview` and
`CurrentDaySchedule` executions that had no legacy poller; repeated cutover
dry-runs found no remaining non-Go running execution. A diagnostic
`CurrentDayScheduleWorkflow` with a missing Fluctlight was consumed and failed
with the expected `resource not found` activity result, proving the Worker can
advance a workflow task rather than only reporting a healthy process.

After aligning schedule planning with the contract's `cognitive_assessment`
Provider role, a final blank fixture `fluctlight_b074e37a6fecf326e429cdf4bac2cac2`
was created on the rebuilt image. Its lifecycle workflow accepted
`schedule_7d6a8f6e6ecf070bfdd76876c319c272` with 12 contiguous
`Asia/Shanghai` full-day items, confirming the role alignment did not regress
Case 5.

The final timer-enabled image was then exercised with blank fixture
`fluctlight_fc4ee74b7216bade8f4b9684bf6979be`. Its public detail returned 12
accepted `Asia/Shanghai` items, and Temporal history for
`go:schedule:fluctlight_fc4ee74b7216bade8f4b9684bf6979be` reached
`EVENT_TYPE_TIMER_STARTED` after the activity completed, proving the schedule
workflow remains durable instead of exiting after one pass.

After the final `pending/continue-as-new` timer fix, the complete Compose
project was rebuilt again from this worktree. A post-rebuild public sweep passed:

- Case 1 new turn `go-final-post-rebuild-case1-1788137308` returned
  `message → token → completed` with sequence `0,1,2`.
- Case 2 asset `asset_media_intent_d8037ae3240a424cb84d210cb9f6e3f4` still
  returned `200 image/png` with `195045` bytes and `206 bytes=0-4` with the
  matching `Content-Range`.
- Cases 3–7 re-read the final blank, described/detail-schedule, Moment and
  proactive fixtures through the host-published BFF; all assertions passed.
