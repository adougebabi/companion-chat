package core

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
