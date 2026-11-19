package migrations

// Evaluations preserve each accepted input and its disposition without
// rewriting legacy claim identity, evidence or expiry during upgrade.
const claimEvidenceSQL = `
CREATE TABLE IF NOT EXISTS public.cognition_claim_evaluations (
 claim_id varchar(128) NOT NULL REFERENCES public.cognition_claims(id) ON DELETE CASCADE,
 assessment_key varchar(128) NOT NULL,
 version integer NOT NULL CHECK(version>=0),
 original_repetition_key text NOT NULL,
 source_fact_id varchar(128) NOT NULL,
 snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 business_at timestamptz,
 applied boolean NOT NULL,
 origin varchar(32) NOT NULL CHECK(origin IN ('legacy_current','evaluated')),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(claim_id,assessment_key),
 UNIQUE(claim_id,version)
);
CREATE INDEX IF NOT EXISTS ix_claim_evaluations_applied
 ON public.cognition_claim_evaluations(claim_id,version DESC) WHERE applied;
CREATE INDEX IF NOT EXISTS ix_claim_evaluations_source
 ON public.cognition_claim_evaluations(source_fact_id,claim_id);
`
