package core

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func r6PayloadHasTool(payload map[string]any, name string) bool {
	for _, raw := range arrayValue(payload["tools"]) {
		if capabilitycontract.CapabilityNamesMatch(stringValue(mapValue(mapValue(raw)["function"])["name"]), name) {
			return true
		}
	}
	return false
}
func r6WornIDs(t *testing.T, f independentToolE2EFixture) []string {
	t.Helper()
	rows, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, row := range rows {
		ids = append(ids, stringValue(mapValue(row)["id"]))
	}
	slices.Sort(ids)
	return ids
}
func r6Outfit(t *testing.T, f independentToolE2EFixture) (string, []string, int) {
	t.Helper()
	added, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/inventory/items", nil, f.ownerID, map[string]any{"expected_revision": wardrobeRevision(t, f), "reason": "受控登记长裤", "idempotency_key": "r6-pants", "item": map[string]any{"item_kind": "wearable", "category": "clothing", "slot": "lower_body", "clothing_type": "lower_body", "seasons": []any{}, "description": "R6 受控长裤", "ownership": "owned", "availability": "available"}})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{wardrobeItemID(t, f, "白衬衫"), wardrobeItemID(t, f, "黑色短靴"), stringValue(mapValue(added["item"])["id"])}
	saved, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/inventory/outfits", nil, f.ownerID, map[string]any{"name": "R6 真实套装", "tags": []any{"日常"}, "profile_id": "default", "item_ids": ids, "expected_revision": wardrobeRevision(t, f), "expected_outfit_revision": 0, "reason": "受控保存套装，不代表穿上", "idempotency_key": "r6-outfit"})
	if err != nil {
		t.Fatal(err)
	}
	outfit := mapValue(saved["outfit"])
	return stringValue(outfit["id"]), ids, intValue(outfit["revision"])
}

