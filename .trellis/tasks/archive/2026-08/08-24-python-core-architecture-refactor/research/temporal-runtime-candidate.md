# Temporal Runtime Candidate

## Status

Accepted sole runtime after T01 DBOS FAIL and T01B core validation. Parent D036 supersedes the old resource-duration/12-hour soak blocker; remaining functional checks are assigned to T02/T09/T11.

## Official Documentation Evidence

Context7 sources fetched 2026-08-24:

- `/temporalio/documentation`
- `/temporalio/helm-charts`

Confirmed capabilities/facts:

- Temporal Service consists of durable server roles plus application Worker processes.
- Server roles may be grouped in one process; production guidance generally prefers separate roles for independent scaling, but Fluctlight intentionally targets non-HA personal NAS.
- PostgreSQL can provide both default and visibility stores; Elasticsearch/OpenSearch is not required.
- `temporal server start-dev` is for local development/testing, not the sustained NAS topology.
- Python SDK supports Workflow definitions, task queues, durable timers, Activity heartbeat/timeouts, cancellation, Signals, Queries, Updates, continue-as-new and Event History Replayer.
- Worker/task queue versioning APIs evolve; deprecated Build ID redirect APIs are not accepted. T01B must prove the current Worker Deployment Versioning path.
- Official Helm resources are unset by default. A documented HA example requests 512 MiB/500m per service pod, which is not the chosen NAS topology and cannot be treated as a minimum.

## Target Topology

```text
grouped temporal-server (non-HA)
PostgreSQL: temporal + temporal_visibility
Python Fluctlight Worker: interaction/lifecycle/media task queues
Temporal UI: off by default
Elasticsearch/OpenSearch: absent
Prometheus/Grafana/OTLP collector: absent
```

## Capacity Context

- Target NAS: 16 GiB RAM.
- MTPLX, ComfyUI and h3 run on another machine.
- Estimated Temporal incremental idle footprint: roughly 600 MiB-1.2 GiB including PostgreSQL workload overhead; this is a planning estimate, not official minimum.
- T01B measured roughly 139 MiB Temporal RSS and 425 MiB complete gate stack; these observations are sufficient and are not future hard gates.

## Why Temporal After DBOS

- T01 proved DBOS resource efficiency and many durability primitives.
- T01 rejected DBOS because required pause/restart was absent and active-history upgrade replay failed.
- Temporal's core model explicitly centers durable Event History, replay, signals/queries/updates, activity heartbeats, cancellation and Worker versioning.
- The expected additional memory is acceptable on the target NAS and avoids owning a custom long-term release/version protocol.

## Rejection Conditions

- Grouped server cannot run without Elasticsearch/external monitoring or requires the dev server for sustained operation.
- Signals/queries/updates/cancel/reset cannot provide the approved management semantics.
- Saved real histories cannot replay or route safely across current Worker Deployment versions.
- Long Activity recovery/idempotency or continue-as-new loses domain/business identity.
- PostgreSQL backup/restore cannot resume active workflows.
- Operation requires a second workflow/task runtime or custom queue fallback.
