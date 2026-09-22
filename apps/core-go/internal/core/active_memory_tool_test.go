package core

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestPostgresDirectActiveMemoryToolCommitsRejectsReplaysAndPersistsAudit(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID := "actor_active_tool_" + suffix
	fluctlightID := "fluctlight_active_tool_" + suffix
	conversationID := "conversation_active_tool_" + suffix
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}',$3,'{}','{}','{}','{}')`, fluctlightID, ownerID, jsonBytes(map[string]any{"timezone": "UTC"})); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'direct active memory')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	frozenNow := time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC)
	app := &App{DB: repository, Clock: fixedClock(frozenNow)}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = mustCapabilityRegistry(activeMemoryEventCapability{service: app})
	request := ToolExecutionRequest{
		CapabilityName: "active_memory_event", OperationID: "active-create-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		EvidenceID: "owner-command-" + suffix, Surface: CapabilitySurfaceNativeCognition,
		Arguments: jsonBytes(map[string]any{
			"operation": "create", "kind": "commitment", "content": "今晚完成发布检查",
			"confidence": 0.95, "importance": 0.9, "original_time_expression": "", "time_precision": "unknown",
		}),
	}
	created, err := app.ExecuteTool(ctx, request)
	if err != nil || created.Result.Status != "completed" || !boolValueForTest(mapValue(created.Result.Output)["recorded"]) {
		t.Fatalf("active create receipt=%#v err=%v", created, err)
	}
	var memoryID, sourceFact, actorID string
	var occurredAt time.Time
	if err := repository.Pool().QueryRow(ctx, `SELECT id,source_fact_id,last_relevant_at FROM public.active_memories WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&memoryID, &sourceFact, &occurredAt); err != nil {
		t.Fatal(err)
	}
	if sourceFact != request.EvidenceID || !occurredAt.Equal(frozenNow) {
		t.Fatalf("stored source=%q occurred_at=%s", sourceFact, occurredAt)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT actor_id FROM public.active_memory_commands WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&actorID); err != nil || actorID != ownerID {
		t.Fatalf("command actor=%q err=%v", actorID, err)
	}
	var fabricated int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE id=$1`, request.EvidenceID).Scan(&fabricated); err != nil || fabricated != 0 {
		t.Fatalf("direct Tool fabricated cognition rows=%d err=%v", fabricated, err)
	}
	retry := request
	retry.NativeToolCallID, retry.ProviderRequestID = "native-active-retry-"+suffix, "provider-active-retry-"+suffix
	replayed, err := app.ExecuteTool(ctx, retry)
	if err != nil || replayed.Result.Status != "completed" || !boolValueForTest(mapValue(replayed.Result.Output)["replayed"]) {
		t.Fatalf("active replay receipt=%#v err=%v", replayed, err)
	}
	conflict := request
	conflict.Arguments = jsonBytes(map[string]any{
		"operation": "create", "kind": "commitment", "content": "不同内容",
		"confidence": 0.95, "importance": 0.9, "original_time_expression": "", "time_precision": "unknown",
	})
	if receipt, conflictErr := app.ExecuteTool(ctx, conflict); conflictErr == nil || receipt.Result.Status != "failed" || !errors.Is(conflictErr, ErrConflict) {
		t.Fatalf("active conflict receipt=%#v err=%v", receipt, conflictErr)
	}
	unknownTarget := ToolExecutionRequest{
		CapabilityName: "active_memory_event", OperationID: "active-close-missing-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		EvidenceID: "owner-close-command-" + suffix,
		Arguments:  jsonBytes(map[string]any{"operation": "complete", "target_ref": "active_memory:ctx_0123456789abcdef0123456789abcdef"}),
	}
	if receipt, rejectErr := app.ExecuteTool(ctx, unknownTarget); rejectErr == nil || receipt.Result.Status != "failed" {
		t.Fatalf("unknown target receipt=%#v err=%v", receipt, rejectErr)
	}
	ref := recallOpaqueRef("active_memory", memoryID+":1", MemoryRecallRequest{FluctlightID: fluctlightID, ConversationID: conversationID})
	closeRequest := unknownTarget
	closeRequest.OperationID = "active-close-" + suffix
	closeRequest.Arguments = jsonBytes(map[string]any{"operation": "complete", "target_ref": ref})
	closed, err := app.ExecuteTool(ctx, closeRequest)
	if err != nil || closed.Result.Status != "completed" || stringValue(mapValue(closed.Result.Output)["status"]) != "completed" {
		t.Fatalf("active close receipt=%#v err=%v", closed, err)
	}
	var status string
	var revision, commands, revisions, outbox int
	if err := repository.Pool().QueryRow(ctx, `SELECT status,revision FROM public.active_memories WHERE id=$1`, memoryID).Scan(&status, &revision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.active_memory_commands WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.active_memory_revisions WHERE active_memory_id=$1`, memoryID).Scan(&revisions); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind='active_memory.lifecycle.applied'`, fluctlightID).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || revision != 2 || commands != 2 || revisions != 2 || outbox != 2 {
		t.Fatalf("persistence status=%s revision=%d commands=%d revisions=%d outbox=%d", status, revision, commands, revisions, outbox)
	}
}
