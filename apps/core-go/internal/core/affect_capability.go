package core

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type affectEventSpec struct {
	Label  string
	Deltas map[string]float64
}

var affectEventSpecs = map[string]affectEventSpec{
	"happy":       {Label: "开心", Deltas: map[string]float64{"pad.pleasure": 0.12, "pad.arousal": 0.02, "pad.dominance": 0.02, "mood.intensity": 0.12}},
	"sad":         {Label: "难过", Deltas: map[string]float64{"pad.pleasure": -0.12, "pad.arousal": -0.02, "pad.dominance": -0.05, "mood.intensity": 0.10}},
	"angry":       {Label: "生气", Deltas: map[string]float64{"pad.pleasure": -0.10, "pad.arousal": 0.12, "pad.dominance": 0.08, "mood.intensity": 0.12}},
	"afraid":      {Label: "害怕", Deltas: map[string]float64{"pad.pleasure": -0.10, "pad.arousal": 0.12, "pad.dominance": -0.10, "mood.intensity": 0.12}},
	"anxious":     {Label: "焦虑", Deltas: map[string]float64{"pad.pleasure": -0.08, "pad.arousal": 0.10, "pad.dominance": -0.06, "mood.intensity": 0.10}},
	"calm":        {Label: "平静", Deltas: map[string]float64{"pad.arousal": -0.10, "pad.dominance": 0.02, "mood.intensity": -0.12}},
	"relieved":    {Label: "释然", Deltas: map[string]float64{"pad.pleasure": 0.10, "pad.arousal": -0.05, "pad.dominance": 0.04, "mood.intensity": 0.08}},
	"embarrassed": {Label: "害羞", Deltas: map[string]float64{"pad.pleasure": 0.03, "pad.arousal": 0.12, "pad.dominance": -0.08, "mood.intensity": 0.10}},
	"excited":     {Label: "兴奋", Deltas: map[string]float64{"pad.pleasure": 0.10, "pad.arousal": 0.12, "pad.dominance": 0.04, "mood.intensity": 0.12}},
	"lonely":      {Label: "孤独", Deltas: map[string]float64{"pad.pleasure": -0.10, "pad.arousal": -0.04, "pad.dominance": -0.05, "mood.intensity": 0.08}},
	"frustrated":  {Label: "挫败", Deltas: map[string]float64{"pad.pleasure": -0.10, "pad.arousal": 0.08, "pad.dominance": -0.08, "mood.intensity": 0.10}},
	"touched":     {Label: "感动", Deltas: map[string]float64{"pad.pleasure": 0.10, "pad.arousal": 0.04, "pad.dominance": 0.02, "mood.intensity": 0.10}},
}

func affectEventCapabilityDefinition() CapabilityDefinition {
	eventType := make([]any, 0, len(affectEventSpecs))
	for key := range affectEventSpecs {
		eventType = append(eventType, key)
	}
	sort.Slice(eventType, func(i, j int) bool { return stringValue(eventType[i]) < stringValue(eventType[j]) })
	return CapabilityDefinition{
		Name: "affect_event", Version: "v1", Type: CapabilityTypeAction,
		Description:     "Record a semantic affect event; Core owns the numeric reducer.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy},
		FailurePolicy:   FailurePolicyOptionalInternal,
		RequiredContext: []ContextSlot{SlotCurrentState},
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"event"},
			"properties": map[string]any{
				"event": map[string]any{
					"type": "object", "additionalProperties": false,
					"required": []any{"type", "confidence"},
					"properties": map[string]any{
						"type":       map[string]any{"type": "string", "enum": eventType},
						"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
					},
				},
			},
		},
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"event_id", "type", "label", "intensity", "revision"},
			"properties": map[string]any{
				"event_id":  map[string]any{"type": "string"},
				"type":      map[string]any{"type": "string"},
				"label":     map[string]any{"type": "string"},
				"intensity": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				"revision":  map[string]any{"type": "integer", "minimum": 0},
			},
		},
		SideEffectClass: "native_projection", SuccessBoundary: "state_revision_committed", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true,
		ProvenanceFields: []string{"evidence_refs", "idempotency_key"}, NestedProvenanceObject: "event",
	}
}

type affectEventService struct{ service affectCapabilityService }

func (service *affectEventService) execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return service.executeWith(ctx, nil, invocation, resolved)
}

func (service *affectEventService) executeTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return service.executeWith(ctx, tx, invocation, resolved)
}

