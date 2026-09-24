package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const personaCompilationRulesVersion = "working-persona.compile.v2"
const defaultWorkingPersonaMaxRunes = 3600

type PersonaCompilationInput struct {
	CorePersona         map[string]any
	ProfileID           string
	EffectivePersona    map[string]any
	SourceRevision      int
	OverlayRevision     int
	TargetBudgetRunes   int
	EffectiveLifeHabits *[]any
}

type PersonaPortraitFact struct {
	Category   string   `json:"category"`
	Text       string   `json:"text"`
	SourceRefs []string `json:"source_refs"`
}

type PersonaPortraitOmission struct {
	SourceRef string `json:"source_ref"`
	Reason    string `json:"reason"`
}

type CompiledWorkingPersona struct {
	ProfileID       string                    `json:"profile_id"`
	Facts           []PersonaPortraitFact     `json:"facts"`
	Omissions       []PersonaPortraitOmission `json:"omissions"`
	SourceRevision  int                       `json:"source_revision"`
	SourceHash      string                    `json:"source_hash"`
	OverlayRevision int                       `json:"overlay_revision"`
	RulesVersion    string                    `json:"rules_version"`
	BudgetRunes     int                       `json:"budget_runes"`
}

func personaCompilationResponseSchema(profileID string) map[string]any {
	fact := objectSchema(map[string]any{
		"category":    enumStringSchema("identity", "core_mechanisms", "language_expression", "behavior_boundaries", "stable_preferences"),
		"text":        map[string]any{"type": "string", "minLength": 1, "maxLength": 1000},
		"source_refs": arraySchema(map[string]any{"type": "string", "minLength": 1, "maxLength": 256}),
	}, []string{"category", "text", "source_refs"}, false)
	omission := objectSchema(map[string]any{
		"source_ref": map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
		"reason":     map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
	}, []string{"source_ref", "reason"}, false)
	var profileIDSchema map[string]any
	if trimmed := strings.TrimSpace(profileID); trimmed != "" {
		profileIDSchema = enumStringSchema(trimmed)
	} else {
		profileIDSchema = map[string]any{"type": "string", "minLength": 1, "maxLength": 128}
	}
	return objectSchema(map[string]any{
		"profile_id": profileIDSchema,
		"facts":      map[string]any{"type": "array", "minItems": 1, "maxItems": 80, "items": fact},
		"omissions":  map[string]any{"type": "array", "maxItems": 80, "items": omission},
	}, []string{"profile_id", "facts", "omissions"}, false)
}

const personaCompilationInstruction = `Compile one complete, validated persona profile into a short self portrait. Preserve identity, core interaction patterns and communication frequency (including proactive outreach, message density/bombardment tendencies, and initiation habits), behavior mechanisms, stable values, voice, boundaries, conditions, exceptions, and each explicit short stable preference. Remove repeated wording and story detail, while keeping causal behavior meaning. A preference is available self knowledge, not a current desire or completed action. Never invent traits, preferences, history, values, or universal mannerisms. Do not include current body or hair state, current mood, clothing, scene, schedule, temporary intention, or evolving relationship state. A preferred hairstyle is a preference; a currently worn hairstyle is a body state. Preserve shared identity and this profile's differences; never mix other profiles. Each fact must cite real dot-separated paths relative to the supplied source object. Array items use their zero-based index as a path component. Cite every separately declared preference/habit child path, or list that exact path and the reason in omissions; citing only a parent preferences/habits path is insufficient. If another distinct source fact must stay only in full detail due to budget, list its path and reason in omissions. Set profile_id in the output JSON to the exact profile_id string provided in the input, without translation, abbreviation, or substitution. Return only the specified JSON.`

