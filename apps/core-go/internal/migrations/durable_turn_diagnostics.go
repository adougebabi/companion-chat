package migrations

// Backfill only trace identity. Current state is read from the same inbox as
// chat; no prose or completion is invented and no workflow is restarted.
const durableTurnDiagnosticsSQL = `
INSERT INTO public.diagnostic_turns(id,fluctlight_id,conversation_id,source_event_id,correlation_id,status,created_at)
SELECT id,fluctlight_id,payload->>'conversation_id',id,correlation_id,status,created_at
FROM public.cognition_inbox WHERE event_type='conversation.turn'
AND COALESCE(payload->>'conversation_id','')<>''
ON CONFLICT(id) DO NOTHING;
`
