package migrations

const notificationCenterSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.owner_notifications (
 id varchar(128) PRIMARY KEY,
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 fluctlight_id varchar(128) REFERENCES public.fluctlights(id),
 source_event_id varchar(128) NOT NULL,
 source_kind varchar(128) NOT NULL,
 source_id varchar(256) NOT NULL,
 source_occurred_at timestamptz NOT NULL,
 title text NOT NULL CHECK(length(title)>0),
 body text NOT NULL,
 link text NOT NULL CHECK(link LIKE '#/%'),
 quiet boolean NOT NULL DEFAULT false,
 read_at timestamptz,
 metadata jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(metadata)='object'),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(owner_actor_id,source_event_id)
);
CREATE INDEX IF NOT EXISTS ix_owner_notifications_page ON public.owner_notifications(owner_actor_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS ix_owner_notifications_unread ON public.owner_notifications(owner_actor_id,quiet,created_at DESC,id DESC) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_owner_notifications_fluctlight ON public.owner_notifications(owner_actor_id,fluctlight_id,created_at DESC,id DESC);
`
