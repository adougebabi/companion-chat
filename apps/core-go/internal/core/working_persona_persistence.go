package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func declaredPersonaProfileIDs(corePersona map[string]any) []string {
	ids := make([]string, 0)
	for _, raw := range arrayValue(mapValue(corePersona["personality_system"])["profiles"]) {
		if id := strings.TrimSpace(stringValue(mapValue(raw)["id"])); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		ids = append(ids, initialPersonalityProfileID(corePersona))
	}
	sort.Strings(ids)
	return ids
}

func readWorkingPersonaBudget(ctx context.Context, query DBTX) (int, error) {
	var raw string
	err := query.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='working_persona_budget'`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultWorkingPersonaMaxRunes, nil
	}
	if err != nil {
		return 0, err
	}
	value := decodeObject([]byte(raw))
	budget := intValue(value["max_runes"])
	if budget < 512 || budget > 12000 {
		return 0, errors.New("working_persona_budget_invalid")
	}
	return budget, nil
}

func verifyCompiledBudgetTx(ctx context.Context, tx pgx.Tx, compiled []CompiledWorkingPersona) error {
	var raw string
	if err := tx.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='working_persona_budget' FOR SHARE`).Scan(&raw); err != nil {
		return err
	}
	budget := intValue(decodeObject([]byte(raw))["max_runes"])
	if budget < 512 || budget > 12000 {
		return errors.New("working_persona_budget_invalid")
	}
	for _, item := range compiled {
		if item.BudgetRunes != budget {
			return ErrConflict
		}
	}
	return nil
}

func (a *App) compileFoundationWorkingPersonas(ctx context.Context, fluctlightID string, corePersona map[string]any, sourceRevision int, mode string, loadOverlays bool) ([]CompiledWorkingPersona, error) {
	profiles := declaredPersonaProfileIDs(corePersona)
	compiled := make([]CompiledWorkingPersona, 0, len(profiles))
	for _, profileID := range profiles {
		input, err := a.personaCompilationInputForProfile(ctx, fluctlightID, corePersona, sourceRevision, profileID, loadOverlays)
		if err != nil {
			return nil, err
		}
		if loadOverlays {
			source, err := personaCompilationSource(input)
			if err != nil {
				return nil, err
			}
			var encoded []byte
			var savedHash, savedRules, savedStatus string
			var savedOverlay, savedBudget int
			err = a.DB.Pool().QueryRow(ctx, `SELECT source_hash,overlay_revision,rules_version,budget_runes,status,compiled_json FROM public.fluctlight_working_personas WHERE fluctlight_id=$1 AND profile_id=$2`, fluctlightID, profileID).Scan(&savedHash, &savedOverlay, &savedRules, &savedBudget, &savedStatus, &encoded)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
			if err == nil && savedHash == stableDigest(jsonString(source)) && savedOverlay == input.OverlayRevision && savedRules == personaCompilationRulesVersion && savedBudget == input.TargetBudgetRunes && savedStatus == "completed" {
				var previous CompiledWorkingPersona
				if err := json.Unmarshal(encoded, &previous); err != nil {
					return nil, err
				}
				previous.SourceRevision = sourceRevision
				compiled = append(compiled, previous)
				continue
			}
		}
		item, err := a.compileOneWorkingPersona(ctx, input, mode)
		if err != nil {
			return nil, fmt.Errorf("compile profile %s: %w", profileID, err)
		}
		compiled = append(compiled, item)
	}
	return compiled, nil
}

func (a *App) personaCompilationInputForProfile(ctx context.Context, fluctlightID string, corePersona map[string]any, sourceRevision int, profileID string, loadOverlays bool) (PersonaCompilationInput, error) {
	input := PersonaCompilationInput{CorePersona: corePersona, ProfileID: profileID, SourceRevision: sourceRevision}
	budget, err := readWorkingPersonaBudget(ctx, a.DB.Pool())
	if err != nil {
		return PersonaCompilationInput{}, err
	}
	input.TargetBudgetRunes = budget
	if !loadOverlays {
		return input, nil
	}
	baseline := personaEvolutionBaseline(fluctlightID, mapValue(corePersona["personality"]), mapValue(corePersona["behavioral_policy"]), mapValue(corePersona["personality_system"]), map[string]any{"active_profile_id": profileID})
	state, err := loadPersonaEvolutionState(ctx, a.DB.Pool(), baseline)
	if err != nil {
		return PersonaCompilationInput{}, err
	}
	effective, err := ComposeEffectivePersona(state)
	if err != nil {
		return PersonaCompilationInput{}, err
	}
	input.OverlayRevision = portraitOverlayRevision(state)
	input.EffectivePersona = map[string]any{"personality": effective.Personality, "behavioral_policy": effective.BehaviorPolicy}
	return input, nil
}

