package core

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func wardrobeSelectionTestItems() []map[string]any {
	return []map[string]any{
		{"id": "existing-top", "description": "简约白衬衫", "clothing_type": "upper_body", "slot": "upper_body", "source_verified": true, "availability": "available", "ownership": "owned"},
		{"id": "existing-bottom", "description": "深蓝长裤", "clothing_type": "lower_body", "slot": "lower_body", "source_verified": true, "availability": "available", "ownership": "borrowed"},
	}
}

// The fake transport repeats the same inventory response on every retry, as
// the reported model did. No network or database is used.
func TestWardrobeSelectionRepeatedInventoryControlledProvider(t *testing.T) {
	for _, byName := range []bool{false, true} {
		name := "real_ids"
		if byName {
			name = "exact_descriptions"
		}
		t.Run(name, func(t *testing.T) {
			items := wardrobeSelectionTestItems()
			echoes := []any{}
			refs := []any{}
			for i, item := range items {
				key := stringValue(item["id"])
				if byName {
					key = []string{"top", "bottom"}[i]
				}
				echoes = append(echoes, map[string]any{"key": key, "description": item["description"], "clothing_type": item["clothing_type"], "seasons": []any{}})
				refs = append(refs, key)
			}
			calls := 0
			router := newFakeProviderRouter().on("wardrobe_preparation_v1", func(_ map[string]any) fakeProviderResult {
				calls++
				return fakeProviderResult{Structured: map[string]any{"items": echoes, "plans": []any{}, "wear_item_refs": refs, "reason": "选择已登记衣物"}}
			})
			provider := &ProviderClient{HTTP: &http.Client{Transport: router}}
			var selected []string
			var selectionErr error
			for attempt := 0; attempt < 3; attempt++ {
				ctx := WithADKCapabilityInvoker(context.Background(), &adkFakeInvoker{}, &ADKCapabilityTrace{})
				completion, err := provider.generateWithEino(ctx, EinoModelCall{
					Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: "http://wardrobe.invalid", ModelID: "fake", Timeout: 10 * time.Second},
					Role:       "cognitive_assessment", Scenario: "wardrobe_preparation", Messages: formalPromptMessages(FormalAgentWardrobePreparation, wardrobePreparationMessages(nil, nil, nil, items, nil, true)),
					JSONMode: true, SchemaName: "wardrobe_preparation_v1", ResponseSchema: wardrobePreparationSchema(),
				})
				if err != nil {
					t.Fatal(err)
				}
				selected, selectionErr = wardrobeSelectionItemIDs(decodeObject([]byte(completion.Message.Content)), items)
				if selectionErr == nil {
					break
				}
			}
			if selectionErr != nil {
				t.Fatalf("selection exhausted %d attempts: %v", calls, selectionErr)
			}
			if calls != 1 || !reflect.DeepEqual(selected, []string{"existing-bottom", "existing-top"}) {
				t.Fatalf("wrong selection/retry count: %v calls=%d", selected, calls)
			}
		})
	}
}

func TestWardrobeSelectionKeepsValidationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any, []map[string]any)
		want   string
	}{
		{"new_item", func(p map[string]any, _ []map[string]any) {
			p["items"] = []any{map[string]any{"key": "new", "description": "红裙子", "clothing_type": "lower_body"}}
		}, "wardrobe_select_only_created_inventory"},
		{"ambiguous_description", func(p map[string]any, items []map[string]any) {
			items[1]["description"] = items[0]["description"]
			items[1]["clothing_type"] = items[0]["clothing_type"]
			p["items"] = []any{map[string]any{"key": "top", "description": items[0]["description"], "clothing_type": "upper_body"}}
		}, "wardrobe_select_only_created_inventory"},
		{"foreign_id", func(p map[string]any, _ []map[string]any) {
			p["wear_item_refs"] = []any{"foreign-top", "existing-bottom"}
		}, "wardrobe_wear_selection_unavailable"},
		{"unverified", func(_ map[string]any, items []map[string]any) { items[0]["source_verified"] = false }, "wardrobe_wear_selection_unavailable"},
		{"unavailable", func(_ map[string]any, items []map[string]any) { items[0]["availability"] = "unavailable" }, "wardrobe_wear_selection_unavailable"},
		{"unknown_ownership", func(_ map[string]any, items []map[string]any) { items[0]["ownership"] = "unknown" }, "wardrobe_wear_selection_unavailable"},
		{"incomplete_body", func(p map[string]any, _ []map[string]any) { p["wear_item_refs"] = []any{"existing-bottom"} }, "wardrobe_initial_wear_incomplete"},
		{"new_plan", func(p map[string]any, _ []map[string]any) { p["plans"] = []any{map[string]any{"name": "新方案"}} }, "wardrobe_select_only_created_inventory"},
		{"missing_selection", func(p map[string]any, _ []map[string]any) { p["wear_item_refs"] = []any{} }, "wardrobe_wear_selection_missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := wardrobeSelectionTestItems()
			p := map[string]any{"items": []any{}, "plans": []any{}, "wear_item_refs": []any{"existing-top", "existing-bottom"}}
			tc.change(p, items)
			before := jsonString([]any{p, items})
			if _, err := wardrobeSelectionItemIDs(p, items); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %s, got %v", tc.want, err)
			}
			if before != jsonString([]any{p, items}) {
				t.Fatal("selection mutated model output or inventory")
			}
		})
	}
}

