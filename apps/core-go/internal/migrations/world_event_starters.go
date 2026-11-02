package migrations

// Starter templates are explicit configuration, never evidence that an event
// occurred. Keep immutable Studio provenance and require Owner enablement.
const worldEventStartersSQL = `
CREATE OR REPLACE FUNCTION public.seed_world_event_starters(target_world text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE w record; starter record; resource text; template text; definition jsonb; body jsonb;
BEGIN
 SELECT id,owner_actor_id INTO STRICT w FROM public.life_worlds WHERE id=target_world;
 FOR starter IN SELECT * FROM (VALUES
  ('light_rain','短暂小雨','天空落下短暂的小雨。','weather','小雨'),
  ('quiet_evening','宁静片刻','周围暂时安静下来。','atmosphere','宁静')
 ) AS s(key,name,description,kind,value) LOOP
  resource:='resource_world_starter_'||md5(w.id||':'||starter.key);
  template:='world_template_starter_'||md5(w.id||':'||starter.key);
  definition:=jsonb_build_object('schema_version','world-event.v1','description',starter.description,
   'category','environment','intensity','low','duration_minutes',15,
   'trigger',jsonb_build_object('mode','manual','interval_minutes',60,'cooldown_minutes',15,'probability',1),
   'condition',jsonb_build_object('day','daily','start','00:00','end','00:00','place_ids','[]'::jsonb,'require_presence',false),
   'effects',jsonb_build_array(jsonb_build_object('kind',starter.kind,'scope','world','value',starter.value)));
  body:=jsonb_build_object('kind','event_template','name',starter.name,'body',definition,
   'draft_version','v1','published_version','v1','versions',jsonb_build_array(jsonb_build_object('version','v1','body',definition)));
  INSERT INTO public.owner_surface_records(id,owner_actor_id,kind,resource_key,status,payload,revision)
   VALUES(resource,w.owner_actor_id,'studio_resource',resource,'published',body,1) ON CONFLICT DO NOTHING;
  INSERT INTO public.studio_resource_versions(resource_id,version,owner_actor_id,kind,name,body)
   VALUES(resource,'v1',w.owner_actor_id,'event_template',starter.name,definition) ON CONFLICT DO NOTHING;
  INSERT INTO public.world_event_templates(id,world_id,source_resource_id,source_version,name,definition,enabled,next_check_at)
   VALUES(template,w.id,resource,'v1',starter.name,definition,false,now()) ON CONFLICT DO NOTHING;
 END LOOP;
END $$;
`
