package migrations

const businessBackupsSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.business_backup_jobs(
 id varchar(128) PRIMARY KEY,owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 kind varchar(16) NOT NULL CHECK(kind IN ('backup','restore')),source_backup_id varchar(128) REFERENCES public.business_backup_jobs(id),
 target_name varchar(96),status varchar(32) NOT NULL CHECK(status IN ('queued','running','completed','failed','cancelled')),
 revision integer NOT NULL DEFAULT 1,attempt integer NOT NULL DEFAULT 1,reason text NOT NULL,
 execution_digest varchar(64),stage varchar(64) NOT NULL DEFAULT 'awaiting_maintenance_operator',
 result jsonb NOT NULL DEFAULT '{}',operator_result jsonb NOT NULL DEFAULT '{}',steps jsonb NOT NULL DEFAULT '[]',
 error_code varchar(128),created_at timestamptz NOT NULL DEFAULT now(),started_at timestamptz,finished_at timestamptz,updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS business_backup_jobs_owner_page ON public.business_backup_jobs(owner_actor_id,created_at DESC,id DESC);
`
