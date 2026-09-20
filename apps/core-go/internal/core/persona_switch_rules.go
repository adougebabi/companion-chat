package core

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/personality"
)

const personaSwitchRuleSetVersion = personality.PersonaSwitchRuleSetVersion
const personaTakeoverRuleVersion = personality.PersonaTakeoverRuleVersion

type personaSwitchRuleKind = personality.PersonaSwitchRuleKind

const (
	switchRulePersistentDeterministic = personality.SwitchRulePersistentDeterministic
	switchRulePersistentSemantic      = personality.SwitchRulePersistentSemantic
	switchRuleTurnTakeover            = personality.SwitchRuleTurnTakeover
	switchRuleUnclassified            = personality.SwitchRuleUnclassified
)

const (
	personaSwitchSourceSwitching          = personality.PersonaSwitchSourceSwitching
	personaSwitchSourceSwitchingList      = personality.PersonaSwitchSourceSwitchingList
	personaSwitchSourceSwitchingDefault   = personality.PersonaSwitchSourceSwitchingDefault
	personaSwitchSourceTakeoverRules      = personality.PersonaSwitchSourceTakeoverRules
	personaSwitchSourceForcedActivation   = personality.PersonaSwitchSourceForcedActivation
	persistentSwitchGrantScenarioMain     = personality.PersistentSwitchGrantScenarioMain
	persistentSwitchGrantScenarioTakeover = personality.PersistentSwitchGrantScenarioTakeover
	persistentSwitchGrantScenarioQuery    = personality.PersistentSwitchGrantScenarioQuery
	persistentSwitchGrantScenarioJudge    = personality.PersistentSwitchGrantScenarioJudge
)

const (
	personaSwitchDiagnosticUnclassified       = personality.PersonaSwitchDiagnosticUnclassified
	personaSwitchDiagnosticTargetUnresolved   = personality.PersonaSwitchDiagnosticTargetUnresolved
	personaSwitchDiagnosticTargetAmbiguous    = personality.PersonaSwitchDiagnosticTargetAmbiguous
	personaSwitchDiagnosticRuleIDMissing      = personality.PersonaSwitchDiagnosticRuleIDMissing
	personaSwitchDiagnosticSourceUnresolved   = personality.PersonaSwitchDiagnosticSourceUnresolved
	personaSwitchDiagnosticTakeoverCooldown   = personality.PersonaSwitchDiagnosticTakeoverCooldown
	personaSwitchDiagnosticProfileIndexAbsent = personality.PersonaSwitchDiagnosticProfileIndexAbsent
	personaSwitchDiagnosticKindInvalid        = personality.PersonaSwitchDiagnosticKindInvalid
	personaSwitchDiagnosticVersionInvalid     = personality.PersonaSwitchDiagnosticVersionInvalid
	personaSwitchDiagnosticFieldMissing       = personality.PersonaSwitchDiagnosticFieldMissing
	personaSwitchDiagnosticTargetExplicit     = personality.PersonaSwitchDiagnosticTargetExplicit
	personaSwitchDiagnosticEnabledInvalid     = personality.PersonaSwitchDiagnosticEnabledInvalid
)

type personaSwitchRule = personality.PersonaSwitchRule
type personaSwitchDiagnostic = personality.PersonaSwitchDiagnostic
type personaSwitchNormalization = personality.PersonaSwitchNormalization
type persistentSwitchGrant = personality.PersistentSwitchGrant
type turnPersonaScope = personality.TurnPersonaScope


// ---------------------------------------------------------------------------
// Declaration parsing helpers
// ---------------------------------------------------------------------------

// personaForcedActivationClauseSplitPattern splits a free-text
// forced_activation leaf into clause-level diagnostics. The normalizer never
// infers a switch semantic from this text: a clause without an explicit,
// declared `kind` is reported as unclassified and preserved verbatim
// (F-03 / persona-layer-contract.md / fluctlight-cognitive-runtime.md).
var personaForcedActivationClauseSplitPattern = regexp.MustCompile(`[；;。\n\r]+`)

var personaForcedActivationBulletTrimPattern = regexp.MustCompile(`^[\-\*•·\d]+[\.\)、]?\s*`)

// Object keys that make a forced_activation child look like a declared rule
// rather than free prose. Their presence is structural recognition only; it
// never determines the rule kind.
var personaRuleLikeKeys = []string{
	"id", "condition", "trigger", "triggers", "when", "target_profile_id",
	"target_profile", "source_profile_id", "priority", "cooldown_seconds", "enabled",
}

var personaTargetProfileKeys = []string{
	"target_profile_id", "target_profile", "target", "profile_id", "profile", "to_profile_id",
}

var personaSourceProfileKeys = []string{"source_profile_id", "source_profile", "from_profile_id", "from_profile"}

func normalizePersonaSwitchKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func personaSwitchKeyMatched(key string, candidates []string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	for _, candidate := range candidates {
		if normalized == candidate {
			return true
		}
	}
	return false
}