func personaCompilationSource(input PersonaCompilationInput) (map[string]any, error) {
	profileID := strings.TrimSpace(input.ProfileID)
	if profileID == "" {
		return nil, errors.New("persona_compilation_profile_required")
	}
	core := input.CorePersona
	if len(core) == 0 {
		return nil, errors.New("persona_compilation_source_required")
	}
	profile := map[string]any{}
	profiles := arrayValue(mapValue(core["personality_system"])["profiles"])
	if len(profiles) > 0 {
		for _, raw := range profiles {
			candidate := mapValue(raw)
			if stringValue(candidate["id"]) == profileID {
				profile = cloneMap(candidate)
				break
			}
		}
		if len(profile) == 0 {
			return nil, fmt.Errorf("persona_compilation_profile_not_found: %s", profileID)
		}
	} else {
		profile = map[string]any{"id": profileID}
	}
	profile = safePersonaFactMap(profile)
	if identity := mapValue(profile["identity"]); len(identity) > 0 {
		profile["identity"] = stableCompilationIdentity(identity)
	}
	delete(profile, "emotional_state")
	delete(profile, "visual_identity")
	profile["id"] = profileID
	for _, key := range []string{"personality", "behavioral_policy"} {
		coreVal := safePersonaFactMap(mapValue(core[key]))
		profVal := safePersonaFactMap(mapValue(profile[key]))
		merged := cloneMap(coreVal)
		if merged == nil {
			merged = map[string]any{}
		}
		for k, v := range profVal {
			merged[k] = v
		}
		if input.OverlayRevision > 0 && len(mapValue(input.EffectivePersona[key])) > 0 {
			effectiveVal := safePersonaFactMap(mapValue(input.EffectivePersona[key]))
			for k, v := range effectiveVal {
				merged[k] = v
			}
		}
		profile[key] = merged
	}
	source := map[string]any{"profile": profile}
	if behavior := mapValue(core["behavioral_policy"]); len(behavior) > 0 {
		if stable := safePersonaFactMap(behavior); len(stable) > 0 {
			source["behavioral_policy"] = stable
		}
	}
	if personality := mapValue(core["personality"]); len(personality) > 0 {
		if stable := safePersonaFactMap(personality); len(stable) > 0 {
			source["personality"] = stable
		}
	}
	if identity := mapValue(core["identity"]); len(identity) > 0 {
		if stable := stableCompilationIdentity(identity); len(stable) > 0 {
			source["identity"] = stable
		}
	}
	if life := mapValue(core["life_profile"]); len(life) > 0 || input.EffectiveLifeHabits != nil {
		stableLife := map[string]any{}
		for _, key := range []string{"preferences", "life_habits", "character_constraints"} {
			if value, exists := life[key]; exists {
				stableLife[key] = value
			}
		}
		if input.EffectiveLifeHabits != nil {
			stableLife["life_habits"] = *input.EffectiveLifeHabits
		}
		if style := mapValue(mapValue(life["appearance"])["style_preferences"]); len(style) > 0 {
			stableLife["style_preferences"] = style
		}
		if stable := safePersonaFactMap(stableLife); len(stable) > 0 {
			source["life_profile"] = stable
		}
	}
	if extensions := mapValue(core["extensions"]); len(extensions) > 0 {
		if stable := safePersonaFactMap(extensions); len(stable) > 0 {
			source["extensions"] = stable
		}
	}
	if shared := sharedPersonalitySystemSource(mapValue(core["personality_system"])); len(shared) > 0 {
		source["shared_system"] = shared
	}
	return source, nil
}

func stableCompilationIdentity(identity map[string]any) map[string]any {
	stable := map[string]any{}
	for _, key := range []string{"name", "nickname", "gender", "occupation", "birthplace", "birthday", "core_values", "worldview"} {
		if value, exists := identity[key]; exists && value != nil {
			stable[key] = value
		}
	}
	return safePersonaFactMap(stable)
}

func sharedPersonalitySystemSource(system map[string]any) map[string]any {
	shared := map[string]any{}
	for _, key := range []string{"core_relationship", "core_conflict", "influence", "conflict_resolution", "integration", "behavior_state_machine", "extensions"} {
		value, exists := system[key]
		if !exists || value == nil {
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			value = safePersonaFactMap(nested)
			if len(mapValue(value)) == 0 {
				continue
			}
		}
		shared[key] = value
	}
	return shared
}

var transientPersonaSourceKeys = map[string]struct{}{
	"current_mood": {}, "current_emotion": {}, "current_outfit": {}, "current_clothing": {},
	"current_scene": {}, "current_activity": {}, "current_schedule": {}, "current_plan": {},
	"temporary_intent": {}, "today_schedule": {}, "initial_state": {}, "relationship_progress": {},
	"timezone": {},
	// Legacy free-form extensions may contain current body or wardrobe facts.
	// Keep stable rituals and declared preferences, but exclude mutable state
	// even when nested under an extension instead of its canonical domain.
	"appearance": {}, "physical_features": {}, "hair_length": {}, "hair_color": {}, "hair_style": {},
	"injuries": {}, "outfit": {}, "clothing": {}, "wardrobe": {}, "wardrobe_items": {}, "worn_items": {},
}

func stablePersonaSourceMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, child := range value {
		if _, transient := transientPersonaSourceKeys[key]; transient {
			continue
		}
		switch typed := child.(type) {
		case map[string]any:
			result[key] = stablePersonaSourceMap(typed)
		case []any:
			items := make([]any, len(typed))
			for index, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					items[index] = stablePersonaSourceMap(nested)
				} else {
					items[index] = item
				}
			}
			result[key] = items
		default:
			result[key] = child
		}
	}
	return result
}

