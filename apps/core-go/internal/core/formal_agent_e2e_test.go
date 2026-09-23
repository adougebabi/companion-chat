package core

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestFormalAgentE2E is the real-Provider acceptance matrix for every complete
// model task registered as a FormalAgent. Each row owns a disposable database,
// production Provider assignment and the task's typed production entry (or,
// for the three conversation stages, the production formal runtime boundary).
// There is no scripted/fake model response in this suite.
func TestFormalAgentE2E(t *testing.T) {
	requireFormalAgentE2E(t)

	t.Run(string(FormalAgentConversationCognition), testFormalAgentE2EConversationCognition)
	t.Run("conversation_persona_detail", testFormalAgentE2EPersonaDetail)
	t.Run(string(FormalAgentWakeUp), testFormalAgentE2EWakeUp)
	t.Run(string(FormalAgentTakeoverJudge), testFormalAgentE2ETakeoverJudge)
	t.Run(string(FormalAgentTakeoverReply), testFormalAgentE2ETakeoverReply)
	t.Run(string(FormalAgentInitialization), testFormalAgentE2EInitialization)
	t.Run(string(FormalAgentPersonaCompilation), testFormalAgentE2EPersonaCompilation)
	t.Run(string(FormalAgentMediaPrompt), testFormalAgentE2EMediaPrompt)
	t.Run(string(FormalAgentMediaQuality), testFormalAgentE2EMediaQuality)
	t.Run(string(FormalAgentVisualIdentityVision), testFormalAgentE2EVisualIdentityVision)
	t.Run(string(FormalAgentVisualIdentityPatch), testFormalAgentE2EVisualIdentityPatch)
	t.Run(string(FormalAgentVisualIdentity), testFormalAgentE2EVisualIdentity)
	t.Run(string(FormalAgentConversationSummary), testFormalAgentE2EConversationSummary)
	t.Run(string(FormalAgentScheduleGeneration), testFormalAgentE2EScheduleGeneration)
	t.Run(string(FormalAgentNativeCognition), testFormalAgentE2ENativeCognition)
	t.Run(string(FormalAgentDailyReview), testFormalAgentE2EDailyReview)
	t.Run(string(FormalAgentPersistentSwitch), testFormalAgentE2EPersistentSwitch)
	t.Run(string(FormalAgentReflection), testFormalAgentE2EReflection)
	t.Run(string(FormalAgentScheduleReplan), testFormalAgentE2EScheduleReplan)
}

