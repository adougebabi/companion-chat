package core

// Independent cognition lanes may complete out of sequence. A processed-only
// scan must not advance the cumulative Reflection watermark past unfinished
// earlier work: its real evidence would otherwise be lost on later settlement.
const reflectionSourceQuery = `
SELECT i.id,i.sequence,i.event_type,i.payload,i.occurred_at,public.cognition_source_fingerprint(i.payload)
FROM public.cognition_inbox i
WHERE i.fluctlight_id=$1 AND i.sequence>$2 AND i.status='processed'
  AND COALESCE(i.payload->>'source_message_invalidated','false')<>'true'
  AND NOT EXISTS (
    SELECT 1 FROM public.cognition_inbox waiting
    WHERE waiting.fluctlight_id=i.fluctlight_id AND waiting.sequence<i.sequence
      AND waiting.status IN ('pending','claimed','waiting_reply'))
ORDER BY i.sequence LIMIT 100`
