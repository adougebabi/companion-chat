package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func reflectionHasMemoryEvidence(refs []string) bool {
	for _, ref := range refs {
		if strings.HasPrefix(ref, string(ContextReferenceMemory)+":") {
			return true
		}
	}
	return false
}

// Reflection already persists its Core reference index. Reuse those exact
// Memory versions and the existing recursive liveness authority for portraits.
func reflectionMemoryEvidenceLive(ctx context.Context, query currentAuthorityReader, fid string, refs []string, index ContextReferenceIndex) (bool, error) {
	// The trusted proposal's JSONB snapshot normalizes JSON byte encoding.
	// Its signed index was validated before persistence. Recheck the stable
	// identity/scope fields and exact live Memory version at this read boundary.
	if reflectionHasMemoryEvidence(refs) && (index.SchemaVersion != contextReferenceIndexVersion || index.FluctlightID != fid || index.OwnerActorID == "") {
		return false, nil
	}
	for _, ref := range refs {
		entry, found := index.ByRef[ref]
		if !found || entry.Kind != ContextReferenceMemory {
			if strings.HasPrefix(ref, string(ContextReferenceMemory)+":") {
				return false, nil
			}
			continue
		}
		binding := FrozenMemoryEvidenceSource{Kind: "memory", ID: entry.EntityID, Revision: entry.Revision, Fingerprint: stringValue(decodeObject(entry.Snapshot)["request_digest"])}
		if entry.Ref != ref || entry.Scope != index.expectedScope() || !contextReferencePattern.MatchString(ref) || binding.ID == "" || binding.Revision < 0 || len(binding.Fingerprint) != 32 {
			return false, nil
		}
		var live bool
		err := query.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.memories m WHERE m.id=$1 AND m.owner_fluctlight_id=$2 AND m.revision=$3 AND m.request_digest=$4 AND public.memory_source_is_live('memory',m.id,m.revision,m.request_digest))`, binding.ID, fid, binding.Revision, binding.Fingerprint).Scan(&live)
		if err != nil || !live {
			return false, err
		}
	}
	return true, nil
}

func reflectionProposalMemoryEvidenceLive(ctx context.Context, query currentAuthorityReader, fid, proposalID string, refs []string) (bool, error) {
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
	return reflectionMemoryEvidenceLive(ctx, query, fid, refs, evolution.ReferenceIndex)
}

func developingSelfMemoryEvidenceLive(ctx context.Context, query currentAuthorityReader, fid string, provenance map[string]any, refs []string) (bool, error) {
	source := stringValue(provenance["source"])
	if source != "reflection_v2" && source != "reflection" {
		return true, nil
	}
	return reflectionProposalMemoryEvidenceLive(ctx, query, fid, stringValue(provenance["proposal_id"]), refs)
}

// Overlay evidence windows are already tied to persisted Reflection proposals.
// Resolve their original Memory references through those frozen contexts.
func reflectionOverlayMemoryEvidenceLive(ctx context.Context, query scheduleQuerier, fid string, overlay EvolutionOverlay) (bool, error) {
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
	live, err := reflectionMemoryEvidenceLive(ctx, query, fid, candidate.EvidenceRefs, evolution.ReferenceIndex)
	if err != nil || !live {
		return false, err
	}
	return reflectionOverlayHistoryMemoryEvidenceLive(ctx, query, fid, overlay.EvidenceWindows, fmt.Sprintf("sequence:%d-%d", from, to), candidate, domain)
}

// The current window is supported by the accepted candidate. Each earlier
// window uses the first matching candidate in each proposal, matching the
// existing window collector. Independent proposals can support the same window.
func reflectionOverlayHistoryMemoryEvidenceLive(ctx context.Context, query scheduleQuerier, fid string, windows []string, currentWindow string, candidate ReflectionOverlayCandidateV2, domain EvolutionDomain) (bool, error) {
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
			live, err := reflectionMemoryEvidenceLive(ctx, query, fid, item.EvidenceRefs, evolution.ReferenceIndex)
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
