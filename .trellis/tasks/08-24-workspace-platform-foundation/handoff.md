# T02 Platform Foundation Evidence Handoff

Status: implementation and static evidence recorded; `acceptance_owner=T12`;
`acceptance=pending`; T02 itself remains `in_progress`.

## Available Evidence

- Workspace locks, API/Worker entrypoints, generated clients, platform
  contracts and focused Temporal management/history unit tests are present.
- The T02 brief's `test:platform` commands are provided by BFF and Web package
  scripts and execute the current transport/browser platform tests.
- Current focused Python command passes `49` platform/contract/architecture/
  Temporal tests. Generated Core OpenAPI is checked against the live FastAPI
  path/method/version surface.

## Remaining Completion Work

- Run and record real PostgreSQL empty-to-head migration/readiness, Redis and
  MinIO platform behavior, and clean Compose smoke.
- Run live Temporal management authorization, reset/restart/history-point and
  Worker recovery evidence; unit tests do not substitute for this.
- Decide whether the retained `temporal_gate` fixtures must be removed or
  formally classified as test-only before final cutover.
- Complete the T02 PASS report, then add completion/commit/merge metadata.

## T12 Coverage

T12 must rerun the platform recovery, schema, Compose and generated-client
matrix; this handoff does not grant final acceptance or cutover authority.
