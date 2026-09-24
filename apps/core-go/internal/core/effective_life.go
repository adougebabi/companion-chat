package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// appearanceField carries an explicit known/cleared/unknown state. Omitting a
// field from the initial card means unknown, not a reason to restore a later
// cleared value from the Foundation.
func initialAppearanceState(corePersona map[string]any) map[string]any {
	appearance := mapValue(mapValue(corePersona["life_profile"])["appearance"])
	physical := mapValue(appearance["physical_features"])
	fields := map[string]any{}
	for _, field := range []struct {
		name  string
		value any
	}{
		{"hair_length", physical["hair_length"]},
		{"hair_color", physical["hair_color"]},
		{"hair_style", appearance["hair_style"]},
		{"chest_cup", appearance["chest_cup"]},
	} {
		if value := strings.TrimSpace(stringValue(field.value)); value != "" {
			fields[field.name] = map[string]any{"status": "known", "value": value}
		}
	}
	if injuries, ok := appearance["injuries"].([]any); ok && len(injuries) > 0 {
		fields["injuries"] = map[string]any{"status": "known", "value": injuries}
	}
	return fields
}

func initializeEffectiveLifeTx(ctx context.Context, tx pgx.Tx, fluctlightID string, corePersona map[string]any) error {
	if tx == nil || strings.TrimSpace(fluctlightID) == "" {
		return errors.New("effective_life_initialization_scope_invalid")
	}
	sourceRef := "foundation_" + stableDigest(fluctlightID)
	body := initialAppearanceState(corePersona)
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_appearance_states(fluctlight_id,revision,state_json,source_kind,source_ref) VALUES($1,0,$2,'initialization',$3) ON CONFLICT(fluctlight_id) DO NOTHING`, fluctlightID, jsonBytes(body), sourceRef); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_appearance_revisions(fluctlight_id,revision,state_json,source_kind,source_ref) VALUES($1,0,$2,'initialization',$3) ON CONFLICT(fluctlight_id,revision) DO NOTHING`, fluctlightID, jsonBytes(body), sourceRef); err != nil {
		return err
	}
	wardrobeCommand, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_states(fluctlight_id,revision) VALUES($1,0) ON CONFLICT(fluctlight_id) DO NOTHING`, fluctlightID)
	if err != nil {
		return err
	}
	newWardrobeState := wardrobeCommand.RowsAffected() == 1
	life := mapValue(corePersona["life_profile"])
	appearance := mapValue(life["appearance"])
	if raw, present := appearance["wardrobe_items"]; newWardrobeState && present && raw != nil {
		items, ok := raw.([]any)
		if !ok {
			return errors.New("initial_wardrobe_items_must_be_array")
		}
		wornSlots := map[string]struct{}{}
		for index, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				return fmt.Errorf("initial_wardrobe_item_%d_invalid", index)
			}
			category := strings.TrimSpace(stringValue(item["category"]))
			slot := strings.TrimSpace(stringValue(item["slot"]))
			description := strings.TrimSpace(stringValue(item["description"]))
			if category == "" || slot == "" || description == "" || len([]rune(category)) > 64 || len([]rune(slot)) > 64 || len([]rune(description)) > 512 {
				return fmt.Errorf("initial_wardrobe_item_%d_fields_invalid", index)
			}
			ownership := firstString(stringValue(item["ownership"]), "unknown")
			if ownership != "owned" && ownership != "borrowed" && ownership != "unknown" {
				return fmt.Errorf("initial_wardrobe_item_%d_ownership_invalid", index)
			}
			available := true
			if value, exists := item["available"]; exists {
				flag, ok := value.(bool)
				if !ok {
					return fmt.Errorf("initial_wardrobe_item_%d_availability_invalid", index)
				}
				available = flag
			}
			worn := false
			if value, exists := item["currently_worn"]; exists {
				flag, ok := value.(bool)
				if !ok {
					return fmt.Errorf("initial_wardrobe_item_%d_worn_invalid", index)
				}
				worn = flag
			}
			if worn && !available {
				return fmt.Errorf("initial_wardrobe_item_%d_unavailable_but_worn", index)
			}
			if worn {
				if _, duplicate := wornSlots[slot]; duplicate {
					return fmt.Errorf("initial_wardrobe_slot_%s_duplicate", slot)
				}
				wornSlots[slot] = struct{}{}
			}
			key := fmt.Sprintf("%d", index)
			itemID := "wardrobe_" + stableDigest(fluctlightID+"\x1f"+sourceRef+"\x1f"+key)
			availability := "available"
			if !available {
				availability = "unavailable"
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key) VALUES($1,$2,$3,$4,$5,$6,$7,'initialization',$8,$9) ON CONFLICT(id) DO NOTHING`, itemID, fluctlightID, category, slot, description, ownership, availability, sourceRef, key); err != nil {
				return err
			}
			if worn && newWardrobeState {
				if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id) VALUES($1,$2,$3) ON CONFLICT(fluctlight_id,slot) DO NOTHING`, fluctlightID, slot, itemID); err != nil {
					return err
				}
			}
		}
		if len(wornSlots) > 0 && newWardrobeState {
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET wearing_state='known' WHERE fluctlight_id=$1`, fluctlightID); err != nil {
				return err
			}
		}
	}
	initialHabits := arrayValue(life["life_habits"])
	if initialHabits == nil {
		initialHabits = []any{}
	}
	for _, profileID := range declaredPersonaProfileIDs(corePersona) {
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_profile_habits(fluctlight_id,profile_id,revision,habits_json,source_kind,source_ref) VALUES($1,$2,0,$3,'initialization',$4) ON CONFLICT(fluctlight_id,profile_id) DO NOTHING`, fluctlightID, profileID, jsonBytes(initialHabits), sourceRef); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_profile_habit_revisions(fluctlight_id,profile_id,revision,habits_json,source_kind,source_ref) VALUES($1,$2,0,$3,'initialization',$4) ON CONFLICT(fluctlight_id,profile_id,revision) DO NOTHING`, fluctlightID, profileID, jsonBytes(initialHabits), sourceRef); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) readEffectiveLifeSnapshot(ctx context.Context, fluctlightID string, at time.Time) (map[string]any, int, int, error) {
	if a == nil || a.DB == nil {
		return nil, 0, 0, errors.New("effective_life_unavailable")
	}
	tx, err := a.DB.Pool().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, 0, err
	}
	defer tx.Rollback(ctx)
	bodyRevision := -1
	wardrobeRevision := -1
	fields := map[string]any{}
	var encoded []byte
	err = tx.QueryRow(ctx, `SELECT revision,state_json FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&bodyRevision, &encoded)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, 0, err
	}
	if err == nil {
		fields = decodeObject(encoded)
	}
	for _, key := range []string{"hair_length", "hair_color", "hair_style", "injuries"} {
		if _, exists := fields[key]; !exists {
			fields[key] = map[string]any{"status": "unknown"}
		}
	}
	wearingState := "unknown"
	err = tx.QueryRow(ctx, `SELECT revision,wearing_state FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&wardrobeRevision, &wearingState)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, 0, err
	}
	worn, err := readCurrentWornItems(ctx, tx, fluctlightID)
	if err != nil {
		return nil, 0, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 0, 0, err
	}
	appearance := map[string]any{
		"body_revision": bodyRevision, "body_fields": fields,
		"wardrobe_revision": wardrobeRevision, "wearing_state": wearingState,
		"worn_items": worn, "captured_at": at.UTC().Format(time.RFC3339Nano),
	}
	return appearance, bodyRevision, wardrobeRevision, nil
}

