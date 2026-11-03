# Durable private-chat workflow boundary

Source: `.trellis/spec/backend/fluctlight-workflow-contract.md`, especially
“Cognition Claim Cleanup And Stream Retry” and “PostgreSQL-Authoritative
Recurring Wake-Up”. Read those complete sections when changing Worker or
WakeUp behavior.

- Public browser turns accept an unclaimed `cognition_inbox` fact and a
  `cognition.processing` intent. The Worker claims it. Internal synchronous
  `HandleTurn` keeps its older claim path.
- `ProcessCognitionInbox` uses the stable inbox and user-message identity.
  Workflow IDs can advance on an explicit retry after the old run is terminal;
  the user message and inbox IDs remain stable.
- A browser observer disconnect never cancels Worker execution. Explicit
  cancellation sets a Provider marker and requests Temporal cancellation;
  `cancel_requested` must become terminal before retry reopens the inbox.
- Chat cognition preempts WakeUp, then schedules WakeUp at configured interval
  plus ten minutes. Reflection has its own fixed ten-minute quiet period.
  `EnsureWakeUpIntents` repairs `superseded` after no executable cognition
  remains. Redis expiry is only a hint; PostgreSQL due time is authority.
