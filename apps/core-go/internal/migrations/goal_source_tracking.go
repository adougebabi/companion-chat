package migrations

// Journal changes at the owning source transaction, including changes made by
// established domain services. Withdrawal never rewrites prior speech/history.
const goalSourceTrackingSchemaSQL = `
CREATE OR REPLACE FUNCTION public.track_goal_source_change() RETURNS trigger LANGUAGE plpgsql AS $goal_sources$
DECLARE old_value jsonb; new_value jsonb; kind text; owner_id text; subject_id text;
 version_value text; event_id bigint; affected record; request_id text; actual_request text;
BEGIN
 kind:=TG_ARGV[0];
 IF TG_OP<>'INSERT' THEN old_value:=to_jsonb(OLD); END IF;
 IF TG_OP<>'DELETE' THEN new_value:=to_jsonb(NEW); END IF;
 IF kind='message' THEN
  IF TG_OP='UPDATE' AND NEW.text IS NOT DISTINCT FROM OLD.text AND NEW.attachment_refs IS NOT DISTINCT FROM OLD.attachment_refs THEN RETURN NEW; END IF;
 ELSE
  IF TG_OP='UPDATE' AND new_value IS NOT DISTINCT FROM old_value THEN RETURN NEW; END IF;
 END IF;
 IF TG_OP<>'INSERT' THEN
  FOR affected IN SELECT DISTINCT fluctlight_id,COALESCE(profile_id,'') profile_id,conversation_id,subject_actor_id,source_version,occurred_at
   FROM public.goal_source_events WHERE source_kind=kind AND source_id=old_value->>'id' AND source_status='committed' LOOP
   INSERT INTO public.goal_source_events(fluctlight_id,profile_id,source_kind,source_id,source_version,conversation_id,subject_actor_id,source_status,occurred_at)
    VALUES(affected.fluctlight_id,NULLIF(affected.profile_id,''),kind,old_value->>'id',affected.source_version,affected.conversation_id,affected.subject_actor_id,'withdrawn',affected.occurred_at)
    ON CONFLICT(fluctlight_id,source_kind,source_id,source_version,source_status) DO UPDATE SET source_id=EXCLUDED.source_id RETURNING id INTO event_id;
   UPDATE public.goal_evidence_links SET status='withdrawn',reason='authoritative source changed',updated_at=now()
    WHERE source_event_id IN (SELECT id FROM public.goal_source_events WHERE fluctlight_id=affected.fluctlight_id AND source_kind=kind AND source_id=old_value->>'id');
   INSERT INTO public.goal_evidence_review_flags(goal_id,source_event_id,reason)
    SELECT DISTINCT l.goal_id,event_id,'ended Goal source changed; review required' FROM public.goal_evidence_links l
    JOIN public.goal_source_events e ON e.id=l.source_event_id JOIN public.fluctlight_goals g ON g.id=l.goal_id
    WHERE e.fluctlight_id=affected.fluctlight_id AND e.source_kind=kind AND e.source_id=old_value->>'id' AND g.status IN ('completed','cancelled','abandoned') ON CONFLICT DO NOTHING;
   FOR owner_id IN SELECT DISTINCT g.id FROM public.fluctlight_goals g JOIN public.goal_evidence_links l ON l.goal_id=g.id JOIN public.goal_source_events e ON e.id=l.source_event_id
    WHERE e.fluctlight_id=affected.fluctlight_id AND e.source_kind=kind AND e.source_id=old_value->>'id' AND g.status IN ('active','paused') LOOP
    request_id:='goal_evaluation_' || substr(encode(digest('withdraw:' || event_id::text || ':' || owner_id,'sha256'),'hex'),1,32);
    INSERT INTO public.goal_evaluation_requests(id,fluctlight_id,profile_id,goal_ids,reason,status)
     SELECT request_id,g.fluctlight_id,COALESCE(g.profile_id,''),jsonb_build_array(g.id),'source_withdrawal','pending' FROM public.fluctlight_goals g WHERE g.id=owner_id
     ON CONFLICT(fluctlight_id,profile_id) WHERE status='pending' DO UPDATE SET goal_ids=goal_evaluation_requests.goal_ids || EXCLUDED.goal_ids RETURNING id INTO actual_request;
    INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload)
     SELECT actual_request,'goal-evaluation:' || actual_request,'lifecycle','goal.evaluate',jsonb_build_object('fluctlight_id',fluctlight_id,'evaluation_id',actual_request)
     FROM public.goal_evaluation_requests WHERE id=actual_request ON CONFLICT(intent_id) DO NOTHING;
   END LOOP;
  END LOOP;
 END IF;
 IF kind<>'message' AND TG_OP<>'DELETE' THEN
  owner_id:=COALESCE(new_value->>'owner_fluctlight_id',new_value->>'fluctlight_id');
  subject_id:=COALESCE(new_value->>'subject_actor_id',owner_id);
  version_value:=CASE WHEN kind IN ('activity','actor_fact') THEN new_value->>'revision' ELSE md5(new_value::text) END;
  INSERT INTO public.goal_source_events(fluctlight_id,source_kind,source_id,source_version,subject_actor_id,source_status,occurred_at)
   VALUES(owner_id,kind,new_value->>'id',version_value,subject_id,'committed',COALESCE((new_value->>'resolved_at')::timestamptz,(new_value->>'created_at')::timestamptz,now())) ON CONFLICT DO NOTHING;
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $goal_sources$;
DROP TRIGGER IF EXISTS goal_message_source_change ON public.conversation_messages;
CREATE TRIGGER goal_message_source_change AFTER UPDATE OF text,attachment_refs OR DELETE ON public.conversation_messages FOR EACH ROW EXECUTE FUNCTION public.track_goal_source_change('message');
DROP TRIGGER IF EXISTS goal_actor_fact_source_change ON public.actor_facts;
CREATE TRIGGER goal_actor_fact_source_change AFTER INSERT OR UPDATE OR DELETE ON public.actor_facts FOR EACH ROW EXECUTE FUNCTION public.track_goal_source_change('actor_fact');
DROP TRIGGER IF EXISTS goal_item_source_change ON public.fluctlight_wardrobe_items;
CREATE TRIGGER goal_item_source_change AFTER INSERT OR UPDATE OR DELETE ON public.fluctlight_wardrobe_items FOR EACH ROW EXECUTE FUNCTION public.track_goal_source_change('item');
DROP TRIGGER IF EXISTS goal_activity_source_change ON public.fluctlight_life_activity_runs;
CREATE TRIGGER goal_activity_source_change AFTER INSERT OR UPDATE OR DELETE ON public.fluctlight_life_activity_runs FOR EACH ROW EXECUTE FUNCTION public.track_goal_source_change('activity');
`
