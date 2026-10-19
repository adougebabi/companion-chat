package core

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type wardrobeOutfitWrite struct {
	FluctlightID, ProfileID, OutfitID, Name, OperationID, SourceRef string
	ItemIDs                                                         []string
	ExpectedWardrobeRevision, ExpectedOutfitRevision                int
	DefinitionSource                                                *worldPublication
}

const inventoryOutfitProjection = `to_jsonb(o)||jsonb_build_object('items',(SELECT COALESCE(jsonb_agg(to_jsonb(i)||jsonb_build_object('saved_slot',r.slot,'definition_slot_matches',r.slot=i.slot) ORDER BY r.slot),'[]'::jsonb) FROM public.fluctlight_wardrobe_outfit_items r JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=r.fluctlight_id AND i.id=r.item_id WHERE r.fluctlight_id=o.fluctlight_id AND r.outfit_id=o.id))`

func readInventoryOutfitWith(ctx context.Context, q lifeContextQuerier, fid, id string) (map[string]any, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT `+inventoryOutfitProjection+` FROM public.fluctlight_wardrobe_outfits o WHERE o.fluctlight_id=$1 AND o.id=$2`, fid, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}

// The registered Tool and explicit Owner catalogue editor share this domain
// writer. Neither saving an outfit nor its revisions changes current wearing.
func (s *WardrobeService) saveOutfitTx(ctx context.Context, tx pgx.Tx, cmd wardrobeOutfitWrite) (map[string]any, error) {
	fail := func(code string, retry bool, err error) (map[string]any, error) {
		return nil, newCapabilityError(code, retry, err)
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" || len([]rune(name)) > 128 || len(cmd.ItemIDs) < 1 || len(cmd.ItemIDs) > 16 {
		return fail("outfit_fields_invalid", false, ErrInvalidArguments)
	}
	var wardrobeRevision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, cmd.FluctlightID).Scan(&wardrobeRevision); err != nil {
		return fail("wardrobe_state_unavailable", true, err)
	}
	if wardrobeRevision != cmd.ExpectedWardrobeRevision {
		return fail("wardrobe_revision_conflict", true, ErrConflict)
	}
	outfitID := strings.TrimSpace(cmd.OutfitID)
	if outfitID == "" {
		outfitID = "outfit_" + stableDigest(cmd.FluctlightID+"\x1f"+cmd.ProfileID+"\x1f"+cmd.OperationID)
	}
	items := map[string]string{}
	seen := map[string]bool{}
	for _, id := range cmd.ItemIDs {
		if seen[id] {
			return fail("outfit_item_duplicate", false, ErrInvalidArguments)
		}
		seen[id] = true
		var slot, kind string
		err := tx.QueryRow(ctx, `SELECT slot,item_kind FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 AND archived_at IS NULL FOR SHARE`, cmd.FluctlightID, id).Scan(&slot, &kind)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail("wardrobe_item_not_found", false, ErrNotFound)
		}
		if err != nil {
			return fail("wardrobe_item_read_failed", true, err)
		}
		if kind != "wearable" {
			return fail("item_not_wearable", false, ErrInvalidArguments)
		}
		if _, duplicate := items[slot]; duplicate {
			return fail("outfit_slot_duplicate", false, ErrInvalidArguments)
		}
		items[slot] = id
	}
	var before map[string]any
	if cmd.ExpectedOutfitRevision == 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_outfits(id,fluctlight_id,profile_id,name,revision)VALUES($1,$2,$3,$4,1)`, outfitID, cmd.FluctlightID, cmd.ProfileID, name); err != nil {
			return fail("outfit_insert_failed", true, err)
		}
	} else {
		var revision int
		var profile string
		err := tx.QueryRow(ctx, `SELECT profile_id,revision FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND id=$2 FOR UPDATE`, cmd.FluctlightID, outfitID).Scan(&profile, &revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail("outfit_not_found", false, ErrNotFound)
		}
		if err != nil {
			return fail("outfit_read_failed", true, err)
		}
		if profile != cmd.ProfileID || revision != cmd.ExpectedOutfitRevision {
			return fail("outfit_revision_conflict", true, ErrConflict)
		}
		before, err = readInventoryOutfitWith(ctx, tx, cmd.FluctlightID, outfitID)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_outfits SET name=$3,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND id=$2`, cmd.FluctlightID, outfitID, name); err != nil {
			return fail("outfit_update_failed", true, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_wardrobe_outfit_items WHERE fluctlight_id=$1 AND outfit_id=$2`, cmd.FluctlightID, outfitID); err != nil {
			return fail("outfit_update_failed", true, err)
		}
	}
	for slot, id := range items {
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_outfit_items(fluctlight_id,outfit_id,slot,item_id)VALUES($1,$2,$3,$4)`, cmd.FluctlightID, outfitID, slot, id); err != nil {
			return fail("outfit_item_insert_failed", true, err)
		}
	}
	if err := refreshOutfitDefinitionSourceTx(ctx, tx, cmd.FluctlightID, outfitID, cmd.DefinitionSource); err != nil {
		return nil, err
	}
	next := wardrobeRevision + 1
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=$2,updated_at=now() WHERE fluctlight_id=$1`, cmd.FluctlightID, next); err != nil {
		return fail("outfit_state_failed", true, err)
	}
	after, err := readInventoryOutfitWith(ctx, tx, cmd.FluctlightID, outfitID)
	if err != nil {
		return nil, err
	}
	var previous any
	if before != nil {
		previous = jsonBytes(before)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_outfit_revisions(id,fluctlight_id,outfit_id,profile_id,revision,source_ref,before_value,after_value)VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, randomID("outfit_revision_"), cmd.FluctlightID, outfitID, cmd.ProfileID, cmd.ExpectedOutfitRevision+1, cmd.SourceRef, previous, jsonBytes(after)); err != nil {
		return nil, err
	}
	if err := appendOutboxTx(ctx, tx, "wardrobe.outfit.saved", "wardrobe_outfit", outfitID, cmd.FluctlightID, cmd.SourceRef, "wardrobe-outfit:"+outfitID, "wardrobe-outfit-save:"+outfitID+":"+fmt.Sprint(cmd.ExpectedOutfitRevision+1), map[string]any{"outfit_id": outfitID, "revision": cmd.ExpectedOutfitRevision + 1}); err != nil {
		return fail("outfit_event_failed", true, err)
	}
	return map[string]any{"outfit_id": outfitID, "revision": cmd.ExpectedOutfitRevision + 1, "wardrobe_revision": next, "item_count": len(items)}, nil
}

func recordWearingTransitionTx(ctx context.Context, tx pgx.Tx, fid, sourceKind, sourceRef string, revision int, before []map[string]any, at time.Time) error {
	after, err := readCurrentWornItems(ctx, tx, fid)
	if err != nil {
		return err
	}
	if jsonString(before) == jsonString(after) {
		return nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_wearing_history(id,fluctlight_id,wardrobe_revision,source_kind,source_ref,before_items,after_items,occurred_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, randomID("wearing_history_"), fid, revision, sourceKind, sourceRef, jsonBytes(before), jsonBytes(after), at)
	return err
}
