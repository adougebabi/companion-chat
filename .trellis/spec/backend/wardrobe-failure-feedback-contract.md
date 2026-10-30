# Wardrobe preparation failure feedback

## 1. Scope / trigger

Preparation failure must explain the actual failed boundary without converting
all database failures into model setup failures. Existing task/attempt/command,
committed inventory, recovery and diagnostic ownership remain unchanged.

## 2. Signatures

- `providerAssignmentLookupError(role, err)` wraps the actual assignment query.
- `providerRoleUnassignedError` identifies only that query's `pgx.ErrNoRows`.
- `providerRoleLookupReadError` retains other assignment-query read failures.
- `wardrobeContextReadFailure(stage, err)` wraps fixed pre-model read boundaries.
- `wardrobePreparationFailure(err) (code, reason)` supplies the existing persisted
  preparation `error_code` / `result.failure_reason` fields; no DTO shape change.

## 3. Contracts

Typed wrappers preserve `Unwrap`, `errors.Is/As`, role and the original bounded
diagnostic cause. Role lookup's prior diagnostic string is compatible. Context
stages are code-owned labels: resource, effective_profile, persona_detail,
life_snapshot, wardrobe_items, wardrobe_revision, wardrobe_plans. They never copy
raw SQL, arbitrary IDs or secret values into ordinary failure prose.

Use `wardrobe_provider_unconfigured` only for the exact missing role binding;
`wardrobe_provider_assignment_unavailable` means the configuration read failed;
`wardrobe_preparation_context_failed` names the actual pre-model read step.
Cancellation remains cancellation. Storage/read deadlines do not establish a
model-response timeout. True model timeout, truncated response, missing plan,
invalid schema and invalid JSON retain their owning classifications. Unknown
causes use neutral wardrobe guidance, without guessing model setup. Keep deferred
failure persistence, recovery identities and original committed effects unchanged.

## 4. Validation / error matrix

| Evidence | Product result |
| --- | --- |
| Role query's typed missing binding | Configure the actual required model purpose |
| Role query connection/deadline error | Configuration read unavailable; no model timeout claim |
| Typed current-persona/life/wardrobe read error | Actual read step failed before model invocation |
| Plain or historical bare SQL/no-rows error | Unknown preparation failure; no inferred binding absence |
| Model timeout/schema/plan/JSON failure | Preserve the specific owning code |

## 5. Good / base / bad

Good: a life snapshot deadline stays a context-read failure with the original
cause retained. Base: an exact unassigned generic binding offers its configuration
entry. Bad: any `no rows` becomes generic_llm missing, or a frontend raw-string
heuristic replaces a stable persisted reason.

## 6. Required tests

`r20_wardrobe_failure_test.go` covers exact typed assignment vs ordinary DB errors,
fixed context stages, `Is/As`, deadlines, historical bare errors and existing
terminal classifications. Run focused race tests and retain real PG/model gaps;
skipped database cases are not persistence acceptance. Frontend consumers use
`r20-failure-presentation.test.mjs` to keep stable codes and task identity first.

## 7. Wrong versus correct

Wrong: match `sql: no rows in result set` globally and instruct model setup.
Correct: classify at the actual typed read boundary, retain the original cause,
and expose only the matching actionable product reason.