func personaLookupString(value map[string]any, candidates []string) (string, bool) {
	for key, child := range value {
		if !personaSwitchKeyMatched(key, candidates) {
			continue
		}
		if text := strings.TrimSpace(stringValue(child)); text != "" {
			return text, true
		}
	}
	return "", false
}

// personaLookupBool returns the declared boolean and whether the key was
// present at all. A present but malformed value fails closed.
func personaLookupBool(value map[string]any, candidates []string) (result bool, present bool) {
	for key, child := range value {
		if !personaSwitchKeyMatched(key, candidates) {
			continue
		}
		flag, ok := child.(bool)
		if !ok {
			return false, true
		}
		return flag, true
	}
	return false, false
}

func personaLookupNumber(value map[string]any, candidates []string) (float64, bool) {
	for key, child := range value {
		if !personaSwitchKeyMatched(key, candidates) {
			continue
		}
		if number, ok := numberFloat(child); ok {
			return number, true
		}
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// Profile index
// ---------------------------------------------------------------------------

type personaProfileIndex struct {
	ordered []string
	byKey   map[string]string
	names   map[string]string
}

// corePersonaData unwraps the Core Persona envelope the context projection
// carries. The projection stores the persisted Core Persona as
// {"authority":"hard_constraint","data":<core_persona>} so the Working Memory
// keeps the authority marker; every consumer that reads the *declared persona
// semantics* must therefore unwrap first. Reading the envelope directly yields
// an empty persona, which silently disables the whole persona-switch surface.
//
// A raw Core Persona has no authority marker, so it is returned unchanged and
// the helper is idempotent.
func corePersonaData(corePersona map[string]any) map[string]any {
	if strings.TrimSpace(stringValue(corePersona["authority"])) == "" {
		return corePersona
	}
	if data := mapValue(corePersona["data"]); len(data) > 0 {
		return data
	}
	return corePersona
}

func buildPersonaProfileIndex(corePersona map[string]any) personaProfileIndex {
	index := personaProfileIndex{byKey: map[string]string{}, names: map[string]string{}}
	system := mapValue(corePersonaData(corePersona)["personality_system"])
	for _, raw := range arrayValue(system["profiles"]) {
		profile := mapValue(raw)
		id := strings.TrimSpace(stringValue(profile["id"]))
		name := strings.TrimSpace(stringValue(profile["name"]))
		if id == "" {
			continue
		}
		if _, exists := index.byKey[normalizePersonaSwitchKey(id)]; !exists {
			index.ordered = append(index.ordered, id)
		}
		index.byKey[normalizePersonaSwitchKey(id)] = id
		if name != "" {
			index.names[id] = name
			if _, exists := index.byKey[normalizePersonaSwitchKey(name)]; !exists {
				index.byKey[normalizePersonaSwitchKey(name)] = id
			}
		}
	}
	if active := strings.TrimSpace(stringValue(system["active_profile_id"])); active == "default" {
		// "default" is a virtual shared profile. It is never a takeover target.
		if _, exists := index.byKey["default"]; !exists {
			index.byKey["default"] = "default"
			index.ordered = append(index.ordered, "default")
		}
	}
	return index
}

func (index personaProfileIndex) resolve(reference string) (string, bool) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", false
	}
	id, ok := index.byKey[normalizePersonaSwitchKey(reference)]
	return id, ok
}

func (index personaProfileIndex) declaredSet() map[string]struct{} {
	result := make(map[string]struct{}, len(index.ordered))
	for _, id := range index.ordered {
		result[id] = struct{}{}
	}
	return result
}

func isASCIIText(value string) bool {
	for _, character := range value {
		if character > 127 {
			return false
		}
	}
	return true
}

// personaTextNamesProfile reports whether a natural-language clause refers to
// one declared profile. ASCII identifiers shorter than three characters are
// ignored because they match unrelated substrings (for example an identifier
// such as "a" inside "actor_user").
func personaTextNamesProfile(clause, reference string) bool {
	reference = strings.TrimSpace(reference)
	if reference == "" || clause == "" {
		return false
	}
	if !isASCIIText(reference) {
		return strings.Contains(clause, reference)
	}
	if len(reference) < 3 {
		return false
	}
	pattern := regexp.MustCompile(`(?i)(^|[^a-z0-9_])` + regexp.QuoteMeta(reference) + `([^a-z0-9_]|$)`)
	return pattern.MatchString(clause)
}

