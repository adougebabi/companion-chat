package core

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func reflectionHasMemoryEvidence(refs []string) bool {
	for _, ref := range refs {
		if strings.HasPrefix(ref, string(ContextReferenceMemory)+":") || strings.HasPrefix(ref, string(ContextReferenceActiveMemory)+":") || strings.HasPrefix(ref, string(ContextReferenceOutcome)+":") {
			return true
		}
	}
	return false
}

// Reflection already persists its Core reference index. Reuse those exact
// Memory, ActiveMemory and Outcome versions remain Core-owned portrait inputs.
func reflectionMemoryEvidenceLive(ctx context.Context, query currentAuthorityReader, fid string, refs []string, index ContextReferenceIndex, clocks ...time.Time) (bool, error) {
	// The trusted proposal's JSONB snapshot normalizes JSON byte encoding.
	// Its signed index was validated before persistence. Recheck the stable
	// identity/scope fields and exact live Memory version at this read boundary.
	if reflectionHasMemoryEvidence(refs) && (index.SchemaVersion != contextReferenceIndexVersion || index.FluctlightID != fid || index.OwnerActorID == "") {
		return false, nil
	}
	at := time.Now().UTC()
	if len(clocks) > 0 && !clocks[0].IsZero() {
		at = clocks[0].UTC()
	}
	for _, ref := range refs {
		entry, found := index.ByRef[ref]
		if !found || (entry.Kind != ContextReferenceMemory && entry.Kind != ContextReferenceActiveMemory && entry.Kind != ContextReferenceOutcome) {
			if strings.HasPrefix(ref, string(ContextReferenceMemory)+":") || strings.HasPrefix(ref, string(ContextReferenceActiveMemory)+":") || strings.HasPrefix(ref, string(ContextReferenceOutcome)+":") {
				return false, nil
			}
			continue
		}
		binding := FrozenMemoryEvidenceSource{Kind: "memory", ID: entry.EntityID, Revision: entry.Revision, Fingerprint: stringValue(decodeObject(entry.Snapshot)["request_digest"])}
		if entry.Ref != ref || entry.Scope != index.expectedScope() || !contextReferencePattern.MatchString(ref) || binding.ID == "" || binding.Revision < 0 || binding.Fingerprint == "" {
			return false, nil
		}
		var live bool
		var err error
		switch entry.Kind {
		case ContextReferenceMemory:
			err = query.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.memories m WHERE m.id=$1 AND m.owner_fluctlight_id=$2 AND m.revision=$3 AND m.request_digest=$4 AND public.memory_source_is_live('memory',m.id,m.revision,m.request_digest))`, binding.ID, fid, binding.Revision, binding.Fingerprint).Scan(&live)
		case ContextReferenceActiveMemory:
			err = query.QueryRow(ctx, `SELECT EXISTS(
			 SELECT 1 FROM public.active_memories a
			 WHERE a.id=$1 AND a.owner_fluctlight_id=$2 AND a.revision=$3 AND a.request_digest=$4
			  AND a.status='active'
			  AND NOT EXISTS(
			   SELECT 1 FROM public.actor_fact_artifacts d JOIN public.actor_facts f ON f.id=d.fact_id
			   WHERE d.artifact_kind='active_memory' AND d.artifact_id=a.id AND d.artifact_revision=a.revision
			    AND (f.status<>'active' OR f.revision<>d.fact_revision OR f.valid_from>$5
			         OR (f.valid_until IS NOT NULL AND f.valid_until<=$5))
			  ) AND (a.valid_from IS NULL OR a.valid_from<=$5)
			  AND (a.valid_until IS NULL OR a.valid_until>$5)
			  AND COALESCE((
			   SELECT public.active_memory_source_is_live(c.command->>'source_kind',c.source_fact_id,c.command->>'source_fingerprint',c.id)
			   FROM public.active_memory_commands c
			   WHERE c.owner_fluctlight_id=a.owner_fluctlight_id AND c.result->>'active_memory_id'=a.id
			    AND (c.result->>'revision')::int=a.revision
			   ORDER BY c.created_at DESC,c.id DESC LIMIT 1
			  ),false))`, binding.ID, fid, binding.Revision, binding.Fingerprint, at).Scan(&live)
		case ContextReferenceOutcome:
			requestDigest := stringValue(decodeObject(entry.Snapshot)["request_digest"])
			fingerprint := fmt.Sprintf("%x", md5.Sum([]byte(requestDigest+":"+fmt.Sprint(entry.Revision))))
			err = query.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_action_outcomes o WHERE o.id=$1 AND o.fluctlight_id=$2 AND public.memory_source_is_live('outcome',o.id,$3,$4))`, entry.EntityID, fid, entry.Revision, fingerprint).Scan(&live)
		}
		if err != nil || !live {
			return false, err
		}
	}
	return true, nil
}

func reflectionProposalMemoryEvidenceLive(ctx context.Context, query currentAuthorityReader, fid, proposalID string, refs []string, clocks ...time.Time) (bool, error) {
	if !reflectionHasMemoryEvidence(refs) {
		return true, nil
	}
	if strings.TrimSpace(proposalID) == "" {
		return false, nil
	}
	var raw []byte
	if err := query.QueryRow(ctx, `SELECT context_snapshot FROM public.cognition_reflection_proposals WHERE id=$1 AND fluctlight_id=$2`, proposalID, fid).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	var evolution EvolutionContext
	if json.Unmarshal(raw, &evolution) != nil || evolution.FluctlightID != fid {
		return false, nil
	}
	return reflectionMemoryEvidenceLive(ctx, query, fid, refs, evolution.ReferenceIndex, clocks...)
}

