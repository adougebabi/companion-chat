package migrations

// Goal evaluation now follows durable source activity instead of an endless
// interval after the previous model admission. This bundle is additive and
// idempotent so current databases repair the schema on startup.
const goalEvaluationQuietPeriodSchemaSQL = `
ALTER TABLE public.goal_agent_cadence
 ADD COLUMN IF NOT EXISTS last_activity_at timestamptz,
 ADD COLUMN IF NOT EXISTS activity_revision bigint NOT NULL DEFAULT 0,
 ADD COLUMN IF NOT EXISTS evaluation_activity_revision bigint NOT NULL DEFAULT 0;

-- Preserve unprocessed evidence already present before this schema was added.
WITH pending_activity AS (
 SELECT fluctlight_id,max(occurred_at) AS last_activity_at
 FROM public.goal_source_events
 WHERE processed_at IS NULL AND source_status IN ('committed','withdrawn')
 GROUP BY fluctlight_id
)
INSERT INTO public.goal_agent_cadence(fluctlight_id,last_activity_at,activity_revision)
 SELECT fluctlight_id,last_activity_at,1 FROM pending_activity
ON CONFLICT(fluctlight_id) DO UPDATE
 SET last_activity_at=GREATEST(COALESCE(goal_agent_cadence.last_activity_at,'-infinity'::timestamptz),EXCLUDED.last_activity_at),
     activity_revision=GREATEST(goal_agent_cadence.activity_revision,1),
     updated_at=now()
 WHERE goal_agent_cadence.activity_revision=0;

CREATE OR REPLACE FUNCTION public.capture_goal_evaluation_activity() RETURNS trigger LANGUAGE plpgsql AS $goal_evaluation_activity$
BEGIN
 IF NEW.source_status NOT IN ('committed','withdrawn') THEN RETURN NEW; END IF;
 -- Match the lock order used by Goal request producers and evaluation claims.
 PERFORM pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || NEW.fluctlight_id));
 INSERT INTO public.goal_agent_cadence(fluctlight_id,last_activity_at,activity_revision)
  VALUES(NEW.fluctlight_id,now(),1)
  ON CONFLICT(fluctlight_id) DO UPDATE
   SET last_activity_at=GREATEST(COALESCE(goal_agent_cadence.last_activity_at,'-infinity'::timestamptz),now()),
       activity_revision=goal_agent_cadence.activity_revision+1,
       updated_at=now();
 RETURN NEW;
END
$goal_evaluation_activity$;
DROP TRIGGER IF EXISTS trg_goal_evaluation_activity ON public.goal_source_events;
CREATE TRIGGER trg_goal_evaluation_activity AFTER INSERT ON public.goal_source_events
 FOR EACH ROW EXECUTE FUNCTION public.capture_goal_evaluation_activity();
`
