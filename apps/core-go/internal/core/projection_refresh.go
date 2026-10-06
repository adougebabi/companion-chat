package core

import "context"

// bindProjectionRefresh preserves a surface's original authorization, task
// rules and query scope for physical requests after committed Tool mutations.
// The original frozen projection remains intact. Ordinary refresh keeps its
// settlement baseline; a verified stale-reply recovery may anchor a new model
// decision to the reread state and a server-owned trace boundary.
func (a *App) bindProjectionRefresh(
	ctx context.Context,
	request ContextProjectionRequest,
	surface ProviderContextSurface,
	role string,
	operationRules []string,
	currentInput string,
	definitions []CapabilityDefinition,
	schemaName string,
	schema map[string]any,
	decorate func(context.Context, *ContextProjection) error,
) context.Context {
	rules := append([]string(nil), operationRules...)
	tools := append([]CapabilityDefinition(nil), definitions...)
	return withRuntimeContextRefreshPlan(ctx, func(refreshCtx context.Context) (modelContextRefreshContent, error) {
		projection, err := a.BuildContextProjectionFor(refreshCtx, request)
		if err != nil {
			return modelContextRefreshContent{}, err
		}
		if adkContext, ok := adkCapabilityContext(refreshCtx); ok && adkContext.Trace != nil {
			workingProfileID := workingProfileForToolExecution(projection, adkContext.Trace)
			if workingProfileID != "" && workingProfileID != stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"]) {
				workingRequest := request
				workingRequest.WorkingProfileID = workingProfileID
				projection, err = a.BuildContextProjectionFor(refreshCtx, workingRequest)
				if err != nil {
					return modelContextRefreshContent{}, err
				}
			}
		}
		if decorate != nil {
			if err := decorate(refreshCtx, &projection); err != nil {
				return modelContextRefreshContent{}, err
			}
		}
		if adkContext, ok := adkCapabilityContext(refreshCtx); ok {
			if err := adkContext.Refs.registerIndex(projection.ReferenceIndex); err != nil {
				return modelContextRefreshContent{}, err
			}
		}
		modelProjection := projection
		modelProjection.RecentMessages = append([]map[string]any(nil), projection.RecentMessages...)
		if adkContext, ok := adkCapabilityContext(refreshCtx); ok {
			suppressCommittedReplyHistoryForContinuation(&modelProjection, adkContext.Trace)
		}
		assembly, assembledProjection, err := a.assembleProjectionPromptForSurface(refreshCtx, surface, modelProjection, role, rules, currentInput, tools, schemaName, schema)
		if err != nil {
			return modelContextRefreshContent{}, err
		}
		assembledProjection.RecentMessages = projection.RecentMessages
		content, err := modelContextContent(assembly.Messages)
		content.Projection = assembledProjection
		content.BudgetTrace = assembly.Trace
		return content, err
	})
}

// The current ADK transcript already contains its own reply ToolCall. Remove
// only replies whose committed message IDs appear in this exact run's trace;
// a fresh retry has a new trace and must still see the earlier publication.
func suppressCommittedReplyHistoryForContinuation(projection *ContextProjection, trace *ADKCapabilityTrace) {
	if projection == nil || trace == nil || len(projection.RecentMessages) == 0 {
		return
	}
	_, results := trace.Snapshot()
	committed := make(map[string]struct{})
	for _, result := range results {
		if result.CapabilityName != conversationReplyCapabilityName || result.Status != "completed" {
			continue
		}
		if targetID := stringValue(mapValue(result.Output)["target_ref"]); targetID != "" {
			committed[targetID] = struct{}{}
		}
	}
	if len(committed) == 0 {
		return
	}
	filtered := make([]map[string]any, 0, len(projection.RecentMessages))
	for _, message := range projection.RecentMessages {
		if stringValue(message["kind"]) == "assistant" {
			if _, sameRun := committed[stringValue(message["id"])]; sameRun {
				continue
			}
		}
		filtered = append(filtered, message)
	}
	projection.RecentMessages = filtered
}
