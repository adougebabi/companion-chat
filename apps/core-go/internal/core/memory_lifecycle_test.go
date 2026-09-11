package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func memoryLifecycleTestCommand(operation MemoryOperation, key string, semantic *MemorySemanticInput, target *MemoryTarget, merge []MemoryTarget) PreparedMemoryMutation {
	command := PreparedMemoryMutation{
		SchemaVersion: memoryLifecycleSchemaVersion, Operation: operation,
		OwnerFluctlightID: "memory-life-fluctlight", OwnerActorID: "memory-life-owner", ActorID: "memory-life-fluctlight",
		ActorRefs: []string{}, EventRefs: []string{}, EvidenceRefs: []string{"evidence:" + key},
		Target: target, MergeTargets: merge, Semantic: semantic, Visibility: "private",
		OccurredAt:   time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC).Add(time.Duration(len(key)) * time.Minute),
		SourceFactID: "source:" + key, SourceWindow: "window:" + key,
		ProposalID: "proposal:" + key, CandidateIndex: 0, SemanticReason: "test " + key,
		IdempotencyKey: "memory-command:" + key,
	}
	command.RequestDigest = memoryCommandDigest(command)
	return command
}

func TestProcessMemoryEmbeddingRevisionZeroIsExactAndRequiresActiveMemory(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-embed-owner"
	fluctlightID := "memory-embed-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	var providerCalls atomic.Int32
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		providerCalls.Add(1)
		return embeddingHTTPResponse(request, http.StatusOK, `{"data":[{"embedding":[0.1,0.2]}]}`), nil
	})}}}
	semantic := &MemorySemanticInput{Type: "semantic", Content: "revision zero", Confidence: 0.9, Importance: 0.8, EmotionalSignificance: 0.2}
	create := memoryLifecycleTestCommand(MemoryCreate, "embedding-revision-zero", semantic, nil, nil)
	create.OwnerFluctlightID, create.OwnerActorID, create.ActorID = fluctlightID, ownerID, fluctlightID
	create.RequestDigest = memoryCommandDigest(create)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, create)
	confirm := memoryLifecycleTestCommand(MemoryConfirm, "embedding-confirm", nil, &MemoryTarget{Ref: "memory-ref", MemoryID: created.MemoryID, ExpectedRevision: 0}, nil)
	confirm.OwnerFluctlightID, confirm.OwnerActorID, confirm.ActorID = fluctlightID, ownerID, fluctlightID
	confirm.RequestDigest = memoryCommandDigest(confirm)
	confirmed := applyMemoryLifecycleTestCommand(t, ctx, app, confirm)
	result, err := app.ProcessMemoryEmbeddingIntentAt(ctx, "", created.MemoryID, 0, "", "")
	if err != nil || stringValue(result["status"]) != "stale" || providerCalls.Load() != 0 {
		t.Fatalf("revision-zero stale result=%#v err=%v provider_calls=%d", result, err, providerCalls.Load())
	}
	deprecate := memoryLifecycleTestCommand(MemoryDeprecate, "embedding-deprecate", nil, &MemoryTarget{Ref: "memory-ref", MemoryID: created.MemoryID, ExpectedRevision: confirmed.Revision}, nil)
	deprecate.OwnerFluctlightID, deprecate.OwnerActorID, deprecate.ActorID = fluctlightID, ownerID, fluctlightID
	deprecate.RequestDigest = memoryCommandDigest(deprecate)
	deprecated := applyMemoryLifecycleTestCommand(t, ctx, app, deprecate)
	result, err = app.ProcessMemoryEmbeddingIntentAt(ctx, "", created.MemoryID, deprecated.Revision, "", "")
	if err != nil || stringValue(result["status"]) != "stale" || providerCalls.Load() != 0 {
		t.Fatalf("deprecated result=%#v err=%v provider_calls=%d", result, err, providerCalls.Load())
	}
	if _, err := app.ProcessMemoryEmbeddingIntentAt(ctx, "", created.MemoryID, -1, "", ""); err == nil || err.Error() != "memory_embedding_revision_invalid" {
		t.Fatalf("negative revision err=%v", err)
	}
}

