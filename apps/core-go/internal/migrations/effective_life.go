package migrations

// Effective life facts are separate from their immutable initialization source.
// Every table is Fluctlight-scoped; profile-specific habits use an explicit
// profile id, while one body and one wardrobe survive persona switches.
const effectiveLifeSchemaSQL = `
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS context_stale_at_completion boolean;
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS capture_body_revision integer;
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS capture_wardrobe_revision integer;
DO $effective_life$
BEGIN
 IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_fluctlight_intention_revision_authority_v2'
    AND conrelid='public.fluctlight_intention_revisions'::regclass
    AND pg_get_constraintdef(oid) NOT LIKE '%''start''%') THEN
   ALTER TABLE public.fluctlight_intention_revisions DROP CONSTRAINT ck_fluctlight_intention_revision_authority_v2;
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_fluctlight_intention_revision_authority_v2'
    AND conrelid='public.fluctlight_intention_revisions'::regclass) THEN
   ALTER TABLE public.fluctlight_intention_revisions ADD CONSTRAINT ck_fluctlight_intention_revision_authority_v2 CHECK (
    operation IN ('create','update','qualify','mark_due','start','retry','pause','resume','complete','expire','cancel')
    AND base_revision>=0 AND jsonb_typeof(snapshot)='object' AND jsonb_typeof(evidence_refs)='array'
    AND btrim(idempotency_key)<>'' AND btrim(policy_version)<>'' AND request_digest ~ '^[a-f0-9]{32}$'
   );
 END IF;
END
$effective_life$;
CREATE TABLE IF NOT EXISTS public.fluctlight_appearance_states (
 fluctlight_id varchar(128) PRIMARY KEY REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 revision integer NOT NULL DEFAULT 0 CHECK (revision >= 0),
 state_json jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(state_json)='object'),
 source_kind varchar(32) NOT NULL DEFAULT 'unknown',
 source_ref varchar(128),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS public.fluctlight_appearance_revisions (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 revision integer NOT NULL CHECK (revision >= 0),
 state_json jsonb NOT NULL CHECK (jsonb_typeof(state_json)='object'),
 source_kind varchar(32) NOT NULL,
 source_ref varchar(128),
 occurred_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,revision)
);
CREATE TABLE IF NOT EXISTS public.fluctlight_wardrobe_states (
 fluctlight_id varchar(128) PRIMARY KEY REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 revision integer NOT NULL DEFAULT 0 CHECK (revision >= 0),
 inventory_complete boolean NOT NULL DEFAULT false,
 wearing_state varchar(16) NOT NULL DEFAULT 'unknown' CHECK (wearing_state IN ('unknown','known')),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS public.fluctlight_wardrobe_items (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 category varchar(64) NOT NULL CHECK (length(trim(category))>0),
 slot varchar(64) NOT NULL CHECK (length(trim(slot))>0),
 description text NOT NULL CHECK (length(trim(description))>0),
 ownership varchar(16) NOT NULL CHECK (ownership IN ('owned','borrowed','unknown')),
 availability varchar(16) NOT NULL CHECK (availability IN ('available','unavailable','lost')),
 source_kind varchar(32) NOT NULL CHECK (source_kind IN ('initialization','purchase_result','gift_event','accepted_event')),
 source_ref varchar(128) NOT NULL CHECK (length(trim(source_ref))>0),
 source_item_key varchar(128) NOT NULL CHECK (length(trim(source_item_key))>0),
 revision integer NOT NULL DEFAULT 1 CHECK (revision >= 1),
 acquired_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(fluctlight_id,source_kind,source_ref,source_item_key),
 UNIQUE(fluctlight_id,id)
);
CREATE INDEX IF NOT EXISTS ix_wardrobe_items_lookup ON public.fluctlight_wardrobe_items(fluctlight_id,availability,category,id);
CREATE TABLE IF NOT EXISTS public.fluctlight_wardrobe_outfits (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 profile_id varchar(128) NOT NULL DEFAULT '',
 name varchar(128) NOT NULL CHECK (length(trim(name))>0),
 revision integer NOT NULL DEFAULT 1 CHECK (revision >= 1),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(fluctlight_id,id)
);
CREATE TABLE IF NOT EXISTS public.fluctlight_wardrobe_outfit_items (
 fluctlight_id varchar(128) NOT NULL,
 outfit_id varchar(128) NOT NULL,
 slot varchar(64) NOT NULL,
 item_id varchar(128) NOT NULL,
 PRIMARY KEY(fluctlight_id,outfit_id,slot),
 FOREIGN KEY(fluctlight_id,outfit_id) REFERENCES public.fluctlight_wardrobe_outfits(fluctlight_id,id) ON DELETE CASCADE,
 FOREIGN KEY(fluctlight_id,item_id) REFERENCES public.fluctlight_wardrobe_items(fluctlight_id,id)
);
CREATE TABLE IF NOT EXISTS public.fluctlight_worn_items (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 slot varchar(64) NOT NULL,
 item_id varchar(128) NOT NULL,
 changed_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,slot),
 FOREIGN KEY(fluctlight_id,item_id) REFERENCES public.fluctlight_wardrobe_items(fluctlight_id,id)
);
CREATE TABLE IF NOT EXISTS public.fluctlight_profile_habits (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 profile_id varchar(128) NOT NULL,
 revision integer NOT NULL DEFAULT 0 CHECK (revision >= 0),
 habits_json jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(habits_json)='array'),
 source_kind varchar(32) NOT NULL DEFAULT 'initialization',
 source_ref varchar(128),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,profile_id)
);
CREATE TABLE IF NOT EXISTS public.fluctlight_profile_habit_revisions (
 fluctlight_id varchar(128) NOT NULL,
 profile_id varchar(128) NOT NULL,
 revision integer NOT NULL CHECK (revision >= 0),
 habits_json jsonb NOT NULL CHECK (jsonb_typeof(habits_json)='array'),
 source_kind varchar(32) NOT NULL,
 source_ref varchar(128),
 occurred_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,profile_id,revision),
 FOREIGN KEY(fluctlight_id,profile_id) REFERENCES public.fluctlight_profile_habits(fluctlight_id,profile_id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS public.fluctlight_life_activity_runs (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 profile_id varchar(128) NOT NULL,
 kind varchar(32) NOT NULL CHECK (kind IN ('virtual_shopping','haircut')),
 status varchar(16) NOT NULL CHECK (status IN ('scheduled','in_progress','completed','failed','cancelled','deferred')),
 intention_id varchar(128) REFERENCES public.fluctlight_intentions(id),
 source_event_id varchar(128),
 scheduled_at timestamptz NOT NULL,
 started_at timestamptz,
 not_before timestamptz NOT NULL,
 resolved_at timestamptz,
 result_json jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(result_json)='object'),
 revision integer NOT NULL DEFAULT 1 CHECK (revision >= 1),
 operation_id varchar(256) NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(fluctlight_id,operation_id)
);
CREATE INDEX IF NOT EXISTS ix_life_activity_due ON public.fluctlight_life_activity_runs(status,scheduled_at);
`
