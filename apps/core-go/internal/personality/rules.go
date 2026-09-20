package personality

import (
	"sort"
	"strings"
	"time"
)

const PersonaSwitchRuleSetVersion = "persona-switch-rules.v1"
const PersonaTakeoverRuleVersion = "turn-takeover.v1"

type PersonaSwitchRuleKind string

const (
	SwitchRulePersistentDeterministic PersonaSwitchRuleKind = "persistent_deterministic"
	SwitchRulePersistentSemantic      PersonaSwitchRuleKind = "persistent_semantic"
	SwitchRuleTurnTakeover            PersonaSwitchRuleKind = "turn_takeover"
	SwitchRuleUnclassified            PersonaSwitchRuleKind = "unclassified"
)

const (
	PersonaSwitchSourceSwitching          = "switching.rules"
	PersonaSwitchSourceSwitchingList      = "switching"
	PersonaSwitchSourceSwitchingDefault   = "switching.default_profile_id"
	PersonaSwitchSourceTakeoverRules      = "takeover_rules"
	PersonaSwitchSourceForcedActivation   = "forced_activation"
	PersistentSwitchGrantScenarioMain     = "cognitive_assessment"
	PersistentSwitchGrantScenarioTakeover = "takeover_reply"
	PersistentSwitchGrantScenarioQuery    = "query_continuation"
	PersistentSwitchGrantScenarioJudge    = "takeover_judge"
)

const (
	PersonaSwitchDiagnosticUnclassified       = "unclassified"
	PersonaSwitchDiagnosticTargetUnresolved   = "target_profile_unresolved"
	PersonaSwitchDiagnosticTargetAmbiguous    = "target_profile_ambiguous"
	PersonaSwitchDiagnosticRuleIDMissing      = "switch_rule_id_missing"
	PersonaSwitchDiagnosticSourceUnresolved   = "source_profile_unresolved"
	PersonaSwitchDiagnosticTakeoverCooldown   = "takeover_cooldown_active"
	PersonaSwitchDiagnosticProfileIndexAbsent = "profile_index_empty"
	PersonaSwitchDiagnosticKindInvalid        = "takeover_rule_kind_invalid"
	PersonaSwitchDiagnosticVersionInvalid     = "takeover_rule_version_unsupported"
	PersonaSwitchDiagnosticFieldMissing       = "takeover_rule_field_missing"
	PersonaSwitchDiagnosticTargetExplicit     = "takeover_target_explicit_required"
	PersonaSwitchDiagnosticEnabledInvalid     = "takeover_rule_enabled_invalid"
)

type PersonaSwitchRule struct {
	RuleID             string
	RuleContentDigest  string
	DeclarationVersion string
	Kind               PersonaSwitchRuleKind
	Source             string
	SourceProfileID    string
	TargetProfileID    string
	Condition          string
	Priority           float64
	CooldownSeconds    int
	Enabled            bool
	Raw                map[string]any
}

type PersonaSwitchDiagnostic struct {
	Code    string
	Source  string
	Path    string
	RuleID  string
	Excerpt string
	Detail  string
}

func (value PersonaSwitchDiagnostic) AsMap() map[string]any {
	result := map[string]any{"code": value.Code}
	for key, child := range map[string]string{
		"source": value.Source, "path": value.Path, "rule_id": value.RuleID,
		"excerpt": value.Excerpt, "detail": value.Detail,
	} {
		if strings.TrimSpace(child) != "" {
			result[key] = child
		}
	}
	return result
}

type PersonaSwitchNormalization struct {
	Version                   string
	Rules                     []PersonaSwitchRule
	Diagnostics               []PersonaSwitchDiagnostic
	DeclaredProfileIDs        map[string]struct{}
	DeclaredProfileIDsOrdered []string
}

func (value PersonaSwitchNormalization) HasDeclaredPersistentSwitch() bool {
	for _, rule := range value.Rules {
		if strings.HasPrefix(rule.Source, "switching") {
			return true
		}
	}
	return false
}

func (value PersonaSwitchNormalization) DeclaredRuleIDs() []string {
	result := make([]string, 0, len(value.Rules))
	for _, rule := range value.Rules {
		if strings.HasPrefix(rule.Source, "switching") {
			result = append(result, rule.RuleID)
		}
	}
	sort.Strings(result)
	return result
}

type PersistentSwitchGrant struct {
	Allowed       bool
	Scenario      string
	DeclaredRules []string
	Reason        string
}

type TurnPersonaScope struct {
	ActiveProfileID     string
	ReplyOwnerProfileID string
	PersonaRevision     int
	OverlayRevision     int
	ScopeRevision       int
}

func (value TurnPersonaScope) ReplyOwner() string {
	if owner := strings.TrimSpace(value.ReplyOwnerProfileID); owner != "" {
		return owner
	}
	return strings.TrimSpace(value.ActiveProfileID)
}

func TakeoverCooldownActive(cooldownUntil *time.Time, now time.Time) bool {
	return cooldownUntil != nil && !cooldownUntil.IsZero() && now.Before(cooldownUntil.UTC())
}

func FilterTakeoverAcceptableRules(rules []PersonaSwitchRule) []PersonaSwitchRule {
	result := make([]PersonaSwitchRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Kind == SwitchRuleTurnTakeover && rule.DeclarationVersion == PersonaTakeoverRuleVersion && rule.Enabled && rule.TargetProfileID != "" {
			result = append(result, rule)
		}
	}
	return result
}

func SelectTurnTakeoverRule(rules []PersonaSwitchRule) (PersonaSwitchRule, bool) {
	eligible := FilterTakeoverAcceptableRules(rules)
	if len(eligible) == 0 {
		return PersonaSwitchRule{}, false
	}
	sort.SliceStable(eligible, func(left, right int) bool {
		if eligible[left].Priority != eligible[right].Priority {
			return eligible[left].Priority > eligible[right].Priority
		}
		return eligible[left].RuleID < eligible[right].RuleID
	})
	return eligible[0], true
}
