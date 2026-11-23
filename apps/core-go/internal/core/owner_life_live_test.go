package core

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestLiveOwnerLifeRequestExecution(t *testing.T) {
	runLiveOwnerLifeRequestExecution(t, false)
}

// Fix the first response to the reported failure so the local model must
// actually exercise the required native action, even if it normally would
// have called that action itself on the first turn.
func TestLiveOwnerLifeExecutionAfterTextualAcceptance(t *testing.T) {
	runLiveOwnerLifeRequestExecution(t, true)
}

func runLiveOwnerLifeRequestExecution(t *testing.T, replayAcceptance bool) {
	t.Helper()
	baseURL, model := liveProviderConfig(t)
	if os.Getenv("GO_CORE_TEST_DATABASE_URL") == "" {
		t.Fatal("live owner request recovery requires disposable PostgreSQL")
	}
	for _, scenario := range []struct {
		name string
		text string
	}{
		{"wear/boots", "请穿上这双黑色短靴，其他衣服保持原样。"},
		{"wear/shirt", "请穿上这件白衬衫。"},
		{"wear/necklace", "请戴上这条银项链。"},
		{"moment/rain", "如果愿意，发一条关于雨后清晨的动态。"},
		{"moment/reading", "请发条动态分享你喜欢的阅读时光。"},
		{"moment/quiet", "想看你发一条关于如何度过安静午后的动态。"},
	} {
		if replayAcceptance && scenario.name != "wear/boots" && scenario.name != "moment/rain" {
			continue
		}
		t.Run(scenario.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
			defer cancel()
			f := seedWardrobeToolFixture(t)
			f.ctx = ctx
			if _, err := f.repository.Pool().Exec(ctx, "INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')", f.ownerID); err != nil {
				t.Fatal(err)
			}
			endpoint := "owner-life-live"
			seedCognitiveProviderRole(t, ctx, f.repository, endpoint)
			if _, err := f.repository.Pool().Exec(ctx, "UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1", endpoint, baseURL); err != nil {
				t.Fatal(err)
			}
			if _, err := f.repository.Pool().Exec(ctx, "UPDATE public.model_roles SET model_id=$2,timeout_seconds=$3 WHERE provider_endpoint_id=$1", endpoint, model, int(liveProviderRequestTimeout().Seconds())); err != nil {
				t.Fatal(err)
			}
			wire := &r7LiveWire{}
			if dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				wire.path = filepath.Join(dir, "wire-"+stableDigest(t.Name())[:10]+".json")
			}
			var transport http.RoundTripper = wire
			if replayAcceptance {
				replay := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
					return fakeProviderResult{Text: "decision: accepted\nreason: 好的，我会按请求处理。\n\n<call:pseudo_action/>"}
				})
				first := true
				transport = projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
					if first {
						first = false
						return replay.RoundTrip(request)
					}
					return wire.RoundTrip(request)
				})
			}
			f.app = newTestApp(t, f.repository, transport)
			path := "/internal/fluctlights/" + f.fluctlightID
			body := map[string]any{"idempotency_key": "owner-life-live"}
			tool := "moment.publish"
			var target string
			var before []string
			initialRevision := 0
			switch scenario.name {
			case "wear/boots", "wear/shirt", "wear/necklace":
				path += "/wardrobe/wear"
				tool = wardrobeWearCapabilityName
				item, slot := "黑色短靴", "shoes"
				if scenario.name == "wear/shirt" || scenario.name == "wear/necklace" {
					boots := wardrobeItemID(t, f, "黑色短靴")
					result, err := f.app.ExecuteTool(ctx, f.request(wardrobeWearCapabilityName, "initial-wear", map[string]any{"mode": "full", "item_ids": []string{boots}}))
					if err != nil || result.Result.Status != "completed" {
						t.Fatalf("test setup wear: result=%v err=%v", result.Result, err)
					}
					item, slot = "白衬衫", "top"
					if scenario.name == "wear/necklace" {
						item, slot = "银项链", "accessory"
					}
				}
				target = wardrobeItemID(t, f, item)
				if item == "白衬衫" {
					if _, err := f.repository.Pool().Exec(ctx, "UPDATE public.fluctlight_wardrobe_items SET ownership='owned' WHERE id=$1", target); err != nil {
						t.Fatal(err)
					}
				}
				before = r6WornIDs(t, f)
				initialRevision = wardrobeRevision(t, f)
				body["item_id"], body["slot"], body["expected_revision"], body["reason"] = target, slot, initialRevision, scenario.text
			default:
				path += "/moment-suggestions"
				body["text"] = scenario.text
			}
			admitted, err := f.app.DispatchOwnerSurface(ctx, "POST", path, nil, f.ownerID, body)
			if err != nil {
				t.Fatal(err)
			}
			request := mapValue(admitted["request"])
			inbox, id := stringValue(request["source_fact_id"]), stringValue(request["id"])
			if id == "" || inbox == "" {
				t.Fatalf("missing admitted request: %v", admitted)
			}
			if _, err := f.app.ProcessCognitionInbox(ctx, inbox); err != nil {
				t.Fatal(err)
			}
			read, err := f.app.DispatchOwnerSurface(ctx, "GET", "/internal/owner-requests/"+id, nil, f.ownerID, nil)
			if err != nil {
				t.Fatal(err)
			}
			receipts, err := f.app.ownerLifeCommittedResults(ctx, f.ownerID, f.fluctlightID, inbox)
			if err != nil {
				t.Fatal(err)
			}
			var commits []CapabilityResult
			for _, receipt := range receipts {
				if receipt.CapabilityName == tool && receipt.Status == "completed" {
					commits = append(commits, receipt)
				}
			}
			t.Logf("status=%v reason=%v completed_tool_receipts=%d provider_requests=%d evidence=%s", read["status"], read["decision_reason"], len(commits), len(wire.rows), wire.path)
			if read["status"] != "completed" || mapValue(read["result"])["mutation_committed"] != true || len(arrayValue(read["result_refs"])) == 0 || len(commits) != 1 {
				t.Fatalf("request did not produce one committed effect: request=%v receipts=%v", read, receipts)
			}
			if replayAcceptance {
				firstRequest := mapValue(wire.rows[0]["request"])
				if firstRequest["tool_choice"] != "required" || len(arrayValue(firstRequest["tools"])) != 1 ||
					!r6PayloadHasTool(firstRequest, tool) {
					t.Fatal("the first live request did not require the exact action")
				}
			}
			if target != "" {
				want := append(slices.Clone(before), target)
				wardrobe, err := f.app.WardrobeItems(ctx, f.ownerID, f.fluctlightID, "")
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, item := range arrayValue(wardrobe["worn_items"]) {
					got = append(got, stringValue(mapValue(item)["item_id"]))
				}
				slices.Sort(want)
				slices.Sort(got)
				if !slices.Equal(got, want) || wardrobeRevision(t, f) != initialRevision+1 {
					t.Fatalf("wardrobe result = %v, want %v", got, want)
				}
			} else {
				output := mapValue(commits[0].Output)
				feed, err := f.app.DispatchOwnerSurface(ctx, "GET", "/internal/moment-feed", url.Values{"fluctlightId": {f.fluctlightID}}, f.ownerID, nil)
				if err != nil {
					t.Fatal(err)
				}
				items := arrayValue(feed["items"])
				if len(items) != 1 || output["target_kind"] != "moment" || mapValue(items[0])["id"] != output["target_ref"] || stringValue(mapValue(items[0])["text"]) == "" {
					t.Fatalf("committed Moment not visible in feed: %v, receipt=%v", feed, output)
				}
			}
			calls := len(wire.rows)
			if _, err := f.app.ProcessCognitionInbox(ctx, inbox); err != nil || len(wire.rows) != calls {
				t.Fatalf("replayed completed request called the model again: %v", err)
			}
		})
	}
}