func TestProcessMemoryEmbeddingFailureThenSuccessUsesOneFrozenTuple(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-embed-retry-owner"
	fluctlightID := "memory-embed-retry-fluctlight"
	endpointID := "memory-embed-endpoint-a"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://embedding-a.invalid','memory-embed-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('embedding',$1,'embedding-model-a','embedding',4096,5,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	var providerCalls atomic.Int32
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		call := providerCalls.Add(1)
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"model":"embedding-model-a"`) {
			t.Fatalf("embedding assignment was not frozen: %s", body)
		}
		if call == 1 {
			return embeddingHTTPResponse(request, http.StatusServiceUnavailable, `{}`), nil
		}
		return embeddingHTTPResponse(request, http.StatusOK, `{"data":[{"embedding":[0.25,-0.5,0.75]}]}`), nil
	})}}}
	semantic := &MemorySemanticInput{Type: "semantic", Content: "retry one tuple", Confidence: 0.9, Importance: 0.8, EmotionalSignificance: 0.2}
	create := memoryLifecycleTestCommand(MemoryCreate, "embedding-retry", semantic, nil, nil)
	create.OwnerFluctlightID, create.OwnerActorID, create.ActorID = fluctlightID, ownerID, fluctlightID
	create.RequestDigest = memoryCommandDigest(create)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, create)
	intentID := "memory_embedding_intent:" + created.MemoryID + ":0"
	var intentPayload []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&intentPayload); err != nil {
		t.Fatal(err)
	}
	payload := decodeObject(intentPayload)
	if stringValue(payload["provider_endpoint_id"]) != endpointID || stringValue(payload["model_id"]) != "embedding-model-a" {
		t.Fatalf("embedding binding was not frozen in intent: %#v", payload)
	}
	if _, err := app.ProcessMemoryEmbeddingIntentAt(ctx, intentID, created.MemoryID, 0, endpointID, "embedding-model-a"); err == nil {
		t.Fatal("first Provider failure was not returned")
	}
	var rowCount, dimensions int
	var status, errorCode string
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*),max(status),max(dimensions),max(COALESCE(error_code,'')) FROM public.memory_embeddings WHERE memory_id=$1 AND memory_revision=0 AND model_id='embedding-model-a'`, created.MemoryID).Scan(&rowCount, &status, &dimensions, &errorCode); err != nil {
		t.Fatal(err)
	}
	if rowCount != 1 || status != "failed" || dimensions != 0 || errorCode != "provider_request_failed" {
		t.Fatalf("failed tuple count=%d status=%s dimensions=%d error=%s", rowCount, status, dimensions, errorCode)
	}
	if _, err := app.ProcessMemoryEmbeddingIntentAt(ctx, intentID, created.MemoryID, 0, "other-endpoint", "embedding-model-a"); err == nil || err.Error() != "memory_embedding_assignment_conflict" {
		t.Fatalf("mismatched frozen assignment err=%v", err)
	}
	result, err := app.ProcessMemoryEmbeddingIntentAt(ctx, intentID, created.MemoryID, 0, "", "")
	if err != nil || stringValue(result["status"]) != "ready" || intValue(result["dimensions"]) != 3 {
		t.Fatalf("ready result=%#v err=%v", result, err)
	}
	result, err = app.ProcessMemoryEmbeddingIntentAt(ctx, intentID, created.MemoryID, 0, "", "")
	if err != nil || !boolValueForTest(result["replayed"]) || providerCalls.Load() != 2 {
		t.Fatalf("ready replay=%#v err=%v provider_calls=%d", result, err, providerCalls.Load())
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*),max(status),max(dimensions),max(COALESCE(error_code,'')) FROM public.memory_embeddings WHERE memory_id=$1 AND memory_revision=0 AND model_id='embedding-model-a'`, created.MemoryID).Scan(&rowCount, &status, &dimensions, &errorCode); err != nil {
		t.Fatal(err)
	}
	if rowCount != 1 || status != "ready" || dimensions != 3 || errorCode != "" {
		t.Fatalf("ready tuple count=%d status=%s dimensions=%d error=%s", rowCount, status, dimensions, errorCode)
	}
}

func TestOwnerMemoryReviseRollbackAndForgetUseLifecycleAuthority(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-owner-api-human"
	fluctlightID := "memory-owner-api-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	original := &MemorySemanticInput{Type: "semantic", Content: "原始内容", Confidence: 0.8, Importance: 0.7, EmotionalSignificance: 0.2}
	create := memoryLifecycleTestCommand(MemoryCreate, "owner-api-create", original, nil, nil)
	create.OwnerFluctlightID, create.OwnerActorID, create.ActorID = fluctlightID, ownerID, ownerID
	create.RequestDigest = memoryCommandDigest(create)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, create)
	zero := 0
	revised, err := app.ReviseMemory(ctx, ownerID, created.MemoryID, "修订内容", &zero, []any{"owner-command:revise"})
	if err != nil || intValue(revised["revision"]) != 1 || stringValue(revised["operation"]) != "revise" {
		t.Fatalf("revise=%#v err=%v", revised, err)
	}
	replayed, err := app.ReviseMemory(ctx, ownerID, created.MemoryID, "修订内容", &zero, []any{"owner-command:revise"})
	if err != nil || !boolValueForTest(replayed["replayed"]) || intValue(replayed["revision"]) != 1 {
		t.Fatalf("revise replay=%#v err=%v", replayed, err)
	}
	rolledBack, err := app.RollbackMemory(ctx, ownerID, created.MemoryID, 0, 1, []any{"owner-command:rollback"})
	if err != nil || intValue(rolledBack["revision"]) != 2 || stringValue(rolledBack["operation"]) != "rollback" {
		t.Fatalf("rollback=%#v err=%v", rolledBack, err)
	}
	var content, status string
	var revision int
	if err := repository.Pool().QueryRow(ctx, `SELECT content,status,revision FROM public.memories WHERE id=$1`, created.MemoryID).Scan(&content, &status, &revision); err != nil || content != "原始内容" || status != "active" || revision != 2 {
		t.Fatalf("rolled-back Memory content=%q status=%q revision=%d err=%v", content, status, revision, err)
	}
	two := 2
	forgotten, err := app.ForgetMemory(ctx, ownerID, created.MemoryID, &two, []any{"owner-command:forget"})
	if err != nil || stringValue(forgotten["status"]) != "forgotten" || intValue(forgotten["revision"]) != 3 {
		t.Fatalf("forget=%#v err=%v", forgotten, err)
	}
	forgottenReplay, err := app.ForgetMemory(ctx, ownerID, created.MemoryID, &two, []any{"owner-command:forget"})
	if err != nil || !boolValueForTest(forgottenReplay["replayed"]) {
		t.Fatalf("forget replay=%#v err=%v", forgottenReplay, err)
	}
	three := 3
	if _, err := app.ReviseMemory(ctx, ownerID, created.MemoryID, "不应复活", &three, []any{"owner-command:invalid"}); err == nil {
		t.Fatal("forgotten Memory was revised back to active")
	}
	forgottenRollback, err := app.RollbackMemory(ctx, ownerID, created.MemoryID, 2, 3, []any{"owner-command:rollback-forget"})
	if err != nil || stringValue(forgottenRollback["status"]) != "active" || intValue(forgottenRollback["revision"]) != 4 {
		t.Fatalf("forgotten rollback=%#v err=%v", forgottenRollback, err)
	}
	deprecate := memoryLifecycleTestCommand(MemoryDeprecate, "owner-api-deprecate", nil, &MemoryTarget{Ref: "memory-owner-api-ref", MemoryID: created.MemoryID, ExpectedRevision: 4}, nil)
	deprecate.OwnerFluctlightID, deprecate.OwnerActorID, deprecate.ActorID = fluctlightID, ownerID, ownerID
	deprecate.RequestDigest = memoryCommandDigest(deprecate)
	deprecated := applyMemoryLifecycleTestCommand(t, ctx, app, deprecate)
	if deprecated.Status != "deprecated" || deprecated.Revision != 5 {
		t.Fatalf("deprecated=%#v", deprecated)
	}
	deprecatedRollback, err := app.RollbackMemory(ctx, ownerID, created.MemoryID, 4, 5, []any{"owner-command:rollback-deprecate"})
	if err != nil || stringValue(deprecatedRollback["status"]) != "active" || intValue(deprecatedRollback["revision"]) != 6 {
		t.Fatalf("deprecated rollback=%#v err=%v", deprecatedRollback, err)
	}
	var governanceCount, revisionCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memory_governance WHERE target_memory_id=$1 OR result->>'memory_id'=$1`, created.MemoryID).Scan(&governanceCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memory_revisions WHERE memory_id=$1`, created.MemoryID).Scan(&revisionCount); err != nil {
		t.Fatal(err)
	}
	if governanceCount != 7 || revisionCount != 7 {
		t.Fatalf("authority counts governance=%d revisions=%d", governanceCount, revisionCount)
	}
}

func TestOwnerMemoryRollbackCompensatesMergeAndSupersedeLineage(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-lineage-owner"
	fluctlightID := "memory-lineage-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	create := func(key, content string) MemoryApplyResult {
		semantic := &MemorySemanticInput{Type: "semantic", Content: content, Confidence: 0.8, Importance: 0.7, EmotionalSignificance: 0.2}
		command := memoryLifecycleTestCommand(MemoryCreate, key, semantic, nil, nil)
		command.OwnerFluctlightID, command.OwnerActorID, command.ActorID = fluctlightID, ownerID, fluctlightID
		command.RequestDigest = memoryCommandDigest(command)
		return applyMemoryLifecycleTestCommand(t, ctx, app, command)
	}
	primary := create("lineage-primary", "合并前主记忆")
	secondary := create("lineage-secondary", "合并前次记忆")
	mergedSemantic := &MemorySemanticInput{Type: "semantic", Content: "合并后的记忆", Confidence: 0.9, Importance: 0.8, EmotionalSignificance: 0.3}
	merge := memoryLifecycleTestCommand(MemoryMerge, "lineage-merge", mergedSemantic, &MemoryTarget{Ref: "primary-ref", MemoryID: primary.MemoryID, ExpectedRevision: 0}, []MemoryTarget{{Ref: "secondary-ref", MemoryID: secondary.MemoryID, ExpectedRevision: 0}})
	merge.OwnerFluctlightID, merge.OwnerActorID, merge.ActorID = fluctlightID, ownerID, fluctlightID
	merge.RequestDigest = memoryCommandDigest(merge)
	merged := applyMemoryLifecycleTestCommand(t, ctx, app, merge)
	if merged.Revision != 1 {
		t.Fatalf("merged=%#v", merged)
	}
	mergeRollback, err := app.RollbackMemory(ctx, ownerID, primary.MemoryID, 0, 1, []any{"owner-command:rollback-merge"})
	if err != nil || intValue(mergeRollback["revision"]) != 2 {
		t.Fatalf("merge rollback=%#v err=%v", mergeRollback, err)
	}
	for memoryID, content := range map[string]string{primary.MemoryID: "合并前主记忆", secondary.MemoryID: "合并前次记忆"} {
		var currentContent, status string
		var revision int
		if err := repository.Pool().QueryRow(ctx, `SELECT content,status,revision FROM public.memories WHERE id=$1`, memoryID).Scan(&currentContent, &status, &revision); err != nil || currentContent != content || status != "active" || revision != 2 {
			t.Fatalf("merge compensation %s content=%q status=%q revision=%d err=%v", memoryID, currentContent, status, revision, err)
		}
	}
	replacementSemantic := &MemorySemanticInput{Type: "semantic", Content: "替代后的新记忆", Confidence: 0.95, Importance: 0.85, EmotionalSignificance: 0.25}
	supersede := memoryLifecycleTestCommand(MemorySupersede, "lineage-supersede", replacementSemantic, &MemoryTarget{Ref: "primary-ref-v2", MemoryID: primary.MemoryID, ExpectedRevision: 2}, nil)
	supersede.OwnerFluctlightID, supersede.OwnerActorID, supersede.ActorID = fluctlightID, ownerID, fluctlightID
	supersede.RequestDigest = memoryCommandDigest(supersede)
	replacement := applyMemoryLifecycleTestCommand(t, ctx, app, supersede)
	if replacement.MemoryID == primary.MemoryID {
		t.Fatalf("supersede did not create replacement: %#v", replacement)
	}
	supersedeRollback, err := app.RollbackMemory(ctx, ownerID, primary.MemoryID, 2, 3, []any{"owner-command:rollback-supersede"})
	if err != nil || intValue(supersedeRollback["revision"]) != 4 {
		t.Fatalf("supersede rollback=%#v err=%v", supersedeRollback, err)
	}
	var primaryContent, primaryStatus, replacementStatus string
	var primaryRevision, replacementRevision int
	if err := repository.Pool().QueryRow(ctx, `SELECT content,status,revision FROM public.memories WHERE id=$1`, primary.MemoryID).Scan(&primaryContent, &primaryStatus, &primaryRevision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status,revision FROM public.memories WHERE id=$1`, replacement.MemoryID).Scan(&replacementStatus, &replacementRevision); err != nil {
		t.Fatal(err)
	}
	if primaryContent != "合并前主记忆" || primaryStatus != "active" || primaryRevision != 4 || replacementStatus != "deprecated" || replacementRevision != 1 {
		t.Fatalf("supersede compensation primary=(%q,%q,%d) replacement=(%q,%d)", primaryContent, primaryStatus, primaryRevision, replacementStatus, replacementRevision)
	}
	replayed, err := app.RollbackMemory(ctx, ownerID, primary.MemoryID, 2, 3, []any{"owner-command:rollback-supersede"})
	if err != nil || !boolValueForTest(replayed["replayed"]) || intValue(replayed["revision"]) != 4 {
		t.Fatalf("supersede rollback replay=%#v err=%v", replayed, err)
	}
}

