# T06 No-History Handoff Dry Run

Date: 2026-08-25

The child consumes the T05 public cognition/diagnostics handoff and owns the
Conversation vertical slice. It has exclusive ownership of the paths listed
in `implement.md` for this serialized turn.

## Execution

1. Define Actor-aware Conversation, Participant, Message, read/delivery and
   typed turn contracts without importing transport or Provider internals.
2. Add the `0005` migration and services for ordered, idempotent message
   persistence and cursor history.
3. Add Core JSON/NDJSON routes and regenerate the Core client artifact.
4. Add BFF TypeBox routes and an incremental browser stream translator that
   validates sequence/terminal events and forwards aborts.
5. Regenerate the browser client and implement the `apps/web` chat surface.
6. Run focused Python/TypeScript/Vue checks; hand final aggregate and browser
   acceptance back to T12.

## Exclusions / Risks

T06 does not implement Memory, Relationship, Life World, Media storage,
Diagnostics UI, backup, or legacy deletion. Attachment values are references
only. Real PostgreSQL, Core+BFF cancellation, CSRF, browser accessibility and
cross-module acceptance remain T12 scenarios.

Conclusion: the T05 handoff and repository evidence resolve all planning
questions necessary to start this child.
