package migrations

// No default or historical backfill: reference creation is not proof that an
// attachment was first published at that time.
const mediaPublicationTimeSQL = `
ALTER TABLE public.media_references ADD COLUMN IF NOT EXISTS published_at timestamptz;
`
