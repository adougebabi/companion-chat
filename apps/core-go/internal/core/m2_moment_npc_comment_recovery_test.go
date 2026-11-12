package core

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// The fixtures use a disposable database and controlled Provider responses.
// The encounter, Moment publication and inbox processing use formal paths.
func TestM2MomentNPCCommentPersistsAndRespectsDisableRecovery(t *testing.T) {
	for _, scenario := range []string{"normal", "disabled_before_decision", "disabled_after_checkpoint"} {
		t.Run(scenario, func(t *testing.T) {
			f, npcID, placeID, now := seedNPCEncounterRuntime(t, false)
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
				t.Fatal(err)
			}
			seedCognitiveProviderRole(t, f.ctx, f.repository, "m2-npc-moment-"+f.suffix)
			_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, *now)
			if err != nil {
				t.Fatal(err)
			}
			encounter := map[string]any{
				"confirmed": true, "summary": "在书店认识了店主", "start_at": now.Add(-10 * time.Minute).Format(time.RFC3339Nano),
				"end_at": now.Add(-5 * time.Minute).Format(time.RFC3339Nano), "place_id": placeID,
				"evidence_refs": []any{f.conversationID}, "expected_life_context_revision": life["context_revision"],
				"idempotency_key": "m2-npc-encounter-" + scenario, "reason": "隔离测试中的已确认结识",
			}
			path := "/internal/fluctlights/" + f.fluctlightID + "/npcs/" + npcID + "/encounters"
			if _, err := f.app.DispatchWorldSurface(f.ctx, "POST", path, nil, f.ownerID, encounter); err != nil {
				t.Fatalf("formal confirmed encounter: %v", err)
			}

			writerCalls, npcCalls := 0, 0
			router := newFakeProviderRouter().on("npc_moment_response_v1", func(payload map[string]any) fakeProviderResult {
				npcCalls++
				wire := jsonString(payload["messages"])
				if !strings.Contains(wire, "今天在书店翻到一本好书。") || !strings.Contains(wire, "Runtime keeper") {
					t.Error("NPC response did not receive its own definition and the published Moment")
				}
				return fakeProviderResult{Structured: map[string]any{"decision": "comment", "text": "那本书很适合慢慢读。", "reaction": "none", "reason": "回应公开动态"}}
			}).otherwise(func(map[string]any) fakeProviderResult {
				writerCalls++
				return fakeProviderResult{Text: "今天在书店翻到一本好书。"}
			})
			f.app.Provider.HTTP = &http.Client{Transport: router}
			publication := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-npc-moment-" + scenario, AuthorizationActorID: f.ownerID, FluctlightID: f.fluctlightID, Arguments: jsonBytes(map[string]any{})}
			receipt, err := f.app.ExecuteTool(f.ctx, publication)
			if err != nil || receipt.Result.Status != "completed" || writerCalls != 1 {
				t.Fatalf("formal Moment publication: %#v writer_calls=%d err=%v", receipt.Result, writerCalls, err)
			}
			momentID := stringValue(mapValue(receipt.Result.Output)["target_ref"])
			var interactionID, inboxID string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id,source_fact_id FROM public.moment_npc_interactions WHERE moment_id=$1 AND npc_id=$2`, momentID, npcID).Scan(&interactionID, &inboxID); err != nil {
				t.Fatalf("published Moment did not enqueue the known NPC interaction: %v", err)
			}
			consumeNPCOpportunityPredecessors(t, f, inboxID)
			if scenario == "disabled_after_checkpoint" {
				if _, err := f.repository.Pool().Exec(f.ctx, `CREATE FUNCTION public.m2_fail_npc_comment() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'm2_npc_comment_injected_failure'; END $$; CREATE TRIGGER m2_fail_npc_comment BEFORE INSERT ON public.moment_comments FOR EACH ROW EXECUTE FUNCTION public.m2_fail_npc_comment()`); err != nil {
					t.Fatal(err)
				}
				if result, err := f.app.ProcessCognitionInbox(f.ctx, inboxID); err == nil || !strings.Contains(err.Error(), "m2_npc_comment_injected_failure") || npcCalls != 1 {
					t.Fatalf("checkpoint interruption not exercised: result=%#v calls=%d err=%v", result, npcCalls, err)
				}
				var status string
				var response []byte
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,response FROM public.moment_npc_interactions WHERE id=$1`, interactionID).Scan(&status, &response); err != nil || status != "pending" || !validNPCMomentResponse(decodeObject(response)) {
					t.Fatalf("valid decision was not checkpointed before interruption: status=%s err=%v", status, err)
				}
				if _, err := f.repository.Pool().Exec(f.ctx, `DROP TRIGGER m2_fail_npc_comment ON public.moment_comments; DROP FUNCTION public.m2_fail_npc_comment()`); err != nil {
					t.Fatal(err)
				}
			}
			if scenario != "normal" {
				if _, err := f.app.UpdateSettings(f.ctx, f.ownerID, map[string]any{"values": map[string]any{domainPluginsSettingKey: map[string]any{"image": true, "moment": false}}, "expected_versions": map[string]any{}, "idempotency_key": "m2-disable-moment-" + scenario}); err != nil {
					t.Fatalf("disable Moment via formal settings: %v", err)
				}
			}
			result, err := f.app.ProcessCognitionInbox(f.ctx, inboxID)
			if err != nil {
				t.Fatalf("NPC Moment processing: result=%#v err=%v", result, err)
			}
			wantStatus, wantEffects, wantCalls := "completed", 1, 1
			if scenario == "disabled_before_decision" {
				wantStatus, wantEffects, wantCalls = "suppressed", 0, 0
			}
			check := func() {
				t.Helper()
				var status string
				var comments, events int
				err := f.repository.Pool().QueryRow(f.ctx, `SELECT i.status,
					(SELECT count(*) FROM public.moment_comments c WHERE c.source_interaction_id=i.id AND c.moment_id=i.moment_id),
					(SELECT count(*) FROM public.platform_outbox_events o WHERE o.kind='moment.comment.created' AND o.payload->>'source_interaction_id'=i.id)
					FROM public.moment_npc_interactions i WHERE i.id=$1`, interactionID).Scan(&status, &comments, &events)
				if err != nil || status != wantStatus || comments != wantEffects || events != wantEffects || npcCalls != wantCalls {
					t.Fatalf("%s: status=%s comments=%d events=%d npc_calls=%d err=%v", scenario, status, comments, events, npcCalls, err)
				}
			}
			check()
			if replay, err := f.app.ProcessCognitionInbox(f.ctx, inboxID); err != nil || npcCalls != wantCalls {
				t.Fatalf("terminal replay repeated NPC decision: replay=%#v calls=%d err=%v", replay, npcCalls, err)
			}
			check()
			if writerCalls != 1 {
				t.Fatalf("Moment Writer unexpectedly ran %d times", writerCalls)
			}
		})
	}
}
