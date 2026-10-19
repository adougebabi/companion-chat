package migrations

const configurationCommandsSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.configuration_revisions (
 resource_key varchar(256) PRIMARY KEY, revision bigint NOT NULL CHECK(revision>0)
);
INSERT INTO public.configuration_revisions(resource_key,revision) SELECT 'secret:'||purpose,1 FROM public.setting_secrets ON CONFLICT DO NOTHING;
INSERT INTO public.configuration_revisions(resource_key,revision) SELECT 'endpoint:'||id,1 FROM public.provider_endpoints ON CONFLICT DO NOTHING;
INSERT INTO public.configuration_revisions(resource_key,revision) SELECT 'binding:'||role,1 FROM public.model_roles ON CONFLICT DO NOTHING;
CREATE OR REPLACE FUNCTION public.bump_configuration_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE entity jsonb; resource text;
BEGIN
 IF TG_OP='UPDATE' THEN
  IF TG_ARGV[0]='endpoint' AND (to_jsonb(OLD)->'kind',to_jsonb(OLD)->'base_url',to_jsonb(OLD)->'secret_purpose') IS NOT DISTINCT FROM (to_jsonb(NEW)->'kind',to_jsonb(NEW)->'base_url',to_jsonb(NEW)->'secret_purpose') THEN RETURN NEW; END IF;
  IF TG_ARGV[0]='binding' AND (to_jsonb(OLD)-'updated_at')=(to_jsonb(NEW)-'updated_at') THEN RETURN NEW; END IF;
  IF TG_ARGV[0]='secret' AND (to_jsonb(OLD)->'ciphertext',to_jsonb(OLD)->'nonce') IS NOT DISTINCT FROM (to_jsonb(NEW)->'ciphertext',to_jsonb(NEW)->'nonce') THEN RETURN NEW; END IF;
 END IF;
 IF TG_OP='DELETE' THEN entity:=to_jsonb(OLD); ELSE entity:=to_jsonb(NEW); END IF;
 resource:=TG_ARGV[0]||':'||(entity->>TG_ARGV[1]);
 INSERT INTO public.configuration_revisions(resource_key,revision) VALUES(resource,1)
 ON CONFLICT(resource_key) DO UPDATE SET revision=configuration_revisions.revision+1;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF; RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS configuration_revision ON public.setting_secrets;
CREATE TRIGGER configuration_revision AFTER INSERT OR UPDATE OR DELETE ON public.setting_secrets FOR EACH ROW EXECUTE FUNCTION public.bump_configuration_revision('secret','purpose');
DROP TRIGGER IF EXISTS configuration_revision ON public.provider_endpoints;
CREATE TRIGGER configuration_revision AFTER INSERT OR UPDATE OR DELETE ON public.provider_endpoints FOR EACH ROW EXECUTE FUNCTION public.bump_configuration_revision('endpoint','id');
DROP TRIGGER IF EXISTS configuration_revision ON public.model_roles;
CREATE TRIGGER configuration_revision AFTER INSERT OR UPDATE OR DELETE ON public.model_roles FOR EACH ROW EXECUTE FUNCTION public.bump_configuration_revision('binding','role');
`
