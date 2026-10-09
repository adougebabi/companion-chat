package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

type kevPersonaKey struct{}
type kevPersonaExpectedRevisionKey struct{}
type kevPersonaDomain interface {
	preparePersonalityDecision(context.Context, string, map[string]any) (*personalityDecisionPlan, error)
	ExecuteTool(context.Context, ToolExecutionRequest) (ToolExecutionReceipt, error)
}
type kevPersonaAdmission struct {
	mu           sync.Mutex
	app          *App
	domain       kevPersonaDomain
	request      ADKCapabilityRequest
	evaluated    bool
	denied       map[string]bool
	winnerRecord *decision.Result
	winner       string
	switched     bool
}

func kevPersona(ctx context.Context) *kevPersonaAdmission {
	g, _ := ctx.Value(kevPersonaKey{}).(*kevPersonaAdmission)
	return g
}

// inspect runs before the complete native response is released to Eino's tool
// node. A rejected proposal is never installed as an assistant ToolCall batch.
func (g *kevPersonaAdmission) inspect(ctx context.Context, message *schema.Message) (*schema.Message, bool, error) {
	if g == nil || message == nil || !g.app.kevService().Enabled(ctx, "persona.switch") {
		return message, false, ctx.Err()
	}
	g.mu.Lock()
	if g.evaluated {
		g.mu.Unlock()
		return message, false, nil
	}
	g.evaluated = true
	g.mu.Unlock()
	bridge, ok := adkCapabilityContext(ctx)
	if !ok || bridge.Refresh == nil {
		return message, false, nil
	}
	projection := g.request.Projection
	current := workingProfileForToolExecution(projection, nil)
	normalized := normalizePersonaSwitchRules(corePersonaData(projection.CorePersona), projection.PersonalityRuntime, current)
	rules := []personaSwitchRule{}
	candidates := []decision.Candidate{}
	conditions := map[string]string{}
	for _, rule := range normalized.Rules {
		if !rule.Enabled || rule.Kind != switchRulePersistentSemantic || !strings.HasPrefix(rule.Source, "switching") || rule.TargetProfileID == "" || rule.TargetProfileID == current || (rule.SourceProfileID != "" && rule.SourceProfileID != current) {
			continue
		}
		rules = append(rules, rule)
		conditions[rule.RuleID] = rule.Condition
		candidates = append(candidates, decision.Candidate{ID: rule.RuleID, Question: decision.Choice("Is this declared persistent personality switch condition satisfied: " + rule.Condition + "?")})
	}
	if len(rules) == 0 {
		return message, false, nil
	}
	input := map[string]any{"active_profile": current, "conditions": conditions, "proposal": message.Content, "actions": []map[string]any{}}
	actions := []map[string]any{}
	for _, call := range message.ToolCalls {
		actions = append(actions, map[string]any{"capability": call.Function.Name, "arguments": json.RawMessage(call.Function.Arguments)})
	}
	input["actions"] = actions
	// Include the actual current user event, not full private conversation.
	input["event_id"] = g.request.SourceFactID
	input["current_input"] = projection.CurrentUserText
	input["current_state"] = projection.CurrentState
	results, err := g.app.kevService().Decide(ctx, "persona.switch", g.app.kevScope(ctx, projection, "persona.switch"), jsonString(input), candidates)
	if err != nil {
		return nil, false, err
	}
	yes := []personaSwitchRule{}
	byRule := map[string]*decision.Result{}
	for i := range results {
		r := &results[i]
		out, err := r.Admit(ctx)
		if err != nil {
			return nil, false, err
		}
		byRule[r.Record.CandidateID] = r
		if out == decision.Yes {
			yes = append(yes, rules[i])
		}
		if out == decision.No {
			g.mu.Lock()
			g.denied[r.Record.CandidateID] = true
			g.mu.Unlock()
			_ = r.Finish(ctx, "applied", "profile_kept", "")
		}
		if out == decision.Original {
			_ = r.Finish(ctx, "original_used", "", "")
		}
	}
	if len(yes) == 0 {
		return message, false, nil
	}
	sort.SliceStable(yes, func(i, j int) bool { return yes[i].Priority > yes[j].Priority })
	if len(yes) > 1 && yes[0].Priority == yes[1].Priority {
		for _, rule := range yes {
			_ = byRule[rule.RuleID].Finish(ctx, "original_used", "ambiguous_rules", "")
		}
		return message, false, nil
	}
	winner := yes[0]
	r := byRule[winner.RuleID]
	// Recheck the original persona revision before the independent domain Tool.
	domain := g.domain
	if domain == nil {
		domain = g.app
	}
	plan, err := domain.preparePersonalityDecision(ctx, projection.FluctlightID, map[string]any{"decision": "switch", "from_profile_id": current, "target_profile_id": winner.TargetProfileID, "trigger_id": winner.RuleID})
	if cancelled := ctx.Err(); cancelled != nil {
		_ = r.Finish(ctx, "cancelled", "request_cancelled", "")
		return nil, false, cancelled
	}
	if err != nil || plan == nil || plan.ExpectedRevision != intValue(projection.PersonalityRuntime["revision"]) {
		_ = r.Finish(ctx, "stale", "persona_revision_changed", "")
		bridge.Refresh.markDirty()
		return nil, true, nil
	}
	if out, err := r.Admit(ctx); err != nil {
		return nil, false, err
	} else if out == decision.Original {
		return message, false, nil
	}
	for _, lower := range yes[1:] {
		_ = byRule[lower.RuleID].Finish(ctx, "not_applied", "lower_priority", "")
	}
	g.mu.Lock()
	g.winner = winner.RuleID
	g.winnerRecord = r
	g.mu.Unlock()
	definition, _ := formalAgentDefinitionFromContext(ctx)
	commitCtx := context.WithValue(ctx, kevPersonaExpectedRevisionKey{}, plan.ExpectedRevision)
	receipt, err := domain.ExecuteTool(commitCtx, ToolExecutionRequest{CapabilityName: personaSwitchCapabilityName, OperationID: "kev_switch_" + r.Record.ID, AuthorizationActorID: g.request.AuthorizationActorID, SubjectActorID: g.request.SubjectActorID, FluctlightID: g.request.FluctlightID, ConversationID: g.request.ConversationID, EvidenceID: g.request.SourceFactID, Surface: g.request.Surface, AuthorizationPolicy: g.request.AuthorizationPolicy, AgentID: definition.ID, RunID: g.request.OperationID, Arguments: jsonBytes(map[string]any{"decision": "switch", "source_profile_id": current, "target_profile_id": winner.TargetProfileID, "trigger_id": winner.RuleID, "reason": "Declared condition selected by Kev"})})
	if err != nil {
		return nil, false, err
	}
	if receipt.Result.Status != "completed" {
		_ = r.Finish(ctx, "not_applied", receipt.Result.ErrorCode, receipt.ExecutionCallID)
		return message, false, nil
	}
	g.mu.Lock()
	g.switched = true
	g.mu.Unlock()
	_ = r.Finish(ctx, "applied", "", receipt.ExecutionCallID)
	adkContext, ok := adkCapabilityContext(ctx)
	if !ok || adkContext.Refresh == nil {
		return nil, false, errors.New("kev_persona_refresh_unavailable")
	}
	adkContext.Refresh.markDirty()
	return nil, true, nil
}

