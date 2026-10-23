package migrations

// The Owner requested concurrency one for this rollout. Apply once and fence
// older configuration snapshots; later deliberate configuration edits survive.
const serialLLMQueueSQL = `
UPDATE public.runtime_settings
SET value_json=jsonb_set(jsonb_set(value_json::jsonb,'{generated_concurrency}','1'::jsonb,true),'{embedding_concurrency}','1'::jsonb,true)::text,
    revision=revision+1,updated_at=now()
WHERE key='llm.queue' AND jsonb_typeof(value_json::jsonb)='object'
  AND (value_json::jsonb->'generated_concurrency' IS DISTINCT FROM '1'::jsonb
    OR value_json::jsonb->'embedding_concurrency' IS DISTINCT FROM '1'::jsonb);
`
