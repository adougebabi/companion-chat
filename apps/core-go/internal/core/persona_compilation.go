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
	PortraitText    string                    `json:"portrait_text,omitempty"`
	Facts           []PersonaPortraitFact     `json:"facts,omitempty"`
	Omissions       []PersonaPortraitOmission `json:"omissions,omitempty"`
	SourceRevision  int                       `json:"source_revision"`
	SourceHash      string                    `json:"source_hash"`
	OverlayRevision int                       `json:"overlay_revision"`
	RulesVersion    string                    `json:"rules_version"`
	BudgetRunes     int                       `json:"budget_runes"`
}

func personaCompilationResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"portrait_text": map[string]any{"type": "string", "minLength": 1, "maxLength": 12000},
	}, []string{"portrait_text"}, false)
}

const personaCompilationInstruction = `Write one concise Working Persona text for the profile selected by Core in the input. Preserve shared identity, this profile's interaction patterns and communication frequency, stable values, voice, boundaries, conditions, exceptions, and short stable preferences and habits. Keep causal behavior meaning while removing repeated wording and story detail. A preference is available self knowledge, not a current desire or completed action. Do not invent traits, preferences, history, values, or universal mannerisms. Do not include current body or hair state, mood, clothing, scene, schedule, temporary intention, or evolving relationship state. Never mix another profile into this text. Return a JSON object with exactly one field, portrait_text, containing the complete plain-text portrait. Do not return facts, source paths, omissions, or a profile identifier.`

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
		run, runErr := a.runFormalStructuredTask(runCtx, FormalAgentPersonaCompilation, messages, nil, "persona_compilation_response", personaCompilationResponseSchema(), false, nil)
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
		messages = append(messages, map[string]any{"role": "user", "content": fmt.Sprintf("The portrait text was unusable (%v). Return only a JSON object with one non-empty portrait_text string for the same supplied source and profile. Keep it within the stated rune budget; do not include fact arrays or source paths.", validationErr)})
	}
	return CompiledWorkingPersona{}, errors.New("persona_compilation_retry_exhausted")
}

func decodeCompiledWorkingPersona(output, source map[string]any, input PersonaCompilationInput) (CompiledWorkingPersona, error) {
	for _, key := range []string{"persona_compilation_response", "response", "data", "result"} {
		if nested := mapValue(output[key]); len(nested) > 0 {
			output = nested
			break
		}
	}
	result := CompiledWorkingPersona{
		ProfileID: input.ProfileID, SourceRevision: input.SourceRevision,
		SourceHash: stableDigest(jsonString(source)), OverlayRevision: input.OverlayRevision,
		RulesVersion: personaCompilationRulesVersion, BudgetRunes: input.TargetBudgetRunes,
	}
	if result.BudgetRunes == 0 {
		result.BudgetRunes = defaultWorkingPersonaMaxRunes
	}
	result.PortraitText = strings.TrimSpace(firstString(output["portrait_text"], stringValue(output["text"])))
	if result.PortraitText == "" {
		// Older controlled providers may still return categorized facts. Read
		// their text without treating model-authored source paths as authority.
		rawFacts := arrayValue(output["facts"])
		if len(rawFacts) == 0 {
			for _, key := range []string{"Facts", "portrait", "portrait_facts", "items", "statements"} {
				if values := arrayValue(output[key]); len(values) > 0 {
					rawFacts = values
					break
				}
			}
		}
		lines := make([]string, 0, len(rawFacts))
		for _, raw := range rawFacts {
			item := mapValue(raw)
			statement := strings.TrimSpace(firstString(item["text"], stringValue(item["statement"])))
			if statement == "" {
				continue
			}
			category := stringValue(item["category"])
			if !personaPortraitCategory(category) {
				category = "core_mechanisms"
			}
			fact := PersonaPortraitFact{Category: category, Text: statement}
			for _, rawRef := range arrayValue(item["source_refs"]) {
				if ref := resolvePersonaSourceRef(source, stringValue(rawRef)); ref != "" {
					fact.SourceRefs = append(fact.SourceRefs, ref)
				}
			}
			result.Facts = append(result.Facts, fact)
			lines = append(lines, statement)
		}
		result.PortraitText = strings.Join(lines, "\n")
	}
	if result.PortraitText == "" {
		return CompiledWorkingPersona{}, errors.New("persona_compilation_empty")
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

func resolvePersonaSourceRef(source map[string]any, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if personaCompilationSourcePathExists(source, ref) {
		return ref
	}
	// Try trimming known common hallucinated prefixes
	prefixes := []string{
		"extensions.core_persona.",
		"core_persona.extensions.",
		"core_persona.",
		"source.extensions.core_persona.",
		"source.core_persona.",
		"source.",
		"root.",
		"data.",
		"source_object.",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(ref, p) {
			candidate := strings.TrimPrefix(ref, p)
			if personaCompilationSourcePathExists(source, candidate) {
				return candidate
			}
			if personaCompilationSourcePathExists(source, "profile."+candidate) {
				return "profile." + candidate
			}
		}
	}
	// Try "profile." prefix if missing
	if !strings.HasPrefix(ref, "profile.") && personaCompilationSourcePathExists(source, "profile."+ref) {
		return "profile." + ref
	}
	// Try trimming "profile." prefix if present
	if strings.HasPrefix(ref, "profile.") && personaCompilationSourcePathExists(source, strings.TrimPrefix(ref, "profile.")) {
		return strings.TrimPrefix(ref, "profile.")
	}
	// Suffix path matching: if ref is a.b.c.d, check if any subpath exists in source
	parts := strings.Split(ref, ".")
	for i := 1; i < len(parts); i++ {
		candidate := strings.Join(parts[i:], ".")
		if personaCompilationSourcePathExists(source, candidate) {
			return candidate
		}
		if personaCompilationSourcePathExists(source, "profile."+candidate) {
			return "profile." + candidate
		}
	}
	return ""
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
	if text := strings.TrimSpace(compiled.PortraitText); text != "" {
		return map[string]any{workingPersonaProfileIDKey: compiled.ProfileID,
			workingPersonaBodyKey: map[string]any{"profile_id": compiled.ProfileID, "portrait_text": text}}
	}
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
