package core

import (
	"context"
	"errors"
	"strings"
)

// Agent model bindings extend the existing model_roles table. The same
// canonical key is used for configuration CAS, discovery and resolution.
// Variants are task-owned identities, not free-form model or endpoint names.
var agentModelVariants = map[FormalAgentID][]string{
	FormalAgentConversationCognition: {"conversation_turn_response"},
	FormalAgentConversationReply:     {"private_reply", "proactive", "moment"},
	FormalAgentReplyActionCheck:      {"reply_action_consistency", directMediaDeliveryCheckSchema, directImageDiscoverySchema},
	FormalAgentExpressionLearning:    {expressionLearningSchema},
	FormalAgentWakeUp:                {"wake_up_response"},
	FormalAgentTakeoverJudge:         {"takeover_judge_response"},
	FormalAgentTakeoverReply:         {"takeover_reply_response"},
	FormalAgentWardrobePreparation:   {"wardrobe_preparation_v1"},
	FormalAgentInitialization:        {"initialization_response"},
	FormalAgentPersonaCompilation:    {"persona_compilation_response"},
	FormalAgentMediaPrompt:           {"media_prompt_text", "media_subject_parse", "media_prompt_normalized"},
	FormalAgentVisualIdentityPatch:   {"visual_identity_patch_response"},
	FormalAgentConversationSummary:   {"conversation_summary_v1", "conversation_segment_v1", "conversation_daily_memory_v1"},
	FormalAgentScheduleGeneration:    {"schedule_response"},
	FormalAgentNativeCognition:       {"native_cognition_response", "owner_life_request_decision", "npc_opportunity_decision"},
	FormalAgentStudioAssistant:       {"studio_assistant_v1"},
	FormalAgentStudioTextPreview:     {"studio_preview_text"},
	FormalAgentNPCResponse:           {"npc_response_v1", "npc_moment_response_v1"},
	FormalAgentDailyReview:           {"daily_review_response"},
	FormalAgentDiaryMaterial:         {"diary_material_v1"},
	FormalAgentDiaryExpression:       {"diary_expression_text"},
	FormalAgentPersistentSwitch:      {"persistent_switch_assessment"},
	FormalAgentReflection:            {"reflection_proposal_v2"},
	FormalAgentScheduleReplan:        {"schedule_replan_plan"},
	FormalAgentGoalPlanner:           {"goal_planner_v1"},
	FormalAgentGoalEvaluation:        {"goal_evaluation_v1"},
	FormalAgentVirtualActivityResult: {"virtual_activity_result"},
	FormalAgentWorldEventGeneration:  {"world_event_generation_v1"},
}

type agentModelRouteKey struct{}
type agentModelRoute struct {
	agent   FormalAgentID
	variant string
}

func withAgentModelRoute(ctx context.Context, agent FormalAgentID, variant string) context.Context {
	route := agentModelRoute{agent: agent, variant: strings.TrimSpace(variant)}
	// A caller may explicitly freeze a model before entering its first Agent
	// (for example a restored Writer or a visual inspection stage). Adopt that
	// binding for this task only; tools may then start independently routed
	// Agents without inheriting their parent's assignment.
	if frozen, ok := ctx.Value(frozenProviderAssignmentKey{}).(frozenProviderAssignment); ok && !frozen.scoped {
		if definition, found := FormalAgentDefinitionByID(agent); found && definition.Role == frozen.role {
			frozen.route, frozen.scoped = route, true
			ctx = context.WithValue(ctx, frozenProviderAssignmentKey{}, frozen)
		}
	}
	return context.WithValue(ctx, agentModelRouteKey{}, route)
}

func withoutAgentModelRoute(ctx context.Context) context.Context {
	return context.WithValue(ctx, agentModelRouteKey{}, agentModelRoute{})
}

func agentModelBindingKey(agent FormalAgentID, variant string) string {
	key := "agent:" + string(agent)
	if variant != "" {
		key += ":" + variant
	}
	return key
}

func validAgentModelBindingRole(role string) bool {
	if len(role) > 64 {
		return false
	}
	parts := strings.Split(role, ":")
	if len(parts) < 2 || len(parts) > 3 || parts[0] != "agent" {
		return false
	}
	agent := FormalAgentID(parts[1])
	variants, supported := agentModelVariants[agent]
	if !supported {
		// Image-input Agents retain their existing independent visual binding
		// until an alternative passes real-image acceptance.
		return false
	}
	if len(parts) == 2 {
		return true
	}
	return containsString(variants, parts[2])
}