// profileIDsNamedInText returns the declared profile identifiers a clause
// mentions, in declaration order.
func profileIDsNamedInText(clause string, index personaProfileIndex) []string {
	result := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, id := range index.ordered {
		if _, exists := seen[id]; exists {
			continue
		}
		if personaTextNamesProfile(clause, id) {
			seen[id] = struct{}{}
			result = append(result, id)
			continue
		}
		if name := index.names[id]; personaTextNamesProfile(clause, name) {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Normalization
// ---------------------------------------------------------------------------

type personaRuleCandidate struct {
	source     string
	path       string
	index      int
	object     map[string]any
	clause     string
	declaredID string
	ruleLike   bool
}

// normalizePersonaSwitchRules is the single entry point that turns the declared
// persona-switch surface into one normalized rule list. It never mutates or
// drops declared material: everything it cannot execute becomes either an
// explicitly disabled rule or a diagnostic.
func normalizePersonaSwitchRules(corePersona, runtime map[string]any, subjectProfileID string) personaSwitchNormalization {
	result := personaSwitchNormalization{Version: personaSwitchRuleSetVersion}
	index := buildPersonaProfileIndex(corePersona)
	result.DeclaredProfileIDs = index.declaredSet()
	result.DeclaredProfileIDsOrdered = append([]string(nil), index.ordered...)
	if len(index.ordered) == 0 {
		result.Diagnostics = append(result.Diagnostics, personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticProfileIndexAbsent, Path: "core_persona.personality_system.profiles",
			Detail: "the persona declares no profile, so no persona-switch rule can be resolved",
		})
	}
	subject := strings.TrimSpace(subjectProfileID)
	if subject == "" {
		subject = strings.TrimSpace(stringValue(mapValue(runtime)["active_profile_id"]))
	}
	system := mapValue(corePersonaData(corePersona)["personality_system"])

	for _, candidate := range personaSwitchingCandidates(system) {
		appendPersistentCandidate(&result, candidate, subject, index)
	}
	for _, candidate := range personaTakeoverRuleCandidates(system) {
		appendTakeoverCandidate(&result, candidate, index)
	}
	appendDefaultProfileCandidate(&result, system, index)
	appendForcedActivationCandidates(&result, system, index)

	sort.SliceStable(result.Rules, func(left, right int) bool {
		if result.Rules[left].Priority != result.Rules[right].Priority {
			return result.Rules[left].Priority > result.Rules[right].Priority
		}
		return result.Rules[left].RuleID < result.Rules[right].RuleID
	})
	return result
}

func appendPersistentCandidate(value *personaSwitchNormalization, candidate personaRuleCandidate, subject string, index personaProfileIndex) {
	condition := strings.TrimSpace(candidate.clause)
	target := strings.TrimSpace(stringValue(candidate.object["target_profile_id"]))
	resolvedTarget := ""
	enabled := true
	if target != "" {
		if id, ok := index.resolve(target); ok {
			resolvedTarget = id
		} else {
			value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
				Code: personaSwitchDiagnosticTargetUnresolved, Source: candidate.source, Path: candidate.path,
				Excerpt: boundedRuleExcerpt(target),
				Detail:  "a declared switching rule names a target profile that the persona does not declare",
			})
			enabled = false
		}
	}
	ruleID := ""
	if candidate.declaredID != "" {
		ruleID = "switch:" + candidate.declaredID
	} else {
		value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticRuleIDMissing, Source: candidate.source, Path: candidate.path,
			Excerpt: boundedRuleExcerpt(condition),
			Detail:  "a declared switching rule has no stable identifier; a positional identifier was derived",
		})
		ruleID = "switch:index:" + strconv.Itoa(candidate.index)
	}
	source := candidate.source
	sourceProfile := firstNonEmpty(
		personaStringField(candidate.object, personaSourceProfileKeys),
		subject,
	)
	// A switching declaration is structurally persistent. The deterministic
	// kind is only ever honoured when the declaration carries an explicit,
	// valid `kind`; the normalizer never infers it from a time-window shape in
	// the condition text (F-03).
	persistentKind := switchRulePersistentSemantic
	if declared, ok := declaredSwitchRuleKind(candidate.object); ok {
		persistentKind = declared
	}
	rule := personaSwitchRule{
		RuleID:          ruleID,
		Kind:            persistentKind,
		Source:          source,
		SourceProfileID: sourceProfile,
		TargetProfileID: resolvedTarget,
		Condition:       condition,
		Priority:        personaRulePriority(candidate.object),
		CooldownSeconds: personaRuleCooldownSeconds(candidate.object),
		Enabled:         enabled && resolvedTarget != "",
		Raw:             cloneMap(candidate.object),
	}
	rule.RuleContentDigest = personaRuleContentDigest(rule)
	value.Rules = append(value.Rules, rule)
}