func TestProcessReflectionMemoryCandidateUsesOpaqueRefLifecycleAuthority(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-reflection-owner"
	fluctlightID := "memory-reflection-fluctlight"
	endpointID := "memory-reflection-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://reflection.invalid','memory-reflection-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection',$1,'reflection-model','structured_output',4096,5,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{"personality_system":{"active_profile_id":"default"}}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES('reflection-fact-1',$1,1,'conversation.turn','{"conversation_id":"conversation-reflection","text":"我喜欢靠窗的安静位置"}','turn-1','reflection-test','reflection-fact-1',now(),'processed',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	var providerCalls atomic.Int32
	var citedMemoryRef string
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		call := providerCalls.Add(1)
		body, _ := io.ReadAll(request.Body)
		candidate := reflectionMemorySemanticCandidate("create")
		candidate["evidence_refs"] = []any{"sequence:1"}
		candidate["content"] = "用户偏好靠窗的安静位置"
		if call == 2 {
			if !strings.Contains(string(body), "scene_event") || !strings.Contains(string(body), "completed") {
				t.Fatalf("second Reflection request lost the typed ActionOutcome: %s", body)
			}
			matches := regexp.MustCompile(`memory:ctx_[a-f0-9]{32}`).FindAllString(string(body), -1)
			if len(matches) == 0 {
				t.Fatalf("second Reflection request has no opaque Memory ref: %s", body)
			}
			citedMemoryRef = matches[0]
			candidate = reflectionMemorySemanticCandidate("revise")
			candidate["target_ref"] = citedMemoryRef
			candidate["evidence_refs"] = []any{"sequence:2", citedMemoryRef}
			candidate["content"] = "用户明确偏好靠窗、安静且光线柔和的位置"
		}
		proposal := reflectionProposalV2Fixture([]any{candidate})
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(proposal)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: providerHTTP}}
	first, err := app.ProcessReflection(ctx, fluctlightID, "reflection-correlation-1")
	if err != nil || stringValue(first["status"]) != "applied" || intValue(mapValue(mapValue(first["memory"])["counts"])["applied"]) != 1 {
		t.Fatalf("first Reflection=%#v err=%v", first, err)
	}
	var memoryID, conversationID string
	var revision int
	if err := repository.Pool().QueryRow(ctx, `SELECT id,COALESCE(conversation_id,''),revision FROM public.memories WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&memoryID, &conversationID, &revision); err != nil || conversationID != "conversation-reflection" || revision != 0 {
		t.Fatalf("created Memory id=%q conversation=%q revision=%d err=%v", memoryID, conversationID, revision, err)
	}
	projectionBeforeOutcome, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		SourceFactID: "decision-source", MemoryOperation: MemoryForReflection,
		MemoryConversationMode: MemoryConversationAllowedSet, AllowedConversationIDs: []string{"conversation-reflection"},
		MemoryCues: []MemoryQueryCue{{Kind: "test", Text: "靠窗 安静"}},
	})
	if err != nil || len(projectionBeforeOutcome.Memories) != 1 {
		t.Fatalf("pre-outcome projection=%#v err=%v", projectionBeforeOutcome.Memories, err)
	}
	decisionMemoryRef := stringValue(projectionBeforeOutcome.Memories[0]["ref"])
	influences, err := validateDecisionInfluences([]any{map[string]any{"ref": decisionMemoryRef, "role": "grounds", "confidence": 0.9, "note": "该记忆约束了本次场景决定"}}, projectionBeforeOutcome.ReferenceIndex)
	if err != nil || len(influences) != 1 {
		t.Fatalf("Memory influence=%#v err=%v", influences, err)
	}
	outcome := ActionOutcome{
		SchemaVersion: actionOutcomeSchemaVersion, ID: "reflection-memory-outcome", FluctlightID: fluctlightID,
		ActionID: "reflection-memory-action", CallID: "reflection-memory-call", CapabilityName: "scene_event",
		Status: ActionOutcomeCompleted, SuccessBoundary: "scene_committed", Expected: map[string]any{},
		Observed: map[string]any{"status": "active"}, GoalRefs: []string{}, IntentionRefs: []string{},
		EvidenceRefs: []string{"reflection-fact-1"}, ContextReferences: map[string]ContextReference{decisionMemoryRef: projectionBeforeOutcome.ReferenceIndex.ByRef[decisionMemoryRef]},
		Revision: 1, OccurredAt: time.Now().UTC(),
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,2,1) ON CONFLICT(fluctlight_id) DO UPDATE SET next_sequence=GREATEST(cognition_inbox_heads.next_sequence,2),last_processed_sequence=GREATEST(cognition_inbox_heads.last_processed_sequence,1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	err = withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		if err := persistActionOutcomesTx(ctx, tx, []ActionOutcome{outcome}); err != nil {
			return err
		}
		_, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", map[string]any{
			"source_fact_id": "reflection-fact-1", "outcomes": []ActionOutcome{outcome}, "influences": decisionInfluenceMaps(influences),
		}, "reflection-memory-outcome")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.ProcessReflection(ctx, fluctlightID, "reflection-correlation-2")
	if err != nil || stringValue(second["status"]) != "applied" || providerCalls.Load() != 2 || citedMemoryRef == "" {
		t.Fatalf("second Reflection=%#v err=%v calls=%d ref=%q", second, err, providerCalls.Load(), citedMemoryRef)
	}
	var content string
	if err := repository.Pool().QueryRow(ctx, `SELECT content,revision FROM public.memories WHERE id=$1`, memoryID).Scan(&content, &revision); err != nil || revision != 1 || content != "用户明确偏好靠窗、安静且光线柔和的位置" {
		t.Fatalf("revised Memory content=%q revision=%d err=%v", content, revision, err)
	}
	var governanceCount, watermark int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memory_governance WHERE proposal_id IS NOT NULL`).Scan(&governanceCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT watermark FROM public.cognition_reflection_windows WHERE fluctlight_id=$1`, fluctlightID).Scan(&watermark); err != nil {
		t.Fatal(err)
	}
	if governanceCount != 2 || watermark != 2 {
		t.Fatalf("Reflection authority governance=%d watermark=%d", governanceCount, watermark)
	}
	projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		SourceFactID: "reflection-next-projection", MemoryOperation: MemoryForReflection,
		MemoryConversationMode: MemoryConversationAllowedSet, AllowedConversationIDs: []string{"conversation-reflection"},
		MemoryCues: []MemoryQueryCue{{Kind: "test", Text: "靠窗 光线"}},
	})
	if err != nil || len(projection.Memories) != 1 || intValue(projection.Memories[0]["revision"]) != 1 || stringValue(projection.Memories[0]["ref"]) == "" || stringValue(projection.Memories[0]["ref"]) == citedMemoryRef {
		t.Fatalf("next projection Memories=%#v err=%v", projection.Memories, err)
	}
}

func TestProcessReflectionAppliesAllMemoryCandidateV2OperationsAtomically(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-reflection-all-owner"
	fluctlightID := "memory-reflection-all-fluctlight"
	endpointID := "memory-reflection-all-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://reflection-all.invalid','memory-reflection-all-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection',$1,'reflection-all-model','structured_output',4096,5,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{"personality_system":{"active_profile_id":"default"}}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	created := make([]MemoryApplyResult, 0, 6)
	for index, content := range []string{"确认目标", "修订目标", "合并主目标", "合并次目标", "替代目标", "弃用目标"} {
		semantic := &MemorySemanticInput{Type: "semantic", Content: content, Confidence: 0.8, Importance: 0.7, EmotionalSignificance: 0.2}
		command := memoryLifecycleTestCommand(MemoryCreate, fmt.Sprintf("all-seed-%d", index), semantic, nil, nil)
		command.OwnerFluctlightID, command.OwnerActorID, command.ActorID = fluctlightID, ownerID, fluctlightID
		command.RequestDigest = memoryCommandDigest(command)
		created = append(created, applyMemoryLifecycleTestCommand(t, ctx, app, command))
	}
	projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		SourceFactID: "all-operation-projection", MemoryOperation: MemoryForReflection,
		MemoryConversationMode: MemoryConversationGlobalOnly,
	})
	if err != nil || len(projection.Memories) != 6 {
		t.Fatalf("all-operation projection count=%d err=%v", len(projection.Memories), err)
	}
	refs := make(map[string]string, len(projection.Memories))
	for _, memory := range projection.Memories {
		refs[stringValue(memory["id"])] = stringValue(memory["ref"])
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES('reflection-all-fact',$1,1,'life.observation','{"summary":"窗口内事实要求治理已有记忆"}','reflection-all','reflection-all','reflection-all-fact',now(),'processed',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	semanticCandidate := func(operation, targetRef, content string) map[string]any {
		candidate := reflectionMemorySemanticCandidate(operation)
		candidate["target_ref"] = targetRef
		candidate["content"] = content
		candidate["evidence_refs"] = []any{"sequence:1", targetRef}
		return candidate
	}
	createCandidate := reflectionMemorySemanticCandidate("create")
	createCandidate["evidence_refs"] = []any{"sequence:1"}
	createCandidate["content"] = "窗口中新形成的独立事实"
	memoryCandidates := []any{
		map[string]any{"operation": "confirm", "target_ref": refs[created[0].MemoryID], "evidence_refs": []any{"sequence:1", refs[created[0].MemoryID]}, "semantic_reason": "再次确认"},
		semanticCandidate("revise", refs[created[1].MemoryID], "修订后的事实"),
		semanticCandidate("merge", refs[created[2].MemoryID], "合并后的事实"),
		semanticCandidate("supersede", refs[created[4].MemoryID], "替代后的事实"),
		map[string]any{"operation": "deprecate", "target_ref": refs[created[5].MemoryID], "evidence_refs": []any{"sequence:1", refs[created[5].MemoryID]}, "semantic_reason": "事实已过期"},
		createCandidate,
	}
	mapValue(memoryCandidates[2])["merge_refs"] = []any{refs[created[3].MemoryID]}
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		for _, ref := range refs {
			if !strings.Contains(string(body), ref) {
				t.Fatalf("Reflection request omitted target ref %q", ref)
			}
		}
		proposal := reflectionProposalV2Fixture(memoryCandidates)
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(proposal)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}
	app.Provider = &ProviderClient{DB: repository, HTTP: providerHTTP}
	result, err := app.ProcessReflection(ctx, fluctlightID, "reflection-all-correlation")
	if err != nil || intValue(mapValue(mapValue(result["memory"])["counts"])["applied"]) != 6 {
		t.Fatalf("all-operation Reflection=%#v err=%v", result, err)
	}
	want := map[string]struct {
		status   string
		revision int
		content  string
	}{
		created[0].MemoryID: {status: "active", revision: 1, content: "确认目标"},
		created[1].MemoryID: {status: "active", revision: 1, content: "修订后的事实"},
		created[2].MemoryID: {status: "active", revision: 1, content: "合并后的事实"},
		created[3].MemoryID: {status: "superseded", revision: 1, content: "合并次目标"},
		created[4].MemoryID: {status: "superseded", revision: 1, content: "替代目标"},
		created[5].MemoryID: {status: "deprecated", revision: 1, content: "弃用目标"},
	}
	for memoryID, expected := range want {
		var status, content string
		var revision int
		if err := repository.Pool().QueryRow(ctx, `SELECT status,revision,content FROM public.memories WHERE id=$1`, memoryID).Scan(&status, &revision, &content); err != nil || status != expected.status || revision != expected.revision || content != expected.content {
			t.Fatalf("Memory %s status=%q revision=%d content=%q err=%v", memoryID, status, revision, content, err)
		}
	}
	var proposalGovernance, watermark int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memory_governance WHERE proposal_id=$1`, stringValue(result["proposal_id"])).Scan(&proposalGovernance); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT watermark FROM public.cognition_reflection_windows WHERE fluctlight_id=$1`, fluctlightID).Scan(&watermark); err != nil {
		t.Fatal(err)
	}
	if proposalGovernance != 6 || watermark != 1 {
		t.Fatalf("all-operation governance=%d watermark=%d", proposalGovernance, watermark)
	}
}

func TestReflectionMemoryApplyRollsBackWithProposalAndWatermark(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-reflection-rollback-owner"
	fluctlightID := "memory-reflection-rollback-fluctlight"
	endpointID := "memory-reflection-rollback-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://reflection-rollback.invalid','memory-reflection-rollback-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection',$1,'reflection-rollback-model','structured_output',4096,5,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{"personality_system":{"active_profile_id":"default"}}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES('reflection-rollback-fact',$1,1,'conversation.turn','{"text":"事务必须完整回滚"}','reflection-rollback','reflection-rollback','reflection-rollback-fact',now(),'processed',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `
