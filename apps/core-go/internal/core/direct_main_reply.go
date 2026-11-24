package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// directMainReplyCapability is installed for a new direct-conversation main
// run. The model supplies its own visible text and the
// same turn's decision in one native ToolCall; Core owns scope and settlement.
type directMainReplyCapability struct {
	app         *App
	publication *ToolPublicationService
	replyOnly   bool
}

// The direct private conversation presents its directory as the next step
// before publication when the model chooses an action requiring another tool.
type directMainCatalogCapability struct{ Capability }

func (c directMainCatalogCapability) Definition() CapabilityDefinition {
	definition := c.Capability.Definition()
	definition.Description = "Discover and load an action before replying. For an unresolved photo or selfie that you choose to make now, call this tool first with names [\"media.image.generate\"]. A directory call does not start the action. For unknown tools, names [] lists authorized purposes; select a name to load its schema."
	return definition
}

const directMainConversationProtocol = `本回合的第一步是选择下一步原生工具。判断对话中用户在期待什么，以及你是否愿意现在采取行动。若用户正在追问尚未发来的照片，而且你会继续拍摄，那么下一步是 capability_catalog，names 填 media.image.generate；接下来获得图片工具并继续。决定不行动时直接 conversation_reply，说明真实选择。
以生效人格自然表达，长短随情境，括号动作可选。不要假设双方同处，也不要把愿望或过去的承诺当成现状。真正执行后消费回执再回复，受理不表示已送达。终稿用 conversation.reply 的 text 提交，decision 按原生 schema 只提供本轮有依据的信息和有效引用。没有依据的可选字段省略。`

func (c directMainReplyCapability) Definition() CapabilityDefinition {
	d := conversationReplyCapabilityDefinition()
	d.Version = "direct-main.v1"
	d.Description = "Publish the final private reply after any action you choose is done or has a real receipt. If the user is following up on a photo and you choose to make it now, call capability_catalog first to load the image action. For an accepted private image task, reply that it was submitted or is pending. Use only valid current reference IDs; for a simple receipt acknowledgment use decision with action_type=reply, a short response_intent and influences=[]. Omit optional decision fields that are unnecessary."
	d.Surfaces = []CapabilitySurface{CapabilitySurfaceConversation}
	d.ModelPreserveOptionalObjectPaths = []string{"decision"}
	properties := map[string]any{
		"text":     map[string]any{"type": "string", "minLength": 1, "maxLength": 32000, "description": "Your final private message to the other person."},
		"decision": directMainDecisionSchema(),
	}
	if c.replyOnly {
		d.Description = "Publish the missing private reply for the original user message. Confirm only real results. Supply text and decision with action_type=reply, a short response_intent and influences=[]."
		decision := mapValue(properties["decision"])
		original := mapValue(decision["properties"])
		properties["decision"] = objectSchema(map[string]any{"action_type": original["action_type"], "response_intent": original["response_intent"], "influences": original["influences"]}, []string{"action_type", "response_intent", "influences"}, false)
	}
	d.InputSchema = objectSchema(properties, []string{"text", "decision"}, false)
	return d
}

