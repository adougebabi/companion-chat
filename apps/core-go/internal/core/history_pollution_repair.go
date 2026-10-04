package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
)

// A manifest identifies exact rows and their evidence qualification. Text is
// never classified by keywords; unknown or mixed provenance remains a reason
// to inspect, not permission to rewrite an entire history.
type HistoryRepairEntry struct {
	FluctlightID     string `json:"fluctlight_id"`
	Kind             string `json:"kind"`
	ID               string `json:"id"`
	ExpectedRevision int    `json:"expected_revision"`
	Qualification    string `json:"qualification"`
}
type HistoryRepairPlanItem struct {
	Entry   HistoryRepairEntry `json:"entry"`
	Before  map[string]any     `json:"before"`
	Sources []map[string]any   `json:"sources"`
}
type HistoryRepairPlan struct {
	OwnerID              string                  `json:"owner_id"`
	Reason               string                  `json:"reason"`
	Digest               string                  `json:"digest"`
	ClassificationCounts map[string]int          `json:"classification_counts"`
	Items                []HistoryRepairPlanItem `json:"items"`
}

func (a *App) PlanHistoryPollutionRepair(ctx context.Context, owner string, entries []HistoryRepairEntry, reason string) (HistoryRepairPlan, error) {
	plan := HistoryRepairPlan{OwnerID: owner, Reason: strings.TrimSpace(reason), Items: []HistoryRepairPlanItem{}, ClassificationCounts: map[string]int{}}
	if owner == "" || plan.Reason == "" || len(entries) == 0 || len(entries) > 100 {
		return plan, ErrInvalidArguments
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		key := entry.Kind + ":" + entry.ID
		if seen[key] {
			return plan, ErrInvalidArguments
		}
		seen[key] = true
		if _, err := a.DB.GetFluctlight(ctx, entry.FluctlightID, owner); err != nil {
			return plan, err
		}
		item, err := a.readRepairItem(ctx, a.DB.Pool(), entry)
		if err != nil {
			return plan, err
		}
		plan.Items = append(plan.Items, item)
		plan.ClassificationCounts[entry.Kind+":"+entry.Qualification]++
	}
	plan.Digest = historyRepairPlanDigest(plan)
	return plan, nil
}
func historyRepairPlanDigest(plan HistoryRepairPlan) string {
	plan.Digest = ""
	return stableDigest(jsonString(plan))
}
func (a *App) readRepairItem(ctx context.Context, q lifeContextQuerier, entry HistoryRepairEntry) (HistoryRepairPlanItem, error) {
	item := HistoryRepairPlanItem{Entry: entry, Sources: []map[string]any{}}
	table, ownerColumn := "", "fluctlight_id"
	switch entry.Kind {
	case "memory":
		table = "memories"
		ownerColumn = "owner_fluctlight_id"
	case "actor_fact":
		table = "actor_facts"
		ownerColumn = "owner_fluctlight_id"
	case "summary":
		table = "conversation_summaries"
		ownerColumn = "owner_fluctlight_id"
	case "developing_self":
		table = "fluctlight_developing_self_claims"
	case "inventory":
		table = "fluctlight_wardrobe_items"
	default:
		return item, ErrInvalidArguments
	}
	var raw []byte
	if err := q.QueryRow(ctx, `SELECT to_jsonb(target) FROM public.`+table+` target WHERE id=$1 AND `+ownerColumn+`=$2`, entry.ID, entry.FluctlightID).Scan(&raw); err != nil {
		return item, err
	}
	item.Before = decodeObject(raw)
	if entry.Kind == "inventory" {
		if err := addRepairInventoryContext(ctx, q, entry.FluctlightID, entry.ID, item.Before); err != nil {
			return item, err
		}
	}
	if intValue(item.Before["revision"]) != entry.ExpectedRevision {
		return item, ErrConflict
	}
	switch entry.Qualification {
	case "periodic_only":
		if entry.Kind != "memory" && entry.Kind != "developing_self" {
			return item, ErrInvalidArguments
		}
		refs := []string{}
		if entry.Kind == "memory" {
			rows, err := q.Query(ctx, `SELECT l.source_id FROM public.memory_source_links l WHERE l.memory_id=$1 AND l.memory_revision=$2 AND l.source_kind='fact' AND l.status='valid' AND public.memory_source_is_live(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint)`, entry.ID, entry.ExpectedRevision)
			if err != nil {
				return item, err
			}
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return item, err
				}
				refs = append(refs, id)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return item, err
			}
			var total int
			if err := q.QueryRow(ctx, `SELECT count(*) FROM public.memory_source_links WHERE memory_id=$1 AND memory_revision=$2`, entry.ID, entry.ExpectedRevision).Scan(&total); err != nil {
				return item, err
			}
			if total != len(refs) {
				return item, errors.New("repair_mixed_or_unknown_sources")
			}
		} else {
			refs = decisionServiceRefValues(item.Before["evidence_refs"])
		}
		if len(refs) == 0 {
			return item, errors.New("repair_sources_missing")
		}
		for _, ref := range refs {
			var sourceRaw []byte
			if strings.HasPrefix(ref, "sequence:") {
				if err := q.QueryRow(ctx, `SELECT to_jsonb(i) FROM public.cognition_inbox i WHERE fluctlight_id=$1 AND sequence=$2::integer`, entry.FluctlightID, strings.TrimPrefix(ref, "sequence:")).Scan(&sourceRaw); err != nil {
					return item, err
				}
			} else {
				ref = strings.TrimPrefix(ref, "fact:")
				if err := q.QueryRow(ctx, `SELECT to_jsonb(i) FROM public.cognition_inbox i WHERE fluctlight_id=$1 AND id=$2`, entry.FluctlightID, ref).Scan(&sourceRaw); err != nil {
					return item, err
				}
			}
			source := decodeObject(sourceRaw)
			if source["event_type"] != "internal.wake_up" {
				return item, errors.New("repair_not_periodic_only")
			}
			var actualWake bool
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_events WHERE fluctlight_id=$1 AND status='confirmed' AND kind IN ('wake','sleep_interrupt') AND evidence_refs ? $2)`, entry.FluctlightID, stringValue(source["id"])).Scan(&actualWake); err != nil {
				return item, err
			}
			if actualWake {
				return item, errors.New("repair_has_legitimate_wake_event")
			}
			item.Sources = append(item.Sources, source)
		}
	case "unverified":
		if entry.Kind == "memory" && item.Before["provenance_status"] != "legacy_unknown" && item.Before["provenance_status"] != "invalid" {
			return item, errors.New("repair_source_is_verified")
		}
		if entry.Kind == "inventory" {
			var confirmed bool
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_events WHERE id=$1 AND fluctlight_id=$2 AND status='confirmed')`, stringValue(item.Before["source_ref"]), entry.FluctlightID).Scan(&confirmed); err != nil {
				return item, err
			}
			var ownerSource bool
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlights WHERE id=$1 AND $2::text='owner:'||created_by_actor_id)`, entry.FluctlightID, stringValue(item.Before["source_ref"])).Scan(&ownerSource); err != nil {
				return item, err
			}
			if confirmed || item.Before["source_kind"] == "initialization" || (item.Before["source_kind"] == "accepted_event" && ownerSource) {
				return item, errors.New("repair_inventory_has_controlled_source")
			}
		}
		if entry.Kind != "memory" && entry.Kind != "inventory" {
			return item, ErrInvalidArguments
		}
	case "explicit_invalidated":
		// The manifest is an explicit human decision about exact assertions. The
		// content is retained, and the immutable before snapshot is part of review.
		if entry.Kind == "memory" {
			return item, errors.New("repair_memory_requires_source_qualification")
		}
	default:
		return item, ErrInvalidArguments
	}
	if entry.Kind == "actor_fact" {
		if id := stringValue(item.Before["source_message_id"]); id != "" {
			var source []byte
			if err := q.QueryRow(ctx, `SELECT to_jsonb(m) FROM public.conversation_messages m WHERE id=$1`, id).Scan(&source); err != nil {
				return item, err
			}
			item.Sources = append(item.Sources, decodeObject(source))
		}
	}
	return item, nil
}
func (a *App) ApplyHistoryPollutionRepair(ctx context.Context, plan HistoryRepairPlan) (map[string]any, error) {
	if plan.Digest == "" || plan.Digest != historyRepairPlanDigest(plan) {
		return nil, ErrInvalidArguments
	}
	entries := make([]HistoryRepairEntry, 0, len(plan.Items))
	for _, item := range plan.Items {
		entries = append(entries, item.Entry)
	}
	batch := historyRepairManifestBatch(plan.OwnerID, entries, plan.Reason)
	batchStatus := "applied"
	replayed := false
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, batch); err != nil {
			return err
		}
		var status string
		var existingOwner string
		if err := tx.QueryRow(ctx, `SELECT status,owner_actor_id FROM public.history_repair_batches WHERE id=$1`, batch).Scan(&status, &existingOwner); err == nil {
			if existingOwner != plan.OwnerID {
				return ErrUnauthorized
			}
			replayed = true
			batchStatus = status
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// Validate every row before the first domain write; partial plans never
		// become partial successful repairs.
		for _, item := range plan.Items {
			var owner string
			if err := tx.QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1 FOR UPDATE`, item.Entry.FluctlightID).Scan(&owner); err != nil {
				return err
			}
			if owner != plan.OwnerID {
				return ErrUnauthorized
			}
			live, err := a.readRepairItem(ctx, tx, item.Entry)
			if err != nil {
				return err
			}
			if jsonString(live.Before) != jsonString(item.Before) || jsonString(live.Sources) != jsonString(item.Sources) {
				return errors.New("repair_plan_drift")
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.history_repair_batches(id,owner_actor_id,request_digest,reason,status) VALUES($1,$2,$3,$4,'applied')`, batch, plan.OwnerID, plan.Digest, plan.Reason); err != nil {
			return err
		}
		for _, item := range plan.Items {
			entry := item.Entry
			switch entry.Kind {
			case "memory":
				row, err := readMemoryAuthorityRowTx(ctx, tx, entry.ID)
				if err != nil {
					return err
				}
				command := buildOwnerMemoryCommand(row, plan.OwnerID, MemoryForget, entry.ExpectedRevision, []string{"history-repair:" + batch}, nil, plan.Reason, nil, nil)
				if _, err := a.applyMemoryCommandTx(ctx, tx, command); err != nil {
					return err
				}
			case "actor_fact":
				if _, err := tx.Exec(ctx, `UPDATE public.actor_facts SET status='quarantined',revision=revision+1 WHERE id=$1`, entry.ID); err != nil {
					return err
				}
				if err := retireActorFactArtifactsTx(ctx, tx, entry.FluctlightID, entry.ID); err != nil {
					return err
				}
			case "summary":
				if err := invalidateSummaryIDTx(ctx, tx, entry.FluctlightID, entry.ID); err != nil {
					return err
				}
			case "developing_self":
				if err := a.repairDevelopingSelfTx(ctx, tx, item.Before, "forgotten", plan.Reason, batch); err != nil {
					return err
				}
			case "inventory":
				if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_items SET availability='unavailable',revision=revision+1 WHERE id=$1`, entry.ID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, entry.FluctlightID, entry.ID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_item_uses WHERE fluctlight_id=$1 AND item_id=$2`, entry.FluctlightID, entry.ID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET wearing_state=CASE WHEN $2 THEN 'unknown' ELSE wearing_state END,revision=revision+1 WHERE fluctlight_id=$1`, entry.FluctlightID, len(arrayValue(item.Before["repair_worn_refs"])) > 0); err != nil {
					return err
				}
			}
			table, ownerColumn := "", "fluctlight_id"
			switch entry.Kind {
			case "memory":
				table = "memories"
				ownerColumn = "owner_fluctlight_id"
			case "actor_fact":
				table = "actor_facts"
				ownerColumn = "owner_fluctlight_id"
			case "summary":
				table = "conversation_summaries"
				ownerColumn = "owner_fluctlight_id"
			case "developing_self":
				table = "fluctlight_developing_self_claims"
			case "inventory":
				table = "fluctlight_wardrobe_items"
			}
			var after []byte
			if err := tx.QueryRow(ctx, `SELECT to_jsonb(t) FROM public.`+table+` t WHERE id=$1 AND `+ownerColumn+`=$2`, entry.ID, entry.FluctlightID).Scan(&after); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.history_repair_items(batch_id,fluctlight_id,kind,target_id,before_value,after_value,source_records,expected_revision,resulting_revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, batch, entry.FluctlightID, entry.Kind, entry.ID, jsonBytes(item.Before), after, jsonBytes(item.Sources), entry.ExpectedRevision, intValue(decodeObject(after)["revision"])); err != nil {
				return err
			}
		}
		for _, item := range plan.Items {
			if item.Entry.Kind != "inventory" {
				continue
			}
			var raw []byte
			if err := tx.QueryRow(ctx, `SELECT to_jsonb(i) FROM public.fluctlight_wardrobe_items i WHERE id=$1 AND fluctlight_id=$2`, item.Entry.ID, item.Entry.FluctlightID).Scan(&raw); err != nil {
				return err
			}
			after := decodeObject(raw)
			if err := addRepairInventoryContext(ctx, tx, item.Entry.FluctlightID, item.Entry.ID, after); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.history_repair_items SET after_value=$4 WHERE batch_id=$1 AND kind='inventory' AND target_id=$2 AND fluctlight_id=$3`, batch, item.Entry.ID, item.Entry.FluctlightID, jsonBytes(after)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"batch_id": batch, "status": batchStatus, "replayed": replayed, "items": len(plan.Items)}, nil
}
func (a *App) repairDevelopingSelfTx(ctx context.Context, tx pgx.Tx, before map[string]any, status, reason, batch string) error {
	id, owner, revision := stringValue(before["id"]), stringValue(before["fluctlight_id"]), intValue(before["revision"])
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_developing_self_claims SET status=$2,revision=revision+1,updated_at=now() WHERE id=$1 AND revision=$3`, id, status, revision)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	after := cloneMap(before)
	after["status"] = status
	after["reason"] = reason
	return a.writeDevelopingSelfRevision(ctx, tx, owner, id, "forget", "history_repair", "forgotten", batch, revision+1, revision, after, jsonBytes(before), jsonBytes(after), arrayValue(before["evidence_refs"]))
}

