package migrations

// Explicit Owner requests are durable work, separate from the automatic
// current-day lifecycle. Appointments never invent executable action plans.
const ownerSchedulesSQL = `
CREATE TABLE IF NOT EXISTS public.owner_schedule_requests (
 id text PRIMARY KEY,
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 kind text NOT NULL CHECK(kind IN ('generate','promise')),
 local_date date NOT NULL, timezone text NOT NULL,
 status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','completed','failed','conflict')),
 payload jsonb NOT NULL, snapshot jsonb NOT NULL, proposal jsonb,
 result jsonb NOT NULL DEFAULT '{}', error_code text,
 idempotency_key text NOT NULL, request_digest text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(owner_actor_id,idempotency_key)
);
CREATE UNIQUE INDEX IF NOT EXISTS owner_schedule_one_pending_day ON public.owner_schedule_requests(fluctlight_id,local_date) WHERE status IN ('queued','running');
CREATE INDEX IF NOT EXISTS owner_schedule_scope_date ON public.owner_schedule_requests(owner_actor_id,fluctlight_id,local_date,created_at DESC);
ALTER TABLE public.life_schedule_items ADD COLUMN IF NOT EXISTS promise_request_id text REFERENCES public.owner_promise_links(request_id);
CREATE UNIQUE INDEX IF NOT EXISTS schedule_one_promise_per_version ON public.life_schedule_items(schedule_id,promise_request_id) WHERE promise_request_id IS NOT NULL;
`
