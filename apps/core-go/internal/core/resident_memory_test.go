package core

import (
	"testing"
	"time"
)

func TestResidentMemoryPublishesWholeGenerationAndSkipsUnchangedReplay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "memory-life-owner", "memory-life-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	initial, err := app.readResidentMemorySnapshot(ctx, ownerID, ownerID, fluctlightID, time.Now().UTC())
	if err != nil || initial.Trace.Generation < 1 || initial.Trace.SelectedCount != 0 {
		t.Fatalf("initial Resident snapshot=%#v err=%v", initial, err)
	}
	create := memoryLifecycleTestCommand(MemoryCreate, "resident-relationship", &MemorySemanticInput{Type: "relationship", Content: "与用户约定每周交流", Confidence: 0.9, Importance: 0.95}, nil, nil)
	create.ActorID = ownerID
	create.RequestDigest = memoryCommandDigest(create)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, create)
	first, err := app.readResidentMemorySnapshot(ctx, ownerID, ownerID, fluctlightID, time.Now().UTC())
	if err != nil || first.Trace.Generation <= initial.Trace.Generation || len(first.Memories) != 1 || stringValue(first.Memories[0]["id"]) != created.MemoryID {
		t.Fatalf("relationship not published atomically: %#v err=%v", first, err)
	}
	if replay := applyMemoryLifecycleTestCommand(t, ctx, app, create); !replay.Replayed {
		t.Fatalf("memory command replay failed: %#v", replay)
	}
	unchanged, err := app.readResidentMemorySnapshot(ctx, ownerID, ownerID, fluctlightID, time.Now().UTC())
	if err != nil || unchanged.Trace.Generation != first.Trace.Generation {
		t.Fatalf("unchanged source republished Resident: before=%d after=%d err=%v", first.Trace.Generation, unchanged.Trace.Generation, err)
	}
	revise := memoryLifecycleTestCommand(MemoryRevise, "resident-relationship-correction", &MemorySemanticInput{Type: "relationship", Content: "纠正：与用户约定每月交流", Confidence: 0.95, Importance: 0.95}, &MemoryTarget{MemoryID: created.MemoryID, ExpectedRevision: 0}, nil)
	revise.ActorID, revise.ProposalID = ownerID, ""
	revise.RequestDigest = memoryCommandDigest(revise)
	applyMemoryLifecycleTestCommand(t, ctx, app, revise)
	second, err := app.readResidentMemorySnapshot(ctx, ownerID, ownerID, fluctlightID, time.Now().UTC())
	if err != nil || second.Trace.Generation <= first.Trace.Generation || len(second.Memories) != 1 || stringValue(second.Memories[0]["content"]) != "纠正：与用户约定每月交流" {
		t.Fatalf("corrected Resident generation=%#v err=%v", second, err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.active_memories(id,owner_fluctlight_id,kind,content,status,confidence,importance,source_fact_id,evidence_refs,original_time_expression,time_precision,timezone,last_relevant_at,revision,canonical_key,request_digest,created_at,updated_at)
VALUES('resident-commitment',$1,'commitment','下周完成回访','active',0.9,0.9,'owner-commitment','["owner-commitment"]','','unknown','Asia/Shanghai',now(),1,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',now(),now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	withCommitment, err := app.readResidentMemorySnapshot(ctx, ownerID, ownerID, fluctlightID, time.Now().UTC())
	if err != nil || withCommitment.Trace.Generation <= second.Trace.Generation || len(withCommitment.Active) != 1 || stringValue(withCommitment.Active[0]["content"]) != "下周完成回访" {
		t.Fatalf("unfinished commitment missing from Resident: %#v err=%v", withCommitment, err)
	}
	foreignViewer, err := app.readResidentMemorySnapshot(ctx, ownerID, "another-actor", fluctlightID, time.Now().UTC())
	if err != nil || len(foreignViewer.Memories) != 0 || len(foreignViewer.Active) != 0 {
		t.Fatalf("Resident leaked owner-only sources: %#v err=%v", foreignViewer, err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,2,1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES('resident-source-fact',$1,1,'conversation.turn','{"text":"我们会继续合作"}','resident-source-fact','resident-source-fact','resident-source-fact',now(),'processed')`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	sourced := memoryLifecycleTestCommand(MemoryCreate, "resident-sourced", &MemorySemanticInput{Type: "relationship", Content: "我们会继续合作", Confidence: 0.9, Importance: 0.92}, nil, nil)
	sourced.EvidenceRefs, sourced.SourceFactID = []string{"sequence:1"}, "resident-source-fact"
	sourced.RequestDigest = memoryCommandDigest(sourced)
	sourcedResult := applyMemoryLifecycleTestCommand(t, ctx, app, sourced)
	withSource, err := app.readResidentMemorySnapshot(ctx, ownerID, ownerID, fluctlightID, time.Now().UTC())
	if err != nil || len(withSource.Memories) != 2 {
		t.Fatalf("sourced Resident was not published: %#v err=%v", withSource, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload='{"text":"纠正：不再合作"}' WHERE id='resident-source-fact'`); err != nil {
		t.Fatal(err)
	}
	afterCorrection, err := app.readResidentMemorySnapshot(ctx, ownerID, ownerID, fluctlightID, time.Now().UTC())
	if err != nil || afterCorrection.Trace.Generation <= withSource.Trace.Generation || len(afterCorrection.Memories) != 1 || stringValue(afterCorrection.Memories[0]["id"]) == sourcedResult.MemoryID {
		t.Fatalf("corrected source remained in Resident: %#v err=%v", afterCorrection, err)
	}
}
