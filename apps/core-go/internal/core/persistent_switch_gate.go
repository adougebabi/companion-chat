package core

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// The permission spine (design.md 0.4) makes "a turn takeover may not rewrite
// the persistent dominant profile" a structural property of the pipeline
// instead of a prompt-level convention:
//
//	E1 applyPersistentSwitchGrant   drops an unauthorized proposal before it is persisted
//	E2 persistTurnDecision          records the authorization decision on the frozen payload
//	E3 applyPersistentSwitchIfAuthorizedTx  the only path that may write active_profile_id
//	E4 recovery                      reuses the same gate, never re-derives authority
//	E5 resetPersistentSwitchAuthorization   an overwrite never inherits authority
const (
	// persistentSwitchPayloadKey marks the frozen payload with the exact
	// authorization decision made for the generation that produced it.
	persistentSwitchPayloadKey = "persistent_switch"
	// persistentSwitchProposalKey records a dropped proposal instead of
	// failing the turn, so the user-visible reply survives one extra field.
	persistentSwitchProposalKey = "persistent_switch_proposal"
	// turnPersonaScopePayloadKey freezes the persona snapshot the generation
	// actually saw.
	turnPersonaScopePayloadKey = "turn_persona_scope"

	persistentSwitchNotAuthorizedCode = "personality_switch_not_authorized"
	persistentSwitchProposalDropped   = "persistent_switch_proposal_dropped"
	persistentSwitchDeniedByGate      = "persistent_switch_denied_by_gate"
	persistentSwitchRejectedScenario  = "takeover_reply"
	persistentSwitchRejectedReason    = "takeover_reply_owner"
)

// turnDecisionAuthority is the persona-authority context of one generation. It
// is threaded into PersistTurnDecision so the frozen payload can prove, later,
// whether that generation was allowed to propose a persistent switch.
type turnDecisionAuthority struct {
	Grant persistentSwitchGrant
	Scope turnPersonaScope
}

func (value turnDecisionAuthority) payload() map[string]any {
	return map[string]any{
		"authorized":     value.Grant.Allowed,
		"scenario":       value.Grant.Scenario,
		"grant_revision": value.Scope.PersonaRevision,
	}
}

// applyPersistentSwitchGrant implements E1. An unauthorized generation loses
// every persistent-switch field before it reaches the durable proposal, and the
// removal is recorded so the drop is auditable rather than silent.
func applyPersistentSwitchGrant(decision map[string]any, grant persistentSwitchGrant) (map[string]any, []personaSwitchDiagnostic) {
	if decision == nil {
		return decision, nil
	}
	if grant.Allowed {
		return decision, nil
	}
	rawProposal := mapValue(decision["personality_decision"])
	_, hasTransition := decision["personality_transition"]
	proposed := hasTransition || len(rawProposal) > 0
	delete(decision, "personality_decision")
	delete(decision, "personality_transition")
	record := map[string]any{
		"proposed": proposed,
		"applied":  false,
		"scenario": grant.Scenario,
	}
	if grant.Reason != "" {
		record["reason"] = grant.Reason
	}
	if proposed {
		summary := map[string]any{}
		for _, key := range []string{"decision", "from_profile_id", "target_profile_id", "trigger_id"} {
			if value := strings.TrimSpace(stringValue(rawProposal[key])); value != "" {
				summary[key] = value
			}
		}
		if len(summary) > 0 {
			record["proposed_decision"] = summary
		}
	}
	decision[persistentSwitchProposalKey] = record
	if !proposed {
		return decision, nil
	}
	// The declared trigger cannot be resolved without authority; only the raw
	// identifiers are reported, never the dropped plan.
	return decision, []personaSwitchDiagnostic{{
		Code:   persistentSwitchProposalDropped,
		Source: personaSwitchSourceSwitching,
		Detail: "scenario " + grantedScenario(grant) + " is not authorized to change the persistent dominant profile",
	}}
}

func grantedScenario(grant persistentSwitchGrant) string {
	if scenario := strings.TrimSpace(grant.Scenario); scenario != "" {
		return scenario
	}
	return "unspecified"
}

// persistentSwitchPayloadForTurn renders the E2 payload marker.
func persistentSwitchPayloadForTurn(authority turnDecisionAuthority) map[string]any {
	return authority.payload()
}

// persistentSwitchAuthorizedByPayload is the single authoritative reading of the
// persisted authorization decision. It never infers authority from the presence
// of a transition field.
func persistentSwitchAuthorizedByPayload(payload map[string]any) bool {
	marker := mapValue(payload[persistentSwitchPayloadKey])
	authorized, ok := marker["authorized"].(bool)
	return ok && authorized
}

