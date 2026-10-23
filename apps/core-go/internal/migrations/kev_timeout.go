package migrations

// The Owner explicitly requested a ten-second Kev request timeout. Upgrade the
// existing setting once, retain every other option, and fence stale snapshots.
// A fresh database must not create/enable a Kev configuration implicitly.
const kevTimeoutSQL = `
UPDATE public.runtime_settings
SET value_json=jsonb_set(
      jsonb_set(value_json::jsonb,'{timeout_ms}','10000'::jsonb,true),
      '{budget_ms}',to_jsonb(GREATEST(COALESCE((value_json::jsonb->>'budget_ms')::integer,10000),10000)),true
    )::text,
    revision=revision+1,updated_at=now()
WHERE key='kev' AND jsonb_typeof(value_json::jsonb)='object'
  AND (value_json::jsonb->'timeout_ms' IS DISTINCT FROM '10000'::jsonb
       OR COALESCE((value_json::jsonb->>'budget_ms')::integer,0)<10000);
`
