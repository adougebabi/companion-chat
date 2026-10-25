package core

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

// Decode only this synthetic inspection receipt; the model must use the actual
// offered identifier, including its opaque native alias, not a fixture's DB ID.
func r6InspectedItem(payload map[string]any, description string) string {
	var find func(any) string
	find = func(value any) string {
		switch value := value.(type) {
		case map[string]any:
			if value["description"] == description {
				return firstString(value["id"], stringValue(value["item_id"]))
			}
			for _, key := range []string{"output", "data", "result", "items"} {
				if id := find(value[key]); id != "" {
					return id
				}
			}
		case []any:
			for _, item := range value {
				if id := find(item); id != "" {
					return id
				}
			}
		}
		return ""
	}
	for _, raw := range arrayValue(payload["messages"]) {
		message := mapValue(raw)
		if message["role"] == "tool" && message["tool_call_id"] == "r6-inspect" {
			return find(decodeObject([]byte(stringValue(message["content"]))))
		}
	}
	return ""
}

func TestR6PrivateChatDiscoveryWearReceiptAndWriter(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, scenario := range []string{"normal", "after_moment", "same_batch", "reply_first_batch", "wear_unavailable", "same_batch_unavailable", "refusal"} {
			t.Run(fmt.Sprintf("%s/stream=%t", scenario, streaming), func(t *testing.T) {
				steps := []string{"catalog", "load", "inspect", "wear", "reply"}
				if scenario == "after_moment" {
					steps = append([]string{"moment"}, steps...)
				}
				if scenario == "same_batch" || scenario == "reply_first_batch" || scenario == "same_batch_unavailable" {
					steps[3] = "batch"
				}
				if scenario == "refusal" {
					steps = []string{"reply"}
				}
				phase, writers := 0, 0
				var boots string
				var inspectOffered bool
				router := newFakeProviderRouter()
				ctx, repo, app, owner, self, conversation := r5MomentFixture(t, "r6-chat-"+scenario, router)
				var revision int
				if err := repo.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, self).Scan(&revision); err != nil {
					t.Fatal(err)
				}
				added, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/"+self+"/inventory/items", nil, owner, map[string]any{"expected_revision": revision, "reason": "受控已有短靴", "idempotency_key": "r6-chat-boots", "item": map[string]any{"item_kind": "wearable", "category": "clothing", "slot": "shoes", "clothing_type": "shoes", "description": "R6 权威短靴", "seasons": []any{}, "ownership": "owned", "availability": "available"}})
				if err != nil {
					t.Fatal(err)
				}
				boots = stringValue(mapValue(added["item"])["id"])
				if scenario == "wear_unavailable" || scenario == "same_batch_unavailable" {
					if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlight_wardrobe_items SET availability='unavailable' WHERE id=$1`, boots); err != nil {
						t.Fatal(err)
					}
				}
				router.otherwise(func(payload map[string]any) fakeProviderResult {
					if payload["model"] == "r5-writer" {
						writers++
						if strings.Contains(jsonString(payload["messages"]), "面向动态参与者") {
							return fakeProviderResult{Text: "受控测试中的一条动态。"}
						}
						current, err := readCurrentWornItems(ctx, repo.Pool(), self)
						if err != nil {
							t.Fatal(err)
						}
						wearing := false
						for _, raw := range current {
							wearing = wearing || mapValue(raw)["id"] == boots
						}
						shouldWear := scenario != "wear_unavailable" && scenario != "same_batch_unavailable" && scenario != "refusal"
						if wearing != shouldWear {
							t.Error("Writer ran before real wear outcome")
						}
						if shouldWear {
							wire := jsonString(payload["messages"])
							if !strings.Contains(wire, "worn_items") || !strings.Contains(wire, "R6 权威短靴") {
								t.Error("actual Writer request lost authoritative worn clothing")
							}
							return fakeProviderResult{Text: "现在穿着那双短靴。"}
						}
						return fakeProviderResult{Text: "这次保持原来的穿着。"}
					}
					if phase >= len(steps) {
						t.Error("unexpected additional cognition request")
						return fakeProviderResult{Status: 400}
					}
					step := steps[phase]
					phase++
					invoke := func(id, name string, args map[string]any) map[string]any {
						if !r6PayloadHasTool(payload, name) {
							t.Errorf("model attempted undisclosed tool %s at %s", name, step)
						}
						return nativePersonaToolCall(id, capabilitycontract.CapabilityWireName(name), args)
					}
					switch step {
					case "moment":
						return fakeProviderResult{ToolCalls: []map[string]any{invoke("r6-moment", "moment.publish", map[string]any{})}}
					case "catalog":
						return fakeProviderResult{ToolCalls: []map[string]any{invoke("r6-directory", capabilityCatalogName, map[string]any{})}}
					case "load":
						if scenario == "after_moment" && r6PayloadHasTool(payload, "moment.publish") {
							t.Error("completed Moment reopened")
						}
						return fakeProviderResult{ToolCalls: []map[string]any{invoke("r6-load", capabilityCatalogName, map[string]any{"names": []any{wardrobeInspectCapabilityName, wardrobeWearCapabilityName}})}}
					case "inspect":
						return fakeProviderResult{ToolCalls: []map[string]any{invoke("r6-inspect", wardrobeInspectCapabilityName, map[string]any{"operation": "list"})}}
					case "wear", "batch":
						id := r6InspectedItem(payload, "R6 权威短靴")
						if id == "" {
							t.Error("inspection did not offer target item identifier")
							return fakeProviderResult{Status: 400}
						}
						inspectOffered = true
						calls := []map[string]any{invoke("r6-chat-wear", wardrobeWearCapabilityName, map[string]any{"mode": "partial", "item_ids": []any{id}})}
						if step == "batch" {
							calls = append(calls, invoke("r6-too-early-reply", conversationReplyCapabilityName, map[string]any{}))
							if scenario == "reply_first_batch" {
								slices.Reverse(calls)
							}
						}
						return fakeProviderResult{ToolCalls: calls}
					default:
						return fakeProviderResult{ToolCalls: []map[string]any{invoke("r6-final-reply", conversationReplyCapabilityName, map[string]any{})}}
					}
				})
				result, err := app.handleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": "如果愿意，请试一下衣柜里的那双鞋。", "idempotency_key": "r6-chat", "turn_id": "r6-chat", "attachment_refs": []any{}}, turnCallbacks{}, streaming)
				if err != nil {
					t.Fatal("native chat chain failed", result, err)
				}
				var wears, moments int
				_ = repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='wardrobe.wear' AND result->>'status'='completed'`, self).Scan(&wears)
				_ = repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, self).Scan(&moments)
				wantWear, wantWriter, wantMoment := 1, 1, 0
				if scenario == "refusal" || scenario == "wear_unavailable" || scenario == "same_batch_unavailable" {
					wantWear = 0
				}
				if scenario == "after_moment" {
					wantWriter, wantMoment = 2, 1
				}
				if wears != wantWear || writers != wantWriter || moments != wantMoment || phase != len(steps) || (scenario != "refusal" && !inspectOffered) {
					t.Fatal("wrong actual effect/ordering", scenario, wears, writers, moments, phase, len(steps))
				}
			})
		}
	}
}
