package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
	"github.com/jackc/pgx/v5"
)

func TestR5WardrobeRepairUsesSchemaAndPreservesEmptyRefsThroughRealWear(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "r5-wardrobe-provider")
	var preparation string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		preparation, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "r5-source")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	proposal := map[string]any{"items": []any{map[string]any{"key": "top", "description": "受控测试上衣", "clothing_type": "upper_body", "seasons": []any{}}, map[string]any{"key": "bottom", "description": "受控测试长裤", "clothing_type": "lower_body", "seasons": []any{}}}, "plans": []any{map[string]any{"name": "受控测试搭配", "tags": []any{"在家"}, "item_refs": []any{"top", "bottom"}}}, "reason": "初始方案尚未穿着"}
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("wardrobe_preparation_v1", func(payload map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			prompt := jsonString(payload["messages"])
			for _, key := range []string{"wear_item_refs", "clothing_type", "item_refs"} {
				if !strings.Contains(prompt, key) {
					t.Errorf("task text omits required schema key %s", key)
				}
			}
			return fakeProviderResult{Structured: proposal}
		}
		if calls == 2 {
			prompt := jsonString(payload["messages"])
			if strings.Contains(prompt, "use an empty evidence_refs or influences array") {
				t.Error("schema-incompatible repair instruction retained")
			}
			fixed := cloneMap(proposal)
			fixed["wear_item_refs"] = []any{}
			return fakeProviderResult{Structured: fixed}
		}
		if calls > 3 {
			t.Error("wardrobe regeneration repeated")
			return fakeProviderResult{Status: 400}
		}
		rows, err := f.repository.Pool().Query(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_ref=$2 ORDER BY id`, f.fluctlightID, "wardrobe-preparation:"+preparation)
		if err != nil {
			t.Fatal(err)
		}
		ids := []any{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		return fakeProviderResult{Structured: map[string]any{"items": []any{}, "plans": []any{}, "wear_item_refs": ids, "reason": "选择真实已登记衣物"}}
	})}
	first, err := f.app.ProcessWardrobePreparation(f.ctx, preparation)
	if err != nil || first["inventory_committed"] != true || first["status"] != "awaiting_wear" || calls != 2 {
		t.Fatal(first, calls, err)
	}
	var saved []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT proposal FROM public.fluctlight_wardrobe_preparations WHERE id=$1`, preparation).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	refs, exists := decodeObject(saved)["wear_item_refs"]
	if !exists || refs == nil || len(arrayValue(refs)) != 0 {
		t.Fatal("valid empty array lost after repair")
	}
	second, err := f.app.ProcessWardrobePreparation(f.ctx, preparation)
	if err != nil || second["status"] != "completed" || calls != 3 {
		t.Fatal(second, calls, err)
	}
	worn, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(worn) != 2 {
		t.Fatal("real wear missing", worn, err)
	}
	state, err := f.app.OwnerWardrobePreparation(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || mapValue(state["result"])["wear_receipt"] == nil {
		t.Fatal("no actual wear receipt", state, err)
	}
}

func TestR5WardrobeFailureReasonsDistinguishActualCauses(t *testing.T) {
	for _, item := range []struct {
		err  error
		code string
	}{{context.DeadlineExceeded, "wardrobe_provider_timeout"}, {&structuredResponseTruncatedError{role: "cognitive_assessment"}, "wardrobe_response_truncated"}, {errors.New("wardrobe_initial_plans_missing"), "wardrobe_plan_unavailable"}, {errors.New("adk_final_output_invalid: required field"), "wardrobe_response_schema_invalid"}, {errors.New("structured_response_invalid_json"), "wardrobe_response_invalid_json"}} {
		code, reason := wardrobePreparationFailure(item.err)
		if code != item.code || !strings.Contains(reason, "模型") {
			t.Fatal(code, reason)
		}
	}
	compound := fmt.Errorf("%w: %w", errors.New("adk_final_output_invalid: missing wear_item_refs"), fmt.Errorf("adk_final_repair_invalid: %w", errors.New("adk_structured_response_invalid: structured_response_invalid_json")))
	if code, _ := wardrobePreparationFailure(compound); code != "wardrobe_response_invalid_json" {
		t.Fatal("initial error hid terminal repair failure", code)
	}
	compound = fmt.Errorf("%w: %w", errors.New("structured_response_invalid_json"), fmt.Errorf("adk_final_repair_invalid: %w", errors.New("adk_final_output_invalid: unexpected property")))
	if code, _ := wardrobePreparationFailure(compound); code != "wardrobe_response_schema_invalid" {
		t.Fatal("terminal schema error lost", code)
	}
}

func TestR5WardrobeEmptyPlanIsBusinessFailureWithoutFabricatedInventory(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "r5-empty-wardrobe")
	var preparation string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		preparation, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "r5-empty")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("wardrobe_preparation_v1", func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"items": []any{}, "plans": []any{}, "wear_item_refs": []any{}, "reason": "资料不足，未提出方案"}}
	})}
	_, err := f.app.ProcessWardrobePreparation(f.ctx, preparation)
	if err == nil || !strings.Contains(err.Error(), "wardrobe_initial_plans_missing") {
		t.Fatal(err)
	}
	state, err := f.app.OwnerWardrobePreparation(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || state["error_code"] != "wardrobe_plan_unavailable" || state["status"] != "failed" {
		t.Fatal(state, err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_ref=$2`, f.fluctlightID, "wardrobe-preparation:"+preparation).Scan(&count); err != nil || count != 0 {
		t.Fatal("fallback inventory invented", count, err)
	}
}

func TestR5ProvidedWardrobeDiagnosticsPreserveEmptyArrays(t *testing.T) {
	root := os.Getenv("GO_CORE_R5_DIAGNOSTICS_DIR")
	if root == "" {
		t.Skip("local user diagnostic replay not requested")
	}
	raw, err := os.ReadFile(filepath.Join(root, "failed-runs.json"))
	if err != nil {
		t.Fatal(err)
	}
	rows := decodeArray(raw)
	if len(rows) != 18 {
		t.Fatal("unexpected provided sample set")
	}
	for _, index := range []int{3, 5, 8, 10, 14, 16} {
		row := mapValue(rows[index])
		output, ok := parseStructuredCandidates(providerStructuredCandidates(mapValue(row["response"])))
		if !ok {
			t.Fatal("recorded repair failed parse", index)
		}
		prepared := mapValue(prepareModelValue(output, wardrobePreparationSchema()))
		refs, found := prepared["wear_item_refs"]
		if !found || refs == nil || len(arrayValue(refs)) != 0 {
			t.Fatal("recorded empty array changed", index)
		}
		validationErr := capabilitycontract.ValidateCapabilitySchemaValue(prepared, wardrobePreparationSchema())
		if (index == 14) != (validationErr == nil) {
			t.Fatal("recorded schema result differs from diagnostic audit", index, validationErr)
		}
		if index == 14 { // This supplied repair is valid JSON/schema but contains no plan.
			if len(arrayValue(prepared["plans"])) != 0 || len(arrayValue(prepared["items"])) != 0 {
				t.Fatal("recorded empty-plan control changed")
			}
		}
	}
}
