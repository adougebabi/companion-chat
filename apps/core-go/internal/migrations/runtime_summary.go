package migrations

const runtimeSummarySchemaSQL = `
CREATE TABLE IF NOT EXISTS public.conversation_runtime_summaries (
 owner_fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 conversation_id varchar(128) NOT NULL REFERENCES public.conversations(id),
 revision integer NOT NULL DEFAULT 0,
 covered_through integer NOT NULL DEFAULT 0 CHECK (covered_through>=0),
 summary text NOT NULL DEFAULT '',
 source_digest varchar(32) NOT NULL DEFAULT '',
 status varchar(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active','invalidated')),
 updated_at timestamptz NOT NULL DEFAULT now(),
 last_business_at timestamptz,
 PRIMARY KEY(owner_fluctlight_id,conversation_id)
);
CREATE TABLE IF NOT EXISTS public.conversation_runtime_summary_revisions (
 owner_fluctlight_id varchar(128) NOT NULL,
 conversation_id varchar(128) NOT NULL,
 revision integer NOT NULL,
 from_sequence integer NOT NULL,
 to_sequence integer NOT NULL,
 previous_revision integer NOT NULL,
 summary text NOT NULL,
 source_digest varchar(32) NOT NULL,
 message_refs jsonb NOT NULL,
 model_id varchar(256) NOT NULL,
 provider_endpoint_id varchar(128) NOT NULL,
 occurred_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(owner_fluctlight_id,conversation_id,revision),
 FOREIGN KEY(owner_fluctlight_id,conversation_id) REFERENCES public.conversation_runtime_summaries(owner_fluctlight_id,conversation_id)
);
CREATE OR REPLACE FUNCTION public.invalidate_runtime_summary_from_message()
RETURNS trigger LANGUAGE plpgsql AS $runtime_summary_message$
BEGIN
 UPDATE public.conversation_runtime_summaries SET status='invalidated',revision=revision+1
 WHERE conversation_id=OLD.conversation_id AND covered_through>=OLD.sequence AND status='active';
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $runtime_summary_message$;
DROP TRIGGER IF EXISTS runtime_summary_message_invalidation ON public.conversation_messages;
CREATE TRIGGER runtime_summary_message_invalidation AFTER UPDATE OF text,attachment_refs OR DELETE ON public.conversation_messages
 FOR EACH ROW EXECUTE FUNCTION public.invalidate_runtime_summary_from_message();
DROP TRIGGER IF EXISTS context_generation_bump ON public.conversation_runtime_summaries;
CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.conversation_runtime_summaries
 FOR EACH ROW EXECUTE FUNCTION public.bump_context_owner_trigger();
`
