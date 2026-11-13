package core

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestMemoryCorrectionInvalidatesReflectionDevelopingSelf(t *testing.T) {
	testMemoryCorrectionInvalidatesReflectionDevelopingSelf(t, false)
}

func TestMemorySummaryAncestorCorrectionInvalidatesReflectionDevelopingSelf(t *testing.T) {
	testMemoryCorrectionInvalidatesReflectionDevelopingSelf(t, true)
}

func testMemoryCorrectionInvalidatesReflectionDevelopingSelf(t *testing.T, summaryAncestor bool) {
	ctx, repo, app, owner, fid, conversation := durableReplyFixture(t, "memory-self-source")
	index := newContextReferenceIndex(ContextProjection{FluctlightID: fid, OwnerActorID: owner, ConversationID: conversation, CurrentSpeaker: map[string]any{"actor_id": owner}, PersonalityRuntime: map[string]any{"active_profile_id": "default"}})
	var sources []MemoryApplyResult
	var refs []string
	for _, key := range []string{"walking-one", "walking-two"} {
		command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: owner, ActorRefs: []string{owner}, EventRefs: []string{}, EvidenceRefs: []string{"owner:" + key}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "owner:" + key, SemanticReason: "Owner confirmed source", IdempotencyKey: key, Semantic: &MemorySemanticInput{Type: "autobiographical", Content: "角色曾表示喜欢散步 " + key, Confidence: 0.9, Importance: 0.9}}
		command.RequestDigest = memoryCommandDigest(command)
		created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
		sources = append(sources, created)
		var raw []byte
		if err := repo.Pool().QueryRow(ctx, `SELECT to_jsonb(m) FROM public.memories m WHERE id=$1`, created.MemoryID).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		ref, err := index.add(ContextReferenceMemory, created.MemoryID, created.Revision, decodeObject(raw))
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
	}
	if summaryAncestor {
		seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 2)
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text=CASE sequence WHEN 1 THEN '我喜欢散步' ELSE '以前也这么说过' END WHERE conversation_id=$1`, conversation); err != nil {
			t.Fatal(err)
		}
		messages, err := readConversationSummaryMessages(ctx, repo.Pool(), conversation, 1, 2, 2)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose) VALUES('profile-summary-provider','openai_compatible','http://fixture.invalid','fixture')`); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key) VALUES('profile-source-summary',$1,$2,1,2,$3,$4,'角色在交流中说喜欢散步','active',1,'profile-summary-provider','fixture','profile-source-summary','fixture','fixture','fixture','profile-source-summary','profile-source-summary')`, fid, conversation, jsonBytes(conversationSummarySourceRefs(messages)), conversationSummarySourceDigest(messages)); err != nil {
			t.Fatal(err)
		}
		root := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ConversationID: conversation, ActorRefs: []string{}, EventRefs: []string{}, EvidenceRefs: []string{"message:summary-message-1"}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "summary-message-1", SemanticReason: "historical interpretation", IdempotencyKey: "profile-summary-root", Semantic: &MemorySemanticInput{Type: "semantic", Content: "从原话推断有散步偏好", Confidence: 0.9, Importance: 0.9}}
		root.RequestDigest = memoryCommandDigest(root)
		sources[0] = applyMemoryLifecycleTestCommand(t, ctx, app, root)
		var fingerprint string
		if err := repo.Pool().QueryRow(ctx, `SELECT public.conversation_summary_source_fingerprint('profile-source-summary')`).Scan(&fingerprint); err != nil {
			t.Fatal(err)
		}
		derived := root
		derived.IdempotencyKey = "profile-summary-derived"
		derived.EvidenceRefs = []string{"conversation_summary:profile-source-summary"}
		derived.FrozenEvidenceSources = map[string]FrozenMemoryEvidenceSource{derived.EvidenceRefs[0]: {Kind: "conversation_summary", ID: "profile-source-summary", Revision: 1, Fingerprint: fingerprint}}
		derived.RequestDigest = memoryCommandDigest(derived)
		stored := applyMemoryLifecycleTestCommand(t, ctx, app, derived)
		var raw []byte
		if err := repo.Pool().QueryRow(ctx, `SELECT to_jsonb(m) FROM public.memories m WHERE id=$1`, stored.MemoryID).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		refs[0], err = index.add(ContextReferenceMemory, stored.MemoryID, stored.Revision, decodeObject(raw))
		if err != nil {
			t.Fatal(err)
		}
	}
	evolution, err := BuildEvolutionContext(EvolutionContext{ProviderRole: "reflection", FluctlightID: fid, SourceWindow: "sequence:1-2", FromSequence: 1, ToSequence: 2, Watermark: 0, Evidence: []EvolutionEvidence{{Ref: "reflection-opportunity", Kind: "conversation", Summary: "一次反思机会", Sequence: 1, OccurredAt: time.Now().UTC()}}, ReferenceIndex: index, BaseRevisions: map[string]int{string(EvolutionDevelopingSelf): 0, string(EvolutionAffectProfile): 0}})
	if err != nil {
		t.Fatal(err)
	}
	proposal := ReflectionProposalV2{SchemaVersion: reflectionProposalV2SchemaVersion, Summary: "基于现有记忆提议表达个人偏好", DevelopingSelfCandidates: []ReflectionDevelopingSelfCandidateV2{{Operation: "create", SemanticReason: "两条经确认的来源支持该候选", Category: "preference", Claim: "我喜欢散步", Confidence: 0.9, EvidenceRefs: refs}}}
	plan, err := CompileReflectionPlan(proposal, evolution, ReflectionPolicyV2{}, app.now())
	if err != nil || len(plan.Candidates) != 1 || plan.Candidates[0].Disposition != EvolutionAccepted {
		t.Fatalf("normal reflection did not accept sourced candidate: %#v err=%v", plan, err)
	}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		if err := app.applyReflectionDevelopingSelfV2Tx(ctx, tx, fid, proposal, &plan, evolution, nil); err != nil {
			return err
		}
		hydrateReflectionChangedRefs(&plan, proposal, evolution, nil)
		next, result, err := ApplyReflectionPlan(EvolutionState{FluctlightID: fid, Watermark: 0, Revisions: cloneRevisionMap(evolution.BaseRevisions)}, plan, nil)
		if err != nil {
			return err
		}
		_, err = persistReflectionApplyTx(ctx, tx, evolution, proposal, plan, next, result, "personality:ctx_"+stableDigest(fid), "fixture-model", "reflection.v2")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	claims, err := app.listDevelopingSelfClaims(ctx, fid)
	if err != nil || len(claims) != 1 || claims[0].Claim != "我喜欢散步" {
		t.Fatalf("accepted sourced self claim absent: %#v err=%v", claims, err)
	}
	claimID := claims[0].ID
	managed, err := app.OwnerSelfClaim(ctx, owner, fid, claimID)
	if err != nil || managed["source_valid"] != true {
		t.Fatalf("valid source was not visible to Owner: %#v err=%v", managed, err)
	}
	beforeCore := ""
	if err := repo.Pool().QueryRow(ctx, `SELECT core_persona::text FROM public.fluctlights WHERE id=$1`, fid).Scan(&beforeCore); err != nil {
		t.Fatal(err)
	}
	revision := sources[0].Revision
	if _, err := app.ReviseMemory(ctx, owner, sources[0].MemoryID, "更正：先前的散步偏好推断没有依据", &revision, []any{"owner:walking-correction"}); err != nil {
		t.Fatal(err)
	}
	restarted := &App{DB: repo}
	after, err := restarted.listDevelopingSelfClaims(ctx, fid)
	if err != nil || len(after) != 0 {
		t.Fatalf("Memory correction left a derived self claim active: %#v err=%v", after, err)
	}
	managed, err = app.OwnerSelfClaim(ctx, owner, fid, claimID)
	if err != nil || managed["source_valid"] != false || managed["status"] != "active" {
		t.Fatalf("Owner lost the retained source-invalid record: %#v err=%v", managed, err)
	}
	if _, err := app.legacyOwnerSelfCommand(ctx, owner, claimID, "forget", claims[0].Revision, "先停用这条旧认识"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.legacyOwnerSelfCommand(ctx, owner, claimID, "rollback", claims[0].Revision+1, "尝试恢复旧来源"); err == nil {
		t.Fatal("Owner rollback silently rebound a changed Memory source")
	} else {
		var sourceErr *SurfaceError
		if !errors.As(err, &sourceErr) || sourceErr.Code != "self_snapshot_source_changed" {
			t.Fatalf("rollback did not reach the historical source check: %v", err)
		}
	}
	plan.Candidates[0].Disposition = EvolutionAccepted
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		return app.applyReflectionDevelopingSelfV2Tx(ctx, tx, fid, proposal, &plan, evolution, nil)
	}); err != nil {
		t.Fatal(err)
	}
	if plan.Candidates[0].Disposition != EvolutionRejected || plan.Candidates[0].ReasonCode != "reflection_memory_source_stale" {
		t.Fatalf("late reflection snapshot regained the old source: %#v", plan.Candidates[0])
	}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		return app.insertDevelopingSelfSeeds(ctx, tx, fid, []any{map[string]any{"category": "interest", "claim": "OWNER_DEFINED_SELF", "confidence": 0.9}})
	}); err != nil {
		t.Fatal(err)
	}
	live, err := restarted.listDevelopingSelfClaims(ctx, fid)
	if err != nil || len(live) != 1 || live[0].Claim != "OWNER_DEFINED_SELF" {
		t.Fatalf("independent Owner declaration was suppressed: %#v err=%v", live, err)
	}
	var core string
	var retained int
	if err := repo.Pool().QueryRow(ctx, `SELECT core_persona::text,(SELECT count(*) FROM public.fluctlight_developing_self_claims WHERE fluctlight_id=$1 AND id=$2) FROM public.fluctlights WHERE id=$1`, fid, claimID).Scan(&core, &retained); err != nil || core != beforeCore || retained != 1 {
		t.Fatalf("source retirement changed stable persona or deleted history: core_changed=%t retained=%d err=%v", core != beforeCore, retained, err)
	}
}
