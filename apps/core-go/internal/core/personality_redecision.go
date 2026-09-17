package core

import (
	"context"
	"errors"
	"strings"
)

const (
	persistentSwitchAssessmentSchemaName = "persistent_switch_assessment"
	persistentSwitchReplySchemaName      = "persistent_switch_reply_response"
)

const persistentSwitchAssessmentInstruction = `这是人格切换前的认知判断阶段。只判断当前 switching.rules 是否满足，并返回 personality_decision；不要生成最终回复，不要调用任何工具，不要输出 hidden reasoning。decision 只能是 keep 或 switch。switch 时必须使用已声明的 from_profile_id、target_profile_id 和 trigger_id；没有满足条件时返回 keep。人格规则 ID 是协议字段，不是 context reference。`

const persistentSwitchRedecisionInstruction = `这是持久人格切换后的最终认知阶段。当前 Working Persona 已经是切换后的目标人格；请基于同一条用户消息重新判断本轮回复和必要能力。不要再次提出 personality_decision，不要调用 personality.switch，不要再次仲裁；只有确实需要时才调用已声明的能力，并通过 conversation.reply 返回最终可见文本。`

type persistentSwitchRedecisionInput struct {
	InboxID        string
	FluctlightID   string
	ConversationID string
	TurnID         string
	CurrentText    string
	Projection     ContextProjection
	Scope          turnPersonaScope
	Switch         personaSwitchNormalization
	Grant          persistentSwitchGrant
}

type persistentSwitchRedecisionResult struct {
	Projection               ContextProjection
	Scope                    turnPersonaScope
	Decision                 map[string]any
	Invocations              []CapabilityInvocation
	ResponsePlan             map[string]any
	ResponseMode             string
	Action                   string
	Composite                CompositeActionV1
	PersonalityPlan          *personalityDecisionPlan
	ContinuationBaseMessages []map[string]any
}

func persistentSwitchAssessmentSchema() map[string]any {
	decision := objectSchema(map[string]any{
		"decision":          enumStringSchema("keep", "switch"),
		"from_profile_id":   stringSchema(),
		"target_profile_id": stringSchema(),
		"trigger_id":        stringSchema(),
		"reason":            stringSchema(),
		"confidence":        unitNumberSchema(),
		"evidence_refs":     arraySchema(stringSchema()),
	}, []string{"decision"}, false)
	return objectSchema(map[string]any{"personality_decision": decision}, []string{"personality_decision"}, false)
}

func persistentSwitchRedecisionRequired(grant persistentSwitchGrant, normalization personaSwitchNormalization) bool {
	return grant.Allowed && normalization.hasDeclaredPersistentSwitch()
}

