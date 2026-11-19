package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type npcProcessManifest struct {
	Kind, Owner, Fluctlight, NPC, Opportunity, Interaction, Inbox, SourceEvent, Presence string
	Now                                                                                  time.Time
	Request                                                                              ToolExecutionRequest
	Cancelled                                                                            bool
}

// The parent seeds once. Both child PIDs receive the exact same manifest and DB.
func runNPCRecoveryChild(t *testing.T, repo *PostgresRepository, manifest npcProcessManifest, phase string, exit int) int {
	t.Helper()
	u, err := url.Parse(os.Getenv("GO_CORE_TEST_DATABASE_URL"))
	if err != nil || u.Hostname() != "127.0.0.1" || !strings.HasPrefix(repo.Pool().Config().ConnConfig.Database, "lac_core_") {
		t.Fatal("refusing non-isolated child database")
	}
	u.Path = "/" + repo.Pool().Config().ConnConfig.Database
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, jsonBytes(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestNPCCheckpointChild$", "-test.v")
	command.Env = append(os.Environ(), "B4NPC_PHASE="+phase, "B4NPC_DB="+u.String(), "B4NPC_MANIFEST="+path)
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	pid := command.Process.Pid
	err = command.Wait()
	actual := 0
	if err != nil {
		var status *exec.ExitError
		if !errors.As(err, &status) {
			t.Fatal(err)
		}
		actual = status.ExitCode()
	}
	safe := strings.ReplaceAll(output.String(), u.String(), "[isolated DB URL]")
	if u.User != nil {
		if password, ok := u.User.Password(); ok && password != "" {
			safe = strings.ReplaceAll(safe, password, "[disposable DB password]")
		}
	}
	t.Logf("kind=%s phase=%s pid=%d exit=%d\n%s", manifest.Kind, phase, pid, actual, safe)
	if actual != exit {
		t.Fatalf("child exit=%d wanted=%d", actual, exit)
	}
	return pid
}

func npcCheckpointHTTP(t *testing.T, decision string) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	npcCalls, writerCalls := &atomic.Int32{}, &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") || json.NewDecoder(r.Body).Decode(&payload) != nil {
			t.Error("unexpected controlled HTTP request")
			w.WriteHeader(400)
			return
		}
		content := "今天在书店翻到一本好书。"
		switch providerWireSchemaName(payload) {
		case "npc_response_v1":
			npcCalls.Add(1)
			content = jsonString(map[string]any{"decision": decision, "reason": "回应实际公开接触，不引入私人背景"})
		case "npc_moment_response_v1":
			npcCalls.Add(1)
			content = jsonString(map[string]any{"decision": "comment", "text": "那本书很适合慢慢读。", "reaction": "none", "reason": "回应公开动态"})
		default:
			writerCalls.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": content}}}})
	}))
	t.Cleanup(server.Close)
	return server, npcCalls, writerCalls
}

func bindNPCCheckpointHTTP(t *testing.T, f independentToolE2EFixture, endpoint string) {
	t.Helper()
	id := "npc-process-" + f.suffix
	seedCognitiveProviderRole(t, f.ctx, f.repository, id)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, id, endpoint); err != nil {
		t.Fatal(err)
	}
	f.app.Provider.HTTP = &http.Client{Timeout: 15 * time.Second}
}

