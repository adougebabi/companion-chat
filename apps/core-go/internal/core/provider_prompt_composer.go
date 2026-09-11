package core

import (
	"encoding/json"
	"strconv"
	"strings"
)

const providerRuntimeProtocol = `1. 语言：自然语言用中文，协议/字面量保持原文。
2. 约束优先级：core_persona（硬约束）> developing_self（带证据线索）> current_state（当前事实）。
3. 上下文绑定：决策与工具参数必须严格锚定 context（scene, activity, location, mood, appearance）。除 actor_user 明确要求外，禁止擅自变更场景；actor_user 显式变更时标明 context_override.explicit=true。
4. Actor 语义：Human 与 Fluctlight 都是 Actor；消息发送者以 Actor 与关系上下文为准，不要把 transport role=user 当作 actor_user 身份。
5. 认知与生成准则：
   - 认知字段仅写简短摘要，禁止输出推理长文。
   - claims 仅保留有证据的事实或假设，禁止幻觉捏造。
   - 依赖外部能力时直接触发标准 Tool Call；如果文字声称状态已改变、正在改变或将立即改变，且存在对应能力，必须真实调用该能力。
   - 不得绕过标准 Tool Call，直接声称外部能力已经完成；必需能力失败时不得伪造成功。
   - 不得把模型生成的内容伪装成已经发生的事实。
   - 不得把 developing_self 或 current_state 升级为 Core Persona。
6. 多重人格：personality_system 中的 profiles、switching、influence、conflict_resolution、integration、behavior_state_machine 和当前状态都是你的判断输入。你负责在本次 cognition 中判断主导人格、是否切换、行动和回复；服务器只校验并保存你的结构化决定，不根据切换条件自行推导人格。
`

const providerInitializationRuntimeProtocol = `1. 语言：自然语言字段使用中文，协议字段和枚举值保持原文。
2. 任务性质：你正在进行人格初始化信息解析，不是在扮演 actor_self，也不是在进行一次聊天 cognition。
3. 解析边界：只从 Owner 提供的描述中识别、分类和结构化人格信息；不要模拟对话、当前情绪、当前动作、当前回复或未来行动。
4. 多重人格：识别每个人格的独立资料、稳定 ID、行为策略、切换条件、影响关系、冲突处理、融合信息和表达特征；这些内容只是后续 cognition 的输入，初始化阶段不要判断当前哪个人格主导，也不要执行切换。
5. Actor 语义：actor_self 表示正在初始化的 Fluctlight，actor_user 固定表示当前认证 Human 用户；不要把数据库 ID、transport role 或自然语言称呼混入 Actor 标识。
6. 时间与状态：不要把当前场景、疲劳、心情、Presence 或一次性反应写入 Core Persona；Current State 由服务器初始化。
7. 输出边界：只返回初始化 response schema 要求的 JSON 对象；不要输出解释、Markdown、对话、行动建议或隐藏推理。
`

func withContextAuthorityInstruction(messages []map[string]any) []map[string]any {
	if len(messages) == 0 {
		return messages
	}
	return prependSystemMessage(messages, map[string]any{
		"role":    "system",
		"content": providerContextAuthorityRule,
	})
}

// composeProviderMessages centralizes the ordinary (non-media) system and
// dynamic document shape. Existing callers may still provide multiple system
// fragments; they are treated as operation rules and merged deterministically.
func composeProviderMessages(role string, messages []map[string]any) []map[string]any {
	if role == "media_prompt" {
		return formatProviderMessagesForRole(messages, role)
	}
	operationRules := make([]string, 0, len(messages))
	corePersona := map[string]any(nil)
	actorRelationshipContext := map[string]any(nil)
	nonSystem := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		if stringValue(message["role"]) == "system" {
			content := systemMessageContent(message)
			if cleaned, relationship, parsed := extractEmbeddedActorRelationshipContext(content); parsed {
				actorRelationshipContext = relationship
				content = cleaned
			} else if _, value, parsed := decodeProviderJSONPayload(strings.TrimSpace(content)); parsed {
				if relationship := mapValue(mapValue(value)["actor_relationship_context"]); len(relationship) > 0 {
					actorRelationshipContext = relationship
					continue
				}
			}
			if content != "" && content != providerLanguageRule && content != providerContextAuthorityRule {
				operationRules = append(operationRules, content)
			}
			continue
		}
		copyMessage := make(map[string]any, len(message))
		for key, value := range message {
			copyMessage[key] = value
		}
		if content, ok := message["content"].(string); ok {
			if prefix, value, parsed := decodeProviderJSONPayload(strings.TrimSpace(content)); parsed {
				cleaned, found := extractCorePersona(value)
				if found {
					corePersona = mergeCorePersona(corePersona, cleaned)
					value = removeCorePersona(value)
					encoded, err := json.Marshal(value)
					if err == nil {
						content = strings.TrimSpace(string(encoded))
						if prefix != "" {
							content = strings.TrimSpace(prefix) + "\n\n" + content
						}
					}
				}
			}
			copyMessage["content"] = formatProviderDynamicPromptContent(content)
		}
		nonSystem = append(nonSystem, copyMessage)
	}
	result := make([]map[string]any, 0, len(nonSystem)+1)
	result = append(result, map[string]any{"role": "system", "content": renderProviderSystem(operationRules, corePersona, actorRelationshipContext, role)})
	result = append(result, nonSystem...)
	return result
}

