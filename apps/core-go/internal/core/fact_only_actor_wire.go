package core

// Factor only identical, consecutive Actor provenance fields on the outbound
// copy. Each fact keeps its value (including null/false), state, ref and any
// interpretation boundary. No source row, scope or reference binding changes.
func factOnlyActorBackgroundWire(value any) any {
	rows := arrayValue(value)
	if len(rows) < 2 {
		return value
	}
	groups := []any{}
	last := ""
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return value
		}
		fact := mapValue(factOnlyCopyValue(row))
		common := map[string]any{}
		// Effective times are often distinct even for one submitted form. Keep
		// every field's precise timestamp on that fact; never merge its time.
		for _, key := range []string{"subject", "epistemic_kind", "transition_kind"} {
			if v, exists := fact[key]; exists {
				common[key] = v
				delete(fact, key)
			}
		}
		signature := jsonString(common)
		if len(groups) == 0 || signature != last {
			groups = append(groups, map[string]any{"common": common, "facts": []any{}})
			last = signature
		}
		group := mapValue(groups[len(groups)-1])
		group["facts"] = append(arrayValue(group["facts"]), fact)
	}
	if len(groups) == len(rows) {
		return value
	}
	return map[string]any{"encoding": "each_fact_inherits_common_fields", "groups": groups}
}

// Factor repeated Actor provenance deterministically, without token policy.
func renderFactOnlyRuntimeContext(value map[string]any) string {
	copy := cloneMap(value)
	if background, ok := copy["actor_background"]; ok {
		copy["actor_background"] = factOnlyActorBackgroundWire(background)
	}
	return renderProviderRuntimeContext(copy)
}
