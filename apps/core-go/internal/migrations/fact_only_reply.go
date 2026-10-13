package migrations

const FactOnlyReplyPreviousHead = "0060_media_kev"

// Private checkpoints are not chat facts; released columns and enum meanings
// remain intact so the stable image can read its existing data on rollback.
const factOnlyReplySchemaSQL = `
ALTER TABLE public.conversation_messages ADD COLUMN IF NOT EXISTS reply_metadata jsonb NOT NULL DEFAULT '{}';
CREATE TABLE IF NOT EXISTS public.conversation_reply_drafts (
 id varchar(128) PRIMARY KEY,
 logical_key varchar(128) NOT NULL UNIQUE,
 fluctlight_id varchar(128) NOT NULL,
 conversation_id varchar(128) NOT NULL,
 authorization_actor_id varchar(128) NOT NULL,
 subject_actor_id varchar(128) NOT NULL,
 source_fact_id varchar(128) NOT NULL DEFAULT '',
 profile_id varchar(128) NOT NULL DEFAULT '',
 status varchar(32) NOT NULL,
 payload jsonb NOT NULL DEFAULT '{}',
 config_snapshot jsonb NOT NULL,
 config_version bigint NOT NULL DEFAULT 0,
 binding_snapshot jsonb NOT NULL DEFAULT '{}',
 generation_owner varchar(128),
 generation_until timestamptz,
 resume_attempts integer NOT NULL DEFAULT 0,
 last_error varchar(128),
 result_message_id varchar(128),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK (status IN ('generating','prepared','needs_action','waiting_media','ready','published','failed','cancelled','legacy_fallback_required'))
);
CREATE INDEX IF NOT EXISTS ix_conversation_reply_drafts_pending
 ON public.conversation_reply_drafts(status,updated_at) WHERE status IN ('needs_action','waiting_media','ready');
CREATE INDEX IF NOT EXISTS ix_conversation_reply_drafts_source
 ON public.conversation_reply_drafts(fluctlight_id,conversation_id,source_fact_id);
`
