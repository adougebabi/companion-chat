package core

import (
	"encoding/json"
	"strconv"
	"strings"
)

const providerRuntimeProtocol = `1. 语言：自然语言用中文，协议/字面量保持原文。
2. 约束优先级：core_persona（硬约束）> developing_self（带证据线索）> current_state（当前事实）。
3. 上下文绑定：决策与工具参数必须严格锚定 context（scene, activity, location, mood, appearance）。除用户明确要求外，禁止擅自变更场景；用户显式变更时标明 context_override.explicit=true。
4. 认知与生成准则：
   - 认知字段仅写简短摘要，禁止输出推理长文。
   - claims 仅保留有证据的事实或假设，禁止幻觉捏造。
   - 依赖外部能力时直接触发标准 Tool Call。
   - 不得绕过标准 Tool Call，直接声称外部能力已经完成。
   - 不得把模型生成的内容伪装成已经发生的事实。
   - 不得把 developing_self 或 current_state 升级为 Core Persona。
`

// composeProviderMessages centralizes the ordinary (non-media) system and
// dynamic document shape. Existing callers may still provide multiple system
// fragments; they are treated as operation rules and merged deterministically.
func composeProviderMessages(role string, messages []map[string]any) []map[string]any {
	if role == "media_prompt" {
		return formatProviderMessagesForRole(messages, role)
	}
	operationRules := make([]string, 0, len(messages))
	corePersona := map[string]any(nil)
	nonSystem := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		if stringValue(message["role"]) == "system" {
			content := systemMessageContent(message)
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
	result = append(result, map[string]any{"role": "system", "content": renderProviderSystem(operationRules, corePersona, role)})
	result = append(result, nonSystem...)
	return result
}

func renderProviderSystem(operationRules []string, persona map[string]any, role string) string {
	var builder strings.Builder
	builder.WriteString("# 运行协议\n\n")
	builder.WriteString(providerRuntimeProtocol)
	if len(operationRules) > 0 {
		builder.WriteString("\n\noperation_rules:\n")
		for _, rule := range operationRules {
			builder.WriteString("  - ")
			builder.WriteString(strings.ReplaceAll(strings.TrimSpace(rule), "\n", " "))
			builder.WriteByte('\n')
		}
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
	for _, group := range []string{"identity", "personality", "behavioral_policy", "life_profile"} {
		if source := mapValue(value[group]); len(source) > 0 {
			result[group] = filterCorePersonaValue(source)
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
		{"current_state", "当前状态", false},
		{"memories", "记忆", true},
		{"goals", "当前目标", true},
		{"intentions", "当前意图", true},
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
			}
			renderProviderDynamicSection(&builder, section.title, raw, section.toon)
		}
	}
	for _, key := range []string{"text", "current_user_text", "event_type", "fact", "evidence", "response_plan", "tool_results", "local_date"} {
		if raw, exists := value[key]; exists && !isEmptyProviderValue(raw) {
			title := "操作输入"
			if key == "text" || key == "current_user_text" {
				title = "本次用户输入"
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