// extractEmbeddedActorRelationshipContext handles the merged-system-message
// shape produced by prependSystemMessage. The relationship JSON may sit
// between the context authority rule and the operation rule, so it is not a
// complete JSON document anymore and decodeProviderJSONPayload cannot parse it
// as a standalone system message. Remove only that envelope and keep the
// surrounding operation rules separate from the Actor relationship section.
func extractEmbeddedActorRelationshipContext(content string) (string, map[string]any, bool) {
	marker := `{"actor_relationship_context"`
	start := strings.Index(content, marker)
	if start < 0 {
		return content, nil, false
	}
	var value map[string]any
	decoder := json.NewDecoder(strings.NewReader(content[start:]))
	if err := decoder.Decode(&value); err != nil {
		return content, nil, false
	}
	relationship := mapValue(value["actor_relationship_context"])
	if len(relationship) == 0 {
		return content, nil, false
	}
	consumed := int(decoder.InputOffset())
	if consumed <= 0 || start+consumed > len(content) {
		return content, nil, false
	}
	cleaned := strings.TrimSpace(content[:start] + "\n" + content[start+consumed:])
	return cleaned, relationship, true
}

func renderProviderSystem(operationRules []string, persona, actorRelationshipContext map[string]any, role string) string {
	var builder strings.Builder
	builder.WriteString("# 运行协议\n\n")
	if role == "initialization" {
		builder.WriteString(providerInitializationRuntimeProtocol)
	} else {
		builder.WriteString(providerRuntimeProtocol)
	}
	if len(operationRules) > 0 {
		builder.WriteString("\n\noperation_rules:\n")
		for _, rule := range operationRules {
			builder.WriteString("  - ")
			builder.WriteString(strings.ReplaceAll(strings.TrimSpace(rule), "\n", " "))
			builder.WriteByte('\n')
		}
	}
	if len(actorRelationshipContext) > 0 {
		builder.WriteString("\n# Actor 与关系上下文\n\n")
		builder.WriteString(renderProviderYAMLWithMode(actorRelationshipContext, false))
		builder.WriteByte('\n')
	}
	builder.WriteString("\n# 人格设定\n\n")
	if len(persona) == 0 {
		if role == "initialization" {
			builder.WriteString("当前正在初始化 Core Persona；不要将当前场景、疲劳、心情或一次性反应写入固定人格。\n")
		} else {
			builder.WriteString("当前没有已建立的 Core Persona；不得自行补充固定人格事实。\n")
		}
		return strings.TrimRight(builder.String(), "\n")
	}
	builder.WriteString(renderProviderYAMLWithMode(persona, false))
	return strings.TrimRight(builder.String(), "\n")
}

func extractCorePersona(value any) (map[string]any, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	if persona := mapValue(object["core_persona"]); len(persona) > 0 {
		return filterCorePersona(persona), true
	}
	for _, key := range []string{"context", "context_projection"} {
		if nested := mapValue(object[key]); len(nested) > 0 {
			if persona := mapValue(nested["core_persona"]); len(persona) > 0 {
				return filterCorePersona(persona), true
			}
		}
	}
	return nil, false
}

func mergeCorePersona(existing, next map[string]any) map[string]any {
	if len(next) == 0 {
		return existing
	}
	if len(existing) == 0 {
		return next
	}
	result := cloneMap(existing)
	for key, value := range next {
		if _, exists := result[key]; !exists {
			result[key] = value
		}
	}
	return result
}

func removeCorePersona(value any) any {
	object, ok := value.(map[string]any)
	if !ok {
		return value
	}
	result := cloneMap(object)
	delete(result, "core_persona")
	for _, key := range []string{"context", "context_projection"} {
		if nested := mapValue(result[key]); len(nested) > 0 {
			copyNested := cloneMap(nested)
			delete(copyNested, "core_persona")
			result[key] = copyNested
		}
	}
	return result
}

