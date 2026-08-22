# Technical Design

## Parent Boundaries

The parent task coordinates two vertical slices:

```text
description -> interview analyzer -> ready session -> user confirmation -> activate

media intent/activity -> frozen media target -> durable submit job
  -> provider/poll -> compensation/repair -> ready or terminal failed
```

The interview analyzer must not know about media providers. The media compensation path must consume already-frozen media payloads and must not ask an LLM to reinterpret them.

## Shared Contracts

- Existing HTTP route names and Vue wizard stages remain stable.
- New interview metadata is additive (`source`, `inferredFields`); old rows remain readable.
- Durable jobs continue to use `companion_jobs`, `job-repository`, `job-dispatcher`, and the existing lease owner. New follow-up/compensation keys must be deterministic and queryable from payload.
- Provider failures remain server-side and bounded. User-facing error DTOs must not expose credentials, raw prompts, or provider response bodies.

## Rollout and Rollback

Implement and test the analyzer child first because activation data shape is independent of media compensation. Then implement media transaction/follow-up repair. Each child is additive and can be reverted without deleting current persona or job rows. Existing jobs should be readable by old handlers during a partial deployment; new handlers must fail closed rather than mark unknown jobs complete.

## Cross-Layer Acceptance

The final integration pass must prove:

1. No default production path calls the repository regex analyzer.
2. No failed analysis mutates SQLite.
3. A media target cannot remain indefinitely `processing` solely because a follow-up job enqueue failed.
4. Replays preserve the same persona/media identity and do not duplicate visible rows or provider calls.
