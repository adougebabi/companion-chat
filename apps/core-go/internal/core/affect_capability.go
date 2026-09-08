package core

import (
	"context"
	"encoding/json"
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

func affectEventCapabilityManifest() CapabilityManifest {
	eventType := make([]any, 0, len(affectEventSpecs))
	for key := range affectEventSpecs {
		eventType = append(eventType, key)
	}
	sort.Slice(eventType, func(i, j int) bool { return stringValue(eventType[i]) < stringValue(eventType[j]) })
	return CapabilityManifest{
		Name: "affect_event", Version: "v1",
		Description: "Record an evidence-backed semantic emotion event; Core owns the numeric affect reducer.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"event"},
			"properties": map[string]any{
				"event": map[string]any{
					"type": "object", "additionalProperties": false,
					"required": []any{"type", "confidence", "evidence_refs", "idempotency_key"},
					"properties": map[string]any{
						"type":            map[string]any{"type": "string", "enum": eventType},
						"confidence":      map[string]any{"type": "number", "minimum": 0, "maximum": 1},
						"evidence_refs":   map[string]any{"type": "array", "minItems": 1, "maxItems": 32, "items": map[string]any{"type": "string", "maxLength": 256}},
						"idempotency_key": map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
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
		SideEffectClass: "native_projection", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true,
	}
}

type affectEventCapabilityExecutor struct{ app *App }

func (executor *affectEventCapabilityExecutor) Manifest() CapabilityManifest {
	return affectEventCapabilityManifest()
}

func (executor *affectEventCapabilityExecutor) Execute(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	if executor == nil || executor.app == nil {
		return failedToolResult(call, "affect_executor_unavailable", true, "affect executor is unavailable"), errors.New("affect executor unavailable")
	}
	event, err := normalizeAffectEventCall(call, sourceFactID)
	if err != nil {
		return failedToolResult(call, affectErrorCode(err), false, err.Error()), err
	}
	result, err := executor.app.applyAffectEvent(ctx, fluctlightID, sourceFactID, event)
	if err != nil {
		return failedToolResult(call, "affect_persist_failed", true, err.Error()), err
	}
	return ToolResultV1{ToolCallID: call.ID, Name: call.Name, Status: "completed", Output: result, Retryable: false, ProviderRequestID: call.ProviderRequestID, CorrelationID: "affect:" + stringValue(result["event_id"]), SchemaVersion: ToolResultSchemaVersion}, nil
}

type normalizedAffectEvent struct {
	Type           string
	Confidence     float64
	EvidenceRefs   []any
	IdempotencyKey string
}

func normalizeAffectEventCall(call ToolCallV1, sourceFactID string) (normalizedAffectEvent, error) {
	var args map[string]any
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
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
	if sourceFactID != "" && !containsStringValue(refs, sourceFactID) {
		refs = append(refs, sourceFactID)
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
	spec, ok := affectEventSpecs[event.Type]
	if !ok {
		return nil, nil, nil, "", errors.New("affect_type_invalid")
	}
	result := cloneMap(current)
	pad := cloneMap(mapValue(current["pad"]))
	mood := cloneMap(mapValue(current["mood"]))
	intensity := numberOrZero(mood["intensity"])
	requested := make(map[string]any, len(spec.Deltas))
	applied := make(map[string]any, len(spec.Deltas))
	for key, delta := range spec.Deltas {
		scaled := delta * event.Confidence
		requested[key] = scaled
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 {
			continue
		}
		target := pad
		if parts[0] == "mood" {
			target = mood
		}
		before := numberOrZero(target[parts[1]])
		bounded := clampGrowth(scaled)
		target[parts[1]] = clampUnit(before + bounded)
		applied[key] = bounded
		if key == "mood.intensity" {
			intensity = clampUnit(before + bounded)
		}
	}
	mood["label"] = spec.Label
	mood["source"] = "affect_event"
	mood["intensity"] = clampUnit(intensity)
	mood["last_event_type"] = event.Type
	result["pad"] = pad
	result["mood"] = mood
	result["revision"] = int(numberOrZero(current["revision"])) + 1
	result["last_updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	return result, requested, applied, spec.Label, nil
}

func (a *App) applyAffectEvent(ctx context.Context, fluctlightID, sourceFactID string, event normalizedAffectEvent) (map[string]any, error) {
	eventID := "affect_event_" + stableDigest(fluctlightID+":"+event.IdempotencyKey)
	var output map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var existing []byte
		if err := tx.QueryRow(ctx, `SELECT payload FROM public.fluctlight_inner_state_events WHERE id=$1`, eventID).Scan(&existing); err == nil {
			payload := decodeObject(existing)
			output = map[string]any{"event_id": eventID, "type": stringValue(payload["event_type"]), "label": stringValue(payload["label"]), "intensity": mapValue(payload["resulting_state"])["mood"], "revision": payload["revision"]}
			if mood := mapValue(payload["resulting_state"])["mood"]; mood != nil {
				output["intensity"] = mapValue(mood)["intensity"]
			}
			return nil
		} else if err != pgx.ErrNoRows {
			return err
		}
		var revision int
		var pad, mood, momentum, regulation, drives, conflicts []byte
		var updated time.Time
		if err := tx.QueryRow(ctx, `SELECT revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at FROM public.fluctlight_inner_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision, &pad, &mood, &momentum, &regulation, &drives, &conflicts, &updated); err != nil {
			return err
		}
		current := map[string]any{"pad": decodeObject(pad), "mood": decodeObject(mood), "momentum": decodeObject(momentum), "regulation": decodeObject(regulation), "drives": decodeArray(drives), "conflicts": decodeArray(conflicts), "revision": revision, "last_updated_at": updated.Format(time.RFC3339Nano)}
		resulting, requested, applied, label, err := applyAffectDeltas(current, event)
		if err != nil {
			return err
		}
		newRevision := revision + 1
		command, err := tx.Exec(ctx, `UPDATE public.fluctlight_inner_states SET revision=$2,pad=$3,mood=$4,last_updated_at=now() WHERE fluctlight_id=$1 AND revision=$5`, fluctlightID, newRevision, jsonBytes(resulting["pad"]), jsonBytes(resulting["mood"]), revision)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		payload := map[string]any{"event_id": eventID, "event_type": event.Type, "label": label, "confidence": event.Confidence, "source_fact_id": sourceFactID, "evidence_refs": event.EvidenceRefs, "idempotency_key": event.IdempotencyKey, "requested_delta": requested, "applied_delta": applied, "resulting_state": resulting, "revision": newRevision}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_inner_state_events(id,fluctlight_id,event_type,payload,revision) VALUES($1,$2,'affect.event',$3,$4) ON CONFLICT(id) DO NOTHING`, eventID, fluctlightID, jsonBytes(payload), newRevision); err != nil {
			return err
		}
		stateRevisionID := "state_revision_" + stableDigest(eventID)
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_state_revisions(id,fluctlight_id,source_event_id,expected_revision,resulting_revision,previous_state,resulting_state,requested_delta,applied_delta,result,reason_code,policy_version,model_version,evidence_refs,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'applied','affect_event','affect.reducer.v1','configured',$10,$11) ON CONFLICT DO NOTHING`, stateRevisionID, fluctlightID, sourceFactID, revision, newRevision, jsonBytes(current), jsonBytes(resulting), jsonBytes(requested), jsonBytes(applied), jsonBytes(event.EvidenceRefs), event.IdempotencyKey); err != nil {
			return err
		}
		if err := appendOutboxTx(ctx, tx, "affect.updated", "fluctlight", fluctlightID, fluctlightID, sourceFactID, "affect:"+eventID, "affect:"+eventID, map[string]any{"event_id": eventID, "type": event.Type, "label": label, "revision": newRevision}); err != nil {
			return err
		}
		output = map[string]any{"event_id": eventID, "type": event.Type, "label": label, "intensity": mapValue(resulting["mood"])["intensity"], "revision": newRevision}
		return nil
	})
	return output, err
}