func (a *App) RollbackHistoryPollutionRepair(ctx context.Context, owner, batch, reason string) (map[string]any, error) {
	if owner == "" || batch == "" || strings.TrimSpace(reason) == "" {
		return nil, ErrInvalidArguments
	}
	replayed := false
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ`); err != nil {
			return err
		}
		var status, recordedOwner string
		if err := tx.QueryRow(ctx, `SELECT status,owner_actor_id FROM public.history_repair_batches WHERE id=$1 FOR UPDATE`, batch).Scan(&status, &recordedOwner); err != nil {
			return err
		}
		if owner != recordedOwner {
			return ErrUnauthorized
		}
		if status == "rolled_back" {
			replayed = true
			return nil
		}
		type auditItem struct {
			Kind, ID, FluctlightID string
			Before, After          map[string]any
			Expected, Result       int
		}
		items := []auditItem{}
		rows, err := tx.Query(ctx, `SELECT kind,target_id,fluctlight_id,before_value,after_value,expected_revision,resulting_revision FROM public.history_repair_items WHERE batch_id=$1 ORDER BY kind,target_id`, batch)
		if err != nil {
			return err
		}
		for rows.Next() {
			var item auditItem
			var before, after []byte
			if err := rows.Scan(&item.Kind, &item.ID, &item.FluctlightID, &before, &after, &item.Expected, &item.Result); err != nil {
				rows.Close()
				return err
			}
			item.Before = decodeObject(before)
			item.After = decodeObject(after)
			items = append(items, item)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, item := range items {
			table, ownerColumn := "", "fluctlight_id"
			switch item.Kind {
			case "memory":
				table = "memories"
				ownerColumn = "owner_fluctlight_id"
			case "actor_fact":
				table = "actor_facts"
				ownerColumn = "owner_fluctlight_id"
			case "summary":
				table = "conversation_summaries"
				ownerColumn = "owner_fluctlight_id"
			case "developing_self":
				table = "fluctlight_developing_self_claims"
			case "inventory":
				table = "fluctlight_wardrobe_items"
			default:
				return ErrInvalidArguments
			}
			var raw []byte
			if err := tx.QueryRow(ctx, `SELECT to_jsonb(t) FROM public.`+table+` t WHERE id=$1 AND `+ownerColumn+`=$2 FOR UPDATE`, item.ID, item.FluctlightID).Scan(&raw); err != nil {
				return err
			}
			live := decodeObject(raw)
			if item.Kind == "inventory" {
				if err := addRepairInventoryContext(ctx, tx, item.FluctlightID, item.ID, live); err != nil {
					return err
				}
			}
			if intValue(live["revision"]) != item.Result || jsonString(live) != jsonString(item.After) {
				return errors.New("repair_rollback_state_changed")
			}
		}
		restoredWardrobes := map[string]bool{}
		for _, item := range items {
			switch item.Kind {
			case "memory":
				row, err := readMemoryAuthorityRowTx(ctx, tx, item.ID)
				if err != nil {
					return err
				}
				semantic := &MemorySemanticInput{Type: row.Type, Content: stringValue(item.Before["content"]), Confidence: numberOrZero(item.Before["confidence"]), Importance: numberOrZero(item.Before["importance"]), EmotionalSignificance: numberOrZero(item.Before["emotional_significance"])}
				var snapshotRaw []byte
				if err := tx.QueryRow(ctx, `SELECT snapshot FROM public.memory_revisions WHERE memory_id=$1 AND revision=$2`, item.ID, item.Expected).Scan(&snapshotRaw); err != nil {
					return err
				}
				command := buildOwnerMemoryCommand(row, owner, MemoryRollback, item.Result, []string{"history-repair-rollback:" + batch}, semantic, reason, &item.Expected, decodeObject(snapshotRaw))
				if _, err := a.applyMemoryCommandTx(ctx, tx, command); err != nil {
					return err
				}
			case "actor_fact":
				if source := stringValue(item.Before["source_message_id"]); source != "" {
					var live bool
					if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_messages WHERE id=$1 AND md5(text||attachment_refs::text)=$2)`, source, item.Before["source_fingerprint"]).Scan(&live); err != nil {
						return err
					}
					if !live {
						return errors.New("repair_actor_source_changed")
					}
				}
				if _, err := tx.Exec(ctx, `UPDATE public.actor_facts SET status=$2,revision=revision+1 WHERE id=$1`, item.ID, item.Before["status"]); err != nil {
					return err
				}
			case "summary":
				if err := restoreSummaryIDTx(ctx, tx, item.FluctlightID, item.ID, item.Before); err != nil {
					return err
				}
			case "developing_self":
				if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_developing_self_claims SET status=$2,revision=revision+1,updated_at=now() WHERE id=$1`, item.ID, item.Before["status"]); err != nil {
					return err
				}
				if err := a.writeDevelopingSelfRevision(ctx, tx, item.FluctlightID, item.ID, "rollback", "history_repair_rollback", "accepted", batch, item.Result+1, item.Result, item.Before, jsonBytes(item.After), jsonBytes(item.Before), arrayValue(item.Before["evidence_refs"])); err != nil {
					return err
				}
			case "inventory":
				// Aggregate revision and exact target links were verified above.
				// Restore the recorded wear/use only when no later change exists.
				if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_items SET availability=$2,revision=revision+1 WHERE id=$1`, item.ID, item.Before["availability"]); err != nil {
					return err
				}
				for _, raw := range arrayValue(item.Before["repair_worn_refs"]) {
					ref := mapValue(raw)
					if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id,changed_at) VALUES($1,$2,$3,$4::timestamptz)`, item.FluctlightID, ref["slot"], item.ID, ref["changed_at"]); err != nil {
						return err
					}
				}
				for _, raw := range arrayValue(item.Before["repair_used_refs"]) {
					ref := mapValue(raw)
					if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_item_uses(fluctlight_id,item_id,revision,activity,started_at,source_event_id) VALUES($1,$2,$3,$4,$5::timestamptz,$6)`, item.FluctlightID, item.ID, intValue(ref["revision"])+1, ref["activity"], ref["started_at"], ref["source_event_id"]); err != nil {
						return err
					}
				}
				if !restoredWardrobes[item.FluctlightID] {
					if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET wearing_state=$2,revision=revision+1 WHERE fluctlight_id=$1`, item.FluctlightID, mapValue(item.Before["repair_wardrobe_state"])["wearing_state"]); err != nil {
						return err
					}
					restoredWardrobes[item.FluctlightID] = true
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE public.history_repair_items SET rollback_revision=$4 WHERE batch_id=$1 AND kind=$2 AND target_id=$3`, batch, item.Kind, item.ID, item.Result+1); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE public.history_repair_batches SET status='rolled_back',rolled_back_at=now() WHERE id=$1`, batch)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"batch_id": batch, "status": "rolled_back", "replayed": replayed}, nil
}

func addRepairInventoryContext(ctx context.Context, q lifeContextQuerier, owner, id string, target map[string]any) error {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT jsonb_build_object('state',(SELECT to_jsonb(s) FROM public.fluctlight_wardrobe_states s WHERE fluctlight_id=$1),'worn',COALESCE((SELECT jsonb_agg(to_jsonb(w) ORDER BY slot) FROM public.fluctlight_worn_items w WHERE fluctlight_id=$1 AND item_id=$2),'[]'::jsonb),'used',COALESCE((SELECT jsonb_agg(to_jsonb(u) ORDER BY item_id) FROM public.fluctlight_item_uses u WHERE fluctlight_id=$1 AND item_id=$2),'[]'::jsonb))`, owner, id).Scan(&raw)
	if err != nil {
		return err
	}
	data := decodeObject(raw)
	target["repair_wardrobe_state"] = data["state"]
	target["repair_worn_refs"] = data["worn"]
	target["repair_used_refs"] = data["used"]
	return nil
}

func historyRepairManifestBatch(owner string, entries []HistoryRepairEntry, reason string) string {
	return "history_repair_" + stableDigest(jsonString(map[string]any{"owner": owner, "entries": entries, "reason": strings.TrimSpace(reason)}))
}

// A CLI retry of the same exact manifest returns its prior receipt before
// asking the now-mutated rows to match the old expected revisions again.
func (a *App) ReplayHistoryPollutionRepair(ctx context.Context, owner string, entries []HistoryRepairEntry, reason string) (map[string]any, error) {
	if owner == "" || strings.TrimSpace(reason) == "" || len(entries) == 0 || len(entries) > 100 {
		return nil, ErrInvalidArguments
	}
	batch := historyRepairManifestBatch(owner, entries, reason)
	var status, recordedOwner string
	err := a.DB.Pool().QueryRow(ctx, `SELECT status,owner_actor_id FROM public.history_repair_batches WHERE id=$1`, batch).Scan(&status, &recordedOwner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if recordedOwner != owner {
		return nil, ErrUnauthorized
	}
	return map[string]any{"batch_id": batch, "status": status, "replayed": true, "items": len(entries)}, nil
}
