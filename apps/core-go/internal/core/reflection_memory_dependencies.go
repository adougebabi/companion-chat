package core

import (
	"context"
	"encoding/json"
	"errors"
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
