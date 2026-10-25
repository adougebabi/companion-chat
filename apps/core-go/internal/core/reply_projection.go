package core

import (
	"context"
	"strings"
	"time"
)

// Only callers that have validated the private conversation transport use this
// projection. Online/busy, a meeting plan and relationship distance are not
// physical co-presence evidence.
func privateTextChatProjection(projection ContextProjection) ContextProjection {
	projection.CommunicationState = cloneMap(projection.CommunicationState)
	if projection.CommunicationState == nil {
		projection.CommunicationState = map[string]any{}
	}
	projection.CommunicationState["channel"] = "text_chat"
	projection.CommunicationState["channel_source"] = "validated_private_conversation"
	return projection
}

func privateChatCommunicationView(projection ContextProjection) map[string]any {
	view := map[string]any{"channel": "text_chat", "channel_source": "validated_private_conversation", "co_presence": "unknown", "co_presence_source": "no_current_explicit_evidence", "recipient_location": map[string]any{"status": "unknown"}}
	life := projection.LifeContext
	if len(life) == 0 {
		life = mapValue(mapValue(projection.CurrentState["data"])["life_context"])
	}
	if location := stringValue(life["location"]); location != "" && !factOnlyExpired(life, "expires_at", projection.AsOf) {
		view["self_location"] = factOnlyFields(life, "location", "effective_at", "expires_at")
		view["self_location_source"] = "effective_life_context"
	} else {
		view["self_location"] = map[string]any{"status": "unknown"}
	}
	for _, fact := range projection.ActorFacts {
		if stringValue(fact["actor_id"]) != stringValue(projection.CurrentSpeaker["actor_id"]) || factOnlyExpired(fact, "valid_until", projection.AsOf) || factOnlyNotYetEffective(map[string]any{"valid_from": firstString(fact["valid_from"], stringValue(fact["effective_at"]))}, projection.AsOf) || stringValue(fact["epistemic_kind"]) == "inference" || (stringValue(fact["status"]) != "" && fact["status"] != "active") {
			continue
		}
		attribute := stringValue(fact["attribute"])
		if attribute == "location" {
			view["recipient_location"] = factOnlyFields(fact, "value", "epistemic_kind", "effective_at", "valid_from", "valid_until", "source_kind", "source_pointer")
			view["recipient_location_source"] = "actor_fact"
		}
		// Only an explicit current fact with a recorded source can establish this
		// read view; a bare boolean in communication_state or profile cannot.
		if attribute != "co_located" || fact["value"] == nil || len(factOnlyFields(fact, "source_message_id", "source_fact_id", "source_kind", "source_ref", "fact_id")) == 0 {
			continue
		}
		if value, ok := fact["value"].(bool); ok {
			if value {
				view["co_presence"] = "same_place"
			} else {
				view["co_presence"] = "different_place"
			}
			view["co_presence_source"] = "current_actor_fact"
			view["co_presence_evidence"] = factOnlyFields(fact, "epistemic_kind", "effective_at", "valid_from", "valid_until", "source_kind", "source_pointer")
		}
	}
	return view
}

func applyPrivateReplyView(view *factOnlyContextView, projection ContextProjection, capabilities map[string]any) {
	view.Facts["communication_state"] = privateChatCommunicationView(projection)
	state := mapValue(view.Facts["current_state"])
	data := mapValue(state["data"])
	if _, exists := data["appearance"]; exists && capabilities["persona_detail_registered"] == true {
		appearance := factOnlyFields(mapValue(data["appearance"]), "wearing_state", "worn_items")
		appearance["source_kind"], appearance["details"], appearance["wearing_completeness"] = "effective_appearance", "available_via_persona.detail_current_appearance", "partial_unless_explicitly_complete"
		data["appearance"] = appearance
		view.Selection["appearance_selection"] = "omit_unrelated_details; current detail remains available through scoped read-only query"
	} else {
		view.Selection["appearance_selection"] = "retain_authority_details_when_scoped_query_unavailable"
	}
	view.Selection["medium_rule_version"] = replyMediumVersion
	view.Selection["communication_sources"] = view.Facts["communication_state"]
}

