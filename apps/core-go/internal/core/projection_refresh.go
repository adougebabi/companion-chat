package core

import "context"

// bindProjectionRefresh preserves a surface's original authorization, task
// rules and query scope for physical requests after committed Tool mutations.
// It never changes the frozen projection used to authorize the Tool or settle
// the run; it only rebuilds the outbound model view.
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
		assembly, assembledProjection, err := a.assembleProjectionPromptForSurface(refreshCtx, surface, projection, role, rules, currentInput, tools, schemaName, schema)
		if err != nil {
			return modelContextRefreshContent{}, err
		}
		content, err := modelContextContent(assembly.Messages)
		content.Projection = assembledProjection
		content.BudgetTrace = assembly.Trace
		return content, err
	})
}
