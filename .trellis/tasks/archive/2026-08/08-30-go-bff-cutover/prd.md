# Complete Go BFF cutover

## Goal

Remove the legacy Node BFF, make Go BFF the only public gateway, update build/deploy/generation references, and run real Docker browser regression cases 1-7.

## Requirements

- Remove the legacy Node BFF implementation and its Dockerfile, tests,
  package importer, runtime scripts, and direct generation dependency.
- Keep Go BFF as the only public `/auth/*`, `/api/*`, `/health/*`, and
  `OPTIONS` boundary while preserving the checked Browser OpenAPI contract,
  cookies, CSRF, CORS, Core identity headers, DTO mappings, NDJSON, and media
  Range behavior.
- Move Browser OpenAPI generation to an active package that remains after the
  Node BFF deletion; generated artifacts must remain unchanged after
  regeneration.
- Update root scripts, workspace membership, lockfile, Compose, CI, README,
  specs, and acceptance scripts so no active runtime or build path depends on
  `apps/bff` or `@fluctlight/bff`.
- Run real HTTP regression against the user's running Docker deployment. Use
  the Web URL only to discover the runtime BFF origin, then exercise the
  authenticated Go BFF API with the supplied owner password and no mocked Core
  or Provider responses.
- Cover the seven requested scenarios: normal text chat with 影者, image
  request, blank Fluctlight creation, description-based creation, complete
  detail including schedule, Fluctlight moment publication, and proactive
  contact/autonomous action discovery. Long-running calls may take up to ten
  minutes and must not be given a shorter client timeout.
- Do not merge or cherry-pick the unfinished `master` fixes into this branch.

## Acceptance Criteria

- [x] `apps/bff` is absent and no active (non-archived) file references its
  package, Dockerfile, generator, or Node runtime.
- [x] Go BFF remains the only Compose/CI public BFF and `pnpm install`,
  `pnpm generate`, typecheck, tests, and build work with four workspace
  projects.
- [x] Go route, security, NDJSON, media, and architecture tests pass with no
  regression from the deletion.
- [x] Acceptance smoke scripts use the public BFF for browser operations and
  pass shell syntax checks.
- [x] Real Docker regression cases 1–7 are executed and recorded with request
  status, returned IDs, stream terminal state, detail/schedule evidence, and
  autonomous moment/proactive-message evidence.
- [x] No PostgreSQL, Core Python, schema, Redis, Temporal, or browser wire
  contract changes are introduced.
- [x] Changes are committed on `codex/go-bff-cutover` and the Trellis task is
  archived; remote push remains a separate explicit action.

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
