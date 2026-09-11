package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func applyReflectionRelationshipObservationsV2Tx(ctx context.Context, tx pgx.Tx, fluctlightID string, proposal ReflectionProposalV2, plan ReflectionEvolutionPlan, index ContextReferenceIndex) error {
	for _, candidatePlan := range plan.Candidates {
		if candidatePlan.Domain != EvolutionRelationship || candidatePlan.Disposition != EvolutionAccepted {
			continue
		}
		candidate := proposal.RelationshipObservations[candidatePlan.Index]
		entry, ok := index.ByRef[candidate.TargetRef]
		if !ok || entry.Kind != ContextReferenceRelationship {
			return errors.New("reflection_relationship_target_invalid")
		}
		var ownerID, profileID, targetActorID, trend string
		var roleRaw, metricsRaw, emotionalRaw, provenanceRaw []byte
		var summary *string
		var revision int
		if err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,COALESCE(profile_id,'default'),target_actor_id,role,metrics,trend,summary,emotional_association,provenance,revision FROM public.relationships WHERE id=$1 FOR UPDATE`, entry.EntityID).Scan(&ownerID, &profileID, &targetActorID, &roleRaw, &metricsRaw, &trend, &summary, &emotionalRaw, &provenanceRaw, &revision); err != nil {
			return err
		}
		if ownerID != fluctlightID || revision != entry.Revision || profileID != activeEvolutionProfile(index) {
			return errors.New("reflection_relationship_revision_stale")
		}
		metrics, err := validateRelationshipMetrics(decodeObject(metricsRaw))
		if err != nil {
			return err
		}
		sign, nextTrend, err := relationshipEvolutionDirection(candidate.Direction)
		if err != nil {
			return err
		}
		delta := sign * 0.1 * candidate.Strength * candidate.Confidence
		for key, raw := range metrics {
			metrics[key] = clampUnit(numberOrZero(raw) + delta)
		}
		nextRevision := revision + 1
		provenance := map[string]any{"source": "reflection_v2", "proposal_id": plan.ProposalID, "evidence_refs": stringSliceAny(candidate.EvidenceRefs)}
		command, err := tx.Exec(ctx, `UPDATE public.relationships SET metrics=$2,trend=$3,summary=$4,provenance=$5,revision=$6,updated_at=now() WHERE id=$1 AND owner_fluctlight_id=$7 AND revision=$8`, entry.EntityID, jsonBytes(metrics), nextTrend, candidate.Observation, jsonBytes(provenance), nextRevision, fluctlightID, revision)
		if err != nil || command.RowsAffected() != 1 {
			if err == nil {
				err = errors.New("reflection_relationship_revision_stale")
			}
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.relationship_revisions(id,relationship_id,revision,base_revision,role,metrics,trend,summary,emotional_association,evidence_refs,actor_id,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, "relationship_revision_"+stableDigest(plan.ProposalID+"\x1f"+candidatePlan.CandidateID), entry.EntityID, nextRevision, revision, roleRaw, jsonBytes(metrics), nextTrend, candidate.Observation, emotionalRaw, jsonBytes(candidate.EvidenceRefs), fluctlightID, "reflection-v2:"+candidatePlan.CandidateID); err != nil {
			return err
		}
	}
	return nil
}

func relationshipEvolutionDirection(value string) (float64, string, error) {
	switch strings.TrimSpace(value) {
	case "increase", "strengthen", "improve", "positive", "toward":
		return 1, "improving", nil
	case "decrease", "weaken", "decline", "negative", "away":
		return -1, "declining", nil
	case "maintain", "stable":
		return 0, "stable", nil
	default:
		return 0, "", errors.New("reflection_relationship_direction_invalid")
	}
}

func (a *App) applyReflectionSlotCandidatesV2Tx(ctx context.Context, tx pgx.Tx, fluctlightID string, proposal ReflectionProposalV2, plan ReflectionEvolutionPlan, evolution EvolutionContext) error {
	allowed := reflectionAllowedEvidence(evolution)
	apply := func(domain EvolutionDomain, values []ReflectionSlotCandidateV2) error {
		for _, candidatePlan := range plan.Candidates {
			if candidatePlan.Domain != domain || candidatePlan.Disposition != EvolutionAccepted {
				continue
			}
			candidate := values[candidatePlan.Index]
			expectedKind := map[EvolutionDomain]ContextReferenceKind{EvolutionDrive: ContextReferenceDrive, EvolutionPreference: ContextReferencePreference, EvolutionTrigger: ContextReferenceTrigger}[domain]
			entityID := reflectionSlotEntityID(domain, fluctlightID, candidate.Key)
			switch candidate.Operation {
			case "create":
				if candidate.TargetRef != "" {
					return errors.New("reflection_slot_create_target_forbidden")
				}
				var exists bool
				table := map[EvolutionDomain]string{EvolutionDrive: "fluctlight_drive_slots", EvolutionPreference: "fluctlight_preference_slots", EvolutionTrigger: "fluctlight_trigger_preferences"}[domain]
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.`+table+` WHERE id=$1)`, entityID).Scan(&exists); err != nil {
					return err
				}
				if exists {
					return errors.New("reflection_slot_create_conflict")
				}
			case "update", "pause", "deprecate":
				if candidate.TargetRef == "" {
					return errors.New("reflection_slot_target_required")
				}
				entry, ok := evolution.ReferenceIndex.ByRef[candidate.TargetRef]
				if !ok || entry.Kind != expectedKind || entry.EntityID != entityID || stringValue(decodeObject(entry.Snapshot)["key"]) != candidate.Key {
					return errors.New("reflection_slot_target_invalid")
				}
				var liveRevision int
				table := map[EvolutionDomain]string{EvolutionDrive: "fluctlight_drive_slots", EvolutionPreference: "fluctlight_preference_slots", EvolutionTrigger: "fluctlight_trigger_preferences"}[domain]
				if err := tx.QueryRow(ctx, `SELECT revision FROM public.`+table+` WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, entityID, fluctlightID).Scan(&liveRevision); err != nil || liveRevision != entry.Revision {
					if err != nil {
						return err
					}
					return errors.New("reflection_slot_revision_stale")
				}
			default:
				return errors.New("reflection_slot_operation_invalid")
			}
			item, err := reflectionSlotMutation(domain, candidate, evolution.ReferenceIndex)
			if err != nil {
				return err
			}
			item["idempotency_key"] = "reflection-v2:" + candidatePlan.CandidateID
			switch domain {
			case EvolutionDrive:
				err = a.applyDriveSlotCandidateTx(ctx, tx, fluctlightID, item, allowed, plan.SourceWindow, candidatePlan.Index)
			case EvolutionPreference:
				err = a.applyPreferenceSlotCandidateTx(ctx, tx, fluctlightID, item, allowed, plan.SourceWindow, candidatePlan.Index)
			case EvolutionTrigger:
				err = a.applyTriggerPreferenceCandidateTx(ctx, tx, fluctlightID, item, allowed, plan.SourceWindow, candidatePlan.Index)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	if err := apply(EvolutionDrive, proposal.DriveCandidates); err != nil {
		return err
	}
	if err := apply(EvolutionPreference, proposal.PreferenceCandidates); err != nil {
		return err
	}
	return apply(EvolutionTrigger, proposal.TriggerCandidates)
}

func reflectionSlotEntityID(domain EvolutionDomain, fluctlightID, key string) string {
	prefix := map[EvolutionDomain]string{EvolutionDrive: "drive_slot_", EvolutionPreference: "preference_slot_", EvolutionTrigger: "trigger_preference_"}[domain]
	return prefix + stableDigest(fluctlightID+":"+key)
}

func reflectionSlotMutation(domain EvolutionDomain, candidate ReflectionSlotCandidateV2, index ContextReferenceIndex) (map[string]any, error) {
	status := "active"
	if candidate.Operation == "pause" || candidate.Operation == "deprecate" {
		status = "paused"
	}
	base := map[string]any{
		"key": candidate.Key, "label": candidate.Key, "description": candidate.SemanticValue,
		"confidence": candidate.Confidence, "evidence_refs": stringSliceAny(candidate.EvidenceRefs), "status": status,
		"provenance": map[string]any{"source": "reflection_v2"}, "update_policy": map[string]any{"policy_version": reflectionPolicyVersion},
	}
	switch domain {
	case EvolutionDrive:
		pressure := 0.0
		if candidate.TargetRef != "" {
			pressure = numberOrZero(mapValue(decodeObject(index.ByRef[candidate.TargetRef].Snapshot)["value"])["pressure"])
		}
		sign := 0.0
		switch candidate.Direction {
		case "increase", "strengthen", "toward":
			sign = 1
		case "decrease", "weaken", "away":
			sign = -1
		case "maintain":
		default:
			return nil, errors.New("reflection_drive_direction_invalid")
		}
		base["value_schema"] = "pressure"
		base["value"] = map[string]any{"pressure": clampUnit(pressure + sign*0.1*candidate.Strength*candidate.Confidence), "salience": candidate.Confidence, "direction": candidate.Direction}
		base["decay_policy"] = map[string]any{"authority": "affect_profile"}
	case EvolutionPreference:
		base["value_schema"] = "categorical"
		base["value"] = map[string]any{"selected": candidate.SemanticValue}
	case EvolutionTrigger:
		base["value"] = map[string]any{"semantic_value": candidate.SemanticValue, "direction": candidate.Direction}
		base["trigger_schema"] = "semantic.trigger.v2"
	default:
		return nil, fmt.Errorf("reflection slot domain %q unsupported", domain)
	}
	return base, nil
}

func (a *App) applyReflectionDevelopingSelfV2Tx(ctx context.Context, tx pgx.Tx, fluctlightID string, proposal ReflectionProposalV2, plan ReflectionEvolutionPlan, evolution EvolutionContext) error {
	allowed := reflectionAllowedEvidence(evolution)
	for _, candidatePlan := range plan.Candidates {
		if candidatePlan.Domain != EvolutionDevelopingSelf || candidatePlan.Disposition != EvolutionAccepted {
			continue
		}
		candidate := proposal.DevelopingSelfCandidates[candidatePlan.Index]
		item := map[string]any{
			"category": candidate.Category, "claim": candidate.Claim, "value": map[string]any{"statement": candidate.Claim},
			"confidence": candidate.Confidence, "evidence_refs": stringSliceAny(candidate.EvidenceRefs),
			"provenance": map[string]any{"source": "reflection_v2", "proposal_id": plan.ProposalID}, "status": "active",
		}
		if err := a.applyDevelopingSelfCandidateTx(ctx, tx, fluctlightID, item, stringSliceAny(candidate.EvidenceRefs), allowed, plan.SourceWindow); err != nil {
			return err
		}
	}
	return nil
}

func reflectionAllowedEvidence(evolution EvolutionContext) map[string]struct{} {
	result := make(map[string]struct{}, len(evolution.Evidence)+len(evolution.ReferenceIndex.ByRef))
	for _, evidence := range evolution.Evidence {
		result[evidence.Ref] = struct{}{}
	}
	for ref := range evolution.ReferenceIndex.ByRef {
		result[ref] = struct{}{}
	}
	for _, outcome := range evolution.Outcomes {
		result[outcome.ID] = struct{}{}
	}
	return result
}
