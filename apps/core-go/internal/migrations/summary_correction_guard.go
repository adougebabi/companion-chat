package migrations

import (
	"fmt"
	"strings"
)

// Reinstalled after earlier source-liveness functions on every migration run.
// The functions only read existing source authority; they do not rewrite
// messages, Memory, summaries or resident snapshots.
const summaryCorrectionGuardSQL = `
CREATE OR REPLACE FUNCTION public.summary_contains_corrected_memory_source(p_owner text,p_conversation text,p_from integer,p_to integer)
RETURNS boolean LANGUAGE sql STABLE AS $summary_correction$
 WITH RECURSIVE sources(kind,id,revision) AS (
  SELECT source.source_kind,source.source_id,source.source_revision
  FROM public.memories memory
  JOIN public.memory_revisions correction ON correction.memory_id=memory.id
  JOIN public.memory_source_links source ON source.memory_id=correction.memory_id AND source.memory_revision<=correction.base_revision
  WHERE memory.owner_fluctlight_id=p_owner AND correction.revision>correction.base_revision
    AND correction.operation IN ('revise','merge','supersede','deprecate','forget','rollback')
    -- Replacing a daily rollup after its raw messages changed is not an
    -- Owner correction to that rollup. Treating it as one would permanently
    -- fence the freshly rebuilt summary and break the existing repair path.
    AND NOT (correction.operation='revise' AND correction.actor_id=memory.owner_fluctlight_id
      AND COALESCE(correction.proposal_id,'') LIKE 'conversation_daily:%'
      AND correction.semantic_reason='same-conversation local-day episode consolidation'
      AND EXISTS (SELECT 1 FROM public.conversation_daily_memories daily WHERE daily.memory_id=memory.id))
    AND source.status IN ('valid','invalid')
  UNION
  SELECT parent.source_kind,parent.source_id,parent.source_revision
  FROM sources child
  JOIN public.memories memory ON child.kind='memory' AND memory.id=child.id AND memory.owner_fluctlight_id=p_owner
  JOIN public.memory_source_links parent ON parent.memory_id=child.id AND parent.memory_revision<=child.revision
  WHERE parent.status IN ('valid','invalid')
 )
 SELECT EXISTS (
  SELECT 1 FROM sources source
  JOIN public.conversation_messages message ON message.conversation_id=p_conversation AND message.sequence BETWEEN p_from AND p_to
  LEFT JOIN public.conversation_summaries episode ON source.kind='conversation_summary' AND episode.id=source.id
    AND episode.owner_fluctlight_id=p_owner AND episode.conversation_id=p_conversation
  WHERE (source.kind='message' AND source.id=message.id)
     OR (source.kind='fact' AND source.id=message.source_fact_id)
     OR (source.kind='conversation_summary' AND episode.source_message_refs ? ('message:' || message.id))
 );
$summary_correction$;

-- Use the newest typed binding for each retained ref. A legacy Confirm may
-- have lost its typed link, so it can inherit an earlier binding only across
-- Confirm revisions that continuously retain the ref. A semantic revision is
-- a boundary: the same ref may now be frozen to a different source version.
CREATE OR REPLACE FUNCTION public.memory_retained_typed_sources(p_memory text,p_revision integer)
RETURNS SETOF public.memory_source_links LANGUAGE sql STABLE AS $retained_sources$
 SELECT DISTINCT ON (source.source_ref) source.*
 FROM public.memory_revisions snapshot
 JOIN public.memory_source_links source ON source.memory_id=snapshot.memory_id AND source.memory_revision<=snapshot.revision
 WHERE snapshot.memory_id=p_memory AND snapshot.revision=p_revision AND snapshot.evidence_refs ? source.source_ref
  AND source.status IN ('valid','invalid')
  AND NOT EXISTS (
   SELECT 1 FROM public.memory_revisions changed
   WHERE changed.memory_id=snapshot.memory_id AND changed.revision>source.memory_revision AND changed.revision<=snapshot.revision
    AND (changed.operation<>'confirm' OR NOT (changed.evidence_refs ? source.source_ref))
  )
 ORDER BY source.source_ref,source.memory_revision DESC;
$retained_sources$;

CREATE OR REPLACE FUNCTION public.memory_has_corrected_summary_source(p_owner text,p_memory text,p_revision integer)
RETURNS boolean LANGUAGE sql STABLE AS $memory_corrected_summary$
 WITH RECURSIVE ancestry(id,revision) AS (
  SELECT m.id::text,r.revision FROM public.memories m
  JOIN public.memory_revisions r ON r.memory_id=m.id AND r.revision=p_revision
  WHERE m.id=p_memory AND m.owner_fluctlight_id=p_owner
  UNION
  SELECT parent.source_id::text,parent.source_revision
  FROM ancestry child
  JOIN LATERAL public.memory_retained_typed_sources(child.id,child.revision) parent ON parent.source_kind='memory'
  JOIN public.memories linked ON linked.id=parent.source_id AND linked.owner_fluctlight_id=p_owner
 )
 SELECT EXISTS (
  SELECT 1 FROM ancestry child
  JOIN LATERAL public.memory_retained_typed_sources(child.id,child.revision) source ON source.source_kind='conversation_summary'
  JOIN public.conversation_summaries episode ON episode.id=source.source_id AND episode.owner_fluctlight_id=p_owner
  WHERE public.summary_contains_corrected_memory_source(p_owner,episode.conversation_id,episode.from_sequence,episode.to_sequence)
 );
$memory_corrected_summary$;

CREATE OR REPLACE FUNCTION public.memory_source_is_live(source_kind text,source_id text,source_revision integer,source_fingerprint text)
RETURNS boolean LANGUAGE sql STABLE AS $memory_source_live$
 SELECT public.memory_source_is_live_deep($1,$2,$3,$4,0,ARRAY[]::text[]) AND CASE $1
  WHEN 'conversation_summary' THEN COALESCE((SELECT NOT public.summary_contains_corrected_memory_source(s.owner_fluctlight_id,s.conversation_id,s.from_sequence,s.to_sequence) FROM public.conversation_summaries s WHERE s.id=$2),false)
  WHEN 'memory' THEN COALESCE((SELECT NOT public.memory_has_corrected_summary_source(m.owner_fluctlight_id,m.id,$3) FROM public.memories m WHERE m.id=$2),false)
  ELSE true END;
$memory_source_live$;
`

// Install the same resident projection with the new guard before its LIMIT.
// Extract only the function so this step does not run the legacy backfill a
// second time; the existing schema bundle may already have run that backfill.
func guardedResidentMemoryRefreshSQL() (string, error) {
	const start = "CREATE OR REPLACE FUNCTION public.refresh_resident_memory_snapshot"
	const end = "\nDO $resident_backfill$"
	const predicate = "   AND support.live_count>0\n"
	_, body, ok := strings.Cut(residentMemorySchemaSQL, start)
	if !ok {
		return "", fmt.Errorf("resident refresh definition missing")
	}
	body, _, ok = strings.Cut(body, end)
	if !ok || strings.Count(body, predicate) != 1 {
		return "", fmt.Errorf("resident refresh predicate changed")
	}
	body = strings.Replace(body, predicate, predicate+"   AND NOT public.memory_has_corrected_summary_source(m.owner_fluctlight_id,m.id,m.revision)\n", 1)
	return start + body, nil
}