func appendTakeoverCandidate(value *personaSwitchNormalization, candidate personaRuleCandidate, index personaProfileIndex) {
	condition := strings.TrimSpace(candidate.clause)
	ruleID := ""
	valid := true
	if candidate.declaredID != "" {
		ruleID = "takeover:" + candidate.declaredID
	} else {
		value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticRuleIDMissing, Source: candidate.source, Path: candidate.path,
			Excerpt: boundedRuleExcerpt(condition),
			Detail:  "a typed takeover rule requires an explicit stable identifier",
		})
		ruleID = "takeover:index:" + strconv.Itoa(candidate.index)
		valid = false
	}
	kind, kindOK := declaredSwitchRuleKind(candidate.object)
	if !kindOK || kind != switchRuleTurnTakeover {
		value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticKindInvalid, Source: candidate.source, Path: candidate.path,
			Excerpt: boundedRuleExcerpt(stringValue(candidate.object["kind"])),
			Detail:  "a takeover_rules entry must declare kind=turn_takeover",
		})
		valid = false
	}
	declarationVersion, versionOK := personaTakeoverDeclarationVersion(candidate.object)
	if !versionOK || declarationVersion != personaTakeoverRuleVersion {
		value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticVersionInvalid, Source: candidate.source, Path: candidate.path,
			Excerpt: boundedRuleExcerpt(stringValue(candidate.object["version"])),
			Detail:  "a takeover_rules entry must declare the supported typed rule version",
		})
		valid = false
	}
	if condition == "" {
		value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticFieldMissing, Source: candidate.source, Path: candidate.path + ".condition",
			Detail: "a typed takeover rule requires a non-empty condition",
		})
		valid = false
	}
	// A formal takeover target is a stable profile ID, not a name or a profile
	// mentioned in condition prose. The latter is useful to a migration operator
	// but is never allowed to become an execution target in the Runtime.
	explicitTarget := strings.TrimSpace(stringValue(candidate.object["target_profile_id"]))
	target := ""
	if explicitTarget == "" {
		value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticTargetExplicit, Source: candidate.source, Path: candidate.path + ".target_profile_id",
			Excerpt: boundedRuleExcerpt(condition),
			Detail:  "a typed takeover rule requires an explicit declared target_profile_id; condition text is not resolved",
		})
		valid = false
	} else if resolved, ok := resolveDeclaredProfileID(index, explicitTarget); ok {
		target = resolved
	} else {
		value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticTargetUnresolved, Source: candidate.source, Path: candidate.path + ".target_profile_id",
			Excerpt: boundedRuleExcerpt(explicitTarget),
			Detail:  "the declared takeover target ID does not match any declared profile",
		})
		valid = false
	}
	enabled, hasEnabledFlag := personaLookupBool(candidate.object, []string{"enabled"})
	if !hasEnabledFlag {
		enabled = true
	} else if _, ok := candidate.object["enabled"].(bool); !ok {
		value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticEnabledInvalid, Source: candidate.source, Path: candidate.path + ".enabled",
			Detail: "a typed takeover rule enabled value must be boolean",
		})
		valid = false
	}
	sourceProfile := personaStringField(candidate.object, personaSourceProfileKeys)
	if sourceProfile != "" {
		if id, ok := resolveDeclaredProfileID(index, sourceProfile); ok {
			sourceProfile = id
		} else {
			value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
				Code: personaSwitchDiagnosticSourceUnresolved, Source: candidate.source, Path: candidate.path,
				Excerpt: boundedRuleExcerpt(sourceProfile),
				Detail:  "a declared takeover rule names a source profile that the persona does not declare",
			})
			valid = false
			sourceProfile = ""
		}
	}
	rule := personaSwitchRule{
		RuleID:             ruleID,
		DeclarationVersion: declarationVersion,
		Kind:               kind,
		Source:             candidate.source,
		SourceProfileID:    sourceProfile,
		TargetProfileID:    target,
		Condition:          condition,
		Priority:           personaRulePriority(candidate.object),
		CooldownSeconds:    personaRuleCooldownSeconds(candidate.object),
		Enabled:            valid && enabled && target != "",
		Raw:                cloneMap(candidate.object),
	}
	if !kindOK {
		rule.Kind = switchRuleUnclassified
	}
	rule.RuleContentDigest = personaRuleContentDigest(rule)
	value.Rules = append(value.Rules, rule)
}

func appendDefaultProfileCandidate(value *personaSwitchNormalization, system map[string]any, index personaProfileIndex) {
	defaultProfile := strings.TrimSpace(stringValue(mapValue(system["switching"])["default_profile_id"]))
	if defaultProfile == "" {
		return
	}
	resolved, ok := index.resolve(defaultProfile)
	if !ok {
		value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticTargetUnresolved, Source: personaSwitchSourceSwitchingDefault,
			Path: "core_persona.personality_system.switching.default_profile_id", RuleID: "switch:default",
			Excerpt: boundedRuleExcerpt(defaultProfile),
			Detail:  "switching.default_profile_id does not match any declared profile",
		})
	}
	rule := personaSwitchRule{
		RuleID: "switch:default", Kind: switchRulePersistentSemantic, Source: personaSwitchSourceSwitchingDefault,
		TargetProfileID: resolved, Condition: "declared default dominant profile",
		Enabled: ok && resolved != "",
		Raw:     map[string]any{"default_profile_id": defaultProfile},
	}
	rule.RuleContentDigest = personaRuleContentDigest(rule)
	value.Rules = append(value.Rules, rule)
}

