package migrations

const goalReconciliationSchemaSQL = `
ALTER TABLE public.goal_reviews ADD COLUMN IF NOT EXISTS deadline_overdue boolean NOT NULL DEFAULT false;
CREATE TABLE IF NOT EXISTS public.goal_migration_review_flags (
 goal_id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL,
 goal_revision integer NOT NULL,
 reason text NOT NULL,
 snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(fluctlight_id,goal_id) REFERENCES public.fluctlight_goals(fluctlight_id,id)
);
CREATE TABLE IF NOT EXISTS public.goal_reconciliation_batches (
 id varchar(128) PRIMARY KEY,
 actor_id varchar(128) NOT NULL,
 fluctlight_id varchar(128) NOT NULL,
 request_digest varchar(64) NOT NULL,
 result jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
`
