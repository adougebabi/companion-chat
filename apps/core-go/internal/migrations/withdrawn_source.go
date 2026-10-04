package migrations

// Preserve raw admission payloads as audit while withdrawing their current
// evidence qualification after the source message is edited or removed.
const withdrawnSourceSchemaSQL = `
CREATE OR REPLACE FUNCTION public.memory_source_is_live_deep(p_kind text, p_id text, p_revision integer, p_fingerprint text, p_depth integer, p_visited text[])
RETURNS boolean LANGUAGE plpgsql STABLE AS $memory_source_deep$
DECLARE source_memory record;
BEGIN
 IF p_depth > 16 THEN RETURN false; END IF;
 CASE p_kind
  WHEN 'fact' THEN RETURN EXISTS (SELECT 1 FROM public.cognition_inbox i WHERE i.id=p_id AND COALESCE(i.payload->>'source_message_invalidated','false')<>'true' AND public.cognition_source_fingerprint(i.payload)=p_fingerprint);
  WHEN 'message' THEN RETURN EXISTS (SELECT 1 FROM public.conversation_messages msg WHERE msg.id=p_id AND md5(msg.text || msg.attachment_refs::text)=p_fingerprint);
  WHEN 'outcome' THEN RETURN EXISTS (SELECT 1 FROM public.cognition_action_outcomes o WHERE o.id=p_id AND o.revision=p_revision AND md5(o.request_digest || ':' || o.revision::text)=p_fingerprint);
  WHEN 'conversation_summary' THEN RETURN EXISTS (
    SELECT 1 FROM public.conversation_summaries s WHERE s.id=p_id AND s.revision=p_revision
      AND s.status IN ('active','consolidated') AND public.conversation_summary_source_fingerprint(s.id)=p_fingerprint);
  WHEN 'owner_confirmation', 'authenticated_command' THEN
   RETURN EXISTS (SELECT 1 FROM public.memory_governance g WHERE g.idempotency_key=p_id AND g.request_digest=p_fingerprint AND g.disposition IN ('applied','no_change'));
  WHEN 'memory' THEN
   IF p_id=ANY(p_visited) THEN RETURN false; END IF;
   SELECT id,revision,status,provenance_status,request_digest INTO source_memory FROM public.memories WHERE id=p_id;
   IF NOT FOUND OR source_memory.revision<>p_revision OR source_memory.status<>'active'
      OR source_memory.provenance_status NOT IN ('verified','partial') OR source_memory.request_digest<>p_fingerprint THEN
    RETURN false;
   END IF;
   IF EXISTS (SELECT 1 FROM public.conversation_daily_memories d WHERE d.memory_id=p_id) THEN
    RETURN (SELECT count(*)>0 AND bool_and(l.status='valid' AND public.memory_source_is_live_deep(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint,p_depth+1,p_visited || p_id))
      FROM public.memory_source_links l WHERE l.memory_id=p_id AND l.memory_revision=p_revision);
   END IF;
   RETURN EXISTS (SELECT 1 FROM public.memory_source_links l WHERE l.memory_id=p_id AND l.memory_revision=p_revision
     AND l.status='valid' AND public.memory_source_is_live_deep(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint,p_depth+1,p_visited || p_id));
  ELSE RETURN false;
 END CASE;
END
$memory_source_deep$;
CREATE OR REPLACE FUNCTION public.active_memory_source_is_live(source_kind text, source_id text, source_fingerprint text, command_id text)
RETURNS boolean LANGUAGE sql STABLE AS $active_source_live$
 SELECT CASE source_kind
  WHEN 'fact' THEN EXISTS (SELECT 1 FROM public.cognition_inbox i WHERE i.id=source_id AND COALESCE(i.payload->>'source_message_invalidated','false')<>'true' AND public.cognition_source_fingerprint(i.payload)=source_fingerprint)
  WHEN 'authenticated_command' THEN EXISTS (SELECT 1 FROM public.active_memory_commands c WHERE c.id=command_id AND c.command->>'source_fingerprint'=source_fingerprint)
  ELSE false END;
$active_source_live$;
`