func (service *affectEventService) executeWith(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if service == nil || service.service == nil {
		return failedCapabilityResultDetail(invocation, "affect_capability_unavailable", true, "affect capability is unavailable"), errors.New("affect capability unavailable")
	}
	event, err := normalizeAffectEventInvocation(invocation)
	if err != nil {
		return failedCapabilityResultDetail(invocation, affectErrorCode(err), false, err.Error()), err
	}
	state := map[string]any(nil)
	if resolved.State != nil {
		state = resolved.State.Data
	}
	var result map[string]any
	if tx != nil {
		result, err = service.service.applyAffectEventTx(ctx, tx, invocation.Metadata.FluctlightID, invocation.SourceFactID, event, state)
	} else {
		result, err = service.service.applyAffectEvent(ctx, invocation.Metadata.FluctlightID, invocation.SourceFactID, event, state)
	}
	if err != nil {
		code, retryable := capabilityErrorInfo(err, "affect_persist_failed", true)
		return failedCapabilityResultDetail(invocation, code, retryable, err.Error()), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: result, Retryable: false, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "affect:" + stringValue(result["event_id"])}, nil
}

type normalizedAffectEvent struct {
	Type           string
	Confidence     float64
	EvidenceRefs   []any
	IdempotencyKey string
}

func normalizeAffectEventInvocation(invocation CapabilityInvocation) (normalizedAffectEvent, error) {
	args, err := capabilityExecutionArguments(invocation, affectEventCapabilityDefinition())
	if err != nil {
		return normalizedAffectEvent{}, errors.New("affect_arguments_invalid")
	}
	event := mapValue(args["event"])
	if len(event) == 0 {
		return normalizedAffectEvent{}, errors.New("affect_event_required")
	}
	typeName := strings.TrimSpace(stringValue(event["type"]))
	if _, ok := affectEventSpecs[typeName]; !ok {
		return normalizedAffectEvent{}, errors.New("affect_type_invalid")
	}
	confidence, err := requiredBoundedNumber(event["confidence"])
	if err != nil {
		return normalizedAffectEvent{}, errors.New("affect_confidence_invalid")
	}
	refs := arrayValue(event["evidence_refs"])
	if len(refs) == 0 || len(refs) > 32 {
		return normalizedAffectEvent{}, errors.New("affect_evidence_refs_invalid")
	}
	for _, raw := range refs {
		if text := stringValue(raw); text == "" || len([]rune(text)) > 256 {
			return normalizedAffectEvent{}, errors.New("affect_evidence_refs_invalid")
		}
	}
	idempotency := strings.TrimSpace(stringValue(event["idempotency_key"]))
	if idempotency == "" || len([]rune(idempotency)) > 256 {
		return normalizedAffectEvent{}, errors.New("affect_idempotency_key_invalid")
	}
	if invocation.SourceFactID != "" && !containsStringValue(refs, invocation.SourceFactID) {
		refs = append(refs, invocation.SourceFactID)
	}
	return normalizedAffectEvent{Type: typeName, Confidence: confidence, EvidenceRefs: refs, IdempotencyKey: idempotency}, nil
}

func affectErrorCode(err error) string {
	if err == nil {
		return "affect_event_invalid"
	}
	return err.Error()
}

func applyAffectDeltas(current map[string]any, event normalizedAffectEvent) (map[string]any, map[string]any, map[string]any, string, error) {
	return applyAffectDeltasWithProfile(current, event, defaultAffectProfile(), time.Now().UTC())
}

func applyAffectDeltasWithProfile(current map[string]any, event normalizedAffectEvent, profile AffectProfile, at time.Time) (map[string]any, map[string]any, map[string]any, string, error) {
	spec, ok := affectEventSpecs[event.Type]
	if !ok {
		return nil, nil, nil, "", errors.New("affect_type_invalid")
	}
	requested := make(map[string]float64, len(spec.Deltas))
	for key, delta := range spec.Deltas {
		scaled := delta * event.Confidence
		requested[key] = scaled
	}
	result, requestedAudit, applied := reduceAffectState(current, profile, affectReductionInput{Requested: requested, Label: spec.Label, Source: "affect_event"}, at)
	mood := mapValue(result["mood"])
	mood["last_event_type"] = event.Type
	result["mood"] = mood
	return result, requestedAudit, applied, spec.Label, nil
}

func (a *App) applyAffectEvent(ctx context.Context, fluctlightID, sourceFactID string, event normalizedAffectEvent, resolvedState map[string]any) (map[string]any, error) {
	var output map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var applyErr error
		output, applyErr = a.applyAffectEventTx(ctx, tx, fluctlightID, sourceFactID, event, resolvedState)
		return applyErr
	})
	return output, err
}