CREATE FUNCTION public.fail_reflection_watermark_test() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.watermark > OLD.watermark THEN
    RAISE EXCEPTION 'forced reflection watermark failure';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER fail_reflection_watermark_test BEFORE UPDATE ON public.cognition_reflection_windows
FOR EACH ROW EXECUTE FUNCTION public.fail_reflection_watermark_test();`); err != nil {
		t.Fatal(err)
	}
	candidate := reflectionMemorySemanticCandidate("create")
	candidate["evidence_refs"] = []any{"sequence:1"}
	proposal := reflectionProposalV2Fixture([]any{candidate})
	var providerCalls atomic.Int32
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		responseProposal := proposal
		if providerCalls.Add(1) == 1 {
			responseProposal = cloneMap(proposal)
			responseProposal["memory_candidates"] = []any{map[string]any{}}
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(responseProposal)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: providerHTTP}}
	if _, err := app.ProcessReflection(ctx, fluctlightID, "reflection-malformed-correlation"); err == nil || !strings.Contains(err.Error(), "reflection_memory_candidate_0_invalid") {
		t.Fatalf("malformed Reflection err=%v", err)
	}
	var malformedWatermark int
	var malformedStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT watermark,status FROM public.cognition_reflection_windows WHERE fluctlight_id=$1`, fluctlightID).Scan(&malformedWatermark, &malformedStatus); err != nil || malformedWatermark != 0 || malformedStatus != "idle" {
		t.Fatalf("malformed Reflection advanced window watermark=%d status=%q err=%v", malformedWatermark, malformedStatus, err)
	}
	if _, err := app.ProcessReflection(ctx, fluctlightID, "reflection-rollback-correlation"); err == nil {
		t.Fatal("forced watermark failure did not roll back Reflection")
	}
	var memories, revisions, governance, proposals, embeddingIntents, memoryOutbox, watermark int
	var windowStatus string
	queries := []struct {
		destination *int
		query       string
	}{
		{&memories, `SELECT count(*) FROM public.memories WHERE owner_fluctlight_id='memory-reflection-rollback-fluctlight'`},
		{&revisions, `SELECT count(*) FROM public.memory_revisions r JOIN public.memories m ON m.id=r.memory_id WHERE m.owner_fluctlight_id='memory-reflection-rollback-fluctlight'`},
		{&governance, `SELECT count(*) FROM public.memory_governance WHERE fluctlight_id='memory-reflection-rollback-fluctlight'`},
		{&proposals, `SELECT count(*) FROM public.cognition_reflection_proposals WHERE fluctlight_id='memory-reflection-rollback-fluctlight'`},
		{&embeddingIntents, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='memory.embedding' AND payload->>'memory_id' IS NOT NULL`},
		{&memoryOutbox, `SELECT count(*) FROM public.platform_outbox_events WHERE aggregate_type='memory'`},
	}
	for _, query := range queries {
		if err := repository.Pool().QueryRow(ctx, query.query).Scan(query.destination); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT watermark,status FROM public.cognition_reflection_windows WHERE fluctlight_id=$1`, fluctlightID).Scan(&watermark, &windowStatus); err != nil {
		t.Fatal(err)
	}
	if memories != 0 || revisions != 0 || governance != 0 || proposals != 0 || embeddingIntents != 0 || memoryOutbox != 0 || watermark != 0 || windowStatus != "idle" {
		t.Fatalf("rollback leaked memories=%d revisions=%d governance=%d proposals=%d embedding_intents=%d outbox=%d watermark=%d status=%q", memories, revisions, governance, proposals, embeddingIntents, memoryOutbox, watermark, windowStatus)
	}
}

func embeddingHTTPResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}
}

func TestMemoryLifecycleStaticGuardKeepsOneProductionSQLAuthority(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		if name != "memory_lifecycle.go" && (strings.Contains(text, "INSERT INTO public.memories") || strings.Contains(text, "UPDATE public.memories SET") || strings.Contains(text, "INSERT INTO public.memory_revisions")) {
			t.Fatalf("Memory lifecycle SQL authority leaked into %s", name)
		}
		if name != "memory_lifecycle.go" && name != "memory_embedding.go" && (strings.Contains(text, "INSERT INTO public.memory_embeddings") || strings.Contains(text, "UPDATE public.memory_embeddings SET")) {
			t.Fatalf("Memory embedding SQL authority leaked into %s", name)
		}
	}
}

func TestMemoryCapabilityRejectsNonTransactionalExecution(t *testing.T) {
	invocation := CapabilityInvocation{CallID: "memory-non-tx", CapabilityName: "memory_event"}
	result, err := (&App{}).applyMemoryCapability(context.Background(), invocation, CapabilityContext{})
	if err == nil || result.Status != "failed" || result.ErrorCode != "caller_transaction_required" || result.Retryable {
		t.Fatalf("non-transactional Memory result=%#v err=%v", result, err)
	}
}

func TestValidatePreparedMemoryMutationRequiresOperationSpecificShapeAndDigest(t *testing.T) {
	semantic := &MemorySemanticInput{Type: "semantic", Content: "remember this", Confidence: 0.9, Importance: 0.8, EmotionalSignificance: 0.2}
	create := memoryLifecycleTestCommand(MemoryCreate, "create", semantic, nil, nil)
	if err := validatePreparedMemoryMutation(create); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*PreparedMemoryMutation){
		"raw working type": func(value *PreparedMemoryMutation) { value.Semantic.Type = "working" },
		"missing target":   func(value *PreparedMemoryMutation) { value.Operation = MemoryRevise },
		"forged digest":    func(value *PreparedMemoryMutation) { value.RequestDigest = "forged" },
		"empty evidence":   func(value *PreparedMemoryMutation) { value.EvidenceRefs = nil },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := create
			semanticCopy := *semantic
			candidate.Semantic = &semanticCopy
			mutate(&candidate)
			if name != "forged digest" {
				candidate.RequestDigest = memoryCommandDigest(candidate)
			}
			if err := validatePreparedMemoryMutation(candidate); err == nil {
				t.Fatal("invalid Memory command was accepted")
			}
		})
	}
}