func synthesizeBaselineWorkingPersona(input PersonaCompilationInput) (CompiledWorkingPersona, error) {
	source, err := personaCompilationSource(input)
	if err != nil {
		return CompiledWorkingPersona{}, err
	}
	budget := input.TargetBudgetRunes
	if budget < 512 {
		budget = defaultWorkingPersonaMaxRunes
	}

	facts := make([]PersonaPortraitFact, 0)
	seen := make(map[string]struct{})

	// 1. Identity facts
	name := strings.TrimSpace(stringValue(mapValue(source["identity"])["name"]))
	if name != "" && personaCompilationSourcePathExists(source, "identity.name") {
		facts = append(facts, PersonaPortraitFact{Category: "identity", Text: name, SourceRefs: []string{"identity.name"}})
		seen["identity.name"] = struct{}{}
	} else if input.ProfileID != "" {
		facts = append(facts, PersonaPortraitFact{Category: "identity", Text: input.ProfileID, SourceRefs: []string{"profile.id"}})
		seen["profile.id"] = struct{}{}
	}

	if occ := strings.TrimSpace(stringValue(mapValue(source["identity"])["occupation"])); occ != "" && personaCompilationSourcePathExists(source, "identity.occupation") {
		facts = append(facts, PersonaPortraitFact{Category: "identity", Text: occ, SourceRefs: []string{"identity.occupation"}})
		seen["identity.occupation"] = struct{}{}
	}

	// 2. Personality / mechanisms facts
	for _, ref := range []string{"profile.personality", "personality"} {
		if val := mapValue(source[ref]); len(val) > 0 {
			for k, v := range val {
				kPath := ref + "." + k
				if personaCompilationSourcePathExists(source, kPath) {
					text := fmt.Sprintf("%s: %v", k, v)
					if len([]rune(text)) <= 1000 {
						facts = append(facts, PersonaPortraitFact{Category: "core_mechanisms", Text: text, SourceRefs: []string{kPath}})
						seen[kPath] = struct{}{}
					}
				}
			}
		}
	}

	// 3. Behavioral policy / boundaries facts
	for _, ref := range []string{"profile.behavioral_policy", "behavioral_policy"} {
		if val := mapValue(source[ref]); len(val) > 0 {
			for k, v := range val {
				kPath := ref + "." + k
				if personaCompilationSourcePathExists(source, kPath) {
					text := fmt.Sprintf("%s: %v", k, v)
					if len([]rune(text)) <= 1000 {
						facts = append(facts, PersonaPortraitFact{Category: "behavior_boundaries", Text: text, SourceRefs: []string{kPath}})
						seen[kPath] = struct{}{}
					}
				}
			}
		}
	}

	// 4. Stable preferences & habits - every key in life_profile.preferences / habits / profile.preferences / habits MUST be covered
	for _, group := range []struct {
		path  string
		value map[string]any
	}{
		{"life_profile.preferences", mapValue(mapValue(source["life_profile"])["preferences"])},
		{"life_profile.habits", mapValue(mapValue(source["life_profile"])["habits"])},
		{"profile.preferences", mapValue(mapValue(source["profile"])["preferences"])},
		{"profile.habits", mapValue(mapValue(source["profile"])["habits"])},
	} {
		for key, val := range group.value {
			ref := group.path + "." + key
			if _, already := seen[ref]; !already && personaCompilationSourcePathExists(source, ref) {
				text := fmt.Sprintf("%s: %v", key, val)
				if len([]rune(text)) > 1000 {
					text = string([]rune(text)[:1000])
				}
				facts = append(facts, PersonaPortraitFact{Category: "stable_preferences", Text: text, SourceRefs: []string{ref}})
				seen[ref] = struct{}{}
			}
		}
	}

	if len(facts) == 0 {
		facts = append(facts, PersonaPortraitFact{Category: "identity", Text: input.ProfileID, SourceRefs: []string{"profile.id"}})
	}

	return CompiledWorkingPersona{
		ProfileID:       input.ProfileID,
		Facts:           facts,
		SourceRevision:  input.SourceRevision,
		SourceHash:      stableDigest(jsonString(source)),
		OverlayRevision: input.OverlayRevision,
		RulesVersion:    personaCompilationRulesVersion,
		BudgetRunes:     budget,
	}, nil
}

func (a *App) compileOneWorkingPersona(ctx context.Context, input PersonaCompilationInput, mode string) (CompiledWorkingPersona, error) {
	if mode == "blank_slate" && input.SourceRevision == 0 && input.OverlayRevision == 0 {
		return synthesizeBaselineWorkingPersona(input)
	}
	compiled, err := a.CompileWorkingPersona(ctx, input)
	if err == nil {
		return compiled, nil
	}
	if input.SourceRevision == 0 && input.OverlayRevision == 0 {
		slog.Default().Warn("CompileWorkingPersona LLM failed during activation, falling back to baseline working persona",
			"profile_id", input.ProfileID,
			"error", err,
		)
		fallback, fallbackErr := synthesizeBaselineWorkingPersona(input)
		if fallbackErr == nil {
			return fallback, nil
		}
		slog.Default().Error("synthesizeBaselineWorkingPersona fallback also failed",
			"profile_id", input.ProfileID,
			"fallback_error", fallbackErr,
		)
	}
	return CompiledWorkingPersona{}, err
}