// validatePersistentSwitchPersistence implements the E2 defence in depth: a
// transition may only be persisted together with a positive authorization. E1
// normally makes this unreachable.
func validatePersistentSwitchPersistence(payload map[string]any, decision map[string]any) error {
	if _, exists := decision["personality_transition"]; !exists {
		return nil
	}
	if persistentSwitchAuthorizedByPayload(payload) {
		return nil
	}
	return errors.New(persistentSwitchNotAuthorizedCode)
}

// applyPersistentSwitchIfAuthorizedTx implements E3: the only call site allowed
// to write the persistent active profile. A missing or false authorization is a
// structural no-op, not an error, because replay must be idempotent.
func (a *App) applyPersistentSwitchIfAuthorizedTx(ctx context.Context, tx pgx.Tx, fluctlightID string, payload map[string]any, plan *personalityDecisionPlan) (map[string]any, error) {
	if plan == nil {
		return nil, nil
	}
	if !persistentSwitchAuthorizedByPayload(payload) {
		return nil, nil
	}
	return a.applyPersonalityDecisionPlanTx(ctx, tx, fluctlightID, plan)
}

// resetPersistentSwitchAuthorization implements E5. Any payload overwrite must
// call it: authority is never inherited from the candidate being replaced.
func resetPersistentSwitchAuthorization(payload map[string]any, scenario, reason string) {
	if payload == nil {
		return
	}
	if strings.TrimSpace(scenario) == "" {
		scenario = persistentSwitchRejectedScenario
	}
	if strings.TrimSpace(reason) == "" {
		reason = persistentSwitchRejectedReason
	}
	payload[persistentSwitchPayloadKey] = map[string]any{"authorized": false, "scenario": scenario, "reason": reason}
	decision := mapValue(payload["decision"])
	if len(decision) == 0 {
		return
	}
	delete(decision, "personality_transition")
	delete(decision, "personality_decision")
	payload["decision"] = decision
}

// ---------------------------------------------------------------------------
// Turn persona scope
// ---------------------------------------------------------------------------

// resolveTurnPersonaScope freezes the persona view of one turn from the same
// projection the generation consumes. ScopeRevision is a snapshot identity only
// compared for equality, never ordered.
func resolveTurnPersonaScope(projection ContextProjection) turnPersonaScope {
	runtime := mapValue(projection.PersonalityRuntime)
	active := strings.TrimSpace(stringValue(runtime["active_profile_id"]))
	if active == "" {
		active = strings.TrimSpace(initialPersonalityProfileID(projection.CorePersona))
	}
	if active == "" {
		active = "default"
	}
	personaRevision := intValue(runtime["revision"])
	overlayRevision := intValue(mapValue(projection.EffectivePersona)["authority_revision"])
	return turnPersonaScope{
		ActiveProfileID:     active,
		ReplyOwnerProfileID: active,
		PersonaRevision:     personaRevision,
		OverlayRevision:     overlayRevision,
		ScopeRevision:       personaScopeRevision(active, personaRevision, overlayRevision),
	}
}

func personaScopeRevision(activeProfileID string, personaRevision, overlayRevision int) int {
	digest := stableDigest("turn-persona-scope\x1f" + activeProfileID + "\x1f" + strconv.Itoa(personaRevision) + "\x1f" + strconv.Itoa(overlayRevision))
	value, err := strconv.ParseInt(digest[:8], 16, 64)
	if err != nil {
		return 0
	}
	return int(value & 0x7fffffff)
}

// turnPersonaScopeFromPayload reads the frozen snapshot back. It is used by the
// recovery and takeover paths so they never re-derive the owner from the
// current runtime row.
func turnPersonaScopeFromPayload(payload map[string]any) (turnPersonaScope, bool) {
	raw := mapValue(payload[turnPersonaScopePayloadKey])
	if len(raw) == 0 {
		return turnPersonaScope{}, false
	}
	active := strings.TrimSpace(stringValue(raw["active_profile_id"]))
	if active == "" {
		return turnPersonaScope{}, false
	}
	scope := turnPersonaScope{
		ActiveProfileID: active,
		PersonaRevision: intValue(raw["persona_revision"]),
		OverlayRevision: intValue(raw["overlay_revision"]),
		ScopeRevision:   intValue(raw["scope_revision"]),
	}
	scope.ReplyOwnerProfileID = strings.TrimSpace(stringValue(raw["reply_owner_profile_id"]))
	if scope.ReplyOwnerProfileID == "" {
		scope.ReplyOwnerProfileID = active
	}
	return scope, true
}

func turnPersonaScopePayload(scope turnPersonaScope) map[string]any {
	return map[string]any{
		"active_profile_id":      scope.ActiveProfileID,
		"reply_owner_profile_id": scope.ReplyOwner(),
		"persona_revision":       scope.PersonaRevision,
		"overlay_revision":       scope.OverlayRevision,
		"scope_revision":         scope.ScopeRevision,
		"persona_switch_rules":   personaSwitchRuleSetVersion,
	}
}
