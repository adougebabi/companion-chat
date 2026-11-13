package core

import (
	"context"
	"strings"
)

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

// Moment already receives Goals through its independent Goal view. Actor
// facts have a separate Core envelope and an opaque owner-supplied value.
func momentReplyEvidenceFact(entry ContextReference, projection ContextProjection) map[string]any {
	row := decodeObject(entry.Snapshot)
	if stringValue(row["assertion_type"]) == "inference" {
		return nil
	}
	switch entry.Kind {
	case ContextReferenceGoal:
		return nil
	case ContextReferenceActorFact:
		return factOnlyReplyEvidenceFact(entry, projection)
	case ContextReferenceRelationship:
		value := momentReplyRelationshipEvidence(row)
		if targetID := stringValue(row["target_actor_id"]); targetID != "" {
			actor := actorRefForID(projection.Actors, targetID)
			if len(actor) == 0 {
				actor = map[string]any{"type": row["target_actor_type"]}
			}
			value["target_actor"] = momentReplyAssociatedActor(actor)
		}
		return map[string]any{"kind": string(entry.Kind), "value": value, "certainty": "关系记录描述当前对象，不证明其他事件已经发生。"}
	case ContextReferenceActiveMemory:
		value := momentReplyActiveMemoryEvidence(row)
		if refs := arrayValue(row["actor_refs"]); len(refs) > 0 {
			associated := make([]map[string]any, 0, len(refs))
			for _, raw := range refs {
				associated = append(associated, momentReplyAssociatedActor(actorRefForID(projection.Actors, stringValue(raw))))
			}
			value["associated_actors"] = associated
		}
		return map[string]any{"kind": string(entry.Kind), "value": value, "certainty": "关联人物不一定是内容主体；记忆或转述不能代替当前状态或真实结果。"}
	case ContextReferenceDrive, ContextReferencePreference, ContextReferenceTrigger:
		value := copyPromptFields(row, "key", "label", "description", "value_schema", "trigger_schema", "value", "confidence", "status", "updated_at", "pressure", "salience", "direction")
		return map[string]any{"kind": string(entry.Kind), "value": value, "certainty": "对应类型的当前记录；不代表外部事件已经发生。"}
	case ContextReferenceMemory, ContextReferenceIntention, ContextReferenceDevelopingSelf:
		return nil
	}
	return map[string]any{"kind": string(entry.Kind), "value": replySemanticValue(row), "certainty": "仅按对应类型的权威状态/真实消息解释，引用本身不证明事件发生"}
}

// The relationship envelope is Core-owned; its declared domain values remain
// opaque data and must not be recursively filtered by field-name suffix.
func momentReplyRelationshipEvidence(row map[string]any) map[string]any {
	result := copyPromptFields(row, "role", "metrics", "trend", "summary", "emotional_association", "is_current_user")
	if target := mapValue(row["target_actor"]); len(target) > 0 {
		result["target_actor"] = momentReplyAssociatedActor(target)
	}
	return result
}

func momentReplyActiveMemoryEvidence(row map[string]any) map[string]any {
	result := copyPromptFields(row, "kind", "content", "status", "confidence", "importance", "source_validity", "original_time_expression", "valid_from", "valid_until", "time_precision", "timezone", "last_relevant_at", "created_at", "updated_at")
	if raw, exists := row["associated_actors"]; exists {
		associated := make([]map[string]any, 0)
		for _, item := range arrayValue(raw) {
			associated = append(associated, momentReplyAssociatedActor(mapValue(item)))
		}
		result["associated_actors"] = associated
	}
	return result
}

func momentReplyAssociatedActor(row map[string]any) map[string]any {
	view := copyPromptFields(row, "type", "role", "label", "display_name")
	alias := firstString(row["actor"], stringValue(row["ref"]))
	suffix := []rune(strings.TrimPrefix(alias, "actor_"))
	// Actor projection continues with one-rune aliases after actor_z.
	if alias == "actor_self" || alias == "actor_user" || strings.HasPrefix(alias, "actor_") && len(suffix) == 1 && suffix[0] >= 'a' {
		view["actor"] = alias
	} else {
		view["actor"] = "unknown"
	}
	return view
}

func momentReplyToolFacts(facts []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(facts))
	for _, fact := range facts {
		switch stringValue(fact["kind"]) {
		case string(ContextReferenceGoal):
			continue
		case string(ContextReferenceActorFact):
			value := mapValue(fact["value"])
			if value["assertion_type"] == "inference" || value["epistemic_kind"] == "inference" {
				continue
			}
			item := copyPromptFields(fact, "kind", "certainty")
			item["value"] = copyPromptFields(value, "attribute", "value", "epistemic_kind", "effective_at", "valid_until", "subject")
			result = append(result, item)
		case string(ContextReferenceActiveMemory):
			item := copyPromptFields(fact, "kind", "certainty")
			item["value"] = momentReplyActiveMemoryEvidence(mapValue(fact["value"]))
			result = append(result, item)
		case string(ContextReferenceRelationship):
			item := copyPromptFields(fact, "kind", "certainty")
			item["value"] = momentReplyRelationshipEvidence(mapValue(fact["value"]))
			result = append(result, item)
		case string(ContextReferenceAppearance):
			item := copyPromptFields(fact, "kind", "certainty")
			item["value"] = copyPromptFields(mapValue(fact["value"]), "wearing_state", "body_fields", "worn_items", "used_items")
			result = append(result, item)
		case string(ContextReferenceDrive), string(ContextReferencePreference), string(ContextReferenceTrigger):
			item := copyPromptFields(fact, "kind", "certainty")
			item["value"] = copyPromptFields(mapValue(fact["value"]), "key", "label", "description", "value_schema", "trigger_schema", "value", "confidence", "status", "updated_at", "pressure", "salience", "direction")
			result = append(result, item)
		default:
			result = append(result, mapValue(replySemanticValue(fact)))
		}
	}
	return result
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
