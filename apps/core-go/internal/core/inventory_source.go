package core

// i is the existing inventory row alias. Owner governance and controlled
// initialization are explicit sources; a purchase needs a completed Event.
const inventorySourceVerifiedSQL = `(i.source_kind='initialization' OR
 (i.source_kind='accepted_event' AND EXISTS(SELECT 1 FROM public.fluctlights f WHERE f.id=i.fluctlight_id AND i.source_ref='owner:'||f.created_by_actor_id)) OR
 EXISTS(SELECT 1 FROM public.life_events e WHERE e.id=i.source_ref AND e.fluctlight_id=i.fluctlight_id AND e.status='confirmed'
  AND (i.source_kind<>'purchase_result' OR (e.kind='virtual_shopping' AND e.result #>> '{result,status}'='completed'))))`
