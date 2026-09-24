package core

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// This controlled Provider protocol test exercises the formal compiler,
// persisted portrait, actual Main request, native ToolCall, PostgreSQL query,
// Tool result feedback, and final model decision. It is separate from live
// Provider acceptance.
func TestCompiledWorkingPersonaNativeDetailChain(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "portrait-chain-owner", "portrait-chain-fluctlight", "portrait-chain-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	resource, err := repository.GetFluctlight(ctx, fluctlightID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	corePersona := cloneMap(resource.CorePersona)
	life := cloneMap(mapValue(corePersona["life_profile"]))
	life["preferences"] = map[string]any{"drink": "喜欢咖啡，但不喜欢甜咖啡"}
	corePersona["life_profile"] = life
	secret := "只有完整设定知道的往事-" + stableDigest(fluctlightID)[:12]
	system := cloneMap(mapValue(corePersona["personality_system"]))
	profiles := arrayValue(system["profiles"])
	for _, raw := range profiles {
		profile := mapValue(raw)
		if stringValue(profile["id"]) == "spark" {
			profile["secrets"] = map[string]any{"past": secret}
		}
	}
	corePersona["personality_system"] = system
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET core_persona=$2,life_profile=$3 WHERE id=$1`, fluctlightID, jsonBytes(corePersona), jsonBytes(life)); err != nil {
		t.Fatal(err)
	}
	compileIndex := 0
	var firstRequest, continuedRequest string
	router := newFakeProviderRouter().on("persona_compilation_response", func(_ map[string]any) fakeProviderResult {
		compileIndex++
		marker := takeoverChainSparkMarker
		if compileIndex == 2 {
			marker = takeoverChainTwilightMarker
		}
		return fakeProviderResult{Structured: map[string]any{
			"portrait_text": "我是摇光。" + marker + "。喜欢咖啡，但不喜欢甜咖啡。",
		}}
	}).on("conversation_turn_response", func(payload map[string]any) fakeProviderResult {
		if firstRequest == "" {
			firstRequest = jsonString(payload)
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("portrait-detail-read", personaDetailCapabilityName, map[string]any{"operation": "read", "section_id": "profile.secrets"})}}
		}
		continuedRequest = jsonString(payload)
		final := nativePersonaFinal()
		final.Structured["response_intent"] = "已查到：" + secret
		return final
	}).otherwise(func(payload map[string]any) fakeProviderResult {
		t.Logf("unrouted provider response format: %s", jsonString(payload["response_format"]))
		return fakeProviderResult{Status: 500}
	})
	app := newTestApp(t, repository, router)
	if _, err := repository.Pool().Exec(ctx, `DELETE FROM public.fluctlight_working_personas WHERE fluctlight_id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := app.VerifyWorkingPersonasReady(ctx); err == nil {
		t.Fatal("runtime gate accepted missing portrait")
	}
	compiled, err := app.compileFoundationWorkingPersonas(ctx, fluctlightID, corePersona, resource.CurrentRevision, "llm_defined", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled) != 2 || compileIndex != 2 {
		t.Fatalf("compiled profiles=%d calls=%d", len(compiled), compileIndex)
	}
	for index, payload := range router.requests {
		if index >= 2 {
			break
		}
		request := jsonString(payload)
		t.Logf("controlled one-time compiler request profile_index=%d chars=%d bytes=%d estimated_tokens=%d", index, len([]rune(request)), len([]byte(request)), EstimatePromptTokens(request))
	}
	oldProjection := ContextProjection{CorePersona: map[string]any{"data": corePersona}, PersonalitySystem: mapValue(corePersona["personality_system"]), PersonalityRuntime: map[string]any{"active_profile_id": "spark"}}
	oldWorking, _ := projectWorkingPersona(oldProjection, "spark")
	t.Logf("controlled persona baseline old chars=%d bytes=%d estimated_tokens=%d; compiled chars=%d bytes=%d estimated_tokens=%d", len([]rune(jsonString(oldWorking))), len([]byte(jsonString(oldWorking))), EstimatePromptTokens(oldWorking), len([]rune(jsonString(renderCompiledWorkingPersona(compiled[0])))), len([]byte(jsonString(renderCompiledWorkingPersona(compiled[0])))), EstimatePromptTokens(renderCompiledWorkingPersona(compiled[0])))
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error { return insertCompiledWorkingPersonasTx(ctx, tx, fluctlightID, compiled) }); err != nil {
		t.Fatal(err)
	}
	result, err := app.RunConversationCognitionAgent(ctx, ConversationCognitionAgentInput{
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		RunID: "portrait-chain-run", CurrentInput: "忙完后有空，请结合你自己的喜好安排；另外，你过去那件具体经历是什么？",
	})
	if err != nil {
		t.Fatal(err)
	}
	if compileIndex != 2 {
		t.Fatalf("ordinary conversation triggered %d additional portrait compilations", compileIndex-2)
	}
	if diagnostic := mapValue(result.Diagnostics["working_persona"]); stringValue(diagnostic["profile_id"]) != "spark" || stringValue(diagnostic["source_hash_prefix"]) == "" || diagnostic["cache_hit"] != false {
		t.Fatalf("portrait version/cache diagnostic missing: %#v", diagnostic)
	}
	if !strings.Contains(firstRequest, "portrait_text") || strings.Contains(firstRequest, "source_refs") || strings.Contains(firstRequest, "stable_preferences") {
		t.Fatalf("formal request did not carry a single text portrait: %s", firstRequest)
	}
	if !strings.Contains(firstRequest, "喜欢咖啡") || strings.Contains(firstRequest, secret) || strings.Contains(firstRequest, takeoverChainTwilightMarker) {
		t.Fatal("first model request lost stable preference or leaked full/foreign persona")
	}
	if !strings.Contains(continuedRequest, secret) || !strings.Contains(continuedRequest, `"role":"tool"`) {
		t.Fatal("real Tool result was not fed to the next model request")
	}
	t.Logf("controlled full-request initial chars=%d bytes=%d estimated_tokens=%d; continuation chars=%d bytes=%d estimated_tokens=%d", len([]rune(firstRequest)), len([]byte(firstRequest)), EstimatePromptTokens(firstRequest), len([]rune(continuedRequest)), len([]byte(continuedRequest)), EstimatePromptTokens(continuedRequest))
	firstPayload := decodeObject([]byte(firstRequest))
	firstMessages := arrayValue(firstPayload["messages"])
	if len(firstMessages) < 2 {
		t.Fatal("captured request omitted System or Runtime Context")
	}
	systemText := stringValue(mapValue(firstMessages[0])["content"])
	personaStart := strings.Index(systemText, "# 人格设定")
	if personaStart < 0 {
		t.Fatal("compiled persona heading missing from System")
	}
	parts := map[string]any{
		"protocol": systemText[:personaStart], "portrait_and_roster": systemText[personaStart:],
		"dynamic_runtime_context": firstMessages[1], "current_input": firstMessages[2:],
		"tools": firstPayload["tools"], "response_schema": firstPayload["response_format"],
	}
	if contextText := stringValue(mapValue(firstMessages[1])["content"]); strings.Contains(contextText, "recent_turns:") || strings.Contains(contextText, "retrieved_memories:") {
		t.Fatal("budget fixture unexpectedly contains history or memories; measure separately")
	}
	parts["history_and_memory"] = ""
	continuedPayload := decodeObject([]byte(continuedRequest))
	for _, raw := range arrayValue(continuedPayload["messages"]) {
		if stringValue(mapValue(raw)["role"]) == "tool" {
			parts["query_result"] = raw
			break
		}
	}
	for _, label := range []string{"protocol", "portrait_and_roster", "dynamic_runtime_context", "history_and_memory", "current_input", "tools", "response_schema", "query_result"} {
		value := jsonString(parts[label])
		if label == "history_and_memory" {
			value = ""
		}
		t.Logf("controlled slot %s chars=%d bytes=%d estimated_tokens=%d", label, len([]rune(value)), len([]byte(value)), EstimatePromptTokens(value))
	}
	if !strings.Contains(stringValue(result.Completion.Structured["response_intent"]), secret) {
		t.Fatal("final model result did not use queried detail")
	}
	if result.Trace == nil {
		t.Fatal("native Tool trace missing")
	}
	invocations, results := result.Trace.Snapshot()
	if len(invocations) != 1 || len(results) != 1 || invocations[0].CapabilityName != personaDetailCapabilityName || results[0].Status != "completed" {
		t.Fatalf("native query trace invocations=%#v results=%#v", invocations, results)
	}
}

