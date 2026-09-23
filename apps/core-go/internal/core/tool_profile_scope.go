package core

import "context"

// A persona Tool publishes an authoritative working-persona resource. Its
// effect is run-local unless the Tool also commits a persistent switch. The
// Eino loop does not interpret persona names or run another decision protocol.
func workingProfileForToolExecution(projection ContextProjection, trace *ADKCapabilityTrace) string {
	profile := stringValue(projection.PersonalityRuntime["active_profile_id"])
	if profile == "" && len(projection.CorePersona) > 0 {
		profile = initialPersonalityProfileID(projection.CorePersona)
	}
	_, results := trace.Snapshot()
	for _, result := range results {
		if result.Status != "completed" {
			continue
		}
		if id := stringValue(mapValue(mapValue(result.Output)["working_persona"])["id"]); id != "" {
			profile = id
		}
	}
	return profile
}

func personaDetailExpectedRevisions(projection ContextProjection, trace *ADKCapabilityTrace, profileID string) (int, int) {
	sourceRevision := projection.CorePersonaRevision
	overlayRevision := intValue(mapValue(projection.EffectivePersona)["portrait_overlay_revision"])
	if trace == nil {
		return sourceRevision, overlayRevision
	}
	_, results := trace.Snapshot()
	for _, result := range results {
		if result.Status != "completed" || stringValue(mapValue(mapValue(result.Output)["working_persona"])["id"]) != profileID {
			continue
		}
		output := mapValue(result.Output)
		sourceRevision = intValue(output["working_persona_source_revision"])
		overlayRevision = intValue(output["working_persona_overlay_revision"])
	}
	return sourceRevision, overlayRevision
}

type toolProfileContextResolver struct {
	base    ContextResolver
	profile string
}

func (resolver toolProfileContextResolver) Resolve(ctx context.Context, request ContextRequest, slots []ContextSlot) (CapabilityContext, error) {
	resolved, err := resolver.base.Resolve(ctx, request, slots)
	if err != nil {
		return resolved, err
	}
	if resolved.Memory != nil {
		data := cloneMap(resolved.Memory.Data)
		data["active_profile_id"] = resolver.profile
		resolved.Memory = &MemoryScope{Data: data}
	}
	if resolved.Relation != nil {
		data := cloneMap(resolved.Relation.Data)
		data["active_profile_id"] = resolver.profile
		resolved.Relation = &RelationshipScope{Data: data}
	}
	return resolved, nil
}

func replyProfileFromOutcome(outcome agentCommittedOutcome, projection ContextProjection) string {
	if reply, ok := committedConversationReplyResult(outcome.Results); ok && reply.ActingProfileID != "" {
		return reply.ActingProfileID
	}
	trace := &ADKCapabilityTrace{Results: outcome.Results}
	return workingProfileForToolExecution(projection, trace)
}