// The same cognition contract also serves internal tasks. Describe the fields
// only for this direct-reply Tool without changing their validation or effects.
func directMainDecisionSchema() map[string]any {
	schema := cognitiveTurnResponseSchema()
	schema["description"] = "Cognitive data for this turn. Use the declared type for each field; omit optional data or use null when inapplicable."
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
	run := replyRunFromContext(ctx)
	if run == nil || run.Settings.Mode != "direct" || run.Scope.FluctlightID != invocation.Metadata.FluctlightID || run.Scope.ConversationID != invocation.Metadata.ConversationID || run.Scope.AuthorizationActorID != invocation.Metadata.AuthorizationActorID || run.Scope.EvidenceID != invocation.SourceFactID || firstString(run.Scope.SubjectActorID, run.Scope.AuthorizationActorID) != firstString(invocation.Metadata.SubjectActorID, invocation.Metadata.AuthorizationActorID) {
		return invocation, newCapabilityError("direct_reply_scope_invalid", false, ErrUnauthorized)
	}
	run.mu.Lock()
	suggestionsFrozen := run.ExpressionSuggestionsFrozen
	suggestions := append([]expressionSuggestionBinding(nil), run.ExpressionSuggestions...)
	run.mu.Unlock()
	if !suggestionsFrozen {
		return invocation, newCapabilityError("direct_expression_context_missing", false, ErrConflict)
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
	var mediaRefs, mediaIntents []string
	text := stringValue(args["text"])
	if directMediaDeliveryCheckNeeded(text, run.CurrentInput, projection.RecentMessages) {
		subject := firstString(invocation.Metadata.SubjectActorID, invocation.Metadata.AuthorizationActorID)
		deliveries, err := directMediaDeliveries(ctx, c.app.DB.Pool(), invocation.Metadata.AuthorizationActorID, subject, invocation.Metadata.FluctlightID, invocation.Metadata.ConversationID, false)
		if err != nil {
			return invocation, newCapabilityError("direct_media_delivery_check_failed", true, err)
		}
		currentMedia := directCurrentMediaIntents(bridge.Trace, invocation)
		if retry := directReplyRetryFromContext(ctx); retry.intentID != "" {
			currentMedia[retry.intentID] = true
		}
		mediaRefs, err = c.app.checkDirectMediaDeliveryClaims(ctx, projection, run.CurrentInput, text, deliveries, currentMedia)
		if err != nil {
			return invocation, err
		}
		for _, ref := range mediaRefs {
			delivery, err := directMediaDeliveryByRef(deliveries, ref)
			if err != nil {
				return invocation, newCapabilityError("direct_media_delivery_unverified", false, err)
			}
			mediaIntents = append(mediaIntents, delivery.intentID)
		}
	}
	return withCapabilityPreparedData(invocation, "direct_main_settlement", map[string]any{"projection": projection, "decision": decision, "response_plan": plan, "expression_suggestions": suggestions, "media_delivery_refs": mediaRefs, "media_delivery_intents": mediaIntents})
}

func (c directMainReplyCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	data, found, err := capabilityPreparedData(invocation, "direct_main_settlement")
	if err != nil || !found {
		return CapabilityResult{}, errors.New("direct_reply_settlement_missing")
	}
	var suggestions []expressionSuggestionBinding
	if raw, present := mapValue(data)["expression_suggestions"]; present {
		if err := json.Unmarshal(jsonBytes(raw), &suggestions); err != nil {
			return CapabilityResult{}, errors.New("direct_expression_context_invalid")
		}
	}
	if err := validateExpressionSuggestionsTx(ctx, tx, target.AuthorizationActorID, target.FluctlightID,
		firstString(invocation.Metadata.SubjectActorID, target.AuthorizationActorID), target.ConversationID, suggestions); err != nil {
		return CapabilityResult{}, err
	}
	if err := requireDirectMediaDeliveriesTx(ctx, tx, target.AuthorizationActorID,
		firstString(invocation.Metadata.SubjectActorID, target.AuthorizationActorID), target.FluctlightID, target.ConversationID,
		arrayValue(mapValue(data)["media_delivery_refs"]), arrayValue(mapValue(data)["media_delivery_intents"])); err != nil {
		return CapabilityResult{}, directMediaDeliveryTxError(err)
	}
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
	mediaID := firstString(mediaIntentFromCommittedResults(results), directReplyRetryFromContext(ctx).intentID)
	return c.app.settleAgentConversationTurnTx(ctx, tx, sourceFactID, turnID, request.FluctlightID, firstString(request.SubjectActorID, request.AuthorizationActorID), projection, decision, plan, map[string]any{"id": messageID}, mediaID, outcome, true)
}
