package migrations

const diagnosticOperationsSchemaSQL = `
ALTER TABLE public.diagnostic_turns ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
ALTER TABLE public.diagnostic_workflow_links ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
ALTER TABLE public.platform_workflow_intents ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
ALTER TABLE public.kev_decisions ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
ALTER TABLE public.owner_surface_audits ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
ALTER TABLE public.platform_workflow_management_audit ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
CREATE INDEX IF NOT EXISTS owner_surface_audits_page ON public.owner_surface_audits(owner_actor_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS workflow_management_audit_page ON public.platform_workflow_management_audit(actor_id,created_at DESC,id DESC);
CREATE TABLE IF NOT EXISTS public.permanent_harm_rejections (
 id varchar(128) PRIMARY KEY, owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 fluctlight_id varchar(128) REFERENCES public.fluctlights(id), method varchar(16) NOT NULL,
 route text NOT NULL, boundary varchar(64) NOT NULL, reason_code varchar(128) NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id()
);
CREATE INDEX IF NOT EXISTS permanent_harm_rejections_owner_page ON public.permanent_harm_rejections(owner_actor_id,created_at DESC,id DESC);
`
