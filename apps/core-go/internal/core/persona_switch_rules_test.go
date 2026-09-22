package core

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func multiProfileTestPersona() map[string]any {
	return map[string]any{
		"personality_system": map[string]any{
			"mode":              "multiple",
			"active_profile_id": "twilight",
			"profiles": []any{
				map[string]any{"id": "twilight", "name": "暮光", "voice": map[string]any{"tone": "安静、句子短"}},
				map[string]any{"id": "spark", "name": "星火", "voice": map[string]any{"tone": "热烈、语速快"}},
			},
		},
	}
}

func rulesOfKind(rules []personaSwitchRule, kind personaSwitchRuleKind) []personaSwitchRule {
	result := make([]personaSwitchRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Kind == kind {
			result = append(result, rule)
		}
	}
	return result
}

func findRule(rules []personaSwitchRule, ruleID string) (personaSwitchRule, bool) {
	for _, rule := range rules {
		if rule.RuleID == ruleID {
			return rule, true
		}
	}
	return personaSwitchRule{}, false
}

func diagnosticCodes(diagnostics []personaSwitchDiagnostic) []string {
	result := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, diagnostic.Code)
	}
	return result
}

func hasDiagnosticCode(diagnostics []personaSwitchDiagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func TestNormalizePersonaSwitchRulesClassifiesEveryDeclaredSource(t *testing.T) {
	persona := multiProfileTestPersona()
	system := mapValue(persona["personality_system"])
	system["switching"] = map[string]any{
		"default_profile_id": "twilight",
		"cooldown_seconds":   float64(600),
		"rules": []any{
			map[string]any{"id": "night", "condition": "夜间时段由暮光主导", "target_profile_id": "twilight", "priority": float64(5)},
			map[string]any{"id": "daytime", "kind": "persistent_deterministic", "condition": "每天 09:00-18:00 由星火主导", "target_profile_id": "spark", "priority": float64(9)},
		},
	}
	system["takeover_rules"] = []any{
		map[string]any{"id": "public-challenge", "kind": "turn_takeover", "version": personaTakeoverRuleVersion, "condition": "遇到公开质疑", "target_profile_id": "spark", "priority": float64(3)},
	}

	normalization := normalizePersonaSwitchRules(persona, map[string]any{"active_profile_id": "twilight"}, "twilight")

	if got := len(normalization.Rules); got != 4 {
		t.Fatalf("expected 4 normalized rules (2 switching + 1 takeover_rules + 1 default), got %d: %#v", got, normalization.Rules)
	}
	night, ok := findRule(normalization.Rules, "switch:night")
	if !ok || night.Kind != switchRulePersistentSemantic || night.TargetProfileID != "twilight" || night.SourceProfileID != "twilight" {
		t.Fatalf("switch:night = %#v ok=%t", night, ok)
	}
	daytime, ok := findRule(normalization.Rules, "switch:daytime")
	if !ok || daytime.Kind != switchRulePersistentDeterministic {
		t.Fatalf("time-windowed switching rule must classify as persistent_deterministic: %#v ok=%t", daytime, ok)
	}
	if daytime.Enabled != true {
		t.Fatalf("a fully resolved declaration stays valid: %#v", daytime)
	}
	if len(filterTakeoverAcceptableRules([]personaSwitchRule{daytime})) != 0 {
		t.Fatal("a persistent rule is never eligible for turn takeover")
	}
	takeover, ok := findRule(normalization.Rules, "takeover:public-challenge")
	if !ok || takeover.Kind != switchRuleTurnTakeover || takeover.TargetProfileID != "spark" || !takeover.Enabled {
		t.Fatalf("takeover:public-challenge = %#v ok=%t", takeover, ok)
	}
	def, ok := findRule(normalization.Rules, "switch:default")
	if !ok || def.Kind != switchRulePersistentSemantic || def.TargetProfileID != "twilight" {
		t.Fatalf("switch:default = %#v ok=%t", def, ok)
	}
	// Priority ordering: deterministic 9, semantic 5, takeover 3, default 0.
	if normalization.Rules[0].RuleID != "switch:daytime" || normalization.Rules[1].RuleID != "switch:night" {
		t.Fatalf("rules are not priority-ordered: %#v", normalization.Rules)
	}
	if digest := personaSwitchRuleSetDigest(normalization.Rules); len(digest) != 32 {
		t.Fatalf("rule set digest = %q", digest)
	}
}

func TestNormalizePersonaSwitchRulesSupportsSwitchingDeclaredAsList(t *testing.T) {
	persona := multiProfileTestPersona()
	mapValue(persona["personality_system"])["switching"] = []any{
		map[string]any{"id": "legacy", "condition": "工作场景由星火主导", "target_profile_id": "spark"},
	}
	normalization := normalizePersonaSwitchRules(persona, nil, "")
	rule, ok := findRule(normalization.Rules, "switch:legacy")
	if !ok || rule.Kind != switchRulePersistentSemantic || rule.TargetProfileID != "spark" {
		t.Fatalf("legacy switching list form must normalize: %#v ok=%t", rule, ok)
	}
	if !strings.HasPrefix(rule.Source, "switching") {
		t.Fatalf("legacy list form must still count as a declared switching entry: %#v", rule)
	}
}

func TestPersonaSwitchRuleIDSurvivesConditionTextEdits(t *testing.T) {
	persona := multiProfileTestPersona()
	system := mapValue(persona["personality_system"])
	system["takeover_rules"] = []any{
		map[string]any{"id": "public-challenge", "kind": "turn_takeover", "version": personaTakeoverRuleVersion, "condition": "遇到公开质疑", "target_profile_id": "spark"},
	}

	first := normalizePersonaSwitchRules(persona, nil, "twilight")
	second := normalizePersonaSwitchRules(persona, nil, "twilight")
	if first.Rules[0].RuleID != second.Rules[0].RuleID || first.Rules[0].RuleContentDigest != second.Rules[0].RuleContentDigest {
		t.Fatalf("normalization is not deterministic: %#v vs %#v", first.Rules[0], second.Rules[0])
	}

	// Same classification, different wording: identity must not move, content
	// version must.
	system["takeover_rules"] = []any{
		map[string]any{"id": "public-challenge", "kind": "turn_takeover", "version": personaTakeoverRuleVersion, "condition": "当用户当面强烈质疑时接管", "target_profile_id": "spark"},
	}
	edited := normalizePersonaSwitchRules(persona, nil, "twilight")
	if edited.Rules[0].RuleID != first.Rules[0].RuleID {
		t.Fatalf("rule identity must not depend on the condition text: %q vs %q", edited.Rules[0].RuleID, first.Rules[0].RuleID)
	}
	if edited.Rules[0].RuleContentDigest == first.Rules[0].RuleContentDigest {
		t.Fatal("editing the condition text must change the content digest")
	}
}

func TestForcedActivationRuleIDIsStructuralNotTextual(t *testing.T) {
	base := map[string]any{"id": "public-challenge", "kind": "turn_takeover", "version": personaTakeoverRuleVersion, "condition": "遇到强烈的公开质疑时星火可强制接管。", "target_profile_id": "spark"}
	persona := multiProfileTestPersona()

	first := normalizePersonaSwitchRules(
		map[string]any{"personality_system": mergeMaps(mapValue(persona["personality_system"]), map[string]any{"forced_activation": base})},
		nil, "twilight",
	)
	takeovers := rulesOfKind(first.Rules, switchRuleTurnTakeover)
	if len(takeovers) != 1 || takeovers[0].TargetProfileID != "spark" {
		t.Fatalf("a declared kind=turn_takeover activation must produce one executable takeover rule for 星火: %#v", first.Rules)
	}
	if !strings.HasPrefix(takeovers[0].RuleID, "takeover:forced:") {
		t.Fatalf("forced activation rule id = %q", takeovers[0].RuleID)
	}

	rewritten := normalizePersonaSwitchRules(
		map[string]any{"personality_system": mergeMaps(mapValue(persona["personality_system"]), map[string]any{
			"forced_activation": map[string]any{"id": "public-challenge", "kind": "turn_takeover", "version": personaTakeoverRuleVersion, "condition": "在公开质疑非常强烈时由星火强制接管。", "target_profile_id": "spark"},
		})},
		nil, "twilight",
	)
	rewrittenTakeovers := rulesOfKind(rewritten.Rules, switchRuleTurnTakeover)
	if len(rewrittenTakeovers) != 1 {
		t.Fatalf("rewritten declared clause must stay executable: %#v", rewritten.Rules)
	}
	if rewrittenTakeovers[0].RuleID != takeovers[0].RuleID {
		t.Fatalf("structural identity must survive a wording change: %q vs %q", rewrittenTakeovers[0].RuleID, takeovers[0].RuleID)
	}
	if rewrittenTakeovers[0].RuleContentDigest == takeovers[0].RuleContentDigest {
		t.Fatal("rewritten wording must be tracked by the content digest")
	}
}

func mergeMaps(base, overlay map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range base {
		result[key] = value
	}
	for key, value := range overlay {
		result[key] = value
	}
	return result
}

func TestForcedActivationAcceptsProseObjectAndListShapes(t *testing.T) {
	clause := "遇到强烈的公开质疑时星火可强制接管。"
	for name, value := range map[string]any{
		"string":  clause,
		"object":  map[string]any{"kind": "turn_takeover", "condition": clause},
		"trigger": map[string]any{"kind": "turn_takeover", "triggers": []any{clause}},
		"nested":  map[string]any{"activation": map[string]any{"kind": "turn_takeover", "when": clause}},
		"list":    []any{map[string]any{"id": "forced-1", "kind": "turn_takeover", "version": personaTakeoverRuleVersion, "condition": clause, "target_profile_id": "spark"}},
	} {
		t.Run(name, func(t *testing.T) {
			persona := multiProfileTestPersona()
			mapValue(persona["personality_system"])["forced_activation"] = value
			normalization := normalizePersonaSwitchRules(persona, nil, "twilight")
			// A free-text string carries no declared kind, so it is reported
			// as unclassified and never executed (F-03). Structural legacy
			// shapes that carry only kind still fail closed because they lack
			// the version, stable id, and explicit target required by the typed
			// contract. Only the complete list shape is executable.
			if name == "string" {
				unclassified := rulesOfKind(normalization.Rules, switchRuleUnclassified)
				if len(unclassified) == 0 || !hasDiagnosticCode(normalization.Diagnostics, personaSwitchDiagnosticUnclassified) {
					t.Fatalf("a free-text activation without a declared kind must be reported as unclassified: %#v (diagnostics=%#v)", normalization.Rules, diagnosticCodes(normalization.Diagnostics))
				}
				return
			}
			if name != "list" {
				if len(filterTakeoverAcceptableRules(normalization.Rules)) != 0 || !hasDiagnosticCode(normalization.Diagnostics, personaSwitchDiagnosticVersionInvalid) || !hasDiagnosticCode(normalization.Diagnostics, personaSwitchDiagnosticTargetExplicit) {
					t.Fatalf("incomplete %s migration shape must fail closed: rules=%#v diagnostics=%#v", name, normalization.Rules, diagnosticCodes(normalization.Diagnostics))
				}
				return
			}
			takeovers := rulesOfKind(normalization.Rules, switchRuleTurnTakeover)
			if len(takeovers) != 1 || takeovers[0].TargetProfileID != "spark" || !takeovers[0].Enabled {
				t.Fatalf("%s shape did not normalize into one executable takeover rule: %#v", name, normalization.Rules)
			}
		})
	}
}

func TestForcedActivationAmbiguousTargetIsReportedNotGuessed(t *testing.T) {
	persona := multiProfileTestPersona()
	mapValue(persona["personality_system"])["forced_activation"] = map[string]any{
		"condition": "暮光和星火都可以强制接管。",
	}
	normalization := normalizePersonaSwitchRules(persona, nil, "twilight")
	if takeovers := rulesOfKind(normalization.Rules, switchRuleTurnTakeover); len(takeovers) != 0 {
		t.Fatalf("an ambiguous target must never become an executable rule: %#v", takeovers)
	}
	if !hasDiagnosticCode(normalization.Diagnostics, personaSwitchDiagnosticTargetAmbiguous) {
		t.Fatalf("ambiguous target must be reported: %#v", diagnosticCodes(normalization.Diagnostics))
	}
}

func TestForcedActivationUnresolvedTargetIsReported(t *testing.T) {
	persona := multiProfileTestPersona()
	mapValue(persona["personality_system"])["forced_activation"] = map[string]any{
		"kind": "turn_takeover", "condition": "遇到公开质疑时接管", "target_profile_id": "导师人格",
	}
	normalization := normalizePersonaSwitchRules(persona, nil, "twilight")
	if takeovers := rulesOfKind(normalization.Rules, switchRuleTurnTakeover); len(takeovers) != 0 {
		if len(takeovers) != 1 || takeovers[0].Enabled || takeovers[0].TargetProfileID != "" {
			t.Fatalf("an unresolved target must never become an executable rule: %#v", takeovers)
		}
	}
	if !hasDiagnosticCode(normalization.Diagnostics, personaSwitchDiagnosticTargetUnresolved) {
		t.Fatalf("unresolved target must be reported: %#v", diagnosticCodes(normalization.Diagnostics))
	}
}

func TestTakeoverRuleWithUndeclaredTargetIsDisabledAndReported(t *testing.T) {
	persona := multiProfileTestPersona()
	mapValue(persona["personality_system"])["takeover_rules"] = []any{
		map[string]any{"id": "ghost", "kind": "turn_takeover", "version": personaTakeoverRuleVersion, "condition": "任何情况", "target_profile_id": "ghost-profile"},
	}
	normalization := normalizePersonaSwitchRules(persona, nil, "twilight")
	rule, ok := findRule(normalization.Rules, "takeover:ghost")
	if !ok || rule.Enabled || rule.TargetProfileID != "" {
		t.Fatalf("a takeover rule pointing at an undeclared profile must be disabled: %#v ok=%t", rule, ok)
	}
	if !hasDiagnosticCode(normalization.Diagnostics, personaSwitchDiagnosticTargetUnresolved) {
		t.Fatalf("undeclared takeover target must be reported: %#v", diagnosticCodes(normalization.Diagnostics))
	}
}

func TestMalformedTypedTakeoverEntryIsDiagnosedRatherThanDropped(t *testing.T) {
	persona := multiProfileTestPersona()
	mapValue(persona["personality_system"])["takeover_rules"] = []any{nil}
	normalization := normalizePersonaSwitchRules(persona, nil, "twilight")
	if len(normalization.Rules) != 1 {
		t.Fatalf("a malformed typed entry must remain represented in normalization: %#v", normalization.Rules)
	}
	rule := normalization.Rules[0]
	if rule.Enabled || rule.Kind != switchRuleUnclassified {
		t.Fatalf("a malformed typed entry must be disabled and unclassified: %#v", rule)
	}
	for _, code := range []string{personaSwitchDiagnosticRuleIDMissing, personaSwitchDiagnosticKindInvalid, personaSwitchDiagnosticVersionInvalid, personaSwitchDiagnosticFieldMissing, personaSwitchDiagnosticTargetExplicit} {
		if !hasDiagnosticCode(normalization.Diagnostics, code) {
			t.Fatalf("malformed typed entry missing %q diagnostic: %#v", code, normalization.Diagnostics)
		}
	}
}

func TestUnclassifiedForcedActivationKeepsRawValueAndIsReported(t *testing.T) {
	persona := multiProfileTestPersona()
	declared := map[string]any{"condition": "星火在雨天更愿意说话。"}
	mapValue(persona["personality_system"])["forced_activation"] = declared
	normalization := normalizePersonaSwitchRules(persona, nil, "twilight")

	unclassified := rulesOfKind(normalization.Rules, switchRuleUnclassified)
	if len(unclassified) != 1 {
		t.Fatalf("expected exactly one unclassified rule: %#v", normalization.Rules)
	}
	if unclassified[0].Enabled || unclassified[0].Kind != switchRuleUnclassified {
		t.Fatalf("unclassified rules are never executable: %#v", unclassified[0])
	}
	if stringValue(mapValue(unclassified[0].Raw)["condition"]) != "星火在雨天更愿意说话。" {
		t.Fatalf("the original declared value must be preserved verbatim: %#v", unclassified[0].Raw)
	}
	if !hasDiagnosticCode(normalization.Diagnostics, personaSwitchDiagnosticUnclassified) {
		t.Fatalf("unclassified material must be reported: %#v", diagnosticCodes(normalization.Diagnostics))
	}
}

func TestNormalizationNeverMutatesTheDeclaredPersona(t *testing.T) {
	persona := multiProfileTestPersona()
	mapValue(persona["personality_system"])["forced_activation"] = map[string]any{
		"condition": "遇到强烈的公开质疑时星火可强制接管。",
	}
	mapValue(persona["personality_system"])["takeover_rules"] = []any{
		map[string]any{"id": "x", "kind": "turn_takeover", "version": personaTakeoverRuleVersion, "condition": "星火接管", "target_profile_id": "spark"},
	}
	before := jsonString(persona)
	_ = normalizePersonaSwitchRules(persona, map[string]any{"active_profile_id": "twilight"}, "twilight")
	if after := jsonString(persona); after != before {
		t.Fatalf("normalization mutated the declared persona:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestResolvePersistentSwitchGrantScenarioTable(t *testing.T) {
	rules := []personaSwitchRule{
		{RuleID: "switch:night", Source: personaSwitchSourceSwitching},
		{RuleID: "takeover:public", Source: personaSwitchSourceTakeoverRules, Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion},
	}
	scope := turnPersonaScope{ActiveProfileID: "twilight", ReplyOwnerProfileID: "twilight"}

	cases := []struct {
		scenario  string
		allowed   bool
		reason    string
		declaredN int
	}{
		{persistentSwitchGrantScenarioMain, true, "", 1},
		{persistentSwitchGrantScenarioTakeover, false, "takeover_reply_owner", 0},
		{persistentSwitchGrantScenarioJudge, false, "judge_has_no_persistent_switch_field", 0},
		{"wake_up", false, "scenario_not_authorized", 0},
		{"reflection", false, "scenario_not_authorized", 0},
	}
	for _, testCase := range cases {
		grant := resolvePersistentSwitchGrant(scope, testCase.scenario, rules)
		if grant.Allowed != testCase.allowed || grant.Reason != testCase.reason || len(grant.DeclaredRules) != testCase.declaredN {
			t.Fatalf("scenario %q => %#v", testCase.scenario, grant)
		}
	}

	// An instance that declares no switching entry cannot propose a persistent
	// switch even in the main scenario.
	bare := resolvePersistentSwitchGrant(scope, persistentSwitchGrantScenarioMain, nil)
	if bare.Allowed || bare.Reason != "no_declared_switching_entry" {
		t.Fatalf("no declared switching entry => %#v", bare)
	}
}

func TestTakeoverReplyIsDeniedRegardlessOfDeclaredRules(t *testing.T) {
	rules := []personaSwitchRule{
		{RuleID: "switch:night", Source: personaSwitchSourceSwitching},
		{RuleID: "switch:default", Source: personaSwitchSourceSwitchingDefault},
	}
	grant := resolvePersistentSwitchGrant(turnPersonaScope{ActiveProfileID: "twilight", ReplyOwnerProfileID: "spark"}, persistentSwitchGrantScenarioTakeover, rules)
	if grant.Allowed || grant.Reason != "takeover_reply_owner" || len(grant.DeclaredRules) != 0 {
		t.Fatalf("takeover reply must be structurally denied: %#v", grant)
	}
}

func TestSelectTurnTakeoverRuleEligibilityAndOrdering(t *testing.T) {
	scope := turnPersonaScope{ActiveProfileID: "twilight", ReplyOwnerProfileID: "twilight"}
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)

	rules := []personaSwitchRule{
		{RuleID: "takeover:low", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Enabled: true, TargetProfileID: "spark", Priority: 1},
		{RuleID: "takeover:high-b", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Enabled: true, TargetProfileID: "spark", Priority: 5},
		{RuleID: "takeover:high-a", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Enabled: true, TargetProfileID: "spark", Priority: 5},
		{RuleID: "takeover:disabled", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Enabled: false, TargetProfileID: "spark", Priority: 99},
		{RuleID: "takeover:self", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Enabled: true, TargetProfileID: "twilight", Priority: 100},
		{RuleID: "takeover:wrong-source", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Enabled: true, TargetProfileID: "spark", SourceProfileID: "spark", Priority: 100},
		{RuleID: "switch:night", Kind: switchRulePersistentSemantic, Enabled: true, TargetProfileID: "spark", Priority: 100},
	}
	selected, ok := selectTurnTakeoverRule(rules, scope, nil, now)
	if !ok || selected.RuleID != "takeover:high-a" {
		t.Fatalf("selection must be priority-desc then rule-id-asc: %#v ok=%t", selected, ok)
	}

	// A rule whose declared source profile is not the current reply owner never
	// fires for a different owner.
	sourceScoped := []personaSwitchRule{
		{RuleID: "takeover:from-spark", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Enabled: true, TargetProfileID: "twilight", SourceProfileID: "spark", Priority: 10},
	}
	if _, ok := selectTurnTakeoverRule(sourceScoped, scope, nil, now); ok {
		t.Fatal("a rule scoped to a different source profile must not fire for the current owner")
	}
	if selected, ok := selectTurnTakeoverRule(sourceScoped, turnPersonaScope{ActiveProfileID: "spark", ReplyOwnerProfileID: "spark"}, nil, now); !ok || selected.RuleID != "takeover:from-spark" {
		t.Fatalf("a rule scoped to the current owner must fire: %#v ok=%t", selected, ok)
	}

	cooldown := now.Add(5 * time.Minute)
	if _, ok := selectTurnTakeoverRule(rules, scope, &cooldown, now); ok {
		t.Fatal("an active runtime cooldown must block takeover")
	}
	expired := now.Add(-time.Minute)
	if _, ok := selectTurnTakeoverRule(rules, scope, &expired, now); !ok {
		t.Fatal("an expired cooldown must not block takeover")
	}

	noTakeover := []personaSwitchRule{{RuleID: "switch:night", Kind: switchRulePersistentSemantic, Enabled: true, TargetProfileID: "spark"}}
	if _, ok := selectTurnTakeoverRule(noTakeover, scope, nil, now); ok {
		t.Fatal("persistent rules are not takeover rules")
	}
}

func TestPersistentSwitchPromptSectionNeverLeaksTakeoverRules(t *testing.T) {
	rules := []personaSwitchRule{
		{RuleID: "switch:night", Source: personaSwitchSourceSwitching, Condition: "夜间由暮光主导", TargetProfileID: "twilight"},
		{RuleID: "takeover:public", Source: personaSwitchSourceTakeoverRules, Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Condition: "公开质疑时星火接管", TargetProfileID: "spark"},
	}
	scope := turnPersonaScope{ActiveProfileID: "twilight", ReplyOwnerProfileID: "twilight"}
	grant := resolvePersistentSwitchGrant(scope, persistentSwitchGrantScenarioMain, rules)
	section := persistentSwitchPromptSection(scope, grant, rules)
	if len(section) == 0 {
		t.Fatal("an authorized scenario with declared rules must render a persistent switch section")
	}
	rendered := jsonString(section)
	if strings.Contains(rendered, "takeover") || strings.Contains(rendered, "public") || strings.Contains(rendered, "spark") {
		t.Fatalf("main prompt section must not contain takeover material: %s", rendered)
	}
	if !strings.Contains(rendered, "switch:night") {
		t.Fatalf("declared switching rules must be present: %s", rendered)
	}

	// A denied scenario renders nothing at all.
	denied := persistentSwitchPromptSection(scope, resolvePersistentSwitchGrant(scope, persistentSwitchGrantScenarioTakeover, rules), rules)
	if len(denied) != 0 {
		t.Fatalf("an unauthorized scenario must render no persistent switch section: %#v", denied)
	}
}

func TestFilterTakeoverAcceptableRulesKeepsOnlyExecutableTakeovers(t *testing.T) {
	rules := []personaSwitchRule{
		{RuleID: "takeover:ok", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Enabled: true, TargetProfileID: "spark"},
		{RuleID: "takeover:off", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Enabled: false, TargetProfileID: "spark"},
		{RuleID: "takeover:no-target", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, Enabled: true},
		{RuleID: "switch:night", Kind: switchRulePersistentSemantic, Enabled: true, TargetProfileID: "spark"},
		{RuleID: "forced:x", Kind: switchRuleUnclassified, Enabled: false},
	}
	accepted := filterTakeoverAcceptableRules(rules)
	if len(accepted) != 1 || accepted[0].RuleID != "takeover:ok" {
		t.Fatalf("judge input must contain only executable takeover rules: %#v", accepted)
	}
}

// TestPersonaSwitchRulesNormalizeTheRealMultiProfileCard binds the normalizer
// to the repository's own dense multi-profile fixture. The live provider path
// that actually produces forced_activation requires FLUCTLIGHT_LIVE_PROVIDER_TEST=1
// and is skipped here, so this test proves the normalization contract against
// the fixture's real prose rather than against a hand-written ideal shape.
func TestPersonaSwitchRulesNormalizeTheRealMultiProfileCard(t *testing.T) {
	card, err := os.ReadFile("testdata/initialization/dense_multi_card.txt")
	if err != nil {
		t.Fatal(err)
	}
	manifestRaw, err := os.ReadFile("testdata/initialization/dense_multi_expectations.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest InitializationExpectationManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	var forcedContains string
	for _, assertion := range manifest.Assertions {
		if assertion.ID == "forced" {
			forcedContains = assertion.Contains
		}
	}
	if forcedContains == "" {
		t.Fatal("the manifest no longer declares the forced_activation expectation")
	}
	// Extract the real two-clause sentence from the card rather than inventing
	// one: "遇到强烈的公开质疑时星火可强制接管；收到 actor_user 明确的安全确认后暮光可重新主导。"
	cardText := string(card)
	if !strings.Contains(cardText, forcedContains) {
		t.Fatalf("fixture drift: the card no longer states the declared activation condition %q", forcedContains)
	}
	start := strings.Index(cardText, "遇到")
	end := strings.Index(cardText, "暮光可重新主导")
	if start < 0 || end < 0 {
		t.Fatal("fixture drift: the card no longer declares the takeover/re-dominance sentence")
	}
	sentence := cardText[start : end+len("暮光可重新主导。")]

	persona := map[string]any{
		"personality_system": map[string]any{
			"mode":              "multiple",
			"active_profile_id": "default",
			"profiles": []any{
				map[string]any{"id": "twilight", "name": "暮光"},
				map[string]any{"id": "spark", "name": "星火"},
			},
			"forced_activation": map[string]any{"condition": sentence},
		},
	}
	normalization := normalizePersonaSwitchRules(persona, nil, "default")
	// The real card declares forced_activation as prose with no typed `kind`,
	// so the normalizer reports both clauses as unclassified and preserves the
	// original value verbatim. No executable takeover is produced from prose
	// (F-03); a migration provider would have to emit typed rules first.
	takeovers := rulesOfKind(normalization.Rules, switchRuleTurnTakeover)
	if len(takeovers) != 0 {
		t.Fatalf("prose forced_activation must never produce an executable takeover: %#v", takeovers)
	}
	if !hasDiagnosticCode(normalization.Diagnostics, personaSwitchDiagnosticUnclassified) {
		t.Fatalf("unclassified prose must be reported: %#v (diagnostics=%#v)", normalization.Rules, diagnosticCodes(normalization.Diagnostics))
	}
	if len(normalization.Rules) == 0 {
		t.Fatalf("the prose clauses must be preserved as unclassified rules so a migration provider can read them: %#v", normalization.Rules)
	}
	for _, rule := range normalization.Rules {
		if rule.Kind != switchRuleUnclassified || rule.Enabled {
			t.Fatalf("prose rules are never executable: %#v", rule)
		}
	}

	// The migration/provider contract is a separate, fixed structured artifact.
	// It preserves the original prose while adding an explicit, versioned target;
	// Runtime must consume that typed rule and never infer the target from prose.
	migrationRaw, err := os.ReadFile("testdata/initialization/dense_multi_typed_migration.json")
	if err != nil {
		t.Fatal(err)
	}
	var migration map[string]any
	if err := json.Unmarshal(migrationRaw, &migration); err != nil {
		t.Fatal(err)
	}
	migrationSystem := mapValue(mapValue(migration["core_persona"])["personality_system"])
	if !strings.Contains(jsonString(migrationSystem["forced_activation"]), forcedContains) {
		t.Fatalf("typed migration fixture dropped the original forced_activation prose: %#v", migrationSystem["forced_activation"])
	}
	migrated := normalizePersonaSwitchRules(map[string]any{"personality_system": migrationSystem}, nil, "twilight")
	takeovers = filterTakeoverAcceptableRules(migrated.Rules)
	if len(takeovers) != 1 {
		t.Fatalf("typed migration fixture must produce exactly one executable takeover: rules=%#v diagnostics=%#v", migrated.Rules, migrated.Diagnostics)
	}
	rule := takeovers[0]
	if rule.RuleID != "takeover:dense-public-challenge" || rule.TargetProfileID != "spark" || rule.SourceProfileID != "twilight" || rule.DeclarationVersion != personaTakeoverRuleVersion || rule.Condition == "" {
		t.Fatalf("typed migration fixture normalized incorrectly: %#v", rule)
	}
	if !validInitializationTakeoverRules(arrayValue(migrationSystem["takeover_rules"]), map[string]struct{}{"twilight": {}, "spark": {}}) {
		t.Fatalf("typed migration fixture violates initialization takeover contract: %#v", migrationSystem["takeover_rules"])
	}
}

func TestPersonaSwitchRuleSetDigestIgnoresRuleOrder(t *testing.T) {
	left := []personaSwitchRule{
		{RuleID: "a", RuleContentDigest: "1"},
		{RuleID: "b", RuleContentDigest: "2"},
	}
	right := []personaSwitchRule{
		{RuleID: "b", RuleContentDigest: "2"},
		{RuleID: "a", RuleContentDigest: "1"},
	}
	if personaSwitchRuleSetDigest(left) != personaSwitchRuleSetDigest(right) {
		t.Fatal("the rule set digest must be order independent")
	}
	changed := []personaSwitchRule{
		{RuleID: "a", RuleContentDigest: "1"},
		{RuleID: "b", RuleContentDigest: "3"},
	}
	if personaSwitchRuleSetDigest(left) == personaSwitchRuleSetDigest(changed) {
		t.Fatal("a content change must change the rule set digest")
	}
}
