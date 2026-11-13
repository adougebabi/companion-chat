package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func createLegacyUnknownMemoryForTest(t *testing.T, ctx context.Context, repo *PostgresRepository, app *App, owner, fid, key, content string) MemoryApplyResult {
	t.Helper()
	command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ActorRefs: []string{owner}, EventRefs: []string{}, EvidenceRefs: []string{"opaque:" + key}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "legacy:" + key, SemanticReason: "legacy migration fixture", IdempotencyKey: key, Semantic: &MemorySemanticInput{Type: "autobiographical", Content: content, Confidence: 0.9, Importance: 0.9}}
	command.RequestDigest = memoryCommandDigest(command)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.memories SET provenance_status='legacy_unknown' WHERE id=$1`, created.MemoryID); err != nil {
		t.Fatal(err)
	}
	return created
}

func TestLegacyMemoryQuarantinePreservesHistoryAndOwnerReconfirmsSource(t *testing.T) {
	ctx, repo, app, owner, fid, _ := durableReplyFixture(t, "legacy-quarantine")
	created := createLegacyUnknownMemoryForTest(t, ctx, repo, app, owner, fid, "legacy-quarantine", "LEGACY_UNKNOWN_MEMORY 暂存的旧结论")
	plan, err := buildMemoryQueryPlan(MemoryForConversation, []string{owner}, MemoryConversationGlobalOnly, "", nil, "default", []MemoryQueryCue{{Kind: "user", Text: "LEGACY_UNKNOWN_MEMORY"}}, 6, 2400)
	if err != nil {
		t.Fatal(err)
	}
	before, err := app.retrieveMemoryWithPlan(ctx, owner, fid, plan)
	if err != nil || len(before.Items) != 0 {
		t.Fatalf("unresolved legacy memory was automatically retrieved: %#v err=%v", before.Items, err)
	}
	if valid, err := memorySourcesStillValid(ctx, repo.Pool(), created.MemoryID, created.Revision, "legacy_unknown"); err != nil || valid {
		t.Fatalf("unresolved memory passed source validation: valid=%t err=%v", valid, err)
	}
	managed, err := NewMemoryService(repo.Pool(), app).ReadManaged(ctx, owner, created.MemoryID)
	if err != nil || managed["content"] != "LEGACY_UNKNOWN_MEMORY 暂存的旧结论" || managed["status"] != "active" {
		t.Fatalf("quarantine lost the managed record: %#v err=%v", managed, err)
	}
	revision := created.Revision
	if _, err := app.ReviseMemory(ctx, owner, created.MemoryID, "LEGACY_UNKNOWN_MEMORY 经 Owner 重新确认", &revision, []any{"owner:reconfirmed-source"}); err != nil {
		t.Fatal(err)
	}
	after, err := app.retrieveMemoryWithPlan(ctx, owner, fid, plan)
	if err != nil || len(after.Items) != 1 || after.Items[0]["provenance_status"] != "verified" || jsonString(after.Items[0]["effective_source_refs"]) != `["owner:reconfirmed-source"]` {
		t.Fatalf("Owner source confirmation did not restore retrieval: %#v err=%v", after.Items, err)
	}
	var historicalSources int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memory_source_links WHERE memory_id=$1 AND memory_revision=$2 AND source_ref='opaque:legacy-quarantine' AND status='unknown'`, created.MemoryID, created.Revision).Scan(&historicalSources); err != nil || historicalSources != 1 {
		t.Fatalf("quarantine changed historical sources: %d err=%v", historicalSources, err)
	}
}