func privateReplyPersonaView(persona, source map[string]any) (map[string]any, map[string]any) {
	original := jsonString(persona)
	result := cloneMap(persona)
	// Free text is deliberately retained; no lexical classification or prose
	// rewriting. Structured/source mappings are provenance, not instructions.
	body := mapValue(result[workingPersonaBodyKey])
	if len(body) == 0 {
		body = cloneMap(deriveWorkingPersonaBody(persona))
		if body == nil {
			body = map[string]any{}
		}
		result[workingPersonaBodyKey] = body
	}
	body["interpretation_boundary"] = "原文完整保留。性格、表达、冲突方式、亲密倾向、弱点、反差和条件均有效；习惯是条件性倾向而非本轮动作。喜好不证明当前状态；视觉设定不替代当前外貌。交流媒介由运行协议独立规定，不改变人格。"
	diagnostics := cloneMap(source)
	if diagnostics == nil {
		diagnostics = map[string]any{}
	}
	diagnostics["raw_view_hash"] = stableDigest(original)
	diagnostics["reply_view_hash"] = stableDigest(jsonString(result))
	diagnostics["view_version"] = replyViewVersion
	if stringValue(diagnostics["source_kind"]) == "" {
		diagnostics["source_kind"] = "available_working_persona_text_not_original_import"
	}
	if strings.TrimSpace(stringValue(mapValue(persona["working_persona"])["portrait_text"])) != "" {
		diagnostics["mapping"] = "working_persona.portrait_text retained verbatim"
	} else {
		diagnostics["mapping"] = "available structured working persona retained verbatim; no inferred categories"
	}
	return result, diagnostics
}

func labelPrivateReplyHistory(history []PromptFragment) {
	for index := range history {
		content := mapValue(history[index].Content)
		metadata := mapValue(content["metadata"])
		metadata["text_semantics"] = "past_message_verbatim_not_current_event_proof"
		metadata["style_semantics"] = "past_style_not_current_format_rule"
		metadata["projection"] = "verbatim_no_reliable_speech_narration_split"
	}
}

// Preserve source/time metadata for the call-local communication view without
// changing the shared Actor Fact query's public payload or write semantics.
func enrichPrivateCommunicationFacts(ctx context.Context, q lifeContextQuerier, owner, subject string, facts []map[string]any) error {
	ids := []string{}
	for _, fact := range facts {
		if stringValue(fact["actor_id"]) == subject && (fact["attribute"] == "location" || fact["attribute"] == "co_located") {
			if id := stringValue(fact["fact_id"]); id != "" {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.Query(ctx, `SELECT id,source_ref,source_message_id,valid_until FROM public.actor_facts WHERE owner_fluctlight_id=$1 AND id=ANY($2::varchar[])`, owner, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, source string
		var message *string
		var until *time.Time
		if err := rows.Scan(&id, &source, &message, &until); err != nil {
			return err
		}
		for _, fact := range facts {
			if fact["fact_id"] != id {
				continue
			}
			fact["source_kind"] = "current_actor_fact"
			if message != nil {
				fact["source_kind"] = "conversation_message_assertion"
			}
			fact["source_ref"] = source
			fact["source_pointer"] = "actor_fact_source:" + stableDigest(id+"\x1f"+source)
			if until != nil {
				fact["valid_until"] = formatInstant(*until)
			}
		}
	}
	return rows.Err()
}

func appendPrivateCoFacts(facts, extra []map[string]any) []map[string]any {
	result := append([]map[string]any(nil), facts...)
	for _, fact := range extra {
		found := false
		for _, existing := range result {
			if existing["fact_id"] == fact["fact_id"] {
				found = true
				break
			}
		}
		if !found {
			result = append(result, fact)
		}
	}
	return result
}
