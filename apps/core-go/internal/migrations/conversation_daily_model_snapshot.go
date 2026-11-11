package migrations

// A new or reopened generation creates its own row in the enqueue transaction.
// Existing generations are not backfilled with today's configuration.
const conversationDailyModelSnapshotSQL = `
CREATE TABLE IF NOT EXISTS public.conversation_daily_model_snapshots (
 intent_id varchar(128) NOT NULL REFERENCES public.platform_workflow_intents(intent_id) ON DELETE CASCADE,
 source_generation bigint NOT NULL CHECK (source_generation >= 0),
 snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
 selected boolean NOT NULL DEFAULT false,
 PRIMARY KEY (intent_id, source_generation)
);
ALTER TABLE public.conversation_daily_model_snapshots
 ADD COLUMN IF NOT EXISTS selected boolean NOT NULL DEFAULT false;
`
