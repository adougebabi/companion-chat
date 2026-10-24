# r3 recovery and no-progress boundaries

## 1. Scope / Trigger
Initialization final-response errors, native private reply media waits, recurring
schedule generation failures, duplicate conversation segments and browser recovery.

## 2. Signatures
- `structuredResponseTruncatedError`: role and optional usage metadata only.
- `initializationStructuredCause(error)`: closed typed/unwrap error mapping.
- `unchangedReplyMediaResult(trace)`: same canonical draft and media receipt state.
- `EnsureCurrentDaySchedule`: ready/inactive, stale-context pending, or real error.
- `ProcessConversationSegmentIntent`: owner logical lease then status/epoch reread.
- Browser `analyzeFluctlightCreation(input,{signal?})` adds transport cancellation;
  the body and HTTP contract are unchanged.

## 3. Contracts
Initialization stays one generation task. No application input/output token cap
or segmented quota is sent. Explicit length termination is not a successful final
JSON even when the object parses, and does not trigger the final DTO repair.
Ordinary malformed stop output retains its existing single tool-free repair.
Timeout/cancellation error identity takes precedence; partial persona text is
not included in the public typed failure.

A committed private reply still ends the native loop. An accepted waiting media
reply ends local observation into the existing durable continuation, with no
visible message invented. Two unchanged needs_action receipts for the same draft
stop with `reply_media_no_progress`; changed media state remains admissible.
The scoped fact-only reply's valid empty arguments stay valid. Do not infer
dropped reply prose from argument byte counts or reinterpret pseudo-tool text.

Real schedule generation errors reach the existing two-attempt Activity policy,
then fail visibly instead of nil-error pending/ContinueAsNew forever. Only
CAS/life-context staleness keeps the fresh-snapshot waiting path. Ready scheduler
histories still remain alive until local midnight; started alone is not failure.
Initial WakeUp release reads an accepted (or later superseded) Schedule record,
not a completed schedule Workflow; the stable scheduler may remain started.

Segment generation acquires the existing per-owner conversation_summary lease,
then reloads status and idle epoch before model I/O. Completed settlement replays
without inserting a second revision. No memory is generated merely to increase
a UI count. Only the exact missing media.comfyui configuration read maps to the
image-provider-not-configured sentinel; other missing business rows keep errors.

Connection drafts persist only allowlisted ordinary fields and original CAS
versions. Credentials and credential-bearing URLs stay out of storage. A saved
connection directory is never claimed to test an edited draft. Notification
grouping preserves every distinct notice/read command; message previews require
the exact authenticated conversation/message/author scope and remain transient.
Diary pending/running/no_content remain distinct from prose; selected date is
explicit and today stays closed to generation.
Analysis screen state is transient: leaving aborts its observer and invalidates
the analysis epoch; returning restores editable input. A persisted analysis step
without its live request also returns to input with a fresh request identity.
Disposed controllers and late results cannot overwrite the replacement draft.

## 4. Validation & Error Matrix
| Case | Outcome |
| --- | --- |
| Truncated final JSON | Typed initialization_response_truncated, no repair/source activation |
| Same draft media state repeats | Finite no-progress error; real draft/actions preserved |
| Accepted media still running | Existing continuation; no false publication |
| Unchanged schedule source but invalid model output | Activity error and finite retry |
| Schedule source changes during generation | Pending for fresh snapshot |
| Concurrent same segment | One physical generation; completed replay |
| Unused model role | Optional; a partially filled role requires all necessary fields |
| Unsaved connection test | Explain save-first; no request against a nonexistent endpoint ID |
| Diary no_content | Explicit date + no-material feedback, no fabricated body |
| Foreign/late notification source | Keep generic notice, discard unauthorized preview |

## 5. Good / Base / Bad Cases
Good: stop waiting for analysis, retain original description, and fence its late
response from a new attempt. Base: configure two roles while embedding is unused.
Bad: mark failed wardrobe or image stages ready, synthesize Memory, or deduplicate
two different replies merely because their prose matches.

## 6. Tests Required
r3_generation_recovery_test.go exercises real Generate/Stream HTTP responses,
usage/finish metadata, no-cap request bodies and fresh native tool IDs.
r3_persistence_recovery_test.go proves real PG schedule failure without an accepted
plan and concurrent segment single generation. r3_schedule_retry_test.go proves
the two-attempt terminal Activity path. Existing image-provider and segment/daily
memory SQL tests stay active. Frontend tests cover credential-safe CAS recovery,
exact notice identities/authorized preview, explicit diary status, and stopped
analysis/disposal/reload epoch fencing; browser-client tests prove AbortSignal
forwarding.

## 7. Wrong vs Correct
Wrong: catch every schedule error and return pending with nil error, then restart
five minutes later forever. Correct: retain typed real errors for finite retry;
reserve pending for an actual stale snapshot and never manufacture an accepted
plan to release initialization.
