package core

import (
	"context"
	"errors"
)

const persistentSwitchAssessmentSchemaName = "persistent_switch_assessment"

const persistentSwitchAssessmentInstruction = `这是认知回复后的持久人格判断阶段。只判断当前 switching.rules 是否满足，并返回 personality_decision；不要改写已经生成的回复，不要调用任何工具，不要输出 hidden reasoning。decision 只能是 keep 或 switch。switch 时必须使用已声明的 from_profile_id、target_profile_id 和 trigger_id；没有满足条件时返回 keep。人格规则 ID 是协议字段，不是 context reference。`

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

func persistentSwitchAssessmentRequired(grant persistentSwitchGrant, normalization personaSwitchNormalization) bool {
	return grant.Allowed && normalization.hasDeclaredPersistentSwitch()
}

type persistentSwitchPostAssessmentInput struct {
	InboxID        string
	FluctlightID   string
	CurrentText    string
	CandidateReply string
	ResponseIntent string
	Projection     ContextProjection
}

// assessPersistentSwitchAfterCandidate is the small post-cognition decision
// requested by the runtime contract: the normal response candidate already
// exists, and a tool-free assessment decides whether the persistent active
// profile should change for the next turn. It never creates a reply or invokes
// a capability.
func (a *App) assessPersistentSwitchAfterCandidate(ctx context.Context, input persistentSwitchPostAssessmentInput) (*personalityDecisionPlan, map[string]any, error) {
	assessmentInput := map[string]any{
		"user_text":       input.CurrentText,
		"candidate_reply": input.CandidateReply,
		"response_intent": input.ResponseIntent,
	}
	assembly, _, err := a.assembleProjectionPromptForSurface(
		ctx,
		ProviderContextSurfacePersistentSwitch,
		input.Projection,
		"cognitive_assessment",
		[]string{providerContextAuthorityRule, persistentSwitchAssessmentInstruction},
		jsonString(assessmentInput),
		nil,
		persistentSwitchAssessmentSchemaName,
		persistentSwitchAssessmentSchema(),
	)
	if err != nil {
		return nil, nil, err
	}
	providerCtx := WithPromptDiagnostics(
		WithProviderCorrelation(WithProviderScenario(ctx, "cognitive_assessment"), "persona-switch-after:"+input.InboxID),
		assembly.Diagnostics,
	)
	completion, err := a.Provider.StructuredAssembledWithToolsSchema(
		providerCtx,
		"cognitive_assessment",
		assembly.Messages,
		nil,
		persistentSwitchAssessmentSchemaName,
		persistentSwitchAssessmentSchema(),
		false,
	)
	if err != nil {
		return nil, nil, err
	}
	decision := mapValue(completion.Structured["personality_decision"])
	if len(decision) == 0 {
		return nil, nil, errors.New("persistent_switch_assessment_missing")
	}
	plan, err := a.preparePersonalityDecision(ctx, input.FluctlightID, decision)
	if err != nil {
		return nil, decision, err
	}
	return plan, decision, nil
}
