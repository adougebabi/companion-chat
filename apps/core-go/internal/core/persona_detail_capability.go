package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const personaDetailCapabilityName = "persona.detail"

type personaDetailRevisionContextKey struct{}
type personaDetailOverlayContextKey struct{}

type personaDetailService struct {
	repository *PostgresRepository
	appearance func(context.Context, string) (map[string]any, error)
}

func newPersonaDetailService(app *App) personaDetailService {
	if app == nil {
		return personaDetailService{}
	}
	return personaDetailService{repository: app.DB, appearance: func(ctx context.Context, fluctlightID string) (map[string]any, error) {
		current, _, _, err := app.readEffectiveLifeSnapshot(ctx, fluctlightID, time.Now().UTC())
		return current, err
	}}
}

func (s personaDetailService) read(ctx context.Context, fluctlightID, owner, profileID string) (map[string]any, int, int, string, error) {
	if s.repository == nil {
		return nil, 0, 0, "", errors.New("persona detail unavailable")
	}
	resource, err := s.repository.GetFluctlight(ctx, fluctlightID, owner)
	if err != nil {
		return nil, 0, 0, "", err
	}
	if expected, ok := ctx.Value(personaDetailRevisionContextKey{}).(int); ok && resource.CurrentRevision != expected {
		return nil, 0, 0, "", ErrConflict
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		err = s.repository.Pool().QueryRow(ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, resource.ID).Scan(&profileID)
		if err != nil {
			return nil, 0, 0, "", err
		}
	}
	sections, err := personaDetailSections(resource.CorePersona, profileID)
	if err != nil {
		return nil, 0, 0, "", err
	}
	baseline := personaEvolutionBaseline(resource.ID, resource.Personality, resource.BehavioralPolicy, mapValue(resource.CorePersona["personality_system"]), map[string]any{"active_profile_id": profileID})
	state, err := loadPersonaEvolutionState(ctx, s.repository.Pool(), baseline)
	if err != nil {
		return nil, 0, 0, "", err
	}
	if expected, ok := ctx.Value(personaDetailOverlayContextKey{}).(int); ok && portraitOverlayRevision(state) != expected {
		return nil, 0, 0, "", ErrConflict
	}
	if portraitOverlayRevision(state) > 0 {
		effective, err := ComposeEffectivePersona(state)
		if err != nil {
			return nil, 0, 0, "", err
		}
		sections["personality"] = effective.Personality
		sections["behavioral_policy"] = effective.BehaviorPolicy
	}
	habits, _, err := readProfileHabits(ctx, s.repository.Pool(), resource.ID, profileID)
	if err != nil {
		return nil, 0, 0, "", err
	}
	life := cloneMap(mapValue(sections["life_profile"]))
	if life == nil {
		life = map[string]any{}
	}
	if style := mapValue(mapValue(life["appearance"])["style_preferences"]); len(style) > 0 {
		life["style_preferences"] = style
	}
	delete(life, "appearance")
	life["life_habits"] = habits
	sections["life_profile"] = life
	identity := cloneMap(mapValue(sections["identity"]))
	for _, key := range []string{"appearance", "background_story", "biography", "notes"} {
		delete(identity, key)
	}
	sections["identity"] = identity
	if profileIdentity := mapValue(sections["profile.identity"]); len(profileIdentity) > 0 {
		stableProfileIdentity := cloneMap(profileIdentity)
		for _, key := range []string{"appearance", "hair_length", "hair_color", "current_outfit", "clothing"} {
			delete(stableProfileIdentity, key)
		}
		sections["profile.identity"] = stableProfileIdentity
	}
	delete(sections, "profile.emotional_state")
	// Current detail uses the same mutable-field filter as the portrait
	// source; unclassified Foundation wording remains available via history.
	for key, value := range sections {
		if key == "current_appearance" {
			continue
		}
		if nested := mapValue(value); len(nested) > 0 {
			sections[key] = stablePersonaSourceMap(nested)
		}
	}
	if s.appearance != nil {
		current, err := s.appearance(ctx, resource.ID)
		if err != nil {
			return nil, 0, 0, "", err
		}
		sections["current_appearance"] = current
	}
	return sections, resource.CurrentRevision, portraitOverlayRevision(state), profileID, nil
}