func filterCorePersona(value map[string]any) map[string]any {
	if data := mapValue(value["data"]); len(data) > 0 {
		value = data
	}
	result := make(map[string]any, 4)
	for _, group := range []string{"identity", "personality", "behavioral_policy", "life_profile", "personality_system"} {
		if source := mapValue(value[group]); len(source) > 0 {
			if group == "personality_system" {
				result[group] = filterPersonalitySystem(source)
			} else {
				result[group] = filterCorePersonaValue(source)
			}
		}
	}
	return result
}

// Personality profile and switching identifiers are semantic protocol values,
// unlike persistence IDs elsewhere in Core Persona. Keep them so cognition can
// name the active profile and reference a declared switch trigger.
func filterPersonalitySystem(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, child := range value {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
		switch normalized {
		case "activeprofileid", "profileid", "fromprofileid", "targetprofileid", "triggerid":
			result[key] = child
		case "profiles", "switching", "influence", "conflictresolution", "integration", "behaviorstatemachine":
			if list, ok := child.([]any); ok {
				result[key] = filterSemanticProfileList(list)
			} else if object := mapValue(child); len(object) > 0 {
				result[key] = filterSemanticProfileValue(object)
			}
		default:
			if nested := mapValue(child); len(nested) > 0 {
				result[key] = filterCorePersonaValue(nested)
			} else if list, ok := child.([]any); ok {
				result[key] = filterCorePersonaList(list)
			} else {
				result[key] = child
			}
		}
	}
	return result
}

func filterSemanticProfileList(list []any) []any {
	result := make([]any, 0, len(list))
	for _, item := range list {
		if object := mapValue(item); len(object) > 0 {
			result = append(result, filterSemanticProfileValue(object))
		} else {
			result = append(result, item)
		}
	}
	return result
}

func filterSemanticProfileValue(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, child := range value {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
		if normalized == "id" || normalized == "profileid" || normalized == "activeprofileid" || normalized == "fromprofileid" || normalized == "targetprofileid" || normalized == "currentprofileid" || normalized == "dominantprofileid" || normalized == "defaultprofileid" || normalized == "triggerid" || normalized == "name" {
			result[key] = child
			continue
		}
		if normalized == "rules" || normalized == "edges" {
			if list, ok := child.([]any); ok {
				result[key] = filterSemanticProfileList(list)
				continue
			}
		}
		if nested := mapValue(child); len(nested) > 0 {
			result[key] = filterCorePersonaValue(nested)
		} else if list, ok := child.([]any); ok {
			result[key] = filterCorePersonaList(list)
		} else {
			result[key] = child
		}
	}
	return result
}

func filterCorePersonaValue(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, child := range value {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
		if normalized == "id" || strings.HasSuffix(normalized, "id") || normalized == "schemaversion" || normalized == "revision" || normalized == "createdat" || normalized == "updatedat" || normalized == "status" || normalized == "provenance" || normalized == "source" || normalized == "instant" {
			continue
		}
		if key == "update_policy" {
			// Automatic evolution controls belong to Core policy, not persona
			// semantics. The protocol already forbids model-side Core mutation.
			continue
		}
		if nested := mapValue(child); len(nested) > 0 {
			result[key] = filterCorePersonaValue(nested)
			continue
		}
		if list, ok := child.([]any); ok {
			result[key] = filterCorePersonaList(list)
			continue
		}
		result[key] = child
	}
	return result
}

func filterCorePersonaList(list []any) []any {
	result := make([]any, 0, len(list))
	for _, item := range list {
		if nested := mapValue(item); len(nested) > 0 {
			result = append(result, filterCorePersonaValue(nested))
		} else {
			result = append(result, item)
		}
	}
	return result
}

func formatProviderDynamicPromptContent(content string) string {
	prefix, value, ok := decodeProviderJSONPayload(strings.TrimSpace(content))
	if !ok {
		return content
	}
	object, isObject := value.(map[string]any)
	if !isObject {
		return formatProviderPromptContentWithMode(content, true)
	}
	document := renderProviderDynamicDocument(object)
	if prefix != "" {
		return strings.TrimSpace(prefix) + "\n\n" + document
	}
	return document
}

