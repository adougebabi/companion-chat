package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// directMainReplyCapability is installed only for an explicitly selected new
// direct-conversation run. The model supplies its own visible text and the
// same turn's decision in one native ToolCall; Core owns scope and settlement.
type directMainReplyCapability struct {
	app         *App
	publication *ToolPublicationService
}

const directMainConversationProtocol = `你是当前生效人格本人，通过聊天与对方自然交流。保持人物自己的态度、幽默、冷淡或热情，长短随情境变化；括号动作可按需使用，不强制每轮出现。聊天不代表双方同处，想法、愿望和创作不等于现实动作。状态、历史、计划、推断与真实工具回执各自解释，未知保持未知；尊重明确拒绝，不为完成目标编造事件。
需要能力时使用当前提供的原生工具，消费真实结果再决定；accepted 不等于完成，图片未交付不能声称已发送。普通交流无需调用无关工具。你的最终正文由你自己在原生 conversation.reply 的 text 中发送；同一 ToolCall 的 decision 按其 schema 提供本轮认知结果，保留有依据的 claims、influences 和 goal_event_candidates。普通闲聊不自动产生事实、目标或人格切换；没有当轮依据或不适用的可选字段用 null，不要编造空对象或虚拟条目。引用只用当前上下文提供的有效 ref，没有则留空数组。不要把推理、内部协议或未经核实的执行结果发送给用户；真实发布后本轮结束。`

func (c directMainReplyCapability) Definition() CapabilityDefinition {
	d := conversationReplyCapabilityDefinition()
	d.Version = "direct-main.v1"
	d.Description = "Send your own final private message and the associated turn decision. Use real Tool results for actions; a planned or accepted action is not a completed effect. The decision contains only the current turn's valid context references. A committed reply ends this turn."
	d.Surfaces = []CapabilitySurface{CapabilitySurfaceConversation}
	properties := map[string]any{
		"text":     map[string]any{"type": "string", "minLength": 1, "maxLength": 32000, "description": "Your final private message to the other person."},
		"decision": directMainDecisionSchema(),
	}
	d.InputSchema = objectSchema(properties, []string{"text", "decision"}, false)
	return d
}

// The same cognition contract also serves internal tasks. Describe the fields
// only for this direct-reply Tool without changing their validation or effects.
func directMainDecisionSchema() map[string]any {
	schema := cognitiveTurnResponseSchema()
	schema["description"] = "Cognitive data for this turn. Use the declared type for each field; use null for inapplicable optional data."
	descriptions := map[string]string{
		"action_type":                "For a private reply, this must be the string reply.",
		"response_intent":            "A short string describing your intention in this reply.",
		"influences":                 "An array of valid current context references influencing this decision. Each item has ref, role, confidence and note. Use an empty array when none apply.",
		"evidence_refs":              "An array of valid current reference strings supporting this decision. Use null when none are needed; do not put objects in this array.",
		"internal_intent":            "A short string for any relevant inner intention, or null if not needed.",
		"claims":                     "Claims or hypotheses relevant to this turn, including uncertain or unsupported self-claims when actually expressed. Each item uses kind, content, confidence and evidence_refs. Use null when there is no claim.",
		"goal_event_candidates":      "Current goal references supported by the turn for later evaluation. Each item uses goal_ref and reason. Use null when none apply; a candidate does not complete a goal.",
		"appraisal":                  "A structured appraisal using its own numeric appraisal fields and evidence_refs. Use null when no appraisal is proposed.",
		"attention":                  "A string describing relevant attention, or null when not proposed.",
		"thought":                    "A string describing a relevant thought, or null when not proposed.",
		"desire":                     "A string describing a relevant desire, or null when not proposed.",
		"agency":                     "A string describing relevant agency, or null when not proposed.",
		"response_plan":              "A response plan object using its own schema, or null when not needed. The actual private message belongs in text.",
		"personality_decision":       "A supported persistent personality decision proposal using its own schema, or null when no decision is proposed. A proposal is not an applied switch.",
		"output_preference_decision": "A supported output preference decision object using its own schema, or null when not applicable. This is not a media execution or delivery receipt.",
		"core_alignment":             "An object with aligned and note, or null when not proposed.",
		"state_expression":           "An object with body and mood, or null when not proposed.",
		"self_evaluation":            "An object using mode, reason_codes, confidence and note, or null when not proposed.",
	}
	for name, description := range descriptions {
		field := mapValue(mapValue(schema["properties"])[name])
		field["description"] = description
		// The wire converter narrows string-or-open-object cognition stages to
		// their string branch. Keep the annotation on that branch as well.
		for _, raw := range arrayValue(field["anyOf"]) {
			if branch := mapValue(raw); branch["type"] == "string" {
				branch["description"] = description
			}
		}
	}
	return schema
}

func (c directMainReplyCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotCurrentLife}
}
func (c directMainReplyCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}