func cancelNPCCheckpointSource(t *testing.T, f independentToolE2EFixture, event string) {
	t.Helper()
	var revision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.life_events WHERE id=$1`, event).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.CancelLifeEvent(f.ctx, f.ownerID, f.fluctlightID, event, map[string]any{"expected_event_revision": revision, "expected_life_context_revision": life["context_revision"], "idempotency_key": "cancel-process-source", "reason": "隔离恢复场景取消原始来源"}); err != nil {
		t.Fatal(err)
	}
}

func TestNPCCheckpointAcrossProcesses(t *testing.T) {
	for _, decision := range []string{"accept", "decline", "cancelled_source"} {
		t.Run("opportunity_"+decision, func(t *testing.T) {
			f, npc, _, now := seedNPCEncounterRuntime(t, true)
			server, calls, writers := npcCheckpointHTTP(t, map[bool]string{true: "accept", false: decision}[decision == "cancelled_source"])
			bindNPCCheckpointHTTP(t, f, server.URL)
			if _, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil {
				t.Fatal(err)
			}
			page, err := f.app.WorldNPCRecords(f.ctx, f.ownerID, f.fluctlightID, npc, "opportunities", url.Values{})
			if err != nil || len(arrayValue(page["items"])) != 1 {
				t.Fatal("missing opportunity", err)
			}
			p := mapValue(arrayValue(page["items"])[0])
			*now = now.Add(15 * time.Second)
			request := f.request(npcEncounterCapabilityName, "cross-process-"+decision, map[string]any{"opportunity_id": p["id"], "expected_revision": p["revision"], "decision": "contact", "reason": "实际同地接触"})
			request.EvidenceID = stringValue(p["source_fact_id"])
			m := npcProcessManifest{Kind: "opportunity", Owner: f.ownerID, Fluctlight: f.fluctlightID, NPC: npc, Opportunity: stringValue(p["id"]), Inbox: request.EvidenceID, SourceEvent: stringValue(p["source_event_id"]), Presence: stringValue(p["npc_presence_id"]), Now: *now, Request: request}
			a := runNPCRecoveryChild(t, f.repository, m, "checkpoint", 73)
			var responseBefore []byte
			var status string
			var revision, encounters, receipts int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,revision,npc_response,(SELECT count(*) FROM public.world_encounters WHERE opportunity_id=p.id),(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND operation_id=$3) FROM public.world_npc_opportunities p WHERE id=$1`, m.Opportunity, m.Fluctlight, request.OperationID).Scan(&status, &revision, &responseBefore, &encounters, &receipts); err != nil || status != "pending" || revision != intValue(p["revision"]) || encounters != 0 || receipts != 0 || calls.Load() != 1 || mapValue(decodeObject(responseBefore))["state"] != "ready" || stringValue(decodeObject(responseBefore)["request_key"]) == "" {
				t.Fatalf("A failed to stop after durable response: status=%s encounters=%d receipts=%d HTTP=%d err=%v", status, encounters, receipts, calls.Load(), err)
			}
			if decision == "cancelled_source" {
				cancelNPCCheckpointSource(t, f, m.SourceEvent)
				m.Cancelled = true
			}
			b := runNPCRecoveryChild(t, f.repository, m, "resume", 0)
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND operation_id=$2`, m.Fluctlight, m.Request.OperationID).Scan(&receipts); err != nil || receipts != 1 {
				t.Fatalf("recovered tool receipts=%d err=%v", receipts, err)
			}
			if a == b {
				t.Fatal("reused PID")
			}
			var responseAfter []byte
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT npc_response FROM public.world_npc_opportunities WHERE id=$1`, m.Opportunity).Scan(&responseAfter); err != nil || !bytes.Equal(responseBefore, responseAfter) || calls.Load() != 1 || writers.Load() != 0 {
				t.Fatalf("B regenerated or changed checkpoint: HTTP=%d writer=%d err=%v", calls.Load(), writers.Load(), err)
			}
			wantEnc, wantRel := 1, 0
			if decision == "accept" {
				wantRel = 1
			}
			if m.Cancelled {
				wantEnc = 0
			}
			var relationships, memories, events int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.world_encounters WHERE opportunity_id=$1),(SELECT count(*) FROM public.relationships WHERE owner_fluctlight_id=$2),(SELECT count(*) FROM public.memories WHERE owner_fluctlight_id=$2),(SELECT count(*) FROM public.platform_outbox_events WHERE kind='world.npc.encountered' AND payload->>'encounter_id' IN (SELECT id FROM public.world_encounters WHERE opportunity_id=$1))`, m.Opportunity, m.Fluctlight).Scan(&encounters, &relationships, &memories, &events); err != nil || encounters != wantEnc || relationships != wantRel || memories != wantEnc || events != wantEnc {
				t.Fatalf("effects encounters=%d relationships=%d memories=%d outbox=%d err=%v", encounters, relationships, memories, events, err)
			}
			if wantEnc == 1 {
				var kind string
				var refs []byte
				var leaked bool
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT source_kind,evidence_refs FROM public.world_encounters WHERE opportunity_id=$1`, m.Opportunity).Scan(&kind, &refs); err != nil || kind != "virtual_world_encounter" || !strings.Contains(string(refs), m.Presence) {
					t.Fatal("lost actual source attribution", err)
				}
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.memories WHERE owner_fluctlight_id=$1 AND (actor_refs ? $2 OR content LIKE '%PRIVATE_NPC_BIO_NOT_AUTOMATIC_KNOWLEDGE%'))`, m.Fluctlight, m.Owner).Scan(&leaked); err != nil || leaked {
					t.Fatal("invented Owner presence/private biography", err)
				}
			}
		})
	}
	for _, cancelled := range []bool{false, true} {
		name := "moment_comment"
		if cancelled {
			name = "moment_cancelled_source"
		}
		t.Run(name, func(t *testing.T) {
			f, npc, place, now := seedNPCEncounterRuntime(t, false)
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
				t.Fatal(err)
			}
			server, calls, writers := npcCheckpointHTTP(t, "accept")
			bindNPCCheckpointHTTP(t, f, server.URL)
			_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, *now)
			if err != nil {
				t.Fatal(err)
			}
			body := map[string]any{"confirmed": true, "summary": "在书店认识了店主", "start_at": now.Add(-10 * time.Minute).Format(time.RFC3339Nano), "end_at": now.Add(-5 * time.Minute).Format(time.RFC3339Nano), "place_id": place, "evidence_refs": []any{f.conversationID}, "expected_life_context_revision": life["context_revision"], "idempotency_key": "process-confirmed-encounter", "reason": "隔离已确认结识"}
			if _, err := f.app.DispatchWorldSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/npcs/"+npc+"/encounters", nil, f.ownerID, body); err != nil {
				t.Fatal(err)
			}
			publication, err := f.app.ExecuteTool(f.ctx, ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "process-moment-publish", AuthorizationActorID: f.ownerID, FluctlightID: f.fluctlightID, Arguments: jsonBytes(map[string]any{})})
			if err != nil || publication.Result.Status != "completed" {
				t.Fatal("formal Moment publish", publication, err)
			}
			moment := stringValue(mapValue(publication.Result.Output)["target_ref"])
			m := npcProcessManifest{Kind: "moment", Owner: f.ownerID, Fluctlight: f.fluctlightID, NPC: npc, Now: *now}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id,source_fact_id,source_event_id FROM public.moment_npc_interactions WHERE moment_id=$1 AND npc_id=$2`, moment, npc).Scan(&m.Interaction, &m.Inbox, &m.SourceEvent); err != nil {
				t.Fatal(err)
			}
			consumeNPCOpportunityPredecessors(t, f, m.Inbox)
			if _, err := f.repository.Pool().Exec(f.ctx, `CREATE FUNCTION public.fail_process_comment() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'npc_process_comment_injected_failure'; END $$; CREATE TRIGGER fail_process_comment BEFORE INSERT ON public.moment_comments FOR EACH ROW EXECUTE FUNCTION public.fail_process_comment()`); err != nil {
				t.Fatal(err)
			}
			socialBefore := npcRecoveredSocialSnapshot(t, f.ctx, f.repository, m)
			a := runNPCRecoveryChild(t, f.repository, m, "checkpoint", 74)
			var responseBefore []byte
			var status string
			var comments, events int
			var claimFree bool
			var leases int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,response,(SELECT count(*) FROM public.moment_comments WHERE source_interaction_id=i.id),(SELECT count(*) FROM public.platform_outbox_events WHERE kind='moment.comment.created' AND payload->>'source_interaction_id'=i.id) FROM public.moment_npc_interactions i WHERE id=$1`, m.Interaction).Scan(&status, &responseBefore, &comments, &events); err != nil || status != "pending" || !validNPCMomentResponse(decodeObject(responseBefore)) || comments != 0 || events != 0 || calls.Load() != 1 {
				t.Fatalf("Moment checkpoint status=%s comments=%d outbox=%d HTTP=%d err=%v", status, comments, events, calls.Load(), err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status='pending' AND claimed_by IS NULL AND claimed_at IS NULL FROM public.cognition_inbox WHERE id=$1`, m.Inbox).Scan(&claimFree); err != nil || !claimFree {
				t.Fatal("inbox claim not released after returned failure", err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.logical_agent_leases WHERE fluctlight_id=$1 AND expires_at>now()`, m.Fluctlight).Scan(&leases); err != nil || leases != 0 {
				t.Fatal("logical lease not released", err)
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `DROP TRIGGER fail_process_comment ON public.moment_comments; DROP FUNCTION public.fail_process_comment()`); err != nil {
				t.Fatal(err)
			}
			if cancelled {
				cancelNPCCheckpointSource(t, f, m.SourceEvent)
				m.Cancelled = true
			}
			b := runNPCRecoveryChild(t, f.repository, m, "resume", 0)
			if a == b {
				t.Fatal("reused PID")
			}
			if npcRecoveredSocialSnapshot(t, f.ctx, f.repository, m) != socialBefore {
				t.Fatal("Moment recovery changed encounters/relationships/memories")
			}
			var responseAfter []byte
			var code string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,COALESCE(error_code,''),response,(SELECT count(*) FROM public.moment_comments WHERE source_interaction_id=i.id),(SELECT count(*) FROM public.platform_outbox_events WHERE kind='moment.comment.created' AND payload->>'source_interaction_id'=i.id) FROM public.moment_npc_interactions i WHERE id=$1`, m.Interaction).Scan(&status, &code, &responseAfter, &comments, &events); err != nil {
				t.Fatal(err)
			}
			wantStatus, wantEffects := "completed", 1
			if cancelled {
				wantStatus, wantEffects = "suppressed", 0
			}
			if status != wantStatus || comments != wantEffects || events != wantEffects || !bytes.Equal(responseBefore, responseAfter) || calls.Load() != 1 || writers.Load() != 1 || cancelled && code != "npc_moment_source_changed" {
				t.Fatalf("Moment recovery status=%s code=%s comments=%d outbox=%d NPC HTTP=%d writer=%d", status, code, comments, events, calls.Load(), writers.Load())
			}
			if !cancelled {
				var correctComments, correctEvents int
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT
 (SELECT count(*) FROM public.moment_comments c JOIN public.world_npcs n ON n.id=$1 JOIN public.moment_npc_interactions i ON i.id=$2 WHERE c.id=i.comment_id AND c.source_interaction_id=i.id AND c.moment_id=$3 AND c.author_actor_id=n.actor_id AND c.text='那本书很适合慢慢读。'),
 (SELECT count(*) FROM public.platform_outbox_events o JOIN public.world_npcs n ON n.id=$1 WHERE o.kind='moment.comment.created' AND o.payload->>'source_interaction_id'=$2 AND o.payload->>'moment_id'=$3 AND o.payload->>'author_actor_id'=n.actor_id AND o.payload->>'source_event_id'=$4 AND o.payload->'copresent'='false'::jsonb)`, npc, m.Interaction, moment, m.SourceEvent).Scan(&correctComments, &correctEvents); err != nil || correctComments != 1 || correctEvents != 1 {
					t.Fatalf("NPC public identity/provenance incorrect: comments=%d outbox=%d err=%v", correctComments, correctEvents, err)
				}
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status='processed' AND claimed_by IS NULL AND claimed_at IS NULL FROM public.cognition_inbox WHERE id=$1`, m.Inbox).Scan(&claimFree); err != nil || !claimFree {
				t.Fatal("original inbox not settled", err)
			}
		})
	}
}

