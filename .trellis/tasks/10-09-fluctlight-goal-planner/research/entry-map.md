# GP00 production entry map

Baseline: 71f6066febe068e8eae028e97e4fe409747bf149; master; clean tree.
Source inspection and user feedback are distinct from test evidence.

| Entry | Before | After | Permission / invariant |
|---|---|---|---|
| app.insertAgency / CreateFluctlight | Every initial goal active | Stable source import ledger, exact existing-history link, overflow candidate, one initialization request | Owner initialization; current autonomy permission; shared instance capacity |
| ApplyOwnerGoalCommand | Active create/resume without capacity | Same authority with DB/Life-lock capacity, candidate-only, retention and target Actor | GetFluctlight ownership, CAS, existing idempotency ledger |
| goal.decide | Model directly creates/resumes active goal | Model requests unified planning; privileged direct Owner command stays formal | Trusted invocation source; capacity in persistence, no invented active receipt |
| intention.decide | Implicit autonomous Goal create then candidate Intention | Model without linked active Goal queues wish, returns intention_created=false; direct explicit Owner remains formal | Native actual Tool result; ambiguous text binding refused |
| Reflection create/resume | Accepted proposal directly writes active Goal | Writes durable planning hint only | Captured profile/source; no second goal generator |
| Goal Resolution followup | Creates separate candidate directly | Keeps result/evidence/residual motive and queues Planner hint | Completion chain preserved; no autonomous followup writer |
| GoalPlanner commit | Absent | Atomically applies lifecycle/order/dependencies under policy/fact/claim fences | Owner trusted run, automatic/suggestions mode, no external-action tools |

All actual SQL Goal writers converge at persistGoalAuthorityTx plus goal_set_guard.
GoalComplete still belongs exclusively to GoalEvaluation and actual evidence.
Goal dependencies gate both Intention creation/promotion and actual action admission.

Actor authority: actor_facts; relationship authority: relationships/revisions/governance.
Existing actor-user and relationship interfaces remain; scoped Actor context API adds
background/preferences, per-Actor context digest, revisions and invalidation.
Vue Governance retains GoalPanel evidence/history and adds actual policy/order/dependency
and Actor editing panels. Both Browser/Core OpenAPI artifacts and generators are updated.

Review findings corrected: legal system resume, empty-applied durable commit recovery,
private-profile pending event consumption, same-Owner instance watcher, actual policy capacity
in migration, missing prior source review UI and generic Owner background invalidation.
The alleged pre-lock ledger read was disproved: ensureGoalSetTx acquires the shared
Life transaction lock first. A two-connection same-key test now proves replay equivalence.
