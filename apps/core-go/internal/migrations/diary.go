package migrations

const diarySchemaSQL = `
CREATE TABLE IF NOT EXISTS public.fluctlight_diary_days (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 local_date date NOT NULL,
 timezone varchar(128) NOT NULL,
 status varchar(32) NOT NULL CHECK(status IN ('pending','running','ready','no_content','failed','inactive')),
 revision integer NOT NULL DEFAULT 1,
 active_attempt_id varchar(128),
 latest_attempt integer NOT NULL DEFAULT 1,
 last_error_code varchar(128),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(fluctlight_id,local_date,timezone)
);
CREATE TABLE IF NOT EXISTS public.fluctlight_diary_attempts (
 id varchar(128) PRIMARY KEY,
 diary_id varchar(128) NOT NULL REFERENCES public.fluctlight_diary_days(id),
 attempt integer NOT NULL,
 status varchar(32) NOT NULL CHECK(status IN ('pending','running','ready','no_content','failed','inactive')),
 source_snapshot jsonb,
 source_digest varchar(128),
 plan jsonb,
 body text,
 persona_snapshot jsonb,
 model_provenance jsonb NOT NULL DEFAULT '{}',
 error_code varchar(128),
 created_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 UNIQUE(diary_id,attempt)
);
CREATE TABLE IF NOT EXISTS public.fluctlight_diary_annotations (
 diary_id varchar(128) NOT NULL REFERENCES public.fluctlight_diary_days(id),
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 favorite boolean NOT NULL DEFAULT false,
 note text NOT NULL DEFAULT '',
 revision integer NOT NULL DEFAULT 1,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(diary_id,owner_actor_id)
);
CREATE INDEX IF NOT EXISTS ix_diary_days_page ON public.fluctlight_diary_days(fluctlight_id,local_date DESC,id DESC);
CREATE INDEX IF NOT EXISTS ix_diary_attempts_page ON public.fluctlight_diary_attempts(diary_id,attempt DESC);
`
