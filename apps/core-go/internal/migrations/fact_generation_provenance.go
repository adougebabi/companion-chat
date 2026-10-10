package migrations

const FactGenerationProvenancePreviousHead = "0058_semantic_fact_generation"

// Every semantic generation advance records its bounded provenance in the
// same transaction. The journal contains identifiers only; domain payloads
// remain in their owning tables. Replacing the historical trigger functions
// here preserves all of their invalidation and resident-snapshot side effects
// while making their source explicit.
const factGenerationProvenanceSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.fluctlight_context_generation_journal (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 generation bigint NOT NULL CHECK (generation > 0),
 source_table varchar(128) NOT NULL CHECK (btrim(source_table) <> ''),
 source_operation varchar(16) NOT NULL CHECK (source_operation IN ('insert','update','delete','unknown')),
 entity_id varchar(256),
 transaction_id bigint NOT NULL,
 occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(fluctlight_id,generation)
);
CREATE INDEX IF NOT EXISTS ix_context_generation_journal_recent
 ON public.fluctlight_context_generation_journal(fluctlight_id,generation DESC);

CREATE OR REPLACE FUNCTION public.context_generation_entity_id(p_row jsonb)
RETURNS text LANGUAGE sql IMMUTABLE AS $context_generation_entity$
 SELECT NULLIF(btrim(COALESCE(
  p_row->>'id',p_row->>'memory_id',p_row->>'item_id',p_row->>'schedule_id',
  p_row->>'conversation_id',p_row->>'fluctlight_id',p_row->>'owner_fluctlight_id',''
 )), '');
$context_generation_entity$;

CREATE OR REPLACE FUNCTION public.bump_fluctlight_context_generation_with_source(
 p_fluctlight_id text,
 p_source_table text,
 p_source_operation text,
 p_entity_id text DEFAULT NULL
)
RETURNS void LANGUAGE plpgsql AS $context_generation_source$
DECLARE v_generation bigint; v_table text; v_operation text;
BEGIN
 IF p_fluctlight_id IS NULL OR btrim(p_fluctlight_id)='' THEN RETURN; END IF;
 v_table := COALESCE(NULLIF(btrim(p_source_table),''),'unknown');
 v_operation := lower(COALESCE(NULLIF(btrim(p_source_operation),''),'unknown'));
 IF v_operation NOT IN ('insert','update','delete','unknown') THEN v_operation := 'unknown'; END IF;
 INSERT INTO public.fluctlight_context_generations(fluctlight_id,generation,updated_at)
 SELECT id,1,now() FROM public.fluctlights WHERE id=p_fluctlight_id
 ON CONFLICT(fluctlight_id) DO UPDATE
  SET generation=public.fluctlight_context_generations.generation+1,updated_at=now()
 RETURNING generation INTO v_generation;
 IF v_generation IS NULL THEN RETURN; END IF;
 INSERT INTO public.fluctlight_context_generation_journal(
  fluctlight_id,generation,source_table,source_operation,entity_id,transaction_id,occurred_at
 ) VALUES (
  p_fluctlight_id,v_generation,v_table,v_operation,NULLIF(btrim(p_entity_id),''),txid_current(),clock_timestamp()
 );
 DELETE FROM public.fluctlight_context_generation_journal
 WHERE fluctlight_id=p_fluctlight_id AND generation<=v_generation-512;
END
$context_generation_source$;

CREATE OR REPLACE FUNCTION public.bump_fluctlight_context_generation(p_fluctlight_id text)
RETURNS void LANGUAGE plpgsql AS $context_generation$
BEGIN
 PERFORM public.bump_fluctlight_context_generation_with_source(p_fluctlight_id,'unknown','unknown',NULL);
END
$context_generation$;

