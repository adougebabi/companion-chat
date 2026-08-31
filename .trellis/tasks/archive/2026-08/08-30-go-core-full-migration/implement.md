# Execution plan

1. Freeze the live Python OpenAPI, generated clients, database schema and
   workflow inventory. Build the Go contract/fixture suite and ownership
   matrix. Record active Python workflow executions and define the authorized
   fence/cancel/rebuild procedure; history replay compatibility is out of scope.
2. Complete Go platform/config/auth/settings and PostgreSQL repository seams;
   port actors, foundation, Fluctlight lifecycle and direct conversations.
3. Port inner state, life world/schedule, memory, relationships, reflection and
   strict DecisionEffect/ReflectionProposal validation with transaction tests.
4. Port Provider role resolution/structured/streaming adapters, media/MinIO,
   moments, diagnostics and operations; preserve timeout/heartbeat/recovery.
5. Port cognition and autonomy application services, NDJSON turn streaming,
   daily-review registration, action settlement, retry and proactive delivery.
6. Port Temporal workflows/activities and Go Worker. Fence/cancel old Python
   executions, rebuild eligible pending intents with Go workflow IDs, disable
   Python dispatch and polling, and verify no queue has dual owners.
7. Switch the Go BFF/Core URL and Compose service graph; remove Python Core
   Docker/CI runtime references, while keeping migration/replay tooling only if
   explicitly documented and non-production.
8. Run Go race/vet/build, generated client checks, Web/BFF checks, Compose
   readiness, replay/restore checks and resource measurements.
9. Execute real Docker regression cases 1–7 through Web/BFF, recording first
   request outcomes, NDJSON terminals, media bytes, detail/schedule, Moment,
   proactive message and workflow evidence. No hidden retry or manual Worker
   restart.
10. Run final deletion/reference proof for `apps/core`, update specs and docs,
    commit and archive this stage.

Required rollback points: after each domain slice, the prior Python deployment
remains runnable in a separate disposable environment; never reset production
volumes or alter released migration IDs.