// Missing means preserve the old binding; explicit null clears an override.
// Sampling is a model parameter and never a prompt or output token budget.
func optionalModelSampling(payload map[string]any, key string, exclusiveMin bool, maximum float64) (*float64, bool, error) {
	raw, present := payload[key]
	if !present || raw == nil {
		return nil, present, nil
	}
	if _, text := raw.(string); text {
		return nil, true, errors.New("provider_sampling_invalid")
	}
	value, ok := numberFloat(raw)
	if !ok || value < 0 || (exclusiveMin && value == 0) || value > maximum {
		return nil, true, errors.New("provider_sampling_invalid")
	}
	return &value, true, nil
}

func agentModelBindingCandidates(ctx context.Context, bindingRole string) []string {
	result := []string{}
	route, ok := ctx.Value(agentModelRouteKey{}).(agentModelRoute)
	if ok && bindingRole != "embedding" && bindingRole != visionUnderstandingModelRole {
		if route.variant != "" {
			key := agentModelBindingKey(route.agent, route.variant)
			if validAgentModelBindingRole(key) {
				result = append(result, key)
			}
		}
		key := agentModelBindingKey(route.agent, "")
		if validAgentModelBindingRole(key) {
			result = append(result, key)
		}
		if bindingRole == "generic_llm" {
			if route.agent == FormalAgentReplyActionCheck && route.variant == directMediaDeliveryCheckSchema {
				// This direct-only checker follows the configured text model by
				// default; an explicit Agent or variant override still wins.
				bindingRole = conversationReplyModelRole
			}
			switch route.agent {
			case FormalAgentConversationCognition, FormalAgentWakeUp, FormalAgentNPCResponse, FormalAgentExpressionLearning:
				// The configured expression binding is the default text model for
				// character-facing conversation, initiative, NPC expression and
				// the new expression-learning task.
				// An explicit Agent/variant binding still takes precedence.
				bindingRole = conversationReplyModelRole
			}
		}
	}
	return append(result, bindingRole)
}

func agentModelRouteForSchema(schema string) (FormalAgentID, bool) {
	for agent, variants := range agentModelVariants {
		if containsString(variants, schema) {
			return agent, true
		}
	}
	return "", false
}

type agentModelBindingView struct {
	endpoint string
	model    string
	status   string
}

// agentModelRouteCatalog reads the same model_roles authority as Provider
// resolution, without decrypting or returning credentials.
func agentModelRouteCatalog(ctx context.Context, q lifeContextQuerier) ([]map[string]any, error) {
	rows, err := q.Query(ctx, `SELECT r.role,COALESCE(e.id,''),r.model_id,COALESCE(e.capability_status,'') FROM public.model_roles r LEFT JOIN public.provider_endpoints e ON e.id=r.provider_endpoint_id`)
	if err != nil {
		return nil, err
	}
	bindings := map[string]agentModelBindingView{}
	for rows.Next() {
		var role string
		var binding agentModelBindingView
		if err := rows.Scan(&role, &binding.endpoint, &binding.model, &binding.status); err != nil {
			rows.Close()
			return nil, err
		}
		bindings[role] = binding
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := []map[string]any{}
	for _, definition := range FormalAgentDefinitions() {
		variants, writable := agentModelVariants[definition.ID]
		for _, variant := range append([]string{""}, variants...) {
			key := agentModelBindingKey(definition.ID, variant)
			if !writable {
				key = ""
			}
			routeCtx := withAgentModelRoute(context.Background(), definition.ID, variant)
			candidates := agentModelBindingCandidates(routeCtx, providerBindingRole(definition.Role))
			if candidates[len(candidates)-1] == "generic_llm" {
				candidates = append(candidates, "action_realization", "cognitive_assessment", "interaction", "reflection", "initialization", "media_prompt")
			}
			view := map[string]any{"key": key, "agent_id": definition.ID, "variant": variant, "description": definition.Description, "writable": writable, "source": "unconfigured", "state": "unconfigured", "compatibility": "not_verified"}
			if definition.ID == FormalAgentVisualIdentity {
				view["state"] = "stage_dependent"
				view["compatibility"] = "image_input_requires_existing_visual_binding"
				result = append(result, view)
				continue
			}
			for _, candidate := range candidates {
				binding, ok := bindings[candidate]
				if !ok {
					continue
				}
				source := "default"
				if candidate == key && variant != "" {
					source = "variant"
				} else if candidate == agentModelBindingKey(definition.ID, "") {
					source = "agent"
				} else if candidate != "generic_llm" && candidate != conversationReplyModelRole && candidate != visionUnderstandingModelRole && candidate != "embedding" {
					source = "legacy"
				}
				view["source"], view["effective_binding"] = source, candidate
				view["endpoint_id"], view["model_id"] = binding.endpoint, binding.model
				view["state"] = "configured"
				if binding.endpoint == "" || binding.model == "" || strings.EqualFold(binding.status, "failed") {
					view["state"] = "unavailable"
				}
				break
			}
			result = append(result, view)
		}
	}
	return result, nil
}
