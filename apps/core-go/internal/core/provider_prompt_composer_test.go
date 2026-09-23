package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Kept test-local while production callers use PromptComposer.ComposeTaskMessages.
func composeProviderMessages(role string, messages []map[string]any) []map[string]any {
	return composeTaskMessages(role, messages)
}

func TestProductionModelCallersUseTypedTaskBoundaries(t *testing.T) {
	taskSource, err := os.ReadFile(filepath.Clean("model_tasks.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, boundary := range []string{"RunNativeCognitionTask", "RunDailyReviewTask", "RunReflectionProposalTask", "RunPersistentSwitchTask", "RunMediaPromptTask", "RunMediaQualityTask", "RunConversationSummaryTask", "RunScheduleGenerationTask", "RunScheduleReplanTask"} {
		if !strings.Contains(string(taskSource), boundary) {
			t.Fatalf("typed task boundary %s is missing", boundary)
		}
	}
	for _, name := range []string{"agent_result_adapter.go", "reflection_runtime_v2.go"} {
		content, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		taskBoundary := "RunADKStructuredTask"
		if name == "agent_result_adapter.go" {
			taskBoundary = "RunConversationCognitionAgent"
		} else if name == "reflection_runtime_v2.go" {
			taskBoundary = "RunReflectionProposalTask"
		}
		if !strings.Contains(text, taskBoundary) {
			t.Fatalf("%s does not use the typed task/ADK boundary", name)
		}
		for _, forbidden := range []string{"withActorRelationshipSystemContext", `"current_message"`, "compactCognitionContext(projection)", "StructuredWithToolsSchema("} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s retains legacy Main assembly fragment %q", name, forbidden)
			}
		}
	}
	runtimeSource, err := os.ReadFile(filepath.Clean("conversation_runtime.go"))
	if err != nil {
		t.Fatal(err)
	}
	runtimeText := string(runtimeSource)
	for _, required := range []string{
		"type TakeoverJudgeInput struct",
		"type TakeoverReplyInput struct",
		"func (r *conversationRuntime) RunTakeoverJudge",
		"FormalAgentTakeoverJudge",
		"func (r *conversationRuntime) RunTakeoverReply",
		"FormalAgentTakeoverReply",
	} {
		if !strings.Contains(runtimeText, required) {
			t.Fatalf("conversation runtime is missing standalone takeover Agent boundary %q", required)
		}
	}
}

