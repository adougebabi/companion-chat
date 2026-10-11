package migrations

const MediaKevPreviousHead = "0059_fact_generation_provenance"

// Released 0056 SQL remains immutable. This additive upgrade widens only the
// diagnostic decision-point constraint; existing rows and settings are kept.
const mediaKevSchemaSQL = `
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM pg_constraint
    WHERE conrelid='public.kev_decisions'::regclass
      AND conname='kev_decisions_decision_point_check'
      AND (pg_get_constraintdef(oid) NOT LIKE '%media.subject_route%'
        OR pg_get_constraintdef(oid) NOT LIKE '%media.capture_plan%'
        OR pg_get_constraintdef(oid) NOT LIKE '%media.view_plan%')) THEN
  ALTER TABLE public.kev_decisions DROP CONSTRAINT kev_decisions_decision_point_check;
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint
    WHERE conrelid='public.kev_decisions'::regclass
      AND conname='kev_decisions_decision_point_check') THEN
  ALTER TABLE public.kev_decisions ADD CONSTRAINT kev_decisions_decision_point_check
   CHECK(decision_point IN ('runtime.wakeup','runtime.reflection','goal.completion_check',
    'goal.replenish_plan','tools.select','persona.switch','context.select',
    'media.subject_route','media.capture_plan','media.view_plan'));
 END IF;
END $$;
`
