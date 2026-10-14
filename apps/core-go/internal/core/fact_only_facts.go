package core

import (
	"sort"
	"strings"
)

type factOnlyMemoryView struct {
	Input       WorkingMemoryInput
	Diagnostics map[string]any
	Conflicts   []map[string]any
}

func factOnlyMemoryCandidates(projection ContextProjection, active, summaries []map[string]any, cognition bool) factOnlyMemoryView {
	result := factOnlyMemoryView{Diagnostics: map[string]any{}}
	type candidate struct {
		fragment PromptFragment
		sources  []string
		records  []string
		key      string
	}
	items := []candidate{}
	byKey := map[string]int{}
	sourceClaims := map[string][]int{}
	sourceCount, excluded := 0, 0
	groups := []struct {
		rows    []map[string]any
		kind    PromptFragmentKind
		source  string
		refKind ContextReferenceKind
	}{
		{active, PromptFragmentActiveMemory, "active_memory", ContextReferenceActiveMemory},
		{projection.ResidentMemories, PromptFragmentResidentMemory, "memory", ContextReferenceMemory},
		{projection.Memories, PromptFragmentRetrievedMemory, "memory", ContextReferenceMemory},
	}
	for _, group := range groups {
		for _, row := range group.rows {
			sourceCount++
			content := stringValue(row["content"])
			status := stringValue(row["status"])
			if strings.TrimSpace(content) == "" || factOnlyExpired(row, "valid_until", projection.AsOf) || factOnlyNotYetEffective(row, projection.AsOf) || (status != "" && status != "active") {
				excluded++
				continue
			}
			value := factOnlyFields(row, "type", "kind", "content", "confidence", "importance", "epistemic_kind", "provenance_status", "source_validity", "occurred_at", "created_at", "valid_from", "valid_until", "original_time_expression", "time_precision", "timezone")
			value["source_kind"] = group.source
			value["certainty"] = "candidate_not_event_proof"
			value["subject"] = "unknown"
			var associated []map[string]any
			for _, raw := range arrayValue(row["actor_refs"]) {
				if actor := actorRefForID(projection.Actors, stringValue(raw)); len(actor) > 0 {
					associated = append(associated, factOnlyActor(actor))
				}
			}
			if len(associated) > 0 {
				value["associated_actors"] = associated
			}
			if perspective := memoryPerspectiveForProfile(arrayValue(row["personality_perspectives"]), stringValue(projection.PersonalityRuntime["active_profile_id"])); len(perspective) > 0 {
				value["profile_interpretation"] = perspective
			}
			sources := factOnlySourcePointers(row)
			timeIdentity := row["occurred_at"]
			if stringValue(timeIdentity) == "" {
				timeIdentity = sources
			}
			// Unknown time does not make unrelated equal prose the same event.
			key := stableDigest(jsonString(map[string]any{"content": content, "source_kind": group.source, "type": row["type"], "kind": row["kind"],
				"actors": sortedUniqueStrings(decisionServiceRefValues(arrayValue(row["actor_refs"]))), "conversation": row["conversation_id"],
				"occurred_at": timeIdentity, "valid_from": row["valid_from"], "valid_until": row["valid_until"],
				"epistemic_kind": row["epistemic_kind"], "provenance_status": row["provenance_status"], "source_validity": row["source_validity"]}))
			if index, exists := byKey[key]; exists {
				items[index].sources = sortedUniqueStrings(append(items[index].sources, sources...))
				items[index].records = sortedUniqueStrings(append(items[index].records, stringValue(row["id"])))
				for _, source := range sources {
					found := false
					for _, existing := range sourceClaims[source] {
						found = found || existing == index
					}
					if !found {
						sourceClaims[source] = append(sourceClaims[source], index)
					}
				}
				continue
			}
			if cognition {
				factOnlyAddRef(value, row, projection.ReferenceIndex, group.refKind)
				value = mapValue(providerEncodeContextRefs(value, projection.ReferenceIndex))
			}
			index := len(items)
			byKey[key] = index
			items = append(items, candidate{fragment: PromptFragment{Kind: group.kind, Priority: int(numberOrZero(row["importance"]) * 100), Content: value,
				SourceRefs: []string{"fact-view:" + key}}, sources: sources, records: []string{stringValue(row["id"])}, key: key})
			for _, source := range sources {
				sourceClaims[source] = append(sourceClaims[source], index)
			}
		}
	}
	lineage := map[string]any{}
	for _, item := range items {
		lineage[item.key] = map[string]any{"source_refs": item.sources, "record_ids": item.records}
		switch item.fragment.Kind {
		case PromptFragmentActiveMemory:
			result.Input.ActiveCandidates = append(result.Input.ActiveCandidates, item.fragment)
		case PromptFragmentResidentMemory:
			result.Input.ResidentCandidates = append(result.Input.ResidentCandidates, item.fragment)
		default:
			result.Input.RetrievedMemories = append(result.Input.RetrievedMemories, item.fragment)
		}
	}
	// Same evidence can support distinct claims. It is not independent proof,
	// and shared provenance alone is not enough to decide a contradiction.
	seenConflicts := map[string]bool{}
	sourceKeys := make([]string, 0, len(sourceClaims))
	for key := range sourceClaims {
		sourceKeys = append(sourceKeys, key)
	}
	sort.Strings(sourceKeys)
	for _, sourceKey := range sourceKeys {
		indexes := sourceClaims[sourceKey]
		if len(indexes) < 2 {
			continue
		}
		key := jsonString(indexes)
		if seenConflicts[key] {
			continue
		}
		seenConflicts[key] = true
		claims := []map[string]any{}
		for _, index := range indexes {
			value := mapValue(items[index].fragment.Content)
			claims = append(claims, map[string]any{"content_excerpt": factOnlyExcerpt(stringValue(value["content"]), 240), "source_kind": value["source_kind"], "epistemic_kind": value["epistemic_kind"], "occurred_at": value["occurred_at"], "valid_from": value["valid_from"], "valid_until": value["valid_until"]})
		}
		result.Conflicts = append(result.Conflicts, map[string]any{"kind": "shared_source_distinct_claims", "status": "unresolved_not_independent_support", "claims": claims})
	}
	for _, row := range summaries {
		content := factOnlyFields(row, "summary", "started_at", "ended_at", "local_date", "timezone")
		if strings.TrimSpace(stringValue(content["summary"])) == "" {
			continue
		}
		content["evidence_semantics"] = "historical_summary_not_current_authority"
		refs := []string{"summary:" + firstString(row["id"], stableDigest(jsonString(content)))}
		for _, source := range arrayValue(row["source_message_refs"]) {
			if ref := stringValue(source); strings.HasPrefix(ref, "message:") {
				refs = append(refs, ref)
			}
		}
		result.Input.Summaries = append(result.Input.Summaries, PromptFragment{Kind: PromptFragmentSummary, Required: row["required"] == true,
			Priority: intValue(row["to_sequence"]), Content: content, SourceRefs: refs})
	}
	result.Conflicts = append(result.Conflicts, factOnlyActorConflicts(projection)...)
	result.Diagnostics["source_count"] = sourceCount
	result.Diagnostics["candidate_count"] = len(items)
	result.Diagnostics["deduplicated_count"] = sourceCount - excluded - len(items)
	result.Diagnostics["excluded_empty_inactive_or_outside_validity"] = excluded
	result.Diagnostics["sources"] = lineage
	result.Diagnostics["normalization"] = "original_text_preserved_no_unproved_unescaping"
	return result
}

