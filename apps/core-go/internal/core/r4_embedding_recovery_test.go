package core

import (
	"net/http"
	"testing"
)

func TestR4MissingEmbeddingKeepsSavedMemoryAndNamesConfiguration(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	owner, fid := "r4-owner", "r4-role"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, owner, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fid, owner); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo, Provider: &ProviderClient{DB: repo, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		t.Error("unconfigured embedding called a provider")
		return embeddingHTTPResponse(request, 500, `{}`), nil
	})}}}
	command := memoryLifecycleTestCommand(MemoryCreate, "r4-embedding", &MemorySemanticInput{Type: "semantic", Content: "实际保存的记忆", Confidence: .9, Importance: .8}, nil, nil)
	command.OwnerFluctlightID, command.OwnerActorID, command.ActorID = fid, owner, fid
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES('r4-memory-source',$1,1,'conversation.turn','{"text":"实际保存的记忆"}','r4','r4','r4-memory-source',$2,'processed',now())`, fid, command.OccurredAt); err != nil {
		t.Fatal(err)
	}
	command.EvidenceRefs = []string{"fact:r4-memory-source"}
	command.RequestDigest = memoryCommandDigest(command)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	confirm := memoryLifecycleTestCommand(MemoryConfirm, "r4-confirm", nil, &MemoryTarget{Ref: "saved", MemoryID: created.MemoryID, ExpectedRevision: created.Revision}, nil)
	confirm.OwnerFluctlightID, confirm.OwnerActorID, confirm.ActorID = fid, owner, fid
	confirm.EvidenceRefs = []string{"fact:r4-memory-source"}
	confirm.RequestDigest = memoryCommandDigest(confirm)
	confirmed := applyMemoryLifecycleTestCommand(t, ctx, app, confirm)
	_, err := app.ProcessMemoryEmbeddingIntentAt(ctx, "", created.MemoryID, confirmed.Revision, "", "")
	if err == nil || err.Error() != "memory_embedding_unconfigured" {
		t.Fatal("missing embedding was not actionable", err)
	}
	var content, status string
	var revision, count int
	if err := repo.Pool().QueryRow(ctx, `SELECT content,status,revision FROM public.memories WHERE id=$1`, created.MemoryID).Scan(&content, &status, &revision); err != nil {
		t.Fatal(err)
	}
	if content != "实际保存的记忆" || status != "active" || revision != confirmed.Revision {
		t.Fatal("memory authority changed", status, revision)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memory_embeddings WHERE memory_id=$1`, created.MemoryID).Scan(&count); err != nil || count != 0 {
		t.Fatal("unavailable vector was fabricated", count, err)
	}
}
