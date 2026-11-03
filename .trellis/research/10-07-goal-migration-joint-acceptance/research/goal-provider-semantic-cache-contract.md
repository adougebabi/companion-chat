# 2026-10-08 Goal Provider semantic boundary, encoding and cache prefix

Latest user requirements supplement the active PRD:
1. goal_evaluation must obey semantic Provider spec: no raw storage IDs, revisions, criteria versions, profile/conversation/FK/audit/transport bookkeeping in model input or output schema. Minimal typed short refs are allowed solely to select frozen goals/criteria/objects/sources/actors. Core resolves them and injects its own IDs/versions; unknown/wrong-kind/foreign/duplicate refs fail closed. Preserve full durable snapshot and CAS/evidence/profile checks.
2. Evaluate JSON versus repository's existing TOON/YAML hybrid on the SAME semantic data, preserve full proof/criteria, no budget increases. Format before assembly/admission so estimates/diagnostics match real HTTP wire. Structured output remains closed JSON; do not accept legacy raw-ID/version model output as a fallback. Actual current assembled goal task bypasses general formatting and sends JSON on wire.
3. Within each Agent, maximize repeatable prefix: fixed protocol/persona first, stable task definitions and slow runtime facts before volatile evidence/time/current state. Canonical field/table/schema ordering; append-oriented chronological evidence where scope/budget allow. Never move dynamic user facts into system policy, alter actual message chronology, truncate protocol pairs, or cache model decisions just because prefixes match. Measure identical prefix bytes and estimated tokens; real model cache hit/latency requires local Provider evidence.
4. Automatic Goal assessments over the same real evidence/criteria/relevant constraints should not call the model repeatedly. Persist success memo/signature across restarts. Own evaluation IDs/revisions/judgments/clock-second churn are not new evidence. New sources, revocation, changed criteria/constraints, due review window, and explicit Owner reassess remain eligible. Failed/truncated/partial model runs never count as assessed. Preserve finite unoffered-source remainder and >6 Goal batches; never skip new unoffered evidence or create skip/remainder loops.

## Boundaries / ownership

- Goal wire implementer: goal_evaluation_task.go, new typed binding/projection/adapter helpers, source-input admission/render budget changes, own unit tests and Goal fake fixtures. No runtime persistence/queue changes, no shared composer edits.
- Prefix implementer: shared PromptAssembly/PromptComposer rendering/order and cache tests; own shared stable-task-context hook and assembler field. No Goal task or Goal runtime/schema changes.
- Goal memo implementer: goal_evaluation_runtime.go, goal_sources.go queue force/coalesce semantics if needed, memo helpers/tests. No wire DTO/composer edits. Derive semantic authority stamps from durable state, not ephemeral provider aliases. Coordinate pending-source/no-progress cases through existing snapshots without weakening source consumption.
- Main: design decisions, specs/report/commit and final integration facilities/checks. User environment remains read-only, no live model load or deployment.

## Shared hook agreement

Prefix implementer provides `withProviderStableTaskContext(ctx context.Context, value map[string]any) context.Context` for Goal wire implementer to pass stable semantic selected Goal/criterion definitions. Shared assembler reads it, adds required `PromptAssemblyInput.StableTaskContext map[string]any`, renders it before volatile runtime content as user-level task facts. Current input stays final user message. Stable data is never optional/pruned or system policy; budgeting includes it. Wire implementer may use this hook after it exists. Goal-only current input can contain current evaluations/objects and full actual evidence while definitions are provided once through stable context. Evaluate actual MIXED assembled wire rather than merely a stand-alone serialization. Keep projection full and immutable.

## Tests / evidence

- capture actual Provider payload and schema: no seeded raw IDs/versions at ANY nested input or schema path; minimal refs round-trip Core IDs/versions; unknown/wrong-kind/profile/stale refs rejected with zero business commit.
- criteria/optional policy, Stage/Commitment/dependency, reviews/relationship actor binding, full message/fact/domain query payload, true/false/zero/newlines/commas/multilingual values preserved.
- JSON vs TOON/hybrid fixture sizes and conservative estimates; estimator is not a real model tokenizer. No format-only semantic-loss claims.
- same Agent/persona/schema with changed current time/evidence retains byte-identical stable prefix; slow changes invalidate only appropriate later prefix. Actual role boundaries/native tool pairing unaffected.
- same evidence repeat and restart => zero extra Provider call; explicit reassess/new proof/withdrawal/changed criterion/due review => real assessment. Failed assessments and finite remainder not swallowed.
- disposable real PG/Redis where needed, full affected Go race/vet/build and owning Web checks if any. SKIP not PASS; don't call user production Provider or write live state.
