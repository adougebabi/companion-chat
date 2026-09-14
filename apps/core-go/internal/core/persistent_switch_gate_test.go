package core

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// The permission spine tests are split in two layers:
//
//   - pure tests over the E1/E2/E5 helpers and the scope codec, which need no
//     database and therefore cannot accidentally depend on turn plumbing;
//   - real-runtime tests that seed fluctlight_personality_runtime and prove the
//     E3 gate is the only thing that can move active_profile_id.

// ---------------------------------------------------------------------------
// E1: unauthorized proposals never reach the durable decision
// ---------------------------------------------------------------------------

func TestE1UnauthorizedGrantDropsPersistentSwitchField(t *testing.T) {
	decision := map[string]any{
		"visible_text":           "好的，我在。",
		"personality_decision":   map[string]any{"decision": "switch", "from_profile_id": "spark", "target_profile_id": "twilight", "trigger_id": "switch:safety"},
		"personality_transition": map[string]any{"target_profile_id": "twilight"},
	}
	grant := persistentSwitchGrant{Allowed: false, Scenario: persistentSwitchGrantScenarioTakeover, Reason: "takeover_reply_owner"}
	result, diagnostics := applyPersistentSwitchGrant(decision, grant)

	if _, exists := result["personality_transition"]; exists {
		t.Fatalf("unauthorized decision kept personality_transition: %#v", result)
	}
	if _, exists := result["personality_decision"]; exists {
		t.Fatalf("unauthorized decision kept personality_decision: %#v", result)
	}
	if result["visible_text"] != "好的，我在。" {
		t.Fatalf("the user-visible reply must survive the drop: %#v", result)
	}
	proposal, ok := result[persistentSwitchProposalKey].(map[string]any)
	if !ok {
		t.Fatalf("the drop was not recorded: %#v", result[persistentSwitchProposalKey])
	}
	if proposal["proposed"] != true || proposal["applied"] != false || proposal["scenario"] != persistentSwitchGrantScenarioTakeover {
		t.Fatalf("unexpected drop record: %#v", proposal)
	}
	summary, ok := proposal["proposed_decision"].(map[string]any)
	if !ok {
		t.Fatalf("the raw identifiers of the dropped proposal must be kept: %#v", proposal)
	}
	for _, leaked := range []string{"condition", "reason", "plan"} {
		if _, exists := summary[leaked]; exists {
			t.Fatalf("the drop record leaked %q: %#v", leaked, summary)
		}
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != persistentSwitchProposalDropped {
		t.Fatalf("expected exactly one dropped-proposal diagnostic, got %#v", diagnostics)
	}
}

func TestE1AuthorizedGrantLeavesDecisionUntouched(t *testing.T) {
	decision := map[string]any{
		"personality_transition": map[string]any{"target_profile_id": "twilight"},
	}
	grant := persistentSwitchGrant{Allowed: true, Scenario: persistentSwitchGrantScenarioMain, DeclaredRules: []string{"switch:safety"}}
	result, diagnostics := applyPersistentSwitchGrant(decision, grant)

	if len(diagnostics) != 0 {
		t.Fatalf("an authorized grant produced diagnostics: %#v", diagnostics)
	}
	if _, exists := result["personality_transition"]; !exists {
		t.Fatalf("an authorized decision lost its transition: %#v", result)
	}
	if _, exists := result[persistentSwitchProposalKey]; exists {
		t.Fatalf("an authorized decision must not record a dropped proposal: %#v", result)
	}
}

func TestE1WithoutProposalRecordsNothingDropped(t *testing.T) {
	decision := map[string]any{"visible_text": "只是闲聊"}
	result, diagnostics := applyPersistentSwitchGrant(decision, persistentSwitchGrant{Allowed: false, Scenario: persistentSwitchGrantScenarioTakeover})

	proposal := mapValue(result[persistentSwitchProposalKey])
	if proposal["proposed"] != false || proposal["applied"] != false {
		t.Fatalf("a no-op drop must still be recorded as not proposed: %#v", proposal)
	}
	if _, exists := proposal["proposed_decision"]; exists {
		t.Fatalf("a no-op drop must not invent raw identifiers: %#v", proposal)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("a no-op drop must not emit a diagnostic: %#v", diagnostics)
	}
}

// ---------------------------------------------------------------------------
// E2: the frozen payload is the only authority record
// ---------------------------------------------------------------------------

func TestE2ValidatePersistentSwitchPersistence(t *testing.T) {
	authorized := map[string]any{persistentSwitchPayloadKey: map[string]any{"authorized": true}}
	unauthorized := map[string]any{persistentSwitchPayloadKey: map[string]any{"authorized": false}}
	withTransition := map[string]any{"personality_transition": map[string]any{"target_profile_id": "twilight"}}
	withoutTransition := map[string]any{"visible_text": "ok"}

	if err := validatePersistentSwitchPersistence(authorized, withTransition); err != nil {
		t.Fatalf("an authorized transition was rejected: %v", err)
	}
	err := validatePersistentSwitchPersistence(unauthorized, withTransition)
	if err == nil {
		t.Fatal("an unauthorized transition must be rejected")
	}
	if err.Error() != persistentSwitchNotAuthorizedCode {
		t.Fatalf("unexpected rejection reason: %v", err)
	}
	if err := validatePersistentSwitchPersistence(unauthorized, withoutTransition); err != nil {
		t.Fatalf("a decision without a transition needs no authorization: %v", err)
	}
}

func TestE2PayloadMarkerNeverInfersAuthorityFromTransition(t *testing.T) {
	if persistentSwitchAuthorizedByPayload(map[string]any{}) {
		t.Fatal("a missing marker must not be treated as authorized")
	}
	if persistentSwitchAuthorizedByPayload(map[string]any{persistentSwitchPayloadKey: map[string]any{"authorized": "true"}}) {
		t.Fatal("a non-boolean authorization must not be accepted")
	}
	if !persistentSwitchAuthorizedByPayload(map[string]any{persistentSwitchPayloadKey: map[string]any{"authorized": true}}) {
		t.Fatal("a positive boolean authorization must be accepted")
	}
}

// ---------------------------------------------------------------------------
// E3: the gate is a structural no-op without authorization
// ---------------------------------------------------------------------------

func TestE3GateIsStructuralNoopWithoutAuthorization(t *testing.T) {
	// A nil App.DB pool would panic if the gate ever reached the database, so a
	// clean return here proves the no-op happens before any query.
	app := &App{}
	plan := &personalityDecisionPlan{
		SchemaVersion: personalityDecisionPlanVersion, FluctlightID: "fluctlight", Choice: "switch",
		CurrentProfile: "spark", TargetProfile: "twilight", PreviousProfile: "spark",
		ExpectedRevision: 0, ResultingRevision: 1, RuntimeExists: true,
	}

	if result, err := app.applyPersistentSwitchIfAuthorizedTx(context.Background(), nil, "fluctlight", nil, plan); err != nil || result != nil {
		t.Fatalf("a payload with no marker must be a no-op, got (%v, %v)", result, err)
	}
	unauthorized := map[string]any{persistentSwitchPayloadKey: map[string]any{"authorized": false}}
	if result, err := app.applyPersistentSwitchIfAuthorizedTx(context.Background(), nil, "fluctlight", unauthorized, plan); err != nil || result != nil {
		t.Fatalf("an unauthorized payload must be a no-op, got (%v, %v)", result, err)
	}
	authorized := map[string]any{persistentSwitchPayloadKey: map[string]any{"authorized": true}}
	if result, err := app.applyPersistentSwitchIfAuthorizedTx(context.Background(), nil, "fluctlight", authorized, nil); err != nil || result != nil {
		t.Fatalf("a nil plan must be a no-op, got (%v, %v)", result, err)
	}
}

// ---------------------------------------------------------------------------
// E5: an overwrite never inherits authority
// ---------------------------------------------------------------------------

func TestE5OverwriteNeverInheritsAuthorization(t *testing.T) {
	payload := map[string]any{
		persistentSwitchPayloadKey: map[string]any{"authorized": true, "scenario": persistentSwitchGrantScenarioMain},
		"decision": map[string]any{
			"personality_transition": map[string]any{"target_profile_id": "twilight"},
			"personality_decision":   map[string]any{"decision": "switch"},
			"visible_text":           "旧的候选",
		},
	}
	resetPersistentSwitchAuthorization(payload, persistentSwitchGrantScenarioTakeover, "takeover_reply_owner")

	if persistentSwitchAuthorizedByPayload(payload) {
		t.Fatal("the overwrite inherited the previous authorization")
	}
	decision := mapValue(payload["decision"])
	if _, exists := decision["personality_transition"]; exists {
		t.Fatalf("the overwrite kept the previous transition: %#v", decision)
	}
	if _, exists := decision["personality_decision"]; exists {
		t.Fatalf("the overwrite kept the previous decision: %#v", decision)
	}
	if decision["visible_text"] != "旧的候选" {
		t.Fatalf("the overwrite must not touch the visible text: %#v", decision)
	}
	marker := mapValue(payload[persistentSwitchPayloadKey])
	if marker["scenario"] != persistentSwitchGrantScenarioTakeover || marker["reason"] != "takeover_reply_owner" {
		t.Fatalf("the reset marker lost its provenance: %#v", marker)
	}
}

// ---------------------------------------------------------------------------
// Turn persona scope
// ---------------------------------------------------------------------------

func TestTurnPersonaScopeRoundTripThroughPayload(t *testing.T) {
	projection := ContextProjection{
		CorePersona:        map[string]any{"personality_system": map[string]any{"active_profile_id": "spark"}},
		PersonalityRuntime: map[string]any{"active_profile_id": "spark", "revision": 3},
		EffectivePersona:   map[string]any{"authority_revision": 1},
	}
	scope := resolveTurnPersonaScope(projection)
	if scope.ActiveProfileID != "spark" || scope.ReplyOwnerProfileID != "spark" || scope.PersonaRevision != 3 {
		t.Fatalf("unexpected scope: %#v", scope)
	}
	restored, ok := turnPersonaScopeFromPayload(map[string]any{turnPersonaScopePayloadKey: turnPersonaScopePayload(scope)})
	if !ok {
		t.Fatal("the scope payload did not round-trip")
	}
	if restored.ActiveProfileID != scope.ActiveProfileID || restored.ReplyOwnerProfileID != scope.ReplyOwnerProfileID ||
		restored.PersonaRevision != scope.PersonaRevision || restored.ScopeRevision != scope.ScopeRevision {
		t.Fatalf("round-trip mismatch: %#v vs %#v", restored, scope)
	}
}

func TestResolveTurnPersonaScopeFallsBackToDeclaredInitialProfile(t *testing.T) {
	projection := ContextProjection{
		CorePersona: map[string]any{"personality_system": map[string]any{
			"profiles": []any{map[string]any{"id": "spark"}},
		}},
	}
	scope := resolveTurnPersonaScope(projection)
	if scope.ActiveProfileID != "spark" {
		t.Fatalf("expected a fallback to the declared initial profile, got %#v", scope)
	}
}

// ---------------------------------------------------------------------------
// Real runtime: only an authorized scenario may move active_profile_id
// ---------------------------------------------------------------------------

func seedPersonaRuntimeForGate(t *testing.T, ctx context.Context, repository *PostgresRepository, fluctlightID, active string, revision int) {
	t.Helper()
	ownerID := fluctlightID + "-owner"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{"personality_system":{"active_profile_id":"spark","profiles":[{"id":"spark","name":"星火"},{"id":"twilight","name":"暮光"}]}}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,$2,$3)`, fluctlightID, active, revision); err != nil {
		t.Fatal(err)
	}
}

func readActiveProfileForGate(t *testing.T, ctx context.Context, repository *PostgresRepository, fluctlightID string) string {
	t.Helper()
	var active string
	if err := repository.Pool().QueryRow(ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	return active
}

func persistentSwitchTestPlanForGate(fluctlightID string) *personalityDecisionPlan {
	return &personalityDecisionPlan{
		SchemaVersion: personalityDecisionPlanVersion, FluctlightID: fluctlightID, Choice: "switch",
		CurrentProfile: "spark", TargetProfile: "twilight", PreviousProfile: "spark",
		ExpectedRevision: 0, ResultingRevision: 1, RuntimeExists: true,
	}
}

func TestTakeoverReplyCannotWritePersistentActive(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "gate-takeover-fluctlight"
	seedPersonaRuntimeForGate(t, ctx, repository, fluctlightID, "spark", 0)
	app := &App{DB: repository}

	// The takeover speaker produces a legal-looking persistent switch. The
	// authority comes from the frozen scenario, never from the candidate text.
	grant := resolvePersistentSwitchGrant(turnPersonaScope{ActiveProfileID: "spark", ReplyOwnerProfileID: "spark"}, persistentSwitchGrantScenarioTakeover, nil)
	if grant.Allowed {
		t.Fatalf("takeover_reply must never be authorized: %#v", grant)
	}
	decision, _ := applyPersistentSwitchGrant(map[string]any{
		"visible_text":           "由暮光接管的一轮",
		"personality_decision":   map[string]any{"decision": "switch", "target_profile_id": "twilight"},
		"personality_transition": map[string]any{"target_profile_id": "twilight"},
	}, grant)
	payload := map[string]any{"decision": decision, persistentSwitchPayloadKey: persistentSwitchPayloadForTurn(turnDecisionAuthority{Grant: grant})}

	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.applyPersistentSwitchIfAuthorizedTx(ctx, tx, fluctlightID, payload, persistentSwitchTestPlanForGate(fluctlightID))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("a takeover reply changed the persistent active profile to %q", active)
	}
}

func TestPersistentSwitchStillWorksViaAuthorizedScenarios(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "gate-main-fluctlight"
	seedPersonaRuntimeForGate(t, ctx, repository, fluctlightID, "spark", 0)
	app := &App{DB: repository}

	rules := []personaSwitchRule{{RuleID: "switch:safety", Source: personaSwitchSourceSwitchingList, Kind: switchRulePersistentSemantic, TargetProfileID: "twilight", Enabled: true}}
	grant := resolvePersistentSwitchGrant(turnPersonaScope{ActiveProfileID: "spark", ReplyOwnerProfileID: "spark"}, persistentSwitchGrantScenarioMain, rules)
	if !grant.Allowed {
		t.Fatalf("the authorized main scenario must keep working: %#v", grant)
	}
	decision, diagnostics := applyPersistentSwitchGrant(map[string]any{"personality_transition": map[string]any{"target_profile_id": "twilight"}}, grant)
	if len(diagnostics) != 0 {
		t.Fatalf("an authorized switch produced diagnostics: %#v", diagnostics)
	}
	payload := map[string]any{"decision": decision, persistentSwitchPayloadKey: persistentSwitchPayloadForTurn(turnDecisionAuthority{Grant: grant})}

	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.applyPersistentSwitchIfAuthorizedTx(ctx, tx, fluctlightID, payload, persistentSwitchTestPlanForGate(fluctlightID))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "twilight" {
		t.Fatalf("an authorized main switch did not apply, active=%q", active)
	}
}

func TestUnauthorizedScenarioHasNoPersonalityDecisionField(t *testing.T) {
	rules := []personaSwitchRule{{RuleID: "switch:safety", Source: personaSwitchSourceSwitchingList, Kind: switchRulePersistentSemantic, TargetProfileID: "twilight", Enabled: true}}
	owner := turnPersonaScope{ActiveProfileID: "spark", ReplyOwnerProfileID: "spark"}
	for _, scenario := range []string{persistentSwitchGrantScenarioTakeover, persistentSwitchGrantScenarioQuery, persistentSwitchGrantScenarioJudge} {
		grant := resolvePersistentSwitchGrant(owner, scenario, rules)
		decision, _ := applyPersistentSwitchGrant(map[string]any{
			"personality_decision":   map[string]any{"decision": "switch", "target_profile_id": "twilight"},
			"personality_transition": map[string]any{"target_profile_id": "twilight"},
		}, grant)
		if _, exists := decision["personality_decision"]; exists {
			t.Fatalf("scenario %s kept personality_decision: %#v", scenario, decision)
		}
		if _, exists := decision["personality_transition"]; exists {
			t.Fatalf("scenario %s kept personality_transition: %#v", scenario, decision)
		}
	}
}

func TestRecoveryDoesNotApplyUnauthorizedSwitch(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "gate-recovery-fluctlight"
	seedPersonaRuntimeForGate(t, ctx, repository, fluctlightID, "spark", 0)
	app := &App{DB: repository}

	// A frozen payload may still carry a transition (legacy row, or a scenario
	// that was later downgraded). Recovery reads the frozen authority and must
	// not re-derive it from the presence of the transition field.
	payload := map[string]any{
		persistentSwitchPayloadKey: map[string]any{"authorized": false, "scenario": persistentSwitchGrantScenarioTakeover},
		"personality_transition":   map[string]any{"target_profile_id": "twilight"},
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.applyPersistentSwitchIfAuthorizedTx(ctx, tx, fluctlightID, payload, persistentSwitchTestPlanForGate(fluctlightID))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("recovery applied an unauthorized switch, active=%q", active)
	}
}

// ---------------------------------------------------------------------------
// Static guard: the settlement points cannot write active_profile_id directly
// ---------------------------------------------------------------------------

func TestPersistentSwitchStaticGuardOnlyGateWritesActiveProfile(t *testing.T) {
	data, err := os.ReadFile("mutations.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if count := strings.Count(source, "a.applyPersonalityDecisionPlanTx("); count != 0 {
		t.Fatalf("mutations.go must write the persistent active profile only through the authorized gate, found %d direct calls", count)
	}
	if count := strings.Count(source, "a.applyPersistentSwitchIfAuthorizedTx("); count != 3 {
		t.Fatalf("the three settlement points (main reply, query continuation, recovery) must call the gate, found %d", count)
	}
	if count := strings.Count(source, "applyPersistentSwitchGrant(decision, personaGrant)"); count != 0 {
		t.Fatalf("the Main turn must not keep a second E1 call site outside the shared normalizer, found %d", count)
	}

	// E1 lives in the single shared normalizer so the takeover reply is filtered
	// by exactly the same rule as the Main turn.
	normalizer, err := os.ReadFile("turn_decision.go")
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(normalizer), "applyPersistentSwitchGrant(decision, input.Grant)"); count != 1 {
		t.Fatalf("the Main turn must drop an unauthorized persistent proposal before persisting it (E1), found %d call sites", count)
	}

	gate, err := os.ReadFile("persistent_switch_gate.go")
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(gate), "a.applyPersonalityDecisionPlanTx("); count != 1 {
		t.Fatalf("the gate must be the single place that writes the persistent active profile, found %d", count)
	}
	if !strings.Contains(string(gate), "!persistentSwitchAuthorizedByPayload(payload)") {
		t.Fatal("the gate must refuse a payload without positive authorization (E3)")
	}

	runtime, err := os.ReadFile("personality_runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(runtime), "func (a *App) applyPersonalityDecision(") {
		t.Fatal("an un-gated applyPersonalityDecision helper reintroduces a write path that bypasses E3")
	}
}