func TestLegacyActiveMemoryQuarantineRejectsPersistedSnapshotAndCanReconfirm(t *testing.T) {
	ctx, repo, app, owner, fid, _ := durableReplyFixture(t, "legacy-active-quarantine")
	now := time.Now().UTC()
	command := PreparedActiveMemoryMutation{SchemaVersion: activeMemoryLifecycleSchemaVersion, Operation: ActiveMemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: owner, ActorRefs: []string{owner}, EvidenceRefs: []string{"opaque:legacy-active"}, SourceFactID: "opaque:legacy-active", OccurredAt: now, SemanticReason: "legacy active fixture", IdempotencyKey: "legacy-active-create", Semantic: &ActiveMemorySemanticInput{Kind: "commitment", Content: "LEGACY_UNKNOWN_ACTIVE 下周回访", Confidence: 0.9, Importance: 0.9, TimePrecision: "unknown", Timezone: "UTC"}}
	command.RequestDigest = activeMemoryCommandDigest(command)
	created := applyActiveMemoryLifecycleTestCommand(t, ctx, app, command)
	query := ActiveMemoryQuery{AuthorizationActorID: owner, OwnerFluctlightID: fid, Cue: "回访", At: now, Limit: 10}
	before, err := app.retrieveActiveMemories(ctx, query)
	if err != nil || len(before.Items) != 0 {
		t.Fatalf("unknown active source was retrieved: %#v err=%v", before.Items, err)
	}
	published, err := app.readResidentMemorySnapshot(ctx, owner, owner, fid, now)
	if err != nil || len(published.Active) != 0 {
		t.Fatalf("publisher admitted unknown ActiveMemory: %#v err=%v", published, err)
	}
	// Preserve a pre-upgrade snapshot containing the still-active legacy row.
	oldItems := jsonBytes([]any{map[string]any{"source_kind": "active_memory", "id": created.ActiveMemoryID, "revision": created.Revision, "content": "LEGACY_UNKNOWN_ACTIVE 下周回访", "source_validity": "legacy_unknown"}})
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.resident_memory_snapshots SET items_json=$2,source_digest=md5($2::jsonb::text) WHERE fluctlight_id=$1`, fid, oldItems); err != nil {
		t.Fatal(err)
	}
	cached, err := app.readResidentMemorySnapshot(ctx, owner, owner, fid, now)
	if err != nil || len(cached.Active) != 0 {
		t.Fatalf("old snapshot bypassed quarantine: %#v err=%v", cached, err)
	}
	confirmed := command
	confirmed.Operation, confirmed.Semantic = ActiveMemoryConfirm, nil
	confirmed.Target = &ActiveMemoryTarget{ActiveMemoryID: created.ActiveMemoryID, ExpectedRevision: created.Revision}
	confirmed.IdempotencyKey = "legacy-active-owner-reconfirm"
	confirmed.SourceKind = "authenticated_command"
	confirmed.SourceFingerprint = stableDigest(confirmed.IdempotencyKey + "\x1f" + confirmed.SourceFactID)
	confirmed.RequestDigest = activeMemoryCommandDigest(confirmed)
	result := applyActiveMemoryLifecycleTestCommand(t, ctx, app, confirmed)
	after, err := app.retrieveActiveMemories(ctx, query)
	if err != nil || len(after.Items) != 1 || stringValue(after.Items[0]["id"]) != created.ActiveMemoryID || intValue(after.Items[0]["revision"]) != result.Revision {
		t.Fatalf("authenticated Owner confirmation did not restore ActiveMemory: %#v err=%v", after.Items, err)
	}
	resident, err := app.readResidentMemorySnapshot(ctx, owner, owner, fid, now)
	if err != nil || len(resident.Active) != 1 {
		t.Fatalf("reconfirmed ActiveMemory missing from Resident: %#v err=%v", resident, err)
	}
}

func TestLegacyMemoryQuarantineKeepsMixedLiveSupport(t *testing.T) {
	ctx, repo, app, owner, fid, _ := durableReplyFixture(t, "legacy-mixed")
	command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: owner, ActorRefs: []string{owner}, EventRefs: []string{}, EvidenceRefs: []string{"owner:live-support", "fact:missing-legacy-source"}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "owner:live-support", SemanticReason: "mixed provenance fixture", IdempotencyKey: "legacy-mixed", Semantic: &MemorySemanticInput{Type: "semantic", Content: "MIXED_LIVE_MEMORY 保留有依据的资料", Confidence: 0.9, Importance: 0.9}}
	command.RequestDigest = memoryCommandDigest(command)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	// A stale legacy label cannot hide live support or promote the unknown ref.
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.memories SET provenance_status='legacy_unknown' WHERE id=$1`, created.MemoryID); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 150; index++ {
		createLegacyUnknownMemoryForTest(t, ctx, repo, app, owner, fid, fmt.Sprintf("legacy-noise-%d", index), "MIXED_LIVE_MEMORY unknown source")
	}
	plan, err := buildMemoryQueryPlan(MemoryForConversation, []string{owner}, MemoryConversationGlobalOnly, "", nil, "default", []MemoryQueryCue{{Kind: "user", Text: "MIXED_LIVE_MEMORY"}}, 6, 2400)
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.retrieveMemoryWithPlan(ctx, owner, fid, plan)
	if err != nil || len(got.Items) != 1 || got.Trace.AuthorizedCandidateCount != 1 || got.Items[0]["provenance_status"] != "partial" || jsonString(got.Items[0]["effective_source_refs"]) != `["owner:live-support"]` {
		t.Fatalf("mixed live support was lost or unknown promoted: %#v err=%v", got.Items, err)
	}
}

