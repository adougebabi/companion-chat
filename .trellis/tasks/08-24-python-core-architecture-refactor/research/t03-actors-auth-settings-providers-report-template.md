# T03 Actors, Owner Auth, Settings And Providers Report

## Status

`NOT RUN | IMPLEMENTATION EVIDENCE | T12 ACCEPTANCE PENDING`

## Entry Evidence

Record T02 task-status source, T02 PASS report/check/commit/merge/archive evidence, exclusive-writer confirmation, and the final no-history dry-run result.

## Versions And Commands

Record exact Python/uv/Node/pnpm/FastAPI/Fastify/PostgreSQL versions, dependency additions for Argon2id/AEAD, migration revision, commit and every implementation-check command/result. T12 re-runs final acceptance and records the only PASS/FAIL decision.

## Deliverable Matrix

| Deliverable | Result | Evidence | Notes |
| --- | --- | --- | --- |
| Typed Actor / OwnerAccount / Session persistence | NOT RUN | | |
| One-time setup and Argon2id credential policy | NOT RUN | | |
| Opaque session lifecycle, authorization and recovery | NOT RUN | | |
| BFF cookie, CSRF/origin and BFF-only network exposure | NOT RUN | | |
| Runtime settings safe view, AEAD and audit | NOT RUN | | |
| Ciphertext-only persistence / redaction evidence | NOT RUN | | |
| ProviderEndpoint and six explicit Model Roles | NOT RUN | | |
| Capability preflight, health and no-fallback matrix | NOT RUN | | |
| Credential-free provenance and bounded diagnostics | NOT RUN | | |
| Linear migration/readiness and generated-client drift | NOT RUN | | |
| Architecture, real-PostgreSQL, failure and security tests | NOT RUN | | |

## Changed Paths And Boundaries

List owned files, generated artifacts, shared-file changes, migration revision, changed dependency/lock files, and proof frozen old code was not modified. State where session and decrypted secret plaintext are prevented from crossing into Node, DTOs, logs, prompts and diagnostics.

## T04 Handoff

Document the public resolved-HumanActor authorization/reference interface, no-longer-valid assumptions, generated contract artifacts and remaining operational risks. Do not include repositories, raw token material or secret values.

## Evidence Decision

- IMPLEMENTATION EVIDENCE: record owned paths, commands/results, risks and T12 coverage IDs; this does not allow a product PASS or cutover.
- T12 ACCEPTANCE PENDING: final contract/error matrices, real-PostgreSQL/failure/security/browser aggregate scenarios are not decided here.
- BLOCKED: retain exact evidence, keep T04+ blocked where required and return parent planning for any decision or shared-platform change.
