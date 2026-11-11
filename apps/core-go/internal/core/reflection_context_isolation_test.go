package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestReflectionRequestKeepsEvidenceButExcludesAmbientWardrobe(t *testing.T) {
	const ambient = "无关当前红色外套"
	source := " \n" + strings.Repeat("甲", 8000) + "来源消息撤回蓝色围巾的约定 \n"
	projection := ContextProjection{CurrentState: map[string]any{"data": map[string]any{
		"inner_state":       map[string]any{"mood": map[string]any{"label": "平静"}},
		"life_context":      map[string]any{"scene": "书房"},
		"appearance":        map[string]any{"wearing_state": "known", "worn_items": []any{map[string]any{"description": ambient, "slot": "top"}}, "used_items": []any{map[string]any{"description": ambient}}, "body_fields": map[string]any{"hair_color": map[string]any{"status": "known", "value": "黑色"}}},
		"active_activities": []any{map[string]any{"kind": "virtual_shopping", "request": map[string]any{"description": ambient}}},
	}}}
	before := jsonString(projection)
	evidence := compactReflectionEvidenceV2([]map[string]any{{"sequence": 7, "event_type": "conversation.turn", "payload": map[string]any{"text": source}}})
	if len(evidence) != 1 || evidence[0]["observation"] != source {
		t.Fatal("reflection truncated or rewrote the source message")
	}
	working, err := ResolveWorkingMemory(workingMemoryInputFromProjectionForSurface(projection, ProviderContextSurfaceReflection, nil, nil), DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	assembly, err := AssemblePromptContext(PromptAssemblyInput{Surface: ProviderContextSurfaceReflection, Role: "reflection", CorePersona: map[string]any{"identity": map[string]any{"name": "角色"}}, OperationRules: []string{reflectionEvidenceAuthorityRule, reflectionV2Instruction}, WorkingMemory: working, CurrentInput: jsonString(map[string]any{"evidence": evidence}), ResponseFormat: reflectionProposalV2ProviderSchema(), Policy: DefaultPromptBudgetPolicy(0)})
	if err != nil {
		t.Fatal(err)
	}
	text := jsonString(assembly.Messages)
	if strings.Contains(text, ambient) || strings.Contains(text, "wardrobe.inspect") || !strings.Contains(text, "来源消息撤回蓝色围巾的约定") || !strings.Contains(text, "sequence:7") || !strings.Contains(text, "黑色") || !strings.Contains(text, "平静") {
		t.Fatal("reflection input mixed unrelated wardrobe with source evidence")
	}
	if !strings.Contains(jsonString(compactCognitionContextForSurface(projection, ProviderContextSurfaceConversationMain)), ambient) {
		t.Fatal("chat lost its authoritative wearing state")
	}
	if jsonString(projection) != before {
		t.Fatal("reflection projection changed the source state")
	}
	captured := make(chan map[string]any, 1)
	router := newFakeProviderRouter().on("reflection_proposal_v2", func(payload map[string]any) fakeProviderResult {
		captured <- payload
		return fakeProviderResult{Structured: reflectionProposalV2Fixture([]any{})}
	})
	ctx := WithADKCapabilityInvoker(context.Background(), &adkFakeInvoker{}, &ADKCapabilityTrace{})
	_, err = (&ProviderClient{HTTP: &http.Client{Transport: router}}).generateWithEino(ctx, EinoModelCall{
		Assignment: providerAssignment{Role: "reflection", BaseURL: "http://reflection.invalid", ModelID: "fake", Timeout: 10 * time.Second},
		Role:       "reflection", Scenario: "reflection", Messages: formalPromptAssembly(FormalAgentReflection, assembly).Messages,
		JSONMode: true, SchemaName: "reflection_proposal_v2", ResponseSchema: reflectionProposalV2ProviderSchema(), ProviderRequestID: "reflection-source-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	physical := arrayValue((<-captured)["messages"])
	wire := jsonString(physical)
	current := decodeObject([]byte(stringValue(mapValue(physical[len(physical)-1])["content"])))
	if strings.Contains(wire, ambient) || strings.Contains(wire, "wardrobe.inspect") || mapValue(arrayValue(current["evidence"])[0])["observation"] != source || !strings.Contains(wire, "sequence:7") {
		t.Fatal("physical reflection request lost its evidence boundary")
	}
}
