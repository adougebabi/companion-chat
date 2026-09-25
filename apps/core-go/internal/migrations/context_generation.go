package migrations

// One Fluctlight-scoped generation is the lightweight visibility fence for a
// Context Projection. Authority rows stay in their domain tables. Triggers
// advance this counter in the same transaction as every relevant mutation,
// including source correction/deletion, so prompt reads and settlement never
// need to scan or lock the full history.
const contextAuthorityGenerationSQL = `
CREATE TABLE IF NOT EXISTS public.fluctlight_context_generations (
 fluctlight_id varchar(128) PRIMARY KEY REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 generation bigint NOT NULL DEFAULT 0 CHECK (generation >= 0),
 updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO public.fluctlight_context_generations(fluctlight_id)
SELECT id FROM public.fluctlights ON CONFLICT(fluctlight_id) DO NOTHING;

CREATE OR REPLACE FUNCTION public.bump_fluctlight_context_generation(p_fluctlight_id text)
RETURNS void LANGUAGE plpgsql AS $context_generation$
BEGIN
 IF p_fluctlight_id IS NULL OR btrim(p_fluctlight_id)='' THEN RETURN; END IF;
 INSERT INTO public.fluctlight_context_generations(fluctlight_id,generation,updated_at)
 SELECT id,1,now() FROM public.fluctlights WHERE id=p_fluctlight_id
 ON CONFLICT(fluctlight_id) DO UPDATE SET generation=public.fluctlight_context_generations.generation+1,updated_at=now();
END
$context_generation$;

CREATE OR REPLACE FUNCTION public.bump_context_direct_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_direct$
BEGIN
 IF TG_OP='DELETE' THEN
  PERFORM public.bump_fluctlight_context_generation(OLD.fluctlight_id);
  IF TG_TABLE_NAME='memory_governance' THEN
   PERFORM public.refresh_resident_memory_snapshot(OLD.fluctlight_id);
  END IF;
  RETURN OLD;
 END IF;
 PERFORM public.bump_fluctlight_context_generation(NEW.fluctlight_id);
 IF TG_TABLE_NAME='memory_governance' THEN
  PERFORM public.refresh_resident_memory_snapshot(NEW.fluctlight_id);
 END IF;
 RETURN NEW;
END
$context_direct$;

CREATE OR REPLACE FUNCTION public.bump_context_owner_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_owner$
BEGIN
 IF TG_OP='DELETE' THEN
  PERFORM public.bump_fluctlight_context_generation(OLD.owner_fluctlight_id);
  IF TG_TABLE_NAME IN ('memories','active_memories','active_memory_commands') THEN
   PERFORM public.refresh_resident_memory_snapshot(OLD.owner_fluctlight_id);
  END IF;
  RETURN OLD;
 END IF;
 PERFORM public.bump_fluctlight_context_generation(NEW.owner_fluctlight_id);
 IF TG_TABLE_NAME IN ('memories','active_memories','active_memory_commands') THEN
  PERFORM public.refresh_resident_memory_snapshot(NEW.owner_fluctlight_id);
 END IF;
 RETURN NEW;
END
$context_owner$;

CREATE OR REPLACE FUNCTION public.bump_context_memory_child_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_memory_child$
DECLARE v_memory_id text; v_fluctlight_id text;
BEGIN
 IF TG_OP='DELETE' THEN v_memory_id := OLD.memory_id; ELSE v_memory_id := NEW.memory_id; END IF;
 SELECT owner_fluctlight_id INTO v_fluctlight_id FROM public.memories WHERE id=v_memory_id;
 PERFORM public.bump_fluctlight_context_generation(v_fluctlight_id);
 IF TG_TABLE_NAME='memory_source_links' THEN
  PERFORM public.refresh_resident_memory_snapshot(v_fluctlight_id);
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END
$context_memory_child$;

CREATE OR REPLACE FUNCTION public.bump_context_conversation_message_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_message$
DECLARE v_conversation_id text; v_fluctlight_id text;
BEGIN
 -- Assistant output can be published before its turn settlement. That
 -- settlement appends a sourced cognition result in the same owning path;
 -- bumping here would make the turn reject its own visible message.
 IF TG_OP='INSERT' AND NEW.kind='assistant' THEN RETURN NEW; END IF;
 IF TG_OP='DELETE' THEN v_conversation_id := OLD.conversation_id; ELSE v_conversation_id := NEW.conversation_id; END IF;
 IF TG_OP<>'INSERT' THEN
  UPDATE public.conversation_summaries SET status='invalidated'
  WHERE conversation_id=v_conversation_id AND status='active'
   AND source_message_refs ? ('message:' || OLD.id);
 END IF;
 FOR v_fluctlight_id IN
  SELECT p.actor_id FROM public.conversation_participants p
  JOIN public.actors a ON a.id=p.actor_id
  WHERE p.conversation_id=v_conversation_id AND a.actor_type='fluctlight'
 LOOP
  PERFORM public.bump_fluctlight_context_generation(v_fluctlight_id);
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
DECLARE v_fluctlight_id text; v_fact_id text;
BEGIN
 IF TG_OP='UPDATE' AND public.cognition_source_fingerprint(OLD.payload)=public.cognition_source_fingerprint(NEW.payload) THEN
  RETURN NEW;
 END IF;
 IF TG_OP='DELETE' THEN
  v_fluctlight_id := OLD.fluctlight_id;
  v_fact_id := OLD.id;
 ELSE
  v_fluctlight_id := NEW.fluctlight_id;
  v_fact_id := NEW.id;
 END IF;
 IF TG_OP<>'INSERT' THEN
  UPDATE public.conversation_summaries s SET status='invalidated'
  WHERE s.owner_fluctlight_id=v_fluctlight_id AND s.status='active'
   AND EXISTS (SELECT 1 FROM public.conversation_messages msg
     WHERE msg.source_fact_id=v_fact_id AND msg.conversation_id=s.conversation_id
      AND s.source_message_refs ? ('message:' || msg.id));
 END IF;
 PERFORM public.bump_fluctlight_context_generation(v_fluctlight_id);
 IF TG_OP<>'INSERT' THEN
  PERFORM public.refresh_resident_memory_snapshot(v_fluctlight_id);
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END
$context_fact$;

CREATE OR REPLACE FUNCTION public.bump_context_schedule_item_trigger()
RETURNS trigger LANGUAGE plpgsql AS $context_schedule_item$
DECLARE v_schedule_id text; v_fluctlight_id text;
BEGIN
 IF TG_OP='DELETE' THEN v_schedule_id := OLD.schedule_id; ELSE v_schedule_id := NEW.schedule_id; END IF;
 SELECT fluctlight_id INTO v_fluctlight_id FROM public.life_schedules WHERE id=v_schedule_id;
 PERFORM public.bump_fluctlight_context_generation(v_fluctlight_id);
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
 PERFORM public.bump_fluctlight_context_generation(NEW.id);
 RETURN NEW;
END
$context_fluctlight$;

DO $context_generation_triggers$
DECLARE v_table text;
BEGIN
 FOREACH v_table IN ARRAY ARRAY[
  'fluctlight_inner_states','fluctlight_affect_profiles','fluctlight_personality_runtime',
  'fluctlight_evolution_states','fluctlight_profile_habits','fluctlight_working_personas',
  'fluctlight_appearance_states','fluctlight_wardrobe_states','fluctlight_wardrobe_items',
  'fluctlight_wardrobe_outfits','fluctlight_wardrobe_outfit_items','fluctlight_worn_items',
  'fluctlight_life_activity_runs','fluctlight_goals','fluctlight_intentions',
  'fluctlight_drive_slots','fluctlight_preference_slots','fluctlight_trigger_preferences',
  'fluctlight_developing_self_claims','cognition_claims','cognition_action_outcomes','memory_governance','life_events',
  'life_schedules','life_presence_overlays','fluctlight_visual_identities'
 ] LOOP
  EXECUTE format('DROP TRIGGER IF EXISTS context_generation_bump ON public.%I',v_table);
  EXECUTE format('CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION public.bump_context_direct_trigger()',v_table);
 END LOOP;
 FOREACH v_table IN ARRAY ARRAY['memories','active_memories','active_memory_commands','conversation_summaries','relationships'] LOOP
  EXECUTE format('DROP TRIGGER IF EXISTS context_generation_bump ON public.%I',v_table);
  EXECUTE format('CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION public.bump_context_owner_trigger()',v_table);
 END LOOP;
 FOREACH v_table IN ARRAY ARRAY['memory_source_links','memory_embeddings'] LOOP
  EXECUTE format('DROP TRIGGER IF EXISTS context_generation_bump ON public.%I',v_table);
  EXECUTE format('CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION public.bump_context_memory_child_trigger()',v_table);
 END LOOP;
 DROP TRIGGER IF EXISTS context_generation_bump ON public.fluctlights;
 CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OF core_persona,current_revision,status,identity,personality,behavioral_policy,life_profile
  ON public.fluctlights FOR EACH ROW EXECUTE FUNCTION public.bump_context_fluctlight_trigger();
 DROP TRIGGER IF EXISTS context_generation_bump ON public.cognition_inbox;
 CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OF payload OR DELETE
  ON public.cognition_inbox FOR EACH ROW EXECUTE FUNCTION public.bump_context_fact_trigger();
 DROP TRIGGER IF EXISTS context_generation_bump ON public.conversation_messages;
 CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OF text,attachment_refs OR DELETE
  ON public.conversation_messages FOR EACH ROW EXECUTE FUNCTION public.bump_context_conversation_message_trigger();
 DROP TRIGGER IF EXISTS context_generation_bump ON public.life_schedule_items;
 CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE
  ON public.life_schedule_items FOR EACH ROW EXECUTE FUNCTION public.bump_context_schedule_item_trigger();
END
$context_generation_triggers$;
`
