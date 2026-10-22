package migrations

// Restore only an untouched consolidated binding that still matches the legacy
// seed in every copied setting and retains the three accidentally defaulted
// budget fields. Explicitly revised/audited bindings retain their own limits.
const genericModelBudgetRepairSQL = `
WITH source AS (
 SELECT * FROM public.model_roles
 WHERE role IN ('action_realization','cognitive_assessment','interaction','reflection','initialization','media_prompt')
 ORDER BY CASE role WHEN 'action_realization' THEN 1 WHEN 'cognitive_assessment' THEN 2 WHEN 'interaction' THEN 3 WHEN 'reflection' THEN 4 WHEN 'initialization' THEN 5 ELSE 6 END
 LIMIT 1
)
UPDATE public.model_roles AS target SET
 context_window_tokens=source.context_window_tokens,
 max_input_tokens=source.max_input_tokens,
 prompt_budget_policy_version=source.prompt_budget_policy_version
FROM source
WHERE target.role='generic_llm'
 AND (target.context_window_tokens,target.max_input_tokens,target.prompt_budget_policy_version)=(65536,49152,'prompt-budget.v1')
 AND (source.context_window_tokens,source.max_input_tokens,source.prompt_budget_policy_version) IS DISTINCT FROM (65536,49152,'prompt-budget.v1')
 AND (target.provider_endpoint_id,target.model_id,target.token_budget,target.timeout_seconds,target.required_capabilities,target.retry_policy)
     IS NOT DISTINCT FROM (source.provider_endpoint_id,source.model_id,source.token_budget,source.timeout_seconds,source.required_capabilities,source.retry_policy)
 AND EXISTS(SELECT 1 FROM public.configuration_revisions WHERE resource_key='binding:generic_llm' AND revision=1)
 AND NOT EXISTS(SELECT 1 FROM public.owner_surface_audits WHERE action='provider_binding_update' AND reason LIKE '%"binding:generic_llm"%');
`
