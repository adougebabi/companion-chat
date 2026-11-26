package core

import (
	"sort"
	"strings"
)

// directReplyServerOwnedDecisionKeys are written by Core while freezing a
// decision. A model copy of them is discarded instead of failing the reply.
var directReplyServerOwnedDecisionKeys = []string{
	"context_reference_index", "context_reference_version", "goal_refs",
	"intention_refs", "personality_transition", "cognitive_state_transition",
}

// sanitizeDirectReplyDecision keeps the visible reply publishable. Only the
// text is the user-facing product; the accompanying cognition decision is
// best-effort metadata. Fields that do not validate against the schema or the
// current reference index are dropped (and reported) rather than rejecting the
// whole reply and asking the model to try again.
func sanitizeDirectReplyDecision(raw map[string]any, projection ContextProjection) (map[string]any, []string) {
	decision := cloneMap(raw)
	if decision == nil {
		decision = map[string]any{}
	}
	dropped := []string{}
	drop := func(key string) {
		if _, present := decision[key]; present {
			delete(decision, key)
			dropped = append(dropped, key)
		}
	}
	for _, key := range directReplyServerOwnedDecisionKeys {
		drop(key)
	}
	schema := cognitiveTurnResponseSchema()
	properties := mapValue(schema["properties"])
	for key := range decision {
		if _, known := properties[key]; !known {
			drop(key)
		}
	}
	if stringValue(decision["action_type"]) != "reply" {
		decision["action_type"] = "reply"
	}
	if strings.TrimSpace(stringValue(decision["response_intent"])) == "" {
		decision["response_intent"] = "reply"
	}
	if validateCapabilitySchemaValue(decision["influences"], mapValue(properties["influences"])) != nil {
		if _, present := decision["influences"]; present {
			dropped = append(dropped, "influences")
		}
		decision["influences"] = []any{}
	}
	if validateCapabilitySchemaValue(decision, schema) != nil {
		keys := make([]string, 0, len(decision))
		for key := range decision {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if key == "action_type" || key == "response_intent" || key == "influences" {
				continue
			}
			if validateCapabilitySchemaValue(decision[key], mapValue(properties[key])) != nil {
				drop(key)
			}
		}
	}
	if validateGoalEventCandidates(decision, projection.ReferenceIndex) != nil {
		drop("goal_event_candidates")
	}
	if !directReplyDecisionFreezes(decision, projection) {
		// Keep each influence that is valid on its own; drop the rest.
		kept := []any{}
		for _, item := range arrayValue(decision["influences"]) {
			probe := cloneMap(decision)
			probe["influences"] = []any{item}
			delete(probe, "appraisal")
			if directReplyDecisionFreezes(probe, projection) {
				kept = append(kept, item)
			}
		}
		if len(kept) != len(arrayValue(decision["influences"])) {
			dropped = append(dropped, "influences")
		}
		decision["influences"] = kept
		if !directReplyDecisionFreezes(decision, projection) {
			// Remaining failures come from appraisal evidence or drive signals.
			drop("appraisal")
		}
	}
	if _, err := normalizeResponsePlan(cloneMap(decision), projection.SourceFactID, projection); err != nil {
		drop("response_plan")
	}
	return decision, dropped
}

func directReplyDecisionFreezes(decision map[string]any, projection ContextProjection) bool {
	_, err := freezeDecisionInfluences(cloneMap(decision), projection, false)
	return err == nil
}