func appendForcedActivationCandidates(value *personaSwitchNormalization, system map[string]any, index personaProfileIndex) {
	raw, exists := system["forced_activation"]
	if !exists {
		return
	}
	for _, candidate := range personaForcedActivationCandidates(raw) {
		clause := strings.TrimSpace(candidate.clause)
		if clause == "" {
			continue
		}
		kind := switchRuleUnclassified
		kindDeclared := false
		if declared, ok := declaredSwitchRuleKind(candidate.object); ok {
			kind = declared
			kindDeclared = true
		}
		if kind == switchRuleTurnTakeover && candidate.declaredID == "" {
			value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
				Code: personaSwitchDiagnosticRuleIDMissing, Source: personaSwitchSourceForcedActivation,
				Path: candidate.path + ".id", Excerpt: boundedRuleExcerpt(clause),
				Detail: "a migrated takeover activation requires an explicit stable identifier",
			})
		}
		named := profileIDsNamedInText(clause, index)
		explicitTarget := strings.TrimSpace(stringValue(candidate.object["target_profile_id"]))
		// A clause that neither names a declared profile nor carries an explicit
		// target is not a persona rule at all. Report it as unclassified and keep
		// the source value untouched rather than inventing a rule.
		if explicitTarget == "" && len(named) == 0 {
			value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
				Code: personaSwitchDiagnosticUnclassified, Source: personaSwitchSourceForcedActivation,
				Path: candidate.path, Excerpt: boundedRuleExcerpt(clause),
				Detail: "a forced activation condition names no declared profile and declares no explicit target, so no executable rule was produced",
			})
			continue
		}
		target := ""
		if explicitTarget != "" {
			if id, ok := resolveDeclaredProfileID(index, explicitTarget); ok {
				target = id
			} else {
				value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
					Code: personaSwitchDiagnosticTargetUnresolved, Source: personaSwitchSourceForcedActivation,
					Path: candidate.path + ".target_profile_id", Excerpt: boundedRuleExcerpt(explicitTarget),
					Detail: "the declared activation target ID does not match any declared profile",
				})
			}
		} else if len(named) > 1 {
			value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
				Code: personaSwitchDiagnosticTargetAmbiguous, Source: personaSwitchSourceForcedActivation,
				Path: candidate.path, Excerpt: boundedRuleExcerpt(clause),
				Detail: "the condition names more than one declared profile: " + strings.Join(named, ","),
			})
		} else {
			// A profile name in legacy prose is migration evidence only. It cannot
			// become an execution target without an explicit stable ID supplied by
			// the authorized migration/provider output.
			value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
				Code: personaSwitchDiagnosticTargetExplicit, Source: personaSwitchSourceForcedActivation,
				Path: candidate.path + ".target_profile_id", Excerpt: boundedRuleExcerpt(clause),
				Detail: "legacy activation prose names one profile, but migration must provide an explicit target_profile_id",
			})
		}
		declarationVersion, versionOK := personaTakeoverDeclarationVersion(candidate.object)
		if kind == switchRuleTurnTakeover && (!versionOK || declarationVersion != personaTakeoverRuleVersion) {
			value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
				Code: personaSwitchDiagnosticVersionInvalid, Source: personaSwitchSourceForcedActivation,
				Path: candidate.path + ".version", Excerpt: boundedRuleExcerpt(stringValue(candidate.object["version"])),
				Detail: "a migrated takeover activation must declare the supported typed rule version",
			})
		}
		if kind == switchRuleUnclassified || !kindDeclared {
			value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
				Code: personaSwitchDiagnosticUnclassified, Source: personaSwitchSourceForcedActivation,
				Path: candidate.path, Excerpt: boundedRuleExcerpt(clause),
				Detail: "legacy activation prose is preserved; only an authorized typed migration may make it executable",
			})
		}
		sourceProfile := strings.TrimSpace(stringValue(candidate.object["source_profile_id"]))
		if sourceProfile != "" {
			if id, ok := resolveDeclaredProfileID(index, sourceProfile); ok {
				sourceProfile = id
			} else {
				value.Diagnostics = append(value.Diagnostics, personaSwitchDiagnostic{
					Code: personaSwitchDiagnosticSourceUnresolved, Source: personaSwitchSourceForcedActivation,
					Path: candidate.path + ".source_profile_id", Excerpt: boundedRuleExcerpt(sourceProfile),
					Detail: "the migrated activation source ID does not match any declared profile",
				})
				sourceProfile = ""
			}
		}
		rule := personaSwitchRule{
			RuleID:             forcedActivationRuleID(kind, candidate),
			DeclarationVersion: declarationVersion,
			Kind:               kind,
			Source:             personaSwitchSourceForcedActivation,
			SourceProfileID:    sourceProfile,
			TargetProfileID:    target,
			Condition:          clause,
			Priority:           personaRulePriority(candidate.object),
			CooldownSeconds:    personaRuleCooldownSeconds(candidate.object),
			Enabled:            kind == switchRuleTurnTakeover && kindDeclared && candidate.declaredID != "" && versionOK && declarationVersion == personaTakeoverRuleVersion && target != "",
			Raw:                cloneMap(rawAsObject(candidate.object)),
		}
		rule.RuleContentDigest = personaRuleContentDigest(rule)
		value.Rules = append(value.Rules, rule)
	}
}

