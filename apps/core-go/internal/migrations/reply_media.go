package migrations

const ReplyMediaPreviousHead = "0061_fact_only_reply"

// Separate tables keep stable dispatchers from claiming the new protocol.
const replyMediaSchemaSQL = `
ALTER TABLE public.conversation_reply_drafts ADD COLUMN IF NOT EXISTS execution_scope jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.conversation_reply_drafts ADD COLUMN IF NOT EXISTS expires_at timestamptz NOT NULL DEFAULT now()+interval '1 day';
CREATE TABLE IF NOT EXISTS public.conversation_reply_actions (
 ref varchar(128) PRIMARY KEY,
 draft_id varchar(128) NOT NULL REFERENCES public.conversation_reply_drafts(id),
 request_index integer NOT NULL,
 kind varchar(32) NOT NULL,
 intent text NOT NULL,
 media_intent_id varchar(128) UNIQUE,
 status varchar(32) NOT NULL DEFAULT 'requested',
 error_code varchar(128),
 UNIQUE(draft_id,request_index),
 CHECK(status IN ('requested','bound','unavailable','failed'))
);
CREATE TABLE IF NOT EXISTS public.conversation_reply_continuations (
 draft_id varchar(128) PRIMARY KEY REFERENCES public.conversation_reply_drafts(id),
 workflow_id varchar(128) NOT NULL UNIQUE,
 status varchar(32) NOT NULL DEFAULT 'pending',
 attempt_count integer NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL DEFAULT now(),
 started_at timestamptz,
 completed_at timestamptz,
 last_error varchar(128),
 CHECK(status IN ('pending','started','completed','failed','cancelled'))
);
`
