package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// recordRelationshipInteractionTx records the durable interaction fact for an
// already-established directed Relationship. It intentionally updates only
// interaction metadata; semantic metrics/trend remain LLM/reflection-owned.
func (a *App) recordRelationshipInteractionTx(ctx context.Context, tx pgx.Tx, fluctlightID, targetActorID string, meaningful bool) error {
	targetActorID = strings.TrimSpace(targetActorID)
	if targetActorID == "" {
		return nil
	}
	var activeProfile string
	_ = tx.QueryRow(ctx, `SELECT COALESCE(active_profile_id,'default') FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&activeProfile)
	var relationshipID string
	if err := tx.QueryRow(ctx, `SELECT id FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2 AND (profile_id=$3 OR profile_id IS NULL) ORDER BY CASE WHEN profile_id=$3 THEN 0 ELSE 1 END,updated_at DESC LIMIT 1 FOR UPDATE`, fluctlightID, targetActorID, activeProfile).Scan(&relationshipID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// An unknown relationship remains unknown; an interaction alone must
			// not invent a semantic relationship row.
			return nil
		}
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE public.relationships SET interaction_frequency=interaction_frequency+1,last_interaction_at=now(),last_meaningful_interaction_at=CASE WHEN $2 THEN now() ELSE last_meaningful_interaction_at END,updated_at=now() WHERE id=$1`, relationshipID, meaningful)
	return err
}

func normalizeRelationshipRole(value any) (map[string]any, error) {
	role := mapValue(value)
	if len(role) == 0 {
		return map[string]any{"label": "unknown", "addressing": map[string]any{}}, nil
	}
	label := strings.TrimSpace(stringValue(role["label"]))
	if label == "" {
		label = strings.TrimSpace(stringValue(role["primary"]))
	}
	if len([]rune(label)) > 256 {
		return nil, errors.New("relationship_role_label_invalid")
	}
	result := map[string]any{}
	if label != "" {
		result["label"] = label
	}
	if category := strings.TrimSpace(stringValue(role["category"])); category != "" {
		if len([]rune(category)) > 128 {
			return nil, errors.New("relationship_role_category_invalid")
		}
		result["category"] = category
	}
	if secondary := arrayValue(role["secondary"]); len(secondary) > 0 {
		labels := make([]any, 0, len(secondary))
		for _, raw := range secondary {
			value := strings.TrimSpace(stringValue(raw))
			if value != "" && len([]rune(value)) <= 128 {
				labels = append(labels, value)
			}
		}
		if len(labels) > 0 {
			result["secondary"] = labels
		}
	}
	if addressing := mapValue(role["addressing"]); len(addressing) > 0 {
		preferred := strings.TrimSpace(stringValue(addressing["preferred"]))
		if len([]rune(preferred)) > 128 {
			return nil, errors.New("relationship_addressing_invalid")
		}
		if preferred != "" {
			result["addressing"] = map[string]any{"preferred": preferred}
			if selfReference := strings.TrimSpace(stringValue(addressing["self_reference"])); selfReference != "" && len([]rune(selfReference)) <= 128 {
				result["addressing"].(map[string]any)["self_reference"] = selfReference
			}
		}
	}
	if len(result) == 0 {
		return nil, errors.New("relationship_role_empty")
	}
	return result, nil
}

func validateRelationshipMetrics(value any) (map[string]any, error) {
	metrics := mapValue(value)
	if len(metrics) == 0 {
		return map[string]any{}, nil
	}
	for key, raw := range metrics {
		if strings.TrimSpace(key) == "" {
			return nil, errors.New("relationship_metric_key_invalid")
		}
		parsed, ok := numberFloat(raw)
		if !ok || parsed < 0 || parsed > 1 {
			return nil, errors.New("relationship_metric_value_invalid")
		}
	}
	return metrics, nil
}

