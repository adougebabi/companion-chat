package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type WorkingPersonaBackfillItem struct {
	FluctlightID string `json:"fluctlight_id"`
	ProfileID    string `json:"profile_id"`
	Status       string `json:"status"`
	Reason       string `json:"reason,omitempty"`
}

// VerifyWorkingPersonasReady is a deployment gate. A rule/budget upgrade is
// prepared by the maintenance command before API or Worker begins serving;
// ordinary turns never perform paid emergency compilation.
func (a *App) VerifyWorkingPersonasReady(ctx context.Context) error {
	if a == nil || a.DB == nil {
		return errors.New("working_persona_store_unavailable")
	}
	if err := a.VerifyEffectiveLifeReady(ctx); err != nil {
		return err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT DISTINCT created_by_actor_id FROM public.fluctlights ORDER BY created_by_actor_id`)
	if err != nil {
		return err
	}
	owners := make([]string, 0)
	for rows.Next() {
		var owner string
		if err := rows.Scan(&owner); err != nil {
			rows.Close()
			return err
		}
		owners = append(owners, owner)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, owner := range owners {
		report, err := a.BackfillWorkingPersonas(ctx, owner, nil, true, false)
		if err != nil {
			return err
		}
		for _, item := range report {
			if item.Status != "skipped" {
				return fmt.Errorf("working_persona_not_ready: fluctlight=%s profile=%s status=%s; run persona-backfill --owner %s --all --apply before serving", item.FluctlightID, item.ProfileID, item.Status, owner)
			}
		}
	}
	return nil
}

// BackfillWorkingPersonas is an explicit maintenance entry. It reads only the
// requested owner's instances and never changes active profile or dynamic
// state. Dry runs inspect every profile without making model calls or writes.
func (a *App) BackfillWorkingPersonas(ctx context.Context, ownerID string, fluctlightIDs []string, all, apply bool) ([]WorkingPersonaBackfillItem, error) {
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
	report := make([]WorkingPersonaBackfillItem, 0)
	for _, id := range ids {
		resource, err := a.DB.GetFluctlight(ctx, id, ownerID)
		if err != nil {
			report = append(report, WorkingPersonaBackfillItem{FluctlightID: id, Status: "failed", Reason: err.Error()})
			continue
		}
		var mode string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT initialization_mode FROM public.fluctlights WHERE id=$1`, id).Scan(&mode); err != nil {
			report = append(report, WorkingPersonaBackfillItem{FluctlightID: id, Status: "failed", Reason: err.Error()})
			continue
		}
		for _, profileID := range declaredPersonaProfileIDs(resource.CorePersona) {
			item := WorkingPersonaBackfillItem{FluctlightID: id, ProfileID: profileID}
			input, err := a.personaCompilationInputForProfile(ctx, id, resource.CorePersona, resource.CurrentRevision, profileID, true)
			if err != nil {
				item.Status, item.Reason = "failed", err.Error()
				report = append(report, item)
				continue
			}
			source, err := personaCompilationSource(input)
			if err != nil {
				item.Status, item.Reason = "failed", err.Error()
				report = append(report, item)
				continue
			}
			hash := stableDigest(jsonString(source))
			var savedRevision, savedOverlay, savedBudget int
			var savedHash, savedRules, savedStatus string
			err = a.DB.Pool().QueryRow(ctx, `SELECT source_revision,source_hash,overlay_revision,rules_version,budget_runes,status FROM public.fluctlight_working_personas WHERE fluctlight_id=$1 AND profile_id=$2`, id, profileID).Scan(&savedRevision, &savedHash, &savedOverlay, &savedRules, &savedBudget, &savedStatus)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				item.Status, item.Reason = "failed", err.Error()
				report = append(report, item)
				continue
			}
			if err == nil && savedRevision == resource.CurrentRevision && savedHash == hash && savedOverlay == input.OverlayRevision && savedRules == personaCompilationRulesVersion && savedBudget == input.TargetBudgetRunes && savedStatus == "completed" {
				item.Status, item.Reason = "skipped", "current"
				report = append(report, item)
				continue
			}
			if !apply {
				item.Status, item.Reason = "would_compile", "missing_or_stale"
				report = append(report, item)
				continue
			}
			compiled, err := a.compileOneWorkingPersona(ctx, input, mode)
			if err != nil {
				item.Status, item.Reason = "failed", err.Error()
				report = append(report, item)
				continue
			}
			err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
				var currentRevision int
				var currentCore []byte
				if err := tx.QueryRow(ctx, `SELECT current_revision,core_persona FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR SHARE`, id, ownerID).Scan(&currentRevision, &currentCore); err != nil {
					return err
				}
				if currentRevision != input.SourceRevision || stableDigest(jsonString(decodeObject(currentCore))) != stableDigest(jsonString(input.CorePersona)) {
					return ErrConflict
				}
				baseline := personaEvolutionBaseline(id, mapValue(input.CorePersona["personality"]), mapValue(input.CorePersona["behavioral_policy"]), mapValue(input.CorePersona["personality_system"]), map[string]any{"active_profile_id": profileID})
				currentState, err := loadPersonaEvolutionState(ctx, tx, baseline)
				if err != nil {
					return err
				}
				if portraitOverlayRevision(currentState) != input.OverlayRevision {
					return ErrConflict
				}
				if err := verifyCompiledBudgetTx(ctx, tx, []CompiledWorkingPersona{compiled}); err != nil {
					return err
				}
				return insertCompiledWorkingPersonasTx(ctx, tx, id, []CompiledWorkingPersona{compiled})
			})
			if err != nil {
				item.Status, item.Reason = "failed", fmt.Sprintf("publish: %v", err)
			} else {
				item.Status = "compiled"
			}
			report = append(report, item)
		}
	}
	return report, nil
}
