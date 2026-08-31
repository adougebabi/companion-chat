# Real Docker regression record

Date: 2026-08-30 (Asia/Shanghai)

Deployment under test: existing Compose project
`fluctlight-t12-browser-53392`, with PostgreSQL/Redis/Temporal/MinIO volumes
preserved. Core and Worker were rebuilt from `codex/go-core-runtime-stability`.
The Go Core profile was built and run separately as a read/transport slice; it
was stopped after its smoke checks. No mock Core, Provider or media service was
used. Requests were bounded to ten minutes.

## Results

### Case 1 — 影者 normal text chat

PASS. The first request through the public Go BFF returned HTTP 200, a user
message event, streamed visible token events and exactly one `completed`
terminal, with no error event. The first collection command returned before the
long provider completion was flushed; the persisted stream was then complete.
The same `turnId`/`idempotencyKey` replay returned the existing user message and
one completed assistant message without a duplicate user row.

### Case 2 — 影者 requests a photo

PASS on the first request after the compound-effect pre-validation fix. The
public BFF returned HTTP 200 with visible tokens and one completed terminal;
there was no `secondary_effect_must_produce_an_autonomous_side_effect` error.
The conversation contained four real `media_reference` messages, and all four
assets were readable through the BFF media proxy as non-empty `image/png`:

```text
157462, 205625, 174778, 192376 bytes
```

### Case 3 — blank Fluctlight creation

PASS. `POST /api/fluctlights` returned HTTP 200 and active ID
`fluctlight_90e9852e79d14f45bb77c6f6c4bcab70`.

### Case 4 — description-based creation

PASS. Real Provider analysis returned identity, personality,
behavioral_policy, life_profile, goals, intentions and provenance. Activation
created `fluctlight_d2a7bca6708fa80b44372d839cf03f1f` (`晨曦观察员`) after the
activation-transaction fix. The same transaction created the direct
conversation target and current-local-day daily-review intent.

### Case 5 — complete detail and schedule

PASS. The activated Fluctlight detail contained 13/13 non-empty identity
fields, 14/14 personality fields, 15/15 behavioral-policy fields, 7/7
life-profile fields, 3 goals and 3 intentions. A first local-day schedule was
submitted without `expectedRevision` and returned revision 1/accepted with two
contiguous items covering 00:00–24:00 Asia/Shanghai.

### Case 6 — Fluctlight publishes a Moment

PASS after dispatcher starvation fix. No manual Worker restart was performed
after the new activation/schedule. The action
`autonomy_action_e823eec136e2260414d63c5cbd4894aa` reached `completed`, and the
corresponding Moment `moment_autonomy_action_e823eec136e2260414d63c5cbd4894aa`
was `visible` with non-empty text. The first observation window ended while the
action was still frozen; after excluding already-dispatched intents in the
bounded query, the action completed normally.

### Case 7 — Fluctlight proactively contacts Owner

PASS using the existing real 影者 Fluctlight. The public BFF reported two
completed `proactive_message` autonomy actions, and the direct conversation
contained matching Fluctlight-authored assistant messages. This validates the
proactive delivery path without changing immutable daily-review facts or
forcing `no_op` decisions.

### Final no-restart activation check

After the dispatcher and optional-media fixes were deployed, a fresh description
activation created `fluctlight_5ea5d6a278bed1823275a1bed45506fe` and an accepted
revision-1 schedule. Without restarting Worker again, its daily review produced
`autonomy_action_608d23a17afa963b46a623ebf953ff27` as a completed
`proactive_message` and wrote one Fluctlight-authored assistant message to the
new direct conversation. The model selected proactive contact rather than a
Moment for this run; the earlier fresh activation `fluctlight_d2a7bca6708fa80b44372d839cf03f1f`
produced the visible Moment path. Together they cover both autonomous branches
on the final image and prove activation-time registration without a manual
Worker restart.

## Go Core smoke

PASS. `apps/core-go` passed race test, vet and build. Its Docker image started
on the existing Compose network, connected to the real PostgreSQL database,
returned `/health/ready` and service-key ping, authenticated the real session,
listed 8 existing Fluctlights, resolved a direct conversation and read its
history (5 messages, 2 participants). The optional `go-core` Compose profile
also rendered and built successfully.

## Code-level gates

```text
Python Core: 215 passed, 1 skipped (external provider socket fixture)
Go Core: go test -race ./..., go vet ./..., go build ./... — passed
Go BFF: go test -race ./..., go vet ./..., go build ./... — passed
Core OpenAPI: 63 paths / 68 operations match live FastAPI schema
```

The one skipped Python test is the repository's existing external-provider
socket fixture; it is not part of the Docker acceptance path.
