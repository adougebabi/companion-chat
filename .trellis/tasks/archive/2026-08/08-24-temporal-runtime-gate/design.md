# T01B Temporal Runtime Gate Design

## Authority

Parent task owns architecture. T01 archived DBOS FAIL. This child validates D020/D035 and cannot weaken workflow semantics or add another runtime.

## Gate Topology

```text
minimal gate API
minimal Python Temporal Worker
  interaction / lifecycle / media task queues
grouped non-HA Temporal Server
PostgreSQL temporal + temporal_visibility
structured stdout correlation
```

Temporal UI, Elasticsearch/OpenSearch, Prometheus/Grafana and OTLP collector are absent by default. The gate does not use `temporal server start-dev` as its sustained topology.

## Critical Scenarios

- Durable timer and stable Workflow ID across Worker/Server/PostgreSQL restarts.
- Signal pause/resume, Query status, validated Update/repair, cancel/terminate/reset/restart.
- 15-minute fake h3 Activity heartbeat/timeout/cancel and Provider checkpoint idempotency.
- Saved v1 Event History Replayer against v2 code.
- Current Worker Deployment Versioning coexist/drain/rollback.
- Continue-as-new before history limits with domain/signal continuity.
- PostgreSQL default+visibility backup/restore of active execution.
- Quantified 16 GiB NAS RSS/CPU/connections/readiness/recovery/disk gates.

## Decision

PASS authorizes parent preparation of T02 and finalizes Temporal as the sole runtime. FAIL blocks T02-T12 and returns parent workflow-runtime planning.
