# Persona Emergence Contract

## 1. Scope / Trigger

- Trigger: a persona interaction produces LLM-owned appraisal, memory-consolidation, self-model, or agency-intention candidates.
- The contract applies to both `llm_defined` and `blank_slate` personas. Initialization mode changes the starting blueprint/identity anchors, not the runtime candidate protocol.

## 2. Signatures

- Persona initialization: `{initializationMode: 'llm_defined'|'blank_slate', name?: string, role?: string, timezone?: string, language?: string, permissions, safetyBoundaries}`.
- Structured turn: `control.appraisals[]`, `control.memoryConsolidations[]`, `control.selfModelClaims[]`, `control.agencyIntentions[]`.
- Candidate persistence: `plan(command) -> immutable plan`, then `apply(plan, callerTransaction)`; repositories expose persona-scoped idempotency lookup, `list`, and revision-guarded CAS.
- Prompt read model: `selfModelRepository.listActive({personaId, limit}) -> active claims`; debug read model returns bounded `emergence` summaries.

## 3. Contracts

- Semantic meaning is produced by LLM structured output. The server validates schema, persona/source ownership, evidence references, idempotency, transactions, CAS, leases, redaction, and resource bounds.
- `companion.interaction-fact.v1` records the observed source; `companion.appraisal.v1` may carry allowlisted affect/drive candidates; `companion.memory-consolidation.v1` writes only an auditable candidate ledger; `companion.self-model.v1` writes an active claim projection; `companion.agency-intention.v1` writes a candidate lifecycle record.
- Candidate effects are applied with assistant message facts inside the caller-owned SQLite transaction. A failed provider/schema/ownership check preserves visible text but creates no semantic fallback.
- `blank_slate` stores empty or explicitly supplied identity anchors and an empty foundation value; it must reject non-empty foundation/personality input and must not use default personality text.
- Debug responses are persona-scoped and redacted. They may expose category, summary, evidence refs, status, revision, gate/decision summaries, and bounded errors, but never raw credentials, hidden reasoning, or complete prompts.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Candidate has an unknown schema, missing evidence, or malformed bounded field | Drop the optional candidate, retain visible text, and record a bounded diagnostic |
| Source message/fact is missing, foreign, or has the wrong role | Reject the candidate before persistence |
| Duplicate `(persona_id, idempotency_key)` | Replay the existing row without a second effect |
| CAS revision is stale | Return `updated: false`; never overwrite the newer revision |
| Caller transaction fails after assistant facts | Roll back candidate rows and assistant facts together |
| LLM provider unavailable or sidecar invalid | Do not infer from prose, keywords, regex, fixed thresholds, or default personality |
| Debug flag is disabled | Do not register emergence diagnostic routes or return debug-only fields |

## 5. Good / Base / Bad Cases

- Good: a structured appraisal references the current user message, the server validates ownership, and the affect reducer applies bounded deltas in the same transaction as the assistant reply.
- Good: a self-model claim is persisted with uncertainty and evidence, then only its bounded summary is included in the next prompt.
- Base: a text-only completion produces an ordinary reply and no emergence candidate rows.
- Bad: parsing a visible sentence such as “我喜欢茶” with a regex and writing a preference, or converting an agency intention directly into a proactive message without qualification/freeze/lease.

## 6. Tests Required

- Contract tests for all four candidate schemas, unknown fields, malformed sidecars, evidence requirements, and LLM failure without fallback.
- Repository tests for persona isolation, idempotency replay, CAS, list/read projection, migration reopen, and caller-transaction rollback.
- Chat tests asserting candidate effects commit only after assistant facts and preserve the existing `token`/`done` SSE contract.
- Initialization tests asserting `llm_defined` keeps the existing required name/role path while `blank_slate` accepts empty anchors and rejects foundation input.
- Debug tests asserting emergence summaries are bounded, persona-scoped, redacted, and absent when debug is disabled.

## 7. Wrong vs Correct

### Wrong

```js
const preference = /喜欢茶/.test(userText);
if (preference) memoryRepository.upsert({key: 'drink', value: 'tea'});
```

### Correct

```js
const turn = normalizeStructuredTurn(completion, {personaId, sourceMessageId});
const plan = memoryConsolidationFlow.plan({
  personaId,
  sourceMessageId,
  memoryConsolidations: turn.control.memoryConsolidations
});
commitBoundary({facts: assistantFacts, effects: [{payload: {plan, apply: () => memoryConsolidationFlow.apply(plan)}}]});
```
