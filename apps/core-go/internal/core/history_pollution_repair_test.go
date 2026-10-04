package core

import (
	"testing"
	"time"
)

func TestHistoryRepairDryRunApplyReplayAndRollbackPreserveOriginals(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	recorded, err := f.app.ExecuteTool(f.ctx, f.request("memory_event", "repair-memory-create", map[string]any{"content": "未经来源确认的持有断言", "type": "autobiographical", "confidence": 0.6, "importance": 0.8}))
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(mapValue(recorded.Result.Output)["memory_id"])
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.memories SET provenance_status='legacy_unknown' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	plan, err := f.app.PlanHistoryPollutionRepair(f.ctx, f.ownerID, []HistoryRepairEntry{{FluctlightID: f.fluctlightID, Kind: "memory", ID: id, ExpectedRevision: 0, Qualification: "unverified"}}, "隔离无法证明的历史断言")
	if err != nil {
		t.Fatal(err)
	}
	var beforeCount int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.history_repair_batches`).Scan(&beforeCount); err != nil || beforeCount != 0 {
		t.Fatal("dry-run wrote audit state", err)
	}
	applied, err := f.app.ApplyHistoryPollutionRepair(f.ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.app.ApplyHistoryPollutionRepair(f.ctx, plan)
	if err != nil || replay["replayed"] != true {
		t.Fatalf("apply replay %#v %v", replay, err)
	}
	var content, status string
	var revision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT content,status,revision FROM public.memories WHERE id=$1`, id).Scan(&content, &status, &revision); err != nil || content != "未经来源确认的持有断言" || status != "forgotten" || revision != 1 {
		t.Fatalf("cleanup erased original: %q %s %d %v", content, status, revision, err)
	}
	rollback, err := f.app.RollbackHistoryPollutionRepair(f.ctx, f.ownerID, stringValue(applied["batch_id"]), "按审计回滚")
	if err != nil || rollback["status"] != "rolled_back" {
		t.Fatalf("rollback %#v %v", rollback, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT content,status,revision FROM public.memories WHERE id=$1`, id).Scan(&content, &status, &revision); err != nil || status != "active" || revision != 2 {
		t.Fatalf("compensation did not append revision: %s %d %v", status, revision, err)
	}
	again, err := f.app.RollbackHistoryPollutionRepair(f.ctx, f.ownerID, stringValue(applied["batch_id"]), "重复回滚")
	if err != nil || again["replayed"] != true {
		t.Fatalf("rollback replay %#v %v", again, err)
	}
	var revisions int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.memory_revisions WHERE memory_id=$1`, id).Scan(&revisions); err != nil || revisions != 3 {
		t.Fatalf("missing audit revisions %d %v", revisions, err)
	}
}
func TestHistoryRepairRefusesMixedSourcesForeignOwnerAndChangedPlan(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	factID := recordActorFact(t, f, "repair-location", "assert", "location_scope", "abroad", "")
	entry := HistoryRepairEntry{FluctlightID: f.fluctlightID, Kind: "actor_fact", ID: factID, ExpectedRevision: 1, Qualification: "explicit_invalidated"}
	if _, err := f.app.PlanHistoryPollutionRepair(f.ctx, f.foreignOwnerID, []HistoryRepairEntry{entry}, "不属于这个所有者"); err == nil {
		t.Fatal("cross-owner repair permitted")
	}
	plan, err := f.app.PlanHistoryPollutionRepair(f.ctx, f.ownerID, []HistoryRepairEntry{entry}, "明确错误事实")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.actor_facts SET revision=revision+1 WHERE id=$1`, factID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ApplyHistoryPollutionRepair(f.ctx, plan); err == nil {
		t.Fatal("stale dry-run plan applied")
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.history_repair_batches`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial audit left behind %d %v", count, err)
	}
}

func TestHistoryRepairInventoryRestoresExactWearAndRefusesLaterWardrobe(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "exact-rollback", true: "newer-state"}[changed], func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			var id string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND slot='accessory'`, f.fluctlightID).Scan(&id); err != nil {
				t.Fatal(err)
			}
			// Simulate only an explicitly identified legacy pollution record.
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_items SET source_kind='accepted_event',source_ref='unknown' WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
			plan, err := f.app.PlanHistoryPollutionRepair(f.ctx, f.ownerID, []HistoryRepairEntry{{FluctlightID: f.fluctlightID, Kind: "inventory", ID: id, ExpectedRevision: 1, Qualification: "unverified"}}, "明确隔离无来源物品")
			if err != nil {
				t.Fatal(err)
			}
			if len(arrayValue(plan.Items[0].Before["repair_worn_refs"])) != 1 {
				t.Fatal("dry-run omitted exact wearing")
			}
			applied, err := f.app.ApplyHistoryPollutionRepair(f.ctx, plan)
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1 WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
					t.Fatal(err)
				}
			}
			restored, err := f.app.RollbackHistoryPollutionRepair(f.ctx, f.ownerID, stringValue(applied["batch_id"]), "按原审计恢复")
			if changed {
				if err == nil {
					t.Fatal("overwrote newer wardrobe", restored)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var count int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, f.fluctlightID, id).Scan(&count); err != nil || count != 1 {
				t.Fatalf("exact wear lost %d %v", count, err)
			}
		})
	}
}

func TestHistoryRepairPeriodicOnlyPreservesMixedAndLegitimateWakeEvidence(t *testing.T) {
	for _, scenario := range []string{"periodic", "mixed", "legitimate"} {
		t.Run(scenario, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			fact := "repair-periodic-" + f.suffix
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,1000,'internal.wake_up','{}',$1,$1,$1,now(),'processed')`, fact, f.fluctlightID); err != nil {
				t.Fatal(err)
			}
			refs := []any{"fact:" + fact}
			if scenario == "mixed" {
				human := "repair-human-" + f.suffix
				if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,1001,'conversation.turn','{}',$1,$1,$1,now(),'processed')`, human, f.fluctlightID); err != nil {
					t.Fatal(err)
				}
				refs = append(refs, "fact:"+human)
			}
			claim := "repair-periodic-self-" + f.suffix
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_developing_self_claims(id,fluctlight_id,category,claim,value,confidence,evidence_refs,status) VALUES($1,$2,'habit','夜间自然醒','{}',0.9,$3,'active')`, claim, f.fluctlightID, jsonBytes(refs)); err != nil {
				t.Fatal(err)
			}
			if scenario == "legitimate" {
				at := f.app.now()
				life := currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
				if _, err := f.app.CreateLifeEvent(f.ctx, f.ownerID, f.fluctlightID, map[string]any{"kind": "wake", "start_at": formatInstant(at), "end_at": formatInstant(at.Add(time.Hour)), "scene": "卧室", "activity": "合法起床", "evidence_refs": []any{fact}, "idempotency_key": "repair-legitimate-wake", "expected_life_context_revision": life["context_revision"]}); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := f.app.PlanHistoryPollutionRepair(f.ctx, f.ownerID, []HistoryRepairEntry{{FluctlightID: f.fluctlightID, Kind: "developing_self", ID: claim, ExpectedRevision: 1, Qualification: "periodic_only"}}, "撤销只有周期检查来源的自然醒习惯")
			if scenario == "periodic" {
				if err != nil || len(plan.Items) != 1 {
					t.Fatalf("periodic %#v %v", plan, err)
				}
			} else if err == nil {
				t.Fatalf("preserved evidence %s accepted", scenario)
			}
		})
	}
}