func (a *App) EditRelationship(ctx context.Context, actorID, fluctlightID, targetActorID string, payload map[string]any) (map[string]any, error) {
	targetActorID = strings.TrimSpace(targetActorID)
	if targetActorID == "" {
		return nil, errors.New("relationship_target_invalid")
	}
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, ErrUnauthorized
	}
	expected, ok := nonNegativeRevision(payload["expected_revision"])
	if !ok {
		return nil, errors.New("relationship_expected_revision_invalid")
	}
	evidence := arrayValue(payload["evidence_refs"])
	if len(evidence) == 0 {
		return nil, errors.New("relationship_evidence_required")
	}
	profileID := strings.TrimSpace(stringValue(payload["profile_id"]))
	if profileID == "" {
		_ = a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(active_profile_id,'default') FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&profileID)
		if profileID == "" {
			profileID = "default"
		}
	}

	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var id, trend string
		var revision int
		var role, metrics, emotional, provenance []byte
		var summary *string
		var storedProfileID *string
		if err := tx.QueryRow(ctx, `SELECT id,profile_id,role,metrics,trend,summary,emotional_association,provenance,revision FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2 AND (profile_id=$3 OR profile_id IS NULL) ORDER BY CASE WHEN profile_id=$3 THEN 0 ELSE 1 END LIMIT 1 FOR UPDATE`, fluctlightID, targetActorID, profileID).Scan(&id, &storedProfileID, &role, &metrics, &trend, &summary, &emotional, &provenance, &revision); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if revision != expected {
			return ErrConflict
		}

		roleValue := decodeObject(role)
		metricsValue := decodeObject(metrics)
		emotionalValue := decodeObject(emotional)
		if raw, exists := payload["role"]; exists {
			normalizedRole, roleErr := normalizeRelationshipRole(raw)
			if roleErr != nil {
				return roleErr
			}
			roleValue = normalizedRole
		}
		if raw, exists := payload["metrics"]; exists {
			normalizedMetrics, metricsErr := validateRelationshipMetrics(raw)
			if metricsErr != nil {
				return metricsErr
			}
			metricsValue = normalizedMetrics
		}
		if raw, exists := payload["emotional_association"]; exists {
			emotionalValue = mapValue(raw)
		}
		if raw, exists := payload["trend"]; exists {
			trend = strings.TrimSpace(stringValue(raw))
			if trend != "improving" && trend != "stable" && trend != "declining" {
				return errors.New("relationship_trend_invalid")
			}
		}
		if raw, exists := payload["summary"]; exists {
			value := strings.TrimSpace(stringValue(raw))
			if len([]rune(value)) > 32000 {
				return errors.New("relationship_summary_invalid")
			}
			if value == "" {
				summary = nil
			} else {
				summary = &value
			}
		}
		newRevision := revision + 1
		provenanceValue := map[string]any{"source": "manual", "actor_id": actorID, "evidence_refs": evidence}
		if _, err := tx.Exec(ctx, `UPDATE public.relationships SET role=$2,metrics=$3,trend=$4,summary=$5,emotional_association=$6,provenance=$7,revision=$8,updated_at=now() WHERE id=$1 AND revision=$9`, id, jsonBytes(roleValue), jsonBytes(metricsValue), trend, summary, jsonBytes(emotionalValue), jsonBytes(provenanceValue), newRevision, expected); err != nil {
			return err
		}
		revisionID := randomID("relationship_revision_")
		idempotency := "relationship-edit:" + id + ":" + fmt.Sprint(expected) + ":" + stableDigest(jsonString(payload))
		if _, err := tx.Exec(ctx, `INSERT INTO public.relationship_revisions(id,relationship_id,revision,base_revision,role,metrics,trend,summary,emotional_association,evidence_refs,actor_id,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, revisionID, id, newRevision, expected, jsonBytes(roleValue), jsonBytes(metricsValue), trend, summary, jsonBytes(emotionalValue), jsonBytes(evidence), actorID, idempotency); err != nil {
			return err
		}
		reason := strings.TrimSpace(stringValue(payload["reason"]))
		if _, err := tx.Exec(ctx, `INSERT INTO public.relationship_governance(id,relationship_id,revision_id,action,actor_id,reason) VALUES($1,$2,$3,'edit',$4,$5)`, randomID("relationship_governance_"), id, revisionID, actorID, nullableString(reason)); err != nil {
			return err
		}
		result = map[string]any{"id": id, "relationship_id": id, "profile_id": profileID, "target_actor_id": targetActorID, "revision": newRevision, "role": roleValue, "metrics": metricsValue, "trend": trend, "summary": summary, "emotional_association": emotionalValue, "provenance": provenanceValue, "status": "updated"}
		if storedProfileID != nil && strings.TrimSpace(*storedProfileID) != "" {
			result["profile_id"] = *storedProfileID
		}
		_ = provenance
		return nil
	})
	return result, err
}
