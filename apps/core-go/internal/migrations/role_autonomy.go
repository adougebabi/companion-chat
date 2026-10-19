package migrations

const roleAutonomySchemaSQL = `
ALTER TABLE public.autonomy_policies ADD COLUMN IF NOT EXISTS policy_source varchar(16) NOT NULL DEFAULT 'override';
ALTER TABLE public.autonomy_policies ADD COLUMN IF NOT EXISTS outreach_paused boolean NOT NULL DEFAULT false;
ALTER TABLE public.autonomy_policies ADD COLUMN IF NOT EXISTS notifications_muted boolean NOT NULL DEFAULT false;
ALTER TABLE public.autonomy_policies ADD COLUMN IF NOT EXISTS confirm_action_types jsonb NOT NULL DEFAULT '[]';
ALTER TABLE public.autonomy_policies ADD COLUMN IF NOT EXISTS cooldown_seconds integer NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS public.autonomy_policy_history (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 from_revision integer NOT NULL,
 to_revision integer NOT NULL,
 reason text NOT NULL,
 before_policy jsonb NOT NULL,
 after_policy jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(fluctlight_id,to_revision)
);
CREATE INDEX IF NOT EXISTS ix_autonomy_policy_history ON public.autonomy_policy_history(fluctlight_id,created_at DESC,id DESC);
`
