package migrations

// Owner edits reuse actor_facts; this table records atomic batch replay only.
const actorUserBackgroundSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.actor_user_background_commands (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 idempotency_key varchar(256) NOT NULL,
 actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 request_digest varchar(32) NOT NULL,
 result jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,idempotency_key)
);
`
