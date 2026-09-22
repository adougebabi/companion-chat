# FormalAgent real-Provider E2E implementation and evidence

Date: 2026-09-22 (Asia/Shanghai)

## Implementation boundary

This slice adds an independent typed conversation cognition entry and the real-Provider matrix for all 16 registered `FormalAgentID` values. It does not modify the legacy settlement path, existing production callers, or existing tests.

- `apps/core-go/internal/core/cognition_agent.go`
  - Adds `RunConversationCognitionAgent`.
  - Accepts an authorized actor, Fluctlight, optional Conversation, stable Run ID, current input and streaming selection.
  - Builds the production `ContextProjection` itself; no chat message, cognition inbox/source fact, frozen action, Main state, or synthetic Agent state is required.
  - Uses the production `providerContextAuthorityRule`, `capabilityConversationPolicyInstruction`, conversation capability catalog, `conversation_turn_response` schema and `RunFormalAgent` Eino loop.
  - Returns the final typed DTO and native invocation/result trace. It does not publish an assistant message.
- `apps/core-go/internal/core/formal_agent_e2e_test.go`
  - Defines `TestFormalAgentE2E` with exactly 16 named rows, one for each formal Agent.
  - Calls the typed production `Run*Task`, `RunFormalAgent`, or conversation runtime boundary for each row. No scripted model/fake completion is used.
  - The conversation cognition row enables the production streaming entry and writes a random secret through production `ExecuteTool(memory_event)` before requiring a two-step `memory.recall` chain.
  - The media-quality and visual-vision rows send actual PNG bytes through the production multimodal request path. The PNG is a real dependency test asset; the tests do not fabricate a successful visual assessment.
- `apps/core-go/internal/core/formal_agent_e2e_helpers_test.go`
  - Creates one `isolatedCoreTestRepository` database per test row.
  - Installs production Provider assignments for every used model role and encrypts the optional API key in the isolated settings table.
  - Wraps the actual HTTP transport only to observe model requests; it never replaces or edits a response.

The current working-tree file digests after the recorded runs and the final alphanumeric-category fixture correction were:

| File | SHA-256 |
|---|---|
| `cognition_agent.go` | `e8310136b54529c7699b0da0adbade055192c90fe59908ffe3aae6f96c6070c6` |
| `formal_agent_e2e_test.go` | `5815b20ddb92f8692aea013cc548f6d2279e611b1b380d4157d44a661911f21d` |
| `formal_agent_e2e_helpers_test.go` | `41ec4755094712c71c47865662e28fc753862776962827f25bb1390d58128fc6` |

Repository HEAD was `32276a1c20ad68462b966c45c16ab6126230b297`; the tested state also contained the shared uncommitted task changes.

## Environment and evidence policy

The run loaded only the following names from the private per-task `test-env.json`: `FLUCTLIGHT_LIVE_PROVIDER_TEST`, `FLUCTLIGHT_LIVE_PROVIDER_URL`, `FLUCTLIGHT_LIVE_PROVIDER_MODEL`, `FLUCTLIGHT_LIVE_PROVIDER_API_KEY`, `FLUCTLIGHT_LIVE_PROVIDER_REQUEST_TIMEOUT_SECONDS`, `GO_CORE_TEST_DATABASE_URL`, and `GOCACHE`. Values and credentials were not printed or copied into evidence.

Each real test row creates and drops its own database through `isolatedCoreTestRepository`. All Provider requests use the existing OpenAI-compatible production client, formal Eino Runner and actual configured model. The spy retains decoded payloads in process for assertions only. Logs contain statuses and bounded errors, not keys, passwords, raw model payloads, random secrets, or complete private prompts.

## Recorded attempts

| Attempt | Command scope | Exit/result | Evidence |
|---|---|---|---|
| 01 | conversation cognition only | Interrupted before result because the initial `tee` target was wrong; never used as acceptance evidence | `research/runs/formal-agents-01-conversation-secret.log` |
| 02 | conversation cognition only | FAIL after 228.26s; the real Provider made at least three decisions and production `memory.recall` completed twice, but the final DTO did not contain the random database secret | `research/runs/formal-agents-02-conversation-secret.log` |
| 03 | conversation cognition diagnostic | FAIL; Provider HTTP 400, 6308-token prompt required about 1642MB and only about 1019MB was available | `research/runs/formal-agents-03-conversation-secret-diagnostic.log` |
| 04 | reduced-memory-layout conversation cognition | FAIL; Provider HTTP 400, 6182-token prompt required about 1632MB and only about 150MB was available | `research/runs/formal-agents-04-conversation-secret.log` |
| 05 | takeover Judge only | FAIL; even the 287-token request required about 948MB and only about 308MB was available | `research/runs/formal-agents-05-takeover-judge.log` |
| 06 | complete 16-row suite | Overall FAIL; 1 PASS, 15 FAIL, no SKIP | `research/runs/formal-agents-06-full-suite.log` |
| controlled 01 | existing controlled native Tool loop regression | PASS in 1.34s; scripted Provider only, so it does not replace a live row | `research/runs/formal-agents-controlled-01-memory-loop.log` |

The random-memory fixture was tightened after attempt 02 so its second-stage random category is one simple alphanumeric token. A final real run of that revision could not start because the shared Provider lacked enough free GPU memory. Attempt 02 remains useful real-loop evidence, but it is not a passing acceptance result.

## Complete 16-Agent matrix from attempt 06