func TestNPCCheckpointChild(t *testing.T) {
	phase := os.Getenv("B4NPC_PHASE")
	if phase == "" {
		t.Skip("only run by isolated cross-process parent")
	}
	if phase != "checkpoint" && phase != "resume" {
		t.Fatal("invalid child phase")
	}
	u, err := url.Parse(os.Getenv("B4NPC_DB"))
	if err != nil || u.Hostname() != "127.0.0.1" || !strings.HasPrefix(strings.TrimPrefix(u.Path, "/"), "lac_core_") {
		t.Fatal("refusing non-isolated DB")
	}
	raw, err := os.ReadFile(os.Getenv("B4NPC_MANIFEST"))
	if err != nil {
		t.Fatal(err)
	}
	var m npcProcessManifest
	if json.Unmarshal(raw, &m) != nil {
		t.Fatal("invalid manifest")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	repo, err := NewPostgresRepository(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	app := newTestApp(t, repo, nil)
	app.Provider.HTTP = &http.Client{Timeout: 15 * time.Second}
	app.Clock = func() time.Time { return m.Now }
	if m.Kind == "opportunity" {
		if phase == "checkpoint" {
			c := newNPCEncounterCapability(app, m.Opportunity)
			c.service.commit = func(context.Context, pgx.Tx, npcEncounterCommit) (map[string]any, error) {
				os.Exit(73)
				return nil, errors.New("unreachable checkpoint exit")
			}
			registry, err := capabilityRegistryView(app.capabilityRegistry(), c)
			if err != nil {
				t.Fatal(err)
			}
			ctx = withCapabilityRunRegistry(ctx, registry)
			r, err := app.ExecuteTool(ctx, m.Request)
			t.Fatalf("did not reach hard-exit boundary: %#v %v", r, err)
		}
		first, err := app.ExecuteTool(ctx, m.Request)
		if m.Cancelled {
			requireHandledToolNoEffect(t, first, err, "npc_opportunity_expired")
		} else if err != nil || first.Result.Status != "completed" {
			t.Fatal(first, err)
		}
		snapshot := npcRecoveredBusinessSnapshot(t, ctx, repo, m)
		again, err := app.ExecuteTool(ctx, m.Request)
		if err != nil || !again.Replayed || again.OperationID != first.OperationID || stringValue(mapValue(first.Result.Output)["id"]) != stringValue(mapValue(again.Result.Output)["id"]) {
			t.Fatal("receipt replay changed identity", again, err)
		}
		if npcRecoveredBusinessSnapshot(t, ctx, repo, m) != snapshot {
			t.Fatal("opportunity replay changed business rows")
		}
		return
	}
	if m.Kind != "moment" {
		t.Fatal("invalid manifest kind")
	}
	_, err = app.ProcessCognitionInbox(ctx, m.Inbox)
	if phase == "checkpoint" {
		if err == nil || !strings.Contains(err.Error(), "npc_process_comment_injected_failure") {
			t.Fatal("did not hit comment fault", err)
		}
		os.Exit(74)
	}
	if err != nil {
		t.Fatal(err)
	}
	snapshot := npcRecoveredBusinessSnapshot(t, ctx, repo, m)
	if _, err := app.ProcessCognitionInbox(ctx, m.Inbox); err != nil {
		t.Fatal(err)
	}
	if npcRecoveredBusinessSnapshot(t, ctx, repo, m) != snapshot {
		t.Fatal("Moment replay changed business rows")
	}
}

func npcRecoveredBusinessSnapshot(t *testing.T, ctx context.Context, repo *PostgresRepository, m npcProcessManifest) string {
	t.Helper()
	var raw []byte
	err := repo.Pool().QueryRow(ctx, `SELECT jsonb_build_object(
 'opportunity',(SELECT to_jsonb(p) FROM public.world_npc_opportunities p WHERE id=$1),
 'interaction',(SELECT to_jsonb(i) FROM public.moment_npc_interactions i WHERE id=$2),
 'moment',(SELECT to_jsonb(m) FROM public.moments m WHERE id=(SELECT moment_id FROM public.moment_npc_interactions WHERE id=$2)),
 'encounters',(SELECT jsonb_agg(to_jsonb(e) ORDER BY id) FROM public.world_encounters e WHERE npc_id=$3),
 'relationships',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM public.relationships r WHERE owner_fluctlight_id=$4),
 'memories',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM public.memories x WHERE owner_fluctlight_id=$4),
 'comments',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM public.moment_comments c WHERE source_interaction_id=$2),
 'outbox',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM public.platform_outbox_events o WHERE (kind='moment.comment.created' AND payload->>'source_interaction_id'=$2) OR (kind='world.npc.encountered' AND payload->>'encounter_id' IN (SELECT id FROM public.world_encounters WHERE opportunity_id=$1))))`, m.Opportunity, m.Interaction, m.NPC, m.Fluctlight).Scan(&raw)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func npcRecoveredSocialSnapshot(t *testing.T, ctx context.Context, repo *PostgresRepository, m npcProcessManifest) string {
	t.Helper()
	var raw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT jsonb_build_object(
 'encounters',(SELECT jsonb_agg(to_jsonb(e) ORDER BY id) FROM public.world_encounters e WHERE npc_id=$1),
 'relationships',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM public.relationships r WHERE owner_fluctlight_id=$2),
 'memories',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM public.memories x WHERE owner_fluctlight_id=$2))`, m.NPC, m.Fluctlight).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