func renderProviderDynamicDocument(value map[string]any) string {
	var builder strings.Builder
	contextValue := mapValue(value["context"])
	if len(contextValue) == 0 {
		contextValue = mapValue(value["context_projection"])
	}
	if len(contextValue) == 0 {
		return renderProviderYAMLWithMode(value, true)
	}
	if len(contextValue) > 0 {
		renderProviderDynamicSection(&builder, "当前上下文", providerContextDocument(contextValue), false)
	}
	for _, section := range []struct {
		key   string
		title string
		toon  bool
	}{
		{"developing_self", "Developing Self", true},
		{"schedule", "当前日程", true},
		{"current_state", "当前状态", false},
		{"memories", "记忆", true},
		{"goals", "当前目标", true},
		{"intentions", "当前意图", true},
		{"recent_outcomes", "近期行动结果", true},
		{"recent_messages", "最近对话", true},
		{"relationships", "关系", false},
		{"hypotheses", "假设", true},
		{"drive_slots", "驱动", false},
		{"preference_slots", "偏好", false},
		{"trigger_preferences", "触发偏好", false},
		{"visual_identity", "视觉身份", false},
		{"presence", "在场状态", false},
	} {
		if raw, exists := contextValue[section.key]; exists && !isEmptyProviderValue(raw) {
			if section.key == "current_state" {
				raw = providerCurrentStateDocument(mapValue(raw))
			} else if section.key == "schedule" {
				raw = compactScheduleForProvider(mapValue(raw))
			}
			renderProviderDynamicSection(&builder, section.title, raw, section.toon)
		}
	}
	for _, key := range []string{"current_message", "text", "current_user_text", "event_type", "fact", "evidence", "response_plan", "capability_results", "local_date"} {
		if raw, exists := value[key]; exists && !isEmptyProviderValue(raw) {
			title := "操作输入"
			if key == "current_message" {
				title = "本次 Actor 消息"
			} else if key == "text" || key == "current_user_text" {
				title = "本次 actor_user 输入"
			}
			renderProviderDynamicSection(&builder, title, map[string]any{key: raw}, false)
		}
	}
	if builder.Len() == 0 {
		return renderProviderYAMLWithMode(value, true)
	}
	return strings.TrimRight(builder.String(), "\n")
}

func providerContextDocument(contextValue map[string]any) map[string]any {
	result := make(map[string]any, 4)
	if state := mapValue(contextValue["current_state"]); len(state) > 0 {
		data := mapValue(state["data"])
		if len(data) == 0 {
			data = state
		}
		if life := mapValue(data["life_context"]); len(life) > 0 {
			result["life_context"] = compactLifeContext(life)
		}
	}
	if life := mapValue(contextValue["life_context"]); len(life) > 0 {
		result["life_context"] = compactLifeContext(life)
	}
	for _, key := range []string{"scene", "activity", "location", "mood", "appearance", "current_time", "timezone"} {
		if raw, exists := contextValue[key]; exists && !isEmptyProviderValue(raw) {
			result[key] = raw
		}
	}
	return result
}

func providerCurrentStateDocument(value map[string]any) map[string]any {
	data := mapValue(value["data"])
	if len(data) == 0 {
		data = value
	}
	result := cloneMap(data)
	delete(result, "life_context")
	return result
}

func renderProviderDynamicSection(builder *strings.Builder, title string, value any, useTOON bool) {
	if isEmptyProviderValue(value) {
		return
	}
	builder.WriteString("# ")
	builder.WriteString(title)
	builder.WriteString("\n\n")
	if rows, ok := value.([]map[string]any); ok {
		value = mapsToAny(rows)
	}
	if rows, ok := value.([]any); ok && useTOON {
		if fields, valid := providerTOONFields(rows); valid {
			builder.WriteString(renderProviderTOONTable(rows, fields))
			builder.WriteString("\n")
			return
		}
	}
	if object, ok := value.(map[string]any); ok && useTOON {
		builder.WriteString(renderProviderYAMLWithMode(object, true))
	} else {
		builder.WriteString(renderProviderYAMLWithMode(value, false))
	}
	builder.WriteString("\n")
}

func renderProviderTOONTable(rows []any, fields []string) string {
	var builder strings.Builder
	builder.WriteString("[")
	builder.WriteString(strconv.Itoa(len(rows)))
	builder.WriteString("]{")
	builder.WriteString(strings.Join(fields, ","))
	builder.WriteString("}:\n")
	for _, raw := range rows {
		object := mapValue(raw)
		for index, field := range fields {
			if index > 0 {
				builder.WriteByte('|')
			}
			builder.WriteString(formatProviderTOONCell(object[field]))
		}
		builder.WriteByte('\n')
	}
	return strings.TrimRight(builder.String(), "\n")
}

func mapsToAny(values []map[string]any) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func isEmptyProviderValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case []map[string]any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}
