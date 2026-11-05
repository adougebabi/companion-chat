package migrations

// Upgrade saved Kev settings once without enabling Kev or changing other
// options. A longer saved stage budget remains valid; later edits stay intact.
const kevOneMinuteSQL = `
UPDATE public.runtime_settings
SET value_json=jsonb_set(
      jsonb_set(value_json::jsonb,'{timeout_ms}','60000'::jsonb,true),
      '{budget_ms}',to_jsonb(GREATEST(COALESCE((value_json::jsonb->>'budget_ms')::integer,60000),60000)),true
    )::text,
    revision=revision+1,updated_at=now()
WHERE key='kev' AND jsonb_typeof(value_json::jsonb)='object'
  AND (value_json::jsonb->'timeout_ms' IS DISTINCT FROM '60000'::jsonb
       OR COALESCE((value_json::jsonb->>'budget_ms')::integer,0)<60000);
`
