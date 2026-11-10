package migrations

// Nullable values preserve existing provider defaults. These are sampling
// settings, not prompt or output length limits.
const agentModelSamplingSQL = `
ALTER TABLE public.model_roles ADD COLUMN IF NOT EXISTS temperature double precision
 CHECK (temperature IS NULL OR (temperature >= 0 AND temperature <= 2));
ALTER TABLE public.model_roles ADD COLUMN IF NOT EXISTS top_p double precision
 CHECK (top_p IS NULL OR (top_p > 0 AND top_p <= 1));
`
