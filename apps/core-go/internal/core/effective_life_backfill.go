package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type EffectiveLifeBackfillItem struct {
	FluctlightID string   `json:"fluctlight_id"`
	Status       string   `json:"status"`
	Missing      []string `json:"missing,omitempty"`
	Unresolved   []string `json:"unresolved,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

// BackfillEffectiveLife creates only missing current-domain baselines. Existing
// body, wardrobe and habit revisions are never replaced with Foundation text.
// Preview performs no writes or model calls; later portrait recompilation is a
// separate explicit operation because it may consume paid Provider capacity.
func (a *App) BackfillEffectiveLife(ctx context.Context, ownerID string, fluctlightIDs []string, all, apply bool) ([]EffectiveLifeBackfillItem, error) {
	if a == nil || a.DB == nil || strings.TrimSpace(ownerID) == "" || (all == (len(fluctlightIDs) > 0)) {
		return nil, ErrInvalidArguments
	}
	ids := append([]string(nil), fluctlightIDs...)
	if all {
		rows, err := a.DB.Pool().Query(ctx, `SELECT id FROM public.fluctlights WHERE created_by_actor_id=$1 ORDER BY id`, ownerID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	report := make([]EffectiveLifeBackfillItem, 0, len(ids))
	for _, id := range ids {
		item := EffectiveLifeBackfillItem{FluctlightID: id}
		resource, err := a.DB.GetFluctlight(ctx, id, ownerID)
		if err != nil {
			item.Status, item.Reason = "failed", err.Error()
			report = append(report, item)
			continue
		}
		item.Missing, item.Unresolved, err = inspectEffectiveLifeBackfill(ctx, a.DB.Pool(), resource)
		if err != nil {
			item.Status, item.Reason = "failed", err.Error()
			report = append(report, item)
			continue
		}
		if len(item.Missing) == 0 {
			item.Status = "skipped"
			report = append(report, item)
			continue
		}
		if !apply {
			item.Status = "would_initialize"
			report = append(report, item)
			continue
		}
		err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			var currentRevision int
			var rawCore []byte
			if err := tx.QueryRow(ctx, `SELECT current_revision,core_persona FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR SHARE`, id, ownerID).Scan(&currentRevision, &rawCore); err != nil {
				return err
			}
			if currentRevision != resource.CurrentRevision || stableDigest(jsonString(decodeObject(rawCore))) != stableDigest(jsonString(resource.CorePersona)) {
				return ErrConflict
			}
			return initializeEffectiveLifeTx(ctx, tx, id, resource.CorePersona)
		})
		if err != nil {
			item.Status, item.Reason = "failed", err.Error()
		} else {
			item.Status = "initialized"
		}
		report = append(report, item)
	}
	return report, nil
}

func inspectEffectiveLifeBackfill(ctx context.Context, query DBTX, resource Fluctlight) ([]string, []string, error) {
	missing := make([]string, 0)
	for _, table := range []struct{ name, label string }{
		{"fluctlight_appearance_states", "appearance_state"},
		{"fluctlight_wardrobe_states", "wardrobe_state"},
	} {
		var exists bool
		if err := query.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.`+table.name+` WHERE fluctlight_id=$1)`, resource.ID).Scan(&exists); err != nil {
			return nil, nil, err
		}
		if !exists {
			missing = append(missing, table.label)
		}
	}
	for _, profileID := range declaredPersonaProfileIDs(resource.CorePersona) {
		var exists bool
		if err := query.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_profile_habits WHERE fluctlight_id=$1 AND profile_id=$2)`, resource.ID, profileID).Scan(&exists); err != nil {
			return nil, nil, err
		}
		if !exists {
			missing = append(missing, "habits:"+profileID)
		}
	}
	unresolved := make([]string, 0)
	appearance := mapValue(mapValue(resource.CorePersona["life_profile"])["appearance"])
	physical := mapValue(appearance["physical_features"])
	if strings.TrimSpace(stringValue(appearance["description"])) != "" && strings.TrimSpace(stringValue(physical["hair_length"])) == "" && strings.TrimSpace(stringValue(physical["hair_color"])) == "" {
		unresolved = append(unresolved, "appearance_description_requires_semantic_classification")
	}
	if len(arrayValue(appearance["daily_outfit_preferences"])) > 0 && len(arrayValue(appearance["wardrobe_items"])) == 0 {
		unresolved = append(unresolved, "outfit_preference_does_not_prove_ownership")
	}
	return missing, unresolved, nil
}

func (a *App) VerifyEffectiveLifeReady(ctx context.Context) error {
	if a == nil || a.DB == nil {
		return errors.New("effective_life_store_unavailable")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_by_actor_id,core_persona FROM public.fluctlights ORDER BY id`)
	if err != nil {
		return err
	}
	type row struct {
		id, owner string
		core      []byte
	}
	instances := make([]row, 0)
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.owner, &item.core); err != nil {
			rows.Close()
			return err
		}
		instances = append(instances, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range instances {
		missing, _, err := inspectEffectiveLifeBackfill(ctx, a.DB.Pool(), Fluctlight{ID: item.id, CorePersona: decodeObject(item.core)})
		if err != nil {
			return err
		}
		if len(missing) > 0 {
			return fmt.Errorf("effective_life_not_ready: fluctlight=%s missing=%s; run effective-life-backfill --owner %s --all --apply before serving", item.id, strings.Join(missing, ","), item.owner)
		}
	}
	return nil
}
