package core

import "context"

// Resolve provenance before rendering. Owner-provided injury values are opaque
// data, while confirmed body events have specific Core-owned identity fields.
func (s *factOnlyReplyService) appearanceEvidenceFact(ctx context.Context, entry ContextReference, projection ContextProjection) (map[string]any, error) {
	if entry.EntityID != projection.FluctlightID {
		return nil, ErrUnauthorized
	}
	row := decodeObject(entry.Snapshot)
	injuries := arrayValue(mapValue(mapValue(row["body_fields"])["injuries"])["value"])
	ids := []string{}
	seen := map[string]bool{}
	for _, raw := range injuries {
		for _, key := range []string{"id", "recovery_event_id"} {
			id := stringValue(mapValue(raw)[key])
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	events := map[string]map[string]any{}
	if len(ids) > 0 {
		rows, err := s.app.DB.Pool().Query(ctx, `SELECT id,kind,result->'body_effect' FROM public.life_events WHERE fluctlight_id=$1 AND id=ANY($2::text[]) AND status='confirmed' AND kind IN ('body_injury','body_recovery')`, projection.FluctlightID, ids)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id, kind string
			var effect []byte
			if err := rows.Scan(&id, &kind, &effect); err != nil {
				return nil, err
			}
			events[id] = map[string]any{"kind": kind, "effect": decodeObject(effect)}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	view := copyPromptFields(row, "wearing_state")
	fields := map[string]any{}
	for key, raw := range mapValue(row["body_fields"]) {
		field := mapValue(raw)
		if len(field) == 0 {
			fields[key] = copyPromptData(raw)
			continue
		}
		value := copyPromptFields(field, "status", "value")
		if key == "injuries" {
			if items, ok := field["value"].([]any); ok {
				cleaned := make([]any, 0, len(items))
				for _, injury := range items {
					cleaned = append(cleaned, momentInjuryEvidenceValue(injury, events))
				}
				value["value"] = cleaned
			}
		}
		fields[key] = value
	}
	view["body_fields"] = fields
	for _, key := range []string{"worn_items", "used_items"} {
		if raw, exists := row[key]; exists {
			items := []map[string]any{}
			for _, item := range arrayValue(raw) {
				items = append(items, copyPromptFields(mapValue(item), "slot", "category", "description", "ownership", "availability", "source_verified", "activity", "started_at"))
			}
			view[key] = items
		}
	}
	return map[string]any{"kind": string(ContextReferenceAppearance), "value": view, "certainty": "当前权威外貌与穿戴快照；unknown和cleared保持原义，不证明其他动作已发生。"}, nil
}

func momentInjuryEvidenceValue(raw any, events map[string]map[string]any) any {
	value := copyPromptData(raw)
	injury := mapValue(value)
	id := stringValue(injury["id"])
	event := events[id]
	if event["kind"] != "body_injury" || (injury["status"] != "active" && injury["status"] != "recovered") {
		return value
	}
	effect := mapValue(event["effect"])
	if effect["operation"] != "injury" {
		return value
	}
	for _, key := range []string{"body_part", "description", "severity", "impact"} {
		if jsonString(injury[key]) != jsonString(effect[key]) {
			return value
		}
	}
	delete(injury, "id")
	recovery := events[stringValue(injury["recovery_event_id"])]
	if recovery["kind"] == "body_recovery" && mapValue(recovery["effect"])["operation"] == "recover" && mapValue(recovery["effect"])["injury_event_id"] == id {
		delete(injury, "recovery_event_id")
	}
	return value
}
