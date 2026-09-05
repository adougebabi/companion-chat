package core

import (
	"strings"
	"testing"
)

func TestComposeProviderMessagesSeparatesFixedPersonaAndDynamicContext(t *testing.T) {
	messages := []map[string]any{
		{"role": "system", "content": "只输出 conversation_turn_response。"},
		{"role": "user", "content": jsonString(map[string]any{
			"text": "我有点困了，但还不想回去。",
			"context": map[string]any{
				"core_persona": map[string]any{
					"schema_version": 1,
					"data": map[string]any{
						"identity":          map[string]any{"id": "fluctlight_1234567890abcdef", "name": "摇光"},
						"personality":       map[string]any{"curiosity": 0.8, "update_policy": map[string]any{"max_delta": 0.05}},
						"behavioral_policy": map[string]any{"response_style": "温和简洁"},
						"life_profile":      map[string]any{"preferences": map[string]any{"place": "安静"}},
					},
				},
				"current_state": map[string]any{"data": map[string]any{"life_context": map[string]any{"scene": "咖啡馆", "current_time": "2026-09-06 00:10:00 CST", "timezone": "Asia/Shanghai"}}},
				"memories": []any{
					map[string]any{"type": "preference", "content": "喜欢安静的咖啡馆", "confidence": 0.9, "importance": 0.7, "created_at": "2026-09-01T00:00:00Z", "evidence_refs": []any{"fact_a"}},
					map[string]any{"type": "preference", "content": "不喜欢被连续追问", "confidence": 0.8, "importance": 0.6, "created_at": "2026-09-02T00:00:00Z", "evidence_refs": []any{"fact_b"}},
				},
				"recent_messages": []any{
					map[string]any{"role": "assistant", "time": "00:01", "content": "这里很安静。"},
					map[string]any{"role": "user", "time": "00:02", "content": "我有点困了，但还不想回去。"},
				},
			},
		})},
	}
	formatted := composeProviderMessages("cognitive_assessment", messages)
	if len(formatted) != 2 || formatted[0]["role"] != "system" {
		t.Fatalf("formatted messages = %#v", formatted)
	}
	system := stringValue(formatted[0]["content"])
	if !strings.Contains(system, "# 运行协议") || !strings.Contains(system, "# 人格设定") || !strings.Contains(system, "摇光") || !strings.Contains(system, "只输出 conversation_turn_response") {
		t.Fatalf("system composition = %s", system)
	}
	for _, leaked := range []string{"schema_version", "fluctlight_1234567890abcdef", "update_policy", "max_delta"} {
		if strings.Contains(system, leaked) {
			t.Fatalf("internal persona field %q leaked into system: %s", leaked, system)
		}
	}
	user := stringValue(formatted[1]["content"])
	for _, heading := range []string{"# 当前上下文", "# 记忆", "# 最近对话", "# 本次用户输入"} {
		if !strings.Contains(user, heading) {
			t.Fatalf("dynamic heading %q missing: %s", heading, user)
		}
	}
	if strings.Contains(user, "core_persona") || strings.Contains(user, "fluctlight_1234567890abcdef") {
		t.Fatalf("core persona remained in dynamic context: %s", user)
	}
	if !strings.Contains(user, "[2]{confidence,content,created_at,evidence_refs,importance,type}") || !strings.Contains(user, "current_time: '2026-09-06 00:10:00 CST'") {
		t.Fatalf("dynamic formatting did not preserve TOON/time: %s", user)
	}
}

func TestComposeProviderMessagesKeepsMediaPromptOutOfOrdinaryComposer(t *testing.T) {
	messages := []map[string]any{
		{"role": "system", "content": "media prompt instruction"},
		{"role": "user", "content": `{"items":[{"name":"a"},{"name":"b"}]}`},
	}
	formatted := composeProviderMessages("media_prompt", messages)
	if len(formatted) != 2 || stringValue(formatted[0]["content"]) != "media prompt instruction" {
		t.Fatalf("media messages changed by ordinary composer: %#v", formatted)
	}
	if strings.Contains(stringValue(formatted[1]["content"]), "# 运行协议") || strings.Contains(stringValue(formatted[1]["content"]), "items[2]{name}") {
		t.Fatalf("media payload used ordinary composition: %#v", formatted[1])
	}
}

