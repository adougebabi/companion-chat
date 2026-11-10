package core

import (
	"testing"
	"time"
)

func TestCorrectedSummaryCannotReenterThroughDerivedMemory(t *testing.T) {
	ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 4)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversation, owner, fid); err != nil {
		t.Fatal(err)
	}
	messages, err := readConversationSummaryMessages(ctx, repo.Pool(), conversation, 1, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose) VALUES('derived-summary-provider','openai_compatible','http://fixture.invalid','fixture')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key) VALUES('derived-summary',$1,$2,1,2,$3,$4,'旧摘要','active',1,'derived-summary-provider','fixture','fixture','fixture','fixture','fixture','fixture','fixture')`, fid, conversation, jsonBytes(conversationSummarySourceRefs(messages)), conversationSummarySourceDigest(messages)); err != nil {
		t.Fatal(err)
	}
	var fingerprint string
	if err := repo.Pool().QueryRow(ctx, `SELECT public.conversation_summary_source_fingerprint('derived-summary')`).Scan(&fingerprint); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo}
	create := func(key, content string, refs []string, frozen map[string]FrozenMemoryEvidenceSource) MemoryApplyResult {
		t.Helper()
		command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ConversationID: conversation, ActorRefs: []string{}, EventRefs: []string{}, EvidenceRefs: refs, FrozenEvidenceSources: frozen, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "summary-message-1", SemanticReason: "测试来源", IdempotencyKey: key, Semantic: &MemorySemanticInput{Type: "semantic", Content: content, Confidence: 0.9, Importance: 0.8}}
		command.RequestDigest = memoryCommandDigest(command)
		return applyMemoryLifecycleTestCommand(t, ctx, app, command)
	}
	summaryRef := "conversation_summary:derived-summary"
	frozen := map[string]FrozenMemoryEvidenceSource{summaryRef: {Kind: "conversation_summary", ID: "derived-summary", Revision: 1, Fingerprint: fingerprint}}
	original := create("original", "原来的判断", []string{"message:summary-message-1"}, nil)
	derived := create("derived", "摘要派生的旧判断", []string{summaryRef}, frozen)
	mixed := create("mixed", "混合来源的旧判断", []string{summaryRef, "message:summary-message-3"}, frozen)
	independent := create("independent", "同消息的另一条独立事实", []string{"message:summary-message-1"}, nil)
	var derivedDigest string
	if err := repo.Pool().QueryRow(ctx, `SELECT request_digest FROM public.memories WHERE id=$1`, derived.MemoryID).Scan(&derivedDigest); err != nil {
		t.Fatal(err)
	}
	derivedRef := "memory:" + derived.MemoryID
	nested := create("nested", "另一层旧判断", []string{derivedRef}, map[string]FrozenMemoryEvidenceSource{derivedRef: {Kind: "memory", ID: derived.MemoryID, Revision: derived.Revision, Fingerprint: derivedDigest}})
	// Recreate an older Confirm that retained the ref but lost its typed
	// identity. The guard must still follow its last Confirm-only ancestry.
	nestedRow, err := app.readOwnedMemoryAuthorityRow(ctx, owner, nested.MemoryID)
	if err != nil {
		t.Fatal(err)
	}
	confirm := buildOwnerMemoryCommand(nestedRow, owner, MemoryConfirm, nested.Revision, []string{"nested-owner-confirmation"}, nil, "confirm nested memory", nil, nil)
	nested, err = app.applyOwnerMemoryCommand(ctx, confirm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.memory_source_links SET source_kind='legacy',source_id=NULL,source_revision=NULL,source_fingerprint=NULL,status='unknown' WHERE memory_id=$1 AND memory_revision=$2 AND source_ref=$3`, nested.MemoryID, nested.Revision, derivedRef); err != nil {
		t.Fatal(err)
	}
	// Exercise a previously materialized global Resident snapshot as well as
	// normal conversation-scoped retrieval. These rows are isolated fixtures.
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.memories SET conversation_id=NULL,type='relationship' WHERE id=ANY($1::text[])`, []string{derived.MemoryID, mixed.MemoryID, independent.MemoryID}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `SELECT public.refresh_resident_memory_snapshot($1)`, fid); err != nil {
		t.Fatal(err)
	}
	var oldSnapshot []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT items_json FROM public.resident_memory_snapshots WHERE fluctlight_id=$1`, fid).Scan(&oldSnapshot); err != nil {
		t.Fatal(err)
	}
	residentBefore, err := app.readResidentMemorySnapshot(ctx, owner, owner, fid, time.Now())
	if err != nil || len(residentBefore.Memories) != 3 {
		t.Fatalf("fixture resident snapshot=%#v err=%v", residentBefore, err)
	}
	plan, err := buildMemoryQueryPlan(MemoryForCapabilityPlanner, []string{owner}, MemoryConversationExact, conversation, nil, "default", []MemoryQueryCue{{Kind: "recall_intent", Text: "判断"}}, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	read := func() map[string]bool {
		t.Helper()
		result, err := app.retrieveMemoryWithPlan(ctx, owner, fid, plan)
		if err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		for _, item := range result.Items {
			ids[stringValue(item["id"])] = true
		}
		return ids
	}
	before := read()
	if !before[original.MemoryID] || !before[derived.MemoryID] || !before[mixed.MemoryID] || !before[independent.MemoryID] || !before[nested.MemoryID] {
		t.Fatalf("fixture memories missing before correction: %#v", before)
	}
	revision := original.Revision
	if _, err := app.ReviseMemory(ctx, owner, original.MemoryID, "更正后的判断", &revision, []any{"owner-confirmation"}); err != nil {
		t.Fatal(err)
	}
	var sourceLive bool
	if err := repo.Pool().QueryRow(ctx, `SELECT public.memory_source_is_live('conversation_summary','derived-summary',1,$1)`, fingerprint).Scan(&sourceLive); err != nil {
		t.Fatal(err)
	}
	if sourceLive {
		t.Fatal("corrected summary is still a live Memory source")
	}
	valid, err := memorySourcesStillValid(ctx, repo.Pool(), mixed.MemoryID, mixed.Revision, "partial")
	if err != nil || valid {
		t.Fatalf("mixed-source derivative still valid=%t err=%v", valid, err)
	}
	after := read()
	if !after[original.MemoryID] || !after[independent.MemoryID] || after[derived.MemoryID] || after[mixed.MemoryID] || after[nested.MemoryID] {
		t.Fatalf("correction leaked through dependent Memory or removed independent facts: %#v", after)
	}
	items, _, err := newMemoryRecallService(app).Recall(ctx, MemoryRecallRequest{AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, ViewerActorIDs: []string{owner}, ConversationMode: MemoryConversationExact, ActiveProfileID: "default", Intent: "判断"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if stringValue(item["source_kind"]) == "long_term_memory" && (stringValue(item["content"]) == "摘要派生的旧判断" || stringValue(item["content"]) == "混合来源的旧判断") {
			t.Fatalf("memory.recall exposed corrected-summary derivative: %#v", item)
		}
	}
	if _, err := repo.Pool().Exec(ctx, `SELECT public.refresh_resident_memory_snapshot($1)`, fid); err != nil {
		t.Fatal(err)
	}
	assertResident := func() {
		t.Helper()
		resident, err := app.readResidentMemorySnapshot(ctx, owner, owner, fid, time.Now())
		if err != nil || len(resident.Memories) != 1 || stringValue(resident.Memories[0]["id"]) != independent.MemoryID {
			t.Fatalf("resident exposed a corrected derivative or lost independent memory: %#v err=%v", resident, err)
		}
	}
	assertResident()
	// Existing snapshots can predate deployment or a source correction.
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.resident_memory_snapshots SET items_json=$2::jsonb,source_digest=md5(($2::jsonb)::text) WHERE fluctlight_id=$1`, fid, oldSnapshot); err != nil {
		t.Fatal(err)
	}
	assertResident()
	// A subsequent explicit correction can replace the polluted evidence;
	// the guard must not permanently suppress its new, independently supported revision.
	mixedRevision := mixed.Revision
	if _, err := app.ReviseMemory(ctx, owner, mixed.MemoryID, "独立确认的新判断", &mixedRevision, []any{"new-owner-confirmation"}); err != nil {
		t.Fatal(err)
	}
	afterReplace := read()
	if !afterReplace[mixed.MemoryID] || !afterReplace[independent.MemoryID] {
		t.Fatalf("replaced evidence stayed fenced: %#v", afterReplace)
	}
	derivedRevision := derived.Revision
	if _, err := app.ReviseMemory(ctx, owner, derived.MemoryID, "重新确认的上游判断", &derivedRevision, []any{"replacement-owner-confirmation"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT request_digest,revision FROM public.memories WHERE id=$1`, derived.MemoryID).Scan(&derivedDigest, &derivedRevision); err != nil {
		t.Fatal(err)
	}
	row, err := app.readOwnedMemoryAuthorityRow(ctx, owner, nested.MemoryID)
	if err != nil {
		t.Fatal(err)
	}
	update := buildOwnerMemoryCommand(row, owner, MemoryRevise, nested.Revision, []string{derivedRef}, &MemorySemanticInput{Type: "semantic", Content: "引用新版本后的判断", Confidence: 0.9, Importance: 0.8}, "refreeze corrected source", nil, nil)
	update.FrozenEvidenceSources = map[string]FrozenMemoryEvidenceSource{derivedRef: {Kind: "memory", ID: derived.MemoryID, Revision: derivedRevision, Fingerprint: derivedDigest}}
	update.RequestDigest = memoryCommandDigest(update)
	if _, err := app.applyOwnerMemoryCommand(ctx, update); err != nil {
		t.Fatal(err)
	}
	if !read()[nested.MemoryID] {
		t.Fatal("same source ref rebound to the corrected Memory revision stayed fenced")
	}
}
