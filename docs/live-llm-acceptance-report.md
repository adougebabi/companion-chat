# Real LLM Acceptance Recheck

Date: 2026-09-17

This report records the first real OpenAI-compatible Provider run for the
persona and conversation changes. The deterministic Go tests remain useful for
state, CAS, capability, and recovery invariants, but they are not evidence that
a model emits the expected structure. The commands below used the local
`mlx-serve` endpoint at `127.0.0.1:11234` with model
`huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX`.

The explicit entry point is
`infra/acceptance/run-go-live-provider-smoke.sh`. It requires the Provider URL
and model, probes `/models`, and fails when the live environment is absent. The
default Go suite still does not make network calls or silently turn a skipped
live test into a positive acceptance result.

The first live run exposed failures that Fake Provider tests could not see:

- Real initialization output used `content` for goal/intention text,
  `target` for relationship targets, and `associated_profile` for profile
  scope. The production normalizer rejected the foundation or silently lost
  scope. Canonical alias mapping and regression coverage were added in
  `apps/core-go/internal/core/app.go` and
  `initialization_contract_test.go`.
- The normalized Prompt exposed a persistent rule as `switch:safety`, while the
  validator accepted only the raw declaration id `safety`. Real models echoed
  the canonical id and were rejected. The validator now accepts both forms and
  has a direct regression test.
- The local model emitted structured/native tool calls without provider call
  IDs. The strict fixture normalizer still rejects missing IDs, but the real
  Provider boundary now derives a stable `call_derived_<digest>` ID from the
  provider request, sequence, capability name, and canonical arguments. This
  keeps retries idempotent without accepting model text as an identity.
- The model sometimes returned an unregistered `personality.switch` call or
  unrelated state capabilities for a switch-only message. The Provider protocol
  now states that persistent switching is represented by `personality_decision`
  and that no `personality.switch` capability exists.
- Thinking-enabled structured calls could consume the output reserve in
  `reasoning_content` and leave the control JSON empty. Interactive structured
  cognition, WakeUp, takeover reply, native cognition, and daily review now
  omit `enable_thinking`; the structured decision stays in the normal content
  channel. This matches the existing Judge behavior and the local model's real
  protocol.

The following real Provider checks passed after those fixes:

| Check | Result | Evidence |
|---|---|---|
| Dense single initialization | pass | real model, production initializer |
| Dense multi initialization | pass | real model, production initializer |
| Complex two-profile initialization | pass | real model, alias normalization and profile validation |
| Image-generation intent recognition | pass | real model, native/tool normalization |
| Role and runtime-fact organization | pass | real model, assembled Prompt |
| Persistent switch decision | pass | real model, declared `spark → twilight` rule and valid context reference |
| Full `HandleTurn` with disposable PostgreSQL | pass | real `ProviderClient`, freeze/settlement, assistant message, durable active profile |

The full live `HandleTurn` test is
`TestLiveHandleTurnUsesRealProviderForPersonalityDecision`. It seeded a
disposable PostgreSQL database, configured the real endpoint through
`model_roles`, and verified that one user turn made a recognition call followed
by a post-switch Main call. The final database state had
`active_profile_id=twilight` and exactly one assistant message. The live model
also requested a real `memory.recall` continuation, so the physical Provider
request count was three; the test accepts the bounded query continuation rather
than assuming that every switch turn is exactly two HTTP requests.

The same-turn behavior is also covered without a model in
`TestPersistentSwitchReDecidesWithinTheSameTurn`: the first response is a
tool-free switch assessment, the second response is assembled with the target
Working Persona, and only the second candidate is sent. This Fake Provider test
locks the state machine; the live PostgreSQL test proves the real Provider path
reaches the same durable boundary.

The acceptance is still bounded. Active Memory and query-continuation live
smokes can take several minutes on a local reasoning model and are opt-in through
`FLUCTLIGHT_LIVE_PROVIDER_TEST_REGEX`; one broad run hit the Go ten-minute test
deadline while waiting for Active Memory. Real Judge takeover semantics,
ComfyUI/Media Worker execution, real image delivery, token billing/cache
metrics, and deterministic wall-clock persona evaluation remain separate
acceptance items. The deterministic time-window evaluator is still not
implemented, so an evening clock change cannot be claimed from this report.