func TestLegacyMemoryQuarantineCoversRecallAndPhysicalReplyRequests(t *testing.T) {
	for _, mode := range []string{"direct", "fact_only"} {
		t.Run(mode, func(t *testing.T) {
			var requests []map[string]any
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				requests = append(requests, payload)
				if payload["model"] == "legacy-policy-writer" {
					return fakeProviderResult{Text: "现在继续聊。"}
				}
				args := map[string]any{}
				if mode == "direct" {
					args = map[string]any{"text": "现在继续聊。", "decision": map[string]any{"action_type": "reply", "response_intent": "回应当前交流", "influences": []any{}}}
				}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("legacy-policy-%d", len(requests)), conversationReplyCapabilityName, args)}}
			})
			fixture := "legacy-policy-" + mode
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, fixture, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, mode)
			if mode == "fact_only" {
				if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,$2,'legacy-policy-writer',30)`, agentModelBindingKey(FormalAgentConversationReply, "private_reply"), "mixed-endpoint-"+fixture); err != nil {
					t.Fatal(err)
				}
			}
			legacyMemory := createLegacyUnknownMemoryForTest(t, ctx, repo, app, owner, fid, fixture, "QUARANTINE_MEMORY_MARKER 旧的自述结论")
			active := activeMemoryLifecycleTestCommand(ActiveMemoryCreate, fixture, time.Now().UTC(), &ActiveMemorySemanticInput{Kind: "commitment", Content: "QUARANTINE_ACTIVE_MARKER 旧承诺", Confidence: 0.9, Importance: 0.9, TimePrecision: "unknown", Timezone: "UTC"}, nil)
			active.OwnerActorID, active.OwnerFluctlightID, active.ActorID = owner, fid, owner
			active.ActorRefs, active.ConversationID = []string{owner}, ""
			active.RequestDigest = activeMemoryCommandDigest(active)
			created := applyActiveMemoryLifecycleTestCommand(t, ctx, app, active)
			oldItems := jsonBytes([]any{map[string]any{"source_kind": "active_memory", "id": created.ActiveMemoryID, "revision": created.Revision, "content": "QUARANTINE_ACTIVE_MARKER 旧承诺", "source_validity": "legacy_unknown"}})
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.resident_memory_snapshots SET items_json=$2,source_digest=md5($2::jsonb::text) WHERE fluctlight_id=$1`, fid, oldItems); err != nil {
				t.Fatal(err)
			}
			recall, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "memory.recall", OperationID: fixture + "-recall", AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: fid, ConversationID: conversation, EvidenceID: fixture + "-recall", Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{"intent": "旧承诺和旧结论"})})
			if err != nil || recall.Result.Status != "completed" {
				t.Fatalf("formal recall failed: %#v err=%v", recall, err)
			}
			result, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "继续聊聊日常吧", "turn_id": fixture, "idempotency_key": fixture, "attachment_refs": []any{}})
			if err != nil || stringValue(result.Assistant["id"]) == "" {
				t.Fatalf("normal %s reply failed after quarantine: %#v err=%v", mode, result, err)
			}
			want := 1
			if mode == "fact_only" {
				want = 2
			}
			if len(requests) != want {
				t.Fatalf("formal request count=%d want=%d", len(requests), want)
			}
			for _, marker := range []string{"QUARANTINE_MEMORY_MARKER", "QUARANTINE_ACTIVE_MARKER"} {
				if strings.Contains(jsonString(recall.Result.Output), marker) || strings.Contains(jsonString(requests), marker) {
					t.Errorf("unknown source reached recall or physical %s request: %s", mode, marker)
				}
			}
			revision := legacyMemory.Revision
			if _, err := app.ReviseMemory(ctx, owner, legacyMemory.MemoryID, "RESTORED_MEMORY_MARKER 经 Owner 重新确认的旧资料", &revision, []any{"owner:physical-reconfirmation"}); err != nil {
				t.Fatal(err)
			}
			if _, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "现在接着聊吧", "turn_id": fixture + "-restored", "idempotency_key": fixture + "-restored", "attachment_refs": []any{}}); err != nil {
				t.Fatal(err)
			}
			if len(requests) != want*2 || !strings.Contains(jsonString(requests[want:]), "RESTORED_MEMORY_MARKER") || strings.Contains(jsonString(requests[want:]), "QUARANTINE_ACTIVE_MARKER") {
				t.Fatalf("Owner-reconfirmed source did not reach the new %s request", mode)
			}
		})
	}
}
