package workflow

import "fmt"

// Only creations admitted with durable persona preparation use this chain.
// Later wardrobe/visual updates do not re-lock an already completed bootstrap.
// Keep the predicate before LIMIT so waiting creations cannot starve ready work.
const creationStageReadySQL = `
  AND (
    NOT EXISTS (SELECT 1 FROM public.platform_workflow_intents init
      WHERE init.intent_type='persona.compile' AND init.payload->>'source'='initialization'
        AND init.payload->>'fluctlight_id'=candidate.payload->>'fluctlight_id')
    OR CASE
      WHEN candidate.intent_type='persona.compile' AND candidate.payload->>'source'='initialization'
        THEN EXISTS(SELECT 1 FROM public.fluctlight_world_bindings b WHERE b.fluctlight_id=candidate.payload->>'fluctlight_id')
      WHEN candidate.intent_type='wardrobe.initialize'
        THEN EXISTS(SELECT 1 FROM public.fluctlight_world_bindings b WHERE b.fluctlight_id=candidate.payload->>'fluctlight_id')
      WHEN candidate.intent_type='visual_identity.initialize'
        THEN EXISTS(SELECT 1 FROM public.fluctlight_wardrobe_preparations p WHERE p.fluctlight_id=candidate.payload->>'fluctlight_id' AND p.status='completed')
          AND EXISTS(SELECT 1 FROM public.life_schedules s WHERE s.fluctlight_id=candidate.payload->>'fluctlight_id' AND s.status IN ('accepted','superseded'))
      WHEN candidate.intent_id='schedule_intent:'||(candidate.payload->>'fluctlight_id')
        THEN EXISTS(SELECT 1 FROM public.fluctlight_wardrobe_preparations p WHERE p.fluctlight_id=candidate.payload->>'fluctlight_id' AND p.status='completed')
      WHEN candidate.intent_type='wake_up.current'
        THEN EXISTS(SELECT 1 FROM public.life_schedules s WHERE s.fluctlight_id=candidate.payload->>'fluctlight_id' AND s.status IN ('accepted','superseded'))
      ELSE true
    END
  )`

func dispatcherCandidateQuery() string {
	return fmt.Sprintf(`
		WITH eligible AS (
			SELECT candidate.intent_id,candidate.workflow_id,candidate.task_queue,candidate.intent_type,candidate.payload,COALESCE(candidate.attempt_count,0) AS attempt_count,candidate.created_at,
				%s AS intent_priority,
				ROW_NUMBER() OVER (
					PARTITION BY intent_type
					ORDER BY created_at,intent_id
				) AS class_rank
			FROM public.platform_workflow_intents AS candidate
			WHERE (candidate.status IS NULL OR candidate.status IN ('pending','retry'))
			  AND (candidate.next_attempt_at IS NULL OR candidate.next_attempt_at <= now())
			  %s
              AND (candidate.intent_type NOT IN ('cognition.processing','wake_up.current','reflection.run','goal.plan','goal.evaluate','wardrobe.initialize','schedule.current_day','diary.generate','visual_identity.initialize','intention.trigger','autonomy.action','capability.action')
                   OR NOT EXISTS (
                     SELECT 1 FROM public.platform_workflow_intents preparation JOIN public.fluctlights f ON f.id=preparation.payload->>'fluctlight_id'
                     WHERE preparation.intent_type='persona.compile'
                       AND preparation.payload->>'fluctlight_id'=candidate.payload->>'fluctlight_id'
                       AND preparation.payload->>'expected_revision'=f.current_revision::text
                       AND preparation.status<>'completed'
                       AND NOT EXISTS (SELECT 1 FROM public.platform_workflow_intents newer WHERE newer.intent_type='persona.compile' AND newer.payload->>'fluctlight_id'=preparation.payload->>'fluctlight_id' AND newer.payload->>'expected_revision'=preparation.payload->>'expected_revision' AND (newer.created_at,newer.intent_id)>(preparation.created_at,preparation.intent_id))
                   ))
			  AND (
					candidate.intent_type LIKE 'cognition.%%'
					OR candidate.intent_type NOT IN ('wake_up.current','reflection.run')
					OR NOT EXISTS (
						SELECT 1
						FROM public.platform_workflow_intents AS cognition
						WHERE cognition.intent_type LIKE 'cognition.%%'
						  AND cognition.payload->>'fluctlight_id'=candidate.payload->>'fluctlight_id'
						  AND (
								cognition.status IN ('started','cancel_requested')
								OR (cognition.status IN ('pending','retry') AND (cognition.next_attempt_at IS NULL OR cognition.next_attempt_at <= now()))
							  )
						)
				  )
		)
		SELECT intent_id,workflow_id,task_queue,intent_type,payload,attempt_count
		FROM eligible
		ORDER BY class_rank,intent_priority,created_at,intent_id
		LIMIT $1`, dispatcherIntentOrder, creationStageReadySQL)
}
