package migrations

// Compiled portraits are derived from the accepted Core Persona and effective
// stable overlay. A replacement is written with the source in the same short
// transaction, so an old compilation cannot be paired with a newer source.
const workingPersonaSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.fluctlight_working_personas (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 profile_id varchar(128) NOT NULL,
 source_revision integer NOT NULL CHECK (source_revision >= 0),
 source_hash varchar(128) NOT NULL,
 overlay_revision integer NOT NULL CHECK (overlay_revision >= 0),
 rules_version varchar(64) NOT NULL,
 budget_runes integer NOT NULL CHECK (budget_runes BETWEEN 512 AND 12000),
 status varchar(16) NOT NULL CHECK (status='completed'),
 compiled_json jsonb NOT NULL CHECK (jsonb_typeof(compiled_json)='object'),
 compiled_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,profile_id)
);
CREATE INDEX IF NOT EXISTS ix_working_personas_rules ON public.fluctlight_working_personas(rules_version);
ALTER TABLE public.fluctlight_working_personas ADD COLUMN IF NOT EXISTS budget_runes integer NOT NULL DEFAULT 3600;
INSERT INTO public.runtime_settings(key,value_json) VALUES('working_persona_budget','{"max_runes":3600}') ON CONFLICT(key) DO NOTHING;
`