func (a *App) effectiveLifeRevisionsMatch(ctx context.Context, fluctlightID string, bodyRevision, wardrobeRevision int) (bool, error) {
	var body, wardrobe int
	err := a.DB.Pool().QueryRow(ctx, `SELECT COALESCE((SELECT revision FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1),-1),COALESCE((SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1),-1)`, fluctlightID).Scan(&body, &wardrobe)
	if err != nil {
		return false, err
	}
	return body == bodyRevision && wardrobe == wardrobeRevision, nil
}

func readProfileHabits(ctx context.Context, query DBTX, fluctlightID, profileID string) ([]any, int, error) {
	var revision int
	var raw []byte
	err := query.QueryRow(ctx, `SELECT revision,habits_json FROM public.fluctlight_profile_habits WHERE fluctlight_id=$1 AND profile_id=$2`, fluctlightID, profileID).Scan(&revision, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, errors.New("effective_habits_missing")
	}
	if err != nil {
		return nil, 0, err
	}
	var habits []any
	if err := json.Unmarshal(raw, &habits); err != nil {
		return nil, 0, err
	}
	if habits == nil {
		habits = []any{}
	}
	return habits, revision, nil
}

func (a *App) readActiveLifeActivities(ctx context.Context, fluctlightID string, at time.Time) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,profile_id,kind,status,COALESCE(intention_id,''),started_at,not_before,result_json FROM public.fluctlight_life_activity_runs WHERE fluctlight_id=$1 AND status IN ('scheduled','in_progress','deferred') ORDER BY not_before,id LIMIT 12`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	activities := make([]map[string]any, 0)
	for rows.Next() {
		var id, profileID, kind, status, intentionID string
		var started *time.Time
		var notBefore time.Time
		var raw []byte
		if err := rows.Scan(&id, &profileID, &kind, &status, &intentionID, &started, &notBefore, &raw); err != nil {
			return nil, err
		}
		entry := map[string]any{"id": id, "profile_id": profileID, "kind": kind, "status": status,
			"not_before": notBefore.UTC().Format(time.RFC3339Nano), "ready_for_resolution": !at.Before(notBefore),
			"request": mapValue(decodeObject(raw)["request"])}
		if started != nil {
			entry["started_at"] = started.UTC().Format(time.RFC3339Nano)
		}
		if intentionID != "" {
			entry["intention_id"] = intentionID
		}
		activities = append(activities, entry)
	}
	return activities, rows.Err()
}
