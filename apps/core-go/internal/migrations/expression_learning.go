package migrations

// Expression habits are reviewable presentation suggestions, not persona or
// fact authority. Their source identity remains with the original message.
const expressionLearningSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.expression_learning_source_versions (
 message_id varchar(128) PRIMARY KEY,
 revision bigint NOT NULL DEFAULT 0 CHECK (revision>=0)
);
CREATE OR REPLACE FUNCTION public.bump_expression_learning_source_version()
RETURNS trigger LANGUAGE plpgsql AS $expression_source_version$
DECLARE source_id text;
BEGIN
 IF TG_OP='UPDATE' AND OLD.text IS NOT DISTINCT FROM NEW.text AND OLD.attachment_refs IS NOT DISTINCT FROM NEW.attachment_refs THEN RETURN NEW; END IF;
 IF TG_OP='DELETE' THEN source_id := OLD.id; ELSE source_id := NEW.id; END IF;
 INSERT INTO public.expression_learning_source_versions(message_id,revision) VALUES(source_id,1)
 ON CONFLICT(message_id) DO UPDATE SET revision=public.expression_learning_source_versions.revision+1;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END
$expression_source_version$;
DROP TRIGGER IF EXISTS expression_learning_source_version ON public.conversation_messages;
CREATE TRIGGER expression_learning_source_version AFTER UPDATE OF text,attachment_refs OR DELETE ON public.conversation_messages
 FOR EACH ROW EXECUTE FUNCTION public.bump_expression_learning_source_version();
CREATE TABLE IF NOT EXISTS public.expression_learning_settings (
 fluctlight_id varchar(128) PRIMARY KEY REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 learning_enabled boolean NOT NULL DEFAULT false,
 use_enabled boolean NOT NULL DEFAULT false,
 revision integer NOT NULL DEFAULT 0 CHECK (revision>=0),
 learning_generation bigint NOT NULL DEFAULT 0 CHECK (learning_generation>=0),
 use_generation bigint NOT NULL DEFAULT 0 CHECK (use_generation>=0),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS public.expression_learning_candidates (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 owner_actor_id varchar(128) NOT NULL,
 source_actor_id varchar(128) NOT NULL,
 source_conversation_id varchar(128) NOT NULL,
 source_message_id varchar(128) NOT NULL,
 source_fingerprint varchar(32) NOT NULL CHECK (source_fingerprint ~ '^[a-f0-9]{32}$'),
 source_revision bigint NOT NULL CHECK (source_revision>=0),
 excerpt text NOT NULL CHECK (btrim(excerpt)<>''),
 situation text NOT NULL CHECK (btrim(situation)<>''),
 trigger_text text NOT NULL DEFAULT '',
 status varchar(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','enabled','disabled','revoked')),
 revision integer NOT NULL DEFAULT 1 CHECK (revision>=1),
 dedupe_key varchar(32) NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(fluctlight_id,dedupe_key)
);
CREATE INDEX IF NOT EXISTS ix_expression_candidates_owner ON public.expression_learning_candidates(fluctlight_id,owner_actor_id,status,updated_at DESC,id);
CREATE TABLE IF NOT EXISTS public.expression_learning_reviews (
 id varchar(128) PRIMARY KEY,
 candidate_id varchar(128) NOT NULL REFERENCES public.expression_learning_candidates(id) ON DELETE CASCADE,
 candidate_revision integer NOT NULL,
 owner_actor_id varchar(128) NOT NULL,
 action varchar(16) NOT NULL CHECK (action IN ('enable','disable','revoke')),
 reason text NOT NULL,
 idempotency_key varchar(256) NOT NULL,
 request_digest varchar(32) NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(owner_actor_id,idempotency_key),
 UNIQUE(candidate_id,candidate_revision)
);
CREATE TABLE IF NOT EXISTS public.expression_learning_extractions (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 owner_actor_id varchar(128) NOT NULL,
 source_message_id varchar(128) NOT NULL,
 source_fingerprint varchar(32) NOT NULL CHECK (source_fingerprint ~ '^[a-f0-9]{32}$'),
 source_revision bigint NOT NULL CHECK (source_revision>=0),
 learning_generation bigint NOT NULL CHECK (learning_generation>=0),
 idempotency_key varchar(256) NOT NULL,
 request_digest varchar(32) NOT NULL,
 status varchar(16) NOT NULL CHECK (status IN ('running','completed','failed')),
 lease_owner varchar(128),
 lease_until timestamptz,
 binding_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
 result jsonb NOT NULL DEFAULT '{}'::jsonb,
 last_error varchar(128),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(owner_actor_id,idempotency_key)
);
`
