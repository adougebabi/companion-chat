# Technical Design

## Single public boundary

`apps/gateway-go` is the sole browser gateway. The old `apps/bff` tree is
deleted rather than retained as a compatibility runtime. The Compose service
name `bff` and image repository name remain stable so deployment does not need
an unrelated service rename; their Dockerfile and process are Go-only.

## Contract artifact ownership

The Browser OpenAPI generator moves to
`packages/browser-client/scripts/generate-openapi.mjs`, next to the artifact it
creates. Root `pnpm generate` invokes that package script followed by the
client generator. No generated client is hand-edited.

## Deployment and acceptance

Compose, CI, and acceptance scripts continue to use the existing `bff` public
service, but every browser operation in auth/domain/conversation/media smoke
is sent to the BFF URL. Core remains private to the Compose network. The
running deployment is tested through the Web-discovered BFF origin using an
owner session established with the supplied password; calls use a ten-minute
upper bound for slow LLM/media work.

## Autonomous scenarios

Case 6 and case 7 are observed through the authoritative public projections:
the created Fluctlight's moments feed, direct conversation messages, and
autonomy-action list. The description-based creation payload explicitly
requests a publishable moment and proactive contact, while polling is bounded
and records whether the Worker materializes those actions. No database rows
are inserted by the regression client.

## Scope and rollback

This is a deliberate one-way BFF migration. There is no Node rollback runtime
after this branch. Python Core/Worker, PostgreSQL/Alembic, Redis, MinIO,
Temporal, and browser wire schemas remain unchanged. If real regression fails,
the branch is not promoted; fixes happen on this same branch before merge.
