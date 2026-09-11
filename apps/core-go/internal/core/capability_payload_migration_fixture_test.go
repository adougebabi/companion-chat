package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// MigrateCapabilityPayload performs the one-time active-row conversion from
// the released tool envelope into the canonical invocation/result payload. It
// removes the old authority keys; completed/failed audit rows are returned
// semantically unchanged and are never rewritten by callers.
func MigrateCapabilityPayload(payload map[string]any, active bool) (map[string]any, error) {
	if payload == nil {
		return nil, errors.New("capability_payload_required")
	}
	result := decodeObject(jsonBytes(payload))
	if !active {
		return result, nil
	}
	if version, present := result["capability_runtime_version"]; present && stringValue(version) != CapabilityRuntimePayloadVersion {
		return nil, errors.New("capability_payload_version_invalid")
	}
	decision := mapValue(result["decision"])
	if _, canonicalPresent := result["capability_invocations"]; canonicalPresent {
		if _, oldRootPresent := result["tool_calls"]; oldRootPresent {
			return nil, errors.New("capability_payload_dual_invocation_authority")
		}
		if _, oldNestedPresent := decision["tool_calls"]; oldNestedPresent {
			return nil, errors.New("capability_payload_dual_invocation_authority")
		}
	}
	oldCalls, hasOldCalls, err := migrationCallsValue(result, decision)
	if err != nil {
		return nil, err
	}
	surface := CapabilitySurfaceAutonomy
	if len(decision) > 0 {
		surface = CapabilitySurfaceConversation
	}
	rootSnapshot, err := migrationSnapshot(result, decision)
	if err != nil {
		return nil, err
	}
	bindings := migrationBindings(result, decision)
	invocations := make([]CapabilityInvocation, 0, len(oldCalls))
	for index, raw := range oldCalls {
		call := mapValue(raw)
		if len(call) == 0 {
			return nil, fmt.Errorf("capability_payload_call_%d_invalid", index)
		}
		id := stringValue(call["id"])
		name := stringValue(call["name"])
		if id == "" {
			id = stringValue(call["call_id"])
		}
		if name == "" {
			name = stringValue(call["capability_name"])
		}
		if id == "" || name == "" {
			return nil, fmt.Errorf("capability_payload_call_%d_identity_invalid", index)
		}
		sourceFactID := stringValue(call["source_fact_id"])
		providerRequestID := stringValue(call["provider_request_id"])
		if sourceFactID == "" {
			sourceFactID = stringValue(result["source_fact_id"])
		}
		if providerRequestID == "" {
			providerRequestID = stringValue(result["provider_request_id"])
		}
		if sourceFactID == "" || providerRequestID == "" {
			return nil, fmt.Errorf("capability_payload_call_%d_provenance_invalid", index)
		}
		arguments, preparedPayload, err := migrateCapabilityCallPayload(name, call["arguments"], call["prepared_payload"], sourceFactID, id)
		if err != nil {
			return nil, fmt.Errorf("capability_payload_call_%d_arguments_invalid: %w", index, err)
		}
		sequence := index
		if rawSequence, ok := call["sequence"]; ok {
			if sequenceValue, ok := numberFloat(rawSequence); ok && sequenceValue >= 0 {
				sequence = int(sequenceValue)
			}
		}
		invocation := CapabilityInvocation{CallID: id, CapabilityName: name, Arguments: arguments, PreparedPayload: preparedPayload, SourceFactID: sourceFactID, ActionID: stringValue(call["action_id"]), ProviderRequestID: providerRequestID, Sequence: sequence, SchemaVersion: CapabilityInvocationSchemaVersion}
		if invocation.ActionID == "" {
			invocation.ActionID = stringValue(result["action_id"])
		}
		invocation.Metadata.Surface = surface
		invocation.Metadata.FluctlightID = stringValue(result["fluctlight_id"])
		invocation.Metadata.ConversationID = stringValue(result["conversation_id"])
		if metadata := mapValue(call["metadata"]); len(metadata) > 0 {
			if value := stringValue(metadata["surface"]); value != "" {
				invocation.Metadata.Surface = CapabilitySurface(value)
			}
			invocation.Metadata.FluctlightID = firstString(stringValue(metadata["fluctlight_id"]), invocation.Metadata.FluctlightID)
			invocation.Metadata.ConversationID = firstString(stringValue(metadata["conversation_id"]), invocation.Metadata.ConversationID)
		}
		if snapshot := mapValue(call["context_snapshot"]); len(snapshot) > 0 {
			migratedSnapshot, snapshotErr := migrateCapabilitySnapshot(snapshot)
			if snapshotErr != nil {
				return nil, fmt.Errorf("capability_payload_call_%d_context_snapshot_invalid: %w", index, snapshotErr)
			}
			invocation.ContextSnapshot = migratedSnapshot
		}
		if binding, ok := bindings[id]; ok {
			invocation.Metadata.OutputBinding = &binding
		}
		if len(invocation.ContextSnapshot) == 0 && len(rootSnapshot) > 0 {
			invocation.ContextSnapshot = mapValue(boundedSnapshotValue(rootSnapshot))
		}
		if identity := mapValue(invocation.ContextSnapshot["identity"]); len(identity) > 0 {
			invocation.Metadata.FluctlightID = firstString(invocation.Metadata.FluctlightID, stringValue(identity["fluctlight_id"]))
			invocation.Metadata.ConversationID = firstString(invocation.Metadata.ConversationID, stringValue(identity["conversation_id"]))
			invocation.ActionID = firstString(invocation.ActionID, stringValue(identity["action_id"]))
		}
		var migratedContext CapabilityContext
		if required := migrationRequiredContext(name); len(required) > 0 {
			if len(invocation.ContextSnapshot) == 0 {
				return nil, fmt.Errorf("capability_payload_call_%d_context_snapshot_missing", index)
			}
			var resolveErr error
			migratedContext, resolveErr = NewSnapshotContextResolver(invocation.ContextSnapshot).Resolve(context.Background(), ContextRequest{}, required)
			if resolveErr != nil {
				return nil, fmt.Errorf("capability_payload_call_%d_context_snapshot_invalid: %w", index, resolveErr)
			}
		}
		var migratedArguments map[string]any
		_ = json.Unmarshal(invocation.Arguments, &migratedArguments)
		if name == "media.image.generate" {
			if rawConcept, found, preparedErr := capabilityPreparedData(invocation, "media_concept"); preparedErr != nil {
				return nil, fmt.Errorf("capability_payload_call_%d_prepared_payload_invalid: %w", index, preparedErr)
			} else if found {
				if preparedErr := validatePreparedMediaConcept(mapValue(rawConcept), stringValue(migratedArguments["intent"])); preparedErr != nil {
					return nil, fmt.Errorf("capability_payload_call_%d_prepared_payload_invalid: %w", index, preparedErr)
				}
			}
		}
		if name == "schedule.replan" {
			if rawPlan, found, preparedErr := capabilityPreparedData(invocation, "schedule_plan"); preparedErr != nil {
				return nil, fmt.Errorf("capability_payload_call_%d_prepared_payload_invalid: %w", index, preparedErr)
			} else if found {
				if preparedErr := validatePreparedSchedulePlan(mapValue(rawPlan), migratedArguments, migratedContext); preparedErr != nil {
					return nil, fmt.Errorf("capability_payload_call_%d_prepared_payload_invalid: %w", index, preparedErr)
				}
			}
		}
		invocations = append(invocations, invocation)
	}
	result["capability_runtime_version"] = CapabilityRuntimePayloadVersion
	result["capability_invocations"] = invocations
	if len(rootSnapshot) > 0 {
		result["capability_context_snapshot"] = mapValue(boundedSnapshotValue(rootSnapshot))
	}
	if oldResults, hasOldResults, resultErr := migrationResultsValue(result); resultErr != nil {
		return nil, resultErr
	} else if hasOldResults {
		result["capability_results"] = oldResults
	}
	delete(result, "tool_calls")
	delete(result, "tool_results")
	if len(decision) > 0 {
		delete(decision, "tool_calls")
		delete(decision, "context_projection")
		delete(decision, "media_concept")
		delete(decision, "visual_concept")
		delete(decision, "media_request")
	}
	_ = hasOldCalls
	return result, nil
}