func (s personaDetailService) readHistorical(ctx context.Context, fluctlightID, owner, profileID string, revision int) (map[string]any, string, error) {
	resource, err := s.repository.GetFluctlight(ctx, fluctlightID, owner)
	if err != nil {
		return nil, "", err
	}
	if profileID == "" {
		if err := s.repository.Pool().QueryRow(ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, resource.ID).Scan(&profileID); err != nil {
			return nil, "", err
		}
	}
	var raw []byte
	err = s.repository.Pool().QueryRow(ctx, `SELECT core_persona FROM public.fluctlight_foundation_revisions WHERE fluctlight_id=$1 AND revision=$2 AND status='accepted'`, resource.ID, revision).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	sections, err := personaDetailSectionsForTime(decodeObject(raw), profileID, true)
	return sections, profileID, err
}

// persona.detail reads the canonical structured source. Its section IDs are
// data locations, never keyword matches against the current user message.
func personaDetailCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: personaDetailCapabilityName, Version: "v1", Type: CapabilityTypeQuery,
		Description:   "List or read current effective persona details, habits and appearance for the speaking profile. Use history only when past Foundation wording is explicitly needed; historical source is not current state.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false, "required": []any{"operation"},
			"properties": map[string]any{
				"operation":  map[string]any{"type": "string", "enum": []any{"list", "read", "history"}},
				"section_id": map[string]any{"type": "string", "maxLength": 128},
				"cursor":     map[string]any{"type": "integer", "minimum": 0},
				"revision":   map[string]any{"type": "integer", "minimum": 0},
			},
		},
		OutputSchema:    personaDetailOutputSchema(),
		SideEffectClass: "read_only", SuccessBoundary: "query_result_available", ConcurrencyClass: "parallel", SupportsRetry: true,
	}
}

func personaDetailOutputSchema() map[string]any {
	common := map[string]any{
		"profile_id":       map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
		"source_revision":  map[string]any{"type": "integer", "minimum": 0},
		"overlay_revision": map[string]any{"type": "integer", "minimum": 0},
		"time_semantics":   map[string]any{"type": "string", "enum": []any{"current", "historical_foundation"}},
	}
	listProperties := cloneMap(common)
	listProperties["sections"] = map[string]any{"type": "array", "maxItems": 40, "items": objectSchema(map[string]any{
		"section_id":  map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
		"description": map[string]any{"type": "string", "maxLength": 128},
	}, []string{"section_id", "description"}, false)}
	readProperties := cloneMap(common)
	readProperties["section_id"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 128}
	readProperties["cursor"] = map[string]any{"type": "integer", "minimum": 0}
	readProperties["content"] = map[string]any{"type": "string", "maxLength": 2048}
	readProperties["has_more"] = map[string]any{"type": "boolean"}
	readProperties["next_cursor"] = map[string]any{"type": "integer", "minimum": 0}
	return map[string]any{"oneOf": []any{
		objectSchema(listProperties, []string{"profile_id", "source_revision", "overlay_revision", "sections"}, false),
		objectSchema(readProperties, []string{"profile_id", "source_revision", "overlay_revision", "section_id", "cursor", "content", "has_more", "next_cursor"}, false),
	}}
}

func (c personaDetailCapability) Definition() CapabilityDefinition {
	return personaDetailCapabilityDefinition()
}
func (c personaDetailCapability) RequiredContext() []ContextSlot { return nil }