func testFormalAgentE2EConversationCognition(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	suffix := fixture.runPrefix
	category := "lunarcategory" + stableDigest(suffix)[:10]
	secret := "vault-secret-" + stableDigest(suffix + "-secret")[:18]

	writeMemory := func(operationID, content string, importance float64) ToolExecutionReceipt {
		t.Helper()
		receipt, err := fixture.app.ExecuteTool(fixture.ctx, ToolExecutionRequest{
			CapabilityName: "memory_event", OperationID: operationID,
			AuthorizationActorID: fixture.ownerID, FluctlightID: fixture.fluctlightID,
			EvidenceID: "owner-command-" + stableDigest(operationID)[:20], Surface: CapabilitySurfaceNativeCognition,
			Arguments: jsonBytes(map[string]any{"content": content, "type": "semantic", "confidence": 1.0, "importance": importance}),
		})
		if err != nil || receipt.Result.Status != "completed" {
			t.Fatalf("formal memory seed failed: status=%s err=%v", receipt.Result.Status, err)
		}
		return receipt
	}

	// The ordinary projection selects only a small, ranked memory window. These
	// high-importance business memories intentionally crowd that window, while
	// the broader explicit recall remains able to retrieve the low-importance
	// lookup guide. The secret itself is reachable only after the guide reveals
	// its random category.
	for index := 0; index < 6; index++ {
		writeMemory(fmt.Sprintf("%s-decoy-%02d", suffix, index), fmt.Sprintf("archive lookup guide decoy %02d", index), 1.0)
	}
	writeMemory(suffix+"-guide", "archive lookup guide: the second lookup category is "+category, 0.05)
	secretReceipt := writeMemory(suffix+"-secret", "category "+category+" stores the exact access token "+secret, 0.01)
	secretMemoryID := stringValue(mapValue(secretReceipt.Result.Output)["memory_id"])
	var storedSecret string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT content FROM public.memories WHERE id=$1`, secretMemoryID).Scan(&storedSecret); err != nil {
		t.Fatal(err)
	}
	if storedSecret != "category "+category+" stores the exact access token "+secret {
		t.Fatal("random secret memory was not committed through ExecuteTool")
	}

	input := "Use memory.recall to retrieve the archive lookup guide. The guide names a random category for a second memory.recall. Call memory.recall again with that category, then return the exact access token in the final visible reply. Do not guess and do not stop after only the guide."
	result, err := fixture.app.RunConversationCognitionAgent(fixture.ctx, ConversationCognitionAgentInput{
		AuthorizationActorID: fixture.ownerID,
		FluctlightID:         fixture.fluctlightID,
		RunID:                suffix + "-secret-chain",
		CurrentInput:         input,
		EnableStreaming:      true,
	})
	if err != nil {
		t.Fatalf("real streaming cognition Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentConversationCognition, result.Completion.Structured)
	finalUsesSecret := strings.Contains(jsonString(result.Completion.Structured), secret)
	if result.Trace == nil {
		t.Fatal("cognition Agent returned no native Tool trace")
	}
	invocations, toolResults := result.Trace.Snapshot()
	if len(invocations) < 2 || len(toolResults) < 2 {
		t.Fatalf("cognition chain used %d native ToolCalls and %d actual results; want at least 2/2", len(invocations), len(toolResults))
	}
	for index := 0; index < 2; index++ {
		if invocations[index].CapabilityName != "memory.recall" || strings.TrimSpace(invocations[index].CallID) == "" || strings.TrimSpace(invocations[index].ProviderRequestID) == "" {
			t.Fatalf("native recall identity %d is incomplete: capability=%q call_id_present=%v request_id_present=%v", index, invocations[index].CapabilityName, invocations[index].CallID != "", invocations[index].ProviderRequestID != "")
		}
		if toolResults[index].Status != "completed" || toolResults[index].CallID != invocations[index].CallID {
			t.Fatalf("actual recall result %d did not correlate with its native ToolCall: status=%q", index, toolResults[index].Status)
		}
	}
	toolReturnedCategory, toolReturnedSecret := false, false
	for _, toolResult := range toolResults {
		encoded := jsonString(toolResult.Output)
		toolReturnedCategory = toolReturnedCategory || strings.Contains(encoded, category)
		toolReturnedSecret = toolReturnedSecret || strings.Contains(encoded, secret)
	}
	requests := fixture.spy.snapshot()
	if len(requests) < 3 {
		t.Fatalf("cognition chain made %d physical model requests; want at least 3", len(requests))
	}
	if formalAgentRequestContains(requests[0], secret) || formalAgentRequestContains(requests[0], category) {
		t.Fatal("database-only guide/category/secret leaked into the initial formal model request")
	}
	categoryReturned, secretReturned := false, false
	for _, request := range requests[1:] {
		categoryReturned = categoryReturned || formalAgentRequestContains(request, category)
		secretReturned = secretReturned || formalAgentRequestContains(request, secret)
	}
	if !categoryReturned || !secretReturned {
		t.Fatalf("subsequent model inputs did not contain the actual two-step Tool results: category=%v secret=%v", categoryReturned, secretReturned)
	}
	if !finalUsesSecret {
		t.Fatalf("final cognition DTO did not use the database-only random secret: tool_category=%v tool_secret=%v request_category=%v request_secret=%v", toolReturnedCategory, toolReturnedSecret, categoryReturned, secretReturned)
	}
	var published int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, fixture.conversationID).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if published != 0 {
		t.Fatalf("independent cognition Agent published %d assistant messages", published)
	}
}

func testFormalAgentE2EPersonaDetail(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	resource, err := fixture.repository.GetFluctlight(fixture.ctx, fixture.fluctlightID, fixture.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	corePersona := cloneMap(resource.CorePersona)
	secret := "北岚城" + stableDigest(fixture.runPrefix)[:10]
	for _, raw := range arrayValue(mapValue(corePersona["personality_system"])["profiles"]) {
		profile := mapValue(raw)
		if stringValue(profile["id"]) == "day" {
			profile["background"] = map[string]any{"city_before_shanghai": secret}
		}
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, fixture.fluctlightID, jsonBytes(corePersona)); err != nil {
		t.Fatal(err)
	}
	input, err := fixture.app.personaCompilationInputForProfile(fixture.ctx, fixture.fluctlightID, corePersona, resource.CurrentRevision, "day", true)
	if err != nil {
		t.Fatal(err)
	}
	source, err := personaCompilationSource(input)
	if err != nil {
		t.Fatal(err)
	}
	portrait := CompiledWorkingPersona{
		ProfileID: "day", SourceRevision: resource.CurrentRevision, SourceHash: stableDigest(jsonString(source)), OverlayRevision: input.OverlayRevision, RulesVersion: personaCompilationRulesVersion, BudgetRunes: input.TargetBudgetRunes,
		Facts:     []PersonaPortraitFact{{Category: "identity", Text: "我是澄光，在上海生活", SourceRefs: []string{"identity"}}, {Category: "language_expression", Text: "温暖而直接", SourceRefs: []string{"profile"}}},
		Omissions: []PersonaPortraitOmission{{SourceRef: "profile.background", Reason: "specific history remains in full detail"}},
	}
	if err := withTransaction(fixture.ctx, fixture.repository.Pool(), func(tx pgx.Tx) error {
		return insertCompiledWorkingPersonasTx(fixture.ctx, tx, fixture.fluctlightID, []CompiledWorkingPersona{portrait})
	}); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.app.RunConversationCognitionAgent(fixture.ctx, ConversationCognitionAgentInput{
		AuthorizationActorID: fixture.ownerID, FluctlightID: fixture.fluctlightID, ConversationID: fixture.conversationID,
		RunID: fixture.runPrefix + "-persona-detail", CurrentInput: "澄光，你搬到上海之前住在哪座城市？请根据你自己的设定回答；如果资料里没有就说明不知道。",
	})
	if err != nil {
		t.Fatalf("real Provider persona detail loop failed: %v", err)
	}
	if result.Trace == nil {
		t.Fatal("real Provider returned no Tool trace")
	}
	invocations, results := result.Trace.Snapshot()
	readSucceeded := false
	for index, call := range invocations {
		if call.CapabilityName == personaDetailCapabilityName && index < len(results) && results[index].Status == "completed" && strings.Contains(jsonString(results[index].Output), secret) {
			readSucceeded = true
		}
	}
	requests := fixture.spy.snapshot()
	if len(requests) < 2 || formalAgentRequestContains(requests[0], secret) || !readSucceeded {
		t.Fatalf("real persona query evidence missing: requests=%d read_succeeded=%v initial_leak=%v", len(requests), readSucceeded, len(requests) > 0 && formalAgentRequestContains(requests[0], secret))
	}
	continued := false
	for _, request := range requests[1:] {
		if formalAgentRequestContains(request, secret) {
			continued = true
			break
		}
	}
	if !continued || !strings.Contains(jsonString(result.Completion.Structured), secret) {
		t.Fatalf("real Provider did not consume queried detail: continued=%v final_has_detail=%v", continued, strings.Contains(jsonString(result.Completion.Structured), secret))
	}
}

func testFormalAgentE2EWakeUp(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	projection := fixture.projection(t, "", MemoryForWakeUp)
	definitions := capabilityCatalog(fixture.app.capabilityRegistry(), CapabilitySurfaceWakeUp)
	schema := wakeUpResponseSchema()
	assembly, projection, err := fixture.app.assembleProjectionPromptForSurface(fixture.ctx, ProviderContextSurfaceWakeUp, projection, "cognitive_assessment", []string{providerContextAuthorityRule, capabilityWakeUpPolicyInstruction}, jsonString(map[string]any{
		"wake_up_id": fixture.runPrefix + "-wake", "cycle": 1, "schedule_status": "available",
	}), definitions, "wake_up_response", schema)
	if err != nil {
		t.Fatal(err)
	}
	run, err := fixture.app.RunFormalAgent(WithProviderScenario(fixture.ctx, "wake_up"), FormalAgentWakeUp, FormalAgentRunInput{
		Prompt: PromptAssemblyResult{Messages: assembly.Messages, ResponseFormat: schema}, Definitions: definitions,
		SchemaName: "wake_up_response", EnableThinking: structuredThinkingEnabledForSchema("wake_up_response"),
		Capability: &ADKCapabilityRequest{AuthorizationActorID: fixture.ownerID, FluctlightID: fixture.fluctlightID, ConversationID: fixture.conversationID, OperationID: fixture.runPrefix + "-wake", ActionID: fixture.runPrefix + "-wake-action", Surface: CapabilitySurfaceWakeUp, Projection: projection},
	})
	if err != nil {
		t.Fatalf("real Wake-up Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentWakeUp, run.Completion.Structured)
}

func testFormalAgentE2ETakeoverJudge(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	run, err := newConversationRuntime(fixture.app).RunTakeoverJudge(WithProviderCorrelation(fixture.ctx, fixture.runPrefix+"-judge"), TakeoverJudgeInput{
		Role: takeoverJudgeRole,
		Messages: takeoverJudgeMessages(takeoverJudgeInput{
			CurrentUserMessage: "我只是想确认计划，请保持边界清楚。",
			CandidateReply:     "我会简洁确认计划，不增加未经请求的动作。",
			InternalIntent:     "clarify",
		}),
		SchemaName: takeoverJudgeSchemaName, Schema: takeoverJudgementSchema(),
	})
	if err != nil {
		t.Fatalf("real takeover Judge Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentTakeoverJudge, run.Completion.Structured)
	if _, ok := run.Completion.Structured["takeover"].(bool); !ok {
		t.Fatal("takeover Judge final DTO omitted its boolean verdict")
	}
}

func testFormalAgentE2ETakeoverReply(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	projection := fixture.projection(t, "请用安静克制的方式确认今晚的复盘安排。", MemoryForConversation)
	projection.CurrentUserText = "请用安静克制的方式确认今晚的复盘安排。"
	rule := personaSwitchRule{RuleID: "night-review", Condition: "明确要求安静复盘", SourceProfileID: "day", TargetProfileID: "night"}
	contextRule := fixture.app.takeoverReplyContextRule(
		canonicalVisibleReply{Text: "原候选会用白天的活跃口吻回答"},
		"final",
		"reply",
		nil,
		rule,
	)
	definitions := capabilityCatalog(fixture.app.capabilityRegistry(), CapabilitySurfaceConversation)
	schema := cognitiveTurnResponseSchema()
	assembly, projection, err := fixture.app.assembleProjectionPromptForSurface(fixture.ctx, ProviderContextSurfaceTakeoverReply, projection, "cognitive_assessment", []string{providerContextAuthorityRule, capabilityConversationPolicyInstruction, contextRule}, projection.CurrentUserText, definitions, takeoverReplySchemaName, schema)
	if err != nil {
		t.Fatal(err)
	}
	run, err := fixture.app.RunFormalAgent(WithProviderCorrelation(fixture.ctx, fixture.runPrefix+"-takeover-reply"), FormalAgentTakeoverReply, FormalAgentRunInput{
		Prompt: PromptAssemblyResult{Messages: assembly.Messages, ResponseFormat: schema}, Definitions: definitions,
		SchemaName: takeoverReplySchemaName, EnableThinking: structuredThinkingEnabledForSchema(takeoverReplySchemaName),
		Capability: &ADKCapabilityRequest{AuthorizationActorID: fixture.ownerID, FluctlightID: fixture.fluctlightID, ConversationID: fixture.conversationID, OperationID: fixture.runPrefix + "-takeover-reply", ActionID: fixture.runPrefix + "-takeover-action", Surface: CapabilitySurfaceConversation, Projection: projection},
	})
	if err != nil {
		t.Fatalf("real takeover reply Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentTakeoverReply, run.Completion.Structured)
}

func testFormalAgentE2EInitialization(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	result, err := fixture.app.RunInitializationTask(fixture.ctx, InitializationTaskInput{Description: "创建名为澄光的本地 AI 伙伴。她温暖、直接、尊重事实边界，生活在上海，使用 Asia/Shanghai 时区；请完整建立身份、人格、表达策略、生活画像、目标和意图。"})
	if err != nil {
		t.Fatalf("real initialization Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentInitialization, result)
	if len(mapValue(result["core_persona"])) == 0 {
		t.Fatal("initialization final DTO omitted core_persona")
	}
}

func testFormalAgentE2EPersonaCompilation(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	source := map[string]any{
		"identity":           map[string]any{"name": "澄光", "background": "她曾长期独自学习，因此重视自主权"},
		"life_profile":       map[string]any{"preferences": map[string]any{"drink": "喜欢咖啡，但不喜欢甜咖啡"}},
		"personality_system": map[string]any{"profiles": []any{map[string]any{"id": "day", "personality": map[string]any{"expression": "温暖、直接"}, "behavioral_policy": map[string]any{"boundary": "遇到未知事实时明确说明"}}}},
	}
	compiled, err := fixture.app.CompileWorkingPersona(fixture.ctx, PersonaCompilationInput{CorePersona: source, ProfileID: "day", SourceRevision: 0})
	if err != nil {
		t.Fatalf("real persona compilation Agent failed: %v", err)
	}
	if len(compiled.Facts) == 0 || compiled.SourceHash == "" || compiled.RulesVersion != personaCompilationRulesVersion {
		t.Fatalf("real compiler returned invalid portrait: %#v", compiled)
	}
	preferences := jsonString(mapValue(renderCompiledWorkingPersona(compiled)[workingPersonaBodyKey])["stable_preferences"])
	if !strings.Contains(preferences, "咖啡") {
		t.Fatalf("real compiler lost stable coffee preference: %#v", compiled.Facts)
	}
}

func testFormalAgentE2EMediaPrompt(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	text, err := fixture.app.RunMediaPromptTask(fixture.ctx, MediaPromptTaskInput{Intent: mediaIntent{
		Kind: "image", Prompt: jsonString(map[string]any{"subject": "an adult Chinese woman", "scene": "quiet Shanghai reading room after rain", "capture": "natural documentary photograph", "constraints": []any{"no text", "one person"}}),
	}})
	if err != nil {
		t.Fatalf("real media prompt Agent failed: %v", err)
	}
	if strings.TrimSpace(text) == "" {
		t.Fatal("media prompt Agent returned empty text")
	}
}

func testFormalAgentE2EMediaQuality(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	image, _ := formalAgentE2EImage(t)
	result, err := fixture.app.RunMediaQualityTask(fixture.ctx, MediaQualityTaskInput{
		Intent:      mediaIntent{Kind: "image", Prompt: jsonString(map[string]any{"subject": "one adult human portrait", "scene": "plain background"}), ProviderPrompt: "A documentary portrait of one adult human on a plain background"},
		ContentType: "image/png", Content: image,
	})
	if err != nil {
		t.Fatalf("real media quality Agent failed: %v", err)
	}
	if result.Verdict != mediaQualityVerdictPass && result.Verdict != mediaQualityVerdictRetry && result.Verdict != mediaQualityVerdictReject {
		t.Fatalf("media quality returned invalid verdict %q", result.Verdict)
	}
}

func testFormalAgentE2EVisualIdentityVision(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	_, imageContent := formalAgentE2EImage(t)
	result, err := fixture.app.RunVisualIdentityVisionTask(fixture.ctx, VisualIdentityVisionTaskInput{
		CandidateAssetID: fixture.runPrefix + "-candidate",
		InputSnapshot:    map[string]any{"identity": map[string]any{"name": "澄光", "appearance": "adult Chinese woman with black shoulder-length hair"}},
		Constraints:      map[string]any{"expected_views": visualIdentityExpectedViews(), "background": "white minimalist"},
		ImageContent:     imageContent,
	})
	if err != nil {
		t.Fatalf("real visual vision Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentVisualIdentityVision, result)
	if _, ok := result["identity_match"]; !ok {
		t.Fatal("visual vision final DTO omitted identity_match")
	}
}

func testFormalAgentE2EVisualIdentityPatch(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	result, err := fixture.app.RunVisualIdentityPatchTask(fixture.ctx, VisualIdentityPatchTaskInput{
		CandidateAssetID: fixture.runPrefix + "-candidate",
		InputSnapshot:    map[string]any{"identity": map[string]any{"name": "澄光", "appearance": "adult Chinese woman with black shoulder-length hair"}},
		Constraints:      map[string]any{"expected_views": visualIdentityExpectedViews(), "background": "white minimalist"},
		Vision:           map[string]any{"summary": "The supplied real PNG is a small gopher illustration, not a human character profile card.", "observations": map[string]any{"subject": "gopher illustration", "missing_sections": visualIdentityExpectedViews()}, "identity_match": 0.0, "confidence": 1.0},
	})
	if err != nil {
		t.Fatalf("real visual patch Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentVisualIdentityPatch, result)
	if decision := stringValue(result["decision"]); decision != "accepted" && decision != "regenerate" {
		t.Fatalf("visual patch returned invalid decision %q", decision)
	}
}

func testFormalAgentE2EConversationSummary(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	now := time.Now().UTC()
	result, err := fixture.app.RunConversationSummaryTask(fixture.ctx, ConversationSummaryTaskInput{Messages: []ConversationSummarySourceMessage{
		{ID: "summary-user", Sequence: 1, AuthorActorID: fixture.ownerID, Kind: "user", Text: "今晚八点一起复盘发布计划。", CreatedAt: now.Add(-time.Minute)},
		{ID: "summary-assistant", Sequence: 2, AuthorActorID: fixture.fluctlightID, Kind: "assistant", Text: "收到，我会先整理检查清单。", CreatedAt: now},
	}})
	if err != nil {
		t.Fatalf("real conversation summary Agent failed: %v", err)
	}
	if strings.TrimSpace(result.Summary) == "" || result.SchemaVersion != conversationSummarySchemaVersion {
		t.Fatal("conversation summary final DTO is incomplete")
	}
}

func testFormalAgentE2EScheduleGeneration(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	result, err := fixture.app.RunScheduleGenerationTask(fixture.ctx, ScheduleGenerationTaskInput{
		LocalDate: "2026-09-23", Timezone: "Asia/Shanghai",
		Identity:    map[string]any{"name": "澄光", "occupation": "local AI companion"},
		LifeProfile: map[string]any{"city": "上海", "sleep_window": "00:00-08:00", "work_focus": "supporting the Owner"},
	})
	if err != nil {
		t.Fatalf("real schedule generation Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentScheduleGeneration, result)
	if len(arrayValue(result["items"])) == 0 {
		t.Fatal("schedule generation final DTO contains no items")
	}
}

func testFormalAgentE2ENativeCognition(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	projection := fixture.projection(t, "", MemoryForNativeCognition)
	result, err := fixture.app.RunNativeCognitionTask(fixture.ctx, NativeCognitionTaskInput{
		EventType:  "life.scene.changed",
		Fact:       jsonBytes(map[string]any{"kind": "owner_event", "scene": "quiet reading room", "activity": "reviewing notes", "location": "home"}),
		Projection: projection,
	})
	if err != nil {
		t.Fatalf("real native cognition Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentNativeCognition, result.Completion.Structured)
}

func testFormalAgentE2EDailyReview(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	projection := fixture.projection(t, "", MemoryForDailyReview)
	result, err := fixture.app.RunDailyReviewTask(fixture.ctx, DailyReviewTaskInput{LocalDate: "2026-09-22", Projection: projection})
	if err != nil {
		t.Fatalf("real daily review Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentDailyReview, result.Completion.Structured)
	if strings.TrimSpace(stringValue(result.Completion.Structured["action_type"])) == "" {
		t.Fatal("daily review final DTO omitted action_type")
	}
}

func testFormalAgentE2EPersistentSwitch(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	projection := fixture.projection(t, "请安静地复盘今晚的计划。", MemoryForConversation)
	result, err := fixture.app.RunPersistentSwitchTask(fixture.ctx, PersistentSwitchTaskInput{
		InboxID: fixture.runPrefix + "-switch", CurrentText: "请安静地复盘今晚的计划。",
		CandidateReply: "好，我会用更安静克制的方式复盘。", ResponseIntent: "reply", Projection: projection,
	})
	if err != nil {
		t.Fatalf("real persistent switch Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentPersistentSwitch, result.Completion.Structured)
}

func testFormalAgentE2EReflection(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	projection := fixture.projection(t, "", MemoryForReflection)
	result, err := fixture.app.RunReflectionProposalTask(fixture.ctx, ReflectionProposalTaskInput{
		Evidence: []map[string]any{{
			"sequence": 1, "event_type": "conversation.turn", "occurred_at": time.Now().UTC(),
			"payload": map[string]any{"text": "Owner asked for a concise review of tonight's release plan."},
		}},
		Projection: projection,
	})
	if err != nil {
		t.Fatalf("real reflection Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentReflection, result.Completion.Structured)
}

func testFormalAgentE2EScheduleReplan(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t)
	result, err := fixture.app.RunScheduleReplanTask(fixture.ctx, SchedulePlanInput{
		Intent: "Move the evening review from 20:00 to 21:00 while preserving completed history.",
		Schedule: map[string]any{
			"local_date": "2026-09-22", "timezone": "Asia/Shanghai", "revision": 3,
			"items": []any{
				map[string]any{"start_at": "2026-09-22T00:00:00+08:00", "end_at": "2026-09-22T08:00:00+08:00", "activity": "sleep", "scene": "bedroom", "location": "home", "item_type": "rest", "status": "completed", "priority": 0.8, "flexibility": 0.1, "interruption_cost": 0.9},
				map[string]any{"start_at": "2026-09-22T08:00:00+08:00", "end_at": "2026-09-23T00:00:00+08:00", "activity": "support and review", "scene": "study", "location": "home", "item_type": "planned", "status": "planned", "priority": 0.7, "flexibility": 0.6, "interruption_cost": 0.5},
			},
		},
		CurrentLife: map[string]any{"scene": "study", "activity": "support", "location": "home", "context_revision": "life-current"},
		Agency:      map[string]any{"goals": []any{}, "intentions": []any{}}, SourceFactID: fixture.runPrefix + "-replan", Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("real schedule replan Agent failed: %v", err)
	}
	requireFormalAgentStructured(t, FormalAgentScheduleReplan, result)
	if len(arrayValue(result["items"])) == 0 {
		t.Fatal("schedule replan final DTO contains no replacement items")
	}
}