CREATE OR REPLACE FUNCTION public.bump_context_direct_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_direct$
DECLARE v_row jsonb;
BEGIN
 IF TG_OP='UPDATE' AND TG_TABLE_NAME='fluctlight_intentions'
    AND (to_jsonb(OLD)-'trigger_cursor_sequence'-'updated_at')
        IS NOT DISTINCT FROM (to_jsonb(NEW)-'trigger_cursor_sequence'-'updated_at') THEN
  RETURN NEW;
 END IF;
 IF TG_OP='DELETE' THEN
  v_row := to_jsonb(OLD);
  PERFORM public.bump_fluctlight_context_generation_with_source(
   OLD.fluctlight_id,TG_TABLE_NAME,TG_OP,public.context_generation_entity_id(v_row));
  IF TG_TABLE_NAME='memory_governance' THEN
   PERFORM public.refresh_resident_memory_snapshot(OLD.fluctlight_id);
  END IF;
  RETURN OLD;
 END IF;
 v_row := to_jsonb(NEW);
 PERFORM public.bump_fluctlight_context_generation_with_source(
  NEW.fluctlight_id,TG_TABLE_NAME,TG_OP,public.context_generation_entity_id(v_row));
 IF TG_TABLE_NAME='memory_governance' THEN
  PERFORM public.refresh_resident_memory_snapshot(NEW.fluctlight_id);
 END IF;
 RETURN NEW;
END
$context_direct$;

CREATE OR REPLACE FUNCTION public.bump_context_owner_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_owner$
DECLARE v_row jsonb;
BEGIN
 IF TG_OP='DELETE' THEN
  v_row := to_jsonb(OLD);
  PERFORM public.bump_fluctlight_context_generation_with_source(
   OLD.owner_fluctlight_id,TG_TABLE_NAME,TG_OP,public.context_generation_entity_id(v_row));
  IF TG_TABLE_NAME IN ('memories','active_memories','active_memory_commands') THEN
   PERFORM public.refresh_resident_memory_snapshot(OLD.owner_fluctlight_id);
  END IF;
  RETURN OLD;
 END IF;
 v_row := to_jsonb(NEW);
 PERFORM public.bump_fluctlight_context_generation_with_source(
  NEW.owner_fluctlight_id,TG_TABLE_NAME,TG_OP,public.context_generation_entity_id(v_row));
 IF TG_TABLE_NAME IN ('memories','active_memories','active_memory_commands') THEN
  PERFORM public.refresh_resident_memory_snapshot(NEW.owner_fluctlight_id);
 END IF;
 RETURN NEW;
END
$context_owner$;

CREATE OR REPLACE FUNCTION public.bump_context_memory_child_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_memory_child$
DECLARE v_memory_id text; v_fluctlight_id text; v_row jsonb;
BEGIN
 IF TG_OP='DELETE' THEN v_memory_id := OLD.memory_id; v_row := to_jsonb(OLD);
 ELSE v_memory_id := NEW.memory_id; v_row := to_jsonb(NEW); END IF;
 SELECT owner_fluctlight_id INTO v_fluctlight_id FROM public.memories WHERE id=v_memory_id;
 PERFORM public.bump_fluctlight_context_generation_with_source(
  v_fluctlight_id,TG_TABLE_NAME,TG_OP,public.context_generation_entity_id(v_row));
 IF TG_TABLE_NAME='memory_source_links' THEN
  PERFORM public.refresh_resident_memory_snapshot(v_fluctlight_id);
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END
$context_memory_child$;

