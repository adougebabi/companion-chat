package migrations

const studioJobsSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.studio_jobs (
 id varchar(128) PRIMARY KEY,owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 resource_id varchar(128) NOT NULL REFERENCES public.owner_surface_records(id),resource_revision integer NOT NULL CHECK(resource_revision>0),
 kind varchar(32) NOT NULL CHECK(kind IN ('assistant','text_preview')),snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object'),source_digest varchar(128) NOT NULL,
 status varchar(16) NOT NULL CHECK(status IN ('queued','running','completed','failed','discarded')),revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 latest_attempt integer NOT NULL DEFAULT 1 CHECK(latest_attempt>0),result jsonb NOT NULL DEFAULT '{}',error_code varchar(128),
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),UNIQUE(owner_actor_id,id)
);
CREATE TABLE IF NOT EXISTS public.studio_job_attempts (
 id varchar(128) PRIMARY KEY,job_id varchar(128) NOT NULL REFERENCES public.studio_jobs(id),attempt integer NOT NULL CHECK(attempt>0),
 status varchar(16) NOT NULL CHECK(status IN ('queued','running','completed','failed','discarded')),output jsonb NOT NULL DEFAULT '{}',
 error_code varchar(128),provenance jsonb NOT NULL DEFAULT '{}',created_at timestamptz NOT NULL DEFAULT now(),finished_at timestamptz,UNIQUE(job_id,attempt)
);
CREATE TABLE IF NOT EXISTS public.studio_resource_materials (
 id varchar(128) PRIMARY KEY,owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 resource_id varchar(128) NOT NULL REFERENCES public.owner_surface_records(id),job_id varchar(128) NOT NULL UNIQUE REFERENCES public.studio_jobs(id),
 source_digest varchar(128) NOT NULL,text text NOT NULL CHECK(length(btrim(text))>0),label text NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_studio_jobs_owner_resource ON public.studio_jobs(owner_actor_id,resource_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS ix_studio_materials_resource ON public.studio_resource_materials(owner_actor_id,resource_id,created_at DESC,id DESC);
`