func insertCompiledWorkingPersonasTx(ctx context.Context, tx pgx.Tx, fluctlightID string, compiled []CompiledWorkingPersona) error {
	for _, item := range compiled {
		if item.ProfileID == "" || item.SourceHash == "" || item.RulesVersion != personaCompilationRulesVersion || item.BudgetRunes < 512 || len(item.Facts) == 0 {
			return errors.New("working_persona_compiled_invalid")
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_working_personas(fluctlight_id,profile_id,source_revision,source_hash,overlay_revision,rules_version,budget_runes,status,compiled_json)
			VALUES($1,$2,$3,$4,$5,$6,$7,'completed',$8)
			ON CONFLICT(fluctlight_id,profile_id) DO UPDATE SET source_revision=EXCLUDED.source_revision,source_hash=EXCLUDED.source_hash,
			overlay_revision=EXCLUDED.overlay_revision,rules_version=EXCLUDED.rules_version,budget_runes=EXCLUDED.budget_runes,status='completed',compiled_json=EXCLUDED.compiled_json,compiled_at=now()`,
			fluctlightID, item.ProfileID, item.SourceRevision, item.SourceHash, item.OverlayRevision, item.RulesVersion, item.BudgetRunes, jsonBytes(item))
		if err != nil {
			return err
		}
	}
	return nil
}

func verifyCompiledOverlayVersionsTx(ctx context.Context, tx pgx.Tx, fluctlightID string, corePersona map[string]any, compiled []CompiledWorkingPersona) error {
	for _, item := range compiled {
		baseline := personaEvolutionBaseline(fluctlightID, mapValue(corePersona["personality"]), mapValue(corePersona["behavioral_policy"]), mapValue(corePersona["personality_system"]), map[string]any{"active_profile_id": item.ProfileID})
		state, err := loadPersonaEvolutionState(ctx, tx, baseline)
		if err != nil {
			return err
		}
		if portraitOverlayRevision(state) != item.OverlayRevision {
			return ErrConflict
		}
	}
	return nil
}

func (a *App) loadCompiledWorkingPersona(ctx context.Context, projection ContextProjection) (CompiledWorkingPersona, error) {
	if a == nil || a.DB == nil {
		return CompiledWorkingPersona{}, errors.New("working_persona_store_unavailable")
	}
	profileID := stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"])
	if profileID == "" {
		profileID = initialPersonalityProfileID(corePersonaData(projection.CorePersona))
	}
	return loadCompiledWorkingPersonaVersion(ctx, a.DB.Pool(), projection.FluctlightID, profileID, projection.CorePersonaRevision, corePersonaData(projection.CorePersona), projection.EffectivePersona)
}

func loadCompiledWorkingPersonaVersion(ctx context.Context, query DBTX, fluctlightID, profileID string, expectedRevision int, corePersona, effectivePersona map[string]any) (CompiledWorkingPersona, error) {
	expectedOverlay := intValue(mapValue(effectivePersona)["portrait_overlay_revision"])
	source, err := personaCompilationSource(PersonaCompilationInput{CorePersona: corePersona, ProfileID: profileID, EffectivePersona: effectivePersona, OverlayRevision: expectedOverlay})
	if err != nil {
		return CompiledWorkingPersona{}, err
	}
	var sourceRevision, overlayRevision, savedBudget int
	var sourceHash, rulesVersion, status string
	var encoded []byte
	err = query.QueryRow(ctx, `SELECT source_revision,source_hash,overlay_revision,rules_version,budget_runes,status,compiled_json FROM public.fluctlight_working_personas WHERE fluctlight_id=$1 AND profile_id=$2`, fluctlightID, profileID).Scan(&sourceRevision, &sourceHash, &overlayRevision, &rulesVersion, &savedBudget, &status, &encoded)
	if errors.Is(err, pgx.ErrNoRows) {
		return CompiledWorkingPersona{}, errors.New("working_persona_missing")
	}
	if err != nil {
		return CompiledWorkingPersona{}, err
	}
	expectedHash := stableDigest(jsonString(source))
	currentBudget, err := readWorkingPersonaBudget(ctx, query)
	if err != nil {
		return CompiledWorkingPersona{}, err
	}
	if sourceRevision != expectedRevision || sourceHash != expectedHash || overlayRevision != expectedOverlay || rulesVersion != personaCompilationRulesVersion || savedBudget != currentBudget || status != "completed" {
		return CompiledWorkingPersona{}, fmt.Errorf("working_persona_version_mismatch: profile=%s source_revision=%d overlay_revision=%d", profileID, sourceRevision, overlayRevision)
	}
	var compiled CompiledWorkingPersona
	if err := json.Unmarshal(encoded, &compiled); err != nil {
		return CompiledWorkingPersona{}, fmt.Errorf("working_persona_decode_failed: %w", err)
	}
	if compiled.ProfileID != profileID || compiled.SourceHash != sourceHash || compiled.SourceRevision != sourceRevision || compiled.OverlayRevision != overlayRevision || compiled.RulesVersion != rulesVersion || compiled.BudgetRunes != savedBudget || len(compiled.Facts) == 0 {
		return CompiledWorkingPersona{}, errors.New("working_persona_payload_invalid")
	}
	return compiled, nil
}
