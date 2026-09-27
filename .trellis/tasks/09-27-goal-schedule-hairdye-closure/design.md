# Design

## Authority and boundary

Add a typed scheduled virtual activity bridge. A planning Tool runs the existing schedule planner before its mutation transaction, then commits one Goal/Intention and a versioned current-local-day Schedule with one linked item. The linked item stores `intention_id` and a closed `action_plan` (capability `life.activity.start`, kind, bounded request, duration). Intention's time trigger and preferred time are set to that item's start. Normal schedule items remain descriptive and cannot execute prose.

Migration `0038` adds nullable `intention_id` and `action_plan` to `life_schedule_items`, with indexes/constraints; existing rows remain descriptive. Schedule reads and browser DTOs expose the optional link. A new schedule version must preserve or explicitly invalidate still-active linked intentions. No rewrite of earlier accepted versions occurs.

## Execution

The existing `IntentionTriggerWorkflow` is the durable clock. At due, Core reloads the current accepted schedule and linked item, verifies its time window, intention revision/status, and typed action plan, then starts the virtual activity through the existing `ExecuteTool` authorization/idempotency boundary. An unlinked `agency.intention_due` fact continues through Native Cognition; fix the Worker dispatch branch accordingly. The workflow waits until the activity's `not_before` and invokes the existing `life.activity.advance` result Tool, continuing as new on deferral so histories remain bounded. Every action and result has a stable business operation ID.

## Hair dye and conversation truth

Add `hair_dye` to the virtual activity schema. Its request requires `desired_hair_color` and does not require a length change. Result validation requires a confirmed completed color equal to the requested target; settlement changes only `hair_color` and records an appearance revision. `appearance.style` remains for temporary arrangements, and the conversation prompt distinguishes scheduled, in-progress, and completed state using the accepted Schedule and actual activity/result facts. A future plan is never described as a current activity.

## Compatibility and recovery

Existing schedule items and intentions lack action links and retain their current behavior. Existing due facts route correctly to Native Cognition. A scheduled item that is removed by replan, or whose intention is paused/cancelled, cannot start; the workflow reports an explicit stale/deferred state. Provider outage keeps the activity pending for durable retry, without fabricating a successful color change. The new Temporal branch uses history versioning so pre-deployment IntentionTrigger histories replay unchanged.