func (a *App) runPersistentSwitchRedecision(ctx context.Context, input persistentSwitchRedecisionInput) (persistentSwitchRedecisionResult, error) {
	result := persistentSwitchRedecisionResult{}
	assessmentSchema := persistentSwitchAssessmentSchema()
	assessmentAssembly, _, err := a.assembleProjectionPrompt(
		ctx,
		input.Projection,
		"cognitive_assessment",
		[]string{providerContextAuthorityRule, persistentSwitchAssessmentInstruction},
		input.CurrentText,
		nil,
		persistentSwitchAssessmentSchemaName,
		assessmentSchema,
	)
	if err != nil {
		return result, err
	}
	assessmentCtx := WithPromptDiagnostics(
		WithProviderCorrelation(WithProviderScenario(ctx, "cognitive_assessment"), "persona-switch-assessment:"+input.InboxID),
		assessmentAssembly.Diagnostics,
	)
	assessment, err := a.Provider.StructuredAssembledWithToolsSchema(
		assessmentCtx,
		"cognitive_assessment",
		assessmentAssembly.Messages,
		nil,
		persistentSwitchAssessmentSchemaName,
		assessmentSchema,
		false,
	)
	if err != nil {
		return result, err
	}
	if a.cognitionFactSuperseded(ctx, input.InboxID) {
		return result, errCognitionTurnSuperseded
	}
	plan, err := a.preparePersonalityDecision(ctx, input.FluctlightID, mapValue(assessment.Structured["personality_decision"]))
	if err != nil {
		return result, err
	}
	if a.cognitionFactSuperseded(ctx, input.InboxID) {
		return result, errCognitionTurnSuperseded
	}

	targetProjection := input.Projection
	targetProfileID := input.Scope.ActiveProfileID
	if plan != nil && strings.TrimSpace(plan.TargetProfile) != "" {
		targetProfileID = strings.TrimSpace(plan.TargetProfile)
	}
	if targetProfileID != strings.TrimSpace(input.Scope.ActiveProfileID) {
		targetProjection, err = a.resumeProjectionForReplyOwner(ctx, input.Projection, targetProfileID)
		if err != nil {
			return result, err
		}
	}
	targetScope := resolveTurnPersonaScope(targetProjection)
	targetScope.ReplyOwnerProfileID = targetProfileID
	if plan != nil && targetProfileID != strings.TrimSpace(input.Scope.ActiveProfileID) && plan.ResultingRevision > targetScope.PersonaRevision {
		targetScope.PersonaRevision = plan.ResultingRevision
		targetScope.ScopeRevision = personaScopeRevision(targetScope.ActiveProfileID, targetScope.PersonaRevision, targetScope.OverlayRevision)
	}

	finalGrant := resolvePersistentSwitchGrant(targetScope, "persistent_switch_redecision", input.Switch.Rules)
	definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceConversation)
	finalSchema := cognitiveTurnResponseSchemaForGrant(finalGrant)
	finalAssembly, assembledProjection, err := a.assembleProjectionPrompt(
		ctx,
		targetProjection,
		"cognitive_assessment",
		[]string{providerContextAuthorityRule, capabilityConversationPolicyInstruction, persistentSwitchRedecisionInstruction},
		input.CurrentText,
		definitions,
		persistentSwitchReplySchemaName,
		finalSchema,
	)
	if err != nil {
		return result, err
	}
	finalCtx := WithPromptDiagnostics(
		WithProviderCorrelation(WithProviderScenario(ctx, "cognitive_assessment"), "persona-switch-redecision:"+input.InboxID),
		finalAssembly.Diagnostics,
	)
	completion, err := a.Provider.StructuredAssembledWithToolsSchema(
		finalCtx,
		"cognitive_assessment",
		finalAssembly.Messages,
		definitions,
		persistentSwitchReplySchemaName,
		finalSchema,
		false,
	)
	if err != nil {
		return result, err
	}
	normalized, err := a.normalizeTurnDecision(ctx, turnDecisionNormalizationInput{
		InboxID: input.InboxID, FluctlightID: input.FluctlightID, ConversationID: input.ConversationID, TurnID: input.TurnID,
		Projection: assembledProjection, Grant: finalGrant, Decision: completion.Structured, Invocations: completion.ToolCalls,
		Definitions: definitions, StructuredFallback: completion.StructuredFallback,
		ContinuationBaseMessages: cloneMapSlice(finalAssembly.Messages),
	})
	if err != nil {
		return result, err
	}
	if plan != nil {
		normalized.Decision["personality_transition"] = plan
		normalized.PersonalityPlan = plan
	}
	if normalized.Decision == nil {
		return result, errors.New("persistent_switch_redecision_missing")
	}
	result.Projection = assembledProjection
	result.Scope = targetScope
	result.Decision = normalized.Decision
	result.Invocations = normalized.Invocations
	result.ResponsePlan = normalized.ResponsePlan
	result.ResponseMode = normalized.ResponseMode
	result.Action = normalized.Action
	result.Composite = normalized.Composite
	result.PersonalityPlan = normalized.PersonalityPlan
	result.ContinuationBaseMessages = normalized.ContinuationBaseMessages
	return result, nil
}