CREATE OR REPLACE FUNCTION public.bump_context_conversation_message_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_message$
DECLARE v_conversation_id text; v_fluctlight_id text; v_row jsonb;
BEGIN
 IF TG_OP='INSERT' AND NEW.kind='assistant' THEN RETURN NEW; END IF;
 IF TG_OP='DELETE' THEN v_conversation_id := OLD.conversation_id; v_row := to_jsonb(OLD);
 ELSE v_conversation_id := NEW.conversation_id; v_row := to_jsonb(NEW); END IF;
 IF TG_OP<>'INSERT' THEN
  UPDATE public.conversation_summaries SET status='invalidated'
  WHERE conversation_id=v_conversation_id AND status IN ('active','consolidated')
   AND source_message_refs ? ('message:' || OLD.id);
 END IF;
 FOR v_fluctlight_id IN
  SELECT p.actor_id FROM public.conversation_participants p
  JOIN public.actors a ON a.id=p.actor_id
  WHERE p.conversation_id=v_conversation_id AND a.actor_type='fluctlight'
 LOOP
  PERFORM public.bump_fluctlight_context_generation_with_source(
   v_fluctlight_id,TG_TABLE_NAME,TG_OP,public.context_generation_entity_id(v_row));
  IF TG_OP<>'INSERT' THEN
   PERFORM public.refresh_resident_memory_snapshot(v_fluctlight_id);
  END IF;
 END LOOP;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END
$context_message$;

CREATE OR REPLACE FUNCTION public.bump_context_fact_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_fact$
DECLARE v_fluctlight_id text; v_fact_id text; v_row jsonb;
BEGIN
 IF TG_OP='UPDATE' AND public.cognition_source_fingerprint(OLD.payload)=public.cognition_source_fingerprint(NEW.payload) THEN
  RETURN NEW;
 END IF;
 IF TG_OP='DELETE' THEN
  v_fluctlight_id := OLD.fluctlight_id; v_fact_id := OLD.id; v_row := to_jsonb(OLD);
 ELSE
  v_fluctlight_id := NEW.fluctlight_id; v_fact_id := NEW.id; v_row := to_jsonb(NEW);
 END IF;
 IF TG_OP<>'INSERT' THEN
  UPDATE public.conversation_summaries s SET status='invalidated'
  WHERE s.owner_fluctlight_id=v_fluctlight_id AND s.status IN ('active','consolidated')
   AND EXISTS (SELECT 1 FROM public.conversation_messages msg
     WHERE msg.source_fact_id=v_fact_id AND msg.conversation_id=s.conversation_id
      AND s.source_message_refs ? ('message:' || msg.id));
 END IF;
 PERFORM public.bump_fluctlight_context_generation_with_source(
  v_fluctlight_id,TG_TABLE_NAME,TG_OP,public.context_generation_entity_id(v_row));
 IF TG_OP<>'INSERT' THEN
  PERFORM public.refresh_resident_memory_snapshot(v_fluctlight_id);
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END
$context_fact$;

CREATE OR REPLACE FUNCTION public.bump_context_schedule_item_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_schedule_item$
DECLARE v_schedule_id text; v_fluctlight_id text; v_row jsonb;
BEGIN
 IF TG_OP='DELETE' THEN v_schedule_id := OLD.schedule_id; v_row := to_jsonb(OLD);
 ELSE v_schedule_id := NEW.schedule_id; v_row := to_jsonb(NEW); END IF;
 SELECT fluctlight_id INTO v_fluctlight_id FROM public.life_schedules WHERE id=v_schedule_id;
 PERFORM public.bump_fluctlight_context_generation_with_source(
  v_fluctlight_id,TG_TABLE_NAME,TG_OP,public.context_generation_entity_id(v_row));
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END
$context_schedule_item$;

CREATE OR REPLACE FUNCTION public.bump_context_fluctlight_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_fluctlight$
BEGIN
 IF TG_OP='INSERT' THEN
  INSERT INTO public.fluctlight_context_generations(fluctlight_id) VALUES(NEW.id)
   ON CONFLICT(fluctlight_id) DO NOTHING;
  PERFORM public.refresh_resident_memory_snapshot(NEW.id);
  RETURN NEW;
 END IF;
 PERFORM public.bump_fluctlight_context_generation_with_source(NEW.id,TG_TABLE_NAME,TG_OP,NEW.id);
 RETURN NEW;
END
$context_fluctlight$;
`