func (c directMainReplyCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	bridge, ok := adkCapabilityContext(ctx)
	if !ok || bridge.Trace == nil || c.app == nil || invocation.Metadata.Source != "model_tool" {
		return invocation, newCapabilityError("direct_reply_scope_invalid", false, ErrUnauthorized)
	}
	invoker, ok := bridge.Invoker.(*appADKCapabilityInvoker)
	if !ok || invoker.request.Surface != CapabilitySurfaceConversation {
		return invocation, newCapabilityError("direct_reply_scope_invalid", false, ErrUnauthorized)
	}
	projection := invoker.request.Projection
	if fresh := bridge.Refresh.latestProjection(); fresh != nil {
		projection = *fresh
	}
	if projection.FluctlightID != invocation.Metadata.FluctlightID || projection.ConversationID != invocation.Metadata.ConversationID || projection.OwnerActorID != invocation.Metadata.AuthorizationActorID || projection.SourceFactID != invocation.SourceFactID {
		return invocation, newCapabilityError("direct_reply_scope_invalid", false, ErrUnauthorized)
	}
	args := decodeObject(invocation.Arguments)
	decision := cloneMap(mapValue(args["decision"]))
	if len(decision) == 0 {
		return invocation, newCapabilityError("direct_reply_decision_required", false, ErrInvalidArguments)
	}
	if err := validateCapabilitySchemaValue(decision, cognitiveTurnResponseSchema()); err != nil {
		return invocation, newCapabilityError("direct_reply_decision_invalid", false, err)
	}
	if err := validateGoalEventCandidates(decision, projection.ReferenceIndex); err != nil {
		return invocation, newCapabilityError("direct_reply_decision_invalid", false, err)
	}
	if _, err := freezeDecisionInfluences(decision, projection, false); err != nil {
		return invocation, newCapabilityError("direct_reply_decision_invalid", false, err)
	}
	if len(mapValue(decision["appraisal"])) == 0 {
		decision["cognitive_state_transition"] = "not_proposed"
	}
	plan, err := normalizeResponsePlan(decision, invocation.SourceFactID, projection)
	if err != nil {
		return invocation, newCapabilityError("direct_reply_decision_invalid", false, err)
	}
	return withCapabilityPreparedData(invocation, "direct_main_settlement", map[string]any{"projection": projection, "decision": decision, "response_plan": plan})
}

func (c directMainReplyCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	return (conversationReplyCapability{publication: c.publication}).ExecuteDirectTx(ctx, tx, invocation, resolved, target)
}

// The message and Tool receipt are already written in this transaction, but
// neither is committed until the complete turn settlement also succeeds.
func (c directMainReplyCapability) SettleToolTx(ctx context.Context, tx pgx.Tx, request ToolExecutionRequest, prepared CapabilityInvocation, result CapabilityResult) error {
	data, found, err := capabilityPreparedData(prepared, "direct_main_settlement")
	if err != nil || !found {
		return errors.New("direct_reply_settlement_missing")
	}
	value := mapValue(data)
	projection, ok := contextProjectionFromValue(value["projection"])
	if !ok || projection.SourceFactID != request.EvidenceID || projection.FluctlightID != request.FluctlightID || projection.ConversationID != request.ConversationID {
		return errors.New("direct_reply_settlement_scope_invalid")
	}
	decision, plan := mapValue(value["decision"]), mapValue(value["response_plan"])
	if len(decision) == 0 || len(plan) == 0 {
		return errors.New("direct_reply_settlement_invalid")
	}
	bridge, ok := adkCapabilityContext(ctx)
	if !ok || bridge.Trace == nil {
		return errors.New("direct_reply_trace_missing")
	}
	invocations, results := bridge.Trace.Snapshot()
	results = append(results, result)
	byCall := make(map[string]CapabilityResult, len(results))
	for _, item := range results {
		id := strings.TrimSpace(item.CallID)
		if id == "" || byCall[id].CallID != "" {
			return errors.New("direct_reply_trace_invalid")
		}
		byCall[id] = item
	}
	for _, item := range invocations {
		if actual, exists := byCall[item.CallID]; !exists || actual.CapabilityName != item.CapabilityName {
			return errors.New("direct_reply_trace_incomplete")
		}
	}
	messageID := stringValue(mapValue(result.Output)["target_ref"])
	var turnID, sourceFactID string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(turn_id,''),COALESCE(source_fact_id,'') FROM public.conversation_messages WHERE id=$1 AND conversation_id=$2 AND author_actor_id=$3 AND kind='assistant'`, messageID, request.ConversationID, request.FluctlightID).Scan(&turnID, &sourceFactID); err != nil {
		return err
	}
	if turnID == "" || sourceFactID != request.EvidenceID {
		return errors.New("direct_reply_source_invalid")
	}
	// Ensure the prepared decision remains JSON data, never a model-provided
	// authority snapshot. The snapshot above came only from the Core invoker.
	if _, err := json.Marshal(decision); err != nil {
		return err
	}
	outcome := agentCommittedOutcome{Invocations: invocations, Results: results}
	return c.app.settleAgentConversationTurnTx(ctx, tx, sourceFactID, turnID, request.FluctlightID, firstString(request.SubjectActorID, request.AuthorizationActorID), projection, decision, plan, map[string]any{"id": messageID}, mediaIntentFromCommittedResults(results), outcome, true)
}
