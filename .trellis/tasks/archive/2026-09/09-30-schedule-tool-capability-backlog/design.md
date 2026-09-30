# Technical design

## Boundaries

- Go Core remains the only schedule, capability request, and conversation writer. Agent tools are registered through the existing Capability Registry; Browser API remains the public boundary.
- The accepted schedule version and its linked Intention/action plan remain authoritative. Model proposals and Tool arguments are untrusted intent, never direct row mutations.
- The final assistant Content and native ToolCalls keep their distinct authorities. A final-output repair path must not re-execute previously committed ToolCalls.

## Schedule flow

1. Add a read-only schedule inspection capability for the current accepted local day. Return revision, timezone, status, bounded items with stable selection IDs, time, content, and linked-activity status; support explicit detail/pagination if the item list exceeds the result budget. An absent schedule returns `schedule_pending` explicitly.
2. Add a targeted schedule mutation capability with an operation (`move`, `revise`, `cancel`), item selector, requested changes, and reason. Core loads and locks the accepted version, enforces selection/revision and current/future policy, then constructs a new immutable version through the existing accept/replan path. Ordinary descriptive items may be edited; linked executable items keep their Intention/action plan and timed trigger consistent, or are cancelled through the existing linked cancellation path. Reject in-progress/deferred linked activity as the life-world contract requires.
3. Keep `intention.schedule` for creating future executable activities and `schedule.replan` for whole-day semantic replans. Before accepting a planner result, distinguish an invented/altered link from a corrupt current link. Constrain planner-visible links to actual current item IDs; never graft an unknown `intention_id` to an item. A bounded replan correction may reuse the same frozen input; persistent invalid output yields a typed error and preserves the old version.
4. Inspection is on demand. Prompt/catalog text tells the Agent when to inspect before editing, without embedding the whole schedule in every turn.

## Missing capability flow

- Preserve explicit `capability.request`. Update model operation rules to call it when a desired capability is unavailable, including the desired action and known evidence.
- On a native unknown ToolCall, the invoker may create an idempotent request from the authenticated turn identity and tool name before returning a typed non-execution result. The request must not claim that the unavailable action ran. The generated record uses bounded, non-sensitive metadata; rich desired contracts remain the explicit `capability.request` path.
- Keep the existing Owner-only list/review API. Load the pool from every governance entry and expose loading, true empty, and error states. Show the desired contract and source needed for implementation.

## Final output and reference flow

- First preserve raw Provider diagnostics and distinguish malformed Content, schema invalid, unknown alias, and final dynamic ref failure in the durable run classification. Propagate actual finish reason, especially token-limit truncation.
- For a malformed final object or omitted structurally optional empty evidence list, use a bounded, side-effect-free correction at the final-output boundary. The correction consumes the existing transcript and strict schema; it cannot call Tools or replay Tool effects. If correction still fails, return a specific error with sanitized diagnostic context.
- For dynamic refs, only exact tokens registered in the current run may survive. If a decision influence is invalid, a correction request may ask the model to select an exposed ref or remove that optional influence. Core validates the corrected result against the same frozen projection. Unknown aliases and raw IDs never become evidence through string rewriting or guessing.
- Align the `appraisal.evidence_refs` schema/normalizer contract explicitly: either accept a missing field as an empty list only where its absence is semantically equivalent, or require the model to emit `[]` and use bounded correction. Test both conversation and native cognition paths.
- Verify configured Provider support for required structured output/tool calling at preflight where feasible. An incompatible endpoint remains unavailable for that role; runtime failure remains explicit if a previously accepted provider changes behavior.

## Reply publication

- `conversation.reply` owns one assistant publication per `(conversation_id, turn_id)`. Under the conversation/turn serialization boundary, detect an already committed reply before INSERT. Exact idempotent replay returns the existing receipt; a distinct second reply returns a typed `reply_already_published` result. Keep the partial unique index as a final database invariant.
- The agent may continue after a duplicate ToolCall result, but no second message is inserted. Natural fallback must observe the committed reply and avoid publishing another. Cover serial, concurrent, and retry interleavings.

## Compatibility, rollback, and operational notes

- Schedule changes create new versions and keep old versions intact; no in-place migration of accepted items is planned. Legacy linked items with a missing action plan are diagnosed and rejected until safely repaired.
- New capabilities are additive to the catalog. Existing `intention.schedule`, `schedule.replan`, and explicit `capability.request` contracts remain available.
- Correction attempts are bounded and recorded in diagnostics. They do not loosen schema, accept unknown references, or silently switch models.
- Existing user edits in `.gitignore`, `README.md`, and `docs/` are outside this task and remain untouched.