func applyMemoryLifecycleTestCommand(t *testing.T, ctx context.Context, app *App, command PreparedMemoryMutation) MemoryApplyResult {
	t.Helper()
	var result MemoryApplyResult
	if err := withTransaction(ctx, app.DB.Pool(), func(tx pgx.Tx) error {
		var err error
		result, err = app.applyMemoryCommandTx(ctx, tx, command)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestMemoryLifecycleCreateConfirmReviseMergeSupersedeDeprecate(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-life-owner"
	fluctlightID := "memory-life-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	semanticA := &MemorySemanticInput{Type: "semantic", Content: "用户喜欢安静的咖啡馆", Confidence: 0.9, Importance: 0.8, EmotionalSignificance: 0.3}
	createA := memoryLifecycleTestCommand(MemoryCreate, "create-a", semanticA, nil, nil)
	createdA := applyMemoryLifecycleTestCommand(t, ctx, app, createA)
	if createdA.Disposition != "applied" || createdA.Revision != 0 {
		t.Fatalf("create A=%#v", createdA)
	}
	replayed := applyMemoryLifecycleTestCommand(t, ctx, app, createA)
	if !replayed.Replayed || replayed.MemoryID != createdA.MemoryID {
		t.Fatalf("create replay=%#v", replayed)
	}
	conflict := createA
	semanticConflict := *semanticA
	semanticConflict.Content = "different payload"
	conflict.Semantic = &semanticConflict
	conflict.RequestDigest = memoryCommandDigest(conflict)
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.applyMemoryCommandTx(ctx, tx, conflict)
		return err
	}); err == nil || err.Error() != "memory_command_idempotency_conflict" {
		t.Fatalf("idempotency conflict err=%v", err)
	}
	duplicate := memoryLifecycleTestCommand(MemoryCreate, "duplicate-a", semanticA, nil, nil)
	duplicateResult := applyMemoryLifecycleTestCommand(t, ctx, app, duplicate)
	if duplicateResult.Disposition != "no_change" || duplicateResult.MemoryID != createdA.MemoryID {
		t.Fatalf("exact duplicate=%#v", duplicateResult)
	}

	confirm := memoryLifecycleTestCommand(MemoryConfirm, "confirm-a", nil, &MemoryTarget{Ref: "memory-ref-a-0", MemoryID: createdA.MemoryID, ExpectedRevision: 0}, nil)
	confirmed := applyMemoryLifecycleTestCommand(t, ctx, app, confirm)
	if confirmed.Revision != 1 || confirmed.ReasonCode != "confirmed" {
		t.Fatalf("confirm=%#v", confirmed)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.memory_embeddings(id,memory_id,memory_revision,provider_endpoint_id,model_id,dimensions,embedding,status,error_code) VALUES('embedding-a',$1,1,'test-endpoint','test-model',0,'[]','failed','test')`, createdA.MemoryID); err != nil {
		t.Fatal(err)
	}
	semanticRevised := &MemorySemanticInput{Type: "semantic", Content: "用户偏好安静且靠窗的咖啡馆", Confidence: 0.95, Importance: 0.85, EmotionalSignificance: 0.35}
	revise := memoryLifecycleTestCommand(MemoryRevise, "revise-a", semanticRevised, &MemoryTarget{Ref: "memory-ref-a-1", MemoryID: createdA.MemoryID, ExpectedRevision: 1}, nil)
	revised := applyMemoryLifecycleTestCommand(t, ctx, app, revise)
	if revised.Revision != 2 {
		t.Fatalf("revise=%#v", revised)
	}
	var embeddingStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.memory_embeddings WHERE id='embedding-a'`).Scan(&embeddingStatus); err != nil || embeddingStatus != "stale" {
		t.Fatalf("revised embedding status=%q err=%v", embeddingStatus, err)
	}

	semanticB := &MemorySemanticInput{Type: "episodic", Content: "上周在河边咖啡馆聊天", Confidence: 0.8, Importance: 0.6, EmotionalSignificance: 0.4}
	createdB := applyMemoryLifecycleTestCommand(t, ctx, app, memoryLifecycleTestCommand(MemoryCreate, "create-b", semanticB, nil, nil))
	merge := memoryLifecycleTestCommand(MemoryMerge, "merge-ab", semanticRevised, &MemoryTarget{Ref: "memory-ref-a-2", MemoryID: createdA.MemoryID, ExpectedRevision: 2}, []MemoryTarget{{Ref: "memory-ref-b-0", MemoryID: createdB.MemoryID, ExpectedRevision: 0}})
	merged := applyMemoryLifecycleTestCommand(t, ctx, app, merge)
	if merged.Revision != 3 || len(merged.RelatedMemoryIDs) != 1 || merged.ResultingRevisions[createdB.MemoryID] != 1 {
		t.Fatalf("merge=%#v", merged)
	}

	replacementSemantic := &MemorySemanticInput{Type: "semantic", Content: "用户现在更偏好安静的茶馆", Confidence: 0.9, Importance: 0.8, EmotionalSignificance: 0.2}
	supersede := memoryLifecycleTestCommand(MemorySupersede, "supersede-a", replacementSemantic, &MemoryTarget{Ref: "memory-ref-a-3", MemoryID: createdA.MemoryID, ExpectedRevision: 3}, nil)
	superseded := applyMemoryLifecycleTestCommand(t, ctx, app, supersede)
	if superseded.MemoryID == createdA.MemoryID || superseded.Revision != 0 || superseded.ResultingRevisions[createdA.MemoryID] != 4 {
		t.Fatalf("supersede=%#v", superseded)
	}

	deprecate := memoryLifecycleTestCommand(MemoryDeprecate, "deprecate-replacement", nil, &MemoryTarget{Ref: "memory-ref-replacement-0", MemoryID: superseded.MemoryID, ExpectedRevision: 0}, nil)
	rollbackErr := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		if _, err := app.applyMemoryCommandTx(ctx, tx, deprecate); err != nil {
			return err
		}
		return errors.New("force_memory_lifecycle_rollback")
	})
	if rollbackErr == nil {
		t.Fatal("forced lifecycle rollback unexpectedly committed")
	}
	var replacementStatus string
	var replacementRevision int
	if err := repository.Pool().QueryRow(ctx, `SELECT status,revision FROM public.memories WHERE id=$1`, superseded.MemoryID).Scan(&replacementStatus, &replacementRevision); err != nil || replacementStatus != "active" || replacementRevision != 0 {
		t.Fatalf("forced rollback leaked: status=%s revision=%d err=%v", replacementStatus, replacementRevision, err)
	}
	deprecated := applyMemoryLifecycleTestCommand(t, ctx, app, deprecate)
	if deprecated.Status != "deprecated" || deprecated.Revision != 1 {
		t.Fatalf("deprecate=%#v", deprecated)
	}

	var memoryCount, revisionCount, governanceCount, embeddingIntentCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memories WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&memoryCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memory_revisions r JOIN public.memories m ON m.id=r.memory_id WHERE m.owner_fluctlight_id=$1`, fluctlightID).Scan(&revisionCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memory_governance WHERE fluctlight_id=$1`, fluctlightID).Scan(&governanceCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='memory.embedding' AND payload->>'memory_id' IN ($1,$2,$3)`, createdA.MemoryID, createdB.MemoryID, superseded.MemoryID).Scan(&embeddingIntentCount); err != nil {
		t.Fatal(err)
	}
	if memoryCount != 3 || revisionCount != 9 || governanceCount != 8 || embeddingIntentCount != 6 {
		t.Fatalf("lifecycle counts memories=%d revisions=%d governance=%d embedding_intents=%d", memoryCount, revisionCount, governanceCount, embeddingIntentCount)
	}
	var statusA, statusB, replacementLink string
	if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(superseded_by_memory_id,'') FROM public.memories WHERE id=$1`, createdA.MemoryID).Scan(&statusA, &replacementLink); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.memories WHERE id=$1`, createdB.MemoryID).Scan(&statusB); err != nil {
		t.Fatal(err)
	}
	if statusA != "superseded" || statusB != "superseded" || replacementLink != superseded.MemoryID {
		t.Fatalf("final lineage A=%s B=%s replacement=%s", statusA, statusB, replacementLink)
	}
	var currentEmbeddingDuplicates int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM (SELECT memory_id,memory_revision,model_id FROM public.memory_embeddings WHERE status<>'stale' GROUP BY memory_id,memory_revision,model_id HAVING count(*)>1) duplicates`).Scan(&currentEmbeddingDuplicates); err != nil || currentEmbeddingDuplicates != 0 {
		t.Fatalf("embedding tuple duplicates=%d err=%v", currentEmbeddingDuplicates, err)
	}
}