func (c personaDetailCapability) Execute(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service.repository == nil {
		return failedCapabilityResultDetail(invocation, "persona_detail_unavailable", true, "persona detail is unavailable"), errors.New("persona detail unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, personaDetailCapabilityDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	owner := strings.TrimSpace(invocation.Metadata.AuthorizationActorID)
	operation := stringValue(args["operation"])
	var sections map[string]any
	var revision, overlayRevision int
	var profileID string
	var timeSemantics string
	if operation == "history" {
		revision = intValue(args["revision"])
		sections, profileID, err = c.service.readHistorical(ctx, invocation.Metadata.FluctlightID, owner, invocation.Metadata.WorkingProfileID, revision)
		timeSemantics = "historical_foundation"
	} else {
		sections, revision, overlayRevision, profileID, err = c.service.read(ctx, invocation.Metadata.FluctlightID, owner, invocation.Metadata.WorkingProfileID)
		timeSemantics = "current"
	}
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return failedCapabilityResultDetail(invocation, "persona_detail_source_changed", true, "persona source changed during this Agent run"), err
		}
		if errors.Is(err, ErrNotFound) {
			return failedCapabilityResultDetail(invocation, "persona_detail_source_not_found", false, "persona source unavailable"), err
		}
		return failedCapabilityResultDetail(invocation, "persona_detail_source_failed", true, "persona source unavailable"), err
	}
	output := map[string]any{"profile_id": profileID, "source_revision": revision, "overlay_revision": overlayRevision, "time_semantics": timeSemantics}
	switch operation {
	case "list":
		catalog := make([]map[string]any, 0, len(sections))
		for _, id := range workingPersonaSortedKeys(sections) {
			catalog = append(catalog, map[string]any{"section_id": id, "description": personaDetailSectionDescription(id)})
		}
		output["sections"] = catalog
	case "read", "history":
		id := strings.TrimSpace(stringValue(args["section_id"]))
		value, ok := sections[id]
		if !ok {
			return failedCapabilityResultDetail(invocation, "persona_detail_section_not_found", false, "section does not exist in speaking profile"), ErrNotFound
		}
		serialized, err := json.Marshal(value)
		if err != nil {
			return failedCapabilityResultDetail(invocation, "persona_detail_encoding_failed", true, "section encoding failed"), err
		}
		content := []rune(string(serialized))
		cursor := intValue(args["cursor"])
		if cursor < 0 || cursor > len(content) {
			return failedCapabilityResultDetail(invocation, "persona_detail_cursor_invalid", false, "cursor outside section"), ErrInvalidArguments
		}
		const pageRunes = 2048
		end := cursor + pageRunes
		if end > len(content) {
			end = len(content)
		}
		output["section_id"] = id
		output["cursor"] = cursor
		output["content"] = string(content[cursor:end])
		output["has_more"] = end < len(content)
		output["next_cursor"] = end
	default:
		return failedCapabilityResultDetail(invocation, "persona_detail_operation_invalid", false, "operation must be list or read"), ErrInvalidArguments
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "persona-detail:" + stableDigest(invocation.CallID)}, nil
}

func personaDetailSections(corePersona map[string]any, profileID string) (map[string]any, error) {
	return personaDetailSectionsForTime(corePersona, profileID, false)
}

func personaDetailSectionsForTime(corePersona map[string]any, profileID string, historical bool) (map[string]any, error) {
	factFilter := safePersonaFactMap
	if historical {
		// History preserves original accepted Foundation facts, including former
		// body/clothing claims. Only bookkeeping and private source fields leave.
		factFilter = filterCorePersonaValue
	}
	profiles := arrayValue(mapValue(corePersona["personality_system"])["profiles"])
	profile := map[string]any{}
	for _, raw := range profiles {
		candidate := mapValue(raw)
		if strings.TrimSpace(stringValue(candidate["id"])) == profileID {
			profile = candidate
			break
		}
	}
	if len(profiles) > 0 && len(profile) == 0 {
		return nil, fmt.Errorf("%w: speaking profile %q is not declared", ErrNotFound, profileID)
	}
	profile = factFilter(profile)
	profile["id"] = profileID
	sections := map[string]any{}
	if shared := sharedPersonalitySystemSource(mapValue(corePersona["personality_system"])); len(shared) > 0 {
		sections["shared_system"] = shared
	}
	for _, key := range []string{"identity", "life_profile", "extensions"} {
		if value := mapValue(corePersona[key]); len(value) > 0 {
			sections[key] = factFilter(value)
		}
	}
	for _, key := range []string{"personality", "behavioral_policy"} {
		value := mapValue(profile[key])
		if len(value) == 0 {
			value = mapValue(corePersona[key])
		}
		if len(value) > 0 {
			sections[key] = factFilter(value)
		}
	}
	for key, value := range profile {
		if key == "id" || key == "profile_id" || key == "name" || key == "personality" || key == "behavioral_policy" || key == "visual_identity" {
			continue
		}
		if value != nil {
			if nested, ok := value.(map[string]any); ok {
				value = factFilter(nested)
			}
			sections["profile."+key] = value
		}
	}
	return sections, nil
}

func personaDetailSectionDescription(id string) string {
	switch id {
	case "identity":
		return "shared identity and background"
	case "life_profile":
		return "stable life details, preferences and habits"
	case "current_appearance":
		return "current shared body state and actual wearing"
	case "extensions":
		return "additional declared persona details"
	case "shared_system":
		return "stable relationships and mechanisms shared by profiles"
	case "personality":
		return "effective personality traits and expression"
	case "behavioral_policy":
		return "effective behavior and boundaries"
	default:
		return "declared speaking-profile field"
	}
}
