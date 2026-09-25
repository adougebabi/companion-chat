package migrations

// Source links carry identity and validity only. Raw messages, facts, outcomes
// and their payloads stay in the owning tables; an unresolved historical ref
// remains explicitly unknown rather than acquiring invented evidence.
const memoryProvenanceSchemaSQL = `
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS provenance_status varchar(24) NOT NULL DEFAULT 'legacy_unknown';
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS epistemic_kind varchar(24) NOT NULL DEFAULT 'legacy_unknown';
DO $provenance_status$
BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_memories_provenance_status') THEN
  ALTER TABLE public.memories ADD CONSTRAINT ck_memories_provenance_status
   CHECK (provenance_status IN ('verified','partial','pending','legacy_unknown','invalid'));
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_memories_epistemic_kind') THEN
  ALTER TABLE public.memories ADD CONSTRAINT ck_memories_epistemic_kind
   CHECK (epistemic_kind IN ('user_statement','action_result','self_report','inference','summary','legacy_unknown'));
 END IF;
END
$provenance_status$;
DO $episode_status$
BEGIN
 IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_conversation_summaries_status'
   AND conrelid='public.conversation_summaries'::regclass
   AND pg_get_constraintdef(oid) NOT LIKE '%invalidated%') THEN
  ALTER TABLE public.conversation_summaries DROP CONSTRAINT ck_conversation_summaries_status;
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_conversation_summaries_status'
   AND conrelid='public.conversation_summaries'::regclass) THEN
  ALTER TABLE public.conversation_summaries ADD CONSTRAINT ck_conversation_summaries_status
   CHECK (status IN ('active','superseded','invalidated'));
 END IF;
END
$episode_status$;

CREATE TABLE IF NOT EXISTS public.memory_source_links (
 memory_id varchar(128) NOT NULL,
 memory_revision integer NOT NULL CHECK (memory_revision >= 0),
 source_ref varchar(256) NOT NULL CHECK (btrim(source_ref)<>''),
 source_kind varchar(24) NOT NULL DEFAULT 'legacy' CHECK (source_kind IN ('legacy','fact','message','outcome','memory','owner_confirmation','authenticated_command')),
 source_id varchar(256),
 source_revision integer,
 source_fingerprint varchar(32),
 occurred_at timestamptz,
 status varchar(16) NOT NULL DEFAULT 'unknown' CHECK (status IN ('valid','unknown','invalid')),
 superseded_by_ref varchar(256),
 recorded_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(memory_id,memory_revision,source_ref),
 FOREIGN KEY(memory_id,memory_revision) REFERENCES public.memory_revisions(memory_id,revision) ON DELETE CASCADE,
 CHECK ((status='unknown' AND source_id IS NULL) OR (status IN ('valid','invalid') AND source_id IS NOT NULL)),
 CHECK (source_fingerprint IS NULL OR source_fingerprint ~ '^[a-f0-9]{32}$')
);
ALTER TABLE public.memory_source_links ALTER COLUMN source_id TYPE varchar(256);
ALTER TABLE public.memory_source_links DROP CONSTRAINT IF EXISTS memory_source_links_source_kind_check;
ALTER TABLE public.memory_source_links ADD CONSTRAINT memory_source_links_source_kind_check
 CHECK (source_kind IN ('legacy','fact','message','outcome','memory','owner_confirmation','authenticated_command'));
CREATE INDEX IF NOT EXISTS ix_memory_source_lookup ON public.memory_source_links(source_kind,source_id,status,memory_id);
CREATE INDEX IF NOT EXISTS ix_memory_source_current ON public.memory_source_links(memory_id,memory_revision,status);

CREATE OR REPLACE FUNCTION public.cognition_source_fingerprint(source_payload jsonb)
RETURNS text LANGUAGE sql IMMUTABLE AS $cognition_source_fingerprint$
 SELECT md5((source_payload - 'agent_result' - 'agent_partial' - 'due_context' - 'cycle_guard')::text);
$cognition_source_fingerprint$;

CREATE OR REPLACE FUNCTION public.memory_source_is_live_deep(p_kind text, p_id text, p_revision integer, p_fingerprint text, p_depth integer, p_visited text[])
RETURNS boolean LANGUAGE plpgsql STABLE AS $memory_source_deep$
DECLARE source_memory record;
BEGIN
 IF p_depth > 16 THEN RETURN false; END IF;
 CASE p_kind
  WHEN 'fact' THEN RETURN EXISTS (SELECT 1 FROM public.cognition_inbox i WHERE i.id=p_id AND public.cognition_source_fingerprint(i.payload)=p_fingerprint);
  WHEN 'message' THEN RETURN EXISTS (SELECT 1 FROM public.conversation_messages msg WHERE msg.id=p_id AND md5(msg.text || msg.attachment_refs::text)=p_fingerprint);
  WHEN 'outcome' THEN RETURN EXISTS (SELECT 1 FROM public.cognition_action_outcomes o WHERE o.id=p_id AND o.revision=p_revision AND md5(o.request_digest || ':' || o.revision::text)=p_fingerprint);
  WHEN 'owner_confirmation', 'authenticated_command' THEN
   RETURN EXISTS (SELECT 1 FROM public.memory_governance g WHERE g.idempotency_key=p_id AND g.request_digest=p_fingerprint AND g.disposition IN ('applied','no_change'));
  WHEN 'memory' THEN
   IF p_id=ANY(p_visited) THEN RETURN false; END IF;
   SELECT id,revision,status,provenance_status,request_digest INTO source_memory FROM public.memories WHERE id=p_id;
   IF NOT FOUND OR source_memory.revision<>p_revision OR source_memory.status<>'active'
      OR source_memory.provenance_status NOT IN ('verified','partial') OR source_memory.request_digest<>p_fingerprint THEN
    RETURN false;
   END IF;
   RETURN EXISTS (SELECT 1 FROM public.memory_source_links l WHERE l.memory_id=p_id AND l.memory_revision=p_revision
     AND l.status='valid' AND public.memory_source_is_live_deep(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint,p_depth+1,p_visited || p_id));
  ELSE RETURN false;
 END CASE;
END
$memory_source_deep$;

CREATE OR REPLACE FUNCTION public.memory_source_is_live(source_kind text, source_id text, source_revision integer, source_fingerprint text)
RETURNS boolean LANGUAGE sql STABLE AS $memory_source_live$
 SELECT public.memory_source_is_live_deep($1,$2,$3,$4,0,ARRAY[]::text[]);
$memory_source_live$;

CREATE OR REPLACE FUNCTION public.active_memory_source_is_live(source_kind text, source_id text, source_fingerprint text, command_id text)
RETURNS boolean LANGUAGE sql STABLE AS $active_source_live$
 SELECT CASE source_kind
  WHEN 'fact' THEN EXISTS (SELECT 1 FROM public.cognition_inbox i WHERE i.id=source_id AND public.cognition_source_fingerprint(i.payload)=source_fingerprint)
  WHEN 'authenticated_command' THEN EXISTS (SELECT 1 FROM public.active_memory_commands c WHERE c.id=command_id AND c.command->>'source_fingerprint'=source_fingerprint)
  ELSE false END;
$active_source_live$;

INSERT INTO public.memory_source_links(memory_id,memory_revision,source_ref)
SELECT r.memory_id,r.revision,refs.ref
FROM public.memory_revisions r
CROSS JOIN LATERAL jsonb_array_elements_text(r.evidence_refs) AS refs(ref)
ON CONFLICT(memory_id,memory_revision,source_ref) DO NOTHING;

UPDATE public.memory_source_links l
SET source_kind='fact',source_id=i.id,source_fingerprint=public.cognition_source_fingerprint(i.payload),
 occurred_at=i.occurred_at,status='valid'
FROM public.memories m JOIN public.cognition_inbox i ON i.fluctlight_id=m.owner_fluctlight_id
WHERE l.memory_id=m.id AND l.status='unknown'
 AND ((l.source_ref='fact:' || i.id) OR l.source_ref=i.id
   OR (l.source_ref ~ '^sequence:[0-9]{1,9}$'
    AND i.sequence=CASE WHEN l.source_ref ~ '^sequence:[0-9]{1,9}$' THEN substring(l.source_ref from 10)::integer ELSE -1 END));

UPDATE public.memory_source_links l
SET source_kind='message',source_id=msg.id,
 source_fingerprint=md5(msg.text || msg.attachment_refs::text),
 occurred_at=msg.created_at,status='valid'
FROM public.memories m JOIN public.conversation_messages msg ON
 (m.conversation_id=msg.conversation_id OR EXISTS (
   SELECT 1 FROM public.conversation_participants p
   WHERE p.conversation_id=msg.conversation_id AND p.actor_id=m.owner_fluctlight_id))
WHERE l.memory_id=m.id AND l.status='unknown' AND l.source_ref=('message:' || msg.id);

UPDATE public.memory_source_links l
SET source_kind='outcome',source_id=o.id,source_revision=o.revision,
 source_fingerprint=md5(o.request_digest || ':' || o.revision::text),
 occurred_at=o.occurred_at,status='valid'
FROM public.memories m JOIN public.cognition_action_outcomes o ON o.fluctlight_id=m.owner_fluctlight_id
WHERE l.memory_id=m.id AND l.status='unknown' AND l.source_ref=('outcome:' || o.id);

WITH assessed AS (
 SELECT m.id,CASE
  WHEN NOT EXISTS (SELECT 1 FROM public.memory_source_links l WHERE l.memory_id=m.id AND l.memory_revision=m.revision AND l.status='unknown')
       AND EXISTS (SELECT 1 FROM public.memory_source_links l WHERE l.memory_id=m.id AND l.memory_revision=m.revision AND l.status='valid') THEN 'verified'
  WHEN EXISTS (SELECT 1 FROM public.memory_source_links l WHERE l.memory_id=m.id AND l.memory_revision=m.revision AND l.status='valid') THEN 'partial'
  ELSE 'legacy_unknown' END AS next_status
 FROM public.memories m WHERE m.provenance_status='legacy_unknown'
)
UPDATE public.memories m SET provenance_status=assessed.next_status
FROM assessed WHERE m.id=assessed.id AND m.provenance_status<>assessed.next_status;
`
