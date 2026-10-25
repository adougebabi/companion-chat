package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Live tests use a disposable database and an explicitly selected Provider.
// Unlike the scripted R6 protocol tests, no tool choice or reply is supplied by
// this fixture: the production conversation loop must discover and act itself.
type r7LiveWire struct {
	mu   sync.Mutex
	rows []map[string]any
	path string
}

func (wire *r7LiveWire) RoundTrip(request *http.Request) (*http.Response, error) {
	raw, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	request.Body = io.NopCloser(bytes.NewReader(raw))
	started := time.Now()
	response, err := http.DefaultTransport.RoundTrip(request)
	row := map[string]any{"request": decodeObject(raw)}
	if err != nil {
		row["error"] = err.Error()
	} else {
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		response.Body = io.NopCloser(bytes.NewReader(body))
		row["http_status"], row["response"] = response.StatusCode, decodeObject(body)
		if !json.Valid(body) {
			row["response_body"] = string(body)
		} // Preserve native SSE without interpreting prose as tools.
		if readErr != nil {
			row["error"] = readErr.Error()
			err = readErr
		}
	}
	row["elapsed_ms"] = time.Since(started).Milliseconds()
	wire.mu.Lock()
	wire.rows = append(wire.rows, row)
	if wire.path != "" {
		if writeErr := os.WriteFile(wire.path, jsonBytes(wire.rows), 0600); writeErr != nil && err == nil {
			err = writeErr
		}
	}
	wire.mu.Unlock()
	return response, err
}

func TestR7LiveConversationWear(t *testing.T) {
	baseURL, model := liveProviderConfig(t)
	if os.Getenv("GO_CORE_TEST_DATABASE_URL") == "" {
		t.Fatal("live wear requires disposable PostgreSQL")
	}
	for _, scenario := range []string{"fresh", "after_moment", "unavailable", "discussion", "stream"} {
		t.Run(scenario, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			endpoint := "r7-live-wear"
			seedCognitiveProviderRole(t, f.ctx, f.repository, endpoint)
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, endpoint, baseURL); err != nil {
				t.Fatal(err)
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id=$2,timeout_seconds=$3 WHERE provider_endpoint_id=$1`, endpoint, model, int(liveProviderRequestTimeout().Seconds())); err != nil {
				t.Fatal(err)
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, f.ownerID, f.fluctlightID, f.conversationID); err != nil {
				t.Fatal(err)
			}
			wire := &r7LiveWire{}
			if dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				wire.path = filepath.Join(dir, "wire-"+scenario+".json")
			}
			app := newTestApp(t, f.repository, wire)
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
			defer cancel()
			ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: baseURL, ModelID: model, Timeout: liveProviderRequestTimeout()})
			f.ctx = ctx // The ordinary SQL fixture deadline is only 90s; live inference owns its longer context.
			boots := wardrobeItemID(t, f, "黑色短靴")
			if scenario == "unavailable" {
				if _, err := f.repository.Pool().Exec(ctx, `UPDATE public.fluctlight_wardrobe_items SET availability='unavailable' WHERE id=$1`, boots); err != nil {
					t.Fatal(err)
				}
			}
			before := r6WornIDs(t, f)
			publication := map[string]any{}
			defer func() {
				auditCtx, auditCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer auditCancel()
				f.ctx = auditCtx
				rows, err := f.repository.Pool().Query(auditCtx, `SELECT capability_name,result FROM public.tool_executions WHERE fluctlight_id=$1 ORDER BY committed_at`, f.fluctlightID)
				if err != nil {
					t.Error(err)
					return
				}
				var receipts []any
				for rows.Next() {
					var name string
					var result []byte
					if err := rows.Scan(&name, &result); err != nil {
						t.Error(err)
						break
					}
					receipts = append(receipts, map[string]any{"name": name, "result": decodeObject(result)})
				}
				rows.Close()
				evidence := map[string]any{"model": model, "scenario": scenario, "before": before, "after": r6WornIDs(t, f), "receipts": receipts, "wire": wire.rows, "publication": publication}
				if dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Error(err)
						return
					}
					body, err := json.MarshalIndent(evidence, "", "  ")
					if err != nil {
						t.Error(err)
						return
					}
					if err := os.WriteFile(filepath.Join(dir, "wear-"+scenario+".json"), body, 0600); err != nil {
						t.Error(err)
					}
				}
			}()
			input := "请现在穿上衣柜里那双黑色短靴，其他衣服保持原样，换好后告诉我。"
			if scenario == "after_moment" {
				input = "先发条动态分享今天想试试新搭配，然后穿上衣柜里那双黑色短靴，其他衣服保持原样，换好后告诉我。"
			}
			if scenario == "discussion" {
				input = "现在先不用换鞋。只聊聊黑色短靴配什么衣服好看？"
			}
			result, err := app.handleTurn(ctx, f.ownerID, f.conversationID, takeoverChainTurnPayload(f.fluctlightID, input, "r7-wear", "r7-wear"), turnCallbacks{}, scenario == "stream")
			t.Logf("first reply=%q err=%v", stringValue(result.Assistant["text"]), err)
			if err != nil {
				if scenario == "unavailable" {
					var rejected, published int
					if readErr := f.repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_reply_drafts WHERE fluctlight_id=$1 AND status='failed' AND last_error='reply_action_outcome_conflict'`, f.fluctlightID).Scan(&rejected); readErr != nil {
						t.Fatal(readErr)
					}
					if readErr := f.repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE author_actor_id=$1`, f.fluctlightID).Scan(&published); readErr != nil {
						t.Fatal(readErr)
					}
					if rejected == 1 && published == 0 && slices.Equal(before, r6WornIDs(t, f)) {
						publication["status"] = "blocked_inconsistent_draft"
						publication["assistant_messages"] = published
						t.Log("verified: inconsistent model draft blocked; no assistant publication or wearing change")
						return
					}
				}
				t.Fatal(err)
			}
			publication["status"], publication["text"] = "published", stringValue(result.Assistant["text"])
			var count int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, f.fluctlightID, boots).Scan(&count); err != nil {
				t.Fatal(err)
			}
			wantWear := scenario != "unavailable" && scenario != "discussion"
			if !wantWear {
				if count != 0 || !slices.Equal(before, r6WornIDs(t, f)) {
					t.Fatal("discussion/unavailable request changed actual wearing")
				}
				return
			}
			if count != 1 {
				t.Fatalf("natural-language request did not produce actual wear; reply=%q", result.Assistant["text"])
			}
			expected := append(slices.Clone(before), boots)
			slices.Sort(expected)
			if !slices.Equal(slices.Compact(expected), r6WornIDs(t, f)) {
				t.Fatal("changing shoes removed or replaced unrelated worn clothing")
			}
			var committed int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='wardrobe.wear' AND result->>'status'='completed'`, f.fluctlightID).Scan(&committed); err != nil {
				t.Fatal(err)
			}
			if committed != 1 {
				t.Fatalf("want one real wear receipt, got %d", committed)
			}
			if scenario == "after_moment" {
				var moments int
				if err := f.repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, f.fluctlightID).Scan(&moments); err != nil {
					t.Fatal(err)
				}
				if moments != 1 {
					t.Fatalf("want one published Moment, got %d", moments)
				}
			}
			reloaded := newTestApp(t, f.repository, wire)
			after, err := reloaded.HandleTurn(ctx, f.ownerID, f.conversationID, takeoverChainTurnPayload(f.fluctlightID, "你现在穿着哪双鞋？", "r7-readback", "r7-readback"))
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("after reload reply=%q", after.Assistant["text"])
			if !strings.Contains(stringValue(after.Assistant["text"]), "黑色短靴") {
				t.Fatal("reloaded reply lost authoritative wearing", fmt.Sprint(after.Assistant["text"]))
			}
		})
	}
}