func safePersonaFactMap(value map[string]any) map[string]any {
	return filterCorePersonaValue(stablePersonaSourceMap(value))
}

func (a *App) CompileWorkingPersona(ctx context.Context, input PersonaCompilationInput) (CompiledWorkingPersona, error) {
	if input.TargetBudgetRunes == 0 {
		if a == nil || a.DB == nil {
			return CompiledWorkingPersona{}, errors.New("persona_compilation_budget_unavailable")
		}
		budget, err := readWorkingPersonaBudget(ctx, a.DB.Pool())
		if err != nil {
			return CompiledWorkingPersona{}, err
		}
		input.TargetBudgetRunes = budget
	}
	source, err := personaCompilationSource(input)
	if err != nil {
		return CompiledWorkingPersona{}, err
	}
	messages := (&PromptComposer{}).ComposeTaskMessages("initialization", []map[string]any{
		{"role": "system", "content": personaCompilationInstruction},
		{"role": "user", "content": jsonString(map[string]any{"profile_id": input.ProfileID, "rules_version": personaCompilationRulesVersion, "target_max_runes": input.TargetBudgetRunes, "source": source})},
	})
	for attempt := 0; attempt < 2; attempt++ {
		runCtx := WithProviderScenario(ctx, "persona_compilation")
		if attempt > 0 {
			runCtx = WithProviderCorrelation(runCtx, firstString(providerCorrelation(ctx), "persona-compilation")+":repair")
		}
		run, runErr := a.runFormalStructuredTask(runCtx, FormalAgentPersonaCompilation, messages, nil, "persona_compilation_response", personaCompilationResponseSchema(input.ProfileID), false, nil)
		if runErr != nil {
			return CompiledWorkingPersona{}, runErr
		}
		compiled, validationErr := decodeCompiledWorkingPersona(run.Completion.Structured, source, input)
		if validationErr == nil {
			return compiled, nil
		}
		if attempt == 1 {
			return CompiledWorkingPersona{}, validationErr
		}
		messages = append(messages, map[string]any{"role": "user", "content": fmt.Sprintf("The prior portrait failed validation (%v). Rebuild the complete JSON from the original source, retaining each cited fact and exception. Do not add unsupported facts. Ensure profile_id is exactly %q.", validationErr, input.ProfileID)})
	}
	return CompiledWorkingPersona{}, errors.New("persona_compilation_retry_exhausted")
}

