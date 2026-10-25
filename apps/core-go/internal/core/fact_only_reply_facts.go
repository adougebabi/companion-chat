package core

import "context"

// The Writer is a child of the current native loop. Optional model-selected
// refs must not decide whether it learns that a just-attempted action failed.
// Read the server-owned invocation/result pairs, never the model's arguments,
// final reasoning, or unrelated historical outcomes.
func (s *factOnlyReplyService) currentActionFacts(ctx context.Context, reply CapabilityInvocation) []map[string]any {
	bridge, ok := adkCapabilityContext(ctx)
	if !ok || bridge.Trace == nil || reply.CapabilityName != conversationReplyCapabilityName {
		return nil
	}
	invocations, results := bridge.Trace.Snapshot()
	authorized := map[string]CapabilityInvocation{}
	registry := s.app.capabilityRegistryForContext(ctx)
	for _, call := range invocations {
		if call.Metadata.AuthorizationActorID != reply.Metadata.AuthorizationActorID ||
			call.Metadata.FluctlightID != reply.Metadata.FluctlightID ||
			call.Metadata.ConversationID != reply.Metadata.ConversationID || call.SourceFactID != reply.SourceFactID {
			continue
		}
		tool, found := registry.Lookup(call.CapabilityName)
		if !found || (tool.Definition().Type != CapabilityTypeAction && call.CapabilityName != momentInspectCapabilityName) || call.CapabilityName == conversationReplyCapabilityName {
			continue
		}
		authorized[call.CallID] = call
	}
	facts := []map[string]any{}
	for _, result := range results {
		call, found := authorized[result.CallID]
		if !found || result.CapabilityName != call.CapabilityName {
			continue
		}
		if call.CapabilityName == momentInspectCapabilityName {
			if result.Status == "completed" {
				facts = append(facts, map[string]any{"kind": "published_moments", "value": factOnlyFields(mapValue(result.Output), "items", "has_more")})
			}
			continue
		}
		value := map[string]any{"capability_name": result.CapabilityName, "status": result.Status}
		if result.ErrorCode != "" {
			value["error_code"] = result.ErrorCode
		}
		value["observed"] = factOnlyFields(mapValue(result.Output), "disposition", "warning_code", "effect_applied", "mutation_committed", "handled", "retry_same_arguments")
		if call.CapabilityName == chatPromiseCapabilityName && result.Status == "accepted" {
			value["observed"] = factOnlyFields(mapValue(result.Output), "promise_status", "accepted_by_character", "scheduled", "window_start", "window_end")
		}
		if call.CapabilityName == "moment.publish" && result.Status == "completed" && mapValue(result.Output)["target_kind"] == "moment" {
			mapValue(value["observed"])["published_text"] = stringValue(mapValue(result.Output)["text"])
		}
		facts = append(facts, map[string]any{"kind": "current_action_result", "value": value})
	}
	return facts
}

// Entity snapshots are not Writer inputs. In particular, a valid Goal ref can
// point to execution advice; the Writer reads all visible Goals through its own
// projection. Only typed statement/result facts need additional material here.
func factOnlyReplyEvidenceFact(entry ContextReference, projection ContextProjection) map[string]any {
	row := decodeObject(entry.Snapshot)
	var value map[string]any
	switch entry.Kind {
	case ContextReferenceOutcome:
		value = factOnlyFields(row, "capability_name", "status", "error_code", "occurred_at")
		if observed := providerSafeOutcomeSignal(mapValue(row["observed"]), []string{"status", "type", "target_kind", "delivery_status", "reason_code"}); len(observed) > 0 {
			value["observed"] = observed
		}
	case ContextReferenceActorFact:
		if row["assertion_type"] == "inference" || row["epistemic_kind"] == "inference" {
			return nil
		}
		value = factOnlyFields(row, "attribute", "value", "epistemic_kind", "effective_at", "valid_until")
		value["subject"] = factOnlyActor(actorRefForID(projection.Actors, stringValue(row["actor_id"])))
	default:
		return nil
	}
	return map[string]any{"kind": string(entry.Kind), "value": value, "certainty": "仅证明对应来源的陈述或真实工具状态；不证明语义成立、对方接受或Goal完成。"}
}

func factOnlyReplyToolFacts(facts []map[string]any) []map[string]any {
	result := []map[string]any{}
	for _, fact := range facts {
		kind := stringValue(fact["kind"])
		var value any
		switch kind {
		case "published_moments":
			value = factOnlyFields(mapValue(fact["value"]), "items", "has_more")
		case "current_action_result":
			value = factOnlyFields(mapValue(fact["value"]), "capability_name", "status", "error_code", "observed")
		case "media_action_result":
			value = factOnlyFields(mapValue(fact["value"]), "kind", "intent", "status", "error_code")
		case "private_candidate_text":
			// This value can only be supplied by the durable draft owner, never
			// by native arguments or the parent's reasoning/final response.
			value = stringValue(fact["value"])
		case string(ContextReferenceOutcome):
			value = factOnlyFields(mapValue(fact["value"]), "capability_name", "status", "error_code", "occurred_at", "observed")
		case string(ContextReferenceActorFact):
			value = factOnlyFields(mapValue(fact["value"]), "attribute", "value", "epistemic_kind", "effective_at", "valid_until", "subject")
		default:
			continue
		}
		result = append(result, map[string]any{"kind": kind, "value": value, "certainty": "真实工具/来源状态；private_candidate_text仍未发送，媒体ready不证明看过或送达，不作为社交指令。"})
	}
	return result
}
