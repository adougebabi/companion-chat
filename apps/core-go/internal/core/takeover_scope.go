package core

import (
	"context"
	"errors"
	"strings"
)

// A takeover reply speaks as the frozen reply owner, not as the persistent
// dominant profile. Everything profile-scoped that the generation reads — the
// Working Persona, retrieved memories, relationships, goals, intentions, the
// accepted Evolution Overlay and the opaque reference index — must therefore be
// re-derived from that owner instead of being read back from the current runtime
// row (design.md 4.1 / 10, F04).
//
// The projection is re-scoped in memory only. The persistent
// fluctlight_personality_runtime row is never written, which is what makes
// `takeover_once` a per-turn decision rather than a persistent switch.

var errFrozenProjectionInvalid = errors.New("frozen_context_projection_invalid")

// takeoverScopeError keeps persistence/codec failures bounded at the
// arbitration boundary. The underlying cause remains available to internal
// callers through Unwrap, while Error intentionally exposes only the stable
// diagnostic code so database/provider details cannot be copied into a frozen
// payload or a user-facing error.
type takeoverScopeError struct {
	code  string
	cause error
}

func (e *takeoverScopeError) Error() string {
	if e == nil || strings.TrimSpace(e.code) == "" {
		return "takeover_scope_failed"
	}
	return e.code
}

func (e *takeoverScopeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

const (
	takeoverControlViewProjectionInvalid = "takeover_control_view_projection_invalid"
	takeoverOverlayLoadFailed            = "takeover_overlay_load_failed"
	takeoverOverlayComposeFailed         = "takeover_overlay_compose_failed"
)

func newTakeoverScopeError(code string, cause error) error {
	return &takeoverScopeError{code: code, cause: cause}
}

func takeoverScopeErrorCode(err error) string {
	var scopedErr *takeoverScopeError
	if errors.As(err, &scopedErr) && scopedErr != nil {
		return scopedErr.Error()
	}
	return "takeover_scope_failed"
}

// resumeProjectionForReplyOwner returns a projection whose active profile is the
// reply owner. It is deterministic for the same input and never mutates the
// projection it is given.
func (a *App) resumeProjectionForReplyOwner(ctx context.Context, projection ContextProjection, ownerProfileID string) (ContextProjection, error) {
	owner := strings.TrimSpace(ownerProfileID)
	if owner == "" {
		return projection, nil
	}
	current := strings.TrimSpace(stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"]))
	if current == "" {
		current = strings.TrimSpace(projection.ReferenceIndex.ActiveProfileID)
	}
	if owner == current {
		return projection, nil
	}
	// Clone through the canonical codec so no map or slice is shared with the
	// frozen projection the caller still owns.
	scoped, ok := contextProjectionFromValue(projection)
	if !ok {
		return projection, newTakeoverScopeError(takeoverControlViewProjectionInvalid, errFrozenProjectionInvalid)
	}
	runtimeCopy := mapValue(scoped.PersonalityRuntime)
	if runtimeCopy == nil {
		runtimeCopy = map[string]any{}
	}
	runtimeCopy["active_profile_id"] = owner
	scoped.PersonalityRuntime = runtimeCopy
	// The accepted Evolution Overlay of the persistent active profile must not
	// leak onto the takeover owner, and the owner's own accepted evolution must
	// not be silently dropped.
	if err := a.rescopeEffectivePersona(ctx, &scoped, owner); err != nil {
		return projection, err
	}
	// Rebuild the opaque reference index for the new profile scope: the scope is
	// part of every token, so an index that still carries the previous profile
	// would make the owner's decisions unverifiable.
	if err := buildContextReferenceIndex(&scoped); err != nil {
		return projection, err
	}
	return scoped, nil
}

// rescopeEffectivePersona recomputes the composed persona for the reply owner
// from the same overlays the projection builder uses. Without a database (pure
// unit tests) the previous composition is dropped rather than misapplied.
func (a *App) rescopeEffectivePersona(ctx context.Context, scoped *ContextProjection, owner string) error {
	if scoped == nil {
		return errFrozenProjectionInvalid
	}
	if a == nil || a.DB == nil || ctx == nil {
		scoped.EffectivePersona = nil
		scoped.EvolutionOverlays = nil
		return nil
	}
	system := mapValue(scoped.PersonalitySystem)
	if len(system) == 0 {
		system = mapValue(corePersonaData(scoped.CorePersona)["personality_system"])
	}
	baseline := personaEvolutionBaseline(scoped.FluctlightID, scoped.Personality, scoped.BehavioralPolicy, system, map[string]any{"active_profile_id": owner})
	state, err := loadPersonaEvolutionState(ctx, a.DB.Pool(), baseline)
	if err != nil {
		return newTakeoverScopeError(takeoverOverlayLoadFailed, err)
	}
	effective, err := ComposeEffectivePersona(state)
	if err != nil {
		return newTakeoverScopeError(takeoverOverlayComposeFailed, err)
	}
	overlays := make([]map[string]any, 0, len(state.Overlays))
	for _, overlay := range state.Overlays {
		if overlay.Status != OverlayActive {
			continue
		}
		overlays = append(overlays, map[string]any{
			"id": overlay.ID, "ref": overlay.Ref, "kind": overlay.Kind, "field_path": overlay.FieldPath,
			"semantic_direction": overlay.SemanticDirection, "confidence": overlay.Confidence,
			"policy_version": overlay.PolicyVersion, "revision": overlay.Revision,
		})
	}
	scoped.EffectivePersona = map[string]any{
		"profile_id": owner, "profile_ref": effective.ProfileRef, "authority_revision": effective.AuthorityRevision,
		"personality": effective.Personality, "behavioral_policy": effective.BehaviorPolicy,
	}
	scoped.EvolutionOverlays = overlays
	return nil
}
