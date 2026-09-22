# Reply / Moment / image Tool implementation

## Implemented boundary

`App.ExecuteTool` now dispatches target-owning capabilities through the generic
`DirectToolCapability` interface after the ordinary registry lookup, schema
validation, context resolution, preflight and Prepare phases. Prepare remains
outside the database transaction. There is no capability-name switch and no
Main, cognition inbox, autonomy Action, persona, or caller-owned transaction
requirement.

`ToolExecutionRequest` adds explicit `TargetKind` and `TargetRef` resource
binding. `ConversationID` remains the explicit conversation scope. Direct and
native ToolCall callers use the same `ExecuteTool` implementation; native
diagnostic Call ID / Provider request ID remain separate from the stable
business `OperationID`.

## Production publication service

`ToolPublicationService` is the shared short-transaction service for:

- assistant message publication with conversation participant ownership,
  deterministic message identity, exact replay, payload-conflict rejection,
  sequence allocation, and the existing conversation-summary intent hook;
- Moment publication with deterministic Moment identity, exact replay,
  payload-conflict rejection, and the existing `moment.published` outbox kind;
- explicit media target ownership for conversation, conversation message, and
  Moment targets;
- stable media-intent replay/conflict checks before delegating to the existing
  `createMediaIntentTargetTx` authority, which writes `media_intents`,
  `platform_workflow_intents`, and the existing `media.intent.created` outbox.

No new event kind, compatibility message, Moment, or alternative media intent
table was added.

## Capability behavior

- `conversation.reply` commits one assistant `conversation_messages` row and
  returns its real message ID. Same operation/same payload replays it even when
  the native ToolCall ID changes. Same operation/different text or target
  conflicts.
- `moment.publish` commits one visible participant Moment and one existing
  `moment.published` outbox row. Same operation/same payload replays; changed
  text conflicts.
- `media.image.generate` requires an explicit owned target, keeps the existing
  context-bound Prepare and life-context CAS, creates the existing durable
  media/workflow intent, and returns `accepted` with both the real
  `media_intent_id` and workflow `task_id`. It does not claim that an image
  asset is complete.

The existing caller-owned deferred implementations remain available during
the production-call-site migration. The existing media path still reports its
legacy settlement result to those callers; the standalone execution boundary
reports the asynchronous accepted state required by the new Tool contract.

## Old call sites still requiring migration

The following production callers still create output targets outside the Tool
and must be moved to `ToolPublicationService` / `ExecuteTool` by the owning
integration slice before old settlement code can be removed:

- `mutations.go`: assistant insertion in `completeTurnCognitionTx`, recovery
  settlement, `settleDeferredCapabilitiesTx`, and the existing
  `createMediaIntentTargetTx` method body;
- `workflow_ops.go`: proactive message publication, typed Moment publication,
  generic capability output target creation, and their repeated
  `moment.published` outbox calls;
- `autonomy.go`: `appendAssistantTxWithID` remains used by old callers;
- `cognition_growth.go` and `wakeup.go`: deferred capability settlement still
  binds wake-up/action targets externally.

This slice did not modify those files because they are owned by other
integration slices and changing them here would create duplicate publication
during the transition.

## Tests and evidence

`tool_publication_test.go` uses `isolatedCoreTestRepository`, so every test
creates, migrates, and drops its own PostgreSQL database. It covers:

- reply direct success, native-adapter replay with a different ToolCall ID,
  same-operation payload conflict, unowned target rejection, and independent
  message verification;
- Moment success, replay, conflict, independent Moment verification, and
  `moment.published` outbox verification;
- image accepted task/intent success, native-adapter replay, conflict,
  target rejection, dependency/preflight failure, and independent verification
  of media intent, workflow intent, outbox, and pending status.

Executed validation:

- `python3 /tmp/lac-run-tool-test.py publication-tools-direct 'TestExecuteTool(ConversationReply|MomentPublish|ImageGenerate)'`
  - exit 0;
  - six real PostgreSQL cases passed in 16.722 seconds;
  - every case created/migrated/dropped its own database through
    `isolatedCoreTestRepository`;
  - sanitized immutable output:
    `research/runs/publication-tools-direct.jsonl`;
  - run metadata including exit code, HEAD, working-tree digest and isolated
    Compose project:
    `research/runs/publication-tools-direct.meta.json`.
- `GOCACHE=/tmp/fluctlight-go-cache go test ./internal/core -run 'Test(ImageCapability|ImageIntent|CapabilityRenderer|MomentPublishCapability|ConversationCapabilityCatalog|ImageDeferredFailure|ToolOnlyAction|OptionalToolFailures)' -count=1`
  - exit 0.
- `git diff --check` over the owned implementation/test files
  - exit 0.

The full `go test ./internal/core -count=1` command compiled and ran but is
currently nonzero because concurrently edited, out-of-scope Agent/persona
tests fail (`persona_action_service.go` static switch guard and several ADK
request-identity assertions). The publication tests themselves pass both the
ordinary focused run and the real isolated PostgreSQL run. `go vet
./internal/core` is likewise currently blocked by an out-of-scope type mismatch
in `persona_action_service.go:167`. These are not counted as publication-slice
successes and remain for the owning integration slices.

## Shared result-status follow-up

The standalone media boundary returns `CapabilityResult.Status="accepted"` as
required. The shared `internal/capability.CapabilityResult.Validate` currently
recognizes only `completed|failed|rejected|deferred`; this slice was explicitly
forbidden from changing `internal/capability/capability.go`. The main
integration owner must add `accepted` to that shared contract before routing
the standalone result through code that calls the legacy validator. The direct
`ExecuteTool` path itself does not invoke that legacy validation branch.