func TestR6OwnerOutfitNativeDiscoveryActuallyWearsAndReplays(t *testing.T) {
	for _, scenario := range []string{"success", "rejected", "deferred", "pretend_success", "wrong_target", "stale", "no_change", "final_failure"} {
		t.Run(scenario, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			outfit, ids, outfitRevision := r6Outfit(t, f)
			if scenario == "no_change" {
				receipt, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "r6-before", map[string]any{"mode": "full", "item_ids": ids}))
				if err != nil || receipt.Result.Status != "completed" {
					t.Fatal(receipt, err)
				}
			}
			before, revision := r6WornIDs(t, f), wardrobeRevision(t, f)
			var historyBefore int
			_ = f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wearing_history WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&historyBefore)
			calls, phase := 0, 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				for _, name := range []string{conversationReplyCapabilityName, "moment.publish", "persona.switch", "media.image.generate"} {
					if r6PayloadHasTool(payload, name) {
						t.Errorf("Owner request gained unrelated tool %s", name)
					}
				}
				if phase == 0 {
					if !r6PayloadHasTool(payload, capabilityCatalogName) || len(arrayValue(payload["tools"])) != 1 {
						t.Error("first native request must expose its scoped discovery tool")
					}
					if scenario == "rejected" || scenario == "deferred" {
						return fakeProviderResult{Structured: map[string]any{"decision": scenario, "reason": "今天保持原来的搭配"}}
					}
					if scenario == "pretend_success" {
						return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "模型声称已换好，但没有真实工具"}}
					}
					phase = 1
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("r6-catalog", capabilitycontract.CapabilityWireName(capabilityCatalogName), map[string]any{"names": []any{wardrobeWearCapabilityName}})}}
				}
				if phase == 1 {
					if !r6PayloadHasTool(payload, wardrobeWearCapabilityName) {
						t.Error("discovery did not install actual native wear schema")
						return fakeProviderResult{Status: 400}
					}
					phase = 2
					args := map[string]any{"mode": "full", "item_ids": ids}
					if scenario == "wrong_target" {
						args["item_ids"] = append(append([]string{}, ids...), wardrobeItemID(t, f, "银项链"))
					}
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("r6-wear", capabilitycontract.CapabilityWireName(wardrobeWearCapabilityName), args)}}
				}
				if scenario == "final_failure" {
					return fakeProviderResult{Text: "{broken-final"}
				}
				return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "已消费实际穿着工具的回执"}}
			})
			f.app.Provider.HTTP = &http.Client{Transport: router}
			seedCognitiveProviderRole(t, f.ctx, f.repository, "r6-outfit-role")
			admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/inventory/requests", nil, f.ownerID, map[string]any{"kind": "outfit", "outfit_id": outfit, "expected_outfit_revision": outfitRevision, "profile_id": "default", "expected_revision": revision, "reason": "请试穿这套已登记套装", "idempotency_key": "r6-request"})
			if err != nil {
				t.Fatal(err)
			}
			request := mapValue(admitted["request"])
			if request["status"] != "pending" || calls != 0 || !slices.Equal(before, r6WornIDs(t, f)) {
				t.Fatal("202 admission pretended to wear", admitted, calls)
			}
			if scenario == "stale" {
				if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1 WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
					t.Fatal(err)
				}
			}
			inbox, id := stringValue(request["source_fact_id"]), stringValue(request["id"])
			if _, err := f.app.ProcessCognitionInbox(f.ctx, inbox); err != nil {
				t.Fatal(err)
			}
			read, err := f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/owner-requests/"+id, nil, f.ownerID, nil)
			if err != nil {
				t.Fatal(err)
			}
			result := mapValue(read["result"])
			slices.Sort(ids)
			success := scenario == "success" || scenario == "final_failure" || scenario == "no_change"
			if success {
				if read["status"] != "completed" || !slices.Equal(ids, r6WornIDs(t, f)) {
					t.Fatal("real outfit never reached persisted wearing", read, r6WornIDs(t, f))
				}
				if (result["mutation_committed"] == true) != (scenario != "no_change") {
					t.Fatal("receipt mutation truth wrong", result)
				}
			} else if read["status"] == "completed" || !slices.Equal(before, r6WornIDs(t, f)) {
				t.Fatal("rejected/invalid request changed clothing", read)
			}
			var historyAfter, wearReceipts int
			_ = f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wearing_history WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&historyAfter)
			_ = f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='wardrobe.wear' AND invocation->>'source_fact_id'=$2 AND result->>'status'='completed'`, f.fluctlightID, inbox).Scan(&wearReceipts)
			wantDelta := 0
			if scenario == "success" || scenario == "final_failure" {
				wantDelta = 1
			}
			if historyAfter-historyBefore != wantDelta || wearReceipts != wantDelta {
				t.Fatal("missing/duplicate actual wear authority", historyBefore, historyAfter, wearReceipts)
			}
			count := calls
			if _, err := f.app.ProcessCognitionInbox(f.ctx, inbox); err != nil || calls != count {
				t.Fatal("terminal replay reran model", calls, count, err)
			}
			page, err := f.app.WardrobeItems(f.ctx, f.ownerID, f.fluctlightID, "")
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := page["worn_items"].([]any); !ok {
				t.Fatal("authoritative read omitted wearing")
			}
			if success && scenario != "no_change" {
				zone, _ := time.LoadLocation("Asia/Shanghai")
				now := f.app.now().In(zone)
				midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, zone)
				for _, at := range []time.Time{midnight.Add(-time.Minute), midnight.Add(time.Minute)} {
					appearance, _, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, at)
					if err != nil {
						t.Fatal(err)
					}
					worn := arrayValue(appearance["worn_items"])
					if len(worn) != len(ids) {
						t.Fatal("wearing disappeared across midnight", at, len(worn), len(ids))
					}
				}
				if !slices.Equal(ids, r6WornIDs(t, f)) {
					t.Fatal("midnight read changed wearing")
				}
			}
		})
	}
}

func TestR6SameBatchWearRequiresReceiptBeforeReply(t *testing.T) {
	for _, order := range []bool{false, true} {
		refresh := &runtimeContextRefresh{}
		calls := []schema.ToolCall{{ID: "wear", Function: schema.FunctionCall{Name: capabilitycontract.CapabilityWireName(wardrobeWearCapabilityName)}}, {ID: "reply", Function: schema.FunctionCall{Name: capabilitycontract.CapabilityWireName(conversationReplyCapabilityName)}}}
		if order {
			slices.Reverse(calls)
		}
		refresh.noteModelToolCalls(1, calls)
		code, _, blocked := replyBlockedBySameBatchSideEffect(refresh, 1, "reply", nil)
		if !blocked || code != "wardrobe_context_refresh_required" {
			t.Fatal("reply could race real wear", code, blocked)
		}
		if _, _, blocked = replyBlockedBySameBatchSideEffect(refresh, 2, "next-reply", nil); blocked {
			t.Fatal("later receipt-consuming reply stayed blocked")
		}
	}
}

func TestR6WriterRetainsCurrentWearingWithoutUnrelatedAppearance(t *testing.T) {
	view := factOnlyContextView{Facts: map[string]any{"current_state": map[string]any{"data": map[string]any{"appearance": map[string]any{"wearing_state": "known", "worn_items": []any{map[string]any{"description": "黑色短靴", "slot": "shoes"}}, "hair_color": "unrelated-hair"}}}}, Selection: map[string]any{}}
	before := jsonString(view.Facts)
	applyPrivateReplyView(&view, ContextProjection{}, map[string]any{"persona_detail_registered": true})
	wire := jsonString(view.Facts)
	if !strings.Contains(wire, "黑色短靴") || !strings.Contains(wire, "known") || strings.Contains(wire, "unrelated-hair") || before == wire {
		t.Fatal("current wearing lost or unrelated appearance retained")
	}
}

func TestR6ProvidedOutfitDiagnosticsExplainZeroNativeTools(t *testing.T) {
	root := os.Getenv("GO_CORE_R6_DIAGNOSTICS_DIR")
	if root == "" {
		t.Skip("local supplied diagnostic replay not requested")
	}
	raw, err := os.ReadFile(filepath.Join(root, "outfit-request-native-cognition.json"))
	if err != nil {
		t.Fatal(err)
	}
	rows := decodeArray(raw)
	if len(rows) != 3 {
		t.Fatal("unexpected record count")
	}
	for index, raw := range rows {
		row := mapValue(raw)
		count := mapValue(mapValue(mapValue(row["metrics"])["prompt_budget"])["section_counts"])["tools"]
		if count == nil || intValue(count) != 0 || len(arrayValue(mapValue(row["response"])["tool_calls"])) != 0 {
			t.Fatal("sample no longer matches unavailable-tool evidence", index)
		}
	}
}
