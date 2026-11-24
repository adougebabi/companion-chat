package migrations

// Persist the scheduled local date and the actual (possibly merged) request.
// No Goal/source facts are invented by the daily clock.
const goalDailyReviewSchemaSQL = `
ALTER TABLE public.goal_agent_cadence
 ADD COLUMN IF NOT EXISTS daily_review_date date,
 ADD COLUMN IF NOT EXISTS daily_review_request_id text;
`
