package core

import "strings"

// The durable inbox owns the business outcome. Temporal/intent completion only
// says that its execution ended. Keep both facts, including for historical rows.
// alias is a source-owned SQL alias, never request input.
func cognitionTaskOutcomeProjectionSQL(alias string) string {
	return strings.ReplaceAll(`CASE WHEN task.intent_type='cognition.processing' THEN COALESCE((
 SELECT jsonb_build_object(
 'workflow_status',task.status,
 'inbox_status',c.status,
 'turn_status',CASE WHEN c.status='processed' THEN 'completed' WHEN c.error_code='user_cancelled' THEN 'cancelled' WHEN c.status IN ('claimed','waiting_reply') THEN 'running' ELSE c.status END,
 'status',CASE WHEN c.status='processed' THEN 'completed' WHEN c.error_code='user_cancelled' THEN 'cancelled' WHEN c.status='failed' THEN 'failed' WHEN task.status='completed' AND c.status IN ('pending','claimed','waiting_reply','retry') THEN CASE WHEN c.status='pending' THEN 'pending' ELSE 'running' END ELSE task.status END,
 'error_code',COALESCE(c.error_code,''),
 'last_error',CASE WHEN c.status='failed' THEN COALESCE(NULLIF(c.error_code,''),'conversation_turn_failed') ELSE COALESCE(task.last_error,'') END,
 'turn_id',COALESCE(c.payload->>'turn_id',''),
 'correlation_id',CASE WHEN COALESCE(c.payload->>'turn_id','')<>'' THEN 'turn:'||(c.payload->>'turn_id') ELSE task.intent_id END,
 'reply_delivered',EXISTS(SELECT 1 FROM public.conversation_messages m WHERE m.source_fact_id=c.id AND m.conversation_id=c.payload->>'conversation_id' AND m.kind='assistant'))
 FROM public.cognition_inbox c WHERE c.id=task.payload->>'inbox_id' AND c.fluctlight_id=task.payload->>'fluctlight_id'
 ),'{}'::jsonb) ELSE '{}'::jsonb END`, "task.", alias+".")
}

func applyCognitionTaskOutcome(task, outcome map[string]any) {
	for key, value := range outcome {
		task[key] = value
	}
}