func decodeCompiledWorkingPersona(output, source map[string]any, input PersonaCompilationInput) (CompiledWorkingPersona, error) {
	outputProfileID := strings.TrimSpace(stringValue(output["profile_id"]))
	if outputProfileID != input.ProfileID {
		acceptable := false
		if outputProfileID == "" || strings.EqualFold(outputProfileID, input.ProfileID) {
			acceptable = true
		} else if trimmed := strings.TrimSuffix(input.ProfileID, "_main"); trimmed != "" && strings.EqualFold(outputProfileID, trimmed) {
			acceptable = true
		} else {
			identity := mapValue(source["identity"])
			name := strings.TrimSpace(stringValue(identity["name"]))
			nickname := strings.TrimSpace(stringValue(identity["nickname"]))
			profileName := strings.TrimSpace(stringValue(mapValue(source["profile"])["name"]))
			if (name != "" && (outputProfileID == name || strings.EqualFold(outputProfileID, name))) ||
				(nickname != "" && (outputProfileID == nickname || strings.EqualFold(outputProfileID, nickname))) ||
				(profileName != "" && (outputProfileID == profileName || strings.EqualFold(outputProfileID, profileName))) {
				acceptable = true
			}
		}
		if !acceptable {
			return CompiledWorkingPersona{}, fmt.Errorf("persona_compilation_profile_mismatch: expected %q, got %q", input.ProfileID, outputProfileID)
		}
	}
	var result CompiledWorkingPersona
	result.ProfileID = input.ProfileID
	result.SourceRevision = input.SourceRevision
	result.SourceHash = stableDigest(jsonString(source))
	result.OverlayRevision = input.OverlayRevision
	result.RulesVersion = personaCompilationRulesVersion
	result.BudgetRunes = input.TargetBudgetRunes
	if result.BudgetRunes == 0 {
		result.BudgetRunes = defaultWorkingPersonaMaxRunes
	}
	seen := make(map[string]struct{})
	for _, raw := range arrayValue(output["facts"]) {
		item := mapValue(raw)
		category := stringValue(item["category"])
		if !personaPortraitCategory(category) {
			return CompiledWorkingPersona{}, errors.New("persona_compilation_category_invalid")
		}
		statement := strings.TrimSpace(stringValue(item["text"]))
		if statement == "" || len([]rune(statement)) > 1000 {
			return CompiledWorkingPersona{}, errors.New("persona_compilation_fact_invalid")
		}
		refs := make([]string, 0)
		for _, value := range arrayValue(item["source_refs"]) {
			ref := strings.TrimSpace(stringValue(value))
			if !personaCompilationSourcePathExists(source, ref) {
				return CompiledWorkingPersona{}, fmt.Errorf("persona_compilation_ref_invalid: %s", ref)
			}
			refs = append(refs, ref)
			seen[ref] = struct{}{}
		}
		if len(refs) == 0 {
			return CompiledWorkingPersona{}, errors.New("persona_compilation_fact_without_source")
		}
		result.Facts = append(result.Facts, PersonaPortraitFact{Category: category, Text: statement, SourceRefs: refs})
	}
	if len(result.Facts) == 0 {
		return CompiledWorkingPersona{}, errors.New("persona_compilation_empty")
	}
	for _, raw := range arrayValue(output["omissions"]) {
		item := mapValue(raw)
		ref := strings.TrimSpace(stringValue(item["source_ref"]))
		reason := strings.TrimSpace(stringValue(item["reason"]))
		if !personaCompilationSourcePathExists(source, ref) || reason == "" {
			return CompiledWorkingPersona{}, errors.New("persona_compilation_omission_invalid")
		}
		result.Omissions = append(result.Omissions, PersonaPortraitOmission{SourceRef: ref, Reason: reason})
		seen[ref] = struct{}{}
	}
	// Every separately declared preference or habit must either be carried by
	// a fact or explicitly diagnosed as a deliberate omission. A parent ref is
	// insufficient because it could hide the loss of one short exception.
	for _, group := range []struct {
		path  string
		value map[string]any
	}{
		{"life_profile.preferences", mapValue(mapValue(source["life_profile"])["preferences"])},
		{"life_profile.style_preferences", mapValue(mapValue(source["life_profile"])["style_preferences"])},
		{"profile.preferences", mapValue(mapValue(source["profile"])["preferences"])},
		{"profile.habits", mapValue(mapValue(source["profile"])["habits"])},
	} {
		for key := range group.value {
			ref := group.path + "." + key
			covered := false
			for cited := range seen {
				if cited == ref || strings.HasPrefix(cited, ref+".") {
					covered = true
					break
				}
			}
			if !covered {
				return CompiledWorkingPersona{}, fmt.Errorf("persona_compilation_preference_unaccounted: %s", ref)
			}
		}
	}
	for index, habit := range arrayValue(mapValue(source["life_profile"])["life_habits"]) {
		if habit == nil {
			continue
		}
		ref := fmt.Sprintf("life_profile.life_habits.%d", index)
		covered := false
		for cited := range seen {
			if cited == ref || strings.HasPrefix(cited, ref+".") {
				covered = true
				break
			}
		}
		if !covered {
			return CompiledWorkingPersona{}, fmt.Errorf("persona_compilation_preference_unaccounted: %s", ref)
		}
	}
	if len([]rune(jsonString(renderCompiledWorkingPersona(result)))) > result.BudgetRunes {
		return CompiledWorkingPersona{}, errors.New("persona_compilation_over_budget")
	}
	return result, nil
}

func personaPortraitCategory(category string) bool {
	switch category {
	case "identity", "core_mechanisms", "language_expression", "behavior_boundaries", "stable_preferences":
		return true
	}
	return false
}

func personaCompilationSourcePathExists(source map[string]any, path string) bool {
	if path == "" {
		return false
	}
	var value any = source
	for _, part := range strings.Split(path, ".") {
		if values, ok := value.([]any); ok {
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(values) {
				return false
			}
			value = values[index]
			if value == nil {
				return false
			}
			continue
		}
		current := mapValue(value)
		if current == nil {
			return false
		}
		var ok bool
		value, ok = current[part]
		if !ok || value == nil {
			return false
		}
	}
	return true
}

func renderCompiledWorkingPersona(compiled CompiledWorkingPersona) map[string]any {
	body := map[string]any{"profile_id": compiled.ProfileID}
	for _, category := range []string{"identity", "core_mechanisms", "language_expression", "behavior_boundaries", "stable_preferences"} {
		lines := make([]string, 0)
		for _, fact := range compiled.Facts {
			if fact.Category == category {
				lines = append(lines, fact.Text)
			}
		}
		if len(lines) > 0 {
			body[category] = lines
		}
	}
	result := map[string]any{workingPersonaProfileIDKey: compiled.ProfileID, workingPersonaBodyKey: body}
	if identity, ok := body["identity"]; ok {
		result[workingPersonaSharedIdentityKey] = map[string]any{"identity": identity}
		delete(body, "identity")
	}
	return result
}
