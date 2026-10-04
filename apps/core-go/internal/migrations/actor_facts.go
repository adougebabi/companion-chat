package migrations

// Attribute-level Actor assertions preserve raw messages and distinguish a
// corrected mistake from a genuine subsequent change. Derived projections
// link to the precise assertion revision instead of invalidating whole chats.
const actorFactsSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.actor_facts (
 id varchar(128) PRIMARY KEY,
 owner_fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 subject_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 attribute varchar(64) NOT NULL CHECK (attribute ~ '^[a-z][a-z0-9_]{0,63}$'),
 value_json jsonb NOT NULL,
 epistemic_kind varchar(24) NOT NULL CHECK (epistemic_kind IN ('user_statement','owner_defined','inference')),
 status varchar(24) NOT NULL CHECK (status IN ('active','uncertain','corrected','quarantined')),
 valid_from timestamptz NOT NULL,
 valid_until timestamptz,
 source_message_id varchar(128) REFERENCES public.conversation_messages(id),
 source_ref varchar(256) NOT NULL,
 source_fingerprint varchar(32) NOT NULL,
 revision integer NOT NULL DEFAULT 1 CHECK (revision>0),
 replaced_by varchar(128),
 operation_id varchar(256) NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK (valid_until IS NULL OR valid_until>valid_from),
 CHECK ((epistemic_kind='inference' AND status<>'active') OR epistemic_kind<>'inference'),
 UNIQUE(owner_fluctlight_id,operation_id)
);
ALTER TABLE public.actor_facts ADD COLUMN IF NOT EXISTS transition_kind varchar(16) NOT NULL DEFAULT 'assert' CHECK (transition_kind IN ('assert','correct','change'));
CREATE UNIQUE INDEX IF NOT EXISTS ix_actor_fact_current ON public.actor_facts(owner_fluctlight_id,subject_actor_id,attribute)
 WHERE status='active' AND valid_until IS NULL;
CREATE INDEX IF NOT EXISTS ix_actor_fact_source ON public.actor_facts(source_message_id);
CREATE TABLE IF NOT EXISTS public.actor_fact_artifacts (
 fact_id varchar(128) NOT NULL REFERENCES public.actor_facts(id),
 fact_revision integer NOT NULL,
 artifact_kind varchar(32) NOT NULL CHECK (artifact_kind IN ('memory','summary','developing_self','active_memory','evolution_overlay')),
 artifact_id varchar(128) NOT NULL,
 artifact_revision integer NOT NULL,
 PRIMARY KEY(fact_id,artifact_kind,artifact_id,artifact_revision)
);
DROP TRIGGER IF EXISTS context_generation_bump ON public.actor_facts;
CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.actor_facts
 FOR EACH ROW EXECUTE FUNCTION public.bump_context_owner_trigger();
`
