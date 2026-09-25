package migrations

// Resident is a bounded materialized read model of current sourced long-term
// relationship/autobiographical Memory and unfinished Active commitments. It
// contains no separate fact authority. The owning mutation transaction
// publishes a whole generation or fails; prompt assembly only reads it.
const residentMemorySchemaSQL = `
CREATE TABLE IF NOT EXISTS public.resident_memory_snapshots (
 fluctlight_id varchar(128) PRIMARY KEY REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 generation bigint NOT NULL CHECK (generation >= 1),
 source_digest varchar(32) NOT NULL CHECK (source_digest ~ '^[a-f0-9]{32}$'),
 items_json jsonb NOT NULL CHECK (jsonb_typeof(items_json)='array'),
 status varchar(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active','stale')),
 published_at timestamptz NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION public.refresh_resident_memory_snapshot(p_fluctlight_id text)
RETURNS void LANGUAGE plpgsql AS $resident_refresh$
DECLARE memory_items jsonb; active_items jsonb; combined_items jsonb; next_digest text;
BEGIN
 IF p_fluctlight_id IS NULL OR btrim(p_fluctlight_id)='' OR NOT EXISTS (SELECT 1 FROM public.fluctlights WHERE id=p_fluctlight_id) THEN RETURN; END IF;
 SELECT COALESCE(jsonb_agg(item ORDER BY importance DESC,id),'[]'::jsonb) INTO memory_items
 FROM (
  SELECT m.id,m.importance,jsonb_build_object(
   'source_kind','memory','id',m.id,'revision',m.revision,'type',m.type,
   'content',left(m.content,512),'confidence',m.confidence,'importance',m.importance,
   'provenance_status',CASE WHEN support.live_count=support.total_count THEN 'verified' ELSE 'partial' END,
   'epistemic_kind',m.epistemic_kind,'source_refs',support.live_refs,
   'request_digest',m.request_digest,
   'occurred_at',m.occurred_at,'visibility',m.visibility,'actor_refs',m.actor_refs,
   'personality_perspectives',m.personality_perspectives,'conversation_id',m.conversation_id
  ) AS item
  FROM public.memories m
  CROSS JOIN LATERAL (
   SELECT count(*) FILTER (WHERE link.live) AS live_count,count(*) AS total_count,
    COALESCE(jsonb_agg(link.source_ref ORDER BY link.source_ref) FILTER (WHERE link.live),'[]'::jsonb) AS live_refs
   FROM (SELECT source_ref,status='valid' AND public.memory_source_is_live(source_kind,source_id,source_revision,source_fingerprint) AS live
    FROM public.memory_source_links WHERE memory_id=m.id AND memory_revision=m.revision) link
  ) support
  WHERE m.owner_fluctlight_id=p_fluctlight_id AND m.conversation_id IS NULL
   AND m.status='active' AND m.type IN ('relationship','autobiographical')
   AND m.provenance_status IN ('verified','partial')
   AND support.live_count>0
  ORDER BY m.importance DESC,m.id LIMIT 4
 ) selected;
 SELECT COALESCE(jsonb_agg(item ORDER BY importance DESC,id),'[]'::jsonb) INTO active_items
 FROM (
  SELECT a.id,a.importance,jsonb_build_object(
   'source_kind','active_memory','id',a.id,'revision',a.revision,'type',a.kind,
   'content',left(a.content,512),'confidence',a.confidence,'importance',a.importance,
   'source_validity',CASE WHEN support.source_kind IS NULL OR support.source_kind='' THEN 'legacy_unknown' ELSE 'verified' END,
   'source_refs',a.evidence_refs,'original_time_expression',a.original_time_expression,'valid_from',a.valid_from,
   'valid_until',a.valid_until,'conversation_id',a.conversation_id
  ) AS item
  FROM public.active_memories a
  LEFT JOIN LATERAL (
   SELECT c.id AS command_id,c.source_fact_id,c.command->>'source_kind' AS source_kind,
    c.command->>'source_fingerprint' AS source_fingerprint
   FROM public.active_memory_commands c
   WHERE c.owner_fluctlight_id=a.owner_fluctlight_id
    AND c.result->>'active_memory_id'=a.id AND (c.result->>'revision')::integer=a.revision
   ORDER BY c.created_at DESC,c.id DESC LIMIT 1
  ) support ON true
  WHERE a.owner_fluctlight_id=p_fluctlight_id AND a.conversation_id IS NULL
   AND a.status='active' AND a.kind IN ('commitment','future_event')
   AND (support.source_kind IS NULL OR support.source_kind=''
        OR public.active_memory_source_is_live(support.source_kind,support.source_fact_id,support.source_fingerprint,support.command_id))
   AND (a.valid_from IS NULL OR a.valid_from<=now())
   AND (a.valid_until IS NULL OR a.valid_until>now())
  ORDER BY a.importance DESC,a.id LIMIT 4
 ) selected;
 combined_items := memory_items || active_items;
 next_digest := md5(combined_items::text);
 INSERT INTO public.resident_memory_snapshots(fluctlight_id,generation,source_digest,items_json,status,published_at)
 VALUES(p_fluctlight_id,1,next_digest,combined_items,'active',now())
 ON CONFLICT(fluctlight_id) DO UPDATE SET
  generation=public.resident_memory_snapshots.generation+1,
  source_digest=excluded.source_digest,items_json=excluded.items_json,status='active',published_at=now()
 WHERE public.resident_memory_snapshots.source_digest IS DISTINCT FROM excluded.source_digest
    OR public.resident_memory_snapshots.status<>'active';
END
$resident_refresh$;

DO $resident_backfill$
DECLARE v_fluctlight_id text;
BEGIN
 FOR v_fluctlight_id IN SELECT id FROM public.fluctlights LOOP
  PERFORM public.refresh_resident_memory_snapshot(v_fluctlight_id);
 END LOOP;
END
$resident_backfill$;
`
