package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

type privateReplyRunKey struct{}

type privateReplyRun struct {
	Settings        ConversationReplySettings
	ConfigVersion   int64
	Assignment      providerAssignment
	AssignmentError error
	Scope           ToolExecutionRequest
	CurrentInput    string
	// Only execution progress is mutable. The settings/model contract above
	// is fixed before the first physical request and never re-read by a Tool.
	mu                  sync.Mutex
	DraftRef            string
	ActionFacts         []replyActionState
	ActionDraftText     string
	EffectiveAssignment *providerAssignment
	EffectiveMode       string
}

func replyRunFromContext(ctx context.Context) *privateReplyRun {
	run, _ := ctx.Value(privateReplyRunKey{}).(*privateReplyRun)
	return run
}

func (a *App) withPrivateReplyRun(ctx context.Context, scope ToolExecutionRequest, currentInput string) (context.Context, error) {
	if prior := replyRunFromContext(ctx); prior != nil {
		if prior.Scope.FluctlightID != scope.FluctlightID || prior.Scope.ConversationID != scope.ConversationID || prior.Scope.AuthorizationActorID != scope.AuthorizationActorID {
			return ctx, ErrUnauthorized
		}
		return ctx, nil
	}
	if scope.ConversationID == "" || a == nil || a.DB == nil || a.DB.Pool() == nil {
		return ctx, nil
	}
	var private bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_direct_conversations WHERE owner_actor_id=$1 AND fluctlight_actor_id=$2 AND conversation_id=$3)`, scope.AuthorizationActorID, scope.FluctlightID, scope.ConversationID).Scan(&private); err != nil {
		return ctx, err
	}
	if !private {
		return ctx, nil
	}
	config, version, err := a.conversationReplySettings(ctx)
	if err != nil {
		return ctx, err
	}
	run := &privateReplyRun{Settings: config, ConfigVersion: version, Scope: scope, CurrentInput: currentInput}
	ctx = context.WithValue(ctx, privateReplyRunKey{}, run)
	if config.Mode == "legacy" {
		return ctx, nil
	}
	if config.ModelRef == "" || a.Provider == nil {
		run.AssignmentError = errors.New("reply_model_unconfigured")
	} else {
		run.Assignment, err = a.Provider.assignment(ctx, config.ModelRef)
		if err != nil {
			run.AssignmentError = errors.New("reply_model_unavailable")
		}
	}
	if run.AssignmentError == nil {
		run.Assignment.TokenBudget = min(run.Assignment.TokenBudget, config.MaxTokens)
		run.Assignment.MaxInputTokens = min(run.Assignment.MaxInputTokens, config.MaxInputTokens)
		run.Assignment.Timeout = time.Duration(config.TimeoutSeconds) * time.Second
		temperature, topP := config.Temperature, config.TopP
		run.Assignment.Temperature, run.Assignment.TopP = &temperature, &topP
		if _, err := promptBudgetPolicyForAssignment(run.Assignment); err != nil {
			run.AssignmentError = errors.New("reply_model_budget_invalid")
		}
	}
	view, err := a.privateReplyRegistry(run)
	if err != nil {
		return ctx, err
	}
	return withCapabilityRunRegistry(ctx, view), nil
}

func privateReplyLogicalKey(ctx context.Context, inv CapabilityInvocation) string {
	source := inv.SourceFactID
	if source == "" {
		if run := replyRunFromContext(ctx); run != nil {
			source = firstString(run.Scope.RunID, run.Scope.OperationID)
		}
	}
	source = firstString(source, inv.Metadata.OperationID)
	return "reply_" + stableDigest(strings.Join([]string{inv.Metadata.FluctlightID, inv.Metadata.ConversationID, inv.Metadata.AuthorizationActorID, source}, "\x1f"))
}

func replyRunDiagnostics(ctx context.Context, original map[string]any) map[string]any {
	result := cloneMap(original)
	if result == nil {
		result = map[string]any{}
	}
	if run := replyRunFromContext(ctx); run != nil {
		result["reply_mode"] = run.Settings.Mode
		result["reply_config_version"] = run.ConfigVersion
		result["reply_prompt_version"] = factOnlyReplyPromptVersion
		result["reply_model_ref"] = run.Settings.ModelRef
	}
	return result
}

func (a *App) privateReplyRegistry(run *privateReplyRun) (*CapabilityRegistry, error) {
	replacements := []Capability{factOnlyReplyCapability{service: newFactOnlyReplyService(a), run: run, publication: NewToolPublicationService(a)}}
	if entry, ok := a.capabilityRegistry().LookupCapability("media.image.generate"); ok {
		if image, ok := entry.(imageGenerateCapability); ok {
			replacements = append(replacements, replyImageCapability{imageGenerateCapability: image, app: a, run: run})
		}
	}
	return capabilityRegistryView(a.capabilityRegistry(), replacements...)
}