// forcedActivationRuleID derives a stable identifier for a legacy
// forced_activation entry. The digest covers only the structural position and
// any declared identifier: editing the condition text changes the content
// digest, never the rule identity (design.md 3.4).
func forcedActivationRuleID(kind personaSwitchRuleKind, candidate personaRuleCandidate) string {
	digest := stableDigest("forced_activation\x1f" + strconv.Itoa(candidate.index) + "\x1f" + candidate.declaredID)
	switch kind {
	case switchRuleTurnTakeover:
		return "takeover:forced:" + digest
	case switchRulePersistentSemantic, switchRulePersistentDeterministic:
		return "switch:forced:" + digest
	default:
		return "forced:" + digest
	}
}

func rawAsObject(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

// ---------------------------------------------------------------------------
// Candidate collection
// ---------------------------------------------------------------------------

func personaSwitchingCandidates(system map[string]any) []personaRuleCandidate {
	switching := system["switching"]
	result := make([]personaRuleCandidate, 0, 2)
	appendRule := func(index int, path string, raw any) {
		object := mapValue(raw)
		if len(object) == 0 {
			return
		}
		result = append(result, personaRuleCandidate{
			source: personaSwitchSourceSwitching, path: path, index: index, object: object,
			clause: strings.TrimSpace(stringValue(object["condition"])), declaredID: strings.TrimSpace(stringValue(object["id"])),
		})
	}
	if list, ok := switching.([]any); ok {
		for position, raw := range list {
			object := mapValue(raw)
			if len(object) == 0 {
				continue
			}
			result = append(result, personaRuleCandidate{
				source: personaSwitchSourceSwitchingList, path: "core_persona.personality_system.switching[" + strconv.Itoa(position) + "]",
				index: position, object: object, clause: strings.TrimSpace(stringValue(object["condition"])),
				declaredID: strings.TrimSpace(stringValue(object["id"])),
			})
		}
		return result
	}
	for position, raw := range arrayValue(mapValue(switching)["rules"]) {
		appendRule(position, "core_persona.personality_system.switching.rules["+strconv.Itoa(position)+"]", raw)
	}
	return result
}

func personaTakeoverRuleCandidates(system map[string]any) []personaRuleCandidate {
	result := make([]personaRuleCandidate, 0, 2)
	for position, raw := range arrayValue(system["takeover_rules"]) {
		object := mapValue(raw)
		// Keep malformed entries in the candidate stream. The typed normalizer
		// will emit bounded missing-field diagnostics and a disabled rule rather
		// than silently dropping a declaration that may need migration.
		result = append(result, personaRuleCandidate{
			source: personaSwitchSourceTakeoverRules,
			path:   "core_persona.personality_system.takeover_rules[" + strconv.Itoa(position) + "]",
			index:  position, object: object, clause: strings.TrimSpace(stringValue(object["condition"])),
			declaredID: strings.TrimSpace(stringValue(object["id"])),
		})
	}
	return result
}

// personaForcedActivationCandidates flattens the open forced_activation value
// into clause-level candidates. The value has no declared schema, so it is
// walked generically: rule-like objects become one candidate each, and every
// remaining free-text leaf is split into clauses.
func personaForcedActivationCandidates(value any) []personaRuleCandidate {
	result := make([]personaRuleCandidate, 0, 2)
	position := 0
	var walk func(path string, current any, carrier map[string]any)
	emit := func(path string, object map[string]any, text string) {
		for _, clause := range personaSplitClauses(text) {
			result = append(result, personaRuleCandidate{
				source: personaSwitchSourceForcedActivation, path: path, index: position,
				object: object, clause: clause,
				declaredID: strings.TrimSpace(stringValue(object["id"])),
				ruleLike:   personaLooksLikeRule(object),
			})
			position++
		}
	}
	walk = func(path string, current any, carrier map[string]any) {
		switch typed := current.(type) {
		case string:
			emit(path, carrier, typed)
		case []any:
			for index, child := range typed {
				walk(path+"["+strconv.Itoa(index)+"]", child, carrier)
			}
		case map[string]any:
			if personaLooksLikeRule(typed) {
				text := firstNonEmpty(
					personaStringField(typed, []string{"condition"}),
					personaStringField(typed, []string{"trigger", "triggers", "when", "text", "description"}),
					personaJoinStringLeaves(typed),
				)
				if strings.TrimSpace(text) != "" {
					emit(path, typed, text)
					return
				}
				// A rule-like envelope whose text lives in a nested list (for
				// example {"triggers": [...]}) keeps its declared metadata while
				// the leaves below are walked individually.
				carrier = typed
			}
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				walk(path+"."+key, typed[key], carrier)
			}
		}
	}
	walk("core_persona.personality_system.forced_activation", value, map[string]any{})
	return result
}

func personaLooksLikeRule(value map[string]any) bool {
	for key := range value {
		if personaSwitchKeyMatched(key, personaRuleLikeKeys) {
			return true
		}
	}
	return false
}

