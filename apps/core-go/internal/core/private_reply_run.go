package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

type privateReplyRunKey struct{}
type directMainConversationKey struct{}

type privateReplyRun struct {
	Settings        ConversationReplySettings
	ConfigVersion   int64
	Assignment      providerAssignment
	AssignmentError error
	Scope           ToolExecutionRequest
	CurrentInput    string
	Expression      ReplyExpressionContract
	// Only execution progress is mutable. The settings/model contract above
	// is fixed before the first physical request and never re-read by a Tool.
	mu                          sync.Mutex
	DraftRef                    string
	ActionFacts                 []replyActionState
	ActionDraftText             string
	ExpressionSuggestions       []expressionSuggestionBinding
	ExpressionSuggestionsFrozen bool
	EffectiveAssignment         *providerAssignment
	EffectiveMode               string
	GenerationOwner             string
}

func replyRunFromContext(ctx context.Context) *privateReplyRun {
	run, _ := ctx.Value(privateReplyRunKey{}).(*privateReplyRun)
	return run
}

func (a *App) withPrivateReplyRun(ctx context.Context, scope ToolExecutionRequest, currentInput string) (context.Context, error) {
	ctx, err := a.withPrivateConversationReplyRun(ctx, scope, currentInput)
	if err != nil {
		return ctx, err
	}
	return a.withMomentWriterRun(ctx, scope)
}

func (a *App) withPrivateConversationReplyRun(ctx context.Context, scope ToolExecutionRequest, currentInput string) (context.Context, error) {
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
	run.Expression, err = replyExpressionContract(config.ExpressionVersion)
	if err != nil {
		return ctx, err
	}
	main, _ := ctx.Value(directMainConversationKey{}).(bool)
	priorDraft := ""
	if main && scope.Surface == CapabilitySurfaceConversation {
		var status string
		priorDraft, status, err = a.priorPrivateReplyDraft(ctx, scope)
		if err != nil {
			return ctx, err
		}
		switch status {
		case "generating", "prepared", "needs_action", "waiting_media", "ready":
			// A resumed run must bind subsequent media to the existing draft
			// before the model has had a chance to call conversation.reply.
			run.DraftRef = priorDraft
		}
	}
	ctx = context.WithValue(ctx, privateReplyRunKey{}, run)
	if config.Mode == "direct" {
		// Only the durable main turn can use the atomic decision contract.
		// Independent Tool calls and WakeUp keep their existing Writer route.
		if main && scope.Surface == CapabilitySurfaceConversation {
			if priorDraft == "" {
				registry := a.capabilityRegistryForContext(ctx)
				replacements := []Capability{directMainReplyCapability{app: a, publication: NewToolPublicationService(a)}}
				if catalog, ok := registry.Lookup(capabilityCatalogName); ok {
					replacements = append(replacements, directMainCatalogCapability{Capability: catalog})
				}
				view, err := capabilityRegistryView(registry, replacements...)
				if err != nil {
					return ctx, err
				}
				return withCapabilityRunRegistry(ctx, view), nil
			}
		}
		run.Settings.Mode = "fact_only"
	}
	if config.Mode == "legacy" {
		return ctx, nil
	}
	variant := "private_reply"
	if scope.Surface == CapabilitySurfaceWakeUp {
		variant = "proactive"
	}
	a.freezePersonaWriterAssignment(ctx, run, FormalAgentConversationReply, variant)
	view, err := a.privateReplyRegistry(run)
	if err != nil {
		return ctx, err
	}
	return withCapabilityRunRegistry(ctx, view), nil
}

// Old durable drafts keep their original Writer contract even when a later
// configuration requests direct main replies. The Core-derived logical key,
// not a model-supplied draft hint, identifies the same source and scope.
func (a *App) priorPrivateReplyDraft(ctx context.Context, scope ToolExecutionRequest) (string, string, error) {
	invocation := CapabilityInvocation{SourceFactID: scope.EvidenceID, Metadata: InvocationMetadata{
		FluctlightID: scope.FluctlightID, ConversationID: scope.ConversationID, AuthorizationActorID: scope.AuthorizationActorID,
	}}
	key := privateReplyLogicalKey(ctx, invocation)
	var ref, status string
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,status FROM public.conversation_reply_drafts
		WHERE logical_key=$1 AND fluctlight_id=$2 AND conversation_id=$3 AND authorization_actor_id=$4
		AND status IN ('generating','prepared','needs_action','waiting_media','ready','published','failed','cancelled','legacy_fallback_required')`,
		key, scope.FluctlightID, scope.ConversationID, scope.AuthorizationActorID).Scan(&ref, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return ref, status, err
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
		result["reply_prompt_version"] = firstString(run.Expression.Version, firstString(run.Settings.ExpressionVersion, factOnlyReplyPromptVersion))
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

func (a *App) freezePersonaWriterAssignment(ctx context.Context, run *privateReplyRun, agent FormalAgentID, variant string) {
	config := run.Settings
	var err error
	if config.ModelRef == "" || a.Provider == nil {
		run.AssignmentError = errors.New("reply_model_unconfigured")
	} else {
		run.Assignment, err = a.Provider.assignment(withAgentModelRoute(ctx, agent, variant), config.ModelRef)
		if err != nil {
			run.AssignmentError = errors.New("reply_model_unavailable")
		}
	}
	if run.AssignmentError == nil {
		// An explicit task binding owns its physical request timeout. The
		// private/WakeUp/Moment reply wrapper still has its own deadline; a
		// default binding keeps the historical global timeout behavior. Diary
		// and Studio use their respective workflow deadlines.
		if !strings.HasPrefix(run.Assignment.SourceBindingRole, "agent:") {
			run.Assignment.Timeout = time.Duration(config.TimeoutSeconds) * time.Second
		}
		temperature, topP := config.Temperature, config.TopP
		if run.Assignment.Temperature == nil {
			run.Assignment.Temperature = &temperature
		}
		if run.Assignment.TopP == nil {
			run.Assignment.TopP = &topP
		}
		if _, err := promptBudgetPolicyForAssignment(run.Assignment); err != nil {
			run.AssignmentError = errors.New("reply_model_budget_invalid")
		}
	}
}
