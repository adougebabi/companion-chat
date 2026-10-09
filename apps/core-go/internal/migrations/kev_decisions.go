package migrations

const KevDecisionsPreviousHead = "0055_goal_planner_cadence"

const kevDecisionsSchemaSQL = `
ALTER TABLE public.runtime_settings ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1 CHECK(revision>0);
CREATE TABLE IF NOT EXISTS public.kev_decisions (
 id varchar(128) PRIMARY KEY,
 request_id varchar(128) NOT NULL,
 question_id varchar(128) NOT NULL,
 decision_point varchar(64) NOT NULL CHECK(decision_point IN ('runtime.wakeup','runtime.reflection','goal.completion_check','goal.replenish_plan','tools.select','persona.switch','context.select')),
 actor_self varchar(128) NOT NULL,
 actor_user varchar(128),
 agent varchar(128),
 run_id varchar(256),
 candidate_id text NOT NULL,
 started_at timestamptz NOT NULL,
 completed_at timestamptz NOT NULL,
 payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object'),
 UNIQUE(request_id,question_id)
);
CREATE INDEX IF NOT EXISTS ix_kev_decisions_started ON public.kev_decisions(started_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS ix_kev_decisions_scope ON public.kev_decisions(actor_self,decision_point,started_at DESC,id DESC);
CREATE TABLE IF NOT EXISTS public.kev_deferrals (
 decision_point varchar(64) NOT NULL,
 actor_self varchar(128) NOT NULL,
 candidate_id text NOT NULL,
 deferral_count integer NOT NULL DEFAULT 0 CHECK(deferral_count>=0),
 deferred_until timestamptz NOT NULL,
 PRIMARY KEY(decision_point,actor_self,candidate_id)
);
`
