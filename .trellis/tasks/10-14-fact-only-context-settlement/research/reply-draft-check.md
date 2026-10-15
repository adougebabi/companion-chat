# F20 independent Trellis check — 2026-10-14

Scope: baseline HEAD 42a2d7c, F20 only; F19 and other task edits preserved. No production file was changed by this reviewer. Reviewer fixes are confined to `apps/core-go/internal/core/reply_draft_recovery_test.go`.

## Findings (fixed)

- Required-context fixture failure: actual first PG run panicked at `factOnlyReplyCapability.Prepare`, fact_only_reply_capability.go:56. Three direct Prepare fixture sites passed an empty CapabilityContext although Definition requires SlotCurrentLife and production resolver supplies it. Supplied `CurrentLifeContext` to direct fixtures, rather than weakening the production requirement. First FAIL/panic is retained in the record; subsequent actual PG/race runs are GREEN.
- Warning coverage: added `assertDraftRecoveryWarning` (reply_draft_recovery_test.go:81), validating real reply OutputSchema and warning_code/disposition in completed publication, published replay and accepted media results. Changed the original raw-hint assertion from whole-JSON equality to substring containment.
- Media/diagnostics coverage: changed initial media admission to real Prepare→ExecuteDirectTx, added durable canonical waiting_media/bound state recovery with actual accepted waiting result, and checked persisted `reply.draft.reconciled` payload against raw old/foreign pointers/body/assets/canonical ID. The controlled waiting state is deliberately seeded; it does not claim a real image generator ran.
- Organized test import groups.

## Behavior verified

- `factOnlyReplyService.prepare`, fact_only_reply_capability.go:177–201 derives canonical draft from existing privateReplyLogicalKey before hint reconciliation, then queries only canonical ID/logical key/current instance/conversation/authorization actor. No query uses the hint. Existing logical-key algorithm remains private_reply_run.go:83.
- `reconcileReplyDraftHint`, fact_only_reply_capability.go:524 compares without resolution and emits only safe counts/code/disposition/reason. Warning is frozen at Prepare:57 and restored to successful result at ExecuteDirectTx:65; private output schema declares it at :35.
- Cached/published recovery remains before assignment/generation; actual PG test creates a foreign draft containing secret body/asset/config, observes one Writer and one assistant message, then replays with unavailable Writer assignment, leaving foreign row unchanged.
- Real `PublishConversationReplyTx` source constraint is exercised: invocation correlation is actual accepted `turn:<TurnID>`, not tool-operation fallback. Closed source test marks the current inbox failed and real ExecuteDirectTx refuses publication, with zero assistant messages. Source fence code is tool_publication.go:279 (`pending`/`claimed`, or authorized waiting_reply).
- Optional unknown draft alias proceeds only through `decodePrivateReplyArguments`, provider_ref_codec.go:193; actual ADK private invoker branch is adk_conversation_runtime.go:234. Moments retains decodeReplyArguments. Pure negative tests retain unknown action/target alias errors, plus PG canonical media rejects foreign action resource with ErrUnauthorized.
- needs_action→waiting_media→ready replay uses same canonical draft and actual result states, without extra Writer calls on waiting/ready replay. Result-aware failure adjustment sees only canonical media failure.
- Provider, cancellation, closed pool failures remain real errors; no successful message is manufactured. Cancellation before DB is tested without an App. Existing media continuation source/cancellation regression also actually ran against PG.

## Findings (not fixed)

Broader related-expression selector found `TestPrivateChatCommittedStyleRepairAndMalformedSettlement` FAIL at reply_expression_integration_test.go:94 (Qwen read unrepaired candidate), :87 (settlement repair lost actual final text), and :126 (committed narrative differs from repaired expected text). This existing test sends `{}` (no draft hint); F20 reconciliation consequently returns nil and optional-draft decoder follows equivalent argument handling. The failure belongs to private expression/style behavior, outside reviewer ownership and the explicitly preserved v5 boundary. No baseline PG comparison was performed, so this report does not claim proof it already failed at HEAD. Recommend main thread investigate separately, and do not call the combined broad selector green. No other concrete unfixed F20 issue found.

Spec sync belongs to main thread: document optional private draft hint reconciliation, canonical authority, safe warning and unchanged required-resource/Moments authorization. Reviewer did not edit specs/task status/commit.

## Verification

All PG runs used only `postgres://reply_test@127.0.0.1:57485/postgres?sslmode=disable`, after checking `/private/tmp/codex-reply-draft-pg-ready`. Fixtures create/drop their unique isolated databases through isolatedCoreTestRepository. No unknown DSN, shared database, real model, ComfyUI or Temporal was used; main PG was neither stopped nor deleted.

1. Pure selector with empty database URL: 8 top-level PASS, 6 PG SKIP. This is not persistence acceptance.
2. First `TestReplyDraftRecovery` PG run: FAIL/panic from missing fixture Life context; fixed as above.
3. Corrected `TestReplyDraftRecovery` PG run: all 6 PG top-level PASS plus cancellation-before-DB PASS, zero SKIP. `/private/tmp/reply-draft-check-pg.log`.
4. Expanded final PG/race selector after extra assertions: 29 top-level PASS, 1 existing style test FAIL, zero SKIP. `/private/tmp/reply-draft-check-final.log`.
5. Scoped final PG/race selector: 23 top-level PASS, zero FAIL, zero SKIP (includes all 6 F20 PG cases and actual F17/media continuation regressions). `/private/tmp/reply-draft-check-scoped.log`.

Actual scoped command (login=false, local-network escalation authorized):

```sh
FLUCTLIGHT_LIVE_PROVIDER_TEST=0 GO_CORE_TEST_DATABASE_URL='postgres://reply_test@127.0.0.1:57485/postgres?sslmode=disable' GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test -race ./internal/core -run 'Test(ReplyDraft|ReplyEvidence|ReplyMedia|ProviderContextRefCodec|FactOnly.*(Registry|Definition))' -count=1 -v
```

The broad command adds `|PrivateChat.*(Style|Repair|Contract)` to the selector; its FAIL is recorded above.

- Lint: PASS — `go vet ./internal/core`, owning changed-file `gofmt -l`, `git diff --check`.
- TypeCheck/build: PASS — `GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go build ./...`.
- Tests: F20 scoped PASS with real disposable PG/race; broader related-expression suite FAIL as recorded.
- NOT_RUN: frontend gates/full Go aggregate (main thread owns final gate), real Provider/image/Temporal, live incident replay. Missing original incident arguments remain unavailable.