func TestR7LiveOwnerOutfitAlreadyWorn(t *testing.T) {
	baseURL, model := liveProviderConfig(t)
	if os.Getenv("GO_CORE_TEST_DATABASE_URL") == "" {
		t.Fatal("live wear requires disposable PostgreSQL")
	}
	f := seedWardrobeToolFixture(t)
	outfit, ids, outfitRevision := r6Outfit(t, f)
	initial, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "r7-initial-wear", map[string]any{"mode": "full", "item_ids": ids}))
	if err != nil || initial.Result.Status != "completed" {
		t.Fatal(initial, err)
	}
	before, revision := r6WornIDs(t, f), wardrobeRevision(t, f)
	endpoint := "r7-live-owner"
	seedCognitiveProviderRole(t, f.ctx, f.repository, endpoint)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, endpoint, baseURL); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id=$2,timeout_seconds=$3 WHERE provider_endpoint_id=$1`, endpoint, model, int(liveProviderRequestTimeout().Seconds())); err != nil {
		t.Fatal(err)
	}
	wire := &r7LiveWire{}
	if dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		wire.path = filepath.Join(dir, "wire-owner-no-change.json")
	}
	f.app = newTestApp(t, f.repository, wire)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	f.ctx = ctx
	admitted, err := f.app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/inventory/requests", nil, f.ownerID, map[string]any{"kind": "outfit", "outfit_id": outfit, "expected_outfit_revision": outfitRevision, "profile_id": "default", "expected_revision": revision, "reason": "请穿上这套搭配", "idempotency_key": "r7-owner-no-change"})
	if err != nil {
		t.Fatal(err)
	}
	request := mapValue(admitted["request"])
	inbox, id := stringValue(request["source_fact_id"]), stringValue(request["id"])
	if _, err := f.app.ProcessCognitionInbox(ctx, inbox); err != nil {
		t.Fatal(err)
	}
	read, err := f.app.DispatchOwnerSurface(ctx, "GET", "/internal/owner-requests/"+id, nil, f.ownerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := mapValue(read["result"])
	t.Logf("owner outcome status=%v disposition=%v mutation=%v", read["status"], result["disposition"], result["mutation_committed"])
	if dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, "owner-no-change.json"), jsonBytes(map[string]any{"model": model, "outcome": read, "before": before, "after": r6WornIDs(t, f)}), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if read["status"] != "completed" || result["disposition"] != "no_change" || result["mutation_committed"] != false || !slices.Equal(before, r6WornIDs(t, f)) || wardrobeRevision(t, f) != revision {
		t.Fatal("matching outfit was not truthfully satisfied without mutation", read)
	}
	count := len(wire.rows)
	if _, err := f.app.ProcessCognitionInbox(ctx, inbox); err != nil || len(wire.rows) != count {
		t.Fatal("terminal request replay called model again", err)
	}
}
