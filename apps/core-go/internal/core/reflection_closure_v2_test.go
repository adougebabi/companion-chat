package core

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestReflectionV2AppliesMemoryGoalIntentionAndAffectThenReprojects(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "reflection-v2-owner", "reflection-v2-fluctlight"
	endpointID := "reflection-v2-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://reflection-v2.invalid','reflection-v2-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection',$1,'reflection-v2-model','structured_output',4096,5,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{"personality_system":{"active_profile_id":"default","profiles":[{"id":"default","personality":{"openness":0.4},"behavioral_policy":{"response_style":"concise"}}]}}','{"timezone":"Asia/Shanghai"}','{"openness":0.4}','{"response_style":"concise"}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id) VALUES($1,'default')`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,profile_id,target_actor_id,role,metrics,trend,summary,emotional_association,provenance,revision) VALUES('reflection-v2-relationship',$1,'default',$2,'{"label":"friend"}','{"trust":0.5}','stable','已有关系','{}','{}',0)`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	goal, goalRecord, err := CreateGoalAuthority(GoalAuthority{
		EntityID: "reflection-v2-goal", SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + strings.Repeat("a", 32),
		FluctlightID: fluctlightID, ProfileID: "default", DesiredOutcome: "完成旧目标", SuccessCriteria: []string{"旧标准"}, Motivation: "旧动机",
		Scope: "project", Importance: 0.7, Urgency: 0.5, Progress: 0, Status: GoalActive, Revision: 1, EvidenceRefs: []string{"foundation:goal"},
	}, []string{"foundation:goal"}, now)
	if err != nil {
		t.Fatal(err)
	}
	intention, intentionRecord, err := CreateIntentionAuthority(IntentionAuthority{
		EntityID: "reflection-v2-intention", GoalEntityID: goal.EntityID, SchemaVersion: intentionAuthoritySchemaVersion,
		Ref: "intention:ctx_" + strings.Repeat("b", 32), FluctlightID: fluctlightID, ProfileID: "default", GoalRef: goal.Ref,
		ActionIntent: "执行旧动作", ExpectedOutcome: "得到旧结果", Trigger: TypedIntentionTrigger{Type: IntentionTriggerSemantic},
		Expiration: now.Add(24 * time.Hour), Confidence: 0.8, Status: IntentionCandidate, Revision: 1, EvidenceRefs: []string{"foundation:intention"},
	}, []string{"foundation:intention"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		if _, err := persistGoalAuthorityTx(ctx, tx, nil, goal, goalRecord, "reflection-v2-goal-create"); err != nil {
			return err
		}
		_, err := persistIntentionAuthorityTx(ctx, tx, nil, intention, intentionRecord, "reflection-v2-intention-create")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,4,0)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES('reflection-v2-fact-1',$1,1,'conversation.turn','{"conversation_id":"reflection-v2-conversation","text":"我希望把这件事真正完成"}','turn-1','reflection-v2','reflection-v2-fact-1',now(),'processed',now()),('reflection-v2-fact-2',$1,2,'life.observation','{"summary":"行动已经形成明确方向","internal_id":"must-not-egress"}','fact-2','reflection-v2','reflection-v2-fact-2',now(),'processed',now()),('reflection-v2-fact-3',$1,3,'life.observation','{"summary":"连续证据支持更温和的回应方式"}','fact-3','reflection-v2','reflection-v2-fact-3',now(),'processed',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}

	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		bodyText := string(body)
		if !strings.Contains(bodyText, "我希望把这件事真正完成") || strings.Contains(bodyText, "must-not-egress") || strings.Contains(bodyText, "internal_id") {
			t.Fatalf("Reflection evidence allowlist mismatch: %s", bodyText)
		}
		goalRefs := regexp.MustCompile(`goal:ctx_[a-f0-9]{32}`).FindAllString(bodyText, -1)
		intentionRefs := regexp.MustCompile(`intention:ctx_[a-f0-9]{32}`).FindAllString(bodyText, -1)
		relationshipRefs := regexp.MustCompile(`relationship:ctx_[a-f0-9]{32}`).FindAllString(bodyText, -1)
		if len(goalRefs) == 0 || len(intentionRefs) == 0 || len(relationshipRefs) == 0 {
			t.Fatalf("Reflection request omitted Goal/Intention refs: %s", bodyText)
		}
		memory := reflectionMemorySemanticCandidate("create")
		memory["content"] = "用户希望真正完成当前事项"
		memory["evidence_refs"] = []any{"sequence:1"}
		proposal := reflectionProposalV2Fixture([]any{memory})
		proposal["summary"] = "用户目标、意图和情绪模式获得了新证据"
		proposal["goal_candidates"] = []any{map[string]any{
			"operation": "update", "target_ref": goalRefs[0], "desired_outcome": "真正完成当前事项", "success_criteria": []any{"真实行动完成", "结果被验证"},
			"motivation": "兑现明确承诺", "direction": "increase", "strength": 0.8, "confidence": 0.9, "evidence_refs": []any{"sequence:1"}, "semantic_reason": "用户给出明确目标",
		}}
		proposal["intention_candidates"] = []any{map[string]any{
			"operation": "update", "target_ref": intentionRefs[0], "goal_ref": goalRefs[0], "action_intent": "执行下一项受控行动", "expected_outcome": "产生可验证结果",
			"capability_constraints": []any{}, "confidence": 0.9, "evidence_refs": []any{"sequence:2"}, "semantic_reason": "新事实支持更新意图",
		}}
		proposal["emotional_summary"] = map[string]any{
			"dominant_patterns": []any{"面对明确目标时更专注"}, "triggers": []any{"清晰承诺"}, "recovery_patterns": []any{"通过行动恢复稳定"},
			"conflicts": []any{}, "evidence_refs": []any{"sequence:1", "sequence:2"},
		}
		proposal["affect_recalibration_candidates"] = []any{map[string]any{
			"target": "baseline.pleasure", "direction": "increase", "strength": 0.5, "confidence": 0.9,
			"evidence_refs": []any{"sequence:1", "sequence:2"}, "semantic_reason": "跨事实证据支持轻微提高积极情绪基线",
		}}
		proposal["personality_evolution_candidates"] = []any{map[string]any{
			"field_path": "traits.openness", "direction": "increase", "strength": 0.6, "confidence": 0.9,
			"evidence_refs": []any{"sequence:1", "sequence:2"}, "semantic_reason": "两个独立事实持续支持更开放的探索方式",
		}}
		proposal["behavior_policy_evolution_candidates"] = []any{map[string]any{
			"field_path": "response.style", "direction": "toward", "semantic_value": "warm_concise", "strength": 0.6, "confidence": 0.95,
			"evidence_refs": []any{"sequence:1", "sequence:2", "sequence:3"}, "semantic_reason": "三个独立事实支持温和且简洁的回应策略",
		}}
		proposal["relationship_observations"] = []any{map[string]any{
			"target_ref": relationshipRefs[0], "observation": "持续协作使关系更稳固", "direction": "improve", "strength": 0.6, "confidence": 0.9,
			"evidence_refs": []any{"sequence:1", "sequence:2"}, "semantic_reason": "两个事实均体现持续协作",
		}}
		proposal["drive_candidates"] = []any{map[string]any{
			"operation": "create", "key": "achievement", "semantic_value": "完成重要目标的需要", "direction": "increase", "strength": 0.7, "confidence": 0.9,
			"evidence_refs": []any{"sequence:1", "sequence:2"}, "semantic_reason": "持续完成倾向",
		}}
		proposal["preference_candidates"] = []any{map[string]any{
			"operation": "create", "key": "work_style", "semantic_value": "step_by_step", "direction": "toward", "strength": 0.6, "confidence": 0.9,
			"evidence_refs": []any{"sequence:1", "sequence:2"}, "semantic_reason": "偏好逐步推进",
		}}
		proposal["trigger_candidates"] = []any{map[string]any{
			"operation": "create", "key": "goal_check_in", "semantic_value": "new verified outcome", "direction": "toward", "strength": 0.5, "confidence": 0.9,
			"evidence_refs": []any{"sequence:1", "sequence:2"}, "semantic_reason": "新结果出现时值得复查",
		}}
		proposal["developing_self_candidates"] = []any{map[string]any{
			"operation": "create", "category": "habit", "claim": "倾向把复杂目标拆成可验证步骤", "confidence": 0.9,
			"evidence_refs": []any{"sequence:1", "sequence:2"}, "semantic_reason": "多个事实支持该习惯",
		}}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(proposal)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: providerHTTP}}
	result, err := app.ProcessReflection(ctx, fluctlightID, "reflection-v2-correlation")
	if err != nil || stringValue(result["status"]) != "applied" {
		t.Fatalf("Reflection V2 result=%#v err=%v", result, err)
	}
	counts := result["counts"].(map[EvolutionDomain]ReflectionDispositionCounts)
	for _, domain := range []EvolutionDomain{EvolutionMemory, EvolutionRelationship, EvolutionGoal, EvolutionIntention, EvolutionAffectProfile, EvolutionDrive, EvolutionPreference, EvolutionTrigger, EvolutionDevelopingSelf, EvolutionPersonality, EvolutionBehaviorPolicy} {
		expected := 1
		if domain == EvolutionAffectProfile {
			expected = 2
		}
		if domain == EvolutionPersonality || domain == EvolutionBehaviorPolicy {
			expected = 0
			if counts[domain].Deferred != 1 {
				t.Fatalf("single-window overlay domain %s was not deferred: %#v", domain, counts[domain])
			}
		}
		if counts[domain].Applied != expected {
			t.Fatalf("domain %s counts=%#v", domain, counts[domain])
		}
	}
	var memoryCount, goalRevision, intentionRevision, affectRevision, overlayCount, relationshipRevision, driveCount, preferenceCount, triggerCount, selfCount int
	var desiredOutcome, actionIntent string
	var emotionalSummary, baselinePAD []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memories WHERE owner_fluctlight_id=$1 AND content='用户希望真正完成当前事项'`, fluctlightID).Scan(&memoryCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT desired_outcome,revision FROM public.fluctlight_goals WHERE id=$1`, goal.EntityID).Scan(&desiredOutcome, &goalRevision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT action_intent,revision FROM public.fluctlight_intentions WHERE id=$1`, intention.EntityID).Scan(&actionIntent, &intentionRevision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT emotional_summary,baseline_pad,revision FROM public.fluctlight_affect_profiles WHERE fluctlight_id=$1`, fluctlightID).Scan(&emotionalSummary, &baselinePAD, &affectRevision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_evolution_overlays WHERE fluctlight_id=$1 AND profile_id='default' AND status='active'`, fluctlightID).Scan(&overlayCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.relationships WHERE id='reflection-v2-relationship'`).Scan(&relationshipRevision); err != nil {
		t.Fatal(err)
	}
	for query, target := range map[string]*int{
		`SELECT count(*) FROM public.fluctlight_drive_slots WHERE fluctlight_id=$1 AND key='achievement' AND status='active'`:                 &driveCount,
		`SELECT count(*) FROM public.fluctlight_preference_slots WHERE fluctlight_id=$1 AND key='work_style' AND status='active'`:             &preferenceCount,
		`SELECT count(*) FROM public.fluctlight_trigger_preferences WHERE fluctlight_id=$1 AND key='goal_check_in' AND status='active'`:       &triggerCount,
		`SELECT count(*) FROM public.fluctlight_developing_self_claims WHERE fluctlight_id=$1 AND claim='倾向把复杂目标拆成可验证步骤' AND status='active'`: &selfCount,
	} {
		if err := repository.Pool().QueryRow(ctx, query, fluctlightID).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if memoryCount != 1 || desiredOutcome != "真正完成当前事项" || goalRevision != 2 || actionIntent != "执行下一项受控行动" || intentionRevision != 2 || affectRevision != 2 || numberOrZero(decodeObject(baselinePAD)["pleasure"]) <= 0 || overlayCount != 0 || relationshipRevision != 1 || driveCount != 1 || preferenceCount != 1 || triggerCount != 1 || selfCount != 1 || !strings.Contains(string(emotionalSummary), "面对明确目标时更专注") {
		t.Fatalf("closure memory=%d goal=(%q,%d) intention=(%q,%d) affect=(%s,%d) overlays=%d relationship=%d drive=%d preference=%d trigger=%d self=%d", memoryCount, desiredOutcome, goalRevision, actionIntent, intentionRevision, emotionalSummary, affectRevision, overlayCount, relationshipRevision, driveCount, preferenceCount, triggerCount, selfCount)
	}
	projectionAfter, err := app.BuildContextProjection(ctx, ownerID, fluctlightID, "", "reflection-v2-next", "")
	if err != nil || len(projectionAfter.Goals) != 1 || len(projectionAfter.Intentions) != 1 || stringValue(projectionAfter.Goals[0]["desired_outcome"]) != desiredOutcome || stringValue(projectionAfter.Intentions[0]["action_intent"]) != actionIntent {
		t.Fatalf("next projection goals=%#v intentions=%#v err=%v", projectionAfter.Goals, projectionAfter.Intentions, err)
	}
	if len(projectionAfter.DriveSlots) == 0 || len(projectionAfter.PreferenceSlots) == 0 || len(projectionAfter.TriggerPreferences) == 0 || len(projectionAfter.DevelopingSelf) == 0 || len(projectionAfter.Relationships) == 0 {
		t.Fatalf("next projection did not expose all evolved domains: drive=%#v preference=%#v trigger=%#v self=%#v relationship=%#v", projectionAfter.DriveSlots, projectionAfter.PreferenceSlots, projectionAfter.TriggerPreferences, projectionAfter.DevelopingSelf, projectionAfter.Relationships)
	}
}