func TestAssembledProviderMessagesRequireOneSystemRealRolesAndFinalUser(t *testing.T) {
	valid := []map[string]any{{"role": "system", "content": "stable"}, {"role": "user", "content": "runtime"}, {"role": "assistant", "content": "recent"}, {"role": "user", "content": "current"}}
	if !validAssembledProviderMessages(valid) {
		t.Fatalf("valid B-layout rejected: %#v", valid)
	}
	for name, messages := range map[string][]map[string]any{
		"tool role":       {{"role": "system", "content": "stable"}, {"role": "tool", "content": "forbidden"}, {"role": "user", "content": "current"}},
		"second system":   {{"role": "system", "content": "stable"}, {"role": "system", "content": "dynamic"}, {"role": "user", "content": "current"}},
		"assistant final": {{"role": "system", "content": "stable"}, {"role": "assistant", "content": "not current"}},
	} {
		if validAssembledProviderMessages(messages) {
			t.Fatalf("%s shape was accepted: %#v", name, messages)
		}
	}
}

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
				"schedule": map[string]any{
					"local_date": "2026-09-06", "timezone": "Asia/Shanghai", "revision": 2,
					"completed_before": "2026-09-06T00:10:00+08:00",
					"items":            []any{map[string]any{"start_at": "2026-09-06T00:00:00+08:00", "end_at": "2026-09-06T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}},
				},
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
	for _, heading := range []string{"# 当前上下文", "# 当前日程", "# 记忆", "# 最近对话", "# 本次 actor_user 输入"} {
		if !strings.Contains(user, heading) {
			t.Fatalf("dynamic heading %q missing: %s", heading, user)
		}
	}
	if !strings.Contains(user, "expected_revision: 2") || !strings.Contains(user, "completed_before") {
		t.Fatalf("schedule was dropped from provider dynamic payload: %s", user)
	}
	if strings.Contains(user, "core_persona") || strings.Contains(user, "fluctlight_1234567890abcdef") {
		t.Fatalf("core persona remained in dynamic context: %s", user)
	}
	if !strings.Contains(user, "[2]{confidence,content,created_at,evidence_refs,importance,type}") || !strings.Contains(user, "current_time: '2026-09-06 00:10:00 CST'") {
		t.Fatalf("dynamic formatting did not preserve TOON/time: %s", user)
	}
}

func TestRecentActionOutcomeProviderRequestUsesOnlySafeProjection(t *testing.T) {
	ref := "outcome:ctx_0123456789abcdef0123456789abcdef"
	projection := ContextProjection{
		CorePersona:  map[string]any{"authority": "hard_constraint", "data": map[string]any{}},
		CurrentState: map[string]any{"authority": "transient_state", "data": map[string]any{}},
		RecentOutcomes: []map[string]any{{
			"ref": ref, "id": "outcome_internal", "action_id": "action_internal", "call_id": "call_internal",
			"capability_name": "media.image.generate", "status": "completed", "success_boundary": "durable_media_intent_created",
			"occurred_at": "2026-09-10T12:00:00Z", "goal_refs": []any{"goal:ctx_0123456789abcdef0123456789abcdef"},
			"expected":           map[string]any{"text": "private expected text"},
			"observed":           map[string]any{"status": "completed", "target_kind": "conversation_message", "text": "private assistant text", "target_ref": "message_internal"},
			"context_references": map[string]any{ref: map[string]any{"entity_id": "internal_entity"}},
			"evidence_refs":      []any{"fact_internal"}, "provenance": map[string]any{"provider_request_id": "provider_internal"},
		}},
	}
	contextValue := compactCognitionContext(projection)
	formatted := composeProviderMessages("cognitive_assessment", []map[string]any{{
		"role": "user", "content": jsonString(map[string]any{"context": contextValue}),
	}})
	if len(formatted) != 2 || stringValue(formatted[0]["role"]) != "system" || stringValue(formatted[1]["role"]) != "user" {
		t.Fatalf("formatted messages=%#v", formatted)
	}
	user := stringValue(formatted[1]["content"])
	for _, allowed := range []string{"# 近期行动结果", ref, "media.image.generate", "completed", "durable_media_intent_created", "target_kind"} {
		if !strings.Contains(user, allowed) {
			t.Fatalf("allowlisted outcome field %q missing: %s", allowed, user)
		}
	}
	for _, forbidden := range []string{
		"outcome_internal", "action_internal", "call_internal", "private expected text", "private assistant text",
		"message_internal", "context_references", "internal_entity", "evidence_refs", "fact_internal", "provenance", "provider_internal", "expected",
	} {
		if strings.Contains(user, forbidden) {
			t.Fatalf("provider request leaked outcome field %q: %s", forbidden, user)
		}
	}
}

func TestComposeProviderMessagesPreservesSemanticPersonalityIdentifiers(t *testing.T) {
	messages := []map[string]any{{"role": "user", "content": jsonString(map[string]any{
		"context": map[string]any{"core_persona": map[string]any{"data": map[string]any{
			"identity": map[string]any{"name": "影者", "id": "fluctlight_internal"},
			"personality_system": map[string]any{
				"active_profile_id": "guarded",
				"profiles":          []any{map[string]any{"id": "warm", "name": "温柔"}, map[string]any{"id": "guarded", "name": "克制"}},
				"switching":         map[string]any{"rules": []any{map[string]any{"id": "stress_trigger", "condition": "压力"}}},
			},
		}}},
	})}}
	formatted := composeProviderMessages("cognitive_assessment", messages)
	system := stringValue(formatted[0]["content"])
	for _, fragment := range []string{"active_profile_id: guarded", "id: warm", "id: guarded", "id: stress_trigger"} {
		if !strings.Contains(system, fragment) {
			t.Fatalf("semantic profile identifier %q missing: %s", fragment, system)
		}
	}
	if strings.Contains(system, "fluctlight_internal") {
		t.Fatalf("persistence actor id leaked: %s", system)
	}
}

func TestComposeProviderMessagesPreservesMultiPersonalityDecisionInputs(t *testing.T) {
	messages := []map[string]any{{"role": "user", "content": jsonString(map[string]any{
		"context": map[string]any{"core_persona": map[string]any{"data": map[string]any{
			"personality_system": map[string]any{
				"mode": "multiple", "active_profile_id": "warm",
				"profiles": []any{map[string]any{
					"id": "warm", "name": "温柔", "voice": map[string]any{"speed": 0.8, "volume": 0.4},
					"body_language":        map[string]any{"posture": "开放"},
					"behavior_loops":       map[string]any{"loops": []any{"先安抚再行动"}},
					"scenario_behavior":    map[string]any{"conflict": "先询问"},
					"secrets":              map[string]any{"information_asymmetry": map[string]any{"owner": "隐藏"}},
					"intimacy_progression": map[string]any{"stage": "升温"},
					"output_preferences":   map[string]any{"channels": []any{"text", "image"}},
					"fears":                []any{"失去信任"}, "desires": []any{"持续靠近"},
					"extensions": map[string]any{"future_field": "保留"},
				}},
				"switching":              map[string]any{"rules": []any{map[string]any{"id": "stress", "condition": "高压", "target_profile_id": "guarded"}}},
				"influence":              map[string]any{"edges": []any{map[string]any{"from_profile_id": "warm", "to_profile_id": "guarded", "strength": 0.7, "direction": "toward"}}},
				"conflict_resolution":    map[string]any{"strategy": "dominant_profile", "dominant_profile_id": "warm"},
				"integration":            map[string]any{"fusion_progress": 0.35, "stage": "分离共存"},
				"behavior_state_machine": map[string]any{"initial_state": "calm"},
			},
		}}},
	})}}
	formatted := composeProviderMessages("cognitive_assessment", messages)
	system := stringValue(formatted[0]["content"])
	for _, fragment := range []string{"speed: 0.8", "volume: 0.4", "posture: 开放", "先安抚再行动", "隐藏", "升温", "持续靠近", "stress", "target_profile_id: guarded", "fusion_progress: 0.35", "dominant_profile_id: warm"} {
		if !strings.Contains(system, fragment) {
			t.Fatalf("multi-personality decision input %q missing: %s", fragment, system)
		}
	}
}

func TestComposeProviderMessagesUsesAnalysisProtocolForInitialization(t *testing.T) {
	formatted := composeProviderMessages("initialization", []map[string]any{
		{"role": "system", "content": "Analyze the Owner description and return the initialization schema."},
		{"role": "user", "content": "她有两个独立人格。"},
	})
	if len(formatted) != 2 || formatted[0]["role"] != "system" {
		t.Fatalf("initialization message shape = %#v", formatted)
	}
	system := stringValue(formatted[0]["content"])
	for _, required := range []string{"人格初始化信息解析", "不要模拟对话", "不要判断当前哪个人格主导", "actor_user"} {
		if !strings.Contains(system, required) {
			t.Fatalf("initialization protocol missing %q: %s", required, system)
		}
	}
	for _, forbidden := range []string{"本次 cognition 中判断主导人格、是否切换、行动和回复", "当前没有已建立的 Core Persona"} {
		if strings.Contains(system, forbidden) {
			t.Fatalf("initialization protocol contains runtime instruction %q: %s", forbidden, system)
		}
	}
}

func TestComposeProviderMessagesRendersActorRelationshipSystemContext(t *testing.T) {
	projection := ContextProjection{
		SelfActor:      map[string]any{"ref": "actor_self", "type": "fluctlight", "actor_id": "fl-1", "display_name": "影者"},
		CurrentSpeaker: map[string]any{"ref": "actor_user", "type": "human", "actor_id": "human-1", "display_name": "actor_user"},
		Relationships: []map[string]any{{
			"target_actor_id": "human-1",
			"role":            map[string]any{"primary": "romantic_partner"},
			"trend":           "improving",
			"revision":        3,
		}},
	}
	messages := withActorRelationshipSystemContext([]map[string]any{{"role": "user", "content": `{"current_message":{"sender":{"ref":"actor_user","type":"human"},"content":"你好"}}`}}, projection)
	formatted := composeProviderMessages("cognitive_assessment", messages)
	if len(formatted) != 2 {
		t.Fatalf("formatted messages = %#v", formatted)
	}
	system := stringValue(formatted[0]["content"])
	if !strings.Contains(system, "# Actor 与关系上下文") || !strings.Contains(system, "romantic_partner") || !strings.Contains(system, "actor_user") {
		t.Fatalf("relationship context missing from system: %s", system)
	}
	if strings.Contains(system, "human-1") {
		t.Fatalf("raw actor id leaked into provider system: %s", system)
	}
}

func TestComposeProviderMessagesSeparatesRelationshipFromMergedSystemRules(t *testing.T) {
	projection := ContextProjection{
		SelfActor:      map[string]any{"ref": "actor_self", "type": "fluctlight", "actor_id": "fl-1", "display_name": "影者"},
		CurrentSpeaker: map[string]any{"ref": "actor_user", "type": "human", "actor_id": "human-1", "display_name": "actor_user"},
		Relationships: []map[string]any{{
			"target_actor_id": "human-1",
			"role":            map[string]any{"label": "恋人"},
			"trend":           "improving",
			"revision":        1,
		}},
	}
	messages := withActorRelationshipSystemContext([]map[string]any{
		{"role": "system", "content": capabilityConversationPolicyInstruction},
		{"role": "user", "content": `{"current_message":{"content":"请帮我做成视觉作品，并告诉我构图重点"}}`},
	}, projection)
	messages = withContextAuthorityInstruction(messages)
	formatted := composeProviderMessages("cognitive_assessment", messages)
	system := stringValue(formatted[0]["content"])
	if !strings.Contains(system, "# Actor 与关系上下文") || !strings.Contains(system, "label: 恋人") {
		t.Fatalf("relationship context missing after system merge: %s", system)
	}
	if strings.Contains(system, "actor_relationship_context") {
		t.Fatalf("relationship envelope leaked into operation rules: %s", system)
	}
	if strings.Count(system, "# 人格设定") != 1 || strings.Count(system, "# Actor 与关系上下文") != 1 {
		t.Fatalf("system sections duplicated: %s", system)
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

func TestRenderProviderSystemOmitsMultiPersonalityRuleForSingleAndFiltersRedundantRules(t *testing.T) {
	singlePersona := map[string]any{
		"shared_identity": map[string]any{"identity": map[string]any{"name": "摇光"}},
		"working_persona": map[string]any{"personality": map[string]any{"traits": "温和"}},
	}
	rendered := renderProviderSystem([]string{providerContextAuthorityRule, capabilityConversationPolicyInstruction}, singlePersona, nil, "cognitive_assessment")

	// 1. Single personality should NOT contain rule 6 (多重人格)
	if strings.Contains(rendered, "6. 多重人格") || strings.Contains(rendered, "personality_system 中的 profiles") {
		t.Fatalf("single personality prompt should not contain multi-personality rule: %s", rendered)
	}
	// 2. providerContextAuthorityRule should be filtered out because it is redundant with runtime protocol
	if strings.Contains(rendered, "life_context authority 为 confirmed Event") {
		t.Fatalf("operation_rules duplicated providerContextAuthorityRule: %s", rendered)
	}
	// 3. capabilityConversationPolicyInstruction should remain
	if !strings.Contains(rendered, "正式 Agent") {
		t.Fatalf("capabilityConversationPolicyInstruction was unexpectedly dropped: %s", rendered)
	}
}

func TestFilterCorePersonaStripsDialogueExamplesWhileKeepingLore(t *testing.T) {
	persona := map[string]any{
		"identity": map[string]any{
			"name":        "苏晚",
			"background":  "从小在成都长大，热爱绘画与传统工艺。",
			"description": "性格温和，善于观察细节。",
		},
		"extensions": map[string]any{
			"world_info":                    "世界设定：一个充满古籍修复师与画家的安宁小镇。",
			"special_ritual":                "睡前整理画稿。",
			"top_level.mes_example":         "<START>\n{{user}}: 你好\n{{char}}: 嗨，今天过得好吗？\n<START>\n{{user}}: 请帮我看看这幅画\n{{char}}: 好的，线条非常细腻。",
			"top_level.alternate_greetings": []any{"你好呀！", "欢迎来到我的画室。"},
			"top_level.first_mes":           "窗外的雨停了，要喝杯茶吗？",
		},
	}

	filtered := filterCorePersona(persona)
	serialized := jsonString(filtered)

	// Dialogue examples should be stripped
	for _, stripped := range []string{"mes_example", "alternate_greetings", "first_mes", "你好呀！", "欢迎来到我的画室", "窗外的雨停了"} {
		if strings.Contains(serialized, stripped) {
			t.Fatalf("dialogue example %q leaked into filtered persona: %s", stripped, serialized)
		}
	}

	// Lore, background, and settings must be kept
	for _, kept := range []string{"从小在成都长大", "性格温和，善于观察细节", "一个充满古籍修复师与画家的安宁小镇", "睡前整理画稿"} {
		if !strings.Contains(serialized, kept) {
			t.Fatalf("lore/setting %q was dropped from persona: %s", kept, serialized)
		}
	}
}

func TestSinglePersonalityProtocolOmitsEvidenceRefsRuleAndFiltersPersonaTools(t *testing.T) {
	singlePersona := map[string]any{
		"shared_identity": map[string]any{"identity": map[string]any{"name": "测试角色"}},
	}
	rendered := renderProviderSystem(nil, singlePersona, nil, "cognitive_assessment")
	if strings.Contains(rendered, "引用边界") || strings.Contains(rendered, "evidence_refs 只能逐字使用") {
		t.Fatalf("single personality protocol contains evidence_refs rule: %s", rendered)
	}

	defs := []CapabilityDefinition{
		{Name: "conversation.reply"},
		{Name: "persona.switch"},
		{Name: "persona.takeover"},
		{Name: "memory.recall"},
	}
	filtered := filterPersonaActionCapabilities(defs)
	if len(filtered) != 2 || filtered[0].Name != "conversation.reply" || filtered[1].Name != "memory.recall" {
		t.Fatalf("filterPersonaActionCapabilities = %#v", filtered)
	}
}
