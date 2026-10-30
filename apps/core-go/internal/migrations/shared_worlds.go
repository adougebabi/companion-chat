package migrations

// World identity is shared; creator fluctlight_id is provenance only. Existing
// worlds and their histories retain their IDs and are never merged by name.
const sharedWorldsSQL = `
ALTER TABLE public.life_schedules ADD COLUMN IF NOT EXISTS place_binding_version integer NOT NULL DEFAULT 0;
ALTER TABLE public.world_place_default_history ALTER COLUMN fluctlight_id DROP NOT NULL;
ALTER TABLE public.life_worlds ADD COLUMN IF NOT EXISTS owner_actor_id varchar(128) REFERENCES public.actors(id);
ALTER TABLE public.life_worlds ADD COLUMN IF NOT EXISTS timezone text;
UPDATE public.life_worlds w SET owner_actor_id=f.created_by_actor_id FROM public.fluctlights f WHERE w.fluctlight_id=f.id AND w.owner_actor_id IS NULL;
UPDATE public.life_worlds w SET timezone=COALESCE((SELECT z.name FROM public.fluctlights f JOIN pg_timezone_names z ON z.name=f.identity->>'timezone' WHERE f.id=w.fluctlight_id LIMIT 1),'Asia/Shanghai') WHERE timezone IS NULL;
ALTER TABLE public.life_worlds ALTER COLUMN owner_actor_id SET NOT NULL;
ALTER TABLE public.life_worlds ALTER COLUMN timezone SET NOT NULL;
ALTER TABLE public.life_worlds ALTER COLUMN fluctlight_id DROP NOT NULL;
ALTER TABLE public.life_worlds DROP CONSTRAINT IF EXISTS life_worlds_fluctlight_id_fkey;
ALTER TABLE public.life_worlds ADD CONSTRAINT life_worlds_fluctlight_id_fkey FOREIGN KEY(fluctlight_id) REFERENCES public.fluctlights(id) ON DELETE SET NULL;
CREATE OR REPLACE FUNCTION public.initialize_world_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.owner_actor_id IS NULL THEN SELECT created_by_actor_id INTO NEW.owner_actor_id FROM public.fluctlights WHERE id=NEW.fluctlight_id; END IF;
 IF NEW.timezone IS NULL THEN SELECT z.name INTO NEW.timezone FROM public.fluctlights f JOIN pg_timezone_names z ON z.name=f.identity->>'timezone' WHERE f.id=NEW.fluctlight_id LIMIT 1; NEW.timezone:=COALESCE(NEW.timezone,'Asia/Shanghai'); END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_timezone_names WHERE name=NEW.timezone) THEN RAISE EXCEPTION 'world_timezone_invalid' USING ERRCODE='23514'; END IF;
 IF TG_OP='UPDATE' AND NEW.owner_actor_id IS DISTINCT FROM OLD.owner_actor_id THEN RAISE EXCEPTION 'world_owner_immutable' USING ERRCODE='23514'; END IF;
 RETURN NEW; END $$;
DROP TRIGGER IF EXISTS world_owner_defaults ON public.life_worlds;
CREATE TRIGGER world_owner_defaults BEFORE INSERT OR UPDATE ON public.life_worlds FOR EACH ROW EXECUTE FUNCTION public.initialize_world_owner();
ALTER TABLE public.fluctlight_world_bindings DROP CONSTRAINT IF EXISTS fluctlight_world_bindings_fluctlight_id_world_id_fkey;
ALTER TABLE public.world_items DROP CONSTRAINT IF EXISTS world_items_fluctlight_id_world_id_fkey;
ALTER TABLE public.fluctlight_wardrobe_items DROP CONSTRAINT IF EXISTS inventory_world_item_source_fk;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='shared_world_binding_fk') THEN ALTER TABLE public.fluctlight_world_bindings ADD CONSTRAINT shared_world_binding_fk FOREIGN KEY(world_id) REFERENCES public.life_worlds(id); END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='shared_world_item_fk') THEN ALTER TABLE public.world_items ADD CONSTRAINT shared_world_item_fk FOREIGN KEY(world_id) REFERENCES public.life_worlds(id); END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='shared_inventory_world_item_fk') THEN ALTER TABLE public.fluctlight_wardrobe_items ADD CONSTRAINT shared_inventory_world_item_fk FOREIGN KEY(source_world_item_id) REFERENCES public.world_items(id); END IF;
END $$;
CREATE OR REPLACE FUNCTION public.validate_world_member_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM public.life_worlds w JOIN public.fluctlights f ON f.created_by_actor_id=w.owner_actor_id WHERE w.id=NEW.world_id AND f.id=NEW.fluctlight_id) THEN RAISE EXCEPTION 'world_owner_mismatch' USING ERRCODE='23514'; END IF;
 RETURN NEW; END $$;
DROP TRIGGER IF EXISTS world_member_owner ON public.fluctlight_world_bindings;
CREATE TRIGGER world_member_owner BEFORE INSERT OR UPDATE ON public.fluctlight_world_bindings FOR EACH ROW EXECUTE FUNCTION public.validate_world_member_owner();
DROP TRIGGER IF EXISTS world_item_owner ON public.world_items;
CREATE TRIGGER world_item_owner BEFORE INSERT OR UPDATE ON public.world_items FOR EACH ROW EXECUTE FUNCTION public.validate_world_member_owner();
CREATE OR REPLACE FUNCTION public.validate_inventory_world_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.source_world_item_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM public.world_items i JOIN public.life_worlds w ON w.id=i.world_id JOIN public.fluctlights f ON f.created_by_actor_id=w.owner_actor_id WHERE i.id=NEW.source_world_item_id AND f.id=NEW.fluctlight_id) THEN RAISE EXCEPTION 'inventory_world_owner_mismatch' USING ERRCODE='23514'; END IF;
 RETURN NEW; END $$;
DROP TRIGGER IF EXISTS inventory_world_owner ON public.fluctlight_wardrobe_items;
CREATE TRIGGER inventory_world_owner BEFORE INSERT OR UPDATE ON public.fluctlight_wardrobe_items FOR EACH ROW EXECUTE FUNCTION public.validate_inventory_world_owner();
CREATE OR REPLACE FUNCTION public.bump_world_context_trigger() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE v_world text; v_previous text; v_fid text;
BEGIN
 IF to_regclass('public.fluctlight_world_bindings') IS NULL THEN IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF; END IF;
 IF TG_TABLE_NAME='life_worlds' THEN IF TG_OP='DELETE' THEN v_world:=OLD.id; ELSE v_world:=NEW.id; END IF;
 ELSE IF TG_OP='DELETE' THEN v_world:=OLD.world_id; ELSE v_world:=NEW.world_id; END IF; IF TG_OP='UPDATE' THEN v_previous:=OLD.world_id; END IF; END IF;
 FOR v_fid IN SELECT fluctlight_id FROM public.fluctlight_world_bindings WHERE world_id=v_world OR world_id=v_previous ORDER BY fluctlight_id LOOP
  PERFORM public.bump_fluctlight_context_generation_with_source(v_fid,TG_TABLE_NAME,TG_OP,NULL);
 END LOOP;
 IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END $$;
DROP TRIGGER IF EXISTS context_generation_bump ON public.life_worlds;
CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.life_worlds FOR EACH ROW EXECUTE FUNCTION public.bump_world_context_trigger();
DROP TRIGGER IF EXISTS shared_world_context_bump ON public.fluctlight_world_bindings;
CREATE TRIGGER shared_world_context_bump AFTER INSERT OR UPDATE OR DELETE ON public.fluctlight_world_bindings FOR EACH ROW EXECUTE FUNCTION public.bump_world_context_trigger();
CREATE TABLE IF NOT EXISTS public.world_event_generations (
 id varchar(128) PRIMARY KEY,world_id varchar(128) NOT NULL REFERENCES public.life_worlds(id),
 slot_at timestamptz NOT NULL,status varchar(32) NOT NULL,proposal jsonb,context jsonb NOT NULL DEFAULT '{}',
 model_correlation_id varchar(128) NOT NULL,kev_decision_id varchar(128),kev_record jsonb,
 template_id varchar(128),run_id varchar(128),error_code varchar(128),created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),UNIQUE(world_id,slot_at));
ALTER TABLE public.world_event_generations ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
ALTER TABLE public.world_event_templates ALTER COLUMN source_resource_id DROP NOT NULL;
ALTER TABLE public.world_event_templates ALTER COLUMN source_version DROP NOT NULL;
ALTER TABLE public.world_event_templates ADD COLUMN IF NOT EXISTS generation_id varchar(128) REFERENCES public.world_event_generations(id);
ALTER TABLE public.world_event_runs ALTER COLUMN fluctlight_id DROP NOT NULL;
CREATE OR REPLACE VIEW public.world_npc_effective_presence_sources AS SELECT p.* FROM public.world_npc_presence_runs p WHERE p.source_kind<>'world_event_execution' OR EXISTS(SELECT 1 FROM public.world_event_runs r LEFT JOIN public.life_events e ON e.id=r.life_event_id WHERE r.id=p.source_ref AND r.world_id=p.world_id AND r.status='applied' AND (r.life_event_id IS NULL OR e.status='confirmed'));
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='world_event_template_source') THEN ALTER TABLE public.world_event_templates ADD CONSTRAINT world_event_template_source CHECK((generation_id IS NOT NULL AND source_resource_id IS NULL AND source_version IS NULL) OR (generation_id IS NULL AND source_resource_id IS NOT NULL AND source_version IS NOT NULL)); END IF;
 IF EXISTS(SELECT 1 FROM pg_constraint WHERE conname='kev_decisions_decision_point_check' AND pg_get_constraintdef(oid) NOT LIKE '%world.event.admit%') THEN ALTER TABLE public.kev_decisions DROP CONSTRAINT kev_decisions_decision_point_check; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='kev_decisions_decision_point_check') THEN ALTER TABLE public.kev_decisions ADD CONSTRAINT kev_decisions_decision_point_check CHECK(decision_point IN('runtime.wakeup','runtime.reflection','goal.completion_check','goal.replenish_plan','tools.select','persona.switch','context.select','media.subject_route','media.capture_plan','media.view_plan','world.event.admit')); END IF;
END $$;
`
