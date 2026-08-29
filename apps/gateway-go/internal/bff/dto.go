package bff

// Browser DTO mapping is intentionally explicit.  Core's snake_case records
// are not recursively converted: most Core responses are already part of the
// browser contract, while conversation/diagnostic/provider responses have
// documented, route-specific mappings.

func browserPage(page map[string]any) map[string]any {
	conversation := object(page["conversation"])
	conversationOut := map[string]any{
		"id":               conversation["id"],
		"createdByActorId": conversation["created_by_actor_id"],

		"revision":  conversation["revision"],
		"createdAt": conversation["created_at"],
		"updatedAt": conversation["updated_at"],
	}
	participants := make([]any, 0)
	for _, value := range array(page["participants"]) {
		item := object(value)
		mapped := map[string]any{
			"conversationId": item["conversation_id"],
			"actorId":        item["actor_id"],
			"role":           item["role"],
			"status":         item["status"],
			"joinedAt":       item["joined_at"],
		}
		if leftAt, exists := item["left_at"]; exists {
			mapped["leftAt"] = leftAt
		}
		participants = append(participants, mapped)
	}
	messages := make([]any, 0)
	for _, value := range array(page["messages"]) {
		messages = append(messages, browserMessage(object(value)))
	}
	next := page["next_before_sequence"]
	if next == nil {
		next = nil
	}
	return map[string]any{
		"conversation":       conversationOut,
		"participants":       participants,
		"messages":           messages,
		"nextBeforeSequence": next,
	}
}

func browserMessage(message map[string]any) map[string]any {
	attachments := message["attachment_refs"]
	if _, ok := attachments.([]any); !ok {
		attachments = []any{}
	}
	return map[string]any{
		"id":             message["id"],
		"conversationId": message["conversation_id"],
		"sequence":       message["sequence"],
		"authorActorId":  message["author_actor_id"],
		"kind":           message["kind"],
		"text":           message["text"],
		"attachmentRefs": attachments,
		"createdAt":      message["created_at"],
	}
}

func browserDiagnostic(event map[string]any) map[string]any {
	result := map[string]any{
		"id":            stringValue(first(event, "id")),
		"eventType":     stringValue(first(event, "event_type")),
		"severity":      stringValue(first(event, "severity")),
		"correlationId": stringValue(first(event, "correlation_id")),
		"payload":       objectValue(event["payload"]),
	}
	if value, exists := event["fluctlight_id"]; exists {
		result["fluctlightId"] = value
	}
	if value, exists := event["causation_id"]; exists {
		result["causationId"] = value
	}
	if value, exists := event["created_at"]; exists {
		result["createdAt"] = value
	}
	return result
}

func browserDiagnosticModelRun(row map[string]any) map[string]any {
	result := map[string]any{
		"id":            stringValue(first(row, "id")),
		"role":          stringValue(first(row, "role")),
		"modelId":       stringValue(first(row, "model_id")),
		"prompt":        objectValue(row["prompt"]),
		"status":        stringValue(first(row, "status")),
		"correlationId": stringValue(first(row, "correlation_id")),
		"createdAt":     stringValue(first(row, "created_at")),
	}
	if value, exists := row["endpoint_id"]; exists {
		result["endpointId"] = value
	}
	if value, exists := row["response"]; exists {
		result["response"] = value
	}
	if value, exists := row["error_code"]; exists {
		result["errorCode"] = value
	}
	return result
}

func browserProviderPreflight(value map[string]any) map[string]any {
	result := map[string]any{
		"role":      value["role"],
		"available": value["available"],
	}
	if version, exists := value["capability_version"]; exists {
		result["capabilityVersion"] = version
	}
	return result
}