func personaJoinStringLeaves(value map[string]any) string {
	parts := make([]string, 0, len(value))
	for _, key := range []string{"condition", "trigger", "when", "text", "description"} {
		if text := strings.TrimSpace(stringValue(value[key])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "；")
}

func personaSplitClauses(text string) []string {
	result := make([]string, 0, 2)
	for _, clause := range personaForcedActivationClauseSplitPattern.Split(text, -1) {
		clause = strings.TrimSpace(personaForcedActivationBulletTrimPattern.ReplaceAllString(strings.TrimSpace(clause), ""))
		if clause == "" {
			continue
		}
		result = append(result, clause)
	}
	return result
}

// ---------------------------------------------------------------------------
// Kind resolution
// ---------------------------------------------------------------------------

// declaredSwitchRuleKind reads the explicit `kind` field a declaration
// carries. The normalizer never infers a semantic kind from prose: a rule
// without a valid declared kind is unclassified and preserved verbatim, so
// only typed, versioned, authorized rules can drive a takeover
// (F-03 / persona-layer-contract.md / fluctlight-cognitive-runtime.md).
func declaredSwitchRuleKind(object map[string]any) (personaSwitchRuleKind, bool) {
	raw, ok := personaLookupString(object, []string{"kind"})
	if !ok || strings.TrimSpace(raw) == "" {
		return "", false
	}
	switch personaSwitchRuleKind(raw) {
	case switchRulePersistentDeterministic, switchRulePersistentSemantic, switchRuleTurnTakeover:
		return personaSwitchRuleKind(raw), true
	}
	return "", false
}

func personaRulePriority(object map[string]any) float64 {
	if value, ok := personaLookupNumber(object, []string{"priority"}); ok {
		return value
	}
	return 0
}

func personaRuleCooldownSeconds(object map[string]any) int {
	value, ok := personaLookupNumber(object, []string{"cooldown_seconds"})
	if !ok || value <= 0 {
		return 0
	}
	if value > 7*24*60*60 {
		return 7 * 24 * 60 * 60
	}
	return int(value)
}

func personaStringField(value map[string]any, candidates []string) string {
	text, _ := personaLookupString(value, candidates)
	return text
}

func personaTakeoverDeclarationVersion(object map[string]any) (string, bool) {
	version, ok := object["version"].(string)
	version = strings.TrimSpace(version)
	return version, ok && version != ""
}

// resolveDeclaredProfileID resolves only a stable, case-sensitive profile ID.
// Profile display names and natural-language mentions are intentionally not
// accepted for execution targets. They are useful migration diagnostics, but
// they are not an authorization boundary.
func resolveDeclaredProfileID(index personaProfileIndex, reference string) (string, bool) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", false
	}
	for _, id := range index.ordered {
		if id == reference {
			return id, true
		}
	}
	return "", false
}

// resolveCandidateTarget resolves a takeover target from the explicit stable
// profile ID field only. Condition prose is never promoted to an executable
// target, even when it happens to mention one profile name uniquely.
func resolveCandidateTarget(candidate personaRuleCandidate, index personaProfileIndex) (string, *personaSwitchDiagnostic) {
	if explicit := strings.TrimSpace(stringValue(candidate.object["target_profile_id"])); explicit != "" {
		if id, ok := resolveDeclaredProfileID(index, explicit); ok {
			return id, nil
		}
		return "", &personaSwitchDiagnostic{
			Code: personaSwitchDiagnosticTargetUnresolved, Source: candidate.source, Path: candidate.path,
			Excerpt: boundedRuleExcerpt(explicit),
			Detail:  "the declared takeover target ID does not match any declared profile",
		}
	}
	return "", &personaSwitchDiagnostic{
		Code: personaSwitchDiagnosticTargetExplicit, Source: candidate.source, Path: candidate.path + ".target_profile_id",
		Excerpt: boundedRuleExcerpt(candidate.clause),
		Detail:  "the takeover rule has no explicit target_profile_id; condition text is not resolved",
	}
}

func personaRuleContentDigest(rule personaSwitchRule) string {
	fingerprint := strings.Join([]string{
		string(rule.Kind), rule.DeclarationVersion, rule.Source, rule.TargetProfileID, rule.SourceProfileID,
		strconv.FormatFloat(rule.Priority, 'f', -1, 64), strconv.Itoa(rule.CooldownSeconds),
		strconv.FormatBool(rule.Enabled), strings.TrimSpace(rule.Condition),
	}, "\x1f")
	return stableDigest("persona-switch-rule\x1f" + fingerprint)
}

// personaSwitchRuleSetDigest is a deterministic cache key for a normalized rule
// set. Callers key their memo on it instead of re-deriving rules each turn.
func personaSwitchRuleSetDigest(rules []personaSwitchRule) string {
	parts := make([]string, 0, len(rules))
	for _, rule := range rules {
		parts = append(parts, rule.RuleID+"\x1e"+rule.RuleContentDigest)
	}
	sort.Strings(parts)
	return stableDigest(personaSwitchRuleSetVersion + "\x1f" + strings.Join(parts, "\x1d"))
}