func affectEventRequestDigest(fluctlightID, sourceFactID string, event normalizedAffectEvent) string {
	refs := make([]string, 0, len(event.EvidenceRefs))
	for _, raw := range event.EvidenceRefs {
		if ref := strings.TrimSpace(stringValue(raw)); ref != "" {
			refs = append(refs, ref)
		}
	}
	sort.Strings(refs)
	return stableDigest(string(jsonBytes(map[string]any{
		"fluctlight_id": fluctlightID, "source_fact_id": sourceFactID, "type": event.Type,
		"confidence": event.Confidence, "evidence_refs": refs, "idempotency_key": event.IdempotencyKey,
	})))
}

func replayAffectEvent(payload map[string]any, eventID, fluctlightID, sourceFactID string, event normalizedAffectEvent) (map[string]any, error) {
	requestDigest := affectEventRequestDigest(fluctlightID, sourceFactID, event)
	storedDigest := strings.TrimSpace(stringValue(payload["request_digest"]))
	if storedDigest == "" {
		storedDigest = affectEventRequestDigest(fluctlightID, stringValue(payload["source_fact_id"]), normalizedAffectEvent{
			Type: stringValue(payload["event_type"]), Confidence: numberOrZero(payload["confidence"]),
			EvidenceRefs: arrayValue(payload["evidence_refs"]), IdempotencyKey: stringValue(payload["idempotency_key"]),
		})
	}
	if requestDigest != storedDigest {
		return nil, newCapabilityError("affect_idempotency_conflict", false, ErrConflict)
	}
	resultingState := mapValue(payload["resulting_state"])
	return map[string]any{
		"event_id": eventID, "type": stringValue(payload["event_type"]), "label": stringValue(payload["label"]),
		"intensity": mapValue(resultingState["mood"])["intensity"], "revision": payload["revision"],
	}, nil
}

