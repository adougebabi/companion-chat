package migrations

// A segment remains source-auditable after daily consolidation. The Memory
// link checks the immutable revision and source fingerprint, while the raw
// message trigger invalidates both active and consolidated segments on edit.
const conversationDailyMemorySchemaSQL = `
ALTER TABLE public.conversation_summaries ADD COLUMN IF NOT EXISTS started_at timestamptz;
ALTER TABLE public.conversation_summaries ADD COLUMN IF NOT EXISTS ended_at timestamptz;
ALTER TABLE public.conversation_summaries ADD COLUMN IF NOT EXISTS timezone varchar(128);
ALTER TABLE public.conversation_summaries ADD COLUMN IF NOT EXISTS local_date date;
ALTER TABLE public.conversation_summaries ADD COLUMN IF NOT EXISTS ending_state text;
ALTER TABLE public.conversation_summaries ADD COLUMN IF NOT EXISTS open_threads jsonb NOT NULL DEFAULT '[]';
ALTER TABLE public.conversation_summaries ADD COLUMN IF NOT EXISTS core_events jsonb NOT NULL DEFAULT '[]';
ALTER TABLE public.conversation_summaries ADD COLUMN IF NOT EXISTS consolidated_into_memory_id varchar(128);
ALTER TABLE public.conversation_summaries DROP CONSTRAINT IF EXISTS ck_conversation_summaries_status;
ALTER TABLE public.conversation_summaries ADD CONSTRAINT ck_conversation_summaries_status
 CHECK (status IN ('active','superseded','invalidated','consolidated'));
CREATE INDEX IF NOT EXISTS ix_conversation_summaries_local_day
 ON public.conversation_summaries(owner_fluctlight_id,conversation_id,local_date,status,from_sequence);
CREATE INDEX IF NOT EXISTS ix_conversation_legacy_summary_reconcile
 ON public.conversation_summaries(completed_at,id) WHERE status='active' AND local_date IS NULL;

CREATE TABLE IF NOT EXISTS public.conversation_daily_memories (
 owner_fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 conversation_id varchar(128) NOT NULL REFERENCES public.conversations(id),
 local_date date NOT NULL,
 timezone varchar(128) NOT NULL,
 memory_id varchar(128) REFERENCES public.memories(id),
 source_fingerprint varchar(32),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(owner_fluctlight_id,conversation_id,local_date,timezone)
);
DO $daily_pk$
BEGIN
 IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.conversation_daily_memories'::regclass
   AND conname='conversation_daily_memories_pkey' AND pg_get_constraintdef(oid) NOT LIKE '%timezone%') THEN
  ALTER TABLE public.conversation_daily_memories DROP CONSTRAINT conversation_daily_memories_pkey;
  ALTER TABLE public.conversation_daily_memories ADD PRIMARY KEY(owner_fluctlight_id,conversation_id,local_date,timezone);
 END IF;
END
$daily_pk$;

ALTER TABLE public.memory_source_links DROP CONSTRAINT IF EXISTS memory_source_links_source_kind_check;
ALTER TABLE public.memory_source_links ADD CONSTRAINT memory_source_links_source_kind_check
 CHECK (source_kind IN ('legacy','fact','message','outcome','memory','conversation_summary','owner_confirmation','authenticated_command'));

CREATE OR REPLACE FUNCTION public.conversation_summary_source_fingerprint(p_id text)
RETURNS text LANGUAGE sql STABLE AS $summary_fingerprint$
 SELECT substr(encode(digest(
   s.id || chr(31) || s.revision::text || chr(31) || s.source_digest || chr(31) ||
   s.summary || chr(31) || s.prompt_version || chr(31) || s.schema_version || chr(31) || s.policy_version,
   'sha256'),'hex'),1,32)
 FROM public.conversation_summaries s WHERE s.id=p_id;
$summary_fingerprint$;

CREATE OR REPLACE FUNCTION public.memory_source_is_live_deep(p_kind text, p_id text, p_revision integer, p_fingerprint text, p_depth integer, p_visited text[])
RETURNS boolean LANGUAGE plpgsql STABLE AS $memory_source_deep$
DECLARE source_memory record;
BEGIN
 IF p_depth > 16 THEN RETURN false; END IF;
 CASE p_kind
  WHEN 'fact' THEN RETURN EXISTS (SELECT 1 FROM public.cognition_inbox i WHERE i.id=p_id AND public.cognition_source_fingerprint(i.payload)=p_fingerprint);
  WHEN 'message' THEN RETURN EXISTS (SELECT 1 FROM public.conversation_messages msg WHERE msg.id=p_id AND md5(msg.text || msg.attachment_refs::text)=p_fingerprint);
  WHEN 'outcome' THEN RETURN EXISTS (SELECT 1 FROM public.cognition_action_outcomes o WHERE o.id=p_id AND o.revision=p_revision AND md5(o.request_digest || ':' || o.revision::text)=p_fingerprint);
  WHEN 'conversation_summary' THEN RETURN EXISTS (
    SELECT 1 FROM public.conversation_summaries s WHERE s.id=p_id AND s.revision=p_revision
      AND s.status IN ('active','consolidated') AND public.conversation_summary_source_fingerprint(s.id)=p_fingerprint);
  WHEN 'owner_confirmation', 'authenticated_command' THEN
   RETURN EXISTS (SELECT 1 FROM public.memory_governance g WHERE g.idempotency_key=p_id AND g.request_digest=p_fingerprint AND g.disposition IN ('applied','no_change'));
  WHEN 'memory' THEN
   IF p_id=ANY(p_visited) THEN RETURN false; END IF;
   SELECT id,revision,status,provenance_status,request_digest INTO source_memory FROM public.memories WHERE id=p_id;
   IF NOT FOUND OR source_memory.revision<>p_revision OR source_memory.status<>'active'
      OR source_memory.provenance_status NOT IN ('verified','partial') OR source_memory.request_digest<>p_fingerprint THEN
    RETURN false;
   END IF;
   IF EXISTS (SELECT 1 FROM public.conversation_daily_memories d WHERE d.memory_id=p_id) THEN
    RETURN (SELECT count(*)>0 AND bool_and(l.status='valid' AND public.memory_source_is_live_deep(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint,p_depth+1,p_visited || p_id))
      FROM public.memory_source_links l WHERE l.memory_id=p_id AND l.memory_revision=p_revision);
   END IF;
   RETURN EXISTS (SELECT 1 FROM public.memory_source_links l WHERE l.memory_id=p_id AND l.memory_revision=p_revision
     AND l.status='valid' AND public.memory_source_is_live_deep(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint,p_depth+1,p_visited || p_id));
  ELSE RETURN false;
 END CASE;
END
$memory_source_deep$;

CREATE OR REPLACE FUNCTION public.invalidate_daily_memory_from_summary()
RETURNS trigger LANGUAGE plpgsql AS $invalidate_daily_memory$
BEGIN
 IF OLD.status IN ('active','consolidated') AND NEW.status IN ('invalidated','superseded') THEN
  UPDATE public.memories m SET provenance_status='invalid'
  WHERE m.status='active' AND m.provenance_status<>'invalid'
    AND EXISTS (SELECT 1 FROM public.conversation_daily_memories d WHERE d.memory_id=m.id)
    AND EXISTS (SELECT 1 FROM public.memory_source_links l WHERE l.memory_id=m.id AND l.memory_revision=m.revision
      AND l.source_kind='conversation_summary' AND l.source_id=OLD.id);
 END IF;
 RETURN NEW;
END
$invalidate_daily_memory$;
DROP TRIGGER IF EXISTS conversation_summary_daily_memory_invalidation ON public.conversation_summaries;
CREATE TRIGGER conversation_summary_daily_memory_invalidation
AFTER UPDATE OF status ON public.conversation_summaries
FOR EACH ROW EXECUTE FUNCTION public.invalidate_daily_memory_from_summary();
`