func TestMemoryEventFreezesRuntimePlanAndSharesCallerTransaction(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-event-owner"
	fluctlightID := "memory-event-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	capability := memoryEventCapability{service: app}
	registry := mustCapabilityRegistry(capability)
	resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{
		SlotCorePersona: func(context.Context, ContextRequest) (any, error) { return map[string]any{}, nil },
		SlotMemoryScope: func(context.Context, ContextRequest) (any, error) {
			return map[string]any{"owner_actor_id": ownerID, "active_profile_id": "default", "viewer_actor_ids": []any{ownerID}, "conversation_mode": "global_only", "memories": []any{}}, nil
		},
	})
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Capabilities, app.ContextResolver, app.Runtime = registry, resolver, runtime
	arguments := `{"type":"semantic","content":"用户偏好安静的工作环境","confidence":0.9,"importance":0.8}`
	invocation := CapabilityInvocation{
		CallID: "memory-event-call", CapabilityName: "memory_event", SchemaVersion: CapabilityInvocationSchemaVersion,
		Arguments: []byte(arguments), SourceFactID: "memory-event-source", ProviderRequestID: "memory-event-provider",
		Metadata: InvocationMetadata{FluctlightID: fluctlightID, Surface: CapabilitySurfaceConversation},
	}
	prepared, _, err := runtime.Prepare(ctx, invocation)
	if err != nil {
		t.Fatal(err)
	}
	if string(prepared.Arguments) != arguments {
		t.Fatalf("Provider arguments were mutated: %s", prepared.Arguments)
	}
	rawPlan, found, err := capabilityPreparedData(prepared, "memory_plan")
	if err != nil || !found {
		t.Fatalf("frozen Memory plan missing: %#v err=%v", rawPlan, err)
	}
	var plan PreparedMemoryMutation
	if err := jsonUnmarshal(jsonBytes(rawPlan), &plan); err != nil || plan.Operation != MemoryCreate || len(plan.EvidenceRefs) != 1 || plan.EvidenceRefs[0] != invocation.SourceFactID || plan.OwnerActorID != ownerID {
		t.Fatalf("prepared Memory plan=%#v err=%v", plan, err)
	}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.ExecuteTransactional(ctx, tx, prepared)
	if err != nil || result.Status != "completed" {
		_ = tx.Rollback(ctx)
		t.Fatalf("transactional Memory result=%#v err=%v", result, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for authority, query := range map[string]string{
		"memory":     `SELECT count(*) FROM public.memories WHERE owner_fluctlight_id=$1`,
		"revision":   `SELECT count(*) FROM public.memory_revisions r JOIN public.memories m ON m.id=r.memory_id WHERE m.owner_fluctlight_id=$1`,
		"governance": `SELECT count(*) FROM public.memory_governance WHERE fluctlight_id=$1`,
		"workflow":   `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='memory.embedding' AND payload->>'memory_id' IS NOT NULL AND $1<>''`,
		"outbox":     `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1`,
	} {
		var count int
		if err := repository.Pool().QueryRow(ctx, query, fluctlightID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s escaped rollback: count=%d err=%v", authority, count, err)
		}
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		var executeErr error
		result, executeErr = runtime.ExecuteTransactional(ctx, tx, prepared)
		return executeErr
	}); err != nil {
		t.Fatal(err)
	}
	if err := capability.Definition().ValidateOutput(result.Output); err != nil {
		t.Fatal(err)
	}
	var replay CapabilityResult
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		var executeErr error
		replay, executeErr = runtime.ExecuteTransactional(ctx, tx, prepared)
		return executeErr
	}); err != nil {
		t.Fatal(err)
	}
	if replay.Status != "completed" || !boolValueForTest(mapValue(replay.Output)["replayed"]) {
		t.Fatalf("Memory capability replay=%#v", replay)
	}
	var memoryCount, revisionCount, governanceCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memories WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&memoryCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memory_revisions`).Scan(&revisionCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memory_governance WHERE fluctlight_id=$1`, fluctlightID).Scan(&governanceCount); err != nil {
		t.Fatal(err)
	}
	if memoryCount != 1 || revisionCount != 1 || governanceCount != 1 {
		t.Fatalf("Memory replay duplicated authority: memories=%d revisions=%d governance=%d", memoryCount, revisionCount, governanceCount)
	}
}