| Formal Agent | Production entry exercised | Real result | Evidence / exact blocker |
|---|---|---|---|
| `conversation_cognition` | `RunConversationCognitionAgent` → streaming `RunFormalAgent` | FAIL | 6186-token request needed ~1633MB; ~953MB available. Earlier attempt 02 completed two real recalls and three model decisions but final DTO omitted the secret. |
| `wake_up` | formal Wake-up projection/policy assembly → `RunFormalAgent` | FAIL | 4088-token request needed ~1469MB; ~869MB available. |
| `takeover_judge` | `ConversationRuntime.RunTakeoverJudge` | FAIL | 287-token request needed ~948MB; ~757MB available. |
| `takeover_reply` | production takeover context rule + projection/policy assembly → `RunFormalAgent` | FAIL | 5396-token request needed ~1571MB; ~766MB available. |
| `initialization` | `RunInitializationTask` | FAIL | 1903-token request needed ~1298MB; ~1251MB available. |
| `media_prompt` | `RunMediaPromptTask` | FAIL | Real model call returned, then production boundary reported `adk_structured_response_invalid`. Static follow-up found that the ADK structured-candidate rejection in `provider.go` is applied when a text Agent emits JSON-looking text, even though this Agent declares a text output contract. This production file was outside this slice and was not changed. |
| `media_quality` | `RunMediaQualityTask` with actual PNG bytes | **PASS** | Completed in 9.37s through the real Provider and returned a valid typed verdict. |
| `visual_identity_vision` | `RunVisualIdentityVisionTask` with actual PNG image content | FAIL | 1060-token request needed ~1232MB; ~1189MB available. |
| `visual_identity_patch` | `RunVisualIdentityPatchTask` with the real image's factual non-human observation | FAIL | 1074-token request needed ~1233MB; ~1077MB available. |
| `conversation_summary` | `RunConversationSummaryTask` | FAIL | 724-token request needed ~1206MB; ~582MB available. |
| `schedule_generation` | `RunScheduleGenerationTask` | FAIL | 953-token request needed ~1224MB; ~886MB available. |
| `native_cognition` | `RunNativeCognitionTask` | FAIL | 3975-token request needed ~1460MB; ~775MB available. |
| `daily_review` | `RunDailyReviewTask` | FAIL | 3985-token request needed ~1461MB; ~672MB available. |
| `persistent_switch` | `RunPersistentSwitchTask` | FAIL | 1517-token request needed ~1268MB; ~173MB available. |
| `reflection` | `RunReflectionProposalTask` | FAIL | 3744-token request needed ~1442MB; ~219MB available. |
| `schedule_replan` | `RunScheduleReplanTask` | FAIL | 1162-token request needed ~1240MB; ~154MB available. |

Attempt 06 ran all required rows and recorded no SKIP. Its non-zero result is intentional and must remain a failing overall acceptance result until all 16 rows pass in one final run.

## Random secret chain assertions

The conversation row encodes the complete required evidence chain:

1. It generates a per-run random category and random secret.
2. It commits the guide, secret and ranking-layout memories through production `ExecuteTool(memory_event)` and independently reads the secret memory row back from PostgreSQL.
3. The initial formal request must contain neither the random category nor secret.
4. The model must emit at least two native `memory.recall` calls. Each call must have a real native call ID and Provider request ID.
5. Every actual result must be `completed` and correlated to its invocation call ID.
6. At least three physical model requests must be observed.
7. Later model inputs must contain the actual guide/category and actual secret Tool results.
8. The final structured DTO must contain the database value.
9. The optional Conversation is omitted for this run, and PostgreSQL must contain zero newly published assistant messages.

Attempt 02 reached steps 1–6 with two completed recall calls and three physical model decisions, then failed step 8. Later attempts were rejected before the first decision by Provider GPU-capacity errors, so the final corrected category layout has not passed real validation.

## Local checks

The following local checks passed after the shared `handleTurn` production slice became available:

```text
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test ./internal/core -run '^$'
ok github.com/fluctlight/local-ai-companion/apps/core-go/internal/core [no tests to run]

GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go vet ./internal/core
exit 0
```

`git diff --check` also passed for the three files in this slice.

The existing controlled regression also passed with the isolated real PostgreSQL database:

```text
go -C apps/core-go test ./internal/core -run '^TestFormalConversationAgentRecallsRandomDatabaseSecretThroughRealToolBridge$' -count=1 -v
PASS (1.34s)
```

This regression proves the deterministic Eino native ToolCall/result bridge, database-backed memory retrieval, next-model-input feedback, request identity correlation and final DTO handling. Its Provider is an `httptest` script and is therefore recorded only as controlled regression evidence.

## User-run serial live checklist

Per the final resource instruction, no more live requests are issued by this implementation slice. After loading the same private environment variables, run exactly one row at a time and keep a new uniquely named log for every attempt:

```text
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/takeover_judge$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/conversation_summary$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/schedule_generation$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/visual_identity_vision$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/visual_identity_patch$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/schedule_replan$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/initialization$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/persistent_switch$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/reflection$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/native_cognition$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/daily_review$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/wake_up$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/takeover_reply$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/conversation_cognition$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/media_prompt$' -count=1 -v
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/media_quality$' -count=1 -v
```

The order starts with smaller prompts and leaves the largest/tool-loop rows until later. `media_prompt` should be rerun only after its text-output boundary is fixed; `media_quality` already has one real PASS but remains in the final serial checklist so the eventual accepted state is uniform.

## Remaining acceptance work

The implementation compiles, and every registered Agent has a concrete real-Provider row, but the real acceptance suite is not complete. The next checks must follow the serial checklist with exclusive or sufficient Provider GPU memory (the smallest failed request still required approximately 948MB) and must retain a new uniquely named log for each row. The `media_prompt` text-output/JSON-looking-content failure also needs a production boundary fix outside this slice. The random secret row must pass all nine assertions in the same final code state; attempt 02 cannot be counted as success.
