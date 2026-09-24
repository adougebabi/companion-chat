package core

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestImageCapabilityPrepareConsumesResolvedContext(t *testing.T) {
	capability := imageGenerateCapability{}
	invocation := CapabilityInvocation{CallID: "image-1", CapabilityName: "media.image.generate", Arguments: []byte(`{"intent":"portrait"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	resolved := CapabilityContext{
		Visual: &VisualIdentityContext{Data: map[string]any{"asset_id": "visual-1"}},
		Life:   &CurrentLifeContext{Data: map[string]any{"scene": "studio"}},
		Outfit: &AppearanceContext{Data: map[string]any{"hair": "long"}},
		State:  &CurrentStateContext{Data: map[string]any{"mood": map[string]any{"label": "calm"}}},
	}
	prepared, err := capability.Prepare(nil, invocation, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if string(prepared.Arguments) != string(invocation.Arguments) {
		t.Fatalf("provider arguments changed during prepare: before=%s after=%s", invocation.Arguments, prepared.Arguments)
	}
	rawConcept, found, decodeErr := capabilityPreparedData(prepared, "media_concept")
	if decodeErr != nil || !found {
		t.Fatalf("prepared image payload missing: found=%v err=%v payload=%s", found, decodeErr, prepared.PreparedPayload)
	}
	concept := mapValue(rawConcept)
	binding := mapValue(concept["context_binding"])
	if stringValue(concept["intent"]) != "portrait" || stringValue(mapValue(binding["current_life"])["scene"]) != "studio" || stringValue(mapValue(binding["appearance"])["hair"]) != "long" {
		t.Fatalf("prepared image payload lost context: %#v", concept)
	}
}

func TestImageIntentAndCanonicalContextReachMediaPromptInput(t *testing.T) {
	capability := imageGenerateCapability{}
	invocation := CapabilityInvocation{CallID: "image-2", CapabilityName: "media.image.generate", Arguments: []byte(`{"intent":"在窗边读书"}`), SourceFactID: "fact-2", ProviderRequestID: "provider-2"}
	resolved := CapabilityContext{
		Visual: &VisualIdentityContext{Data: map[string]any{"status": "active", "renderer_constraints": map[string]any{"chest_cup": "B"}}},
		Life:   &CurrentLifeContext{Data: map[string]any{"scene": "窗边", "activity": "阅读", "location": "客厅"}},
		Outfit: &AppearanceContext{Data: map[string]any{"outfit": "针织衫"}},
		State:  &CurrentStateContext{Data: map[string]any{"mood": map[string]any{"label": "平静", "intensity": 0.4}, "pad": map[string]any{"pleasure": 0.3, "arousal": 0.1, "dominance": 0.2}}},
	}
	prepared, err := capability.Prepare(context.Background(), invocation, resolved)
	if err != nil {
		t.Fatal(err)
	}
	rawConcept, found, err := capabilityPreparedData(prepared, "media_concept")
	if err != nil || !found {
		t.Fatalf("prepared concept missing: found=%v err=%v", found, err)
	}
	promptInput := mediaPromptInput(mediaIntent{Prompt: jsonString(rawConcept)})
	var providerConcept map[string]any
	if err := json.Unmarshal([]byte(promptInput), &providerConcept); err != nil {
		t.Fatalf("media prompt input is not JSON: %v: %s", err, promptInput)
	}
	binding := mapValue(providerConcept["context_binding"])
	if stringValue(providerConcept["intent"]) != "在窗边读书" ||
		stringValue(mapValue(binding["current_life"])["scene"]) != "窗边" ||
		stringValue(mapValue(mapValue(binding["current_state"])["mood"])["label"]) != "平静" ||
		stringValue(mapValue(binding["appearance"])["outfit"]) != "针织衫" ||
		stringValue(mapValue(mapValue(binding["visual_identity"])["renderer_constraints"])["chest_cup"]) != "B" {
		t.Fatalf("canonical media intent/context was lost: %s", promptInput)
	}
	for _, legacy := range []string{"life_context", "inner_state"} {
		if _, found := binding[legacy]; found {
			t.Fatalf("legacy media context key %q remains: %s", legacy, promptInput)
		}
	}
}

func TestSceneMediaPromptUsesCurrentAppearanceWithoutHistoricalVisualSnapshot(t *testing.T) {
	concept := map[string]any{
		"intent": "拍一张现在的照片",
		"visual_identity": map[string]any{"status": "active", "identity_snapshot": map[string]any{
			"identity":     map[string]any{"name": "摇光", "appearance": "旧长发"},
			"life_profile": map[string]any{"appearance": map[string]any{"description": "过去穿白衬衫"}},
		}},
		"context_binding": map[string]any{
			"visual_identity": map[string]any{"status": "active", "reference_asset_id": "historical-reference"},
			"appearance": map[string]any{"body_revision": 2, "wardrobe_revision": 3,
				"body_fields": map[string]any{"hair_length": map[string]any{"status": "known", "value": "短发"}},
				"worn_items":  []any{map[string]any{"id": "item-dark", "description": "深色上衣", "slot": "top"}},
			},
		},
	}
	prompt := compactMediaConceptForProvider(jsonString(concept))
	if os.Getenv("YAOGUANG_CAPTURE_WIRE") == "1" {
		t.Logf("WIRE_MEDIA_CONCEPT=%s", prompt)
	}
	if !strings.Contains(prompt, "短发") || !strings.Contains(prompt, "深色上衣") || strings.Contains(prompt, "旧长发") || strings.Contains(prompt, "过去穿白衬衫") || strings.Contains(prompt, "historical-reference") {
		t.Fatalf("scene media prompt mixed historical/current appearance: %s", prompt)
	}
	instruction := mediaPromptSystemInstruction(mediaIntent{Prompt: jsonString(concept)})
	if !strings.Contains(instruction, "当前发长") || !strings.Contains(instruction, "历史设定") {
		t.Fatalf("media prompt did not explain current binding authority: %s", instruction)
	}
}

func TestCompletedMediaIntentMarksCapturedAppearanceStaleWhenBodyChanges(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	appearance, _, _, err := fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	intentID := "media-capture-stale-" + fixture.suffix
	concept := map[string]any{"intent": "现在的样子", "context_binding": map[string]any{"appearance": appearance}}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES($1,$2,'image','image/png',$3,$4,$5,'pending')`, intentID, fixture.fluctlightID, jsonString(concept), "provider-"+intentID, "workflow-"+intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(appearanceStyleCapabilityName, "media-change-hair-style", map[string]any{"operation": "set", "style": "扎起头发", "reason": "拍摄任务尚未完成时换发型"})); err != nil {
		t.Fatal(err)
	}
	if err := fixture.app.markMediaIntentCompleted(fixture.ctx, intentID, "asset-historical-photo"); err != nil {
		t.Fatal(err)
	}
	var stale bool
	var capturedBody, capturedWardrobe int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT context_stale_at_completion,capture_body_revision,capture_wardrobe_revision FROM public.media_intents WHERE id=$1`, intentID).Scan(&stale, &capturedBody, &capturedWardrobe); err != nil || !stale || capturedBody != 0 || capturedWardrobe != 0 {
		t.Fatalf("old image was marked current: stale=%v body=%d wardrobe=%d err=%v", stale, capturedBody, capturedWardrobe, err)
	}
}

func TestWithContextAuthorityInstructionKeepsUserMessageLast(t *testing.T) {
	messages := withContextAuthorityInstruction([]map[string]any{
		{"role": "system", "content": "decide"},
		{"role": "user", "content": "current request"},
	})
	if len(messages) != 2 || stringValue(messages[0]["role"]) != "system" || stringValue(messages[1]["role"]) != "user" {
		t.Fatalf("messages = %#v", messages)
	}
	if !strings.Contains(stringValue(messages[0]["content"]), "context.current_state") || !strings.Contains(stringValue(messages[0]["content"]), "life_context.current_time") || !strings.Contains(stringValue(messages[0]["content"]), "decide") {
		t.Fatalf("authority instruction = %#v", messages[0])
	}
}

func TestWithChineseOutputInstructionExcludesMediaPromptRole(t *testing.T) {
	messages := []map[string]any{
		{"role": "system", "content": "规则"},
		{"role": "user", "content": "内容"},
	}
	localized := withChineseOutputInstruction("cognitive_assessment", messages)
	if len(localized) != 2 || stringValue(localized[0]["role"]) != "system" || stringValue(localized[1]["role"]) != "user" {
		t.Fatalf("localized messages = %#v", localized)
	}
	if !strings.Contains(stringValue(localized[0]["content"]), "自然语言内容使用中文") || !strings.Contains(stringValue(localized[0]["content"]), "规则") {
		t.Fatalf("language instruction = %#v", localized[0])
	}
	media := withChineseOutputInstruction("media_prompt", messages)
	if len(media) != len(messages) {
		t.Fatalf("media prompt messages were changed: %#v", media)
	}
}

func TestWithChineseOutputInstructionMovesLateSystemMessagesToFront(t *testing.T) {
	localized := withChineseOutputInstruction("cognitive_assessment", []map[string]any{
		{"role": "user", "content": "先出现的用户消息"},
		{"role": "system", "content": "迟到的系统规则"},
		{"role": "assistant", "content": "历史回复"},
	})
	if len(localized) != 3 || stringValue(localized[0]["role"]) != "system" || stringValue(localized[1]["role"]) != "user" || stringValue(localized[2]["role"]) != "assistant" {
		t.Fatalf("localized messages = %#v", localized)
	}
	if !strings.Contains(stringValue(localized[0]["content"]), "迟到的系统规则") {
		t.Fatalf("late system rule was lost: %#v", localized[0])
	}
}

func TestSystemInstructionMergesToExactlyOneSystemMessage(t *testing.T) {
	localized := withChineseOutputInstruction("cognitive_assessment", withContextAuthorityInstruction([]map[string]any{
		{"role": "system", "content": "operation rules"},
		{"role": "user", "content": "request"},
		{"role": "system", "content": "late rules"},
	}))
	systemCount := 0
	for _, message := range localized {
		if stringValue(message["role"]) == "system" {
			systemCount++
		}
	}
	if systemCount != 1 || stringValue(localized[0]["role"]) != "system" {
		t.Fatalf("system messages = %#v", localized)
	}
	for _, expected := range []string{"operation rules", "late rules", "context.current_state", "自然语言内容使用中文"} {
		if !strings.Contains(stringValue(localized[0]["content"]), expected) {
			t.Fatalf("merged system content missing %q: %s", expected, localized[0]["content"])
		}
	}
}