func (a *App) applyAffectEventTx(ctx context.Context, tx pgx.Tx, fluctlightID, sourceFactID string, event normalizedAffectEvent, resolvedState map[string]any) (map[string]any, error) {
	eventID := "affect_event_" + stableDigest(fluctlightID+":"+event.IdempotencyKey)
	var revision int
	var pad, mood, momentum, regulation, drives, conflicts []byte
	var updated time.Time
	if err := tx.QueryRow(ctx, `SELECT revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at FROM public.fluctlight_inner_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision, &pad, &mood, &momentum, &regulation, &drives, &conflicts, &updated); err != nil {
		return nil, err
	}
	// The state-row lock serializes idempotency lookup with mutation. A second
	// transaction for the same event must observe the first event before it can
	// calculate or apply another delta.
	var existing []byte
	if err := tx.QueryRow(ctx, `SELECT payload FROM public.fluctlight_inner_state_events WHERE id=$1`, eventID).Scan(&existing); err == nil {
		return replayAffectEvent(decodeObject(existing), eventID, fluctlightID, sourceFactID, event)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	current := map[string]any{"pad": decodeObject(pad), "mood": decodeObject(mood), "momentum": decodeObject(momentum), "regulation": decodeObject(regulation), "drives": decodeArray(drives), "conflicts": decodeArray(conflicts), "revision": revision, "last_updated_at": updated.Format(time.RFC3339Nano)}
	var liveProfileRevision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_affect_profiles WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&liveProfileRevision); err != nil {
		return nil, err
	}
	profile, profileProjection, err := a.readAffectProfileTx(ctx, tx, fluctlightID)
	if err != nil {
		return nil, err
	}
	if frozenProfile := mapValue(resolvedState["affect_profile"]); len(frozenProfile) > 0 {
		if _, present := frozenProfile["revision"]; !present || intValue(frozenProfile["revision"]) != liveProfileRevision {
			return nil, newCapabilityError("affect_profile_revision_conflict", false, ErrConflict)
		}
		profile, err = affectProfileFromProjection(frozenProfile)
		if err != nil {
			return nil, err
		}
		profileProjection = cloneMap(frozenProfile)
	}
	current["affect_profile"] = profileProjection
	var existingStateRevisionID string
	var baseRevision int
	var existingRequestedRaw, existingAppliedRaw []byte
	combined := false
	var existingResultingRevision int
	if err := tx.QueryRow(ctx, `SELECT id,expected_revision,resulting_revision,requested_delta,applied_delta FROM public.fluctlight_state_revisions WHERE fluctlight_id=$1 AND source_event_id=$2 ORDER BY resulting_revision DESC LIMIT 1 FOR UPDATE`, fluctlightID, sourceFactID).Scan(&existingStateRevisionID, &baseRevision, &existingResultingRevision, &existingRequestedRaw, &existingAppliedRaw); err == nil {
		if existingResultingRevision != revision {
			return nil, newCapabilityError("affect_source_revision_conflict", false, ErrConflict)
		}
		combined = true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if expectedRaw, present := resolvedState["revision"]; present {
		expected := intValue(expectedRaw)
		if combined && expected != baseRevision {
			return nil, newCapabilityError("affect_state_revision_conflict", false, ErrConflict)
		}
		if !combined && expected != revision {
			return nil, newCapabilityError("affect_state_revision_conflict", false, ErrConflict)
		}
	}
	transitionAt := time.Now().UTC()
	resulting, requested, applied, label, err := applyAffectDeltasWithProfile(current, event, profile, transitionAt)
	if err != nil {
		return nil, err
	}
	newRevision := revision + 1
	if combined {
		newRevision = revision
		resulting["revision"] = revision
		requested = mergeAffectDeltaMaps(decodeObject(existingRequestedRaw), requested)
		applied = mergeAffectDeltaMaps(decodeObject(existingAppliedRaw), applied)
	}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_inner_states SET revision=$2,pad=$3,mood=$4,momentum=$5,regulation=$6,drives=$7,conflicts=$8,last_updated_at=$9 WHERE fluctlight_id=$1 AND revision=$10`, fluctlightID, newRevision, jsonBytes(resulting["pad"]), jsonBytes(resulting["mood"]), jsonBytes(resulting["momentum"]), jsonBytes(resulting["regulation"]), jsonBytes(resulting["drives"]), jsonBytes(resulting["conflicts"]), transitionAt, revision)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() != 1 {
		return nil, newCapabilityError("affect_state_revision_conflict", false, ErrConflict)
	}
	payload := map[string]any{"event_id": eventID, "event_type": event.Type, "label": label, "confidence": event.Confidence, "source_fact_id": sourceFactID, "evidence_refs": event.EvidenceRefs, "idempotency_key": event.IdempotencyKey, "request_digest": affectEventRequestDigest(fluctlightID, sourceFactID, event), "requested_delta": requested, "applied_delta": applied, "resulting_state": resulting, "revision": newRevision}
	insertedEvent, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_inner_state_events(id,fluctlight_id,event_type,payload,revision) VALUES($1,$2,'affect.event',$3,$4) ON CONFLICT(id) DO NOTHING`, eventID, fluctlightID, jsonBytes(payload), newRevision)
	if err != nil {
		return nil, err
	}
	if insertedEvent.RowsAffected() != 1 {
		return nil, newCapabilityError("affect_idempotency_conflict", false, ErrConflict)
	}
	if combined {
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_state_revisions SET resulting_state=$2,requested_delta=$3,applied_delta=$4,reason_code='cognitive_growth+affect_event',policy_version=$5,evidence_refs=(SELECT jsonb_agg(DISTINCT value) FROM jsonb_array_elements(evidence_refs || $6::jsonb)) WHERE id=$1`, existingStateRevisionID, jsonBytes(resulting), jsonBytes(requested), jsonBytes(applied), profile.PolicyVersion, jsonBytes(event.EvidenceRefs)); err != nil {
			return nil, err
		}
	} else {
		stateRevisionID := "state_revision_" + stableDigest(eventID)
		insertedRevision, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_state_revisions(id,fluctlight_id,source_event_id,expected_revision,resulting_revision,previous_state,resulting_state,requested_delta,applied_delta,result,reason_code,policy_version,model_version,evidence_refs,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'applied','affect_event',$10,'configured',$11,$12) ON CONFLICT DO NOTHING`, stateRevisionID, fluctlightID, sourceFactID, revision, newRevision, jsonBytes(current), jsonBytes(resulting), jsonBytes(requested), jsonBytes(applied), profile.PolicyVersion, jsonBytes(event.EvidenceRefs), event.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if insertedRevision.RowsAffected() != 1 {
			return nil, newCapabilityError("affect_source_revision_conflict", false, ErrConflict)
		}
	}
	if err := appendOutboxTx(ctx, tx, "affect.updated", "fluctlight", fluctlightID, fluctlightID, sourceFactID, "affect:"+eventID, "affect:"+eventID, map[string]any{"event_id": eventID, "type": event.Type, "label": label, "revision": newRevision}); err != nil {
		return nil, err
	}
	return map[string]any{"event_id": eventID, "type": event.Type, "label": label, "intensity": mapValue(resulting["mood"])["intensity"], "revision": newRevision}, nil
}
