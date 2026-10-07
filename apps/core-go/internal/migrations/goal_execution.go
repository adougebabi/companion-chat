package migrations

// 0050 adds durable execution admission and retry state without rewriting facts.
const goalExecutionSchemaSQL = `
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS next_attempt_at timestamptz;
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS retry_count integer NOT NULL DEFAULT 0 CHECK (retry_count >= 0);
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS retry_reason text;
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS goal_hold_status varchar(32);
ALTER TABLE public.fluctlight_intention_attempts ADD COLUMN IF NOT EXISTS started_at timestamptz;
ALTER TABLE public.fluctlight_intention_attempts ADD COLUMN IF NOT EXISTS settled_at timestamptz;
ALTER TABLE public.fluctlight_intention_attempts ADD COLUMN IF NOT EXISTS wait_ref varchar(128);
ALTER TABLE public.fluctlight_intention_attempts ADD COLUMN IF NOT EXISTS deadline timestamptz;
ALTER TABLE public.fluctlight_intention_attempts ADD COLUMN IF NOT EXISTS intention_id varchar(128);
ALTER TABLE public.fluctlight_intention_attempts ADD COLUMN IF NOT EXISTS reconciliation_count integer NOT NULL DEFAULT 0 CHECK (reconciliation_count>=0);
ALTER TABLE public.fluctlight_intention_attempts DROP CONSTRAINT IF EXISTS ck_intention_attempt_authority_v2;
ALTER TABLE public.fluctlight_intention_attempts ADD CONSTRAINT ck_intention_attempt_authority_v2 CHECK (
 status IN ('running','waiting','needs_reconciliation','succeeded','failed','cancelled','suppressed')
 AND (outcome_digest ~ '^[a-f0-9]{32}$' OR (status IN ('running','waiting','needs_reconciliation') AND outcome_digest=''))
 AND jsonb_typeof(result)='object'
);
UPDATE public.fluctlight_intention_attempts SET settled_at=occurred_at
 WHERE settled_at IS NULL AND status IN ('succeeded','failed','cancelled','suppressed');
CREATE INDEX IF NOT EXISTS ix_intention_attempts_unsettled ON public.fluctlight_intention_attempts(fluctlight_id,deadline,attempt_id)
 WHERE status IN ('running','waiting');
CREATE INDEX IF NOT EXISTS ix_intentions_next_attempt ON public.fluctlight_intentions(next_attempt_at,id) WHERE status='qualified';
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS criteria_version integer NOT NULL DEFAULT 1 CHECK (criteria_version > 0);
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS criterion_ids jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(criterion_ids)='array');
UPDATE public.fluctlight_goals g SET criterion_ids=(
 SELECT jsonb_agg('criterion_' || substr(encode(digest(g.id || chr(31) || g.criteria_version::text || chr(31) || (ordinality-1)::text,'sha256'),'hex'),1,32) ORDER BY ordinality)
 FROM jsonb_array_elements(g.success_criteria) WITH ORDINALITY
) WHERE jsonb_array_length(g.criterion_ids)=0 AND jsonb_array_length(g.success_criteria)>0;
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS deadline_policy varchar(16) NOT NULL DEFAULT 'soft' CHECK (deadline_policy IN ('soft','hard'));
`
