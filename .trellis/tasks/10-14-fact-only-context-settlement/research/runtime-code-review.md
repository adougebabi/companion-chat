# Runtime code review

Read-only review of the task PRD/design, Fact-only native arguments and evidence projection, request phase/policy/runtime, media continuation, publication suppression, and controlled transport/persistence tests. No tests, database, or network calls executed.

## P2 — Keep proactive suppression effective across old and new purpose identities

Location: `apps/core-go/internal/core/fact_only_reply_capability.go:101` (also `:110`, `:119`), with exact suppression lookup at `tool_publication.go:168`.

Trigger: publish an autonomy reply using the pre-v2 schema with topic `check-in` and purpose `care`; upgrade while still inside the proactive window, without a new inbound user message; then issue a new v2 autonomy reply on `check-in` whose generated text differs. The new request uses `independent_persona_reply` as its purpose, so the SQL predicate `purpose=$4` cannot find the previous `care` delivery. Exact-text suppression does not help because the Writer generated different text, and a second proactive reply is published. The reverse ordering also occurs when an old media draft resumes after a new v2 publication: the old draft retains its stored purpose at line 110 and misses the new stable-purpose delivery.

The code preserves the window, topic and inbound sequence but changes the suppression identity with no compatibility lookup. This conflicts with F09/design section 7's cross-version/proactive compatibility requirement. Preserve server-owned purpose and add an explicitly scoped compatibility lookup for prior Fact-only identities (without broadening unrelated legacy behavior), or another concrete existing-version-based bridge. Add a regression covering old delivery → new v2 reply and new delivery → old draft continuation, with different generated text and unchanged inbound sequence. Current `fact_only_context_view_test.go` verifies that incoming purpose is rejected and persisted scope uses the stable constant, but does not exercise delivery lookup across the identity change; the new settlement integration test is ordinary conversation mode and cannot catch it.
