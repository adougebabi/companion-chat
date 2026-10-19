package migrations

const momentFeedSchemaSQL = `
ALTER TABLE public.moments ALTER COLUMN owner_fluctlight_id DROP NOT NULL;
ALTER TABLE public.moments ADD COLUMN IF NOT EXISTS owner_actor_id varchar(128) REFERENCES public.actors(id);
ALTER TABLE public.moments ADD COLUMN IF NOT EXISTS revision integer NOT NULL DEFAULT 1 CHECK(revision>=1);
ALTER TABLE public.moments ADD COLUMN IF NOT EXISTS owner_request_id varchar(128) REFERENCES public.owner_surface_records(id);
ALTER TABLE public.moments ADD COLUMN IF NOT EXISTS source_event_id varchar(128) REFERENCES public.life_events(id);
UPDATE public.moments m SET owner_actor_id=f.created_by_actor_id FROM public.fluctlights f WHERE f.id=m.owner_fluctlight_id AND m.owner_actor_id IS NULL;
-- Preserve the original Owner publication identity and timestamp. Old global
-- posts lived only in surface records and could never be commented on.
INSERT INTO public.moments(id,owner_actor_id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids,created_at)
SELECT r.id,r.owner_actor_id,NULLIF(r.fluctlight_id,''),r.owner_actor_id,r.payload->>'text','owner',r.status,COALESCE(r.payload->'media_ids','[]'::jsonb),r.created_at
FROM public.owner_surface_records r WHERE r.kind='user_moment' AND NOT EXISTS(SELECT 1 FROM public.moments m WHERE m.id=r.id);
ALTER TABLE public.moments ALTER COLUMN owner_actor_id SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS moments_owner_request_unique ON public.moments(owner_request_id) WHERE owner_request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS moments_owner_feed ON public.moments(owner_actor_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS moment_comments_page ON public.moment_comments(moment_id,created_at,id);
CREATE OR REPLACE FUNCTION public.moment_owner_scope() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE actual_owner varchar(128);
BEGIN
 IF NEW.owner_fluctlight_id IS NOT NULL THEN
  SELECT created_by_actor_id INTO STRICT actual_owner FROM public.fluctlights WHERE id=NEW.owner_fluctlight_id;
  IF NEW.owner_actor_id IS NULL THEN NEW.owner_actor_id:=actual_owner; END IF;
  IF NEW.owner_actor_id<>actual_owner THEN RAISE EXCEPTION 'moment_owner_scope_mismatch'; END IF;
 ELSIF NEW.author_actor_id<>NEW.owner_actor_id THEN
  RAISE EXCEPTION 'global_moment_author_must_be_owner';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS moment_owner_scope ON public.moments;
CREATE TRIGGER moment_owner_scope BEFORE INSERT OR UPDATE OF owner_actor_id,owner_fluctlight_id,author_actor_id ON public.moments FOR EACH ROW EXECUTE FUNCTION public.moment_owner_scope();
`