func TestWardrobeSelectionEchoWithoutWearRefs(t *testing.T) {
	items := wardrobeSelectionTestItems()
	p := map[string]any{"items": []any{map[string]any{"key": "existing-top"}, map[string]any{"key": "existing-bottom"}}, "plans": []any{}, "wear_item_refs": []any{}}
	if ids, err := wardrobeSelectionItemIDs(p, items); err != nil || !reflect.DeepEqual(ids, []string{"existing-bottom", "existing-top"}) {
		t.Fatal(ids, err)
	}
	// A declared selection must not silently pick every echoed catalogue entry.
	p["wear_item_refs"] = []any{"existing-bottom"}
	if _, err := wardrobeSelectionItemIDs(p, items); err == nil || err.Error() != "wardrobe_initial_wear_incomplete" {
		t.Fatal(err)
	}
}

func TestWardrobeSelectionFeedbackControlledProvider(t *testing.T) {
	items := wardrobeSelectionTestItems()
	previous := map[string]any{"items": []any{map[string]any{"key": "new-dress", "description": "不存在的裙子", "clothing_type": "lower_body"}}, "plans": []any{}, "wear_item_refs": []any{}}
	captured := make(chan map[string]any, 1)
	router := newFakeProviderRouter().on("wardrobe_preparation_v1", func(payload map[string]any) fakeProviderResult {
		captured <- payload
		return fakeProviderResult{Structured: map[string]any{"items": []any{}, "plans": []any{}, "wear_item_refs": []any{"existing-top", "existing-bottom"}, "reason": "修正为已有库存引用"}}
	})
	ctx := WithADKCapabilityInvoker(context.Background(), &adkFakeInvoker{}, &ADKCapabilityTrace{})
	response, err := (&ProviderClient{HTTP: &http.Client{Transport: router}}).generateWithEino(ctx, EinoModelCall{
		Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: "http://wardrobe.invalid", ModelID: "fake", Timeout: 10 * time.Second},
		Role:       "cognitive_assessment", Scenario: "wardrobe_preparation", Messages: formalPromptMessages(FormalAgentWardrobePreparation, wardrobePreparationMessages(nil, nil, nil, items, nil, true, previous)),
		JSONMode: true, SchemaName: "wardrobe_preparation_v1", ResponseSchema: wardrobePreparationSchema(),
	})
	if err != nil {
		t.Fatal(err)
	}
	wire := <-captured
	messages := arrayValue(wire["messages"])
	input := decodeObject([]byte(stringValue(mapValue(messages[1])["content"])))
	feedback := mapValue(input["selection_feedback"])
	if !strings.Contains(stringValue(feedback["error"]), "new-dress") || !reflect.DeepEqual(decisionServiceRefValues(feedback["selectable_item_refs"]), []string{"existing-bottom", "existing-top"}) {
		t.Fatal("missing actionable feedback", feedback)
	}
	if _, err := wardrobeSelectionItemIDs(decodeObject([]byte(response.Message.Content)), items); err != nil {
		t.Fatal(err)
	}
	generate := wardrobePreparationMessages(nil, nil, nil, items, nil, false, previous)
	if strings.Contains(jsonString(generate), "selection_feedback") {
		t.Fatal("selection feedback leaked into generation")
	}
}
