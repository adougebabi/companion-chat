package migrations

// Owner-surface rows hold owner preferences, drafts, and requests.
// They are not life facts, memories, persona state, or agent decisions.
const ownerSurfaceSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.owner_surface_records (
  id varchar(128) PRIMARY KEY,
  owner_actor_id varchar(128) NOT NULL,
  fluctlight_id varchar(128) NOT NULL DEFAULT '',
  kind varchar(64) NOT NULL,
  resource_key varchar(256) NOT NULL DEFAULT '',
  status varchar(32) NOT NULL,
  payload jsonb NOT NULL,
  revision integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (owner_actor_id, fluctlight_id, kind, resource_key)
);
CREATE INDEX IF NOT EXISTS ix_owner_surface_records_list
  ON public.owner_surface_records(owner_actor_id, kind, created_at DESC, id DESC);
CREATE TABLE IF NOT EXISTS public.owner_surface_idempotency (
  owner_actor_id varchar(128) NOT NULL,
  idempotency_key varchar(256) NOT NULL,
  digest varchar(128) NOT NULL,
  response jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (owner_actor_id, idempotency_key)
);
CREATE TABLE IF NOT EXISTS public.owner_surface_audits (
  id varchar(128) PRIMARY KEY,
  owner_actor_id varchar(128) NOT NULL,
  fluctlight_id varchar(128) NOT NULL DEFAULT '',
  identity varchar(32) NOT NULL,
  actor varchar(32) NOT NULL,
  action varchar(64) NOT NULL,
  reason text NOT NULL DEFAULT '',
  record_id varchar(128) NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
`