func kevRejectNativeTool(ctx context.Context, name string, arguments json.RawMessage) string {
	if selection := kevSelection(ctx); selection != nil && !selection.permits(ctx, name) {
		return "capability_not_disclosed"
	}
	g := kevPersona(ctx)
	if g == nil || name != personaSwitchCapabilityName || !g.app.kevService().Enabled(ctx, "persona.switch") {
		return ""
	}
	var args map[string]any
	_ = json.Unmarshal(arguments, &args)
	rule := stringValue(args["trigger_id"])
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.switched {
		return "kev_persona_already_switched"
	}
	if g.denied[rule] || g.denied["switch:"+strings.TrimPrefix(rule, "switch:")] {
		return "kev_persona_condition_false"
	}
	if g.winner != "" && g.winner != "switch:"+strings.TrimPrefix(rule, "switch:") {
		return "kev_persona_rule_not_selected"
	}
	return ""
}

func (g *kevPersonaAdmission) needsCheck(ctx context.Context) bool {
	if g == nil || !g.app.kevService().Enabled(ctx, "persona.switch") {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.evaluated
}
func consumeKevCandidateStream(ctx context.Context, stream *schema.StreamReader[*schema.Message]) (*schema.Message, error) {
	defer stream.Close()
	messages := []*schema.Message{}
	bytes := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		message, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if message == nil {
			continue
		}
		bytes += len(message.Content) + len(message.ReasoningContent)
		for _, call := range message.ToolCalls {
			bytes += len(call.Function.Arguments) + len(call.Function.Name)
		}
		if bytes > 2*1024*1024 {
			return nil, errors.New("kev_persona_proposal_too_large")
		}
		messages = append(messages, message)
	}
	if len(messages) == 0 {
		return nil, errors.New("kev_persona_proposal_empty")
	}
	return schema.ConcatMessages(messages)
}

func (g *kevPersonaAdmission) adoptedRevision(ctx context.Context) (int, bool) {
	g.mu.Lock()
	revision := intValue(g.request.Projection.PersonalityRuntime["revision"])
	adopted := g.winner != ""
	g.mu.Unlock()
	return revision, adopted && g.app.kevService().Enabled(ctx, "persona.switch")
}

func (g *kevPersonaAdmission) nativeResult(ctx context.Context, result CapabilityResult) {
	g.mu.Lock()
	record := g.winnerRecord
	g.mu.Unlock()
	if record == nil {
		return
	}
	status := "not_applied"
	if result.Status == "completed" && stringValue(mapValue(result.Output)["disposition"]) == "applied" {
		status = "applied"
	}
	_ = record.Finish(ctx, status, result.ErrorCode, result.CallID)
}

func kevPersonaInstalled(definitions []CapabilityDefinition) bool {
	for _, definition := range definitions {
		if definition.Name == personaSwitchCapabilityName {
			return true
		}
	}
	return false
}
