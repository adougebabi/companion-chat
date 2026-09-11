package core

import (
	"io"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestProcessReflectionV2AdvancesGoalOnlyFromBoundCompletedOutcome(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "goal-progress-owner", "goal-progress-fluctlight"
	endpointID := "goal-progress-endpoint"
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://goal-progress.invalid','goal-progress-secret','ready',now())`, []any{endpointID}},
		{`INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection',$1,'goal-progress-model','structured_output',4096,5,'{}')`, []any{endpointID}},
		{`INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, []any{ownerID, fluctlightID}},
		{`INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{"personality_system":{"active_profile_id":"default"}}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, []any{fluctlightID, ownerID}},
		{`INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id) VALUES($1,'default')`, []any{fluctlightID}},
		{`INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, []any{fluctlightID}},
		{`INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, []any{fluctlightID}},
		{`INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,1,0)`, []any{fluctlightID}},
	} {
		if _, err := repository.Pool().Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	goal, record, err := CreateGoalAuthority(GoalAuthority{
		EntityID: "goal-progress-goal", SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FluctlightID: fluctlightID, ProfileID: "default", DesiredOutcome: "交付可验证结果", SuccessCriteria: []string{"产出真实结果", "结果被确认"},
		Motivation: "完成目标", Scope: "general", Importance: 0.9, Urgency: 0.8, Status: GoalActive, Revision: 1, EvidenceRefs: []string{"owner:goal"},
	}, []string{"owner:goal"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistGoalAuthorityTx(ctx, tx, nil, goal, record, "goal-progress-create")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	projection, err := app.BuildContextProjection(ctx, ownerID, fluctlightID, "", "goal-progress-seed", "")
	if err != nil {
		t.Fatal(err)
	}
	var goalEntry ContextReference
	for _, entry := range projection.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceGoal && entry.EntityID == goal.EntityID {
			goalEntry = entry
		}
	}
	outcome := ActionOutcome{
		SchemaVersion: actionOutcomeSchemaVersion, ID: "goal-progress-outcome", FluctlightID: fluctlightID,
		ActionID: "goal-progress-action", CallID: actionPrimaryCallID, Status: ActionOutcomeCompleted, SuccessBoundary: "action_settled",
		Expected: map[string]any{"action_type": "capability"}, Observed: map[string]any{"status": "completed"},
		GoalRefs: []string{goalEntry.Ref}, EvidenceRefs: []string{"goal-progress-source"},
		ContextReferences: map[string]ContextReference{goalEntry.Ref: goalEntry}, Revision: 1, OccurredAt: now,
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		if err := persistActionOutcomesTx(ctx, tx, []ActionOutcome{outcome}); err != nil {
			return err
		}
		_, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", map[string]any{"outcome": outcome}, "goal-progress-outcome-fact")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		text := string(body)
		goalRefs := regexp.MustCompile(`goal:ctx_[a-f0-9]{32}`).FindAllString(text, -1)
		outcomeRefs := regexp.MustCompile(`outcome:ctx_[a-f0-9]{32}`).FindAllString(text, -1)
		if len(goalRefs) == 0 || len(outcomeRefs) == 0 {
			t.Fatalf("Provider request omitted Goal/Outcome refs: %s", text)
		}
		proposal := reflectionProposalV2Fixture(nil)
		proposal["summary"] = "完成结果支持目标进展"
		proposal["goal_candidates"] = []any{map[string]any{
			"operation": "update", "target_ref": goalRefs[0], "direction": "increase", "strength": 0.8, "confidence": 0.9,
			"outcome_refs": []any{outcomeRefs[0]}, "criterion_indexes": []any{0}, "complete": false,
			"evidence_refs": []any{"sequence:1"}, "semantic_reason": "真实 completed Outcome 满足第一项成功标准",
		}}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(proposal)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}
	app.Provider = &ProviderClient{DB: repository, HTTP: providerHTTP}
	result, err := app.ProcessReflection(ctx, fluctlightID, "goal-progress-reflection")
	if err != nil || stringValue(result["status"]) != "applied" {
		t.Fatalf("Reflection result=%#v err=%v", result, err)
	}
	var progress float64
	var revision int
	if err := repository.Pool().QueryRow(ctx, `SELECT progress,revision FROM public.fluctlight_goals WHERE id=$1`, goal.EntityID).Scan(&progress, &revision); err != nil {
		t.Fatal(err)
	}
	if progress < 0.5 || revision != 2 {
		t.Fatalf("Goal progress=%v revision=%d", progress, revision)
	}
}
