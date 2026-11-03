# Code-path findings (read-only investigation)

## Schedule

- `intention.schedule` exists (`scheduled_activity_capability.go:13,28-49`; `builtin_capabilities.go:133-149`). `schedule.replan` exists (`schedule_capability.go:13-40`) but there is no agent-side schedule query or targeted edit/cancel capability (`phase8_contract_matrix_test.go:21-38`).
- `preserveScheduledActionPlans` (`scheduled_life_action.go:72-92`) maps current `intention_id` to `action_plan`; it emits `schedule_replan_intention_link_unknown` when a planned link has no nonempty current plan. Planner schema permits arbitrary string IDs (`schedule_capability.go:134-153`). `intention.schedule` calls this before creating its new Intention (`scheduled_activity_capability.go:108-126`).
- Existing schedule acceptance checks and closure tests are in `mutations.go:193-280` and `scheduled_activity_closure_test.go:16-120`.

## Capability requests

- Explicit `capability.request` reaches `capability_requests.go:44-173`; unknown tool exits at `adk_conversation_runtime.go:100-108`. Prompt policy does not direct missing capabilities to the request tool (`capability_prompt_policy.go:7-10`).
- Governance listing is `capability_requests.go:176-199` → `browser/routes.go:299` → `browser/dto.go:260` → `control-center.ts:628` → `GovernanceView.vue:157-160`.
- `InstancesView.vue:94-101` opens governance without loading capability requests; `App.vue:102-106` does load them. UI has no empty/loading distinction.

## Final output

- `decision_influence_N_ref_invalid` comes from `context_reference.go:445-486`; N is the array position. `provider_context_ref_alias_unknown` comes from current-run alias lookup in `provider_ref_codec.go:136-150`. Tool result and projection refresh registration paths exist (`adk_conversation_runtime.go:238-247`; `projection_refresh.go:23-54`).
- `appraisal.evidence_refs` is required by `provider_schemas.go:119-130`, although `normalizeAppraisal` fills a missing field with `[]` later (`cognition_growth.go:18-59`).
- Strict final JSON parse accepts only a full Content object (`provider.go:596-623`); ADK precheck wraps malformed content at `eino_model_runtime.go:951-968`. No final-output repair retry is present. `classifyAgentRunFailure` defaults to `agent_run_failed` (`agent_run_record.go:35-73`). Physical finish reason is hardcoded to `stop` at `eino_model_runtime.go:740-743`.
- Provider role setup checks `/models` existence but not actual JSON Schema/tool-call behavior (`operations.go:748-852`). The Provider spec's wrapper-tolerant parsing text (`fluctlight-provider-contract.md:144-148`) conflicts with strict code/tests.

## Reply publication

- `uq_conversation_messages_turn_kind` is a partial unique index on `(conversation_id,turn_id,kind)` (`migrations/runner.go:2270-2272`). Native Tool OperationID includes model call sequence and tool index (`adk_conversation_runtime.go:147-165`); ledger idempotency keys on OperationID (`tool_execution_ledger.go:40-101`).
- `conversation.reply` enters `PublishConversationReplyTx` (`builtin_capabilities.go:221-255`). Publisher looks up message ID and idempotency key, not turn/kind (`tool_publication.go:82-112`), then performs plain INSERT (`tool_publication.go:120-145`). A second reply with a distinct OperationID can hit the unique index.

## Evidence limits

These anchors establish the failure conditions, not the exact content of the reported live runs. The implementation phase should inspect sanitized diagnostic records for each run and avoid attributing an individual failure to a model, stale alias, legacy schedule row, or output truncation without that evidence.
