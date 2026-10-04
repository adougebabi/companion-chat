package migrations

const historyRepairSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.history_repair_batches (
 id varchar(128) PRIMARY KEY,
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 request_digest varchar(32) NOT NULL,
 reason text NOT NULL,
 status varchar(16) NOT NULL CHECK (status IN ('applied','rolled_back')),
 created_at timestamptz NOT NULL DEFAULT now(),
 rolled_back_at timestamptz
);
CREATE TABLE IF NOT EXISTS public.history_repair_items (
 batch_id varchar(128) NOT NULL REFERENCES public.history_repair_batches(id),
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 kind varchar(32) NOT NULL,
 target_id varchar(128) NOT NULL,
 before_value jsonb NOT NULL,
 after_value jsonb NOT NULL,
 source_records jsonb NOT NULL,
 expected_revision integer NOT NULL,
 resulting_revision integer NOT NULL,
 rollback_revision integer,
 PRIMARY KEY(batch_id,kind,target_id)
);
CREATE OR REPLACE FUNCTION public.invalidate_actor_fact_source()
RETURNS trigger LANGUAGE plpgsql AS $actor_fact_source$
DECLARE affected record;
BEGIN
 IF TG_OP='UPDATE' AND NEW.text IS NOT DISTINCT FROM OLD.text AND NEW.attachment_refs IS NOT DISTINCT FROM OLD.attachment_refs THEN RETURN NEW; END IF;
 UPDATE public.cognition_inbox SET payload=jsonb_set(payload,'{source_message_invalidated}','true'::jsonb,true) WHERE id=OLD.source_fact_id AND event_type='conversation.turn';
 FOR affected IN SELECT id,owner_fluctlight_id FROM public.actor_facts WHERE source_message_id=OLD.id AND status IN ('active','uncertain') LOOP
  UPDATE public.actor_facts SET status='quarantined',revision=revision+1 WHERE id=affected.id;
  UPDATE public.memories m SET provenance_status='invalid'
   WHERE m.owner_fluctlight_id=affected.owner_fluctlight_id AND EXISTS(SELECT 1 FROM public.actor_fact_artifacts d WHERE d.fact_id=affected.id AND d.artifact_kind='memory' AND d.artifact_id=m.id AND d.artifact_revision=m.revision);
  UPDATE public.conversation_runtime_summaries SET status='invalidated',revision=revision+1
   WHERE owner_fluctlight_id=affected.owner_fluctlight_id AND status='active';
 END LOOP;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $actor_fact_source$;
ALTER TABLE public.actor_facts DROP CONSTRAINT IF EXISTS actor_facts_source_message_id_fkey;
ALTER TABLE public.actor_facts ADD CONSTRAINT actor_facts_source_message_id_fkey FOREIGN KEY(source_message_id) REFERENCES public.conversation_messages(id) ON DELETE SET NULL;
DROP TRIGGER IF EXISTS actor_fact_source_invalidation ON public.conversation_messages;
CREATE TRIGGER actor_fact_source_invalidation BEFORE UPDATE OF text,attachment_refs OR DELETE ON public.conversation_messages
 FOR EACH ROW EXECUTE FUNCTION public.invalidate_actor_fact_source();
`
