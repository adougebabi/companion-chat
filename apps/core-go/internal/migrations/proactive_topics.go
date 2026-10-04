package migrations

const proactiveTopicsSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.conversation_proactive_deliveries (
 message_id varchar(128) PRIMARY KEY REFERENCES public.conversation_messages(id) ON DELETE CASCADE,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 conversation_id varchar(128) NOT NULL REFERENCES public.conversations(id),
 topic_key varchar(128) NOT NULL,
 purpose varchar(256) NOT NULL,
 inbound_sequence integer NOT NULL,
 occurred_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_proactive_topic_window ON public.conversation_proactive_deliveries(fluctlight_id,conversation_id,occurred_at DESC);
`