func migrateCapabilityCallPayload(name string, rawArguments, rawPrepared any, sourceFactID, callID string) (json.RawMessage, json.RawMessage, error) {
	arguments, err := normalizeToolArguments(rawArguments)
	if err != nil {
		return nil, nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(arguments, &object); err != nil || object == nil {
		return nil, nil, errors.New("arguments must be an object")
	}
	payload := CapabilityPreparedPayload{SchemaVersion: CapabilityPreparedPayloadSchemaVersion, Data: map[string]any{}, Provenance: map[string]any{}}
	if rawPrepared != nil {
		preparedBytes := jsonBytes(rawPrepared)
		decoded, decodeErr := decodeCapabilityPreparedPayload(preparedBytes)
		if decodeErr != nil {
			return nil, nil, decodeErr
		}
		payload = decoded
	}
	moveProvenance := func(container map[string]any) {
		for _, key := range []string{"evidence_refs", "idempotency_key"} {
			if value, present := container[key]; present {
				payload.Provenance[key] = value
				delete(container, key)
			}
		}
	}
	moveProvenance(object)
	if name == "affect_event" {
		event := cloneMap(mapValue(object["event"]))
		moveProvenance(event)
		object["event"] = event
	}
	switch name {
	case "media.image.generate":
		if concept := mapValue(object["prepared_concept"]); len(concept) > 0 {
			payload.Data["media_concept"] = concept
			delete(object, "prepared_concept")
		}
		if strings.TrimSpace(stringValue(object["intent"])) == "" {
			return nil, nil, errors.New("image intent is required for active migration")
		}
	case "schedule.replan":
		if len(arrayValue(object["items"])) > 0 {
			payload.Data["schedule_plan"] = cloneMap(object)
			intent := strings.TrimSpace(stringValue(object["intent"]))
			if intent == "" {
				return nil, nil, errors.New("schedule intent is required for active migration")
			}
			object = map[string]any{"intent": intent}
		}
	case "memory_event":
		for _, key := range []string{"operation", "source_fact_id", "conversation_id", "actor_refs", "event_refs", "visibility", "personality_perspectives", "profile_id", "provenance"} {
			delete(object, key)
		}
	case "capability.request":
		delete(object, "side_effect_class")
	case "visual_identity.initialize":
		object = map[string]any{}
	}
	if _, present := payload.Provenance["evidence_refs"]; !present && sourceFactID != "" {
		payload.Provenance["evidence_refs"] = []any{sourceFactID}
	}
	if _, present := payload.Provenance["idempotency_key"]; !present && callID != "" {
		payload.Provenance["idempotency_key"] = "capability:" + callID
	}
	preparedPayload, err := encodeCapabilityPreparedPayload(payload)
	if err != nil {
		return nil, nil, err
	}
	return jsonBytes(object), preparedPayload, nil
}

func migrationCallsValue(result, decision map[string]any) ([]any, bool, error) {
	if value, present := result["capability_invocations"]; present {
		invocations, err := capabilityInvocationsFromValue(value)
		if err != nil {
			return nil, false, err
		}
		raw := make([]any, 0, len(invocations))
		for _, invocation := range invocations {
			var object map[string]any
			if err := json.Unmarshal(jsonBytes(invocation), &object); err != nil {
				return nil, false, err
			}
			raw = append(raw, object)
		}
		return raw, false, nil
	}
	if value, present := decision["tool_calls"]; present {
		if _, ok := value.([]any); !ok {
			return nil, false, errors.New("capability_payload_tool_calls_malformed")
		}
		return value.([]any), true, nil
	}
	if value, present := result["tool_calls"]; present {
		if _, ok := value.([]any); !ok {
			return nil, false, errors.New("capability_payload_tool_calls_malformed")
		}
		return value.([]any), true, nil
	}
	return []any{}, false, nil
}

func migrationSnapshot(result, decision map[string]any) (map[string]any, error) {
	if snapshot := mapValue(result["capability_context_snapshot"]); len(snapshot) > 0 {
		return migrateCapabilitySnapshot(snapshot)
	}
	if projection := mapValue(decision["context_projection"]); len(projection) > 0 {
		if parsed, ok := contextProjectionFromValue(projection); ok {
			return ContextSnapshotFromProjection(parsed), nil
		}
		return nil, errors.New("capability_payload_context_projection_invalid")
	}
	return nil, nil
}

func migrateCapabilitySnapshot(snapshot map[string]any) (map[string]any, error) {
	if len(snapshot) == 0 {
		return nil, nil
	}
	if len(mapValue(snapshot["identity"])) > 0 {
		bounded := mapValue(boundedSnapshotValue(snapshot))
		if len(bounded) == 0 {
			return nil, errors.New("capability_payload_context_snapshot_invalid")
		}
		return bounded, nil
	}
	if parsed, ok := contextProjectionFromValue(snapshot); ok {
		return ContextSnapshotFromProjection(parsed), nil
	}
	return nil, errors.New("capability_payload_context_snapshot_invalid")
}

func migrationRequiredContext(name string) []ContextSlot {
	switch name {
	case "media.image.generate":
		return []ContextSlot{SlotVisualIdentity, SlotCurrentLife, SlotAppearance, SlotCurrentState}
	case "visual_identity.initialize":
		return []ContextSlot{SlotCorePersona, SlotVisualIdentity}
	case "scene_event":
		return []ContextSlot{SlotCurrentLife, SlotSchedule}
	case "presence_event":
		return []ContextSlot{SlotCurrentLife}
	case "schedule.replan":
		return []ContextSlot{SlotSchedule, SlotCurrentLife, SlotAgency}
	case "memory_event":
		return []ContextSlot{SlotCorePersona, SlotMemoryScope}
	case "affect_event":
		return []ContextSlot{SlotCurrentState}
	case "relationship.lookup":
		return []ContextSlot{SlotRelationshipScope}
	default:
		return nil
	}
}

func migrationBindings(result, decision map[string]any) map[string]OutputBindingV1 {
	bindings := make(map[string]OutputBindingV1)
	for _, raw := range append(arrayValue(result["output_bindings"]), arrayValue(decision["output_bindings"])...) {
		binding := mapValue(raw)
		if id := stringValue(binding["tool_call_id"]); id != "" {
			bindings[id] = OutputBindingV1{ToolCallID: id, TargetKind: stringValue(binding["target_kind"]), TargetRef: stringValue(binding["target_ref"])}
		}
	}
	return bindings
}

func migrationResultsValue(result map[string]any) ([]CapabilityResult, bool, error) {
	value, present := result["capability_results"]
	if present {
		results, err := capabilityResultsFromValue(value)
		return results, true, err
	}
	value, present = result["tool_results"]
	if !present {
		return nil, false, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, false, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, false, fmt.Errorf("capability_payload_results_malformed: %w", err)
	}
	results := make([]CapabilityResult, 0, len(raw))
	for index, item := range raw {
		callID, name := stringValue(item["tool_call_id"]), stringValue(item["name"])
		if callID == "" || name == "" {
			return nil, false, fmt.Errorf("capability_payload_result_%d_identity_invalid", index)
		}
		status := stringValue(item["status"])
		switch status {
		case "completed", "failed", "rejected", "deferred":
		default:
			return nil, false, fmt.Errorf("capability_payload_result_%d_status_invalid", index)
		}
		results = append(results, CapabilityResult{CallID: callID, CapabilityName: name, Status: status, Output: item["output"], ErrorCode: stringValue(item["error_code"]), Retryable: boolValue(item["retryable"]), ProviderRequestID: stringValue(item["provider_request_id"]), CorrelationID: stringValue(item["correlation_id"])})
	}
	return results, true, nil
}

func boolValue(value any) bool {
	v, _ := value.(bool)
	return v
}
