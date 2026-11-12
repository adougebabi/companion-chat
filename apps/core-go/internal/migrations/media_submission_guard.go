package migrations

// Existing intents cannot prove that an unrecorded renderer job was never
// submitted. New intents start with a clear guard; existing ones stay uncertain.
const mediaSubmissionGuardSQL = `
ALTER TABLE public.media_intents
 ADD COLUMN IF NOT EXISTS renderer_submission_started boolean NOT NULL DEFAULT true;
ALTER TABLE public.media_intents ALTER COLUMN renderer_submission_started
 SET DEFAULT false;
`
