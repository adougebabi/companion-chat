package core

import (
	"crypto/md5"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestMemoryProvenanceKeepsOtherSourceAndRejectsLastSourceLoss(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "memory-life-owner", "memory-life-fluctlight"
	for _, statement := range []string{
		`INSERT INTO public.actors(id,actor_type,status) VALUES('memory-life-owner','human','active'),('memory-life-fluctlight','fluctlight','active')`,
		`INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('memory-life-fluctlight','memory-life-owner','blank_slate','active','{}','{}','{}','{}','{}','{}')`,
		`INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES('memory-life-fluctlight',3,2)`,
		`INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES('memory-source-a','memory-life-fluctlight',1,'conversation.turn','{"text":"用户喜欢安静"}','memory-source-a','memory-source-a','memory-source-a',now(),'processed')`,
		`INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES('memory-source-b','memory-life-fluctlight',2,'conversation.turn','{"text":"用户再次确认喜欢安静"}','memory-source-b','memory-source-b','memory-source-b',now(),'processed')`,
	} {
		if _, err := repository.Pool().Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{DB: repository}
	command := memoryLifecycleTestCommand(MemoryCreate, "two-source", &MemorySemanticInput{Type: "semantic", Content: "用户喜欢安静", Confidence: 0.9, Importance: 0.9}, nil, nil)
	command.EvidenceRefs = []string{"sequence:1", "sequence:2"}
	command.SourceFactID = "memory-source-a"
	command.RequestDigest = memoryCommandDigest(command)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	var provenance string
	if err := repository.Pool().QueryRow(ctx, `SELECT provenance_status FROM public.memories WHERE id=$1`, created.MemoryID).Scan(&provenance); err != nil || provenance != "verified" {
		t.Fatalf("new memory provenance=%q err=%v", provenance, err)
	}
	plan, err := buildMemoryQueryPlan(MemoryForConversation, []string{ownerID}, MemoryConversationGlobalOnly, "", nil, "default", []MemoryQueryCue{{Kind: "user", Text: "喜欢安静"}}, 6, 2400)
	if err != nil {
		t.Fatal(err)
	}
	assertCount := func(expected int) {
		t.Helper()
		result, err := app.retrieveMemoryWithPlan(ctx, ownerID, fluctlightID, plan)
		if err != nil || len(result.Items) != expected {
			t.Fatalf("effective source count expected=%d memories=%#v err=%v", expected, result.Items, err)
		}
	}
	assertCount(1)
	if _, err := repository.Pool().Exec(ctx, `DELETE FROM public.cognition_inbox WHERE id='memory-source-a'`); err != nil {
		t.Fatal(err)
	}
	assertCount(1)
	remaining, err := app.retrieveMemoryWithPlan(ctx, ownerID, fluctlightID, plan)
	if err != nil || stringValue(remaining.Items[0]["provenance_status"]) != "partial" || jsonString(remaining.Items[0]["effective_source_refs"]) != `["sequence:2"]` {
		t.Fatalf("remaining evidence was misreported: %#v err=%v", remaining.Items, err)
	}
	var frozenFingerprint string
	if err := repository.Pool().QueryRow(ctx, `SELECT public.cognition_source_fingerprint(payload) FROM public.cognition_inbox WHERE id='memory-source-b'`).Scan(&frozenFingerprint); err != nil {
		t.Fatal(err)
	}
	beforeSourceCorrection, err := app.readCurrentFactsRevision(ctx, fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload='{"text":"纠正：不喜欢安静"}' WHERE id='memory-source-b'`); err != nil {
		t.Fatal(err)
	}
	afterSourceCorrection, err := app.readCurrentFactsRevision(ctx, fluctlightID)
	if err != nil || afterSourceCorrection == beforeSourceCorrection {
		t.Fatalf("source correction failed to advance context generation: before=%q after=%q err=%v", beforeSourceCorrection, afterSourceCorrection, err)
	}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.requireCurrentFactsRevisionTx(ctx, tx, fluctlightID, beforeSourceCorrection); !errors.Is(err, ErrCurrentFactsStale) {
		_ = tx.Rollback(ctx)
		t.Fatalf("source correction did not invalidate stale settlement: %v", err)
	}
	_ = tx.Rollback(ctx)
	assertCount(0)
	staleMerge := memoryLifecycleTestCommand(MemoryCreate, "stale-merge", &MemorySemanticInput{Type: "semantic", Content: "后台旧结论", Confidence: 0.9, Importance: 0.8}, nil, nil)
	staleMerge.EvidenceRefs = []string{"sequence:2"}
	staleMerge.ExpectedEvidenceFingerprints = map[string]string{"sequence:2": frozenFingerprint}
	staleMerge.SourceFactID = "memory-source-b"
	staleMerge.RequestDigest = memoryCommandDigest(staleMerge)
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.applyMemoryCommandTx(ctx, tx, staleMerge)
		return err
	}); err == nil || err.Error() != "memory_source_version_stale" {
		t.Fatalf("old background source republished: %v", err)
	}

	late := memoryLifecycleTestCommand(MemoryCreate, "late-source", &MemorySemanticInput{Type: "semantic", Content: "旧来源后台再写", Confidence: 0.9, Importance: 0.8}, nil, nil)
	late.EvidenceRefs = []string{"sequence:1"}
	late.SourceFactID = "memory-source-a"
	late.OccurredAt = time.Now().UTC()
	late.RequestDigest = memoryCommandDigest(late)
	lateResult := applyMemoryLifecycleTestCommand(t, ctx, app, late)
	if err := repository.Pool().QueryRow(ctx, `SELECT provenance_status FROM public.memories WHERE id=$1`, lateResult.MemoryID).Scan(&provenance); err != nil || provenance != "pending" {
		t.Fatalf("late unsupported memory became valid: status=%q err=%v", provenance, err)
	}
}

func TestOwnerMemoryCorrectionInvalidatesOnlyAffectedEpisodeAndOldWorkerResult(t *testing.T) {
	ctx, repository, ownerID, fluctlightID, conversationID := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repository, ownerID, fluctlightID, conversationID, 1, 4)
	messages, err := readConversationSummaryMessages(ctx, repository.Pool(), conversationID, 1, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	refs, digest := conversationSummarySourceRefs(messages), conversationSummarySourceDigest(messages)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('test-provider','openai_compatible','http://provider.invalid','test-secret','ready',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key)
VALUES('episode-affected',$1,$2,1,2,$3,$4,'旧错误结论','active',1,'test-provider','test-model','test-request','test-prompt','test-schema','test-policy','test-digest-a','test-idempotency-a')`, fluctlightID, conversationID, jsonBytes(refs[:2]), conversationSummarySourceDigest(messages[:2])); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key)
VALUES('episode-unaffected',$1,$2,3,4,$3,$4,'其他经历','active',1,'test-provider','test-model','test-request-b','test-prompt','test-schema','test-policy','test-digest-b','test-idempotency-b')`, fluctlightID, conversationID, jsonBytes(refs[2:]), conversationSummarySourceDigest(messages[2:])); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	create := PreparedMemoryMutation{
		SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate,
		OwnerFluctlightID: fluctlightID, OwnerActorID: ownerID, ActorID: fluctlightID,
		ConversationID: conversationID, ActorRefs: []string{}, EventRefs: []string{},
		EvidenceRefs: []string{refs[0]}, Visibility: "private", OccurredAt: time.Now().UTC(),
		SourceFactID: "summary-message-1", SemanticReason: "记录原话", IdempotencyKey: "episode-memory-create",
		Semantic: &MemorySemanticInput{Type: "semantic", Content: "旧错误结论", Confidence: 0.8, Importance: 0.8},
	}
	create.RequestDigest = memoryCommandDigest(create)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, create)
	revise := create
	revise.Operation, revise.ActorID, revise.IdempotencyKey, revise.SourceFactID = MemoryRevise, ownerID, "episode-memory-correct", "owner-correction"
	revise.Target = &MemoryTarget{MemoryID: created.MemoryID, ExpectedRevision: created.Revision}
	revise.EvidenceRefs = []string{"owner-correction"}
	revise.Semantic = &MemorySemanticInput{Type: "semantic", Content: "纠正后的结论", Confidence: 0.95, Importance: 0.8}
	revise.SemanticReason = "用户明确纠正"
	revise.RequestDigest = memoryCommandDigest(revise)
	applyMemoryLifecycleTestCommand(t, ctx, app, revise)
	var affected, unaffected string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.conversation_summaries WHERE id='episode-affected'`).Scan(&affected); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.conversation_summaries WHERE id='episode-unaffected'`).Scan(&unaffected); err != nil {
		t.Fatal(err)
	}
	if affected != "invalidated" || unaffected != "active" {
		t.Fatalf("episode invalidation affected=%s unaffected=%s", affected, unaffected)
	}
	visible, err := app.retrieveConversationSummaries(ctx, ConversationSummaryQuery{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, Limit: 4, MaxRunes: conversationSummaryDefaultBudget})
	if err != nil || len(visible.Items) != 1 || stringValue(visible.Items[0]["summary"]) != "其他经历" {
		t.Fatalf("corrected old episode still visible: %#v err=%v", visible, err)
	}
	work := conversationSummaryWork{FluctlightID: fluctlightID, ConversationID: conversationID, FromSequence: 1, ToSequence: 2, SourceMessageRefs: refs[:2], SourceDigest: conversationSummarySourceDigest(messages[:2]), Messages: messages[:2]}
	if _, err := app.settleConversationSummary(ctx, work, conversationSummaryProviderResponse{SchemaVersion: conversationSummarySchemaVersion, Summary: "后台旧结论"}, providerAssignment{EndpointID: "test-provider", ModelID: "test-model"}, "late-provider", digest); err == nil || err.Error() != "conversation_summary_source_invalidated" {
		t.Fatalf("late summary result resurrected correction: %v", err)
	}
}

func TestOpaqueMemoryLineageRejectsSelfAndInvalidatesTransitiveDescendants(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "memory-life-owner", "memory-life-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	rootCommand := memoryLifecycleTestCommand(MemoryCreate, "lineage-root", &MemorySemanticInput{Type: "semantic", Content: "用户明确确认的源事实", Confidence: 0.9, Importance: 0.8}, nil, nil)
	rootCommand.ActorID = ownerID
	rootCommand.RequestDigest = memoryCommandDigest(rootCommand)
	root := applyMemoryLifecycleTestCommand(t, ctx, app, rootCommand)
	createChild := func(key, content, sourceID string) MemoryApplyResult {
		t.Helper()
		source, err := app.readOwnedMemoryAuthorityRow(ctx, ownerID, sourceID)
		if err != nil {
			t.Fatal(err)
		}
		ref := "memory:ctx_" + stableDigest(key+":"+sourceID)
		command := memoryLifecycleTestCommand(MemoryCreate, key, &MemorySemanticInput{Type: "semantic", Content: content, Confidence: 0.8, Importance: 0.7}, nil, nil)
		command.EvidenceRefs = []string{ref}
		command.FrozenEvidenceSources = map[string]FrozenMemoryEvidenceSource{ref: {Kind: "memory", ID: source.ID, Revision: source.Revision, Fingerprint: source.RequestDigest}}
		command.RequestDigest = memoryCommandDigest(command)
		return applyMemoryLifecycleTestCommand(t, ctx, app, command)
	}
	child := createChild("lineage-child", "从源事实整理的一层", root.MemoryID)
	grandchild := createChild("lineage-grandchild", "从一层再整理的二层", child.MemoryID)
	plan, err := buildMemoryQueryPlan(MemoryForConversation, []string{ownerID}, MemoryConversationGlobalOnly, "", nil, "default", nil, 12, 2400)
	if err != nil {
		t.Fatal(err)
	}
	find := func(id string) bool {
		t.Helper()
		result, err := app.retrieveMemoryWithPlan(ctx, ownerID, fluctlightID, plan)
		if err != nil {
			t.Fatal(err)
		}
		for _, memory := range result.Items {
			if stringValue(memory["id"]) == id {
				return true
			}
		}
		return false
	}
	if !find(child.MemoryID) || !find(grandchild.MemoryID) {
		t.Fatal("valid transitive lineage was not retrievable")
	}
	selfSource, err := app.readOwnedMemoryAuthorityRow(ctx, ownerID, child.MemoryID)
	if err != nil {
		t.Fatal(err)
	}
	selfRef := "memory:ctx_" + stableDigest("self:"+child.MemoryID)
	self := memoryLifecycleTestCommand(MemoryRevise, "lineage-self", &MemorySemanticInput{Type: "semantic", Content: "自我强化的旧结论", Confidence: 0.9, Importance: 0.7}, &MemoryTarget{MemoryID: child.MemoryID, ExpectedRevision: child.Revision}, nil)
	self.EvidenceRefs = []string{selfRef}
	self.ProposalID = ""
	self.FrozenEvidenceSources = map[string]FrozenMemoryEvidenceSource{selfRef: {Kind: "memory", ID: selfSource.ID, Revision: selfSource.Revision, Fingerprint: selfSource.RequestDigest}}
	self.RequestDigest = memoryCommandDigest(self)
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.applyMemoryCommandTx(ctx, tx, self)
		return err
	}); err == nil || err.Error() != "memory_source_cycle" {
		t.Fatalf("self citation strengthened a Memory: %v", err)
	}
	corrected := memoryLifecycleTestCommand(MemoryRevise, "lineage-root-correction", &MemorySemanticInput{Type: "semantic", Content: "用户纠正后的源事实", Confidence: 0.95, Importance: 0.8}, &MemoryTarget{MemoryID: root.MemoryID, ExpectedRevision: root.Revision}, nil)
	corrected.ActorID = ownerID
	corrected.ProposalID = ""
	corrected.RequestDigest = memoryCommandDigest(corrected)
	applyMemoryLifecycleTestCommand(t, ctx, app, corrected)
	if find(child.MemoryID) || find(grandchild.MemoryID) {
		t.Fatal("old derived Memory survived a corrected root source")
	}
}

func TestOpaqueOutcomeEvidenceRejectsLateRevision(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "memory-life-owner", "memory-life-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_action_outcomes(id,fluctlight_id,action_id,call_id,capability_name,status,success_boundary,expected,observed,goal_refs,intention_refs,evidence_refs,context_references,revision,request_digest,occurred_at) VALUES('memory-source-outcome',$1,'memory-source-action','memory-source-call','life.activity.advance','completed','confirmed_result','{}','{"item":"boots"}','[]','[]','[]','{}',1,$2,now())`, fluctlightID, digest); err != nil {
		t.Fatal(err)
	}
	ref := "outcome:ctx_" + stableDigest("memory-source-outcome")
	fingerprint := fmt.Sprintf("%x", md5.Sum([]byte(digest+":1")))
	command := memoryLifecycleTestCommand(MemoryCreate, "outcome-source", &MemorySemanticInput{Type: "episodic", Content: "已获得靴子", Confidence: 0.9, Importance: 0.8}, nil, nil)
	command.EvidenceRefs = []string{ref}
	command.FrozenEvidenceSources = map[string]FrozenMemoryEvidenceSource{ref: {Kind: "outcome", ID: "memory-source-outcome", Revision: 1, Fingerprint: fingerprint}}
	command.RequestDigest = memoryCommandDigest(command)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.cognition_action_outcomes SET revision=2,request_digest=$2 WHERE id=$1`, "memory-source-outcome", strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.applyMemoryCommandTx(ctx, tx, command)
		return err
	}); err == nil || err.Error() != "memory_source_version_stale" {
		t.Fatalf("late outcome revision was accepted: %v", err)
	}
}

func TestCognitionSourceFingerprintIgnoresSettlementMetadata(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('source-owner','human','active'),('source-fluctlight','fluctlight','active')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('source-fluctlight','source-owner','blank_slate','active','{}','{}','{}','{}','{}','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES('source-fluctlight',2,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES('source-fact','source-fluctlight',1,'conversation.turn','{"text":"原话"}','source-fact','source-fact','source-fact',now(),'claimed')`); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	var before, afterMetadata, afterCorrection string
	if err := repository.Pool().QueryRow(ctx, `SELECT public.cognition_source_fingerprint(payload) FROM public.cognition_inbox WHERE id='source-fact'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	firstGeneration, err := app.readCurrentFactsRevision(ctx, "source-fluctlight")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=jsonb_set(payload,'{agent_result}','{"status":"processed"}',true) WHERE id='source-fact'`); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT public.cognition_source_fingerprint(payload) FROM public.cognition_inbox WHERE id='source-fact'`).Scan(&afterMetadata); err != nil {
		t.Fatal(err)
	}
	metadataGeneration, err := app.readCurrentFactsRevision(ctx, "source-fluctlight")
	if err != nil || before != afterMetadata || firstGeneration != metadataGeneration {
		t.Fatalf("normal settlement metadata changed source authority: before=%s after=%s generation=%s/%s err=%v", before, afterMetadata, firstGeneration, metadataGeneration, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=jsonb_set(payload,'{text}','"纠正后的原话"',true) WHERE id='source-fact'`); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT public.cognition_source_fingerprint(payload) FROM public.cognition_inbox WHERE id='source-fact'`).Scan(&afterCorrection); err != nil {
		t.Fatal(err)
	}
	correctedGeneration, err := app.readCurrentFactsRevision(ctx, "source-fluctlight")
	if err != nil || afterCorrection == before || correctedGeneration == metadataGeneration {
		t.Fatalf("source correction was not versioned: before=%s after=%s generation=%s/%s err=%v", before, afterCorrection, metadataGeneration, correctedGeneration, err)
	}
}