func developingSelfMemoryEvidenceLive(ctx context.Context, query currentAuthorityReader, fid string, provenance map[string]any, refs []string, clocks ...time.Time) (bool, error) {
	source := stringValue(provenance["source"])
	if source != "reflection_v2" && source != "reflection" {
		return true, nil
	}
	return reflectionProposalMemoryEvidenceLive(ctx, query, fid, stringValue(provenance["proposal_id"]), refs, clocks...)
}

// Overlay evidence windows are already tied to persisted Reflection proposals.
// Resolve their original Memory references through those frozen contexts.
func reflectionOverlayMemoryEvidenceLive(ctx context.Context, query scheduleQuerier, fid string, overlay EvolutionOverlay, clocks ...time.Time) (bool, error) {
	domain := EvolutionPersonality
	if overlay.Kind == OverlayBehaviorPolicy {
		domain = EvolutionBehaviorPolicy
	}
	var candidateIndex, from, to int
	var payload, snapshot []byte
	err := query.QueryRow(ctx, `SELECT d.candidate_index,p.from_sequence,p.to_sequence,p.payload,p.context_snapshot FROM public.cognition_reflection_candidate_dispositions d JOIN public.cognition_reflection_proposals p ON p.id=d.proposal_id AND p.fluctlight_id=d.fluctlight_id WHERE d.fluctlight_id=$1 AND d.changed_ref=$2 AND d.domain=$3 AND d.disposition='accepted' ORDER BY p.created_at DESC,p.id DESC LIMIT 1`, fid, overlay.Ref, domain).Scan(&candidateIndex, &from, &to, &payload, &snapshot)
	if errors.Is(err, pgx.ErrNoRows) {
		return !reflectionHasMemoryEvidence(overlay.EvidenceRefs), nil
	}
	if err != nil {
		return false, err
	}
	var proposal ReflectionProposalV2
	var evolution EvolutionContext
	if json.Unmarshal(payload, &proposal) != nil || json.Unmarshal(snapshot, &evolution) != nil {
		return false, nil
	}
	candidates := proposal.PersonalityEvolutionCandidates
	if domain == EvolutionBehaviorPolicy {
		candidates = proposal.BehaviorPolicyEvolutionCandidates
	}
	if candidateIndex < 0 || candidateIndex >= len(candidates) {
		return false, nil
	}
	candidate := candidates[candidateIndex]
	if strings.TrimSpace(candidate.FieldPath) != overlay.FieldPath || strings.TrimSpace(candidate.Direction) != overlay.SemanticDirection || !sameEvidence(stringSliceAny(candidate.EvidenceRefs), stringSliceAny(overlay.EvidenceRefs)) {
		return false, nil
	}
	live, err := reflectionMemoryEvidenceLive(ctx, query, fid, candidate.EvidenceRefs, evolution.ReferenceIndex, clocks...)
	if err != nil || !live {
		return false, err
	}
	return reflectionOverlayHistoryMemoryEvidenceLive(ctx, query, fid, overlay.EvidenceWindows, fmt.Sprintf("sequence:%d-%d", from, to), candidate, domain, clocks...)
}

// The current window is supported by the accepted candidate. Each earlier
// window uses the first matching candidate in each proposal, matching the
// existing window collector. Independent proposals can support the same window.
func reflectionOverlayHistoryMemoryEvidenceLive(ctx context.Context, query scheduleQuerier, fid string, windows []string, currentWindow string, candidate ReflectionOverlayCandidateV2, domain EvolutionDomain, clocks ...time.Time) (bool, error) {
	rows, err := query.Query(ctx, `SELECT from_sequence,to_sequence,payload,context_snapshot FROM public.cognition_reflection_proposals WHERE fluctlight_id=$1 AND schema_version=$2 AND ('sequence:'||from_sequence::text||'-'||to_sequence::text)=ANY($3::text[]) ORDER BY to_sequence,id`, fid, reflectionProposalV2SchemaVersion, windows)
	if err != nil {
		return false, err
	}
	type windowSource struct {
		from, to int
		payload  []byte
		context  []byte
	}
	var history []windowSource
	for rows.Next() {
		var item windowSource
		if err := rows.Scan(&item.from, &item.to, &item.payload, &item.context); err != nil {
			rows.Close()
			return false, err
		}
		history = append(history, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	matched := map[string]bool{currentWindow: true}
	for _, source := range history {
		window := fmt.Sprintf("sequence:%d-%d", source.from, source.to)
		if window == currentWindow || matched[window] {
			continue
		}
		var proposal ReflectionProposalV2
		var evolution EvolutionContext
		if json.Unmarshal(source.payload, &proposal) != nil || json.Unmarshal(source.context, &evolution) != nil {
			return false, nil
		}
		items := proposal.PersonalityEvolutionCandidates
		if domain == EvolutionBehaviorPolicy {
			items = proposal.BehaviorPolicyEvolutionCandidates
		}
		for _, item := range items {
			if item.FieldPath != candidate.FieldPath || item.Direction != candidate.Direction || item.SemanticValue != candidate.SemanticValue {
				continue
			}
			live, err := reflectionMemoryEvidenceLive(ctx, query, fid, item.EvidenceRefs, evolution.ReferenceIndex, clocks...)
			if err != nil {
				return false, err
			}
			matched[window] = live
			break
		}
	}
	for _, window := range windows {
		if !matched[window] {
			return false, nil
		}
	}
	return true, nil
}