func TestWorkingPersonaBackfillPreviewApplyAndReplay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	app := &App{DB: repository}
	ownerID, fluctlightID := "portrait-backfill-owner", "portrait-backfill-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateFluctlight(ctx, ownerID, fluctlightID, "摇光", "blank_slate", "", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	var activeBefore string
	var stateBefore int
	if err := repository.Pool().QueryRow(ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&activeBefore); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `DELETE FROM public.fluctlight_working_personas WHERE fluctlight_id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := app.VerifyWorkingPersonasReady(ctx); err == nil {
		t.Fatal("runtime gate accepted missing portrait")
	}
	preview, err := app.BackfillWorkingPersonas(ctx, ownerID, []string{fluctlightID}, false, false)
	if err != nil || len(preview) != 1 || preview[0].Status != "would_compile" {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
	var count int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_working_personas WHERE fluctlight_id=$1`, fluctlightID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("preview wrote rows count=%d err=%v", count, err)
	}
	result, err := app.BackfillWorkingPersonas(ctx, ownerID, []string{fluctlightID}, false, true)
	if err != nil || len(result) != 1 || result[0].Status != "compiled" {
		t.Fatalf("apply=%#v err=%v", result, err)
	}
	replay, err := app.BackfillWorkingPersonas(ctx, ownerID, []string{fluctlightID}, false, true)
	if err != nil || len(replay) != 1 || replay[0].Status != "skipped" {
		t.Fatalf("replay=%#v err=%v", replay, err)
	}
	if err := app.VerifyWorkingPersonasReady(ctx); err != nil {
		t.Fatalf("runtime gate rejected prepared portrait: %v", err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('working_persona_budget','{"max_runes":1024}') ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json`); err != nil {
		t.Fatal(err)
	}
	changedBudget, err := app.BackfillWorkingPersonas(ctx, ownerID, []string{fluctlightID}, false, false)
	if err != nil || len(changedBudget) != 1 || changedBudget[0].Status != "would_compile" {
		t.Fatalf("budget change did not invalidate portrait: %#v err=%v", changedBudget, err)
	}
	if err := app.VerifyWorkingPersonasReady(ctx); err == nil {
		t.Fatal("runtime gate accepted outdated budget")
	}
	updated, err := app.BackfillWorkingPersonas(ctx, ownerID, []string{fluctlightID}, false, true)
	if err != nil || len(updated) != 1 || updated[0].Status != "compiled" {
		t.Fatalf("budget recompilation failed: %#v err=%v", updated, err)
	}
	if err := app.VerifyWorkingPersonasReady(ctx); err != nil {
		t.Fatalf("runtime gate rejected recompilation: %v", err)
	}
	var activeAfter string
	var stateAfter int
	if err := repository.Pool().QueryRow(ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&activeAfter); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateAfter); err != nil {
		t.Fatal(err)
	}
	if activeBefore != activeAfter || stateBefore != stateAfter {
		t.Fatalf("backfill changed runtime state active=%s→%s revision=%d→%d", activeBefore, activeAfter, stateBefore, stateAfter)
	}
}

func TestTakeoverDetailQueryUsesNewSpeakingProfile(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "portrait-takeover-owner", "portrait-takeover-fluctlight", "portrait-takeover-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	resource, err := repository.GetFluctlight(ctx, fluctlightID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	corePersona := cloneMap(resource.CorePersona)
	secret := "暮光独有的旧事-" + stableDigest(fluctlightID)[:12]
	for _, raw := range arrayValue(mapValue(corePersona["personality_system"])["profiles"]) {
		profile := mapValue(raw)
		if stringValue(profile["id"]) == "twilight" {
			profile["secrets"] = map[string]any{"past": secret}
		}
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, fluctlightID, jsonBytes(corePersona)); err != nil {
		t.Fatal(err)
	}
	step := 0
	var firstWire, secondWire, thirdWire string
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(payload map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			firstWire = jsonString(payload)
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("portrait-takeover", personaTakeoverCapabilityName, map[string]any{
				"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "spark", "target_profile_id": "twilight", "reason": "controlled handover",
			})}}
		case 2:
			secondWire = jsonString(payload)
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("portrait-twilight-detail", personaDetailCapabilityName, map[string]any{"operation": "read", "section_id": "profile.secrets"})}}
		case 3:
			thirdWire = jsonString(payload)
			final := nativePersonaFinal()
			final.Structured["response_intent"] = "暮光说出：" + secret
			return final
		default:
			return fakeProviderResult{Status: 500}
		}
	})
	app := newTestApp(t, repository, router)
	input, err := app.personaCompilationInputForProfile(ctx, fluctlightID, corePersona, resource.CurrentRevision, "twilight", true)
	if err != nil {
		t.Fatal(err)
	}
	source, err := personaCompilationSource(input)
	if err != nil {
		t.Fatal(err)
	}
	compiled := CompiledWorkingPersona{
		ProfileID: "twilight", SourceRevision: resource.CurrentRevision, SourceHash: stableDigest(jsonString(source)), OverlayRevision: input.OverlayRevision, RulesVersion: personaCompilationRulesVersion, BudgetRunes: input.TargetBudgetRunes,
		Facts:     []PersonaPortraitFact{{Category: "core_mechanisms", Text: takeoverChainTwilightMarker, SourceRefs: []string{"profile"}}},
		Omissions: []PersonaPortraitOmission{{SourceRef: "profile.secrets", Reason: "details available through persona.detail"}},
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return insertCompiledWorkingPersonasTx(ctx, tx, fluctlightID, []CompiledWorkingPersona{compiled})
	}); err != nil {
		t.Fatal(err)
	}
	_, err = app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "你根本没在听。请由暮光谈谈她自己的旧事。", "portrait-takeover-run", "portrait-takeover-run"))
	if err != nil {
		t.Fatal(err)
	}
	if step != 3 || strings.Contains(firstWire, secret) || strings.Contains(secondWire, secret) || !strings.Contains(secondWire, takeoverChainTwilightMarker) || !strings.Contains(thirdWire, secret) {
		t.Fatalf("profile scope/feedback mismatch: step=%d firstSecret=%v secondSecret=%v secondPortrait=%v thirdSecret=%v", step, strings.Contains(firstWire, secret), strings.Contains(secondWire, secret), strings.Contains(secondWire, takeoverChainTwilightMarker), strings.Contains(thirdWire, secret))
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("takeover changed persistent profile: %s", active)
	}
	results := nativePersonaTrace(t, ctx, repository, "portrait-takeover-run")
	if len(results) != 2 || stringValue(results[1]["capability_name"]) != personaDetailCapabilityName || stringValue(results[1]["acting_profile_id"]) != "twilight" {
		t.Fatalf("query did not run under B: %#v", results)
	}
}

func TestFoundationAcceptancePublishesMatchingPortraitOrKeepsOldVersion(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "portrait-update-owner", "portrait-update-fluctlight", "portrait-update-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	compileCalls := 0
	failCompile := false
	raceBudget := false
	router := newFakeProviderRouter().on("persona_compilation_response", func(payload map[string]any) fakeProviderResult {
		compileCalls++
		if failCompile {
			return fakeProviderResult{Status: 500}
		}
		if raceBudget {
			raceBudget = false
			if _, err := repository.Pool().Exec(ctx, `UPDATE public.runtime_settings SET value_json='{"max_runes":1024}' WHERE key='working_persona_budget'`); err != nil {
				t.Fatal(err)
			}
		}
		profileID := "spark"
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if stringValue(message["role"]) != "user" {
				continue
			}
			for _, line := range strings.Split(stringValue(message["content"]), "\n") {
				if strings.HasPrefix(line, "profile_id:") {
					profileID = strings.TrimSpace(strings.TrimPrefix(line, "profile_id:"))
					break
				}
			}
		}
		return fakeProviderResult{Structured: map[string]any{"profile_id": profileID, "facts": []any{
			map[string]any{"category": "identity", "text": "更新后的摇光", "source_refs": []any{"identity.name"}},
			map[string]any{"category": "core_mechanisms", "text": "保持本人机制", "source_refs": []any{"profile"}},
		}, "omissions": []any{}}}
	})
	app := newTestApp(t, repository, router)
	first, err := app.ProposeFoundation(ctx, ownerID, fluctlightID, map[string]any{"expected_revision": 0, "changes": map[string]any{"identity": map[string]any{"name": "更新后的摇光"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetFoundationDecisionExpected(ctx, ownerID, fluctlightID, stringValue(first["id"]), "accept", "正式更新", nil); err != nil {
		t.Fatal(err)
	}
	if compileCalls != 2 {
		t.Fatalf("expected one compile per profile, got %d", compileCalls)
	}
	resource, err := repository.GetFluctlight(ctx, fluctlightID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if resource.CurrentRevision != 1 {
		t.Fatalf("foundation revision=%d", resource.CurrentRevision)
	}
	for _, profileID := range []string{"spark", "twilight"} {
		input, err := app.personaCompilationInputForProfile(ctx, fluctlightID, resource.CorePersona, 1, profileID, true)
		if err != nil {
			t.Fatal(err)
		}
		source, err := personaCompilationSource(input)
		if err != nil {
			t.Fatal(err)
		}
		var revision int
		var hash string
		if err := repository.Pool().QueryRow(ctx, `SELECT source_revision,source_hash FROM public.fluctlight_working_personas WHERE fluctlight_id=$1 AND profile_id=$2`, fluctlightID, profileID).Scan(&revision, &hash); err != nil || revision != 1 || hash != stableDigest(jsonString(source)) {
			t.Fatalf("profile %s source mismatch revision=%d hash=%s err=%v", profileID, revision, hash, err)
		}
	}
	second, err := app.ProposeFoundation(ctx, ownerID, fluctlightID, map[string]any{"expected_revision": 1, "changes": map[string]any{"identity": map[string]any{"name": "未发布的更新"}}})
	if err != nil {
		t.Fatal(err)
	}
	failCompile = true
	if _, err := app.SetFoundationDecisionExpected(ctx, ownerID, fluctlightID, stringValue(second["id"]), "accept", "应失败", nil); err == nil {
		t.Fatal("failed compiler published new foundation")
	}
	resource, err = repository.GetFluctlight(ctx, fluctlightID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if resource.CurrentRevision != 1 || stringValue(resource.Identity["name"]) != "更新后的摇光" {
		t.Fatalf("failed compile changed foundation: revision=%d identity=%#v", resource.CurrentRevision, resource.Identity)
	}
	var portraitRevision int
	if err := repository.Pool().QueryRow(ctx, `SELECT source_revision FROM public.fluctlight_working_personas WHERE fluctlight_id=$1 AND profile_id='spark'`, fluctlightID).Scan(&portraitRevision); err != nil || portraitRevision != 1 {
		t.Fatalf("failed compile changed portrait revision=%d err=%v", portraitRevision, err)
	}
	failCompile = false
	raceBudget = true
	if _, err := app.SetFoundationDecisionExpected(ctx, ownerID, fluctlightID, stringValue(second["id"]), "accept", "预算竞态", nil); err == nil {
		t.Fatal("budget changed during compilation but foundation published")
	}
	resource, err = repository.GetFluctlight(ctx, fluctlightID, ownerID)
	if err != nil || resource.CurrentRevision != 1 {
		t.Fatalf("budget race changed source revision=%d err=%v", resource.CurrentRevision, err)
	}
}

func TestFormalMainRejectsMissingOrStalePortraitBeforeProvider(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "portrait-missing-owner", "portrait-missing-fluctlight", "portrait-missing-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	router := newFakeProviderRouter().otherwise(func(_ map[string]any) fakeProviderResult { return nativePersonaFinal() })
	app := newTestApp(t, repository, router)
	if _, err := repository.Pool().Exec(ctx, `DELETE FROM public.fluctlight_working_personas WHERE fluctlight_id=$1 AND profile_id='spark'`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	input := ConversationCognitionAgentInput{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, RunID: "portrait-missing-run", CurrentInput: "你好"}
	if _, err := app.RunConversationCognitionAgent(ctx, input); err == nil || !strings.Contains(err.Error(), "working_persona_missing") {
		t.Fatalf("missing portrait did not fail closed: %v", err)
	}
	seedLegacyTestWorkingPersonas(t, app)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET current_revision=current_revision+1 WHERE id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	input.RunID = "portrait-stale-run"
	if _, err := app.RunConversationCognitionAgent(ctx, input); err == nil || !strings.Contains(err.Error(), "working_persona_version_mismatch") {
		t.Fatalf("stale portrait did not fail closed: %v", err)
	}
	if len(router.requests) != 0 {
		t.Fatalf("provider received %d requests despite missing/stale portrait", len(router.requests))
	}
}

func TestPersonaCompilerUsesOneBoundedSemanticRepair(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "portrait-repair-owner", "portrait-repair-fluctlight", "portrait-repair-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	calls := 0
	router := newFakeProviderRouter().on("persona_compilation_response", func(_ map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			return fakeProviderResult{Structured: map[string]any{"portrait_text": " "}}
		}
		return fakeProviderResult{Structured: map[string]any{"portrait_text": "摇光喜欢咖啡，不喜欢甜咖啡。"}}
	})
	app := newTestApp(t, repository, router)
	source := map[string]any{"identity": map[string]any{"name": "摇光"}, "life_profile": map[string]any{"preferences": map[string]any{"drink": "喜欢咖啡，不喜欢甜咖啡"}}, "personality_system": map[string]any{"profiles": []any{map[string]any{"id": "spark"}}}}
	compiled, err := app.CompileWorkingPersona(ctx, PersonaCompilationInput{CorePersona: source, ProfileID: "spark"})
	if err != nil || calls != 2 || compiled.PortraitText != "摇光喜欢咖啡，不喜欢甜咖啡。" {
		t.Fatalf("bounded repair result=%#v calls=%d err=%v", compiled, calls, err)
	}
}

func TestPersonaDetailTraceSurvivesModelContinuationFailure(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "portrait-continuation-owner", "portrait-continuation-fluctlight", "portrait-continuation-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	requests := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(_ map[string]any) fakeProviderResult {
		requests++
		if requests == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("portrait-list-before-failure", personaDetailCapabilityName, map[string]any{"operation": "list"})}}
		}
		return fakeProviderResult{Status: 500}
	})
	app := newTestApp(t, repository, router)
	result, err := app.RunConversationCognitionAgent(ctx, ConversationCognitionAgentInput{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, RunID: "portrait-continuation-failure", CurrentInput: "说说你自己的背景"})
	if err == nil || requests != 2 || result.Trace == nil {
		t.Fatalf("continuation failure was not preserved: err=%v requests=%d trace=%#v", err, requests, result.Trace)
	}
	invocations, results := result.Trace.Snapshot()
	if len(invocations) != 1 || len(results) != 1 || invocations[0].CapabilityName != personaDetailCapabilityName || results[0].Status != "completed" {
		t.Fatalf("successful query was erased by later model failure: invocations=%#v results=%#v", invocations, results)
	}
}

func TestInitializationTextToStoredPortraitToFormalRequest(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "portrait-init-owner", "portrait-init-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('portrait-init-endpoint','openai_compatible','http://portrait-init.invalid','portrait-init-secret','ready',now())`); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"initialization", "cognitive_assessment"} {
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES($1,'portrait-init-endpoint','portrait-init-model','structured_output,tool_calling',8192,30,'{}')`, role); err != nil {
			t.Fatal(err)
		}
	}
	card := "摇光温暖而直接，喜欢咖啡但不喜欢甜咖啡，忙完后会考虑自己喜欢的事。"
	initialization := map[string]any{"schema_version": 2, "core_persona": map[string]any{
		"identity": map[string]any{"name": "摇光"}, "life_profile": map[string]any{
			"preferences": map[string]any{"drink": "喜欢咖啡，但不喜欢甜咖啡"},
			"appearance": map[string]any{"description": "起初留长发，穿白衬衫", "physical_features": map[string]any{"hair_length": "long"},
				"wardrobe_items": []any{map[string]any{"category": "shirt", "slot": "top", "description": "白衬衫", "ownership": "unknown", "available": true, "currently_worn": true}}},
		},
		"personality": map[string]any{"openness": 0.7}, "behavioral_policy": map[string]any{"response_style": "温暖而直接"},
	}, "developing_self": map[string]any{"claims": []any{}}, "initial_relationships": []any{}, "initial_goals": []any{}, "initial_intentions": []any{}, "extensions": map[string]any{}}
	var finalWire string
	router := newFakeProviderRouter().on("persona_compilation_response", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"portrait_text": "摇光温暖而直接，喜欢咖啡，但不喜欢甜咖啡。"}}
	}).on(workingPersonaMainTurnSchema, func(payload map[string]any) fakeProviderResult {
		finalWire = jsonString(payload)
		return nativePersonaFinal()
	}).on("virtual_activity_result", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "虚拟理发已完成", "hair_length": "short"}}
	}).otherwise(func(_ map[string]any) fakeProviderResult { return fakeProviderResult{Structured: initialization} })
	app := newTestApp(t, repository, router)
	analysis, err := app.AnalyzeDescription(ctx, ownerID, card)
	if err != nil {
		t.Fatal(err)
	}
	if got := stringValue(mapValue(mapValue(analysis["core_persona"])["life_profile"])["preferences"].(map[string]any)["drink"]); got != "喜欢咖啡，但不喜欢甜咖啡" {
		t.Fatalf("stable preference lost during text parsing: %q", got)
	}
	analysisID := stringValue(analysis["analysis_id"])
	delete(analysis, "analysis_id")
	delete(analysis, "correlation_id")
	if _, err := app.CreateFluctlight(ctx, ownerID, fluctlightID, "摇光", "llm_defined", analysisID, analysis, nil, nil); err != nil {
		t.Fatal(err)
	}
	var compiledRaw []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT compiled_json FROM public.fluctlight_working_personas WHERE fluctlight_id=$1 AND profile_id='default'`, fluctlightID).Scan(&compiledRaw); err != nil {
		t.Fatal(err)
	}
	if compiled := decodeObject(compiledRaw); stringValue(compiled["portrait_text"]) != "摇光温暖而直接，喜欢咖啡，但不喜欢甜咖啡。" || len(arrayValue(compiled["facts"])) != 0 {
		t.Fatalf("initialization did not store one text portrait: %#v", compiled)
	}
	var sourceID string
	if err := repository.Pool().QueryRow(ctx, `SELECT source_id FROM public.fluctlight_initialization_source_links WHERE fluctlight_id=$1`, fluctlightID).Scan(&sourceID); err != nil || sourceID != analysisID {
		t.Fatalf("source link id=%s err=%v", sourceID, err)
	}
	if _, err := app.RunConversationCognitionAgent(ctx, ConversationCognitionAgentInput{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, RunID: "portrait-init-formal", CurrentInput: "忙完后考虑做件自己喜欢的事"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(finalWire, "喜欢咖啡") || strings.Contains(finalWire, card) {
		t.Fatal("formal request lost compiled preference or leaked original card")
	}
	initialWire := finalWire
	start, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: lifeActivityStartCapabilityName, OperationID: "portrait-haircut-start",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, Surface: CapabilitySurfaceConversation,
		Arguments: jsonBytes(map[string]any{"kind": "haircut", "duration_minutes": 15, "desired_hair_length": "short", "reason": "明确决定剪发"})})
	if err != nil || start.Result.Status != "accepted" {
		t.Fatalf("haircut start=%#v err=%v", start, err)
	}
	activityID := stringValue(mapValue(start.Result.Output)["activity_id"])
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlight_life_activity_runs SET started_at=now()-interval '20 minutes',not_before=now()-interval '1 minute' WHERE id=$1`, activityID); err != nil {
		t.Fatal(err)
	}
	advanced, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: lifeActivityAdvanceCapabilityName, OperationID: "portrait-haircut-advance",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, Surface: CapabilitySurfaceConversation,
		Arguments: jsonBytes(map[string]any{"activity_id": activityID})})
	if err != nil || advanced.Result.Status != "completed" {
		t.Fatalf("haircut advance=%#v err=%v", advanced, err)
	}
	if _, err := app.RunConversationCognitionAgent(ctx, ConversationCognitionAgentInput{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, RunID: "portrait-after-haircut", CurrentInput: "你现在的头发怎么样？"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(finalWire, "value: short") || strings.Contains(finalWire, "value: long") || strings.Contains(finalWire, "起初留长发") || !strings.Contains(finalWire, "白衬衫") || !strings.Contains(initialWire, "value: long") {
		t.Fatalf("final Provider request did not replace current appearance: first=%s after=%s", initialWire, finalWire)
	}
	detail, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: personaDetailCapabilityName, OperationID: "portrait-current-appearance",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, Surface: CapabilitySurfaceConversation,
		Arguments: jsonBytes(map[string]any{"operation": "read", "section_id": "current_appearance"})})
	if err != nil || !strings.Contains(stringValue(mapValue(detail.Result.Output)["content"]), "short") || stringValue(mapValue(detail.Result.Output)["time_semantics"]) != "current" {
		t.Fatalf("current detail did not reflect haircut: receipt=%#v err=%v", detail, err)
	}
	history, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: personaDetailCapabilityName, OperationID: "portrait-historical-appearance",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, Surface: CapabilitySurfaceConversation,
		Arguments: jsonBytes(map[string]any{"operation": "history", "revision": 0, "section_id": "life_profile"})})
	if err != nil || !strings.Contains(stringValue(mapValue(history.Result.Output)["content"]), "long") || stringValue(mapValue(history.Result.Output)["time_semantics"]) != "historical_foundation" {
		t.Fatalf("historical detail was lost or mislabeled: receipt=%#v err=%v", history, err)
	}
}
