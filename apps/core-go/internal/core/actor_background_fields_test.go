package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func completeActorBackground() map[string]any {
	result := map[string]any{}
	for _, key := range actorUserBackgroundFields {
		result[key] = "about-you-" + key
	}
	result["timezone"] = "Asia/Shanghai"
	result["meeting_confirmed"] = false
	result["rules"] = "先询问再给建议\n不把我的偏好当作工具授权"
	return result
}

func TestAboutYouCompleteFieldsPersistScopeClearAndConsumePhysicalRequests(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	other := randomID("human_about_")
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, other); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'member','active')`, f.conversationID, other); err != nil {
		t.Fatal(err)
	}
	recordActorFact(t, f, "separate-owner", "assert", "full_name", "OWNER_ONLY_SECRET", "")
	initial, err := f.app.ActorContext(f.ctx, f.ownerID, f.fluctlightID, other)
	if err != nil {
		t.Fatal(err)
	}
	values := completeActorBackground()
	body := map[string]any{"background": values, "expected_context_version": initial["context_version"], "operation": "correct", "reason": "本人填写", "idempotency_key": "complete-background"}
	saved, err := f.app.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, other, body)
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(saved["background"]) != jsonString(values) {
		t.Fatal("fields lost on save", saved)
	}
	replay, err := f.app.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, other, body)
	if err != nil || replay["replayed"] != true {
		t.Fatal("replay", replay, err)
	}
	stale := cloneMap(body)
	stale["idempotency_key"] = "stale"
	if _, err := f.app.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, other, stale); !errors.Is(err, ErrConflict) {
		t.Fatal("stale accepted", err)
	}
	if _, err := f.app.UpdateActorContext(f.ctx, f.foreignOwnerID, f.fluctlightID, other, body); err == nil {
		t.Fatal("foreign owner accepted")
	}
	if _, err := f.app.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, f.foreignOwnerID, body); err == nil {
		t.Fatal("foreign actor accepted")
	}
	second := randomID("about_second_")
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'fluctlight','active')`, second); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, second, f.ownerID); err != nil {
		t.Fatal(err)
	}
	otherScope, err := f.app.ActorContext(f.ctx, f.ownerID, second, f.ownerID)
	if err != nil || len(mapValue(otherScope["background"])) != 0 {
		t.Fatal("cross-character data", otherScope, err)
	}
	// A new App reads the same fact authority; no in-memory profile cache is used.
	restarted := newTestApp(t, f.repository, nil)
	reread, err := restarted.ActorContext(f.ctx, f.ownerID, f.fluctlightID, other)
	if err != nil || jsonString(reread["background"]) != jsonString(values) {
		t.Fatal("restart", reread, err)
	}
	// Correct one value and explicitly clear another. Omission preserves the other fields.
	correction := map[string]any{"background": map[string]any{"full_name": "LATEST_PARTICIPANT_NAME", "age": nil}, "expected_context_version": reread["context_version"], "operation": "correct", "reason": "纠正本人资料", "idempotency_key": "correct-background"}
	latest, err := restarted.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, other, correction)
	if err != nil {
		t.Fatal(err)
	}
	if v, exists := mapValue(latest["background"])["age"]; !exists || v != nil {
		t.Fatal("clear lost", latest)
	}
	var corrected int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.actor_facts WHERE owner_fluctlight_id=$1 AND subject_actor_id=$2 AND status='corrected' AND attribute IN ('age','full_name')`, f.fluctlightID, other).Scan(&corrected); err != nil || corrected != 2 {
		t.Fatal("correct history", corrected, err)
	}
	// Genuine change ends the former interval and preserves it as formerly true.
	time.Sleep(time.Millisecond)
	change := map[string]any{"background": map[string]any{"habits": "LATEST_PARTICIPANT_HABIT"}, "expected_context_version": latest["context_version"], "operation": "change", "reason": "生活习惯变化", "idempotency_key": "change-background"}
	if _, err := restarted.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, other, change); err != nil {
		t.Fatal(err)
	}
	var former int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.actor_facts WHERE owner_fluctlight_id=$1 AND subject_actor_id=$2 AND attribute='habits' AND status='active' AND valid_until IS NOT NULL`, f.fluctlightID, other).Scan(&former); err != nil || former != 1 {
		t.Fatal("change history", former, err)
	}
	projection, err := restarted.BuildContextProjectionFor(f.ctx, ContextProjectionRequest{AuthorizationActorID: f.ownerID, SpeakerActorID: other, TargetActorID: other, FluctlightID: f.fluctlightID, ConversationID: f.conversationID, CurrentUserText: "聊聊我的近况", MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
	if err != nil {
		t.Fatal(err)
	}
	// Snapshot validation uses the same relevant Actor window as the prompt.
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return validateActorFactSnapshotTx(f.ctx, tx, f.fluctlightID, projection.ActorFacts, restarted.now(), other)
	}); err != nil {
		t.Fatal("valid scoped snapshot rejected", err)
	}
	recordActorFact(t, f, "owner-unrelated-update", "correct", "full_name", "OWNER_ONLY_UPDATED", "")
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return validateActorFactSnapshotTx(f.ctx, tx, f.fluctlightID, projection.ActorFacts, restarted.now(), other)
	}); err != nil {
		t.Fatal("unrelated Actor invalidated snapshot", err)
	}
	if len(compactActorBackground(projection)) != len(actorUserBackgroundFields) {
		t.Fatal("complete background silently truncated", len(compactActorBackground(projection)))
	}
	requireLatest := func(wire string) {
		t.Helper()
		for _, text := range []string{"LATEST_PARTICIPANT_NAME", "LATEST_PARTICIPANT_HABIT", "wants_you_to_know", "shared_information", "rules"} {
			if !strings.Contains(wire, text) {
				t.Fatalf("missing %s in selected request", text)
			}
		}
		for _, text := range []string{"OWNER_ONLY_SECRET", "about-you-full_name", "about-you-age", "about-you-habits"} {
			if strings.Contains(wire, text) {
				t.Fatalf("stale/foreign %s in selected request", text)
			}
		}
	}
	workingInput := factOnlyCognitionMemoryInput(projection, nil, nil)
	workingPolicy, err := reserveRequiredWorkingMemory(workingInput, DefaultWorkingMemoryPolicy(), 32768)
	if err != nil {
		t.Fatal(err)
	}
	wm, err := ResolveWorkingMemory(workingInput, workingPolicy)
	if err != nil {
		t.Fatal(err)
	}
	input := replyContextFixture()
	input.Projection = projection
	policy := DefaultPromptBudgetPolicy(1024)
	policy.MaxInputTokens = 32768
	policy.ContextWindowTokens = 65536
	cognition, err := AssemblePromptContext(PromptAssemblyInput{Surface: ProviderContextSurfaceFactOnlyCognition, Role: "cognitive_assessment", OperationRules: []string{factOnlyExecutionBoundary}, CorePersona: input.WorkingPersona, CurrentInput: projection.CurrentUserText, WorkingMemory: wm, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	requireLatest(jsonString(cognition.Messages))
	if !strings.Contains(jsonString(cognition.Messages), "unknown") {
		t.Fatal("null age missing unknown semantics")
	}
	// Real HTTP serialization to a controlled Provider; this is not a live model claim.
	calls := 0
	transport := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		requireLatest(jsonString(request["messages"]))
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "我记得了。"}}}})
	}))
	defer transport.Close()
	provider := &ProviderClient{HTTP: transport.Client()}
	assignment := providerAssignment{Role: "cognitive_assessment", BaseURL: transport.URL, ModelID: "controlled-qwen", Timeout: 3 * time.Second, TokenBudget: 1024, MaxInputTokens: 32768, ContextWindowTokens: 65536, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
	if _, err := provider.Text(withFrozenProviderAssignment(f.ctx, "cognitive_assessment", assignment), "cognitive_assessment", cognition.Messages); err != nil {
		t.Fatal(err)
	}
	input.CurrentInput = projection.CurrentUserText
	assignment.Role = conversationReplyModelRole
	assignment.ModelID = "controlled-cydonia"
	if _, err := (&App{Provider: provider}).RunConversationReplyTask(f.ctx, ConversationReplyTaskInput{Context: input, Assignment: assignment, Concurrency: 1}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("missing physical requests", calls)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,target_actor_id,profile_id,role,metrics,revision) VALUES($1,$2,$3,'default','{"label":"configured contact"}','{}',1)`, randomID("about_relation_"), f.fluctlightID, other); err != nil {
		t.Fatal(err)
	}
	planner, err := restarted.executeGoalPlannerQuery(f.ctx, CapabilityInvocation{CallID: "about-planner", CapabilityName: goalPlannerQuery, Arguments: jsonBytes(map[string]any{"section": "actor", "actor_id": other}), Metadata: InvocationMetadata{AuthorizationActorID: f.ownerID, FluctlightID: f.fluctlightID, WorkingProfileID: "default"}})
	if err != nil {
		t.Fatal(err)
	}
	requireLatest(jsonString(planner.Output))
	t.Logf("19 fields persisted/restarted; correct/change/null/CAS/replay/2 Actor/2 Fluctlight isolation; 2 controlled physical Provider requests and scoped GoalPlanner query verified")
}

func TestAboutYouCompleteFieldValidationPreservesUnknownAndUnicodeBounds(t *testing.T) {
	for _, key := range actorUserBackgroundFields {
		valid := []any{nil, strings.Repeat("明", 1024)}
		if key == "timezone" {
			valid = []any{nil, "Asia/Shanghai"}
		}
		if key == "meeting_confirmed" {
			valid = []any{nil, false}
		}
		for _, value := range valid {
			if err := validateActorUserSettings(map[string]any{"background": map[string]any{key: value}}); err != nil {
				t.Fatalf("valid %s rejected: %v", key, err)
			}
		}
		for _, value := range []any{[]string{"invalid"}, "", strings.Repeat("明", 1025)} {
			if err := validateActorUserSettings(map[string]any{"background": map[string]any{key: value}}); err == nil {
				t.Fatalf("invalid %s accepted", key)
			}
		}
	}
	if err := validateActorUserSettings(map[string]any{"background": map[string]any{"unknown_key": nil}}); err == nil {
		t.Fatal("unknown null accepted")
	}
}