func factOnlySourcePointers(row map[string]any) []string {
	result := []string{}
	for _, key := range []string{"effective_source_refs", "evidence_refs", "event_refs"} {
		result = append(result, decisionServiceRefValues(arrayValue(row[key]))...)
	}
	if len(result) == 0 {
		result = append(result, firstString(row["source_fact_id"], firstString(row["id"], "unknown:"+stableDigest(jsonString(row)))))
	}
	return sortedUniqueStrings(result)
}

func factOnlyActorConflicts(projection ContextProjection) []map[string]any {
	values := map[string][]map[string]any{}
	for _, row := range projection.ActorFacts {
		key := stringValue(row["actor_id"]) + ":" + stringValue(row["attribute"])
		if row["status"] != "active" || factOnlyExpired(row, "valid_until", projection.AsOf) {
			continue
		}
		values[key] = append(values[key], row)
	}
	result := []map[string]any{}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		rows := values[key]
		seen := map[string]bool{}
		options := []map[string]any{}
		for _, row := range rows {
			key := jsonString(row["value"])
			if seen[key] {
				continue
			}
			seen[key] = true
			options = append(options, factOnlyFields(row, "value", "effective_at", "epistemic_kind"))
		}
		if len(options) > 1 {
			result = append(result, map[string]any{"kind": "actor_attribute_values_differ", "subject": factOnlyActor(actorRefForID(projection.Actors, stringValue(rows[0]["actor_id"]))), "attribute": rows[0]["attribute"], "options": options, "status": "unresolved"})
		}
	}
	return result
}

func factOnlyCognitionMemoryInput(projection ContextProjection, active, summaries []map[string]any) WorkingMemoryInput {
	view := factOnlyContextFacts(projection, true)
	facts := factOnlyMemoryCandidates(projection, active, summaries, true)
	if len(facts.Conflicts) > 0 {
		view.Facts["conflicts"] = facts.Conflicts
	}
	input := facts.Input
	input.RuntimeFacts = factOnlyRuntimeFragments(view, projection, true)
	input.RecentMessages = factOnlyHistory(projection, projection.CurrentUserText)
	return input
}

func factOnlyExcerpt(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}
