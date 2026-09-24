package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	habitInspectCapabilityName = "habit.inspect"
	habitDecideCapabilityName  = "habit.decide"
)

type habitService struct{ app *App }
type habitInspectCapability struct{ service *habitService }
type habitDecideCapability struct{ service *habitService }

func habitInspectDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: habitInspectCapabilityName, Version: "v1", Type: CapabilityTypeQuery,
		Description:   "Read the current speaking profile's effective ordinary life habits with stable indexes and revision; these can be changed by an explicit decision.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema:   objectSchema(map[string]any{}, nil, false), OutputSchema: openObjectSchema(),
		SideEffectClass: "read_only", SuccessBoundary: "query_result_available", ConcurrencyClass: "parallel", SupportsRetry: true,
	}
}
func (c habitInspectCapability) Definition() CapabilityDefinition { return habitInspectDefinition() }
func (c habitInspectCapability) RequiredContext() []ContextSlot   { return nil }
func (c habitInspectCapability) Execute(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.service.app == nil || c.service.app.DB == nil {
		return failedCapabilityResult(invocation, "habit_unavailable", true), errors.New("habit service unavailable")
	}
	profileID, err := c.service.resolveProfile(ctx, c.service.app.DB.Pool(), invocation.Metadata.FluctlightID, invocation.Metadata.WorkingProfileID)
	if err != nil {
		return failedCapabilityResult(invocation, "habit_profile_unavailable", true), err
	}
	habits, revision, err := readProfileHabits(ctx, c.service.app.DB.Pool(), invocation.Metadata.FluctlightID, profileID)
	if err != nil {
		return failedCapabilityResult(invocation, "habit_source_unavailable", true), err
	}
	items := make([]map[string]any, 0, len(habits))
	for index, habit := range habits {
		items = append(items, map[string]any{"index": index, "value": habit})
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"profile_id": profileID, "revision": revision, "habits": items},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "habit:" + profileID}, nil
}

func habitDecideDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: habitDecideCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:   "Commit one explicit decision to append, replace, or remove an ordinary life habit for the current speaking profile. A one-time outfit choice does not call this and this does not change current clothing.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: objectSchema(map[string]any{
			"operation": enumStringSchema("append", "replace", "remove"),
			"index":     map[string]any{"type": "integer", "minimum": 0},
			"text":      map[string]any{"type": "string", "maxLength": 500},
			"reason":    map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		}, []string{"operation", "reason"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "native_projection", SuccessBoundary: "effective_habit_committed",
		ConcurrencyClass: "exclusive", SupportsRetry: true,
	}
}
func (c habitDecideCapability) Definition() CapabilityDefinition { return habitDecideDefinition() }
func (c habitDecideCapability) RequiredContext() []ContextSlot   { return nil }
func (c habitDecideCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}
func (s *habitService) resolveProfile(ctx context.Context, query DBTX, fluctlightID, requested string) (string, error) {
	if requested = strings.TrimSpace(requested); requested != "" {
		return requested, nil
	}
	var active string
	if err := query.QueryRow(ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&active); err != nil {
		return "", err
	}
	return active, nil
}
func (c habitDecideCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil || c.service.app == nil || c.service.app.DB == nil {
		return invocation, errors.New("habit service unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, habitDecideDefinition())
	if err != nil {
		return invocation, err
	}
	app := c.service.app
	fluctlightID := invocation.Metadata.FluctlightID
	owner := invocation.Metadata.AuthorizationActorID
	resource, err := app.DB.GetFluctlight(ctx, fluctlightID, owner)
	if err != nil {
		return invocation, err
	}
	profileID, err := c.service.resolveProfile(ctx, app.DB.Pool(), fluctlightID, invocation.Metadata.WorkingProfileID)
	if err != nil {
		return invocation, err
	}
	habits, habitRevision, err := readProfileHabits(ctx, app.DB.Pool(), fluctlightID, profileID)
	if err != nil {
		return invocation, err
	}
	next := append([]any(nil), habits...)
	operation := stringValue(args["operation"])
	index := intValue(args["index"])
	text := strings.TrimSpace(stringValue(args["text"]))
	switch operation {
	case "append":
		if text == "" {
			return invocation, ErrInvalidArguments
		}
		for _, existing := range next {
			if strings.TrimSpace(stringValue(existing)) == text {
				return invocation, newCapabilityError("habit_already_present", false, ErrConflict)
			}
		}
		next = append(next, text)
	case "replace":
		if _, exists := args["index"]; !exists || index < 0 || index >= len(next) || text == "" {
			return invocation, ErrInvalidArguments
		}
		if strings.TrimSpace(stringValue(next[index])) == text {
			return invocation, newCapabilityError("habit_unchanged", false, ErrConflict)
		}
		next[index] = text
	case "remove":
		if _, exists := args["index"]; !exists || index < 0 || index >= len(next) {
			return invocation, ErrInvalidArguments
		}
		next = append(next[:index], next[index+1:]...)
	default:
		return invocation, ErrInvalidArguments
	}
	if next == nil {
		next = []any{}
	}
	input, err := app.personaCompilationInputForProfile(ctx, fluctlightID, resource.CorePersona, resource.CurrentRevision, profileID, true)
	if err != nil {
		return invocation, err
	}
	input.EffectiveLifeHabits = &next
	var mode string
	if err := app.DB.Pool().QueryRow(ctx, `SELECT initialization_mode FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&mode); err != nil {
		return invocation, err
	}
	compiled, err := app.compileOneWorkingPersona(ctx, input, mode)
	if err != nil {
		return invocation, err
	}
	plan := map[string]any{
		"profile_id": profileID, "habit_revision": habitRevision, "previous_habits_hash": stableDigest(jsonString(habits)),
		"new_habits": next, "source_revision": resource.CurrentRevision, "source_hash": stableDigest(jsonString(resource.CorePersona)),
		"overlay_revision": input.OverlayRevision, "compiled": decodeObject(jsonBytes(compiled)), "reason": strings.TrimSpace(stringValue(args["reason"])),
	}
	return withCapabilityPreparedData(invocation, "habit_decision", plan)
}
func (c habitDecideCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.service.app == nil || c.service.app.DB == nil {
		return failedCapabilityResult(invocation, "habit_unavailable", true), errors.New("habit service unavailable")
	}
	raw, found, err := capabilityPreparedData(invocation, "habit_decision")
	if err != nil || !found {
		return failedCapabilityResult(invocation, "habit_plan_missing", false), errors.New("habit decision plan missing")
	}
	plan := mapValue(raw)
	fluctlightID := invocation.Metadata.FluctlightID
	profileID := stringValue(plan["profile_id"])
	if invocation.Metadata.WorkingProfileID != "" && invocation.Metadata.WorkingProfileID != profileID {
		return failedCapabilityResult(invocation, "habit_profile_changed", true), ErrConflict
	}
	currentProfile, err := c.service.resolveProfile(ctx, tx, fluctlightID, invocation.Metadata.WorkingProfileID)
	if err != nil || currentProfile != profileID {
		return failedCapabilityResult(invocation, "habit_profile_changed", true), ErrConflict
	}
	var revision int
	var currentRaw []byte
	if err := tx.QueryRow(ctx, `SELECT revision,habits_json FROM public.fluctlight_profile_habits WHERE fluctlight_id=$1 AND profile_id=$2 FOR UPDATE`, fluctlightID, profileID).Scan(&revision, &currentRaw); err != nil {
		return failedCapabilityResult(invocation, "habit_source_unavailable", true), err
	}
	var currentHabits []any
	if err := json.Unmarshal(currentRaw, &currentHabits); err != nil {
		return failedCapabilityResult(invocation, "habit_source_invalid", true), err
	}
	if revision != intValue(plan["habit_revision"]) || stableDigest(jsonString(currentHabits)) != stringValue(plan["previous_habits_hash"]) {
		return failedCapabilityResult(invocation, "habit_revision_conflict", true), ErrConflict
	}
	var foundationRevision int
	var coreRaw []byte
	if err := tx.QueryRow(ctx, `SELECT current_revision,core_persona FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR SHARE`, fluctlightID, invocation.Metadata.AuthorizationActorID).Scan(&foundationRevision, &coreRaw); err != nil {
		return failedCapabilityResult(invocation, "habit_foundation_unavailable", true), err
	}
	if foundationRevision != intValue(plan["source_revision"]) || stableDigest(jsonString(decodeObject(coreRaw))) != stringValue(plan["source_hash"]) {
		return failedCapabilityResult(invocation, "habit_foundation_changed", true), ErrConflict
	}
	var compiled CompiledWorkingPersona
	if err := json.Unmarshal(jsonBytes(mapValue(plan["compiled"])), &compiled); err != nil {
		return failedCapabilityResult(invocation, "habit_compilation_invalid", false), err
	}
	if compiled.ProfileID != profileID || compiled.SourceRevision != foundationRevision || compiled.OverlayRevision != intValue(plan["overlay_revision"]) || compiled.SourceHash == "" {
		return failedCapabilityResult(invocation, "habit_compilation_invalid", false), ErrConflict
	}
	if err := verifyCompiledOverlayVersionsTx(ctx, tx, fluctlightID, decodeObject(coreRaw), []CompiledWorkingPersona{compiled}); err != nil {
		return failedCapabilityResult(invocation, "habit_overlay_changed", true), err
	}
	if err := verifyCompiledBudgetTx(ctx, tx, []CompiledWorkingPersona{compiled}); err != nil {
		return failedCapabilityResult(invocation, "habit_budget_changed", true), err
	}
	next := arrayValue(plan["new_habits"])
	if next == nil {
		next = []any{}
	}
	newRevision := revision + 1
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_profile_habits SET revision=$3,habits_json=$4,source_kind='personal_decision',source_ref=$5,updated_at=now() WHERE fluctlight_id=$1 AND profile_id=$2`, fluctlightID, profileID, newRevision, jsonBytes(next), capabilityOperationID(invocation)); err != nil {
		return failedCapabilityResult(invocation, "habit_update_failed", true), err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_profile_habit_revisions(fluctlight_id,profile_id,revision,habits_json,source_kind,source_ref) VALUES($1,$2,$3,$4,'personal_decision',$5)`, fluctlightID, profileID, newRevision, jsonBytes(next), capabilityOperationID(invocation)); err != nil {
		return failedCapabilityResult(invocation, "habit_revision_insert_failed", true), err
	}
	if err := insertCompiledWorkingPersonasTx(ctx, tx, fluctlightID, []CompiledWorkingPersona{compiled}); err != nil {
		return failedCapabilityResult(invocation, "habit_portrait_publish_failed", true), err
	}
	if err := appendOutboxTx(ctx, tx, "habit.revised", "fluctlight", fluctlightID, fluctlightID, invocation.SourceFactID,
		"habit:"+profileID, "habit-revision:"+fluctlightID+":"+profileID+":"+fmt.Sprint(newRevision), map[string]any{"profile_id": profileID, "revision": newRevision}); err != nil {
		return failedCapabilityResult(invocation, "habit_event_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"profile_id": profileID, "revision": newRevision, "habits": next, "working_persona_source_hash_prefix": compiled.SourceHash[:12]},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "habit:" + profileID + ":" + fmt.Sprint(newRevision)}, nil
}