func TestProviderTOONCellQuotesDelimiters(t *testing.T) {
	for _, value := range []string{"包含,逗号", "包含|竖线", "包含[括号]"} {
		formatted := formatProviderTOONCell(value)
		if !strings.HasPrefix(formatted, "'") || !strings.HasSuffix(formatted, "'") {
			t.Fatalf("TOON cell %q was not quoted: %q", value, formatted)
		}
	}
}

func TestDynamicDocumentPreservesReflectionEvidence(t *testing.T) {
	content := jsonString(map[string]any{
		"evidence": []any{map[string]any{"event_type": "conversation.turn", "evidence_ref": "sequence:7", "payload": map[string]any{"summary": "用户表达疲惫"}}},
		"context":  map[string]any{"current_state": map[string]any{"data": map[string]any{"life_context": map[string]any{"current_time": "2026-09-06 00:10:00 CST", "timezone": "Asia/Shanghai"}}}},
	})
	formatted := formatProviderDynamicPromptContent(content)
	if !strings.Contains(formatted, "# 操作输入") || !strings.Contains(formatted, "evidence:") || !strings.Contains(formatted, "sequence:7") {
		t.Fatalf("reflection evidence was omitted: %s", formatted)
	}
}

func TestDynamicDocumentKeepsNonContextOperationPayloads(t *testing.T) {
	formatted := formatProviderDynamicPromptContent(jsonString(map[string]any{
		"local_date":   "2026-09-06",
		"timezone":     "Asia/Shanghai",
		"identity":     map[string]any{"name": "摇光"},
		"life_profile": map[string]any{"preferences": map[string]any{"place": "安静"}},
	}))
	for _, fragment := range []string{"local_date:", "timezone:", "identity:", "life_profile:"} {
		if !strings.Contains(formatted, fragment) {
			t.Fatalf("operation payload lost %q: %s", fragment, formatted)
		}
	}
}

func TestComposeProviderMessagesAlwaysEmitsOneLeadingSystem(t *testing.T) {
	for _, role := range []string{"initialization", "cognitive_assessment", "action_realization", "reflection", "wake_up"} {
		formatted := composeProviderMessages(role, []map[string]any{
			{"role": "system", "content": "operation rule"},
			{"role": "system", "content": providerContextAuthorityRule},
			{"role": "user", "content": `{"context":{"current_state":{"data":{"life_context":{"current_time":"2026-09-06 00:10:00 CST","timezone":"Asia/Shanghai"}}}}}`},
		})
		if len(formatted) != 2 || formatted[0]["role"] != "system" {
			t.Fatalf("role %s did not emit leading system: %#v", role, formatted)
		}
		systemCount := 0
		for _, message := range formatted {
			if message["role"] == "system" {
				systemCount++
			}
		}
		if systemCount != 1 || !strings.Contains(stringValue(formatted[0]["content"]), "operation rule") || !strings.Contains(stringValue(formatted[0]["content"]), "# 人格设定") {
			t.Fatalf("role %s system shape = %#v", role, formatted)
		}
	}
}

func TestCompactContextPreservesNaturalIdLikeText(t *testing.T) {
	compact := compactCognitionContext(ContextProjection{
		CorePersona:  map[string]any{"authority": "hard_constraint", "data": map[string]any{}},
		CurrentState: map[string]any{"authority": "transient_state", "data": map[string]any{}},
		Memories:     []map[string]any{{"type": "semantic", "content": "文档中提到 memory_1234567890abcdef 这个名称", "confidence": 0.8}},
	})
	if !strings.Contains(jsonString(compact), "memory_1234567890abcdef") {
		t.Fatalf("natural ID-like text was unexpectedly rewritten: %#v", compact)
	}
}