func boundedRuleExcerpt(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	const limit = 160
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Authorization
// ---------------------------------------------------------------------------

// resolvePersistentSwitchGrant decides whether the model call being made may
// propose a persistent dominant-persona change. takeover_reply is denied
// unconditionally: the takeover speaker is structurally forbidden from writing
// the persistent active profile, regardless of what the persona declares.
func resolvePersistentSwitchGrant(scope turnPersonaScope, scenario string, rules []personaSwitchRule) persistentSwitchGrant {
	grant := persistentSwitchGrant{Scenario: strings.TrimSpace(scenario)}
	switch grant.Scenario {
	case persistentSwitchGrantScenarioTakeover:
		grant.Allowed = false
		grant.Reason = "takeover_reply_owner"
	case persistentSwitchGrantScenarioQuery:
		grant.Allowed = false
		grant.Reason = "query_continuation_owner"
	case persistentSwitchGrantScenarioJudge:
		grant.Allowed = false
		grant.Reason = "judge_has_no_persistent_switch_field"
	case persistentSwitchGrantScenarioMain:
		declared := make([]string, 0, len(rules))
		for _, rule := range rules {
			if strings.HasPrefix(rule.Source, "switching") {
				declared = append(declared, rule.RuleID)
			}
		}
		sort.Strings(declared)
		if len(declared) == 0 {
			grant.Allowed = false
			grant.Reason = "no_declared_switching_entry"
			return grant
		}
		grant.Allowed = true
		grant.DeclaredRules = declared
	default:
		grant.Allowed = false
		grant.Reason = "scenario_not_authorized"
	}
	return grant
}

// ---------------------------------------------------------------------------
// Takeover selection
// ---------------------------------------------------------------------------

// selectTurnTakeoverRule deterministically picks the single profile allowed to
// take over the current turn. The Judge never participates in this choice: it
// only answers whether the selected takeover is warranted (design.md 0.2).
func selectTurnTakeoverRule(rules []personaSwitchRule, scope turnPersonaScope, cooldownUntil *time.Time, now time.Time) (personaSwitchRule, bool) {
	owner := scope.ReplyOwner()
	eligible := make([]personaSwitchRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Kind != switchRuleTurnTakeover || rule.DeclarationVersion != personaTakeoverRuleVersion || !rule.Enabled {
			continue
		}
		if rule.TargetProfileID == "" || rule.TargetProfileID == owner {
			continue
		}
		if rule.SourceProfileID != "" && rule.SourceProfileID != owner {
			continue
		}
		eligible = append(eligible, rule)
	}
	if len(eligible) == 0 {
		return personaSwitchRule{}, false
	}
	if cooldownUntil != nil && !cooldownUntil.IsZero() && now.Before(cooldownUntil.UTC()) {
		return personaSwitchRule{}, false
	}
	sort.SliceStable(eligible, func(left, right int) bool {
		if eligible[left].Priority != eligible[right].Priority {
			return eligible[left].Priority > eligible[right].Priority
		}
		return eligible[left].RuleID < eligible[right].RuleID
	})
	return eligible[0], true
}

// takeoverCooldownActive reports whether an existing runtime cooldown blocks a
// takeover right now. It is separate from selection so the caller can record a
// precise diagnostic instead of a silent skip.
func takeoverCooldownActive(cooldownUntil *time.Time, now time.Time) bool {
	return cooldownUntil != nil && !cooldownUntil.IsZero() && now.Before(cooldownUntil.UTC())
}

// ---------------------------------------------------------------------------
// Prompt projection
// ---------------------------------------------------------------------------

// persistentSwitchPromptSection renders the only persistent-switch material the
// Main System Persona may contain. It deliberately omits takeover_rules: those
// are Judge-only input (design.md 4.6).
func persistentSwitchPromptSection(scope turnPersonaScope, grant persistentSwitchGrant, rules []personaSwitchRule) map[string]any {
	if !grant.Allowed {
		return nil
	}
	declared := make([]any, 0, len(rules))
	for _, rule := range rules {
		if !strings.HasPrefix(rule.Source, "switching") {
			continue
		}
		entry := map[string]any{"id": rule.RuleID, "condition": rule.Condition}
		if rule.TargetProfileID != "" {
			entry["target_profile_id"] = rule.TargetProfileID
		}
		if rule.CooldownSeconds > 0 {
			entry["cooldown_seconds"] = rule.CooldownSeconds
		}
		declared = append(declared, entry)
	}
	if len(declared) == 0 {
		return nil
	}
	return map[string]any{
		"active_profile_id": scope.ActiveProfileID,
		"authorized":        true,
		"scenario":          grant.Scenario,
		"rules":             declared,
	}
}

// filterTakeoverAcceptableRules drops every rule kind the Judge must not see.
func filterTakeoverAcceptableRules(rules []personaSwitchRule) []personaSwitchRule {
	result := make([]personaSwitchRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Kind == switchRuleTurnTakeover && rule.DeclarationVersion == personaTakeoverRuleVersion && rule.Enabled && rule.TargetProfileID != "" {
			result = append(result, rule)
		}
	}
	return result
}
