package migrations

const workflowControlsSchemaSQL = `
ALTER TABLE public.platform_workflow_intents ADD COLUMN IF NOT EXISTS intent_revision bigint NOT NULL DEFAULT 1;
CREATE OR REPLACE FUNCTION public.bump_workflow_intent_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (to_jsonb(NEW)-'intent_revision') IS DISTINCT FROM (to_jsonb(OLD)-'intent_revision') THEN NEW.intent_revision=OLD.intent_revision+1;
 ELSE NEW.intent_revision=OLD.intent_revision; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS workflow_intent_revision ON public.platform_workflow_intents;
CREATE TRIGGER workflow_intent_revision BEFORE UPDATE ON public.platform_workflow_intents FOR EACH ROW EXECUTE FUNCTION public.bump_workflow_intent_revision();
CREATE TABLE IF NOT EXISTS public.workflow_control_commands (
 id varchar(128) PRIMARY KEY, owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 idempotency_key varchar(256) NOT NULL, digest varchar(128) NOT NULL,
 workflow_id varchar(256) NOT NULL, intent_id varchar(128) NOT NULL REFERENCES public.platform_workflow_intents(intent_id),
 source_run_id varchar(256) NOT NULL, intent_revision bigint NOT NULL,
 action varchar(32) NOT NULL CHECK(action IN ('pause','resume','cancel','restart','reset')),
 reason text NOT NULL, input jsonb NOT NULL,
 status varchar(32) NOT NULL CHECK(status IN ('prepared','sent','unknown','rejected')),
 result jsonb, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id(), UNIQUE(owner_actor_id,idempotency_key)
);
CREATE INDEX IF NOT EXISTS workflow_control_commands_scope ON public.workflow_control_commands(workflow_id,created_at DESC,id DESC);
CREATE TABLE IF NOT EXISTS public.media_recovery_attempts(
 id varchar(128) PRIMARY KEY,owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 media_intent_id varchar(128) NOT NULL REFERENCES public.media_intents(id),source_workflow_id varchar(256) NOT NULL,
 workflow_id varchar(256) NOT NULL UNIQUE,intent_id varchar(128) NOT NULL UNIQUE,mode varchar(32) NOT NULL CHECK(mode IN ('resume','regenerate')),
 reason text NOT NULL,previous jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id()
);
CREATE INDEX IF NOT EXISTS media_recovery_attempts_page ON public.media_recovery_attempts(owner_actor_id,created_at DESC,id DESC);
`
